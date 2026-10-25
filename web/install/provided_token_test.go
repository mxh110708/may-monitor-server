package install

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	logger "github.com/komari-monitor/komari/utils/log"
)

func TestProvidedSetupTokenIsNotPrinted(t *testing.T) {
	const token = "synthetic-private-operator-token-for-test"
	t.Setenv("MAY_MONITOR_SETUP_TOKEN", token)
	controller := NewController(nil)
	// Origin is captured at construction, not guessed from a later environment.
	t.Setenv("MAY_MONITOR_SETUP_TOKEN", "")
	file, err := os.Create(filepath.Join(t.TempDir(), "console.log"))
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = file
	logger.Setup(slog.LevelInfo)
	t.Cleanup(func() { os.Stdout = old; logger.Setup(slog.LevelInfo); _ = file.Close() })
	controller.Activate()
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(output), token) || strings.Contains(string(output), "#setup=") || !strings.Contains(string(output), "operator-supplied") {
		t.Fatal("operator initialization credential was printed")
	}
}
