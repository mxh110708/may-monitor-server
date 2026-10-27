//go:build linux

package monitoring

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWatchdogStateExclusiveAndSafe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	unlock, err := lockWatchdogState(path)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := lockWatchdogState(path); err == nil {
		other()
		t.Fatal("concurrent watchdog accepted")
	}
	unlock()
	unlock, err = lockWatchdogState(path)
	if err != nil {
		t.Fatal("released watchdog lock did not reopen")
	}
	unlock()
	os.Remove(path + ".lock")
	if err := os.Symlink(path, path+".lock"); err != nil {
		t.Fatal(err)
	}
	if other, err := lockWatchdogState(path); err == nil {
		other()
		t.Fatal("symlink lock accepted")
	}
}
