// Package reviewedtheme serves one operator-pinned static frontend without
// enabling plugin execution, web archive import or theme marketplace updates.
package reviewedtheme

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const Environment = "MAY_MONITOR_REVIEWED_THEME"
const Root = "/usr/local/share/may-monitor/themes"

type Lock struct {
	Schema int               `json:"schema"`
	Short  string            `json:"short"`
	Files  map[string]string `json:"files"`
}

type Theme struct {
	Short string
	root  string
	files map[string]string
}

var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var shortPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
var cache struct {
	sync.Mutex
	key   string
	value *Theme
}

// Current never falls back to another installed theme on a pin/ownership error.
// The caller can continue serving its embedded frontend for recovery.
func Current() *Theme {
	key := os.Getenv(Environment)
	cache.Lock()
	defer cache.Unlock()
	if key != cache.key {
		cache.key, cache.value = key, nil
		if digestPattern.MatchString(key) {
			cache.value, _ = load(filepath.Join(Root, key), key, true)
		}
	}
	return cache.value
}

func validName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "\\\x00") && path.Clean(name) == name && !strings.HasPrefix(name, "/") && !strings.HasPrefix(name, "../") &&
		(name == "komari-theme.json" || name == "preview.png" || name == "preview-v2.0.0.png" || strings.HasPrefix(name, "dist/"))
}

func regular(root, name string, requireOwner bool) (os.FileInfo, error) {
	current := root
	parts := strings.Split(filepath.ToSlash(name), "/")
	for i, part := range append([]string{""}, parts...) {
		if part != "" {
			current = filepath.Join(current, part)
		}
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 || requireOwner && !rootOwned(info) {
			return nil, errors.New("reviewed theme ownership is unavailable")
		}
		if i == len(parts) {
			if !info.Mode().IsRegular() || info.Size() > 16<<20 {
				return nil, errors.New("invalid reviewed theme file")
			}
			return info, nil
		}
		if !info.IsDir() {
			return nil, errors.New("invalid reviewed theme directory")
		}
	}
	return nil, errors.New("invalid reviewed theme path")
}

func load(root, pin string, requireOwner bool) (*Theme, error) {
	if requireOwner {
		for directory := filepath.Dir(root); ; directory = filepath.Dir(directory) {
			info, err := os.Lstat(directory)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 || !rootOwned(info) {
				return nil, errors.New("unsafe reviewed theme ancestor")
			}
			if directory == filepath.Dir(directory) {
				break
			}
		}
	}
	if !digestPattern.MatchString(pin) {
		return nil, errors.New("invalid theme pin")
	}
	if _, err := regular(root, "may-theme.lock.json", requireOwner); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(root, "may-theme.lock.json"))
	if err != nil || len(raw) > 2<<20 {
		return nil, errors.New("theme lock is unavailable")
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != pin {
		return nil, errors.New("theme lock digest differs")
	}
	var lock Lock
	if json.Unmarshal(raw, &lock) != nil || lock.Schema != 1 || !shortPattern.MatchString(lock.Short) || lock.Short == "default" || len(lock.Files) < 2 || len(lock.Files) > 10000 {
		return nil, errors.New("invalid reviewed theme manifest")
	}
	if lock.Files["dist/index.html"] == "" || lock.Files["komari-theme.json"] == "" {
		return nil, errors.New("theme entry is missing")
	}
	theme := &Theme{Short: lock.Short, root: root, files: lock.Files}
	var total int64
	for name, digest := range lock.Files {
		if !validName(name) || !digestPattern.MatchString(digest) {
			return nil, errors.New("invalid reviewed theme member")
		}
		info, err := regular(root, name, requireOwner)
		if err != nil {
			return nil, err
		}
		total += info.Size()
		if total > 256<<20 {
			return nil, errors.New("reviewed theme exceeds size bound")
		}
		if _, ok := theme.read(name, requireOwner); !ok {
			return nil, errors.New("reviewed theme content differs")
		}
	}
	manifest, _ := theme.read("komari-theme.json", requireOwner)
	var identity struct {
		Short string `json:"short"`
	}
	if json.Unmarshal(manifest, &identity) != nil || identity.Short != lock.Short {
		return nil, errors.New("theme identity differs")
	}
	return theme, nil
}

func (t *Theme) read(name string, requireOwner bool) ([]byte, bool) {
	if !validName(name) || t.files[name] == "" {
		return nil, false
	}
	if _, err := regular(t.root, name, requireOwner); err != nil {
		return nil, false
	}
	raw, err := os.ReadFile(filepath.Join(t.root, filepath.FromSlash(name)))
	if err != nil {
		return nil, false
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != t.files[name] {
		return nil, false
	}
	return raw, true
}

func (t *Theme) Read(name string) ([]byte, bool) { return t.read(name, true) }
