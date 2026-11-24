// Package clean removes items. Every removal passes through Check first.
package clean

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mac-cleaner-cli/internal/model"
)

// Never touched, even if a scanner (or a bug) hands them over.
func protectedExact(home string) map[string]bool {
	m := map[string]bool{home: true}
	for _, d := range []string{"Library", "Documents", "Desktop", "Downloads", "Pictures", "Movies", "Music",
		"Applications", "Public", "Developer", ".ssh", ".gnupg", ".config", "Library/Application Support",
		"Library/Caches", "Library/Logs", "Library/Preferences", "Library/Containers", "Library/Keychains"} {
		m[filepath.Join(home, d)] = true
	}
	return m
}

func protectedPrefix(home string) []string {
	var out []string
	for _, d := range []string{".ssh", ".gnupg", "Library/Keychains", "Library/Mobile Documents",
		"Library/CloudStorage", "Library/Messages", "Library/Mail", "Library/Preferences"} {
		out = append(out, filepath.Join(home, d)+"/")
	}
	return out
}

// Check returns an error when an item must not be removed.
func Check(home string, it *model.Item) error {
	p := it.Path
	if !filepath.IsAbs(p) || filepath.Clean(p) != p {
		return errors.New("refused: path is not absolute and clean")
	}
	trash := filepath.Join(home, ".Trash")
	if p == trash {
		if it.Action != model.Delete {
			return errors.New("refused: the Trash itself can only be emptied")
		}
		return nil
	}
	if !strings.HasPrefix(p, home+"/") {
		return errors.New("refused: outside your home folder")
	}
	if strings.HasPrefix(p, trash+"/") {
		return errors.New("refused: already in the Trash")
	}
	if protectedExact(home)[p] {
		return errors.New("refused: protected folder")
	}
	for _, pre := range protectedPrefix(home) {
		if strings.HasPrefix(p+"/", pre) {
			return errors.New("refused: protected location")
		}
	}
	info, err := os.Lstat(p)
	if err != nil {
		return errors.New("already gone")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("skipped: symbolic link")
	}
	return nil
}

// Execute removes one item. With dry set it only runs the checks.
func Execute(home string, it *model.Item, dry bool) error {
	if err := Check(home, it); err != nil {
		return err
	}
	if dry {
		return nil
	}
	if it.Path == filepath.Join(home, ".Trash") {
		return emptyDir(it.Path)
	}
	if it.Action == model.Trash {
		return moveToTrash(home, it.Path)
	}
	return os.RemoveAll(it.Path)
}

func emptyDir(dir string) error {
	es, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var first error
	for _, e := range es {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func moveToTrash(home, path string) error {
	trash := filepath.Join(home, ".Trash")
	if err := os.MkdirAll(trash, 0o700); err != nil {
		return err
	}
	dest := filepath.Join(trash, filepath.Base(path))
	if _, err := os.Lstat(dest); err == nil {
		ext := filepath.Ext(dest)
		dest = strings.TrimSuffix(dest, ext) + time.Now().Format(" 15.04.05") + ext
	}
	if err := os.Rename(path, dest); err != nil {
		return fmt.Errorf("could not move to Trash (%v)", err)
	}
	return nil
}

// Record is one line of the history log.
type Record struct {
	Time   time.Time `json:"time"`
	Path   string    `json:"path"`
	Action string    `json:"action"`
	Size   int64     `json:"size"`
	Status string    `json:"status"`
	Dry    bool      `json:"dry_run,omitempty"`
}

// LogPath is where removals are recorded.
func LogPath(home string) string {
	return filepath.Join(home, ".config", "mac-cleaner-cli", "history.log")
}

// Log appends a record; failures are ignored because logging must never block cleaning.
func Log(home string, r Record) {
	p := LogPath(home)
	if os.MkdirAll(filepath.Dir(p), 0o700) != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(r)
	f.Write(append(b, '\n'))
}
