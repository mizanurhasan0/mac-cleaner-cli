package scan

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"mac-cleaner-cli/internal/model"
)

const largeFileMin = 500 * mb

// Folders that are really single documents or libraries; never entered by the large-file walk.
var packageExt = map[string]bool{
	".photoslibrary": true, ".app": true, ".imovielibrary": true, ".fcpbundle": true,
	".tvlibrary": true, ".musiclibrary": true, ".sparsebundle": true, ".vmwarevm": true,
	".pvm": true, ".utm": true, ".logicx": true, ".band": true, ".bundle": true,
}

// deep finds device backups, Mail downloads, the Trash and very large files.
func deep(ctx context.Context, home string, emit func(Event)) {
	lib := filepath.Join(home, "Library")

	emit(Event{Status: "Scanning device backups and Mail downloads"})
	var cs []cand
	for _, e := range readDir(filepath.Join(lib, "Application Support", "MobileSync", "Backup")) {
		p := filepath.Join(lib, "Application Support", "MobileSync", "Backup", e.Name())
		cs = append(cs, cand{p, mk(p, "iPhone/iPad backup "+shortID(e.Name()), "Device backups",
			"Only copy of that device's backup if iCloud backup is off", model.Review, model.Trash)})
	}
	mail := filepath.Join(lib, "Containers", "com.apple.mail", "Data", "Library", "Mail Downloads")
	if exists(mail) {
		cs = append(cs, cand{mail, mk(mail, "Mail attachments opened from Mail", "Mail downloads",
			"Copies of attachments; the originals stay in your mailbox", model.Review, model.Trash)})
	}
	emit(Event{Items: sizeAll(ctx, cs, 1*mb)})

	emit(Event{Status: "Measuring the Trash"})
	trash := filepath.Join(home, ".Trash")
	if exists(trash) {
		emit(Event{Items: sizeAll(ctx, []cand{{trash, mk(trash, "Finder Trash", "Trash",
			"Emptying is permanent and cannot be undone", model.Review, model.Delete)}}, 1)})
	}

	emit(Event{Status: "Looking for large files"})
	var large []cand
	roots := []string{"Documents", "Downloads", "Desktop", "Movies", "Music", "Pictures"}
	for _, r := range roots {
		root := filepath.Join(home, r)
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if p != root && (strings.HasPrefix(name, ".") || packageExt[strings.ToLower(filepath.Ext(name))] || name == "node_modules" || skipTree(p)) {
					return fs.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() || strings.HasSuffix(name, ".icloud") {
				return nil
			}
			if info, e := d.Info(); e == nil && info.Size() >= largeFileMin {
				large = append(large, cand{p, mk(p, tilde(home, p), "Large files",
					"Over 500 MB; check you no longer need it", model.Review, model.Trash)})
			}
			return nil
		})
	}
	emit(Event{Items: sizeAll(ctx, large, largeFileMin/2)})
}

func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

var _ = os.Lstat
