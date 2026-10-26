package agent

import (
	v2 "github.com/komari-monitor/komari/protocol/v2"
	"testing"
	"time"
)

// Keep a normal 3-second, 20-sample window at steady state, excluding startup.
func BenchmarkReportWindowUpdate(b *testing.B) {
	id := "benchmark-window"
	at := time.Now().UTC()
	for i := 0; i < 20; i++ {
		RecordReport(v2.Report{UUID: id, UpdatedAt: at.Add(time.Duration(i-20) * time.Second)})
	}
	b.Cleanup(func() { DeleteLatestReport(id) })
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		recentReports[id] = recentReports[id][1:]
		mu.Unlock()
		RecordReport(v2.Report{UUID: id, UpdatedAt: at.Add(time.Duration(i%1000) * time.Nanosecond)})
	}
}
