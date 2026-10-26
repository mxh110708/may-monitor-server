package client

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	"github.com/komari-monitor/komari/web/agent"
)

func TestMaintenanceStatusIsScopedReadOnlyAndDoesNotRenewPresence(t *testing.T) {
	id, other := uuid.NewString(), uuid.NewString()
	origin := uuid.NewString()
	t.Setenv("MAY_MONITOR_INSTANCE_ID", origin)
	at := time.Now().UTC().Add(-time.Second)
	agent.RecordReport(v2.Report{UUID: id, UpdatedAt: at, Message: "private-message", CPU: v2.CPUReport{Usage: 42}})
	agent.RecordReport(v2.Report{UUID: other, UpdatedAt: at, Message: "other-private"})
	agent.KeepAlivePresence(id, 19, -time.Second)
	t.Cleanup(func() {
		agent.DeleteLatestReport(id)
		agent.DeleteLatestReport(other)
		agent.SetPresence(id, 19, false)
	})
	read := func(node string) map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		if node != "" {
			c.Set("client_uuid", node)
		}
		CheckCredential(c)
		if node == "" {
			if w.Code != 401 {
				t.Fatal("anonymous accepted")
			}
			return nil
		}
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("status cache boundary")
		}
		for _, secret := range []string{other, "private-message", "other-private", "cpu", "metrics", "token"} {
			if strings.Contains(w.Body.String(), `"`+secret+`"`) {
				t.Fatalf("disclosed %s", secret)
			}
		}
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	read("")
	for i := 0; i < 3; i++ {
		got := read(id)
		stamp, err := time.Parse(time.RFC3339Nano, got["report_at"].(string))
		if err != nil || !stamp.Equal(at) || got["uuid"] != id || got["server_instance_id"] != origin || got["maintenance_schema_version"] != float64(1) || got["online"] != false {
			t.Fatal("status read changed report or online state")
		}
	}
	agent.KeepAlivePresence(id, 20, time.Minute)
	if read(id)["online"] != true {
		t.Fatal("real online state missing")
	}
	agent.SetPresence(id, 19, false) // stale disconnect must not clear new session
	if read(id)["online"] != true {
		t.Fatal("old disconnect cleared newer lease")
	}
	agent.SetPresence(id, 20, false)
	agent.DeleteLatestReport(id)
	if got := read(id); got["report_at"] != nil || got["online"] != false {
		t.Fatal("missing data reported as real sample")
	}
}
