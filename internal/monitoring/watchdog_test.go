package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIndependentWatchdogIdentityAndFailureStates(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000011"
	now := time.Now().UTC()
	code := 200
	identity := id
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer read-test-token" {
			t.Error("read credential must stay in header")
		}
		w.WriteHeader(code)
		fmt.Fprintf(w, `{"schema_version":1,"server_instance_id":%q,"captured_at":%q}`, identity, now.Format(time.RFC3339Nano))
	}))
	cfg := WatchdogConfig{Endpoint: server.URL, ServerInstanceID: id, IntervalSeconds: 10, Confirm: 3, Recover: 2}
	if err := ValidateWatchdog(cfg); err != nil {
		t.Fatal(err)
	}
	check := func(want string) {
		t.Helper()
		got, healthy := CheckWatchdog(context.Background(), cfg, "read-test-token", now)
		if got != want || healthy != (want == "healthy") {
			t.Fatalf("got %s, want %s", got, want)
		}
	}
	check("healthy")
	code = 401
	check("credential_rejected")
	code = 200
	identity = "different"
	check("identity_mismatch")
	identity = id
	now = now.Add(-2 * time.Minute)
	got, ok := CheckWatchdog(context.Background(), cfg, "read-test-token", time.Now())
	if got != "stale" || ok {
		t.Fatal("stale response accepted")
	}
	server.Close()
	check("unreachable")
}
