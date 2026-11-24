package clean

import (
	"os"
	"path/filepath"
	"testing"

	"mac-cleaner-cli/internal/model"
)

func TestCheckRefusesProtected(t *testing.T) {
	home := "/Users/test"
	cases := map[string]bool{ // path -> refused
		"/Users/test":                            true,
		"/Users/test/Documents":                  true,
		"/Users/test/Library":                    true,
		"/Users/test/.ssh/id_ed25519":            true,
		"/Users/test/Library/Mobile Documents/x": true,
		"/etc/hosts":                             true,
		"/Users/test2/Library/Caches/x":          true,
		"relative/path":                          true,
		"/Users/test/Library/Caches/../../.ssh":  true,
		"/Users/test/.Trash/old":                 true,
	}
	for p, refused := range cases {
		err := Check(home, &model.Item{Path: p})
		if refused && err == nil {
			t.Errorf("Check(%q) should refuse", p)
		}
	}
}

func TestTrashAndDelete(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Caches", "com.example")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "a"), []byte("x"), 0o644)

	it := &model.Item{Path: dir, Action: model.Trash}
	if err := Execute(home, it, true); err != nil || !exists(dir) {
		t.Fatalf("dry run must not remove: %v", err)
	}
	if err := Execute(home, it, false); err != nil || exists(dir) || !exists(filepath.Join(home, ".Trash", "com.example")) {
		t.Fatalf("trash failed: %v", err)
	}

	dir2 := filepath.Join(home, "Library", "Logs", "x")
	os.MkdirAll(dir2, 0o755)
	if err := Execute(home, &model.Item{Path: dir2, Action: model.Delete}, false); err != nil || exists(dir2) {
		t.Fatalf("delete failed: %v", err)
	}
}

func TestSymlinkSkipped(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, "real")
	os.MkdirAll(target, 0o755)
	link := filepath.Join(home, "Library", "Caches", "link")
	os.MkdirAll(filepath.Dir(link), 0o755)
	os.Symlink(target, link)
	if err := Execute(home, &model.Item{Path: link, Action: model.Delete}, false); err == nil || !exists(target) {
		t.Fatalf("symlink must be skipped, target must survive")
	}
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }
