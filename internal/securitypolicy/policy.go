// Package securitypolicy defines operator-controlled capabilities. HTTP settings
// and API keys cannot enable these process-level gates.
package securitypolicy

import (
	"github.com/komari-monitor/komari/internal/reviewedtheme"
	"os"
	"strings"
)

func Enabled(capability string) bool { return os.Getenv("MAY_MONITOR_ALLOW_"+capability) == "true" }

func RPCAllowed(method string) bool {
	if strings.HasPrefix(method, "admin:db") {
		return Enabled("DATABASE_CONSOLE")
	}
	if strings.Contains(method, "Plugin") {
		return Enabled("EXTENSIONS")
	}
	if strings.HasPrefix(method, "admin:file") {
		return Enabled("REMOTE_CONTROL")
	}
	switch method {
	case "admin:exec", "admin:getAgentStartupConfig", "admin:switchAgentVersion":
		return Enabled("REMOTE_CONTROL")
	}
	return true
}

func HTTPAllowed(method, path string) bool {
	if strings.HasPrefix(path, "/api/plugin/") || strings.HasPrefix(path, "/api/admin/plugin/") {
		return Enabled("EXTENSIONS")
	}
	if strings.HasPrefix(path, "/api/admin/theme/") && path != "/api/admin/theme/list" {
		if path == "/api/admin/theme/settings" && method == "POST" && reviewedtheme.Current() != nil {
			return true
		}
		return Enabled("EXTENSIONS")
	}
	// Archives can install executable plugins/themes or replace auth state.
	if strings.HasPrefix(path, "/api/admin/uploads/") {
		return Enabled("ARCHIVE_IMPORT") && Enabled("EXTENSIONS")
	}
	if strings.HasPrefix(path, "/api/admin/debug/") {
		return Enabled("DEBUG")
	}
	if strings.Contains(path, "/terminal") || strings.Contains(path, "/file/") || strings.HasPrefix(path, "/api/preview/") || strings.HasPrefix(path, "/api/clients/transfer/") {
		return Enabled("REMOTE_CONTROL")
	}
	return true
}
