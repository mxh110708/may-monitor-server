// Package integration exposes a deliberately separate, read-only metrics API.
// Its credential is never promoted to a session, administrator or agent role.
package integration

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/monitoring"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/web/agent"
)

const SnapshotPath = "/api/integration/v1/snapshot"
const TokenPrefix = "may_read_"
const MaxClients = 100

// ReservedAuthorization prevents this namespace from ever becoming an admin
// key, even if an operator mistakenly copies the read token into that setting.
func ReservedAuthorization(header string) bool {
	return strings.HasPrefix(header, "Bearer "+TokenPrefix)
}

type Metrics struct {
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryUsed     int64   `json:"memory_used"`
	MemoryTotal    int64   `json:"memory_total"`
	UploadPerSec   int64   `json:"upload_per_sec"`
	DownloadPerSec int64   `json:"download_per_sec"`
	UploadTotal    int64   `json:"upload_total"`
	DownloadTotal  int64   `json:"download_total"`
}

type Node struct {
	UUID     string             `json:"uuid"`
	Online   bool               `json:"online"`
	ReportAt *time.Time         `json:"report_at"`
	Metrics  *Metrics           `json:"metrics"`
	Alerts   *monitoring.Counts `json:"alerts,omitempty"`
}

type Snapshot struct {
	SchemaVersion       int        `json:"schema_version"`
	ServerVersion       string     `json:"server_version"`
	ServerInstanceID    string     `json:"server_instance_id,omitempty"`
	CapturedAt          time.Time  `json:"captured_at"`
	Nodes               []Node     `json:"nodes"`
	CredentialExpiresAt *time.Time `json:"credential_expires_at,omitempty"`
}

// authorize fails closed unless all three settings are present and valid.
// Credentials and configuration are checked for each request; no token cache,
// database migration, management operation or automatic enrollment is involved.
func authorize(header string, now time.Time) ([]string, bool) {
	if !ReservedAuthorization(header) {
		return nil, false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	if len(token) != len(TokenPrefix)+43 {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(token, TokenPrefix))
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != strings.TrimPrefix(token, TokenPrefix) {
		return nil, false
	}
	expected, err := hex.DecodeString(os.Getenv("MAY_MONITOR_READ_TOKEN_SHA256"))
	actual := sha256.Sum256([]byte(token))
	if err != nil || len(expected) != sha256.Size || subtle.ConstantTimeCompare(expected, actual[:]) != 1 {
		return nil, false
	}
	expires, err := time.Parse(time.RFC3339, os.Getenv("MAY_MONITOR_READ_EXPIRES_AT"))
	if err != nil || !expires.After(now) || expires.After(now.Add(90*24*time.Hour)) {
		return nil, false
	}
	scope := os.Getenv("MAY_MONITOR_READ_CLIENT_UUIDS")
	if len(scope) > MaxClients*37 {
		return nil, false
	}
	ids := strings.Split(scope, ",")
	if len(ids) == 0 || len(ids) > MaxClients {
		return nil, false
	}
	seen := make(map[string]bool, len(ids))
	for i, id := range ids {
		id = strings.TrimSpace(id)
		parsed, err := uuid.Parse(id)
		if err != nil || parsed.String() != id || seen[id] || parsed == uuid.Nil {
			return nil, false
		}
		seen[id], ids[i] = true, id
	}
	return ids, true
}

func ReadSnapshot(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	if c.Request.Method != http.MethodGet {
		c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{"error": "read-only snapshot requires GET"})
		return
	}
	if c.Request.URL.RawQuery != "" || c.Request.ContentLength != 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "read-only snapshot request required"})
		return
	}
	now := time.Now().UTC()
	ids, ok := authorize(c.GetHeader("Authorization"), now)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "read-only credential unavailable, invalid or expired"})
		return
	}
	// Select only UUIDs, not the models' tokens, addresses or private remarks.
	var registered []struct{ UUID string }
	if err := dbcore.GetDBInstance().WithContext(c.Request.Context()).Model(&models.Client{}).Select("uuid").Where("uuid IN ?", ids).Find(&registered).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "monitoring snapshot unavailable"})
		return
	}
	exists := make(map[string]bool, len(registered))
	for _, row := range registered {
		exists[row.UUID] = true
	}
	reports, online := agent.GetScopedMonitoringState(ids, now)
	alertCounts := monitoring.ScopedCounts(ids)
	snapshot := Snapshot{SchemaVersion: 1, ServerVersion: utils.CurrentVersion, ServerInstanceID: utils.MonitorInstanceID(), CapturedAt: now, Nodes: make([]Node, 0, len(registered))}
	if expires, err := time.Parse(time.RFC3339, os.Getenv("MAY_MONITOR_READ_EXPIRES_AT")); err == nil {
		snapshot.CredentialExpiresAt = &expires
	}
	for _, id := range ids {
		if !exists[id] {
			continue
		}
		node := Node{UUID: id, Online: online[id]}
		if counts, found := alertCounts[id]; found {
			node.Alerts = &counts
		}
		if report, found := reports[id]; found && !report.UpdatedAt.IsZero() {
			at := report.UpdatedAt.UTC()
			node.ReportAt = &at
			// An invalid agent sample is absence of usable data, never zero load.
			if !math.IsNaN(report.CPU.Usage) && !math.IsInf(report.CPU.Usage, 0) && report.CPU.Usage >= 0 && report.CPU.Usage <= 100 && report.Ram.Used >= 0 && report.Ram.Total > 0 && report.Ram.Used <= report.Ram.Total && report.Network.Up >= 0 && report.Network.Down >= 0 && report.Network.TotalUp >= 0 && report.Network.TotalDown >= 0 {
				node.Metrics = &Metrics{report.CPU.Usage, report.Ram.Used, report.Ram.Total, report.Network.Up, report.Network.Down, report.Network.TotalUp, report.Network.TotalDown}
			}
		}
		snapshot.Nodes = append(snapshot.Nodes, node)
	}
	// Reports can arrive while this response is assembled. Stamp the completed
	// snapshot after copying them so a new report cannot appear newer merely
	// because it landed between request start and the scoped state read.
	snapshot.CapturedAt = time.Now().UTC()
	c.JSON(http.StatusOK, snapshot)
}
