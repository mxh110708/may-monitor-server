package monitoring

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/komari-monitor/komari/internal/config"
)

const PolicyKey = "may_monitoring_policy_v1"
const StateKey = "may_monitoring_state_v1"

type SourceFunc func(context.Context, Policy, map[string]time.Time, time.Time, time.Time) (map[string]Observation, map[string]time.Time, error)
type persisted struct {
	Schema int                  `json:"schema"`
	Engine StoredState          `json:"engine"`
	Known  map[string]time.Time `json:"known"`
}

type Controller struct {
	control       sync.Mutex
	persistMu     sync.Mutex
	mu            sync.Mutex
	engine        *Engine
	source        SourceFunc
	known         map[string]time.Time
	started       time.Time
	lastEvaluated time.Time
	sourceError   string
	storageError  bool
	cancel        context.CancelFunc
	workers       sync.WaitGroup
}

var controllerMu sync.RWMutex
var activeController *Controller

func Start(source SourceFunc) *Controller {
	policy, err := config.GetAs[Policy](PolicyKey, Policy{Schema: 1, Rules: []Rule{}})
	initialError := ""
	if err != nil || Validate(policy, time.Now()) != nil {
		policy = Policy{Schema: 1, Rules: []Rule{}}
		initialError = "policy_invalid"
	}
	c := &Controller{engine: NewEngine(policy), source: source, known: map[string]time.Time{}, started: time.Now().UTC(), sourceError: initialError}
	if saved, err := config.GetAs[persisted](StateKey); err == nil && saved.Schema == 1 && len(saved.Known) <= MaxNodes {
		c.engine.Restore(saved.Engine)
		c.engine.Configure(policy)
		if saved.Known != nil {
			c.known = saved.Known
		}
	}
	controllerMu.Lock()
	activeController = c
	controllerMu.Unlock()
	c.configureLoops(policy)
	return c
}

func Current() *Controller {
	controllerMu.RLock()
	defer controllerMu.RUnlock()
	return activeController
}

func (c *Controller) configureLoops(policy Policy) {
	if c.cancel != nil {
		c.cancel()
		c.workers.Wait()
		c.cancel = nil
	}
	c.engine.Configure(policy)
	configureProbeCache(policy)
	enabled := false
	for _, rule := range policy.Rules {
		enabled = enabled || rule.Enabled
	}
	if !policy.Enabled || !enabled {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.workers.Add(2)
	go func() {
		defer c.workers.Done()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		lastSave := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				c.evaluate(ctx, now.UTC())
				if now.Sub(lastSave) >= time.Minute {
					c.save()
					lastSave = now
				}
			}
		}
	}()
	go func() {
		defer c.workers.Done()
		var timer *time.Timer
		for {
			item := c.engine.Take(time.Now().UTC())
			if item != nil {
				// Persist the event identity before delivery. A receiver can deduplicate
				// this ID if the process stops after acceptance but before acknowledgement.
				c.save()
				call, cancel := context.WithTimeout(ctx, 10*time.Second)
				code := SendNotification(call, c.engine.Policy(), item.Event)
				cancel()
				c.engine.Complete(item.Event.ID, code, time.Now().UTC())
				c.save()
				timer = time.NewTimer(time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				continue
			}
			next := c.engine.NextAttempt()
			if next == nil {
				select {
				case <-ctx.Done():
					return
				case <-c.engine.Wake():
				}
			} else {
				delay := time.Until(*next)
				if delay < time.Second {
					delay = time.Second
				}
				timer = time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-c.engine.Wake():
					timer.Stop()
				case <-timer.C:
				}
			}
		}
	}()
}

func (c *Controller) evaluate(ctx context.Context, now time.Time) {
	policy := c.engine.Policy()
	c.mu.Lock()
	known := map[string]time.Time{}
	for id, at := range c.known {
		known[id] = at
	}
	c.mu.Unlock()
	observations, updated, err := c.source(ctx, policy, known, c.started, now)
	if ctx.Err() != nil {
		return
	}
	c.mu.Lock()
	if err != nil {
		c.sourceError = "source_unavailable"
		c.mu.Unlock()
		return
	}
	c.known = updated
	c.lastEvaluated = now
	c.sourceError = ""
	c.mu.Unlock()
	for _, rule := range policy.Rules {
		if !rule.Enabled {
			continue
		}
		for _, node := range rule.Nodes {
			c.engine.Step(rule, node, observations[stateKey(rule.ID, node)], now)
		}
	}
}

func (c *Controller) save() {
	c.persistMu.Lock()
	defer c.persistMu.Unlock()
	c.mu.Lock()
	known := map[string]time.Time{}
	for id, at := range c.known {
		known[id] = at
	}
	c.mu.Unlock()
	err := config.Set(StateKey, persisted{Schema: 1, Engine: c.engine.Snapshot(), Known: known})
	c.mu.Lock()
	c.storageError = err != nil
	c.mu.Unlock()
}

func (c *Controller) SetPolicy(policy Policy) error {
	if policy.Rules == nil {
		policy.Rules = []Rule{}
	}
	if err := Validate(policy, time.Now().UTC()); err != nil {
		return err
	}
	c.control.Lock()
	defer c.control.Unlock()
	if err := config.Set(PolicyKey, policy); err != nil {
		return errors.New("monitoring policy was not saved")
	}
	c.mu.Lock()
	c.sourceError = ""
	allowed := map[string]bool{}
	for _, rule := range policy.Rules {
		for _, node := range rule.Nodes {
			allowed[node] = true
		}
	}
	for node := range c.known {
		if !allowed[node] {
			delete(c.known, node)
		}
	}
	c.mu.Unlock()
	c.configureLoops(policy)
	c.save()
	return nil
}

func (c *Controller) Stop() {
	c.control.Lock()
	defer c.control.Unlock()
	if c.cancel != nil {
		c.cancel()
		c.workers.Wait()
		c.cancel = nil
	}
	c.save()
	controllerMu.Lock()
	if activeController == c {
		activeController = nil
	}
	controllerMu.Unlock()
}

type Status struct {
	Email        string           `json:"email"`
	Policy       Policy           `json:"policy"`
	EvaluatedAt  time.Time        `json:"evaluated_at"`
	SourceError  string           `json:"source_error,omitempty"`
	StorageError bool             `json:"storage_error"`
	Total        int              `json:"total"`
	States       []State          `json:"states"`
	History      []DeliveryRecord `json:"history"`
	Pending      int              `json:"pending"`
}

func (c *Controller) Status(offset, limit int) Status {
	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 100 {
		limit = 100
	}
	snapshot := c.engine.Snapshot()
	states := make([]State, 0, len(snapshot.States))
	for _, state := range snapshot.States {
		states = append(states, *state)
	}
	sort.Slice(states, func(i, j int) bool {
		if states[i].RuleID == states[j].RuleID {
			return states[i].Node < states[j].Node
		}
		return states[i].RuleID < states[j].RuleID
	})
	result := Status{Policy: c.engine.Policy(), Email: EmailStatus(), Total: len(states), States: []State{}, History: snapshot.History, Pending: len(snapshot.Pending)}
	if offset < len(states) {
		result.States = states[offset:min(len(states), offset+limit)]
	}
	c.mu.Lock()
	result.EvaluatedAt = c.lastEvaluated
	result.SourceError = c.sourceError
	result.StorageError = c.storageError
	c.mu.Unlock()
	return result
}

type Counts struct {
	Active         int `json:"active"`
	Silenced       int `json:"silenced"`
	DeliveryFailed int `json:"delivery_failed"`
}

func ScopedCounts(nodes []string) map[string]Counts {
	out := map[string]Counts{}
	current := Current()
	if current == nil {
		return out
	}
	return current.engine.Counts(nodes)
}

func (e *Engine) Counts(nodes []string) map[string]Counts {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := map[string]Counts{}
	for _, node := range nodes {
		count := Counts{}
		for id := range e.rules {
			if state := e.data.States[stateKey(id, node)]; state != nil {
				if state.Active {
					count.Active++
					if state.Silenced {
						count.Silenced++
					}
				}
				if state.Delivery == "failed" {
					count.DeliveryFailed++
				}
			}
		}
		out[node] = count
	}
	return out
}

func (e *Engine) NextAttempt() *time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	var next *time.Time
	for _, item := range e.data.Pending {
		if item.InFlight {
			continue
		}
		if next == nil || item.NextAt.Before(*next) {
			at := item.NextAt
			next = &at
		}
	}
	return next
}
