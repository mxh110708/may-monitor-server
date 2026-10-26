package database

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/internal/managedconfig"
	"github.com/komari-monitor/komari/internal/metricstore"
	"github.com/komari-monitor/komari/internal/reviewedtheme"
	"github.com/komari-monitor/komari/internal/securitypolicy"
	logger "github.com/komari-monitor/komari/utils/log"
	"github.com/komari-monitor/komari/web/public"
)

func GetPublicInfo() (map[string]interface{}, error) {
	cstPtr, err := config.GetManyAs[config.Settings]()
	if err != nil {
		return nil, err
	}
	cst := *cstPtr

	all, allErr := config.GetAll()
	hasKey := func(k string) bool {
		if allErr != nil {
			return false
		}
		_, ok := all[k]
		return ok
	}

	// Apply defaults only when a key is missing.
	if !hasKey("sitename") {
		cst.Sitename = "May Monitor"
	}
	if !hasKey("description") {
		cst.Description = "May Monitor, a server monitoring tool based on Komari."
	}
	if !hasKey("theme") {
		cst.Theme = "default"
	}
	if !hasKey("o_auth_provider") {
		cst.OAuthProvider = "github"
	}

	// Fallback defaults if we couldn't enumerate keys.
	if allErr != nil {
		if cst.Sitename == "" {
			cst.Sitename = "May Monitor"
		}
		if cst.Description == "" {
			cst.Description = "May Monitor, a server monitoring tool based on Komari."
		}
	}
	retention, err := metricstore.GetRetentionSummary(context.Background())
	if err != nil {
		return nil, err
	}
	if theme := reviewedtheme.Current(); theme != nil {
		cst.Theme = theme.Short
	}
	if !securitypolicy.Enabled("EXTENSIONS") {
		cst.CustomHead = ""
		cst.CustomBody = ""
	}
	db := dbcore.GetDBInstance()
	tc := models.ThemeConfiguration{}
	err = db.Model(&models.ThemeConfiguration{}).Where("short = ?", cst.Theme).First(&tc).Error
	if err != nil {
		tc.Data = "{}"
	}
	tc_data := gin.H{}
	err = json.Unmarshal([]byte(tc.Data), &tc_data)
	if err != nil {
		logger.Infof("database", "%v", err)
	}
	items := themeConfigurationItems(cst.Theme)
	if cst.Theme != "default" {
		for _, item := range items {
			if item.Key == "" {
				continue
			}
			if _, exists := tc_data[item.Key]; !exists {
				tc_data[item.Key] = managedconfig.DefaultValue(item)
			}
		}
	}
	if err := managedconfig.ResolveForOutput(tc_data, items); err != nil {
		return nil, err
	}

	return gin.H{
		"sitename":                  cst.Sitename,
		"description":               cst.Description,
		"custom_head":               cst.CustomHead,
		"custom_body":               cst.CustomBody,
		"oauth_enable":              cst.OAuthEnabled,
		"oauth_provider":            cst.OAuthProvider,
		"disable_password_login":    cst.DisablePasswordLogin,
		"cors_origin_check_enabled": cst.CorsOriginCheckEnabled,
		"record_enabled":            retention.AllPositive, // 兼容旧版本主题
		"record_preserve_time":      retention.MaxDays * 24,
		"ping_record_preserve_time": retention.MaxDays * 24,
		"private_site":              cst.PrivateSite,
		"theme":                     cst.Theme,
		"theme_settings":            tc_data,
		"may_capabilities":          gin.H{"remote_control": securitypolicy.Enabled("REMOTE_CONTROL"), "extensions": securitypolicy.Enabled("EXTENSIONS"), "debug": securitypolicy.Enabled("DEBUG"), "reviewed_theme": reviewedtheme.Current() != nil},
	}, nil
}

func themeConfigurationItems(short string) []models.ManagedThemeConfigurationItem {
	var manifest models.Theme
	if short == "default" {
		data, err := public.PublicFS.ReadFile("defaultTheme/komari-theme.json")
		if err != nil || json.Unmarshal(data, &manifest) != nil {
			return nil
		}
	} else {
		if theme := reviewedtheme.Current(); theme != nil && theme.Short == short {
			data, ok := theme.Read("komari-theme.json")
			if !ok || json.Unmarshal(data, &manifest) != nil {
				return nil
			}
			return managedconfig.Items(manifest.Configuration)
		}
		data, err := os.ReadFile(filepath.Join("./data/theme", short, "komari-theme.json"))
		if err != nil || json.Unmarshal(data, &manifest) != nil {
			return nil
		}
	}
	return managedconfig.Items(manifest.Configuration)
}
