package jsonrpc

import (
	"context"
	"testing"
	"time"

	"github.com/komari-monitor/komari/internal/monitoring"
	"github.com/komari-monitor/komari/pkg/rpc"
)

func TestMonitoringMethodsRequireAdministratorAndValidateReceiver(t *testing.T) {
	for _, method := range []string{"admin:getMonitoring", "admin:setMonitoring", "admin:getProbeStatus"} {
		for _, role := range []string{rpc.RoleGuest, rpc.RoleClient} {
			response := OnInternalRequest(context.Background(), role, method, map[string]any{})
			if response.Error == nil || response.Error.Code != rpc.PermissionDenied {
				t.Fatalf("%s accepted %s", method, role)
			}
		}
	}
	policy := monitoring.Policy{Schema: 1, LocalReceiver: "https://external.example/events"}
	if monitoring.Validate(policy, time.Now()) == nil {
		t.Fatal("external delivery accepted")
	}
	_, failure := adminSetMonitoring(context.Background(), &rpc.JsonRpcRequest{Params: policy})
	if failure == nil || failure.Code != rpc.InvalidParams {
		t.Fatal("invalid receiver was persisted")
	}
	for _, key := range []string{monitoring.PolicyKey, monitoring.StateKey} {
		_, failure := adminEditSettings(context.Background(), &rpc.JsonRpcRequest{Params: map[string]any{key: map[string]any{}}})
		if failure == nil || failure.Code != rpc.InvalidParams {
			t.Fatal("generic settings bypassed monitoring validation")
		}
	}
}
