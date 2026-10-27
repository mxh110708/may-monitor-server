package monitoring

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type WatchdogConfig struct {
	Delivery         string `json:"delivery,omitempty"`
	Endpoint         string `json:"endpoint"`
	ServerInstanceID string `json:"server_instance_id"`
	LocalReceiver    string `json:"local_receiver"`
	IntervalSeconds  int    `json:"interval_seconds"`
	Confirm          int    `json:"confirm"`
	Recover          int    `json:"recover"`
}
type WatchdogState struct {
	Schema           int         `json:"schema"`
	ServerInstanceID string      `json:"server_instance_id"`
	Armed            bool        `json:"armed"`
	CheckedAt        time.Time   `json:"checked_at"`
	Status           string      `json:"status"`
	Engine           StoredState `json:"engine"`
}

func ValidateWatchdog(cfg WatchdogConfig) error {
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.Path != "" && endpoint.Path != "/" {
		return errors.New("watchdog endpoint requires a bare monitoring origin")
	}
	if endpoint.Scheme != "https" {
		ip := net.ParseIP(endpoint.Hostname())
		if endpoint.Scheme != "http" || ip == nil || !ip.IsLoopback() {
			return errors.New("watchdog requires HTTPS (HTTP is only allowed on loopback)")
		}
	}
	id, err := uuid.Parse(cfg.ServerInstanceID)
	if err != nil || id == uuid.Nil || id.String() != cfg.ServerInstanceID {
		return errors.New("invalid expected server identity")
	}
	if cfg.IntervalSeconds < 10 || cfg.IntervalSeconds > 300 || cfg.Confirm < 2 || cfg.Confirm > 10 || cfg.Recover < 1 || cfg.Recover > 10 {
		return errors.New("watchdog cadence or confirmation count is out of bounds")
	}
	return Validate(Policy{Schema: 1, Delivery: cfg.Delivery, LocalReceiver: cfg.LocalReceiver}, time.Now().UTC())
}

func CheckWatchdog(ctx context.Context, cfg WatchdogConfig, token string, now time.Time) (string, bool) {
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 8 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(cfg.Endpoint, "/")+"/api/integration/v1/snapshot", nil)
	if err != nil {
		return "configuration_error", false
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Cache-Control", "no-store")
	response, err := client.Do(request)
	if err != nil {
		var unknown x509.UnknownAuthorityError
		var hostname x509.HostnameError
		if errors.As(err, &unknown) || errors.As(err, &hostname) {
			return "tls_error", false
		}
		return "unreachable", false
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return "credential_rejected", false
	}
	if response.StatusCode != 200 {
		return "http_error", false
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil || len(body) > 1<<20 {
		return "invalid_response", false
	}
	var snapshot struct {
		Schema   int       `json:"schema_version"`
		ID       string    `json:"server_instance_id"`
		Captured time.Time `json:"captured_at"`
	}
	if json.Unmarshal(body, &snapshot) != nil || snapshot.Schema != 1 || snapshot.Captured.IsZero() {
		return "invalid_response", false
	}
	if snapshot.ID != cfg.ServerInstanceID {
		return "identity_mismatch", false
	}
	if now.Sub(snapshot.Captured) > 90*time.Second || snapshot.Captured.Sub(now) > 5*time.Second {
		return "stale", false
	}
	return "healthy", true
}

// RunWatchdog runs in its own process, with its own narrowly scoped read token.
// Its state file contains event IDs and status only; credentials stay in env.
func RunWatchdog(ctx context.Context, cfg WatchdogConfig, token, statePath string) error {
	if err := ValidateWatchdog(cfg); err != nil {
		return err
	}
	rawToken, tokenError := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(token, "may_read_"))
	if !strings.HasPrefix(token, "may_read_") || len(token) != 52 || tokenError != nil || len(rawToken) != 32 || base64.RawURLEncoding.EncodeToString(rawToken) != strings.TrimPrefix(token, "may_read_") {
		return errors.New("watchdog read token is unavailable")
	}
	unlock, err := lockWatchdogState(statePath)
	if err != nil {
		return err
	}
	defer unlock()
	rule := Rule{ID: "server_liveness", Name: "May Monitor Server availability", Enabled: true, Kind: "offline", Nodes: []string{cfg.ServerInstanceID}, Confirm: cfg.Confirm, Recover: cfg.Recover, Window: 1, CooldownSeconds: 300}
	policy := Policy{Schema: 1, Enabled: true, Delivery: cfg.Delivery, LocalReceiver: cfg.LocalReceiver, Rules: []Rule{rule}}
	engine := NewEngine(policy)
	state := WatchdogState{Schema: 1, ServerInstanceID: cfg.ServerInstanceID}
	if info, err := os.Lstat(statePath); err == nil {
		if !info.Mode().IsRegular() || info.Size() > 2<<20 {
			return errors.New("invalid watchdog state file")
		}
		raw, err := os.ReadFile(statePath)
		if err != nil || json.Unmarshal(raw, &state) != nil || state.Schema != 1 || state.ServerInstanceID != cfg.ServerInstanceID {
			return errors.New("watchdog state identity does not match")
		}
		engine.Restore(state.Engine)
		engine.Configure(policy)
	} else if !os.IsNotExist(err) {
		return errors.New("watchdog state cannot be read")
	}
	ticker := time.NewTicker(time.Duration(cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop()
	for {
		now := time.Now().UTC()
		call, cancel := context.WithTimeout(ctx, 8*time.Second)
		status, healthy := CheckWatchdog(call, cfg, token, now)
		cancel()
		if ctx.Err() != nil {
			return nil
		}
		state.CheckedAt = now
		state.Status = status
		if healthy {
			state.Armed = true
		}
		sample := Observation{At: now, Valid: true, Value: 1, Violating: true}
		if healthy {
			sample.Value = 0
			sample.Violating = false
			sample.Recovering = true
		} else if !state.Armed || status == "credential_rejected" || status == "identity_mismatch" || status == "tls_error" || status == "configuration_error" {
			sample.Valid = false
			sample.Hint = status
		}
		engine.Step(rule, cfg.ServerInstanceID, sample, now)
		state.Engine = engine.Snapshot()
		if err := writeWatchdogState(statePath, state); err != nil {
			return err
		}
		if item := engine.Take(now); item != nil {
			state.Engine = engine.Snapshot()
			if err := writeWatchdogState(statePath, state); err != nil {
				return err
			}
			call, cancel := context.WithTimeout(ctx, 10*time.Second)
			code := SendNotification(call, policy, item.Event)
			cancel()
			engine.Complete(item.Event.ID, code, time.Now().UTC())
			state.Engine = engine.Snapshot()
			if err := writeWatchdogState(statePath, state); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func writeWatchdogState(path string, state WatchdogState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return errors.New("watchdog state encoding failed")
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".may-watchdog-*")
	if err != nil {
		return errors.New("watchdog state directory is unavailable")
	}
	name := temporary.Name()
	defer os.Remove(name)
	if temporary.Chmod(0600) != nil {
		temporary.Close()
		return errors.New("watchdog state permissions failed")
	}
	_, writeErr := temporary.Write(raw)
	syncErr := temporary.Sync()
	closeErr := temporary.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return errors.New("watchdog state write failed")
	}
	if os.Rename(name, path) != nil {
		return errors.New("watchdog state commit failed")
	}
	return nil
}
