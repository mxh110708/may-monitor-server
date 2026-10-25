package securityaudit

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	"github.com/komari-monitor/komari/web/agent"
	"github.com/komari-monitor/komari/web/integration"
)

func readIntegrationBoundary(t *testing.T, engine *gin.Engine) {
	t.Helper()
	token := integration.TokenPrefix + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	digest := sha256.Sum256([]byte(token))
	id := "14dd1315-1a18-4320-99ca-5fd0fefc13b3"
	other := "ad16f260-6ed1-40a9-8bbc-d07cb3a726e3"
	missing := "eaed1347-3a10-4384-8f7b-1066268f85bb"
	for _, clientID := range []string{id, other} {
		client := models.Client{UUID: clientID, Token: "synthetic-agent-secret-" + clientID, Name: "synthetic-private-name", IPv4: "192.0.2.9", Remark: "synthetic-private-remark"}
		if err := dbcore.GetDBInstance().Create(&client).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { dbcore.GetDBInstance().Delete(&models.Client{}, "uuid = ?", clientID) })
	}
	oldPrivate, _ := config.GetAs[bool](config.PrivateSiteKey, false)
	oldKey, _ := config.GetAs[string](config.ApiKeyKey, "")
	if err := config.Set(config.PrivateSiteKey, true); err != nil {
		t.Fatal(err)
	}
	if err := config.Set(config.ApiKeyKey, token); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.Set(config.PrivateSiteKey, oldPrivate); _ = config.Set(config.ApiKeyKey, oldKey) })
	t.Setenv("MAY_MONITOR_READ_TOKEN_SHA256", hex.EncodeToString(digest[:]))
	t.Setenv("MAY_MONITOR_READ_CLIENT_UUIDS", id+","+missing)
	t.Setenv("MAY_MONITOR_READ_EXPIRES_AT", time.Now().UTC().Add(time.Hour).Format(time.RFC3339))
	instanceID := uuid.NewString()
	t.Setenv("MAY_MONITOR_INSTANCE_ID", instanceID)
	report := v2.Report{UUID: id, CPU: v2.CPUReport{Usage: 12.5}, Ram: v2.RamReport{Total: 1024, Used: 512}, Network: v2.NetworkReport{Up: 10, Down: 20, TotalUp: 100, TotalDown: 200}, UpdatedAt: time.Now().UTC(), Message: "synthetic-private-message"}
	agent.RecordReport(report)
	agent.SetPresence(id, 17, true)
	t.Cleanup(func() { agent.SetPresence(id, 17, false); agent.DeleteLatestReport(id) })
	request := func(method, path, authorization string) *httptest.ResponseRecorder {
		body := ""
		if path == "/api/rpc2" {
			body = `{"jsonrpc":"2.0","id":1,"method":"admin:getSettings","params":{}}`
		}
		if path == "/api/clients/credential-check" {
			encoded, _ := json.Marshal(map[string]string{"token": "synthetic-agent-secret-" + id})
			body = string(encoded)
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", authorization)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		return w
	}
	w := request(http.MethodGet, integration.SnapshotPath, "Bearer "+token)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("snapshot status=%d", w.Code)
	}
	var snapshot integration.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Nodes) != 1 || snapshot.Nodes[0].UUID != id || !snapshot.Nodes[0].Online || snapshot.Nodes[0].Metrics == nil || snapshot.Nodes[0].Metrics.CPUPercent != 12.5 {
		t.Fatal("scope or metric projection failed")
	}
	if snapshot.ServerInstanceID != instanceID {
		t.Fatal("authenticated origin identity missing")
	}
	for _, secret := range []string{token, other, missing, "synthetic-agent-secret", "synthetic-private-name", "synthetic-private-remark", "synthetic-private-message", "192.0.2.9"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("non-metric data disclosed")
		}
	}
	for _, path := range []string{"/api/admin/settings/", "/api/nodes", "/api/clients/v2/rpc"} {
		if response := request(http.MethodGet, path, "Bearer "+token); response.Code != http.StatusUnauthorized {
			t.Fatalf("read credential gained another role: %s status=%d", path, response.Code)
		}
	}
	if response := request(http.MethodPost, "/api/rpc2", "Bearer "+token); !strings.Contains(response.Body.String(), `"error"`) || strings.Contains(response.Body.String(), `"result"`) {
		t.Fatal("read credential gained administrator RPC access")
	}
	if response := request(http.MethodPost, "/api/clients/credential-check", ""); response.Code != 200 || !strings.Contains(response.Body.String(), id) || !strings.Contains(response.Body.String(), instanceID) || strings.Contains(response.Body.String(), "synthetic-agent-secret") {
		t.Fatal("dedicated agent credential check did not return only its node identity")
	}
	if response := request(http.MethodPost, "/api/clients/credential-check", "Bearer "+token); response.Code != 401 {
		t.Fatal("read-only credential was promoted to agent identity")
	}
	if request(http.MethodGet, integration.SnapshotPath, "").Code != 401 {
		t.Fatal("anonymous snapshot allowed")
	}
	if request(http.MethodGet, integration.SnapshotPath+"?token=synthetic", "Bearer "+token).Code != 400 {
		t.Fatal("query credentials accepted")
	}
	if request(http.MethodPost, integration.SnapshotPath, "Bearer "+token).Code != 405 {
		t.Fatal("write method accepted")
	}
	agent.SetPresence(id, 17, false)
	w = request(http.MethodGet, integration.SnapshotPath, "Bearer "+token)
	if err := json.Unmarshal(w.Body.Bytes(), &snapshot); err != nil || snapshot.Nodes[0].Online {
		t.Fatal("offline presence misreported")
	}
	report.CPU.Usage = math.NaN()
	report.UpdatedAt = time.Now().UTC()
	agent.RecordReport(report)
	w = request(http.MethodGet, integration.SnapshotPath, "Bearer "+token)
	if err := json.Unmarshal(w.Body.Bytes(), &snapshot); err != nil || snapshot.Nodes[0].Metrics != nil {
		t.Fatal("invalid metric reported as healthy zero")
	}
	t.Setenv("MAY_MONITOR_READ_TOKEN_SHA256", "")
	if request(http.MethodGet, integration.SnapshotPath, "Bearer "+token).Code != 401 {
		t.Fatal("revoked credential accepted")
	}
}
