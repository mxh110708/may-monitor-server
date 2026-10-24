// Defensive regressions: synthetic credentials, temporary SQLite and loopback
// listeners only. No production hosts or remote commands are used.
package securityaudit

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/router"
	"github.com/komari-monitor/komari/web/security"
	"github.com/pquerna/otp/totp"
)

func TestSecurityBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = filepath.Join(t.TempDir(), "synthetic.db")
	db := dbcore.GetDBInstance()
	config.SetDb(db)
	for key, value := range map[string]any{config.GeoIpEnabledKey: false, config.LoginNotificationKey: false, config.NotificationEnabledKey: false, config.PrivateSiteKey: false} {
		if err := config.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	user, err := accounts.CreateAccount("synthetic-owner", "Synthetic-Password-Only")
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	if err := security.ConfigureTrustedProxies(engine); err != nil {
		t.Fatal(err)
	}
	engine.Use(security.NewCorsController(true, "").Middleware(), api.IdentityMiddleware(), api.PrivateSiteMiddleware())
	router.Register(engine)
	host := httptest.NewServer(engine)
	defer host.Close()
	call := func(method, path string, body any, cookies ...*http.Cookie) (*http.Response, []byte) {
		t.Helper()
		var source io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			source = bytes.NewReader(b)
		}
		req, err := http.NewRequest(method, host.URL+path, source)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", host.URL)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		resp, err := host.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		return resp, data
	}
	session := func() *http.Cookie {
		t.Helper()
		value, err := accounts.CreateSession(user.UUID, 3600, "synthetic", "127.0.0.1", "password")
		if err != nil {
			t.Fatal(err)
		}
		return &http.Cookie{Name: "session_token", Value: value}
	}
	callRPC := func(method string, params any, cookie *http.Cookie) rpc.JsonRpcResponse {
		t.Helper()
		var cookies []*http.Cookie
		if cookie != nil {
			cookies = append(cookies, cookie)
		}
		resp, data := call(http.MethodPost, "/api/rpc2", map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}, cookies...)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("RPC HTTP status %d: %s", resp.StatusCode, data)
		}
		var result rpc.JsonRpcResponse
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	dial := func(path string, cookie *http.Cookie, compression bool) *websocket.Conn {
		t.Helper()
		header := http.Header{"Origin": []string{host.URL}}
		if cookie != nil {
			header.Set("Cookie", cookie.String())
		}
		d := websocket.Dialer{EnableCompression: compression, HandshakeTimeout: 3 * time.Second}
		conn, _, err := d.Dial("ws"+strings.TrimPrefix(host.URL, "http")+path, header)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		return conn
	}
	t.Run("anonymous-and-agent-cannot-use-admin", func(t *testing.T) {
		if r := callRPC("admin:getSettings", nil, nil); r.Error == nil {
			t.Fatal("anonymous admin access")
		}
		id, token, err := clients.CreateClient()
		if err != nil {
			t.Fatal(err)
		}
		defer clients.DeleteClient(id)
		resp, data := call("POST", "/api/rpc2?token="+token, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "admin:getSettings"})
		var r rpc.JsonRpcResponse
		json.Unmarshal(data, &r)
		if resp.StatusCode != 200 || r.Error == nil {
			t.Fatal("agent admin access")
		}
		if r := callRPC("admin:getSettings", nil, session()); r.Error != nil {
			t.Fatalf("normal admin denied: %v", r.Error)
		}
	})
	t.Run("default-monitoring-profile-rejects-control-surfaces", func(t *testing.T) {
		for _, method := range []string{"admin:exec", "admin:dbExec", "admin:dbQuery", "admin:switchAgentVersion", "admin:fileList", "admin:setPluginEnabled"} {
			if r := callRPC(method, nil, session()); r.Error == nil || r.Error.Code != rpc.PermissionDenied {
				t.Errorf("%s was not blocked", method)
			}
		}
		for _, path := range []string{"/api/admin/plugin/list", "/api/admin/theme/market/catalog", "/api/admin/client/synthetic/terminal", "/api/admin/uploads/init"} {
			resp, _ := call("GET", path, nil, session())
			if resp.StatusCode != 403 {
				t.Errorf("%s status=%d", path, resp.StatusCode)
			}
		}
	})
	t.Run("factor-enrollment-cannot-replace-existing-factor", func(t *testing.T) {
		if err := accounts.Enable2Fa(user.UUID, "JBSWY3DPEHPK3PXP"); err != nil {
			t.Fatal(err)
		}
		defer accounts.Disable2Fa(user.UUID)
		current := session()
		resp, _ := call("POST", "/api/admin/2fa/generate", nil, current)
		if resp.StatusCode != 403 {
			t.Fatalf("generate status=%d", resp.StatusCode)
		}
		newSecret := "GEZDGNBVGY3TQOJQ"
		code, _ := totp.GenerateCode(newSecret, time.Now())
		resp, _ = call("POST", "/api/admin/2fa/enable?code="+code, nil, current, &http.Cookie{Name: "2fa_secret", Value: newSecret})
		got, _ := accounts.GetUserByUUID(user.UUID)
		if resp.StatusCode != 403 || got.TwoFactor != "JBSWY3DPEHPK3PXP" {
			t.Fatal("existing factor was replaced")
		}
		if err := accounts.Enable2Fa(user.UUID, newSecret); err == nil {
			t.Fatal("atomic database guard missing")
		}
	})
	t.Run("new-enrollment-works-with-protected-cookie", func(t *testing.T) {
		current := session()
		resp, _ := call("POST", "/api/admin/2fa/generate", nil, current)
		var pending *http.Cookie
		for _, c := range resp.Cookies() {
			if c.Name == "2fa_secret" {
				pending = c
			}
		}
		if resp.StatusCode != 200 || pending == nil || !pending.HttpOnly || pending.SameSite != http.SameSiteStrictMode || pending.MaxAge > 300 {
			t.Fatal("enrollment cookie is not protected")
		}
		code, _ := totp.GenerateCode(pending.Value, time.Now())
		resp, _ = call("POST", "/api/admin/2fa/enable?code="+code, nil, current, pending)
		if resp.StatusCode != 200 {
			t.Fatalf("new enrollment failed: %d", resp.StatusCode)
		}
		accounts.Disable2Fa(user.UUID)
	})
	t.Run("SQL-opt-in-still-requires-second-factor", func(t *testing.T) {
		t.Setenv("MAY_MONITOR_ALLOW_DATABASE_CONSOLE", "true")
		accounts.Enable2Fa(user.UUID, "JBSWY3DPEHPK3PXP")
		defer accounts.Disable2Fa(user.UUID)
		for _, method := range []string{"admin:dbExec", "admin:dbQuery"} {
			r := callRPC(method, map[string]any{"sql": "SELECT 1"}, session())
			if r.Error == nil || r.Error.Code != rpc.PermissionDenied {
				t.Errorf("missing OTP gate on %s", method)
			}
		}
	})
	t.Run("revoked-session-WebSocket-is-rejected", func(t *testing.T) {
		current := session()
		conn := dial("/api/rpc2", current, false)
		accounts.DeleteSession(current.Value)
		conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "admin:getSettings"})
		var result rpc.JsonRpcResponse
		if err := conn.ReadJSON(&result); err == nil && result.Error == nil {
			t.Fatal("revoked WebSocket retained authority")
		}
		resp, _ := call("GET", "/api/admin/client/list", nil, current)
		if resp.StatusCode != 401 {
			t.Fatal("revoked HTTP session accepted")
		}
	})
	t.Run("rotated-API-key-revokes-WebSocket", func(t *testing.T) {
		key := strings.Repeat("synthetic-key-", 3)
		config.Set(config.ApiKeyKey, key)
		defer config.Set(config.ApiKeyKey, "")
		header := http.Header{"Authorization": []string{"Bearer " + key}, "Origin": []string{host.URL}}
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(host.URL, "http")+"/api/rpc2", header)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		request := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "admin:getSettings"}
		conn.WriteJSON(request)
		var before rpc.JsonRpcResponse
		if err := conn.ReadJSON(&before); err != nil || before.Error != nil {
			t.Fatal("valid API key denied")
		}
		config.Set(config.ApiKeyKey, strings.Repeat("synthetic-new-", 3))
		conn.WriteJSON(request)
		var after rpc.JsonRpcResponse
		if err := conn.ReadJSON(&after); err == nil && after.Error == nil {
			t.Fatal("rotated API key kept WebSocket authority")
		}
	})
	t.Run("agent-fields-are-isolated-and-token-rotation-revokes-WebSocket", func(t *testing.T) {
		id, token, err := clients.CreateClient()
		if err != nil {
			t.Fatal(err)
		}
		defer clients.DeleteClient(id)
		if err := clients.SaveClient(map[string]interface{}{"uuid": id, "hidden": true, "remark": "controller-only", "name": "owner-name"}); err != nil {
			t.Fatal(err)
		}
		payload := v2.Request{JSONRPC: v2.Version, ID: 1, Method: v2.MethodAgentBasicInfo, Params: map[string]any{"info": map[string]any{"os": "synthetic-os", "hidden": false, "remark": "agent-value", "token": "agent-selected-token", "name": "agent-name", "uuid": "another-synthetic-client"}}}
		resp, _ := call("POST", "/api/clients/v2/rpc?token="+token, payload)
		if resp.StatusCode != 200 {
			t.Fatalf("ordinary inventory failed: %d", resp.StatusCode)
		}
		got, _ := clients.GetClientByUUID(id)
		if got.OS != "synthetic-os" || !got.Hidden || got.Remark != "controller-only" || got.Token != token || got.Name != "owner-name" {
			t.Fatal("agent changed controller fields or inventory failed")
		}
		conn := dial("/api/clients/v2/rpc?token="+token, nil, false)
		clients.SaveClient(map[string]interface{}{"uuid": id, "token": "synthetic-replacement-token"})
		conn.WriteJSON(payload)
		var result v2.Response
		if err := conn.ReadJSON(&result); err == nil && result.Error == nil {
			t.Fatal("rotated token remained authorized")
		}
		resp, _ = call("POST", "/api/clients/v2/rpc?token="+token, payload)
		if resp.StatusCode != 401 {
			t.Fatal("old token accepted by HTTP")
		}
	})
	t.Run("JSON-and-gzip-decoded-size-are-bounded", func(t *testing.T) {
		resp, _ := call("POST", "/api/rpc2", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "public:getVersion", "params": map[string]any{"padding": strings.Repeat("x", 8<<20)}})
		if resp.StatusCode != 413 {
			t.Fatalf("oversized HTTP status=%d", resp.StatusCode)
		}
		id, token, _ := clients.CreateClient()
		defer clients.DeleteClient(id)
		var compressed bytes.Buffer
		zw := gzip.NewWriter(&compressed)
		zw.Write(bytes.Repeat([]byte("x"), 8<<20))
		zw.Close()
		req, _ := http.NewRequest("POST", host.URL+"/api/clients/v2/rpc?token="+token, &compressed)
		req.Header.Set("Content-Encoding", "gzip")
		response, err := host.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 413 {
			t.Fatalf("oversized decoded gzip status=%d", response.StatusCode)
		}
	})
	t.Run("compressed-and-plain-WebSocket-messages-are-bounded", func(t *testing.T) {
		for _, compressed := range []bool{false, true} {
			conn := dial("/api/rpc2", nil, compressed)
			conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "public:getVersion", "params": map[string]any{"padding": strings.Repeat("x", 8<<20)}})
			var result rpc.JsonRpcResponse
			if err := conn.ReadJSON(&result); err == nil && result.Error == nil {
				t.Fatalf("oversized WS accepted; compressed=%v", compressed)
			}
			conn.Close()
		}
	})
	t.Run("private-share-expiry-is-rechecked-on-WebSocket", func(t *testing.T) {
		config.Set(config.PrivateSiteKey, true)
		defer config.Set(config.PrivateSiteKey, false)
		config.Set("tempory_share_token", "synthetic-share")
		config.Set("tempory_share_token_expire_at", time.Now().Add(time.Minute).Unix())
		conn := dial("/api/rpc2", &http.Cookie{Name: "temp_key", Value: "synthetic-share"}, false)
		config.Set("tempory_share_token_expire_at", time.Now().Add(-time.Minute).Unix())
		conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "public:getNodesInformation"})
		var result rpc.JsonRpcResponse
		if err := conn.ReadJSON(&result); err == nil && result.Error == nil {
			t.Fatal("expired private share accepted")
		}
	})
	t.Run("state-changing-GET-and-simple-cross-site-requests-cannot-mutate", func(t *testing.T) {
		t.Setenv("MAY_MONITOR_ALLOW_EXTENSIONS", "true")
		config.Set(config.ThemeKey, "synthetic-before")
		resp, _ := call("GET", "/api/admin/theme/set?theme=default", nil, session())
		got, _ := config.GetAs[string](config.ThemeKey, "")
		if resp.StatusCode == 200 || got != "synthetic-before" {
			t.Fatal("GET mutated theme")
		}
		config.Set(config.ThemeKey, "default")
		req, _ := http.NewRequest("POST", host.URL+"/api/admin/theme/set?theme=default", strings.NewReader(""))
		req.AddCookie(session())
		req.Header.Set("Content-Type", "text/plain")
		response, err := host.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 403 {
			t.Fatalf("simple cookie mutation status=%d", response.StatusCode)
		}
		resp, _ = call("POST", "/api/admin/theme/set?theme=default", nil, session())
		if resp.StatusCode != 200 {
			t.Fatal("same-origin POST failed")
		}
	})
	t.Run("salted-passwords-and-legacy-upgrade", func(t *testing.T) {
		other, err := accounts.CreateAccount("synthetic-second", "Synthetic-Password-Only")
		if err != nil {
			t.Fatal(err)
		}
		if other.Passwd == user.Passwd || !strings.HasPrefix(other.Passwd, "$may-bcrypt-sha256$") {
			t.Fatal("new passwords are not independently salted")
		}
		if _, ok := accounts.CheckPassword(other.Username, "wrong"); ok {
			t.Fatal("wrong password accepted")
		}
		sum := sha256.Sum256([]byte("Synthetic-Password-Only" + "06Wm4Jv1Hkxx"))
		legacy := base64.StdEncoding.EncodeToString(sum[:])
		db.Model(&models.User{}).Where("uuid = ?", other.UUID).Update("passwd", legacy)
		if _, ok := accounts.CheckPassword(other.Username, "Synthetic-Password-Only"); !ok {
			t.Fatal("legacy password failed")
		}
		stored, _ := accounts.GetUserByUUID(other.UUID)
		if !strings.HasPrefix(stored.Passwd, "$may-bcrypt-sha256$") {
			t.Fatal("legacy password was not upgraded")
		}
	})
	t.Run("login-works-and-throttles-bounded-failures", func(t *testing.T) {
		resp, _ := call("POST", "/api/login", map[string]any{"username": user.Username, "password": "Synthetic-Password-Only"})
		if resp.StatusCode != 200 {
			t.Fatal("normal login failed")
		}
		cookieOK := false
		for _, c := range resp.Cookies() {
			if c.Name == "session_token" && c.HttpOnly && c.SameSite == http.SameSiteLaxMode {
				cookieOK = true
			}
		}
		if !cookieOK {
			t.Fatal("missing protected session cookie")
		}
		limited := 0
		for i := 0; i < 40; i++ {
			r, _ := call("POST", "/api/login", map[string]any{"username": "synthetic-missing", "password": "wrong"})
			if r.StatusCode == 429 {
				limited++
			} else if r.StatusCode != 401 {
				t.Fatalf("unexpected login status=%d", r.StatusCode)
			}
		}
		if limited == 0 {
			t.Fatal("no login throttling")
		}
	})
}
