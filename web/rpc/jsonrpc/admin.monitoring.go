package jsonrpc

import (
	"context"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/database/tasks"
	"github.com/komari-monitor/komari/internal/monitoring"
	"github.com/komari-monitor/komari/pkg/rpc"
)

func init() {
	RegisterWithGroupAndMeta("getMonitoring", rpc.RoleAdmin, adminGetMonitoring, &rpc.MethodMeta{Summary: "Read bounded monitoring rule states and delivery history"})
	RegisterWithGroupAndMeta("setMonitoring", rpc.RoleAdmin, adminSetMonitoring, &rpc.MethodMeta{Summary: "Save monitoring rules and select local or protected SMTP delivery"})
	RegisterWithGroupAndMeta("getProbeStatus", rpc.RoleAdmin, adminGetProbeStatus, &rpc.MethodMeta{Summary: "Read structured evidence for one assigned probe and node"})
}

func adminGetMonitoring(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	}
	if req.Params != nil {
		if err := req.BindParams(&params); err != nil {
			return nil, rpc.MakeError(rpc.InvalidParams, "invalid pagination", nil)
		}
	}
	controller := monitoring.Current()
	if controller == nil {
		return nil, rpc.MakeError(rpc.InternalError, "monitoring is not running", nil)
	}
	return controller.Status(params.Offset, params.Limit), nil
}

func adminSetMonitoring(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var policy monitoring.Policy
	if err := req.BindParams(&policy); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "invalid monitoring policy", nil)
	}
	if err := monitoring.Validate(policy, time.Now().UTC()); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
	}
	probeList, err := tasks.GetAllPingTasks()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "probes unavailable", nil)
	}
	probes := map[uint]models.PingTask{}
	for _, task := range probeList {
		probes[task.Id] = task
	}
	scope := map[string]bool{}
	for _, rule := range policy.Rules {
		for _, node := range rule.Nodes {
			scope[node] = true
			if rule.Kind == "latency" || rule.Kind == "failure_rate" {
				probe, ok := probes[rule.TaskID]
				if !ok || !probe.AppliesToClient(node) {
					return nil, rpc.MakeError(rpc.InvalidParams, "probe must be assigned to every rule node", nil)
				}
			}
		}
	}
	nodes := make([]string, 0, len(scope))
	for node := range scope {
		nodes = append(nodes, node)
	}
	if len(nodes) > 0 {
		var count int64
		if dbcore.GetDBInstance().WithContext(ctx).Model(&models.Client{}).Where("uuid IN ?", nodes).Count(&count).Error != nil {
			return nil, rpc.MakeError(rpc.InternalError, "nodes unavailable", nil)
		}
		if int(count) != len(nodes) {
			return nil, rpc.MakeError(rpc.InvalidParams, "monitoring node no longer exists", nil)
		}
	}
	controller := monitoring.Current()
	if controller == nil {
		return nil, rpc.MakeError(rpc.InternalError, "monitoring is not running", nil)
	}
	if err := controller.SetPolicy(policy); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "monitoring policy was not applied", nil)
	}
	return controller.Status(0, 100), nil
}

func adminGetProbeStatus(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		Node   string `json:"node"`
		TaskID uint   `json:"task_id"`
	}
	if req.BindParams(&params) != nil || len(params.Node) != 36 || params.TaskID == 0 {
		return nil, rpc.MakeError(rpc.InvalidParams, "node and task_id required", nil)
	}
	var task models.PingTask
	if dbcore.GetDBInstance().First(&task, params.TaskID).Error != nil || !task.AppliesToClient(params.Node) {
		return nil, rpc.MakeError(rpc.InvalidParams, "probe is not assigned to this node", nil)
	}
	return map[string]any{"direction": "node_to_target", "type": task.Type, "revision": task.Revision(), "samples": monitoring.ProbeSamples(params.Node, params.TaskID)}, nil
}
