package monitoring

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxRules = 32
const MaxNodes = 100
const MaxPending = 128

type Rule struct {
	Revision        string     `json:"-"`
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Enabled         bool       `json:"enabled"`
	Kind            string     `json:"kind"`
	Nodes           []string   `json:"nodes"`
	TaskID          uint       `json:"task_id,omitempty"`
	Region          string     `json:"region,omitempty"`
	Threshold       float64    `json:"threshold"`
	Confirm         int        `json:"confirm"`
	Recover         int        `json:"recover"`
	Window          int        `json:"window"`
	CooldownSeconds int        `json:"cooldown_seconds"`
	SilenceUntil    *time.Time `json:"silence_until,omitempty"`
}

type Policy struct {
	LocalReceiver string `json:"local_receiver"`
	Schema        int    `json:"schema"`
	Enabled       bool   `json:"enabled"`
	Rules         []Rule `json:"rules"`
}

func Validate(policy Policy, now time.Time) error {
	if policy.LocalReceiver != "" {
		if err := ValidateLocalReceiver(policy.LocalReceiver); err != nil {
			return err
		}
	}
	if policy.Schema != 1 || len(policy.Rules) > MaxRules {
		return errors.New("invalid monitoring policy schema or rule count")
	}
	ids := map[string]bool{}
	nodes := map[string]bool{}
	for _, r := range policy.Rules {
		if !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,48}$`).MatchString(r.ID) || ids[r.ID] || strings.TrimSpace(r.Name) == "" || len(r.Name) > 128 || len(r.Region) > 64 || strings.ContainsAny(r.Name+r.Region, "\r\n\x00") {
			return errors.New("invalid or duplicate monitoring rule identity")
		}
		ids[r.ID] = true
		if len(r.Nodes) < 1 || len(r.Nodes) > MaxNodes || r.Confirm < 1 || r.Confirm > 10 || r.Recover < 1 || r.Recover > 10 || r.Window < 1 || r.Window > 20 || r.CooldownSeconds < 0 || r.CooldownSeconds > 86400 {
			return errors.New("invalid monitoring scope or confirmation limits")
		}
		scope := map[string]bool{}
		for _, id := range r.Nodes {
			value, err := uuid.Parse(id)
			if err != nil || value == uuid.Nil || value.String() != id || scope[id] {
				return errors.New("invalid monitoring node scope")
			}
			scope[id] = true
			nodes[id] = true
		}
		if r.SilenceUntil != nil && r.SilenceUntil.After(now.Add(7*24*time.Hour)) {
			return errors.New("maintenance silence exceeds seven days")
		}
		if math.IsNaN(r.Threshold) || math.IsInf(r.Threshold, 0) {
			return errors.New("invalid monitoring threshold")
		}
		switch r.Kind {
		case "offline":
			if r.TaskID != 0 || r.Threshold != 0 {
				return errors.New("offline rule does not use a probe threshold")
			}
		case "latency":
			if r.TaskID == 0 || r.Threshold < 1 || r.Threshold > 3000 {
				return errors.New("invalid latency threshold or probe")
			}
		case "failure_rate":
			if r.TaskID == 0 || r.Threshold < 1 || r.Threshold > 100 || r.Window < 3 {
				return errors.New("invalid probe failure-rate window")
			}
		case "cpu", "memory", "disk":
			if r.TaskID != 0 || r.Threshold < 1 || r.Threshold > 100 {
				return errors.New("invalid resource threshold")
			}
		default:
			return errors.New("unsupported monitoring rule kind")
		}
	}
	if len(nodes) > MaxNodes {
		return errors.New("monitoring policy exceeds the combined node limit")
	}
	return nil
}

func ruleFingerprint(rule Rule) string {
	// A maintenance window or display-name edit must not restart an incident.
	rule.SilenceUntil = nil
	rule.Name = ""
	rule.Region = ""
	raw, _ := json.Marshal(rule)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func recoveryThreshold(rule Rule) float64 {
	switch rule.Kind {
	case "latency":
		return rule.Threshold * .8
	case "failure_rate":
		return rule.Threshold / 2
	case "cpu", "memory", "disk":
		return math.Max(0, rule.Threshold-5)
	default:
		return rule.Threshold
	}
}

type Observation struct {
	At         time.Time
	Valid      bool
	Violating  bool
	Recovering bool
	Value      float64
	Hint       string
}

func NumericObservation(rule Rule, at time.Time, value float64) Observation {
	valid := !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
	return Observation{At: at, Valid: valid, Value: value, Violating: valid && value >= rule.Threshold, Recovering: valid && value <= recoveryThreshold(rule)}
}
