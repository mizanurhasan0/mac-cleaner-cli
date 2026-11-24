package scan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"mac-cleaner-cli/internal/model"
)

const dupeMin = 1 * mb

type dfile struct {
	path string
	size int64
	mod  time.Time
}

// dupes finds files with identical content. Size first, then a 64 KB head hash, then a full hash.
func dupes(ctx context.Context, home string, emit func(Event)) {
	emit(Event{Status: "Listing files"})
	bySize := map[int64][]dfile{}
	seenInode := map[[2]uint64]bool{}
	for _, r := range []string{"Downloads", "Documents", "Desktop", "Pictures", "Movies", "Music"} {
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
			if !d.Type().IsRegular() || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".icloud") {
				return nil
			}
			info, e := d.Info()
			if e != nil || info.Size() < dupeMin {
				return nil
			}
			if st, ok := info.Sys().(*syscall.Stat_t); ok {
				key := [2]uint64{uint64(st.Dev), st.Ino}
				if seenInode[key] { // hard link to a file already counted
					return nil
				}
				seenInode[key] = true
			}
			bySize[info.Size()] = append(bySize[info.Size()], dfile{p, info.Size(), info.ModTime()})
			return nil
		})
	}

	var cands []dfile
	for _, grp := range bySize {
		if len(grp) > 1 {
			cands = append(cands, grp...)
		}
	}
	emit(Event{Status: fmt.Sprintf("Comparing %d candidate files", len(cands))})

	hashAll := func(files []dfile, head bool) map[string][]dfile {
		out := map[string][]dfile{}
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 4)
		for _, f := range files {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(f dfile) {
				defer wg.Done()
				defer func() { <-sem }()
				h, err := hashFile(ctx, f.path, head)
				if err != nil {
					return
				}
				k := fmt.Sprintf("%d:%s", f.size, h)
				mu.Lock()
				out[k] = append(out[k], f)
				mu.Unlock()
			}(f)
		}
		wg.Wait()
		return out
	}

	var second []dfile
	for _, g := range hashAll(cands, true) {
		if len(g) > 1 {
			second = append(second, g...)
		}
	}
	emit(Event{Status: fmt.Sprintf("Confirming %d files by full content", len(second))})

	var items []*model.Item
	for key, g := range hashAll(second, false) {
		if len(g) < 2 {
			continue
		}
		sort.Slice(g, func(i, j int) bool { return g[i].mod.Before(g[j].mod) })
		title := fmt.Sprintf("%s · %d copies", filepath.Base(g[0].path), len(g))
		for _, f := range g {
			items = append(items, &model.Item{
				Path: f.path, Label: tilde(home, f.path), Category: "Duplicates",
				Note: "Identical content to the other copies in this group", Size: f.size,
				Risk: model.Review, Action: model.Trash, ModTime: f.mod, GroupKey: key, Group: title,
			})
		}
	}
	emit(Event{Items: items})
}

func hashFile(ctx context.Context, path string, head bool) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	var r io.Reader = f
	if head {
		r = io.LimitReader(f, 64*1024)
	}
	buf := make([]byte, 256*1024)
	for {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		n, err := r.Read(buf)
		h.Write(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
