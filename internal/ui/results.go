package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mac-cleaner-cli/internal/model"
)

const rightW = 26

func (m Model) listHeight() int {
	h := m.h - 11
	if h < 5 {
		h = 5
	}
	return h
}

func (m *Model) reflatten() {
	var keep *node
	if m.cur < len(m.flat) {
		keep = m.flat[m.cur]
	}
	m.flat = flatten(m.roots)
	for i, n := range m.flat {
		if n == keep {
			m.cur = i
			return
		}
	}
	if m.cur >= len(m.flat) {
		m.cur = len(m.flat) - 1
	}
	if m.cur < 0 {
		m.cur = 0
	}
}

func (m *Model) clampScroll() {
	h := m.listHeight()
	if m.cur < m.off {
		m.off = m.cur
	}
	if m.cur >= m.off+h {
		m.off = m.cur - h + 1
	}
}

func (m Model) keyResults(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.flat) == 0 {
		m.sc = scMenu
		m.mode = ""
		return m, nil
	}
	m.flash = ""
	n := m.flat[m.cur]
	switch k.String() {
	case "up", "k":
		if m.cur > 0 {
			m.cur--
		}
	case "down", "j":
		if m.cur < len(m.flat)-1 {
			m.cur++
		}
	case "pgup":
		m.cur -= m.listHeight()
	case "pgdown":
		m.cur += m.listHeight()
	case "g", "home":
		m.cur = 0
	case "G", "end":
		m.cur = len(m.flat) - 1
	case " ", "x":
		m.flash = toggle(n)
	case "enter", "right", "l":
		if n.kind != kItem {
			n.open = !n.open
			m.reflatten()
		} else {
			m.flash = toggle(n)
		}
	case "left", "h":
		if n.kind != kItem && n.open {
			n.open = false
		} else if n.parent != nil {
			n.parent.open = false
			for i, f := range m.flat {
				if f == n.parent {
					m.cur = i
				}
			}
		}
		m.reflatten()
	case "a":
		selectSafe(m.roots)
		m.flash = "Selected every safe item. Review items stay as you set them."
	case "A":
		selectEverything(m.roots)
		m.flash = "Selected everything (one copy of each duplicate is kept)."
	case "n":
		clearAll(m.roots)
		m.flash = "Cleared. Nothing is selected."
	case "t":
		for _, it := range n.leaves() {
			it.Action = it.Action.Toggle()
		}
	case "c":
		t := sumSelected(m.roots)
		if t.count() == 0 {
			m.flash = "Nothing selected. Press space on an item, or a to select all safe items."
		} else {
			m.typed = ""
			m.sc = scConfirm
		}
	case "q", "esc":
		clearAll(m.roots)
		m.sc = scMenu
		m.mode = ""
	}
	if m.cur < 0 {
		m.cur = 0
	}
	if m.cur > len(m.flat)-1 {
		m.cur = len(m.flat) - 1
	}
	m.clampScroll()
	return m, nil
}

func checkbox(state int) string {
	switch state {
	case 2:
		return sOK.Render("[x]")
	case 1:
		return sWarn.Render("[-]")
	}
	return sDim.Render("[ ]")
}

func (m Model) row(n *node, cursor bool) string {
	cur := "  "
	if cursor {
		cur = sAccent.Render("▌ ")
	}
	indent := 0
	switch n.kind {
	case kGroup:
		indent = 2
	case kItem:
		if n.parent.kind == kGroup {
			indent = 4
		} else {
			indent = 2
		}
	}
	labelW := m.w - 2 - 4 - indent - 2 - rightW - 1
	if labelW < 12 {
		labelW = 12
	}
	if n.kind == kItem {
		it := n.item
		label := trunc(n.title, labelW)
		if cursor {
			label = sBold.Render(pad(label, labelW))
		} else {
			label = pad(label, labelW)
		}
		risk := sOK.Render(pad("safe", 6))
		if it.Risk == model.Review {
			risk = sWarn.Render(pad("review", 6))
		}
		act := sDim.Render(pad("trash", 6))
		if it.Action == model.Delete {
			act = sBad.Render(pad("delete", 6))
		}
		st := 0
		if it.Selected {
			st = 2
		}
		return fmt.Sprintf("%s%s%s %s %s  %s  %s", cur, strings.Repeat(" ", indent), checkbox(st), label, rpad(human(it.Size), 9), risk, act)
	}
	arrow := "▸"
	if n.open {
		arrow = "▾"
	}
	count := plural(len(n.leaves()), "item", "items")
	if n.kind == kGroup {
		count = fmt.Sprintf("keeps 1 of %d", len(n.kids))
	}
	label := trunc(n.title, labelW-2)
	label = pad(label, labelW-2)
	if n.kind == kCat {
		label = sBold.Render(label)
	}
	return fmt.Sprintf("%s%s%s %s %s %s  %s", cur, strings.Repeat(" ", indent), checkbox(n.state()), sDim.Render(arrow), label, rpad(human(n.reclaim()), 9), sDim.Render(pad(count, 14)))
}

func (m Model) viewResults() string {
	var b strings.Builder
	title := "Review"
	for _, e := range menu {
		if e.mode == m.mode {
			title = e.title
		}
	}
	b.WriteString(m.header(title))
	if len(m.flat) == 0 {
		b.WriteString("\n  " + sOK.Render("Nothing to clean here.") + "\n")
		b.WriteString(sDim.Render("\n  Press any key to go back.") + "\n")
		return b.String()
	}
	var all int64
	for _, r := range m.roots {
		all += r.reclaim()
	}
	b.WriteString(sDim.Render(fmt.Sprintf("  %s can be freed across %d groups. Nothing is selected until you choose.", human(all), len(m.roots))) + "\n\n")

	h := m.listHeight()
	end := m.off + h
	if end > len(m.flat) {
		end = len(m.flat)
	}
	for i := m.off; i < end; i++ {
		b.WriteString(m.row(m.flat[i], i == m.cur) + "\n")
	}
	for i := end - m.off; i < h; i++ {
		b.WriteString("\n")
	}

	n := m.flat[m.cur]
	detail := ""
	if n.kind == kItem {
		detail = truncLeft(n.item.Path, m.w-4) + "\n  " + sDim.Render(trunc(n.item.Note, m.w-4))
		detail = sDim.Render(truncLeft(n.item.Path, m.w-4)) + "\n  " + sDim.Render(trunc(n.item.Note, m.w-4))
	} else {
		detail = "\n  "
	}
	t := sumSelected(m.roots)
	b.WriteString("\n  " + detail + "\n")
	sum := sDim.Render("  Selected: none")
	if t.count() > 0 {
		sum = fmt.Sprintf("  Selected: %s · %s  (%s to Trash, %s delete for good)", sBold.Render(plural(t.count(), "item", "items")),
			sBold.Render(human(t.size())), human(t.trash), sBad.Render(human(t.del)))
	}
	b.WriteString(sum + "\n")
	if m.flash != "" {
		b.WriteString("  " + sWarn.Render(m.flash) + "\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString("  " + hint("↑↓", "move") + sep + hint("space", "tick / untick") + sep + hint("a", "tick all safe") + sep + hint("A", "tick all") + sep + hint("n", "untick all") + "\n")
	clean := hint("c", "clean ticked items")
	if t.count() > 0 {
		clean = sOK.Render(sBold.Render("c")) + " " + sOK.Render("clean ticked items")
	}
	line := "  " + clean + sep + hint("t", "trash ⇄ delete")
	if n.kind != kItem {
		line += sep + hint("enter", "open / close")
	}
	line += sep + hint("q", "cancel, touch nothing")
	b.WriteString(line + "\n")
	return b.String()
}

// ---- confirm ----

func (m Model) keyConfirm(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	t := sumSelected(m.roots)
	needTyped := t.riskyDelN > 0
	s := k.String()
	if s == "esc" || (!needTyped && (s == "n" || s == "q")) {
		m.sc = scResults
		return m, nil
	}
	if needTyped {
		switch s {
		case "enter":
			if strings.EqualFold(m.typed, "delete") {
				return m.startRun()
			}
		case "backspace":
			if len(m.typed) > 0 {
				m.typed = m.typed[:len(m.typed)-1]
			}
		default:
			if k.Type == tea.KeyRunes && len(m.typed) < 12 {
				m.typed += string(k.Runes)
			}
		}
		return m, nil
	}
	if s == "y" || s == "enter" {
		return m.startRun()
	}
	return m, nil
}

func (m Model) viewConfirm() string {
	t := sumSelected(m.roots)
	var b strings.Builder
	b.WriteString(m.header("Confirm"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Move to Trash    %s   %s   %s\n", rpad(plural(t.trashN, "item", "items"), 10), sBold.Render(rpad(human(t.trash), 9)), sDim.Render("restorable; space returns when you empty the Trash")))
	b.WriteString(fmt.Sprintf("  Delete for good  %s   %s   %s\n", rpad(plural(t.delN, "item", "items"), 10), sBad.Render(rpad(human(t.del), 9)), sDim.Render("cannot be undone")))

	var top []*model.Item
	for _, r := range m.roots {
		for _, it := range r.leaves() {
			if it.Selected {
				top = append(top, it)
			}
		}
	}
	b.WriteString("\n" + sDim.Render("  Largest selected:") + "\n")
	shown := 0
	for shown < 6 && len(top) > 0 {
		bi := 0
		for i, it := range top {
			if it.Size > top[bi].Size {
				bi = i
			}
		}
		it := top[bi]
		top = append(top[:bi], top[bi+1:]...)
		act := sDim.Render("trash ")
		if it.Action == model.Delete {
			act = sBad.Render("delete")
		}
		b.WriteString(fmt.Sprintf("    %s %s  %s\n", act, rpad(human(it.Size), 9), trunc(it.Label, m.w-24)))
		shown++
	}
	if rest := t.count() - shown; rest > 0 {
		b.WriteString(sDim.Render(fmt.Sprintf("    … and %d more\n", rest)))
	}
	if m.hasTrashEmpty() && t.trashN > 0 {
		b.WriteString("\n  " + sWarn.Render("Emptying the Trash runs last and also erases what this run just moved there.") + "\n")
	}
	b.WriteString("\n")
	if t.riskyDelN > 0 {
		b.WriteString("  " + sWarn.Render(plural(t.riskyDelN, "review item is", "review items are")+" set to delete for good.") + "\n")
		b.WriteString("  Type " + sBold.Render("delete") + " and press enter to continue: " + sBold.Render(m.typed) + sAccent.Render("▏") + "\n")
		b.WriteString(sDim.Render("\n  esc go back (tip: press t on an item to send it to the Trash instead)") + "\n")
	} else {
		verb := "clean"
		if m.dry {
			verb = "run the dry run"
		}
		b.WriteString("  Press " + sBold.Render("y") + " to " + verb + ", or " + sBold.Render("n") + " to go back.\n")
	}
	return b.String()
}

func (m Model) hasTrashEmpty() bool {
	for _, r := range m.roots {
		for _, it := range r.leaves() {
			if it.Selected && it.Category == "Trash" {
				return true
			}
		}
	}
	return false
}

// hint renders one footer hint: a bold key followed by a dim description.
func hint(k, desc string) string { return sBold.Render(k) + " " + sDim.Render(desc) }

var sep = sDim.Render("  ·  ")
