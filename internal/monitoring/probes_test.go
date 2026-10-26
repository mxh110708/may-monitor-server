package monitoring

import (
	"fmt"
	"testing"
	"time"
)

func TestProbeCacheProtectsConfiguredAlertWindowsWithinFixedBound(t *testing.T) {
	rule := fixtureRule()
	rule.Kind = "latency"
	rule.TaskID = 1
	probeCache.Lock()
	priorRows, priorPriority := probeCache.rows, probeCache.priority
	probeCache.rows = map[string][]ProbeSample{}
	probeCache.Unlock()
	defer func() {
		probeCache.Lock()
		probeCache.rows, probeCache.priority = priorRows, priorPriority
		probeCache.Unlock()
	}()
	configureProbeCache(Policy{Schema: 1, Enabled: true, Rules: []Rule{rule}})
	now := time.Now().UTC()
	RecordProbe(ProbeSample{Node: rule.Nodes[0], TaskID: 1, Revision: "selected", At: now.Add(-2 * time.Hour), Status: "ok", Value: 100})
	for n := 0; n < maxProbeKeys+2; n++ {
		RecordProbe(ProbeSample{Node: fmt.Sprintf("unselected-%d", n), TaskID: 2, Revision: "preview", At: now, Status: "ok", Value: 1})
	}
	if len(ProbeSamples(rule.Nodes[0], 1)) != 1 {
		t.Fatal("unselected previews evicted a configured alert window")
	}
	probeCache.Lock()
	size := len(probeCache.rows)
	probeCache.Unlock()
	if size != maxProbeKeys {
		t.Fatal("probe cache exceeded its fixed bound")
	}
}
