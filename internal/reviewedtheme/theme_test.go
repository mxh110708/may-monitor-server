package reviewedtheme

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	os.Chmod(root, 0755)
	os.Mkdir(filepath.Join(root, "dist"), 0755)
	files := map[string]string{}
	for name, body := range map[string]string{"komari-theme.json": `{"short":"GlassmorphismEnhanced"}`, "dist/index.html": "<html>reviewed</html>"} {
		os.WriteFile(filepath.Join(root, name), []byte(body), 0644)
		sum := sha256.Sum256([]byte(body))
		files[name] = hex.EncodeToString(sum[:])
	}
	raw, _ := json.Marshal(Lock{Schema: 1, Short: "GlassmorphismEnhanced", Files: files})
	os.WriteFile(filepath.Join(root, "may-theme.lock.json"), raw, 0644)
	pin := sha256.Sum256(raw)
	return root, hex.EncodeToString(pin[:])
}

func TestPinnedMembersAndEdits(t *testing.T) {
	root, pin := fixture(t)
	theme, err := load(root, pin, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := theme.read("../outside", false); ok {
		t.Fatal("traversal accepted")
	}
	os.WriteFile(filepath.Join(root, "dist/index.html"), []byte("changed"), 0644)
	if _, ok := theme.read("dist/index.html", false); ok {
		t.Fatal("changed asset served")
	}
	if _, err := load(root, pin, false); err == nil {
		t.Fatal("changed bundle accepted")
	}
}
func TestWrongPinWritableAndLinks(t *testing.T) {
	root, pin := fixture(t)
	if _, err := load(root, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false); err == nil {
		t.Fatal("wrong pin accepted")
	}
	os.Chmod(filepath.Join(root, "dist"), 0777)
	if _, err := load(root, pin, false); err == nil {
		t.Fatal("writable directory accepted")
	}
	os.Chmod(filepath.Join(root, "dist"), 0755)
	file := filepath.Join(root, "dist/index.html")
	os.Remove(file)
	if os.Symlink(filepath.Join(root, "komari-theme.json"), file) == nil {
		if _, err := load(root, pin, false); err == nil {
			t.Fatal("symlink accepted")
		}
	}
}
func TestInactiveAndMalformedConfiguration(t *testing.T) {
	t.Setenv(Environment, "")
	if Current() != nil {
		t.Fatal("theme active by default")
	}
	t.Setenv(Environment, "../unreviewed")
	if Current() != nil {
		t.Fatal("invalid operator configuration accepted")
	}
}
