package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/web/oauth/factory"
	"github.com/patrickmn/go-cache"
)

func (g *Github) GetName() string {
	return "github"
}
func (g *Github) GetConfiguration() factory.Configuration {
	return &g.Addition
}

func (g *Github) GetAuthorizationURL(_ string) (string, string) {
	state := utils.GenerateRandomString(16)

	// 构建GitHub OAuth授权URL
	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&state=%s&scope=user:email",
		url.QueryEscape(g.Addition.ClientId),
		url.QueryEscape(state),
	)
	if !factory.StoreState(g.stateCache, state) {
		return "", ""
	}
	return authURL, state
}
func (g *Github) OnCallback(ctx *gin.Context, state string, query map[string]string, _ string) (factory.OidcCallback, error) {
	code := query["code"]

	// 验证state防止CSRF攻击
	if g.stateCache == nil {
		return factory.OidcCallback{}, fmt.Errorf("state cache not initialized")
	}
	if !factory.ConsumeState(g.stateCache, state) {
		return factory.OidcCallback{}, fmt.Errorf("invalid state")
	}

	if code == "" {
		return factory.OidcCallback{}, fmt.Errorf("no code provided")
	}

	// 获取访问令牌
	tokenURL := "https://github.com/login/oauth/access_token"
	data := url.Values{
		"client_id":     {g.Addition.ClientId},
		"client_secret": {g.Addition.ClientSecret},
		"code":          {code},
	}

	req, _ := http.NewRequestWithContext(ctx.Request.Context(), "POST", tokenURL, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return factory.OidcCallback{}, fmt.Errorf("failed to get access token: %s", utils.DataMasking(err.Error(), []string{g.Addition.ClientSecret, g.Addition.ClientId}))
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
	}

	if resp.StatusCode != http.StatusOK {
		return factory.OidcCallback{}, fmt.Errorf("token endpoint rejected request")
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tokenResp); err != nil {
		return factory.OidcCallback{}, fmt.Errorf("failed to parse access token response: %s", utils.DataMasking(err.Error(), []string{g.Addition.ClientSecret, g.Addition.ClientId}))
	}
	if tokenResp.AccessToken == "" {
		return factory.OidcCallback{}, fmt.Errorf("missing OAuth access token")
	}

	// 获取用户信息
	userReq, _ := http.NewRequestWithContext(ctx.Request.Context(), "GET", "https://api.github.com/user", nil)
	userReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	userReq.Header.Set("Accept", "application/json")

	userResp, err := (&http.Client{Timeout: 20 * time.Second}).Do(userReq)
	if err != nil {
		return factory.OidcCallback{}, fmt.Errorf("failed to get user info: %v", err)
	}
	defer userResp.Body.Close()

	var githubUser GitHubUser
	if userResp.StatusCode != http.StatusOK {
		return factory.OidcCallback{}, fmt.Errorf("user endpoint rejected request")
	}
	if err := json.NewDecoder(io.LimitReader(userResp.Body, 1<<20)).Decode(&githubUser); err != nil {
		return factory.OidcCallback{}, fmt.Errorf("failed to parse user info response: %v", err)
	}
	if githubUser.ID <= 0 {
		return factory.OidcCallback{}, fmt.Errorf("invalid OAuth subject")
	}

	return factory.OidcCallback{UserId: fmt.Sprintf("%d", githubUser.ID)}, nil
}
func (g *Github) Init() error {
	g.stateCache = cache.New(time.Minute*5, time.Minute*10)
	return nil
}
func (g *Github) Destroy() error {
	g.stateCache.Flush()
	return nil
}

var _ factory.IOidcProvider = (*Github)(nil)
