// Package migration serves the authenticated Metric Store structure upgrade guide.
package migration

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	appconfig "github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/internal/metricstore"
	"github.com/komari-monitor/komari/web/api"
	publicapi "github.com/komari-monitor/komari/web/api/public"
	jsonrpc "github.com/komari-monitor/komari/web/rpc/jsonrpc"
)

const (
	PagePath = "/admin/database-migration"
	APIPath  = "/api/admin/database-migration"
)

const structureProgressCeiling = 80.0

type Mode string

const (
	ModeMetricStructure Mode = "metric_store_restructure"
)

// Status is the polling payload for a metric-store structure upgrade.
type Status struct {
	Mode     Mode    `json:"mode"`
	State    string  `json:"state"`
	Phase    string  `json:"phase"`
	Progress float64 `json:"progress"`
	Error    string  `json:"error,omitempty"`

	CurrentMetric string  `json:"current_metric,omitempty"`
	RowsDone      int64   `json:"rows_done,omitempty"`
	RowsTotal     int64   `json:"rows_total,omitempty"`
	MetricsDone   int     `json:"metrics_done,omitempty"`
	MetricsTotal  int     `json:"metrics_total,omitempty"`
	BeforeBytes   int64   `json:"before_bytes,omitempty"`
	AfterBytes    int64   `json:"after_bytes,omitempty"`
	SavedBytes    int64   `json:"saved_bytes,omitempty"`
	SavedPercent  float64 `json:"saved_percent,omitempty"`
}

type Controller struct {
	mode Mode

	active atomic.Bool
	mu     sync.RWMutex
	status Status
	done   chan struct{}
	once   sync.Once
}

func NewStructureController() *Controller {
	return &Controller{
		mode:   ModeMetricStructure,
		status: Status{Mode: ModeMetricStructure, State: "ready", Phase: "ready"},
		done:   make(chan struct{}),
	}
}

func (c *Controller) Activate() { c.active.Store(true) }

func (c *Controller) Deactivate() { c.active.Store(false) }

func (c *Controller) Done() <-chan struct{} { return c.done }

func (c *Controller) Register(r *gin.Engine) {
	r.POST("/api/login", publicapi.Login)
	r.GET("/api/me", jsonrpc.Bind("public:getMe", jsonrpc.WithRaw()))
	r.GET("/api/oauth", publicapi.OAuth)
	r.GET("/api/oauth_callback", publicapi.OAuthCallback)

	g := r.Group(APIPath, c.requireActive)
	g.GET("/auth", c.authStatus)
	authorized := g.Group("", api.RequireRole(api.RoleAdmin))
	authorized.GET("/status", c.getStatus)
	authorized.POST("/start", c.start)
	authorized.POST("/discard", c.discard)
}

func (c *Controller) requireActive(ctx *gin.Context) {
	if !c.active.Load() {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	ctx.Next()
}

func (c *Controller) authStatus(ctx *gin.Context) {
	oauthEnabled, _ := appconfig.GetAs[bool](appconfig.OAuthEnabledKey, false)
	oauthProvider, _ := appconfig.GetAs[string](appconfig.OAuthProviderKey, "github")
	disablePassword, _ := appconfig.GetAs[bool](appconfig.DisablePasswordLoginKey, false)
	api.RespondSuccess(ctx, gin.H{
		"oauth_enabled":          oauthEnabled,
		"oauth_provider":         oauthProvider,
		"password_login_enabled": !disablePassword,
		"mode":                   c.mode,
	})
}

func (c *Controller) getStatus(ctx *gin.Context) {
	c.mu.RLock()
	status := c.status
	c.mu.RUnlock()
	api.RespondSuccess(ctx, status)
}

func (c *Controller) start(ctx *gin.Context) {
	c.startStructure(ctx)
}

func (c *Controller) discard(ctx *gin.Context) {
	c.mu.Lock()
	if c.operationActiveLocked() || c.status.State == "completed" {
		c.mu.Unlock()
		api.RespondError(ctx, http.StatusConflict, "database migration is already running or completed")
		return
	}
	c.status = Status{Mode: c.mode, State: "discarding", Phase: "discarding"}
	c.mu.Unlock()

	go c.runDiscard()
	api.RespondSuccessMessage(ctx, "historical metric data deletion started", gin.H{})
}

func (c *Controller) startStructure(ctx *gin.Context) {
	c.mu.Lock()
	if c.operationActiveLocked() || c.status.State == "completed" {
		c.mu.Unlock()
		api.RespondError(ctx, http.StatusConflict, "database migration is already running or completed")
		return
	}
	c.status = Status{Mode: c.mode, State: "copying", Phase: "preparing"}
	c.mu.Unlock()

	go c.runStructure()
	api.RespondSuccessMessage(ctx, "database migration started", gin.H{})
}

func (c *Controller) operationActiveLocked() bool {
	switch c.status.State {
	case "cleaning", "discarding", "migrating", "copying", "reclaiming":
		return true
	default:
		return false
	}
}

func (c *Controller) runStructure() {
	result, err := metricstore.RestructureConfiguredStore(context.Background(), func(progress metricstore.RestructureProgress) {
		c.mu.Lock()
		c.status.Phase = progress.Phase
		c.status.CurrentMetric = progress.CurrentMetric
		c.status.RowsDone = progress.RowsDone
		c.status.RowsTotal = progress.RowsTotal
		c.status.MetricsDone = progress.MetricsDone
		c.status.MetricsTotal = progress.MetricsTotal
		c.status.Progress = structureProgressPercent(progress)
		if progress.Phase == "reclaiming" {
			c.status.State = "reclaiming"
		} else if progress.Phase == "discarding" {
			c.status.State = "discarding"
		} else if c.status.State == "reclaiming" || c.status.State == "discarding" {
			c.status.State = "copying"
		}
		c.mu.Unlock()
	})
	if err != nil {
		c.fail(metricstore.RedactConnectionError(err.Error(), ""), "failed")
		return
	}

	saved := result.BeforeBytes - result.AfterBytes
	if saved < 0 {
		saved = 0
	}
	percent := 0.0
	if result.BeforeBytes > 0 {
		percent = float64(saved) / float64(result.BeforeBytes) * 100
	}
	c.mu.Lock()
	c.status = Status{
		Mode: c.mode, State: "completed", Phase: "completed", Progress: 100,
		RowsDone: result.RowsCopied, RowsTotal: result.RowsCopied,
		MetricsDone: result.Metrics, MetricsTotal: result.Metrics,
		BeforeBytes: result.BeforeBytes, AfterBytes: result.AfterBytes,
		SavedBytes: saved, SavedPercent: percent,
	}
	c.mu.Unlock()
	c.completeLater()
}

func (c *Controller) runDiscard() {
	result, err := metricstore.DiscardConfiguredStoreHistory(context.Background(), func(progress metricstore.RestructureProgress) {
		c.mu.Lock()
		c.status.Phase = progress.Phase
		c.status.CurrentMetric = progress.CurrentMetric
		c.status.RowsDone = progress.RowsDone
		c.status.RowsTotal = progress.RowsTotal
		c.status.MetricsDone = progress.MetricsDone
		c.status.MetricsTotal = progress.MetricsTotal
		switch progress.Phase {
		case "reclaiming":
			c.status.State = "reclaiming"
		case "discarding":
			c.status.State = "discarding"
		}
		c.status.Progress = structureProgressPercent(progress)
		c.mu.Unlock()
	})
	if err != nil {
		c.fail(metricstore.RedactConnectionError(err.Error(), ""), "discarding")
		return
	}
	saved := result.BeforeBytes - result.AfterBytes
	if saved < 0 {
		saved = 0
	}
	percent := 0.0
	if result.BeforeBytes > 0 {
		percent = float64(saved) / float64(result.BeforeBytes) * 100
	}
	c.mu.Lock()
	c.status = Status{
		Mode: c.mode, State: "completed", Phase: "completed", Progress: 100,
		RowsDone: result.RowsCopied, RowsTotal: result.RowsCopied,
		MetricsDone: result.Metrics, MetricsTotal: result.Metrics,
		BeforeBytes: result.BeforeBytes, AfterBytes: result.AfterBytes,
		SavedBytes: saved, SavedPercent: percent,
	}
	c.mu.Unlock()
	c.completeLater()
}

func (c *Controller) completeLater() {
	time.AfterFunc(1500*time.Millisecond, func() {
		c.Deactivate()
		c.once.Do(func() { close(c.done) })
	})
}

func (c *Controller) fail(message, phase string) {
	c.mu.Lock()
	c.status.State = "failed"
	c.status.Phase = phase
	c.status.Error = message
	c.mu.Unlock()
}

func structureProgressPercent(progress metricstore.RestructureProgress) float64 {
	if progress.Phase == "reclaiming" {
		return structureProgressCeiling
	}
	if progress.Phase == "discarding" && progress.RowsTotal == 0 {
		return 1
	}
	if progress.RowsTotal <= 0 {
		return 0
	}
	value := float64(progress.RowsDone) / float64(progress.RowsTotal) * structureProgressCeiling
	if value < 0 {
		return 0
	}
	if value > structureProgressCeiling {
		return structureProgressCeiling
	}
	return value
}
