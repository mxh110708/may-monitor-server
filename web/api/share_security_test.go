package api

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPublicStreamShareAuthorizationIsRefreshed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)
	config.Set(config.PrivateSiteKey, true)
	config.Set("tempory_share_token", "synthetic-share")
	config.Set("tempory_share_token_expire_at", time.Now().Add(time.Minute).Unix())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/clients", nil)
	c.Request.AddCookie(&http.Cookie{Name: "temp_key", Value: "synthetic-share"})
	SetPrincipal(c, rpc.NewAnonymousPrincipal())
	check := principalValidator(c)
	if check == nil || !check() {
		t.Fatal("valid share denied")
	}
	config.Set("tempory_share_token_expire_at", time.Now().Add(-time.Minute).Unix())
	if check() {
		t.Fatal("expired share still authorized")
	}
	config.Set(config.PrivateSiteKey, false)
	if !check() {
		t.Fatal("public stream denied after public mode selected")
	}
}
