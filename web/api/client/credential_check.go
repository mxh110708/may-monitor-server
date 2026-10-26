package client

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/web/agent"
)

// CheckCredential confirms the identity resolved by the normal agent token
// middleware without reporting synthetic metrics or changing inventory. The
// caller supplies its existing token in JSON, never in a probe URL.
func CheckCredential(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	uuid := c.GetString("client_uuid")
	if uuid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "agent credential required"})
		return
	}
	now := time.Now().UTC()
	reports, online := agent.GetScopedMonitoringState([]string{uuid}, now)
	var reportedAt *time.Time
	if report, exists := reports[uuid]; exists && !report.UpdatedAt.IsZero() {
		at := report.UpdatedAt.UTC()
		reportedAt = &at
	}
	// Reading this status never registers a node or renews its online lease.
	// Return only the authenticated node's timestamps, not its host metrics.
	c.JSON(http.StatusOK, gin.H{"schema_version": 1, "maintenance_schema_version": 1, "uuid": uuid, "server_version": utils.CurrentVersion, "server_instance_id": utils.MonitorInstanceID(), "captured_at": now, "online": online[uuid], "report_at": reportedAt})
}
