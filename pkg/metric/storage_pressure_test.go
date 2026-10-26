package metric

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// SQLite's page quota exercises its real SQLITE_FULL path without filling a
// host disk. A failed flush must retain retryable buckets and never add zeros.
func TestStorageFullFlushRetainsSamplesAndRecoversWithoutDuplicates(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, SQLite("file:pressure?mode=memory&cache=shared", WithMaxOpenConns(1)))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.CreateMetric(ctx, Definition{Name: "pressure", Type: TypeGauge, RetentionDays: 1}); err != nil {
		t.Fatal(err)
	}
	var pages int
	if err = s.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, fmt.Sprintf("PRAGMA max_page_count=%d", pages)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	at := now.Truncate(time.Minute)
	points := make([]Point, 1000)
	for i := range points {
		points[i] = Point{MetricName: "pressure", EntityID: fmt.Sprintf("node-%04d-%s", i, strings.Repeat("x", 128)), Timestamp: at, Value: 7}
	}
	if err = s.WriteBatch(ctx, points); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Flush(ctx, now.Add(2*time.Minute)); err == nil || !strings.Contains(strings.ToLower(err.Error()), "full") {
		t.Fatalf("expected actual SQLite full error: %v", err)
	}
	raw, err := s.Query(ctx, Query{MetricName: "pressure", Start: at.Add(-time.Second), End: at.Add(time.Minute)})
	if err != nil || len(raw) != len(points) {
		t.Fatalf("failed flush lost live samples: rows=%d error=%v", len(raw), err)
	}
	for i := 0; i < 20; i++ {
		if err = s.Write(ctx, Point{MetricName: "pressure", EntityID: fmt.Sprintf("rejected-%d", i), Timestamp: at.Add(time.Second), Value: 99}); err == nil {
			t.Fatal("continued accepting new data after storage failure")
		}
	}
	bounded, err := s.Query(ctx, Query{MetricName: "pressure", Start: at.Add(-time.Second), End: at.Add(time.Minute)})
	if err != nil || len(bounded) != len(points) {
		t.Fatal("failed storage grew the raw queue")
	}
	if _, err = s.db.ExecContext(ctx, "PRAGMA max_page_count=1048576"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Flush(ctx, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = s.Write(ctx, Point{MetricName: "pressure", EntityID: points[0].EntityID, Timestamp: at.Add(3 * time.Minute), Value: 7}); err != nil {
		t.Fatal("writes did not resume after storage recovery:", err)
	}
	rows, err := s.AggregateRollup(ctx, AggregateQuery{Query: Query{MetricName: "pressure", Start: at.Add(-time.Second), End: at.Add(time.Minute)}, Interval: time.Minute, Aggregation: AggSum, PreserveSeries: true}, time.Minute)
	if err != nil || len(rows) != len(points) {
		t.Fatalf("retry did not persist samples: rows=%d error=%v", len(rows), err)
	}
	for _, row := range rows {
		if row.Count != 1 || row.Value != 7 {
			t.Fatal("retry duplicated or zero-filled samples")
		}
	}
}
