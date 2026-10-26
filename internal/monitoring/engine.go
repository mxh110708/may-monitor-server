package monitoring

import (
	"encoding/json"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

type State struct {
	RuleID           string    `json:"rule_id"`
	Node             string    `json:"node"`
	Phase            string    `json:"phase"`
	Active           bool      `json:"active"`
	Value            *float64  `json:"value"`
	EvidenceAt       time.Time `json:"evidence_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Delivery         string    `json:"delivery"`
	DeliveryAttempts int       `json:"delivery_attempts"`
	DeliveryError    string    `json:"delivery_error,omitempty"`
	Silenced         bool      `json:"silenced"`
	Bad              int       `json:"bad_count"`
	Good             int       `json:"good_count"`
	Generation       int64     `json:"generation"`
	Attempted        bool      `json:"attempted"`
	FiringDelivered  bool      `json:"firing_delivered"`
	RecoveryPending  bool      `json:"recovery_pending"`
	LastSentAt       time.Time `json:"last_sent_at"`
	Fingerprint      string    `json:"fingerprint"`
}

type Event struct {
	ID         string    `json:"id"`
	RuleID     string    `json:"rule_id"`
	RuleName   string    `json:"rule_name"`
	Node       string    `json:"node"`
	Kind       string    `json:"kind"`
	Condition  string    `json:"condition"`
	Value      *float64  `json:"value"`
	At         time.Time `json:"at"`
	Generation int64     `json:"generation"`
}

type Delivery struct {
	Event    Event     `json:"event"`
	Attempts int       `json:"attempts"`
	NextAt   time.Time `json:"next_at"`
	InFlight bool      `json:"in_flight"`
}

type DeliveryRecord struct {
	Event    Event     `json:"event"`
	Status   string    `json:"status"`
	Attempts int       `json:"attempts"`
	At       time.Time `json:"at"`
}

type StoredState struct {
	Schema  int                  `json:"schema"`
	States  map[string]*State    `json:"states"`
	Pending map[string]*Delivery `json:"pending"`
	History []DeliveryRecord     `json:"history"`
}

type Engine struct {
	mu           sync.Mutex
	policy       Policy
	rules        map[string]Rule
	fingerprints map[string]string
	scopes       map[string]map[string]bool
	data         StoredState
	wake         chan struct{}
}

func NewEngine(policy Policy) *Engine {
	engine := &Engine{data: StoredState{Schema: 1, States: map[string]*State{}, Pending: map[string]*Delivery{}}, wake: make(chan struct{}, 1)}
	engine.Configure(policy)
	return engine
}
func stateKey(rule, node string) string { return rule + "/" + node }
func (e *Engine) Policy() Policy {
	e.mu.Lock()
	defer e.mu.Unlock()
	raw, _ := json.Marshal(e.policy)
	var out Policy
	json.Unmarshal(raw, &out)
	for index := range out.Rules {
		out.Rules[index].Revision = e.fingerprints[out.Rules[index].ID]
	}
	return out
}

func (e *Engine) Configure(policy Policy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	raw, _ := json.Marshal(policy)
	json.Unmarshal(raw, &e.policy)
	e.rules = map[string]Rule{}
	e.fingerprints = map[string]string{}
	e.scopes = map[string]map[string]bool{}
	allowed := map[string]string{}
	if policy.Enabled {
		for _, rule := range policy.Rules {
			if rule.Enabled {
				fingerprint := ruleFingerprint(rule)
				e.rules[rule.ID] = rule
				e.fingerprints[rule.ID] = fingerprint
				e.scopes[rule.ID] = map[string]bool{}
				for _, node := range rule.Nodes {
					allowed[stateKey(rule.ID, node)] = fingerprint
					e.scopes[rule.ID][node] = true
				}
			}
		}
	}
	for key, state := range e.data.States {
		if allowed[key] != state.Fingerprint {
			delete(e.data.States, key)
		} else {
			rule := e.rules[state.RuleID]
			state.Silenced = rule.SilenceUntil != nil && rule.SilenceUntil.After(time.Now())
		}
	}
	for id, item := range e.data.Pending {
		if e.data.States[stateKey(item.Event.RuleID, item.Event.Node)] == nil {
			delete(e.data.Pending, id)
		}
	}
}

func (e *Engine) Step(rule Rule, node string, sample Observation, now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.policy.Enabled || !rule.Enabled {
		return
	}
	registered, exists := e.rules[rule.ID]
	if !exists || !e.scopes[rule.ID][node] || rule.Revision != "" && rule.Revision != e.fingerprints[rule.ID] {
		return
	}
	rule = registered
	if rule.Kind != "offline" && sample.Valid {
		numeric := NumericObservation(rule, sample.At, sample.Value)
		sample.Violating = numeric.Violating
		sample.Recovering = numeric.Recovering
		sample.Valid = numeric.Valid
	}
	key := stateKey(rule.ID, node)
	if sample.Hint == "removed" || sample.Hint == "legacy_managed" {
		delete(e.data.States, key)
		for id, pending := range e.data.Pending {
			if pending.Event.RuleID == rule.ID && pending.Event.Node == node && !pending.InFlight {
				delete(e.data.Pending, id)
			}
		}
		if sample.Hint == "legacy_managed" {
			e.data.States[key] = &State{RuleID: rule.ID, Node: node, Phase: "legacy_managed", Delivery: "none", Fingerprint: e.fingerprints[rule.ID], UpdatedAt: now}
		}
		return
	}
	state := e.data.States[key]
	if state == nil || state.Fingerprint != e.fingerprints[rule.ID] {
		state = &State{RuleID: rule.ID, Node: node, Phase: "waiting", Delivery: "none", Fingerprint: e.fingerprints[rule.ID]}
		e.data.States[key] = state
	}
	state.Silenced = rule.SilenceUntil != nil && rule.SilenceUntil.After(now)
	if !sample.Valid || math.IsNaN(sample.Value) || math.IsInf(sample.Value, 0) || sample.At.IsZero() || sample.At.After(now.Add(5*time.Second)) {
		state.Bad = 0
		state.Good = 0
		state.Phase = sample.Hint
		if state.Phase == "" {
			state.Phase = "unknown"
		}
		state.UpdatedAt = now
		return
	}
	if sample.At.After(state.EvidenceAt) {
		state.EvidenceAt = sample.At
		value := sample.Value
		state.Value = &value
		state.UpdatedAt = now
		if sample.Violating {
			state.Good = 0
			if state.Bad < rule.Confirm {
				state.Bad++
			}
			if !state.Active && state.Bad >= rule.Confirm {
				state.Active = true
				state.Generation++
				state.Attempted = false
				state.FiringDelivered = false
				state.Delivery = "none"
				state.DeliveryAttempts = 0
				state.DeliveryError = ""
			}
			if state.Active {
				state.Phase = "alerting"
			} else {
				state.Phase = "pending"
			}
		} else {
			state.Bad = 0
			if state.Active {
				if sample.Recovering {
					state.Good++
				} else {
					state.Good = 0
				}
				state.Phase = "alerting"
				if state.Good > 0 {
					state.Phase = "recovering"
				}
				if state.Good >= rule.Recover {
					state.Active = false
					state.Good = 0
					state.Generation++
					state.Phase = "healthy"
					if state.FiringDelivered {
						state.RecoveryPending = true
					}
				}
			} else {
				state.Good = 0
				state.Phase = "healthy"
			}
		}
	}
	if !state.Active && state.RecoveryPending && !state.Silenced {
		if e.queue(rule, state, "recovered", now) {
			state.RecoveryPending = false
		}
	}
	if state.Active && !state.Attempted {
		if state.Silenced {
			state.Delivery = "muted"
		} else if !state.LastSentAt.IsZero() && now.Sub(state.LastSentAt) < time.Duration(rule.CooldownSeconds)*time.Second {
			state.Delivery = "cooldown"
		} else {
			e.queue(rule, state, "firing", now)
		}
	}
}

func (e *Engine) queue(rule Rule, state *State, kind string, now time.Time) bool {
	for _, item := range e.data.Pending {
		if item.Event.RuleID == rule.ID && item.Event.Node == state.Node && item.Event.Generation == state.Generation && item.Event.Kind == kind {
			return true
		}
	}
	if len(e.data.Pending) >= MaxPending {
		state.Delivery = "queue_full"
		return false
	}
	event := Event{ID: uuid.NewString(), RuleID: rule.ID, RuleName: rule.Name, Node: state.Node, Kind: kind, Condition: rule.Kind, Value: state.Value, At: now, Generation: state.Generation}
	e.data.Pending[event.ID] = &Delivery{Event: event, NextAt: now}
	state.Delivery = "pending"
	state.DeliveryAttempts = 0
	state.DeliveryError = ""
	if kind == "firing" {
		state.Attempted = true
	}
	select {
	case e.wake <- struct{}{}:
	default:
	}
	return true
}

// Take marks one item in flight, allowing at most one attempt per event even
// if callers overlap. Superseded incidents are removed before they can send.
func (e *Engine) Take(now time.Time) *Delivery {
	e.mu.Lock()
	defer e.mu.Unlock()
	var first *Delivery
	for id, item := range e.data.Pending {
		state := e.data.States[stateKey(item.Event.RuleID, item.Event.Node)]
		if state == nil || state.Generation != item.Event.Generation || state.Silenced {
			if !item.InFlight {
				if state != nil && state.Generation == item.Event.Generation && state.Silenced {
					if item.Event.Kind == "firing" {
						state.Attempted = false
						state.Delivery = "muted"
					} else {
						state.RecoveryPending = true
					}
				}
				delete(e.data.Pending, id)
			}
			continue
		}
		if item.InFlight || item.NextAt.After(now) {
			continue
		}
		if first == nil || item.NextAt.Before(first.NextAt) || item.NextAt.Equal(first.NextAt) && item.Event.ID < first.Event.ID {
			first = item
		}
	}
	if first == nil {
		return nil
	}
	first.InFlight = true
	first.Attempts++
	copy := *first
	return &copy
}

// code is a bounded status, never a provider response body, URL or credential.
func (e *Engine) Complete(id, code string, now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	item := e.data.Pending[id]
	if item == nil {
		return
	}
	item.InFlight = false
	state := e.data.States[stateKey(item.Event.RuleID, item.Event.Node)]
	current := state != nil && state.Generation == item.Event.Generation
	if code == "sent" {
		if current {
			state.Delivery = "sent"
			state.DeliveryAttempts = item.Attempts
			state.DeliveryError = ""
			state.LastSentAt = now
			if item.Event.Kind == "firing" {
				state.FiringDelivered = true
			}
		}
		if !current && state != nil && item.Event.Kind == "firing" && !state.Active {
			state.RecoveryPending = true
			if !state.Silenced {
				for _, rule := range e.policy.Rules {
					if rule.ID == state.RuleID {
						if e.queue(rule, state, "recovered", now) {
							state.RecoveryPending = false
						}
						break
					}
				}
			}
		}
		e.record(item, code, now)
		delete(e.data.Pending, id)
		return
	}
	if code != "disabled" && code != "timeout" && code != "network_error" && code != "send_failed" {
		code = "send_failed"
	}
	if code == "disabled" {
		item.Attempts = 0
	}
	if current {
		state.DeliveryAttempts = item.Attempts
		state.DeliveryError = code
		state.Delivery = "retrying"
	}
	if !current || code == "disabled" || item.Attempts >= 3 {
		if current {
			state.Delivery = "failed"
			if code == "disabled" {
				state.Delivery = "disabled"
			}
		}
		e.record(item, code, now)
		delete(e.data.Pending, id)
		return
	}
	delays := []time.Duration{5 * time.Second, 30 * time.Second}
	item.NextAt = now.Add(delays[item.Attempts-1])
}

func (e *Engine) record(item *Delivery, status string, now time.Time) {
	e.data.History = append(e.data.History, DeliveryRecord{Event: item.Event, Status: status, Attempts: item.Attempts, At: now})
	if len(e.data.History) > 200 {
		copy(e.data.History, e.data.History[len(e.data.History)-200:])
		clear(e.data.History[200:])
		e.data.History = e.data.History[:200]
	}
}

func (e *Engine) Snapshot() StoredState {
	e.mu.Lock()
	defer e.mu.Unlock()
	raw, _ := json.Marshal(e.data)
	var out StoredState
	json.Unmarshal(raw, &out)
	return out
}

func (e *Engine) Restore(data StoredState) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if data.Schema != 1 || len(data.States) > MaxRules*MaxNodes || len(data.Pending) > MaxPending || len(data.History) > 200 {
		return
	}
	if data.States == nil {
		data.States = map[string]*State{}
	}
	if data.Pending == nil {
		data.Pending = map[string]*Delivery{}
	}
	for id, item := range data.Pending {
		if item == nil || item.Attempts >= 3 {
			delete(data.Pending, id)
		} else {
			item.InFlight = false
		}
	}
	for key, state := range data.States {
		if state == nil {
			delete(data.States, key)
		}
	}
	e.data = data
}

func (e *Engine) Wake() <-chan struct{} { return e.wake }
