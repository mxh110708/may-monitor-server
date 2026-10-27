package monitoring

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"net/netip"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SMTP credentials are operator-owned private data, never policy/RPC settings.
// The service working directory is already included in scoped Deploy backups.
type emailConfig struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Security string   `json:"security"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

var smtpHost = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)
var mailEventID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

func validMailbox(value string) bool {
	if len(value) > 254 || strings.ContainsAny(value, "\r\n\x00") {
		return false
	}
	for _, r := range value {
		if r < 33 || r > 126 {
			return false
		}
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Name == "" && address.Address == value && strings.Contains(value, "@")
}

func (cfg emailConfig) validate() error {
	if len(cfg.Host) > 253 || !smtpHost.MatchString(cfg.Host) || net.ParseIP(cfg.Host) != nil ||
		!(cfg.Security == "tls" && cfg.Port == 465 || cfg.Security == "starttls" && cfg.Port == 587) ||
		cfg.Username == "" || len(cfg.Username) > 320 || cfg.Password == "" || len(cfg.Password) > 4096 ||
		strings.ContainsAny(cfg.Username+cfg.Password, "\r\n\x00") || !validMailbox(cfg.From) || len(cfg.To) < 1 || len(cfg.To) > 5 {
		return errors.New("invalid private SMTP configuration")
	}
	seen := map[string]bool{}
	for _, recipient := range cfg.To {
		if !validMailbox(recipient) || seen[recipient] {
			return errors.New("invalid private SMTP recipient")
		}
		seen[recipient] = true
	}
	return nil
}

func loadEmailConfig() (emailConfig, string) {
	var cfg emailConfig
	path := os.Getenv("MAY_MONITOR_SMTP_CONFIG")
	if path == "" {
		path = "notification-smtp.json"
	} else if !filepath.IsAbs(path) {
		return cfg, "invalid"
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return cfg, "unconfigured"
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 16384 {
		return cfg, "invalid"
	}
	file, err := os.Open(path)
	if err != nil {
		return cfg, "invalid"
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm()&0077 != 0 {
		return cfg, "invalid"
	}
	raw, err := io.ReadAll(io.LimitReader(file, 16385))
	if err != nil || len(raw) > 16384 {
		return cfg, "invalid"
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&cfg) != nil || decoder.Decode(new(any)) != io.EOF || cfg.validate() != nil {
		return emailConfig{}, "invalid"
	}
	return cfg, "configured"
}

// EmailStatus describes configuration only; it does not connect or send mail.
func EmailStatus() string {
	_, status := loadEmailConfig()
	return status
}

func SendNotification(ctx context.Context, policy Policy, event Event) string {
	switch policy.Delivery {
	case "", "local":
		return SendLocalReceiver(ctx, policy.LocalReceiver, event)
	case "none":
		return "disabled"
	case "email":
		cfg, status := loadEmailConfig()
		if status == "unconfigured" {
			return "disabled"
		}
		if status != "configured" {
			return "configuration_error"
		}
		return sendEmail(ctx, cfg, event, smtpTransport{lookup: net.DefaultResolver.LookupIPAddr, dial: (&net.Dialer{Timeout: 5 * time.Second}).DialContext})
	default:
		return "configuration_error"
	}
}

// A private transport fixture can substitute a local TLS SMTP server in tests.
// Production always resolves and pins public destination addresses, uses system
// roots, and cannot opt out of hostname/certificate validation.
type smtpTransport struct {
	lookup func(context.Context, string) ([]net.IPAddr, error)
	dial   func(context.Context, string, string) (net.Conn, error)
	roots  *x509.CertPool
}

func publicSMTPAddress(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	for _, value := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		if netip.MustParsePrefix(value).Contains(address) {
			return false
		}
	}
	return true
}

func emailMessage(cfg emailConfig, event Event) ([]byte, error) {
	if !mailEventID.MatchString(event.ID) || (event.Kind != "firing" && event.Kind != "recovered") || event.At.IsZero() || len(event.RuleName) > 128 || strings.ContainsAny(event.RuleName, "\r\n\x00") {
		return nil, errors.New("invalid email event")
	}
	raw, err := json.MarshalIndent(event, "", "  ")
	if err != nil || len(raw) > 65536 {
		return nil, errors.New("email event exceeds limit")
	}
	body := base64.StdEncoding.EncodeToString(append([]byte("May Monitor\n\n"), raw...))
	var wrapped strings.Builder
	for len(body) > 0 {
		n := min(76, len(body))
		wrapped.WriteString(body[:n] + "\r\n")
		body = body[n:]
	}
	subject := mime.QEncoding.Encode("utf-8", "[May Monitor] "+event.Kind+" - "+event.RuleName)
	return []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nDate: %s\r\nMessage-ID: <%s@may-monitor.invalid>\r\nX-May-Event-ID: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s", cfg.From, strings.Join(cfg.To, ", "), event.At.Format(time.RFC1123Z), event.ID, event.ID, subject, wrapped.String())), nil
}

func sendEmail(ctx context.Context, cfg emailConfig, event Event, transport smtpTransport) string {
	if cfg.validate() != nil {
		return "configuration_error"
	}
	message, err := emailMessage(cfg, event)
	if err != nil {
		return "send_failed"
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	failure := func(code string) string {
		if ctx.Err() != nil {
			return "timeout"
		}
		return code
	}
	addresses, err := transport.lookup(ctx, cfg.Host)
	if err != nil || len(addresses) == 0 || len(addresses) > 32 {
		return failure("network_error")
	}
	for _, address := range addresses {
		if address.Zone != "" || !publicSMTPAddress(address.IP) {
			return "configuration_error"
		}
	}
	var conn net.Conn
	for _, address := range addresses {
		conn, err = transport.dial(ctx, "tcp", net.JoinHostPort(address.IP.String(), strconv.Itoa(cfg.Port)))
		if err == nil {
			break
		}
	}
	if err != nil {
		if ctx.Err() != nil {
			return "timeout"
		}
		return "network_error"
	}
	defer conn.Close()
	socket := conn
	stop := context.AfterFunc(ctx, func() { socket.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if conn.SetDeadline(deadline) != nil {
		return "network_error"
	}
	tlsConfig := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12, RootCAs: transport.roots}
	if cfg.Security == "tls" {
		secure := tls.Client(conn, tlsConfig)
		if secure.HandshakeContext(ctx) != nil {
			return "tls_error"
		}
		conn = secure
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return failure("network_error")
	}
	defer client.Close()
	if cfg.Security == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return "tls_error"
		}
		if client.StartTLS(tlsConfig) != nil {
			return failure("tls_error")
		}
	}
	if client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)) != nil {
		return failure("credential_rejected")
	}
	if client.Mail(cfg.From) != nil {
		return failure("send_failed")
	}
	for _, recipient := range cfg.To {
		if client.Rcpt(recipient) != nil {
			return failure("send_failed")
		}
	}
	writer, err := client.Data()
	if err != nil {
		return failure("send_failed")
	}
	_, writeErr := writer.Write(message)
	closeErr := writer.Close()
	if writeErr != nil || closeErr != nil {
		if ctx.Err() != nil {
			return "timeout"
		}
		return "send_failed"
	}
	// Acceptance of DATA is the receipt; a failed QUIT must not resend accepted mail.
	return "sent"
}
