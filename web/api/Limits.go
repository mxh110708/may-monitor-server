package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/web/security"
)

const MaxJSONBody int64 = 4 << 20

func ReadLimitedBody(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxJSONBody+1))
	if err == nil && int64(len(b)) > MaxJSONBody {
		return nil, &http.MaxBytesError{Limit: MaxJSONBody}
	}
	return b, err
}

func boundRequest(c *gin.Context) bool {
	p := c.Request.URL.Path
	// Streaming file/chunk routes have their own size and ownership checks.
	stream := strings.HasPrefix(p, "/api/admin/uploads/") || strings.HasPrefix(p, "/api/clients/transfer/") || strings.HasSuffix(p, "/file/upload")
	if c.Request.Body != nil && !stream {
		if c.Request.ContentLength > MaxJSONBody {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return false
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxJSONBody)
	}
	return true
}

// CORS alone does not prevent a browser from sending a simple cross-site
// state-changing request. This guard is independent of the CORS enable switch.
func cookieMutationAllowed(c *gin.Context) bool {
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	origin := c.GetHeader("Origin")
	if origin != "" {
		allowed, _ := config.GetAs[string](config.CorsAllowedOriginsKey, "")
		return security.OriginMatchesHost(origin, c.Request.Host) || security.OriginInAllowlist(origin, allowed)
	}
	return c.GetHeader("Sec-Fetch-Site") == "same-origin" || c.GetHeader("X-Requested-With") == "XMLHttpRequest" || strings.EqualFold(strings.TrimSpace(strings.Split(c.GetHeader("Content-Type"), ";")[0]), "application/json")
}
