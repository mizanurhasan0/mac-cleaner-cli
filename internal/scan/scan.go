// Package scan finds things that can be cleaned. Scanners are read-only.
package scan

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"mac-cleaner-cli/internal/model"
)

// Mode selects which scanners run.
type Mode string

const (
	Quick Mode = "quick"
	Deep  Mode = "deep"
	Dev   Mode = "dev"
	Full  Mode = "full"
	Dupes Mode = "dupes"
	Disk  Mode = "disk" // handled by the disk browser, not by Start
)

// Event is a progress update from a running scan.
type Event struct {
	Status string
	Items  []*model.Item
}

type scanner func(ctx context.Context, home string, emit func(Event))

func scannersFor(m Mode) []scanner {
	switch m {
	case Quick:
		return []scanner{quick}
	case Deep:
		return []scanner{deep}
	case Dev:
		return []scanner{dev}
	case Dupes:
		return []scanner{dupes}
	case Full:
		return []scanner{quick, deep, dev}
	}
	return nil
}

// Start runs the scanners for a mode concurrently. The channel closes when all finish
// or the context is cancelled.
func Start(ctx context.Context, m Mode, home string) <-chan Event {
	ch := make(chan Event, 64)
	var wg sync.WaitGroup
	emit := func(e Event) {
		select {
		case ch <- e:
		case <-ctx.Done():
		}
	}
	for _, s := range scannersFor(m) {
		wg.Add(1)
		go func(s scanner) {
			defer wg.Done()
			s(ctx, home, emit)
		}(s)
	}
	go func() { wg.Wait(); close(ch) }()
	return ch
}

// Finalize removes duplicates by path and drops items that live inside another item.
func Finalize(items []*model.Item, home string) []*model.Item {
	sorted := append([]*model.Item(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool { return len(sorted[i].Path) < len(sorted[j].Path) })
	kept := map[string]bool{}
	var out []*model.Item
	for _, it := range sorted {
		if kept[it.Path] {
			continue
		}
		nested := false
		for p := filepath.Dir(it.Path); p != "/" && p != home && p != "."; p = filepath.Dir(p) {
			if kept[p] {
				nested = true
				break
			}
		}
		if nested {
			continue
		}
		kept[it.Path] = true
		out = append(out, it)
	}
	return out
}

// ---- size helpers ----

func allocated(info fs.FileInfo) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Blocks * 512
	}
	return info.Size()
}

// skipTree reports folders that are never entered: cloud placeholders and iCloud.
func skipTree(p string) bool {
	return strings.HasSuffix(p, "/Library/CloudStorage") || strings.HasSuffix(p, "/Library/Mobile Documents")
}

// DirSize returns the allocated size of a file or folder tree. Symlinks are not followed.
// partial is true when something could not be read.
func DirSize(ctx context.Context, root string) (size int64, partial bool) {
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			partial = true
			return ctx.Err()
		}
		if err != nil {
			partial = true
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() && skipTree(p) {
			return fs.SkipDir
		}
		info, e := d.Info()
		if e != nil {
			partial = true
			return nil
		}
		size += allocated(info)
		return nil
	})
	return size, partial
}

type cand struct {
	path string
	make func(size int64) *model.Item
}

// sizeAll measures candidates in parallel and returns those at least min bytes.
func sizeAll(ctx context.Context, cs []cand, min int64) []*model.Item {
	sem := make(chan struct{}, 8)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var out []*model.Item
	for _, c := range cs {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(c cand) {
			defer wg.Done()
			defer func() { <-sem }()
			sz, _ := DirSize(ctx, c.path)
			if sz < min {
				return
			}
			it := c.make(sz)
			if info, err := os.Lstat(c.path); err == nil {
				it.ModTime = info.ModTime()
			}
			mu.Lock()
			out = append(out, it)
			mu.Unlock()
		}(c)
	}
	wg.Wait()
	return out
}

func mk(path, label, cat, note string, r model.Risk, a model.Action) func(int64) *model.Item {
	return func(size int64) *model.Item {
		return &model.Item{Path: path, Label: label, Category: cat, Note: note, Size: size, Risk: r, Action: a}
	}
}

func readDir(dir string) []os.DirEntry {
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	return es
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func tilde(home, p string) string {
	if strings.HasPrefix(p, home+"/") {
		return "~" + p[len(home):]
	}
	return p
}

const (
	kb = int64(1000)
	mb = 1000 * kb
	gb = 1000 * mb
)
