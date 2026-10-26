package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/komari-monitor/komari/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestControllerLocalReceiverPersistenceRecoveryAndDisabledIdle(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:monitoring_controller_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	config.SetDb(database)
	var mu sync.Mutex
	events := []Event{}
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event Event
		if json.NewDecoder(r.Body).Decode(&event) != nil {
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
		w.WriteHeader(202)
	}))
	defer receiver.Close()
	var usage atomic.Int64
	usage.Store(95)
	rule := fixtureRule()
	rule.Confirm = 2
	rule.Recover = 2
	source := func(_ context.Context, p Policy, known map[string]time.Time, started, now time.Time) (map[string]Observation, map[string]time.Time, error) {
		return map[string]Observation{stateKey(rule.ID, rule.Nodes[0]): NumericObservation(rule, now, float64(usage.Load()))}, known, nil
	}
	controller := Start(source)
	if controller.cancel != nil {
		t.Fatal("disabled configuration started worker timers")
	}
	if err := controller.SetPolicy(Policy{Schema: 1, Enabled: true, LocalReceiver: receiver.URL, Rules: []Rule{rule}}); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	controller.evaluate(context.Background(), at)
	controller.evaluate(context.Background(), at.Add(time.Millisecond))
	waitFor := func(check func() bool) {
		t.Helper()
		until := time.Now().Add(3 * time.Second)
		for time.Now().Before(until) {
			if check() {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("local receiver state did not settle")
	}
	waitFor(func() bool {
		states := controller.Status(0, 100).States
		return len(states) == 1 && states[0].Delivery == "sent"
	})
	controller.Stop()
	controller = Start(source)
	defer controller.Stop()
	controller.evaluate(context.Background(), at.Add(2*time.Millisecond))
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	count := len(events)
	mu.Unlock()
	if count != 1 {
		t.Fatal("server restart duplicated a delivered incident")
	}
	usage.Store(40)
	controller.evaluate(context.Background(), at.Add(3*time.Millisecond))
	controller.evaluate(context.Background(), at.Add(4*time.Millisecond))
	waitFor(func() bool { mu.Lock(); defer mu.Unlock(); return len(events) == 2 })
	mu.Lock()
	if events[0].Kind != "firing" || events[1].Kind != "recovered" || events[0].ID == events[1].ID {
		t.Error("missing distinct firing/recovery identities")
	}
	mu.Unlock()
	if got := ScopedCounts([]string{rule.Nodes[0], "not-in-scope"}); got["not-in-scope"].Active != 0 {
		t.Fatal("unrelated node received alert state")
	}
	if err := controller.SetPolicy(Policy{Schema: 1, Rules: []Rule{}}); err != nil || controller.cancel != nil {
		t.Fatal("disabling retained active workers")
	}
}
