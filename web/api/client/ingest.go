package client

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/database/tasks"
	"github.com/komari-monitor/komari/internal/metricstore"
	"github.com/komari-monitor/komari/internal/monitoring"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

// ingest.go
// agent 上报数据的传输无关处理逻辑。v2 的 HTTP/WebSocket JSON-RPC 入口
// 经过协议解析后，统一调用这里的函数落库并更新运行时状态。

// ingestReport 保存一次负载上报并刷新运行时状态。
// markPresence 为 true 时按 POST 上报会话刷新在线状态（WS 连接自行管理在线状态，应传 false）。
func ingestReport(uuid string, report v2.Report, markPresence bool) error {
	report.UUID = uuid
	report.UpdatedAt = time.Now().UTC()
	if err := clients.ReportVerify(report); err != nil {
		return err
	}
	savedReport, err := metricstore.WriteReport(context.Background(), report)
	if err != nil {
		return err
	}
	agent_runtime.RecordReport(savedReport)
	agent_runtime.MarkV2Client(uuid)
	if markPresence {
		refreshPostPresence(uuid)
	}
	return nil
}

// ingestBasicInfo 保存客户端基础信息。fallbackIP 在上报未携带 IP 时用作兜底。
func ingestBasicInfo(uuid string, info map[string]interface{}, fallbackIP string) error {
	if info == nil {
		info = map[string]interface{}{}
	}
	return saveClientBasicInfo(info, uuid, fallbackIP)
}

// ingestPingResult 保存一条 ping 探测结果。
func ingestPingResult(uuid string, taskID uint, value int) error {
	return tasks.SavePingRecord(models.PingRecord{
		Client: uuid,
		TaskId: taskID,
		Value:  value,
		Time:   time.Now().UTC(),
	})
}

// Only results for the currently assigned probe can affect quality rules.
// Legacy agents remain visible in history, but cannot claim structured evidence.
func ingestStructuredPingResult(node string, params v2.PingResultParams) error {
	var task models.PingTask
	if params.TaskID == 0 || dbcore.GetDBInstance().First(&task, params.TaskID).Error != nil || !task.AppliesToClient(node) {
		return errors.New("probe is unavailable or not assigned to this node")
	}
	status := params.Status
	if status == "" {
		status = "legacy"
	}
	if !slices.Contains([]string{"ok", "network_error", "timeout", "dns_error", "tls_error", "http_status", "permission_denied", "invalid_target", "busy", "legacy"}, status) || params.Value < -1 || params.Value > 10000 || params.HTTPStatus < 0 || params.HTTPStatus > 599 {
		return errors.New("invalid probe result")
	}
	if status != "legacy" && (params.Revision != task.Revision() || params.PingType != task.Type || (status == "ok" && params.Value < 0) || (status != "ok" && params.Value != -1)) {
		return errors.New("probe configuration changed or result is inconsistent")
	}
	if status != "permission_denied" && status != "invalid_target" && status != "busy" {
		if err := ingestPingResult(node, params.TaskID, params.Value); err != nil {
			return err
		}
	}
	monitoring.RecordProbe(monitoring.ProbeSample{Node: node, TaskID: params.TaskID, Type: task.Type, Revision: params.Revision, Status: status, Value: params.Value, HTTPStatus: params.HTTPStatus, At: time.Now().UTC()})
	return nil
}
