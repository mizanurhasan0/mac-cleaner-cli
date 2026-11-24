package scan

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"mac-cleaner-cli/internal/model"
)

// Names inside ~/Library/Caches that the dev scanner owns, so quick does not list them twice.
var devCacheNames = map[string]bool{
	"Homebrew": true, "pip": true, "Yarn": true, "pnpm": true, "CocoaPods": true,
	"org.swift.swiftpm": true, "go-build": true, "JetBrains": true,
}

var browserHints = []string{
	"google", "bravesoftware", "brave", "microsoft edge", "edgemac", "firefox", "mozilla",
	"thebrowser", "opera", "vivaldi", "safari",
}

func isBrowser(name string) bool {
	n := strings.ToLower(name)
	for _, h := range browserHints {
		if strings.Contains(n, h) {
			return true
		}
	}
	return false
}

var installerExt = map[string]bool{".dmg": true, ".pkg": true, ".iso": true, ".xip": true}

// quick finds caches, logs, browser data and old installers.
func quick(ctx context.Context, home string, emit func(Event)) {
	lib := filepath.Join(home, "Library")

	emit(Event{Status: "Scanning app and browser caches"})
	var cs []cand
	for _, e := range readDir(filepath.Join(lib, "Caches")) {
		name := e.Name()
		if strings.HasPrefix(name, ".") || devCacheNames[name] {
			continue
		}
		// Apple system caches are managed by macOS; only Safari's is a user cache.
		if strings.HasPrefix(name, "com.apple.") && name != "com.apple.Safari" {
			continue
		}
		cat := "App caches"
		if isBrowser(name) {
			cat = "Browsers"
		}
		p := filepath.Join(lib, "Caches", name)
		cs = append(cs, cand{p, mk(p, name, cat, "Rebuilt automatically when the app needs it", model.Safe, model.Delete)})
	}
	emit(Event{Items: sizeAll(ctx, cs, 512*kb)})

	emit(Event{Status: "Scanning logs"})
	cs = nil
	for _, e := range readDir(filepath.Join(lib, "Logs")) {
		p := filepath.Join(lib, "Logs", e.Name())
		cs = append(cs, cand{p, mk(p, e.Name(), "Logs", "Diagnostic and application logs", model.Safe, model.Delete)})
	}
	emit(Event{Items: sizeAll(ctx, cs, 256*kb)})

	emit(Event{Status: "Scanning old installers"})
	cs = nil
	cutoff := time.Now().AddDate(0, 0, -14)
	for _, e := range readDir(filepath.Join(home, "Downloads")) {
		if e.IsDir() || !installerExt[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		p := filepath.Join(home, "Downloads", e.Name())
		cs = append(cs, cand{p, mk(p, e.Name(), "Old installers", "Installer older than 14 days; installed apps keep working", model.Review, model.Trash)})
	}
	emit(Event{Items: sizeAll(ctx, cs, 1*mb)})
}
