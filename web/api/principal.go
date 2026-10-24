package api

// principal.go
// 统一身份识别。IdentifyPrincipal 是全局唯一的主体识别入口,替代此前散落在
// IdentityMiddleware / transport.detectPermissionGroup / transport.buildContextMeta
// 三处的重复逻辑。识别优先级:API Key > Session(用户) > Client Token(agent) > 匿名。

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"time"
)

// principalContextKey 是 principal 在 gin.Context 中的存储键。
const principalContextKey = "principal"

// IdentifyPrincipal 识别当前请求的调用主体。不写入任何状态,可安全多次调用。
// 识别优先级与历史 IdentityMiddleware 一致:API Key > Session > Client Token > 匿名。
func IdentifyPrincipal(c *gin.Context) *rpc.Principal {
	// 1. API Key(Authorization: Bearer <key>)
	if isApiKeyValid(c.GetHeader("Authorization")) {
		return rpc.NewAPIKeyPrincipal()
	}

	// 2. Session(管理员用户)
	if session, err := c.Cookie("session_token"); err == nil && session != "" {
		if uuid, err := accounts.GetSession(session); err == nil && uuid != "" {
			if _, err := accounts.GetUserByUUID(uuid); err == nil {
				return rpc.NewUserPrincipal(uuid)
			}
		}
	}

	// 3. Client Token(agent 客户端)
	if token := extractClientToken(c); token != "" {
		if uuid, err := clients.GetClientUUIDByToken(token); err == nil {
			return rpc.NewAgentPrincipal(uuid)
		}
	}

	// 4. 匿名访客
	return rpc.NewAnonymousPrincipal()
}

// Capture immutable handshake credentials, never the cached handshake role.
// Revalidate on each incoming/outgoing frame, including queued control events.
func principalValidator(c *gin.Context) func() bool {
	p := GetPrincipal(c)
	if p == nil {
		p = IdentifyPrincipal(c)
	}
	switch p.Type {
	case rpc.PrincipalUser:
		session, _ := c.Cookie("session_token")
		return func() bool {
			id, err := accounts.GetSession(session)
			if err != nil || id != p.UserUUID {
				return false
			}
			_, err = accounts.GetUserByUUID(id)
			return err == nil
		}
	case rpc.PrincipalAPIKey:
		auth := c.GetHeader("Authorization")
		return func() bool { return isApiKeyValid(auth) }
	case rpc.PrincipalAgent:
		token := extractClientToken(c)
		return func() bool { id, err := clients.GetClientUUIDByToken(token); return err == nil && id == p.ClientUUID }
	}
	// RPC checks its public login whitelist/share on each call. Other public
	// streams must also stop disclosing data if private mode/share state changes.
	if c.Request.URL.Path != "/api/rpc2" {
		key, _ := c.Cookie("temp_key")
		return func() bool {
			private, _ := config.GetAs[bool](config.PrivateSiteKey, false)
			if !private {
				return true
			}
			allowed, _ := config.GetAs[string]("tempory_share_token", "")
			expires, _ := config.GetAs[int64]("tempory_share_token_expire_at", 0)
			return key != "" && key == allowed && expires >= time.Now().Unix()
		}
	}
	return nil
}

// SetPrincipal 将已识别的主体写入 gin.Context,供下游(RPC 边界等)复用。
func SetPrincipal(c *gin.Context, p *rpc.Principal) {
	c.Set(principalContextKey, p)
}

// GetPrincipal 读取已识别的主体;若中间件未运行则返回 nil。
func GetPrincipal(c *gin.Context) *rpc.Principal {
	p, _ := c.Get(principalContextKey)
	principal, _ := p.(*rpc.Principal)
	return principal
}
