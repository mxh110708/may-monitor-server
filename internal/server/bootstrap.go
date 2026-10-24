package server

import (
	"context"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/utils"
)

// Bootstrap initializes the data directory, primary database, and settings.
func (a *App) Bootstrap() error {
	if err := preparePrivateDataDir(); err != nil {
		return err
	}

	dbcore.SetVersionID(utils.CurrentVersion + "-" + utils.VersionHash)
	if err := dbcore.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	a.dbReady = true
	a.addCleanup("database", func(context.Context) error { return dbcore.Close() })

	gin.SetMode(gin.ReleaseMode)
	settings, err := config.GetManyAs[config.Settings]()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
	a.settings = settings
	return nil
}

func preparePrivateDataDir() error {
	if info, err := os.Lstat("./data"); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
		return fmt.Errorf("data root must be a real private directory")
	}
	if err := os.MkdirAll("./data", 0700); err != nil {
		return err
	}
	if err := os.Chmod("./data", 0700); err != nil {
		return err
	}
	for _, path := range []string{"./data/theme", "./data/plugin", "./data/plugin-data"} {
		if err := os.MkdirAll(path, 0700); err != nil {
			return fmt.Errorf("create private data directory: %w", err)
		}
	}
	return nil
}
