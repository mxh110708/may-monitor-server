package server

import (
	"context"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/database/tasks"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/internal/monitoring"
	agentruntime "github.com/komari-monitor/komari/web/agent"
)

func collectMonitoring(ctx context.Context, policy monitoring.Policy, known map[string]time.Time, started, now time.Time) (map[string]monitoring.Observation, map[string]time.Time, error) {
	scope := map[string]bool{}
	for _, rule := range policy.Rules {
		if rule.Enabled {
			for _, node := range rule.Nodes {
				scope[node] = true
			}
		}
	}
	ids := make([]string, 0, len(scope))
	for id := range scope {
		ids = append(ids, id)
	}
	var registered []string
	if len(ids) > 0 {
		if err := dbcore.GetDBInstance().WithContext(ctx).Model(&models.Client{}).Where("uuid IN ?", ids).Pluck("uuid", &registered).Error; err != nil {
			return nil, known, err
		}
	}
	present := map[string]bool{}
	for _, id := range registered {
		present[id] = true
	}
	reports, online := agentruntime.GetScopedMonitoringState(ids, now)
	for id, report := range reports {
		if !report.UpdatedAt.IsZero() {
			known[id] = report.UpdatedAt
		}
	}
	probes, err := tasks.GetAllPingTasks()
	if err != nil {
		return nil, known, err
	}
	probeByID := map[uint]models.PingTask{}
	for _, task := range probes {
		probeByID[task.Id] = task
	}
	var legacy []models.OfflineNotification
	legacyEnabled, _ := config.GetAs[bool](config.NotificationEnabledKey, false)
	if legacyEnabled && len(ids) > 0 {
		if err := dbcore.GetDBInstance().WithContext(ctx).Where("client IN ? AND enable = ?", ids, true).Find(&legacy).Error; err != nil {
			return nil, known, err
		}
	}
	legacyNodes := map[string]bool{}
	for _, value := range legacy {
		legacyNodes[value.Client] = true
	}
	out := map[string]monitoring.Observation{}
	for _, rule := range policy.Rules {
		if !rule.Enabled {
			continue
		}
		for _, node := range rule.Nodes {
			key := rule.ID + "/" + node
			report, hasReport := reports[node]
			unknown := func(reason string) { out[key] = monitoring.Observation{Hint: reason} }
			if !present[node] {
				unknown("removed")
				continue
			}
			if now.Sub(started) < 2*time.Minute && !hasReport {
				unknown("starting")
				continue
			}
			if rule.Kind == "offline" {
				if legacyNodes[node] {
					unknown("legacy_managed")
					continue
				}
				if known[node].IsZero() {
					unknown("waiting")
					continue
				}
				fresh := hasReport && !report.UpdatedAt.IsZero() && now.Sub(report.UpdatedAt) <= 90*time.Second && report.UpdatedAt.Sub(now) <= 5*time.Second
				healthy := online[node] && fresh
				value := 0.0
				if !healthy {
					value = 1
				}
				out[key] = monitoring.Observation{At: now, Valid: true, Value: value, Violating: !healthy, Recovering: healthy}
				continue
			}
			if rule.Kind == "latency" || rule.Kind == "failure_rate" {
				task, exists := probeByID[rule.TaskID]
				if !exists || !task.AppliesToClient(node) {
					unknown("configuration_changed")
					continue
				}
				out[key] = monitoring.ProbeObservation(rule, node, task.Revision(), task.Interval, now)
				continue
			}
			if !hasReport || report.UpdatedAt.IsZero() {
				unknown("waiting")
				continue
			}
			if !online[node] || now.Sub(report.UpdatedAt) > 90*time.Second {
				unknown("stale")
				continue
			}
			value := 0.0
			switch rule.Kind {
			case "cpu":
				value = report.CPU.Usage
			case "memory":
				if report.Ram.Total <= 0 {
					unknown("invalid_metrics")
					continue
				}
				value = float64(report.Ram.Used) * 100 / float64(report.Ram.Total)
			case "disk":
				if report.Disk.Total <= 0 {
					unknown("invalid_metrics")
					continue
				}
				value = float64(report.Disk.Used) * 100 / float64(report.Disk.Total)
			}
			out[key] = monitoring.NumericObservation(rule, report.UpdatedAt, value)
		}
	}
	return out, known, nil
}
