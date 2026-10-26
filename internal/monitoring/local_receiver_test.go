package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalReceiverBoundaryAndFailure(t *testing.T) {
	for _, endpoint := range []string{"https://example.com/hook", "http://localhost/hook", "http://192.0.2.1/hook", "http://127.0.0.1/hook?token=x", "http://user:password@127.0.0.1/hook", "file:///hook"} {
		if ValidateLocalReceiver(endpoint) == nil {
			t.Fatalf("accepted a nonlocal or credential-bearing receiver")
		}
	}
	if ValidateLocalReceiver("http://[::1]:8765/hook") != nil {
		t.Fatal("IPv6 loopback rejected")
	}
	var calls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("X-May-Event-ID") != "event-1" {
			t.Error("missing deduplication identity")
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer destination.Close()
	t.Setenv("HTTP_PROXY", "http://192.0.2.1:9")
	event := Event{ID: "event-1", Kind: "firing"}
	if got := SendLocalReceiver(context.Background(), destination.URL, event); got != "sent" || calls.Load() != 1 {
		t.Fatalf("local delivery failed: %s", got)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer redirect.Close()
	if got := SendLocalReceiver(context.Background(), redirect.URL, event); got != "send_failed" || calls.Load() != 1 {
		t.Fatalf("redirect followed: %s", got)
	}
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(100 * time.Millisecond) }))
	defer blocked.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if got := SendLocalReceiver(ctx, blocked.URL, event); got != "timeout" {
		t.Fatalf("deadline not reported: %s", got)
	}
	if got := SendLocalReceiver(context.Background(), "", event); got != "disabled" {
		t.Fatal(got)
	}
}
