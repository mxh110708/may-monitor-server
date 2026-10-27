package monitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Local HTTP delivery is restricted to loopback. Enforce a literal loopback IP
// twice: configuration validation and the actual connection; no DNS or proxy.
func ValidateLocalReceiver(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || len(value) > 2048 || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("local receiver requires an HTTP(S) loopback URL without credentials or query")
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return errors.New("M6 receiver is restricted to literal loopback addresses")
	}
	if parsed.Port() != "" {
		port, err := strconv.Atoi(parsed.Port())
		if err != nil || port < 1 || port > 65535 {
			return errors.New("invalid local receiver port")
		}
	}
	return nil
}

func SendLocalReceiver(ctx context.Context, endpoint string, event Event) string {
	if endpoint == "" {
		return "disabled"
	}
	if ValidateLocalReceiver(endpoint) != nil {
		return "send_failed"
	}
	raw, err := json.Marshal(event)
	if err != nil || len(raw) > 65536 {
		return "send_failed"
	}
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return nil, errors.New("non-loopback receiver connection refused")
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "send_failed"
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-May-Event-ID", event.ID)
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "timeout"
		}
		return "network_error"
	}
	response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "send_failed"
	}
	return "sent"
}
