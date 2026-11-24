package scan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mac-cleaner-cli/internal/model"
)

type devCache struct {
	rel, label, cat, note string
	risk                  model.Risk
}

// Known package-manager, toolchain and Xcode locations (relative to $HOME).
var devCaches = []devCache{
	{".npm/_cacache", "npm cache", "Developer caches", "Re-downloaded on next install", model.Safe},
	{"Library/Caches/Yarn", "Yarn cache", "Developer caches", "Re-downloaded on next install", model.Safe},
	{"Library/Caches/pnpm", "pnpm cache", "Developer caches", "Re-downloaded on next install", model.Safe},
	{"Library/pnpm/store", "pnpm store", "Developer caches", "Shared by all projects; they re-link on next install", model.Review},
	{"Library/Caches/pip", "pip cache", "Developer caches", "Re-downloaded on next install", model.Safe},
	{"Library/Caches/Homebrew", "Homebrew downloads", "Developer caches", "Old bottles and downloads", model.Safe},
	{"Library/Caches/CocoaPods", "CocoaPods cache", "Developer caches", "Re-downloaded on next pod install", model.Safe},
	{"Library/Caches/org.swift.swiftpm", "SwiftPM cache", "Developer caches", "Re-downloaded on next build", model.Safe},
	{"Library/Caches/go-build", "Go build cache", "Developer caches", "Rebuilt on next go build", model.Safe},
	{"go/pkg/mod/cache", "Go module cache", "Developer caches", "Re-downloaded on next build", model.Safe},
	{".cargo/registry/cache", "Cargo registry cache", "Developer caches", "Re-downloaded on next build", model.Review},
	{".gradle/caches", "Gradle caches", "Developer caches", "Large re-download on next build", model.Review},
	{".m2/repository", "Maven repository", "Developer caches", "Large re-download on next build", model.Review},
	{"Library/Caches/JetBrains", "JetBrains caches", "Developer caches", "IDE re-indexes on next open", model.Safe},
	{"Library/Developer/Xcode/DerivedData", "Xcode DerivedData", "Xcode", "Rebuilt on next build", model.Safe},
	{"Library/Developer/CoreSimulator/Caches", "Simulator caches", "Xcode", "Rebuilt automatically", model.Safe},
	{"Library/Developer/Xcode/iOS DeviceSupport", "iOS DeviceSupport", "Xcode", "Symbols re-copied when a device connects", model.Review},
	{"Library/Developer/Xcode/Archives", "Xcode Archives", "Xcode", "Past release builds; needed to symbolicate old crashes", model.Review},
}

type artifact struct {
	markers []string
	build   bool // true: rebuilt from source; false: dependencies that need a download
}

var artifacts = map[string]artifact{
	"node_modules":  {[]string{"package.json"}, false},
	"Pods":          {[]string{"Podfile"}, false},
	"venv":          {[]string{"pyproject.toml", "requirements.txt", "setup.py"}, false},
	".venv":         {[]string{"pyproject.toml", "requirements.txt", "setup.py"}, false},
	"target":        {[]string{"Cargo.toml", "pom.xml"}, true},
	".next":         {[]string{"package.json"}, true},
	".nuxt":         {[]string{"package.json"}, true},
	".svelte-kit":   {[]string{"package.json"}, true},
	".turbo":        {[]string{"package.json"}, true},
	".parcel-cache": {[]string{"package.json"}, true},
	"dist":          {[]string{"package.json"}, true},
	"build":         {[]string{"package.json", "build.gradle", "build.gradle.kts", "pom.xml"}, true},
	".gradle":       {[]string{"build.gradle", "build.gradle.kts", "settings.gradle"}, true},
}

var projectRoots = []string{"Developer", "Projects", "projects", "code", "Code", "dev", "src", "work", "workspace",
	"repos", "github", "Documents", "Desktop", "Downloads"}

const artifactMin = 5 * mb

// dev finds package-manager caches, Xcode data and per-project build output.
func dev(ctx context.Context, home string, emit func(Event)) {
	emit(Event{Status: "Scanning developer caches"})
	var cs []cand
	for _, d := range devCaches {
		p := filepath.Join(home, d.rel)
		if !exists(p) {
			continue
		}
		act := model.Delete
		if d.cat == "Xcode" && d.risk == model.Review {
			act = model.Trash
		}
		cs = append(cs, cand{p, mk(p, d.label, d.cat, d.note, d.risk, act)})
	}
	emit(Event{Items: sizeAll(ctx, cs, 1*mb)})

	emit(Event{Status: "Looking for project build output"})
	var found []cand
	for _, r := range projectRoots {
		root := filepath.Join(home, r)
		if !exists(root) {
			continue
		}
		walkProjects(ctx, home, root, 0, &found)
	}
	emit(Event{Items: sizeAll(ctx, found, artifactMin)})
}

func walkProjects(ctx context.Context, home, dir string, depth int, out *[]cand) {
	if depth > 6 || ctx.Err() != nil || skipTree(dir) {
		return
	}
	entries := readDir(dir)
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name()] = true
	}
	for _, e := range entries {
		if !e.IsDir() || e.Type()&os.ModeSymlink != 0 {
			continue
		}
		name := e.Name()
		p := filepath.Join(dir, name)
		if a, ok := artifacts[name]; ok {
			marker := ""
			for _, m := range a.markers {
				if names[m] {
					marker = m
					break
				}
			}
			if marker == "" {
				continue
			}
			*out = append(*out, projectCand(home, dir, p, name, marker, a))
			continue
		}
		if strings.HasPrefix(name, ".") || name == "Library" || name == "node_modules" || name == "Applications" {
			continue
		}
		if packageExt[strings.ToLower(filepath.Ext(name))] {
			continue
		}
		walkProjects(ctx, home, p, depth+1, out)
	}
}

func projectCand(home, project, path, name, marker string, a artifact) cand {
	last := mtime(filepath.Join(project, marker))
	for _, g := range []string{".git/index", ".git/HEAD"} {
		if t := mtime(filepath.Join(project, g)); t.After(last) {
			last = t
		}
	}
	idle := int(time.Since(last).Hours() / 24)
	risk, how := model.Safe, "rebuilt from source"
	if !a.build {
		how = "needs a download to restore"
	}
	note := fmt.Sprintf("Project idle %dd; %s", idle, how)
	if idle < 14 {
		risk = model.Review
		note = fmt.Sprintf("Project active %dd ago; %s", idle, how)
	}
	return cand{path, mk(path, tilde(home, path), "Project artifacts", note, risk, model.Delete)}
}

func mtime(p string) time.Time {
	if info, err := os.Stat(p); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}
