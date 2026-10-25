package client

import (
	"testing"
	"time"

	"github.com/google/uuid"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

func TestWebSocketPullDoesNotCreatePostLease(t *testing.T) {
	id := uuid.NewString()
	response := handleV2RPC(id, v2.Request{JSONRPC: v2.Version, Method: v2.MethodAgentPull, Params: map[string]any{}}, false)
	if response.Error != nil {
		t.Fatal(response.Error)
	}
	postPresenceMu.Lock()
	_, exists := postPresenceStates[id]
	postPresenceMu.Unlock()
	_, online := agent_runtime.GetScopedMonitoringState([]string{id}, time.Now())
	if exists || online[id] {
		t.Fatal("WebSocket polling created a fallback online lease")
	}
	agent_runtime.DeleteConnectedClients(id)
}

func TestWebSocketTakeoverRetiresOnlyItsPostLease(t *testing.T) {
	id, other := uuid.NewString(), uuid.NewString()
	for _, value := range []string{id, other} {
		timer := time.AfterFunc(time.Hour, func() {})
		postPresenceMu.Lock()
		postPresenceStates[value] = &postPresenceEntry{connID: 13, timer: timer}
		postPresenceMu.Unlock()
		agent_runtime.KeepAlivePresence(value, 13, time.Minute)
	}
	t.Cleanup(func() { clearPostPresenceForWebSocket(id); clearPostPresenceForWebSocket(other) })
	clearPostPresenceForWebSocket(id)
	_, online := agent_runtime.GetScopedMonitoringState([]string{id, other}, time.Now())
	if online[id] || !online[other] {
		t.Fatal("WebSocket takeover retained its old lease or cleared another client")
	}
	clearPostPresenceForWebSocket(id) // idempotent
}
