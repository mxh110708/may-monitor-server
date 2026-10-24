package api

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/internal/securitypolicy"
	"net/http"
)

func CapabilityBoundary() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		if !securitypolicy.HTTPAllowed(c.Request.Method, c.Request.URL.Path) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "capability disabled by May Monitor operator policy"})
			return
		}
		c.Next()
	}
}
