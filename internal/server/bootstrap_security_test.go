package server

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDataRootIsPrivateAndDoesNotFollowSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission checks")
	}
	t.Chdir(t.TempDir())
	if err := preparePrivateDataDir(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat("data")
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("data root not private")
	}
	outside := t.TempDir()
	linkRoot := t.TempDir()
	t.Chdir(linkRoot)
	if err := os.Symlink(outside, filepath.Join(linkRoot, "data")); err != nil {
		t.Fatal(err)
	}
	if err := preparePrivateDataDir(); err == nil {
		t.Fatal("symlink data root accepted")
	}
}
