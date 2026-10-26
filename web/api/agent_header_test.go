package api

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestAgentCredentialHeaderIsSeparateFromAdministratorBearer(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/clients/v2/rpc", nil)
	c.Request.Header.Set("X-May-Agent-Token", "synthetic-agent-only")
	if extractClientToken(c) != "synthetic-agent-only" || c.Request.URL.RawQuery != "" || c.GetHeader("Authorization") != "" {
		t.Fatal("agent credential was not kept in its separate header")
	}
	c.Request.Header.Del("X-May-Agent-Token")
	c.Request.Header.Set("Authorization", "Bearer synthetic-admin-key")
	if extractClientToken(c) != "" {
		t.Fatal("administrator bearer was confused with agent authentication")
	}
}
