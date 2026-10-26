package agent

import (
	"testing"
	"time"

	v2 "github.com/komari-monitor/komari/protocol/v2"
)

func TestRecordReportKeepsLatestAndShortRecentWindow(t *testing.T) {
	mu.Lock()
	previousLatest := latestReport
	previousRecent := recentReports
	latestReport = make(map[string]*v2.Report)
	recentReports = make(map[string][]v2.Report)
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		latestReport = previousLatest
		recentReports = previousRecent
		mu.Unlock()
	})

	now := time.Now().UTC()
	RecordReport(v2.Report{UUID: "node-a", UpdatedAt: now.Add(-2 * time.Minute), CPU: v2.CPUReport{Usage: 10}})
	RecordReport(v2.Report{UUID: "node-a", UpdatedAt: now.Add(-30 * time.Second), CPU: v2.CPUReport{Usage: 20}})
	RecordReport(v2.Report{UUID: "node-a", UpdatedAt: now.Add(-45 * time.Second), CPU: v2.CPUReport{Usage: 15}})

	recent := GetRecentReports("node-a")
	if len(recent) != 2 || recent[0].CPU.Usage != 15 || recent[1].CPU.Usage != 20 {
		t.Fatalf("recent reports = %#v", recent)
	}
	recent[0].CPU.Usage = 99
	if got := GetRecentReports("node-a"); len(got) != 2 || got[0].CPU.Usage != 15 {
		t.Fatalf("recent report cache was mutated through returned slice: %#v", got)
	}

	latest := GetLatestReport()
	if latest["node-a"] == nil || latest["node-a"].CPU.Usage != 20 {
		t.Fatalf("latest report = %#v", latest["node-a"])
	}
	latest["node-a"].CPU.Usage = 99
	if got := GetLatestReport()["node-a"]; got == nil || got.CPU.Usage != 20 {
		t.Fatalf("latest report cache was mutated through returned map: %#v", got)
	}

	DeleteLatestReport("node-a")
	if len(GetRecentReports("node-a")) != 0 || GetLatestReport()["node-a"] != nil {
		t.Fatal("deleting latest report did not clear runtime report state")
	}
}

func TestRecentWindowHasBoundedCapacityAndReleasesExpiredReferences(t *testing.T) {
	id := "bounded-burst"
	t.Cleanup(func() { DeleteLatestReport(id) })
	now := time.Now().UTC()
	for i := 0; i < 2000; i++ {
		RecordReport(v2.Report{UUID: id, UpdatedAt: now.Add(time.Duration(i) * time.Nanosecond), Message: "retained"})
	}
	mu.RLock()
	reports := recentReports[id]
	length, capacity := len(reports), cap(reports)
	mu.RUnlock()
	if length != maxRecentReports || capacity > maxRecentReports {
		t.Fatalf("unbounded recent cache: len=%d cap=%d", length, capacity)
	}
	if latest := GetLatestReport()[id]; latest == nil || !latest.UpdatedAt.Equal(now.Add(1999*time.Nanosecond)) {
		t.Fatal("burst lost latest report")
	}
	old := []v2.Report{{UpdatedAt: now.Add(-2 * time.Minute), Message: "expired"}, {UpdatedAt: now, Message: "current"}}
	trimmed := reportsAfter(old, now.Add(-time.Minute))
	if len(trimmed) != 1 || trimmed[0].Message != "current" || old[1].Message != "" {
		t.Fatal("expired backing references retained")
	}
}
