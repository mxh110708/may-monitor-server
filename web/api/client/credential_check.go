package client

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/utils"
)

// CheckCredential confirms the identity resolved by the normal agent token
// middleware without reporting synthetic metrics or changing inventory. The
// caller supplies its existing token in JSON, never in a probe URL.
func CheckCredential(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	uuid := c.GetString("client_uuid")
	if uuid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "agent credential required"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": 1, "uuid": uuid, "server_version": utils.CurrentVersion, "server_instance_id": utils.MonitorInstanceID()})
}
