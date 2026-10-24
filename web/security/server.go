package security

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strings"
	"time"
)

func ConfigureTrustedProxies(r *gin.Engine) error {
	var proxies []string
	if value := strings.TrimSpace(os.Getenv("MAY_MONITOR_TRUSTED_PROXIES")); value != "" {
		for _, part := range strings.Split(value, ",") {
			proxies = append(proxies, strings.TrimSpace(part))
		}
	}
	return r.SetTrustedProxies(proxies)
}

func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	// Streaming transfers and long polling own their write timeouts.
	return &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 64 << 10}
}
