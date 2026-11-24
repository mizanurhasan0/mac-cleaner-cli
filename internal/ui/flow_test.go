package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mac-cleaner-cli/internal/model"
	"mac-cleaner-cli/internal/scan"
)

func write(t *testing.T, p string, n int) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, bytes.Repeat([]byte("a"), n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// drive feeds a message and runs returned commands until they stop producing messages we care about.
func drive(m tea.Model, msg tea.Msg) tea.Model {
	m, cmd := m.Update(msg)
	for i := 0; cmd != nil && i < 200; i++ {
		out := cmd()
		switch out.(type) {
		case scanMsg, scanDoneMsg, runMsg, startMsg:
			m, cmd = m.Update(out)
		default:
			return m // ticks, batches: ignore
		}
	}
	return m
}

func TestFullFlowQuickClean(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, "Library/Caches/com.foo.app/blob"), 3_000_000)
	write(t, filepath.Join(home, "Library/Caches/com.bar.app/blob"), 2_000_000)
	write(t, filepath.Join(home, "Library/Logs/old/log"), 1_000_000)
	write(t, filepath.Join(home, "Downloads/setup.dmg"), 2_000_000)
	os.Chtimes(filepath.Join(home, "Downloads/setup.dmg"), oldTime(), oldTime())

	var m tea.Model = New(home, "test", false, "")
	m = drive(m, tea.WindowSizeMsg{Width: 110, Height: 32})
	m = drive(m, key("2")) // Quick clean -> scan runs to completion
	mm := m.(Model)
	if mm.sc != scResults {
		t.Fatalf("expected results screen, got %v", mm.sc)
	}
	view := m.View()
	for _, want := range []string{"App caches", "com.foo.app", "Logs", "Old installers", "setup.dmg"} {
		if !strings.Contains(view, want) {
			t.Errorf("results view missing %q", want)
		}
	}
	if sumSelected(mm.roots).count() != 0 {
		t.Fatal("nothing may be pre-selected")
	}

	// Deny path: c with nothing selected must not advance.
	m = drive(m, key("c"))
	if m.(Model).sc != scResults {
		t.Fatal("c with empty selection must stay on results")
	}

	// Select all safe, clean. The Review installer must stay.
	m = drive(m, key("a"))
	m = drive(m, key("c"))
	if m.(Model).sc != scConfirm {
		t.Fatal("expected confirm screen")
	}
	m = drive(m, key("y"))
	if m.(Model).sc != scReport {
		t.Fatalf("expected report, got %v", m.(Model).sc)
	}
	if exists(filepath.Join(home, "Library/Caches/com.foo.app")) || exists(filepath.Join(home, "Library/Logs/old")) {
		t.Error("safe items should be deleted")
	}
	if !exists(filepath.Join(home, "Downloads/setup.dmg")) {
		t.Error("unselected review item must survive")
	}
}

func TestDenyAllLeavesEverything(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, "Library/Caches/com.foo.app/blob"), 3_000_000)
	var m tea.Model = New(home, "test", false, "")
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = drive(m, key("2"))
	m = drive(m, key("a"))
	m = drive(m, key("q")) // deny all
	if m.(Model).sc != scMenu || !exists(filepath.Join(home, "Library/Caches/com.foo.app")) {
		t.Fatal("deny must return to menu and remove nothing")
	}
}

func TestReviewDeleteNeedsTypedConfirm(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, "Downloads/setup.dmg"), 2_000_000)
	os.Chtimes(filepath.Join(home, "Downloads/setup.dmg"), oldTime(), oldTime())
	var m tea.Model = New(home, "test", false, "")
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = drive(m, key("2"))
	m = drive(m, key("A")) // select everything
	m = drive(m, key("down"))
	m = drive(m, key("t")) // flip installer to delete for good
	m = drive(m, key("c"))
	m = drive(m, key("y")) // y must not confirm when typing is required
	if m.(Model).sc != scConfirm {
		t.Fatal("review item set to delete must require typing")
	}
}

func TestDuplicatesKeepOneCopy(t *testing.T) {
	home := t.TempDir()
	for _, n := range []string{"a.bin", "b.bin", "c.bin"} {
		write(t, filepath.Join(home, "Documents", n), 1_500_000)
	}
	write(t, filepath.Join(home, "Documents", "other.bin"), 1_600_000)
	var m tea.Model = New(home, "test", false, "")
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = drive(m, key("5"))
	mm := m.(Model)
	if len(mm.roots) != 1 || mm.roots[0].title != "Duplicates" {
		t.Fatalf("expected one Duplicates category, got %d roots", len(mm.roots))
	}
	m = drive(m, key("A"))
	t0 := sumSelected(m.(Model).roots)
	if t0.count() != 2 {
		t.Fatalf("A must select all but one copy, selected %d", t0.count())
	}
}

func TestCheckedItemsInsideOtherItemsAreDropped(t *testing.T) {
	home := "/h"
	items := []*model.Item{{Path: "/h/a"}, {Path: "/h/a/b"}, {Path: "/h/c"}}
	if got := scan.Finalize(items, home); len(got) != 2 {
		t.Fatalf("nested item should be dropped, got %d", len(got))
	}
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func oldTime() time.Time { return time.Now().AddDate(0, 0, -30) }
