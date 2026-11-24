// Package ui is the terminal interface: menu, scan, select, confirm, clean.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mac-cleaner-cli/internal/model"
	"mac-cleaner-cli/internal/scan"
)

type screen int

const (
	scMenu screen = iota
	scScan
	scResults
	scConfirm
	scRun
	scReport
	scDisk
)

type menuEntry struct {
	key   string
	mode  scan.Mode
	title string
	desc  string
}

var menu = []menuEntry{
	{"1", scan.Full, "Full scan", "Quick + Deep + Dev in one pass"},
	{"2", scan.Quick, "Quick clean", "App and browser caches, logs, old installers"},
	{"3", scan.Deep, "Deep clean", "Device backups, Mail downloads, Trash, large files"},
	{"4", scan.Dev, "Dev clean", "node_modules, build output, package and Xcode caches"},
	{"5", scan.Dupes, "Duplicates", "Identical files in Downloads, Documents, Desktop, media"},
	{"6", scan.Disk, "Disk doctor", "See what uses space (read-only)"},
}

// Model is the root Bubble Tea model.
type Model struct {
	home, version string
	dry           bool
	w, h          int
	sc            screen
	frame         int

	menuIdx int

	// scan
	mode    scan.Mode
	cancel  context.CancelFunc
	ch      <-chan scan.Event
	status  string
	items   []*model.Item
	found   int64
	started time.Time

	// results
	roots []*node
	flat  []*node
	cur   int
	off   int
	flash string

	// confirm / run / report
	typed      string
	queue      []*model.Item
	ri         int
	freed      int64
	stop       bool
	failures   []failure
	doneN      int
	freeBefore uint64
	runStart   time.Time
	recent     []string

	// disk doctor
	dk diskState

	free, total uint64
}

type failure struct {
	path string
	err  string
}

type (
	tickMsg     struct{}
	scanMsg     scan.Event
	scanDoneMsg struct{}
)

// New builds the model. A non-empty start mode skips the menu.
func New(home, version string, dry bool, start scan.Mode) Model {
	m := Model{home: home, version: version, dry: dry, w: 100, h: 30}
	m.refreshVolume()
	if start != "" {
		m.mode = start
	}
	return m
}

// Start returns the first command.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tick(), func() tea.Msg { return startMsg{} })
}

type startMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(110*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *Model) refreshVolume() { m.free, m.total = scan.Volume(m.home) }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.frame++
		return m, tick()
	case startMsg:
		if m.mode != "" {
			return m.beginMode(m.mode)
		}
		return m, nil
	case scanMsg:
		if m.sc != scScan {
			return m, nil
		}
		if msg.Status != "" {
			m.status = msg.Status
		}
		for _, it := range msg.Items {
			m.found += it.Size
		}
		m.items = append(m.items, msg.Items...)
		return m, waitScan(m.ch)
	case scanDoneMsg:
		if m.sc == scScan {
			m.finishScan()
		}
		return m, nil
	case runMsg:
		return m.onRun(msg)
	case diskMsg:
		return m.onDisk(msg)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			m.dk.stop()
			return m, tea.Quit
		}
		switch m.sc {
		case scMenu:
			return m.keyMenu(msg)
		case scScan:
			if msg.String() == "esc" || msg.String() == "q" {
				if m.cancel != nil {
					m.cancel()
				}
				m.sc = scMenu
				m.mode = ""
			}
		case scResults:
			return m.keyResults(msg)
		case scConfirm:
			return m.keyConfirm(msg)
		case scRun:
			if msg.String() == "esc" || msg.String() == "q" {
				m.stop = true
			}
		case scReport:
			if msg.String() == "q" {
				return m, tea.Quit
			}
			m.sc = scMenu
			m.mode = ""
			m.refreshVolume()
		case scDisk:
			return m.keyDisk(msg)
		}
	}
	return m, nil
}

func (m Model) keyMenu(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch s := k.String(); s {
	case "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.menuIdx > 0 {
			m.menuIdx--
		}
	case "down", "j":
		if m.menuIdx < len(menu)-1 {
			m.menuIdx++
		}
	case "enter", " ":
		return m.beginMode(menu[m.menuIdx].mode)
	default:
		for i, e := range menu {
			if s == e.key {
				m.menuIdx = i
				return m.beginMode(e.mode)
			}
		}
	}
	return m, nil
}

func (m Model) beginMode(mode scan.Mode) (tea.Model, tea.Cmd) {
	m.mode = mode
	if mode == scan.Disk {
		return m.openDisk()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.ch = scan.Start(ctx, mode, m.home)
	m.items, m.found, m.status = nil, 0, "Starting"
	m.started = time.Now()
	m.sc = scScan
	return m, waitScan(m.ch)
}

func waitScan(ch <-chan scan.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return scanDoneMsg{}
		}
		return scanMsg(ev)
	}
}

func (m *Model) finishScan() {
	items := scan.Finalize(m.items, m.home)
	m.roots = buildTree(items)
	m.flat = flatten(m.roots)
	m.cur, m.off, m.flash = 0, 0, ""
	m.sc = scResults
}

// ---- views ----

func (m Model) View() string {
	var body string
	switch m.sc {
	case scMenu:
		body = m.viewMenu()
	case scScan:
		body = m.viewScan()
	case scResults:
		body = m.viewResults()
	case scConfirm:
		body = m.viewConfirm()
	case scRun:
		body = m.viewRun()
	case scReport:
		body = m.viewReport()
	case scDisk:
		body = m.viewDisk()
	}
	return body
}

func (m Model) header(sub string) string {
	t := sTitle.Render("mac-cleaner-cli") + sDim.Render(" "+m.version)
	if m.dry {
		t += "  " + sWarn.Render("DRY RUN: nothing will be removed")
	}
	if sub != "" {
		t += sDim.Render("  ›  ") + sBold.Render(sub)
	}
	return t + "\n"
}

func (m Model) volumeLine() string {
	if m.total == 0 {
		return ""
	}
	used := float64(m.total-m.free) / float64(m.total)
	return fmt.Sprintf("%s  %s free of %s", bar(used, 24), sBold.Render(human(int64(m.free))), human(int64(m.total)))
}

func (m Model) viewMenu() string {
	var b strings.Builder
	b.WriteString(m.header(""))
	b.WriteString("\n  " + m.volumeLine() + "\n\n")
	for i, e := range menu {
		cur := "  "
		title := e.title
		if i == m.menuIdx {
			cur = sAccent.Render("▌ ")
			title = sBold.Render(pad(e.title, 14))
		} else {
			title = pad(e.title, 14)
		}
		b.WriteString(fmt.Sprintf("%s%s %s  %s\n", cur, sDim.Render(e.key), title, sDim.Render(e.desc)))
	}
	b.WriteString("\n" + sDim.Render("  ↑↓ move · enter select · 1-6 jump · q quit") + "\n")
	b.WriteString(sDim.Render("  Nothing is removed until you review a list and confirm.") + "\n")
	b.WriteString(sDim.Render("\n  © 2026 Mizanur Hasan · eng.mizanur.hasan@gmail.com") + "\n")
	return b.String()
}

func (m Model) viewScan() string {
	var b strings.Builder
	title := "Scanning"
	for _, e := range menu {
		if e.mode == m.mode {
			title = e.title
		}
	}
	b.WriteString(m.header(title))
	el := time.Since(m.started).Round(time.Second)
	b.WriteString(fmt.Sprintf("\n  %s %s\n", sAccent.Render(spinner[m.frame%len(spinner)]), m.status))
	b.WriteString(sDim.Render(fmt.Sprintf("\n  %s found so far · %s elapsed", human(m.found), el)) + "\n")
	b.WriteString(sDim.Render("\n  Scanning only reads. esc cancel") + "\n")
	b.WriteString(sWarn.Render("\n  Stuck? macOS may be asking for folder access. Look for a popup and click Allow.") + "\n")
	return b.String()
}
