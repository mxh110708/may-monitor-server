package monitoring

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func fixtureRule() Rule {
	return Rule{ID: "cpu-test", Name: "CPU", Enabled: true, Kind: "cpu", Nodes: []string{uuid.NewString()}, Threshold: 80, Confirm: 3, Recover: 2, Window: 5, CooldownSeconds: 60}
}
func engineFor(rule Rule) *Engine {
	return NewEngine(Policy{Schema: 1, Enabled: true, Rules: []Rule{rule}})
}

func TestLatencyLimitCannotExceedTheScheduledProbeDeadline(t *testing.T) {
	rule := fixtureRule()
	rule.Kind = "latency"
	rule.TaskID = 1
	rule.Threshold = 3001
	policy := Policy{Schema: 1, Enabled: true, Rules: []Rule{rule}}
	if Validate(policy, time.Now()) == nil {
		t.Fatal("unreachable latency threshold accepted")
	}
	policy.Rules[0].Threshold = 500
	if err := Validate(policy, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestDisabledReceiverDoesNotClaimANetworkAttempt(t *testing.T) {
	rule := fixtureRule()
	rule.Confirm = 1
	engine := engineFor(rule)
	at := time.Now().UTC()
	engine.Step(rule, rule.Nodes[0], NumericObservation(rule, at, 90), at)
	item := engine.Take(at)
	engine.Complete(item.Event.ID, "disabled", at)
	snapshot := engine.Snapshot()
	state := snapshot.States[stateKey(rule.ID, rule.Nodes[0])]
	if state.Delivery != "disabled" || state.DeliveryAttempts != 0 || len(snapshot.Pending) != 0 || snapshot.History[0].Attempts != 0 {
		t.Fatal("unconfigured receiver was presented as a network delivery attempt")
	}
}

func TestDistinctConfirmationsDedupHysteresisAndRecovery(t *testing.T) {
	rule := fixtureRule()
	engine := engineFor(rule)
	at := time.Now().UTC()
	for n := 0; n < 3; n++ {
		sample := NumericObservation(rule, at, 90)
		engine.Step(rule, rule.Nodes[0], sample, at)
	}
	if engine.Take(at) != nil {
		t.Fatal("replayed sample caused an incident")
	}
	engine.Step(rule, rule.Nodes[0], NumericObservation(rule, at.Add(time.Second), 90), at.Add(time.Second))
	engine.Step(rule, rule.Nodes[0], NumericObservation(rule, at.Add(2*time.Second), 90), at.Add(2*time.Second))
	item := engine.Take(at.Add(2 * time.Second))
	if item == nil || item.Event.Kind != "firing" {
		t.Fatal("confirmed incident missing")
	}
	if engine.Take(at.Add(2*time.Second)) != nil {
		t.Fatal("same event selected twice")
	}
	engine.Complete(item.Event.ID, "sent", at.Add(2*time.Second))
	engine.Step(rule, rule.Nodes[0], NumericObservation(rule, at.Add(3*time.Second), 78), at.Add(3*time.Second))
	if !engine.Snapshot().States[stateKey(rule.ID, rule.Nodes[0])].Active {
		t.Fatal("hysteresis cleared too early")
	}
	for n := 4; n <= 5; n++ {
		when := at.Add(time.Duration(n) * time.Second)
		engine.Step(rule, rule.Nodes[0], NumericObservation(rule, when, 70), when)
	}
	item = engine.Take(at.Add(5 * time.Second))
	if item == nil || item.Event.Kind != "recovered" {
		t.Fatal("confirmed recovery missing")
	}
	engine.Complete(item.Event.ID, "sent", at.Add(5*time.Second))
	for n := 6; n <= 8; n++ {
		when := at.Add(time.Duration(n) * time.Second)
		engine.Step(rule, rule.Nodes[0], NumericObservation(rule, when, 90), when)
	}
	if engine.Take(at.Add(8*time.Second)) != nil {
		t.Fatal("cooldown ignored")
	}
	when := at.Add(70 * time.Second)
	engine.Step(rule, rule.Nodes[0], NumericObservation(rule, when, 90), when)
	if engine.Take(when) == nil {
		t.Fatal("persistent incident not sent after cooldown")
	}
}

func TestFailureRetriesAreBoundedAndRestartsPreserveIncident(t *testing.T) {
	rule := fixtureRule()
	rule.Confirm = 1
	engine := engineFor(rule)
	at := time.Now().UTC()
	engine.Step(rule, rule.Nodes[0], NumericObservation(rule, at, 90), at)
	for n, delay := range []time.Duration{0, 5 * time.Second, 35 * time.Second} {
		item := engine.Take(at.Add(delay))
		if item == nil || item.Attempts != n+1 {
			t.Fatal("retry scheduling incorrect")
		}
		engine.Complete(item.Event.ID, "timeout", at.Add(delay))
	}
	if len(engine.Snapshot().Pending) != 0 || engine.Take(at.Add(time.Hour)) != nil {
		t.Fatal("retry limit exceeded")
	}
	restored := engineFor(rule)
	restored.Restore(engine.Snapshot())
	restored.Configure(restored.Policy())
	when := at.Add(time.Hour)
	restored.Step(rule, rule.Nodes[0], NumericObservation(rule, when, 95), when)
	state := restored.Snapshot().States[stateKey(rule.ID, rule.Nodes[0])]
	if restored.Take(when) != nil || state.Delivery != "failed" || state.DeliveryAttempts != 3 {
		t.Fatal("restart retried an exhausted incident or lost its failure")
	}
}

func TestUnknownNaNPermissionAndSilenceCannotCreateFalseRecovery(t *testing.T) {
	rule := fixtureRule()
	rule.Confirm = 1
	rule.Recover = 1
	engine := engineFor(rule)
	at := time.Now().UTC()
	engine.Step(rule, rule.Nodes[0], NumericObservation(rule, at, 95), at)
	item := engine.Take(at)
	engine.Complete(item.Event.ID, "sent", at)
	engine.Step(rule, rule.Nodes[0], Observation{At: at.Add(time.Second), Valid: true, Value: math.NaN()}, at.Add(time.Second))
	state := engine.Snapshot().States[stateKey(rule.ID, rule.Nodes[0])]
	if !state.Active || state.Phase != "unknown" {
		t.Fatal("invalid observation became healthy zero")
	}
	until := at.Add(time.Minute)
	rule.SilenceUntil = &until
	engine.Configure(Policy{Schema: 1, Enabled: true, Rules: []Rule{rule}})
	engine.Step(rule, rule.Nodes[0], NumericObservation(rule, at.Add(2*time.Second), 50), at.Add(2*time.Second))
	if engine.Take(at.Add(2*time.Second)) != nil {
		t.Fatal("silenced recovery sent")
	}
	when := until.Add(time.Second)
	engine.Step(rule, rule.Nodes[0], NumericObservation(rule, when, 50), when)
	if item := engine.Take(when); item == nil || item.Event.Kind != "recovered" {
		t.Fatal("recovery lost after maintenance")
	}
}

func TestProbeFailureRateExcludesRestrictedAndLegacySamples(t *testing.T) {
	rule := fixtureRule()
	rule.Kind = "failure_rate"
	rule.TaskID = 1
	rule.Threshold = 40
	now := time.Now().UTC()
	rows := []ProbeSample{}
	for n := 0; n < 5; n++ {
		status := "ok"
		if n < 2 {
			status = "timeout"
		}
		rows = append(rows, ProbeSample{Revision: "current", Status: status, At: now.Add(time.Duration(n-5) * time.Second)})
	}
	result := observationFromProbeRows(rule, "current", 60, now, rows)
	if !result.Valid || !result.Violating || result.Value != 40 {
		t.Fatal("incorrect failure window")
	}
	rows[4].Status = "permission_denied"
	if observationFromProbeRows(rule, "current", 60, now, rows).Valid {
		t.Fatal("ICMP permission counted as packet loss")
	}
	rows[4].Status = "legacy"
	if observationFromProbeRows(rule, "current", 60, now, rows).Valid {
		t.Fatal("ambiguous legacy error counted as loss")
	}
	rows[4].Status = "ok"
	if observationFromProbeRows(rule, "changed", 60, now, rows).Valid {
		t.Fatal("old target measurements used after edit")
	}
}
