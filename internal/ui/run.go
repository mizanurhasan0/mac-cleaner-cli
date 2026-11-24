package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mac-cleaner-cli/internal/clean"
	"mac-cleaner-cli/internal/model"
)

type runMsg struct {
	it  *model.Item
	err error
}

func (m Model) startRun() (tea.Model, tea.Cmd) {
	var q []*model.Item
	for _, r := range m.roots {
		for _, it := range r.leaves() {
			if it.Selected {
				q = append(q, it)
			}
		}
	}
	rank := func(it *model.Item) int {
		switch {
		case it.Category == "Trash":
			return 2 // emptied last
		case it.Action == model.Delete:
			return 1
		}
		return 0
	}
	sort.SliceStable(q, func(i, j int) bool { return rank(q[i]) < rank(q[j]) })
	m.queue, m.ri, m.freed, m.stop, m.failures, m.doneN = q, 0, 0, false, nil, 0
	m.runStart, m.recent = time.Now(), nil
	m.refreshVolume()
	m.freeBefore = m.free
	m.sc = scRun
	return m, m.runNext()
}

func (m Model) runNext() tea.Cmd {
	if m.ri >= len(m.queue) {
		return func() tea.Msg { return runMsg{} }
	}
	it, home, dry := m.queue[m.ri], m.home, m.dry
	return func() tea.Msg {
		err := clean.Execute(home, it, dry)
		status := "ok"
		if err != nil {
			status = err.Error()
		}
		clean.Log(home, clean.Record{Time: time.Now(), Path: it.Path, Action: it.Action.String(), Size: it.Size, Status: status, Dry: dry})
		return runMsg{it: it, err: err}
	}
}

func (m Model) onRun(msg runMsg) (tea.Model, tea.Cmd) {
	if m.sc != scRun {
		return m, nil
	}
	if msg.it != nil {
		m.ri++
		if msg.err != nil {
			m.failures = append(m.failures, failure{msg.it.Label, msg.err.Error()})
			m.recent = append(m.recent, sWarn.Render("!")+" "+msg.it.Label+"  "+sDim.Render("skipped"))
		} else {
			m.doneN++
			m.freed += msg.it.Size
			m.recent = append(m.recent, sOK.Render("✓")+" "+msg.it.Label+"  "+sDim.Render(human(msg.it.Size)))
		}
		if len(m.recent) > 5 {
			m.recent = m.recent[len(m.recent)-5:]
		}
	}
	if m.stop || m.ri >= len(m.queue) {
		m.refreshVolume()
		m.sc = scReport
		return m, nil
	}
	return m, m.runNext()
}

func (m Model) viewRun() string {
	var b strings.Builder
	b.WriteString(m.header("Cleaning"))
	total := len(m.queue)
	frac := 0.0
	if total > 0 {
		frac = float64(m.ri) / float64(total)
	}
	spin := sAccent.Render(spinner[m.frame%len(spinner)])
	elapsed := time.Since(m.runStart).Round(time.Second)

	if m.dry {
		b.WriteString("\n  " + sBold.Render("Dry run: checking only, nothing is being removed") + "\n")
	} else {
		b.WriteString("\n  " + sBold.Render("Cleaning in progress. Please wait.") + "\n")
	}
	b.WriteString(fmt.Sprintf("\n  %s  %3d%%   step %d of %d   %s\n", bar(frac, 30), int(frac*100), min(m.ri+1, total), total, sDim.Render(elapsed.String()+" elapsed")))

	if m.ri < total {
		it := m.queue[m.ri]
		verb := "Moving to Trash"
		if it.Action == model.Delete {
			verb = "Deleting"
		}
		if m.dry {
			verb = "Checking"
		}
		b.WriteString(fmt.Sprintf("\n  %s %s %s  %s\n", spin, sBold.Render(verb), trunc(it.Label, m.w/2), sDim.Render(human(it.Size))))
		b.WriteString("    " + sDim.Render(trunc(tilde(m.home, it.Path), m.w-6)) + "\n")
		if it.Size > 500_000_000 {
			b.WriteString("    " + sDim.Render("This one is large and can take a while. It is still working.") + "\n")
		}
	}

	if len(m.recent) > 0 {
		b.WriteString("\n")
		for _, r := range m.recent {
			b.WriteString("  " + r + "\n")
		}
	}
	b.WriteString(fmt.Sprintf("\n  %s processed so far\n", sBold.Render(human(m.freed))))
	if m.stop {
		b.WriteString("\n  " + sWarn.Render("Stopping after the current item…") + "\n")
	} else {
		b.WriteString(sDim.Render("\n  Keep this window open. esc stops after the current item.") + "\n")
	}
	return b.String()
}

func (m Model) viewReport() string {
	var b strings.Builder
	b.WriteString(m.header("Done"))
	b.WriteString("\n")
	verb := "Cleaned"
	if m.dry {
		verb = "Would clean"
	}
	b.WriteString(fmt.Sprintf("  %s  %s  in %s\n", sOK.Render("✓"), sBold.Render(verb+" "+human(m.freed)), plural(m.doneN, "item", "items")))
	if !m.dry {
		after := int64(m.free) - int64(m.freeBefore)
		b.WriteString(fmt.Sprintf("     Free space %s → %s (%s)\n", human(int64(m.freeBefore)), human(int64(m.free)), signed(after)))
		b.WriteString(sDim.Render("     Items sent to the Trash free their space once you empty the Trash.") + "\n")
	}
	if len(m.failures) > 0 {
		b.WriteString("\n  " + sWarn.Render(plural(len(m.failures), "item was", "items were")+" skipped:") + "\n")
		for i, f := range m.failures {
			if i == 6 {
				b.WriteString(sDim.Render(fmt.Sprintf("    … and %d more (see the log)\n", len(m.failures)-6)))
				break
			}
			b.WriteString(fmt.Sprintf("    %s  %s\n", trunc(f.path, m.w/2-4), sDim.Render(trunc(f.err, m.w/2-4))))
		}
	}
	b.WriteString(sDim.Render("\n  Log: "+clean.LogPath(m.home)) + "\n")
	b.WriteString(sDim.Render("\n  any key menu · q quit") + "\n")
	return b.String()
}
