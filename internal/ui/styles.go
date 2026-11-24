package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	cAccent = lipgloss.Color("39")
	cOK     = lipgloss.Color("42")
	cWarn   = lipgloss.Color("214")
	cBad    = lipgloss.Color("203")
	cDim    = lipgloss.Color("244")

	sTitle  = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	sBold   = lipgloss.NewStyle().Bold(true)
	sDim    = lipgloss.NewStyle().Foreground(cDim)
	sOK     = lipgloss.NewStyle().Foreground(cOK)
	sWarn   = lipgloss.NewStyle().Foreground(cWarn)
	sBad    = lipgloss.NewStyle().Foreground(cBad)
	sAccent = lipgloss.NewStyle().Foreground(cAccent)
)

// human formats bytes the way Finder does (powers of 1000).
func human(n int64) string {
	if n < 0 {
		return "…"
	}
	const u = 1000
	if n < u {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(u), 0
	for m := n / u; m >= u; m /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTP"[exp])
}

func trunc(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

// truncLeft keeps the end of a path, which is the informative part.
func truncLeft(s string, n int) string {
	r := []rune(s)
	if len(r) <= n || n < 2 {
		return s
	}
	return "…" + string(r[len(r)-n+1:])
}

func bar(frac float64, w int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	f := int(frac*float64(w) + 0.5)
	return sAccent.Render(strings.Repeat("█", f)) + sDim.Render(strings.Repeat("░", w-f))
}

func pad(s string, w int) string {
	if n := w - len([]rune(s)); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func rpad(s string, w int) string {
	if n := w - len([]rune(s)); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func tilde(home, p string) string {
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+"/") {
		return "~" + p[len(home):]
	}
	return p
}

// signed formats a byte delta with an explicit sign.
func signed(n int64) string {
	if n < 0 {
		return "-" + human(-n)
	}
	return "+" + human(n)
}
