package monitoring

import (
	"math"
	"strconv"
	"sync"
	"time"
)

type ProbeSample struct {
	Node       string    `json:"node"`
	TaskID     uint      `json:"task_id"`
	Type       string    `json:"type"`
	Revision   string    `json:"revision"`
	Status     string    `json:"status"`
	Value      int       `json:"value"`
	HTTPStatus int       `json:"http_status,omitempty"`
	At         time.Time `json:"at"`
}

var probeCache = struct {
	sync.Mutex
	rows     map[string][]ProbeSample
	priority map[string]bool
}{rows: map[string][]ProbeSample{}, priority: map[string]bool{}}

const maxProbeKeys = 4096

// Preserve evidence for configured alert windows before the optional recent
// preview cache. At most MaxRules*MaxNodes (3200) priority keys can exist.
func configureProbeCache(policy Policy) {
	priority := map[string]bool{}
	if policy.Enabled {
		for _, rule := range policy.Rules {
			if rule.Enabled && (rule.Kind == "latency" || rule.Kind == "failure_rate") {
				for _, node := range rule.Nodes {
					priority[probeCacheKey(node, rule.TaskID)] = true
				}
			}
		}
	}
	probeCache.Lock()
	probeCache.priority = priority
	probeCache.Unlock()
}

func probeCacheKey(node string, task uint) string { return stateKey(node, stringID(task)) }
func stringID(value uint) string                  { return strconv.FormatUint(uint64(value), 10) }
func RecordProbe(sample ProbeSample) {
	probeCache.Lock()
	defer probeCache.Unlock()
	key := probeCacheKey(sample.Node, sample.TaskID)
	if _, exists := probeCache.rows[key]; !exists && len(probeCache.rows) >= maxProbeKeys {
		oldest := ""
		var oldestAt time.Time
		for candidate, rows := range probeCache.rows {
			if probeCache.priority[candidate] {
				continue
			}
			if len(rows) == 0 {
				oldest = candidate
				break
			}
			at := rows[len(rows)-1].At
			if oldest == "" || at.Before(oldestAt) {
				oldest = candidate
				oldestAt = at
			}
		}
		if oldest == "" {
			return
		}
		delete(probeCache.rows, oldest)
	}
	rows := probeCache.rows[key]
	if len(rows) > 0 && (rows[len(rows)-1].Revision != sample.Revision || rows[len(rows)-1].Type != sample.Type) {
		rows = nil
	}
	if len(rows) == 20 {
		copy(rows, rows[1:])
		rows = rows[:19]
	}
	probeCache.rows[key] = append(rows, sample)
}

func ProbeSamples(node string, task uint) []ProbeSample {
	probeCache.Lock()
	defer probeCache.Unlock()
	return append([]ProbeSample(nil), probeCache.rows[probeCacheKey(node, task)]...)
}

func ProbeObservation(rule Rule, node, expectedRevision string, interval int, now time.Time) Observation {
	rows := ProbeSamples(node, rule.TaskID)
	return observationFromProbeRows(rule, expectedRevision, interval, now, rows)
}

func observationFromProbeRows(rule Rule, revision string, interval int, now time.Time, rows []ProbeSample) Observation {
	if len(rows) == 0 {
		return Observation{Hint: "waiting"}
	}
	last := rows[len(rows)-1]
	result := Observation{At: last.At, Hint: last.Status}
	if last.Revision != revision {
		return Observation{Hint: "configuration_changed"}
	}
	maxAge := time.Duration(max(90, interval*2+10)) * time.Second
	if now.Sub(last.At) > maxAge {
		return Observation{At: last.At, Hint: "stale"}
	}
	if last.Status == "permission_denied" || last.Status == "busy" || last.Status == "invalid_target" || last.Status == "legacy" {
		return result
	}
	if rule.Kind == "latency" {
		if last.Status != "ok" {
			return result
		}
		return NumericObservation(rule, last.At, float64(last.Value))
	}
	if len(rows) < rule.Window {
		return Observation{At: last.At, Hint: "collecting"}
	}
	failures := 0
	for _, row := range rows[len(rows)-rule.Window:] {
		if row.Revision != revision || now.Sub(row.At) > maxAge*time.Duration(rule.Window) || row.Status == "permission_denied" || row.Status == "busy" || row.Status == "invalid_target" || row.Status == "legacy" {
			return Observation{Hint: "unknown"}
		}
		if row.Status != "ok" {
			failures++
		}
	}
	return NumericObservation(rule, last.At, math.Round(float64(failures)*10000/float64(rule.Window))/100)
}
