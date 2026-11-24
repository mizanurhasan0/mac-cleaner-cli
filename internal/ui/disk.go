package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mac-cleaner-cli/internal/scan"
)

type diskState struct {
	dir     string
	entries map[string]scan.Entry
	cur     int
	off     int
	ch      <-chan scan.Entry
	cancel  context.CancelFunc
	err     string
}

type diskMsg struct {
	e  scan.Entry
	ok bool
}

func (d *diskState) stop() {
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
}

func waitDisk(ch <-chan scan.Entry) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		return diskMsg{e, ok}
	}
}

func (m Model) openDisk() (tea.Model, tea.Cmd) {
	m.sc = scDisk
	return m.loadDir(m.home)
}

func (m Model) loadDir(dir string) (tea.Model, tea.Cmd) {
	m.dk.stop()
	ctx, cancel := context.WithCancel(context.Background())
	m.dk = diskState{dir: dir, entries: map[string]scan.Entry{}, cancel: cancel, ch: scan.ListDir(ctx, dir)}
	return m, waitDisk(m.dk.ch)
}

func (m Model) onDisk(msg diskMsg) (tea.Model, tea.Cmd) {
	if m.sc != scDisk || !msg.ok {
		return m, nil
	}
	if msg.e.Err != "" {
		m.dk.err = msg.e.Err
		return m, waitDisk(m.dk.ch)
	}
	m.dk.entries[msg.e.Path] = msg.e
	return m, waitDisk(m.dk.ch)
}

func (m Model) sortedEntries() []scan.Entry {
	out := make([]scan.Entry, 0, len(m.dk.entries))
	for _, e := range m.dk.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Size != out[j].Size {
			return out[i].Size > out[j].Size
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (m Model) keyDisk(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	es := m.sortedEntries()
	switch k.String() {
	case "q", "esc":
		m.dk.stop()
		m.sc = scMenu
		m.mode = ""
	case "up", "k":
		if m.dk.cur > 0 {
			m.dk.cur--
		}
	case "down", "j":
		if m.dk.cur < len(es)-1 {
			m.dk.cur++
		}
	case "enter", "right", "l":
		if m.dk.cur < len(es) && es[m.dk.cur].IsDir {
			return m.loadDir(es[m.dk.cur].Path)
		}
	case "left", "h", "backspace":
		if m.dk.dir != m.home {
			return m.loadDir(filepath.Dir(m.dk.dir))
		}
	case "r":
		return m.loadDir(m.dk.dir)
	}
	h := m.h - 8
	if h < 5 {
		h = 5
	}
	if m.dk.cur < m.dk.off {
		m.dk.off = m.dk.cur
	}
	if m.dk.cur >= m.dk.off+h {
		m.dk.off = m.dk.cur - h + 1
	}
	return m, nil
}

func (m Model) viewDisk() string {
	var b strings.Builder
	b.WriteString(m.header("Disk doctor"))
	b.WriteString("  " + sBold.Render(tilde(m.home, m.dk.dir)) + sDim.Render("   read-only") + "\n")

	es := m.sortedEntries()
	var total int64
	measuring := 0
	for _, e := range es {
		if e.Size < 0 {
			measuring++
		} else {
			total += e.Size
		}
	}
	status := fmt.Sprintf("%s in %s", human(total), plural(len(es), "item", "items"))
	if measuring > 0 {
		status += "  " + sAccent.Render(spinner[m.frame%len(spinner)]) + sDim.Render(fmt.Sprintf(" measuring %d", measuring))
	}
	b.WriteString("  " + status + "\n\n")

	if m.dk.err != "" {
		b.WriteString("  " + sWarn.Render("Cannot read this folder: "+m.dk.err) + "\n")
		b.WriteString(sDim.Render("  Grant Full Disk Access to your terminal in System Settings › Privacy & Security.") + "\n")
	}

	h := m.h - 8
	if h < 5 {
		h = 5
	}
	end := m.dk.off + h
	if end > len(es) {
		end = len(es)
	}
	var max int64 = 1
	for _, e := range es {
		if e.Size > max {
			max = e.Size
		}
	}
	nameW := m.w - 2 - 2 - 22 - 12 - 8
	if nameW < 12 {
		nameW = 12
	}
	for i := m.dk.off; i < end; i++ {
		e := es[i]
		cur := "  "
		name := trunc(e.Name, nameW-2)
		if e.IsDir {
			name += "/"
		}
		name = pad(name, nameW)
		if i == m.dk.cur {
			cur = sAccent.Render("▌ ")
			name = sBold.Render(name)
		}
		pct := ""
		if total > 0 && e.Size >= 0 {
			pct = fmt.Sprintf("%3.0f%%", 100*float64(e.Size)/float64(total))
		}
		size := rpad(human(e.Size), 9)
		partial := " "
		if e.Partial {
			partial = sWarn.Render("~")
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s%s %s\n", cur, name, bar(float64(maxI(e.Size, 0))/float64(max), 20), size, partial, sDim.Render(rpad(pct, 4))))
	}
	b.WriteString(sDim.Render("\n  ↑↓ move · enter open folder · ← back · r rescan · q menu") + "\n")
	b.WriteString(sDim.Render("  ~ = some files could not be read, so the size is a lower bound") + "\n")
	return b.String()
}

func maxI(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
