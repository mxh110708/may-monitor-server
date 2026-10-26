package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/komari-monitor/komari/internal/monitoring"
	"github.com/spf13/cobra"
)

func init() {
	var configPath, statePath string
	command := &cobra.Command{Use: "watchdog", Short: "Independent read-only Server availability check (loopback receiver only)", RunE: func(cmd *cobra.Command, args []string) error {
		info, err := os.Stat(configPath)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 8192 {
			return errors.New("watchdog configuration is unavailable")
		}
		raw, err := os.ReadFile(configPath)
		if err != nil {
			return errors.New("watchdog configuration cannot be read")
		}
		var cfg monitoring.WatchdogConfig
		if json.Unmarshal(raw, &cfg) != nil {
			return errors.New("invalid watchdog configuration")
		}
		if statePath == "" {
			return errors.New("an explicit watchdog state path is required")
		}
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return monitoring.RunWatchdog(ctx, cfg, os.Getenv("MAY_MONITOR_WATCHDOG_READ_TOKEN"), statePath)
	}}
	command.Flags().StringVar(&configPath, "config", "", "Watchdog JSON configuration (no credential)")
	command.Flags().StringVar(&statePath, "state", "", "Owned JSON state file")
	RootCmd.AddCommand(command)
}
