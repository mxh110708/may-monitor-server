package security

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginLimiterExpiresWithoutUnboundedHistory(t *testing.T) {
	var limiter LoginLimiter
	now := time.Now()
	for i := 0; i < 10; i++ {
		if !limiter.Allow("synthetic-ip", now) {
			t.Fatal("early throttle")
		}
	}
	if limiter.Allow("synthetic-ip", now) {
		t.Fatal("unbounded login attempts")
	}
	if !limiter.Allow("synthetic-ip", now.Add(61*time.Second)) {
		t.Fatal("expired throttle not cleared")
	}
	for i := 0; i < 5000; i++ {
		limiter.Allow(time.Unix(int64(i), 0).String(), now.Add(61*time.Second))
	}
	if len(limiter.windows) > 4096 {
		t.Fatal("unbounded address history")
	}
}

func TestProxyHeadersRequireExplicitTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("MAY_MONITOR_TRUSTED_PROXIES", "")
	r := gin.New()
	if err := ConfigureTrustedProxies(r); err != nil {
		t.Fatal(err)
	}
	r.GET("/", func(c *gin.Context) { c.String(200, c.ClientIP()) })
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "192.0.2.1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Body.String() != "127.0.0.1" {
		t.Fatal("untrusted proxy header changed identity")
	}
}
