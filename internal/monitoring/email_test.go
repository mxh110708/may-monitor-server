package monitoring

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/mail"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testEmailConfig() emailConfig {
	return emailConfig{Host: "smtp.example.test", Port: 465, Security: "tls", Username: "sender@example.test", Password: "fixture-not-a-real-password", From: "sender@example.test", To: []string{"recipient@example.test"}}
}

func smtpFixture(t *testing.T, startTLS, allowTLS bool, authCode string) (smtpTransport, <-chan []byte, <-chan bool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "smtp.example.test"}, DNSNames: []string{"smtp.example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	pair, err := tls.X509KeyPair(certPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(certPEM)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	messages := make(chan []byte, 1)
	authenticated := make(chan bool, 1)
	done := make(chan struct{})
	t.Cleanup(func() {
		listener.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("SMTP fixture did not close")
		}
	})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		secure := !startTLS
		if secure {
			conn = tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12})
		}
		reader := textproto.NewReader(bufio.NewReader(conn))
		writer := textproto.NewWriter(bufio.NewWriter(conn))
		if writer.PrintfLine("220 fixture ESMTP") != nil {
			return
		}
		for {
			line, err := reader.ReadLine()
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				if !secure && allowTLS {
					writer.PrintfLine("250-fixture\r\n250-STARTTLS\r\n250 AUTH PLAIN")
				} else {
					writer.PrintfLine("250-fixture\r\n250 AUTH PLAIN")
				}
			case line == "STARTTLS":
				writer.PrintfLine("220 Ready")
				conn = tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12})
				secure = true
				reader = textproto.NewReader(bufio.NewReader(conn))
				writer = textproto.NewWriter(bufio.NewWriter(conn))
			case strings.HasPrefix(line, "AUTH PLAIN "):
				authenticated <- secure
				writer.PrintfLine("%s", authCode)
			case strings.HasPrefix(line, "MAIL FROM:") || strings.HasPrefix(line, "RCPT TO:"):
				writer.PrintfLine("250 OK")
			case line == "DATA":
				writer.PrintfLine("354 Send data")
				raw, err := io.ReadAll(reader.DotReader())
				if err != nil {
					return
				}
				messages <- raw
				writer.PrintfLine("250 Queued")
			case line == "QUIT":
				return
			default:
				writer.PrintfLine("500 Unsupported")
			}
		}
	}()
	transport := smtpTransport{roots: roots, lookup: func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}, dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "8.8.8.8:465" && address != "8.8.8.8:587" {
			t.Error("SMTP destination was not pinned")
		}
		return (&net.Dialer{}).DialContext(ctx, network, listener.Addr().String())
	}}
	return transport, messages, authenticated
}

func TestEmailEncryptedDeliveryAndRecovery(t *testing.T) {
	for _, start := range []bool{false, true} {
		t.Run(map[bool]string{false: "TLS", true: "STARTTLS"}[start], func(t *testing.T) {
			transport, messages, authenticated := smtpFixture(t, start, true, "235 Authenticated")
			cfg := testEmailConfig()
			if start {
				cfg.Security = "starttls"
				cfg.Port = 587
			}
			event := Event{ID: "fixture-event-1", Kind: "firing", RuleName: "节点掉线", At: time.Now().UTC()}
			if start {
				event.Kind = "recovered"
			}
			if got := sendEmail(context.Background(), cfg, event, transport); got != "sent" {
				t.Fatalf("delivery: %s", got)
			}
			if !<-authenticated {
				t.Fatal("credentials were sent before TLS")
			}
			message, err := mail.ReadMessage(strings.NewReader(string(<-messages)))
			if err != nil {
				t.Fatal(err)
			}
			if message.Header.Get("Message-ID") != "<fixture-event-1@may-monitor.invalid>" || message.Header.Get("X-May-Event-ID") != event.ID {
				t.Fatal("retry identity is missing")
			}
			body, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, message.Body))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), event.Kind) || !strings.Contains(string(body), event.RuleName) || strings.Contains(string(body), cfg.Password) {
				t.Fatal("incorrect event content")
			}
		})
	}
}

func TestEmailRejectsUnsafeOrIncompleteConfiguration(t *testing.T) {
	for _, change := range []func(*emailConfig){func(c *emailConfig) { c.Port = 25 }, func(c *emailConfig) { c.Security = "plain" }, func(c *emailConfig) { c.Host = "127.0.0.1" }, func(c *emailConfig) { c.To = []string{"recipient@example.test\r\nBcc: elsewhere@example.test"} }, func(c *emailConfig) { c.Password = "" }, func(c *emailConfig) { c.From = "Name <sender@example.test>" }} {
		cfg := testEmailConfig()
		change(&cfg)
		if cfg.validate() == nil {
			t.Fatal("unsafe SMTP configuration accepted")
		}
	}
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "169.254.169.254", "100.64.0.1", "192.0.2.1", "::1", "fc00::1", "::ffff:192.168.1.1"} {
		if publicSMTPAddress(net.ParseIP(ip)) {
			t.Fatal("nonpublic SMTP address accepted")
		}
	}
	if !publicSMTPAddress(net.ParseIP("8.8.8.8")) || !publicSMTPAddress(net.ParseIP("2606:4700:4700::1111")) {
		t.Fatal("public address rejected")
	}
	transport := smtpTransport{lookup: func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("127.0.0.1")}}, nil
	}, dial: func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("mixed private DNS must not dial")
		return nil, nil
	}}
	if got := sendEmail(context.Background(), testEmailConfig(), Event{ID: "test", Kind: "firing", At: time.Now()}, transport); got != "configuration_error" {
		t.Fatal(got)
	}
}

func TestEmailRefusesDowngradeBadCertificatesAndAuthentication(t *testing.T) {
	for _, scenario := range []string{"no_starttls", "untrusted_certificate", "wrong_hostname", "rejected_credentials"} {
		t.Run(scenario, func(t *testing.T) {
			start := scenario == "no_starttls"
			code := "235 Authenticated"
			if scenario == "rejected_credentials" {
				code = "535 Rejected"
			}
			transport, _, authenticated := smtpFixture(t, start, false, code)
			cfg := testEmailConfig()
			want := "tls_error"
			if start {
				cfg.Security = "starttls"
				cfg.Port = 587
			}
			if scenario == "untrusted_certificate" {
				transport.roots = x509.NewCertPool()
			}
			if scenario == "wrong_hostname" {
				cfg.Host = "different.example.test"
			}
			if scenario == "rejected_credentials" {
				want = "credential_rejected"
			}
			if got := sendEmail(context.Background(), cfg, Event{ID: "test", Kind: "firing", At: time.Now()}, transport); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
			if scenario != "rejected_credentials" {
				select {
				case <-authenticated:
					t.Fatal("authentication before verified TLS")
				default:
				}
			}
		})
	}
}

func TestEmailPrivateFileAndDisabledDelivery(t *testing.T) {
	file := filepath.Join(t.TempDir(), "smtp.json")
	t.Setenv("MAY_MONITOR_SMTP_CONFIG", file)
	if EmailStatus() != "unconfigured" || SendNotification(context.Background(), Policy{Delivery: "email"}, Event{}) != "disabled" {
		t.Fatal("missing configuration must not send")
	}
	raw, _ := json.Marshal(testEmailConfig())
	if os.WriteFile(file, raw, 0600) != nil {
		t.Fatal("fixture write")
	}
	if EmailStatus() != "configured" {
		t.Fatal("private file rejected")
	}
	if got := SendNotification(context.Background(), Policy{Delivery: "none"}, Event{}); got != "disabled" {
		t.Fatal(got)
	}
	if err := os.Chmod(file, 0644); err != nil {
		t.Fatal(err)
	}
	if EmailStatus() != "invalid" {
		t.Fatal("readable-by-others credential file accepted")
	}
	os.Chmod(file, 0600)
	os.WriteFile(file, append(raw, []byte("{}")...), 0600)
	if EmailStatus() != "invalid" {
		t.Fatal("trailing configuration accepted")
	}
	if Validate(Policy{Schema: 1, Delivery: "email"}, time.Now()) != nil || ValidateWatchdog(WatchdogConfig{Endpoint: "https://monitor.example.test", ServerInstanceID: "00000000-0000-4000-8000-000000000011", Delivery: "email", IntervalSeconds: 30, Confirm: 3, Recover: 2}) != nil {
		t.Fatal("email unavailable to controller or watchdog")
	}
}
