package scan

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// Entry is one child of a folder in the Disk Doctor browser.
type Entry struct {
	Path    string
	Name    string
	Size    int64 // -1 while still measuring
	IsDir   bool
	Partial bool
	Err     string
}

// ListDir streams the children of dir: first unmeasured (Size -1), then measured.
// It is read-only.
func ListDir(ctx context.Context, dir string) <-chan Entry {
	ch := make(chan Entry, 128)
	send := func(e Entry) {
		select {
		case ch <- e:
		case <-ctx.Done():
		}
	}
	go func() {
		defer close(ch)
		des, err := os.ReadDir(dir)
		if err != nil {
			send(Entry{Path: dir, Err: err.Error(), Size: 0})
			return
		}
		var list []Entry
		for _, d := range des {
			e := Entry{Path: filepath.Join(dir, d.Name()), Name: d.Name(), IsDir: d.IsDir(), Size: -1}
			list = append(list, e)
			send(e)
		}
		sem := make(chan struct{}, 8)
		var wg sync.WaitGroup
		for _, e := range list {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(e Entry) {
				defer wg.Done()
				defer func() { <-sem }()
				e.Size, e.Partial = DirSize(ctx, e.Path)
				send(e)
			}(e)
		}
		wg.Wait()
	}()
	return ch
}

// Volume returns free and total bytes of the volume holding path.
func Volume(path string) (free, total uint64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0
	}
	return st.Bavail * uint64(st.Bsize), st.Blocks * uint64(st.Bsize)
}
