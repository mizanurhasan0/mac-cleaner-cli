// mac-cleaner-cli: a light, safe terminal cleaner for macOS.
// Scan, review, choose per item, then clean or deny.
//
// Copyright (c) 2026 Mizanur Hasan. Released under the MIT License.
// Contact: eng.mizanur.hasan@gmail.com · https://www.linkedin.com/in/mizanur-hasan/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mac-cleaner-cli/internal/model"
	"mac-cleaner-cli/internal/scan"
	"mac-cleaner-cli/internal/ui"
)

var version = "0.1.0"

// about is shown by --version.
const about = `Copyright (c) 2026 Mizanur Hasan. MIT License.
eng.mizanur.hasan@gmail.com · https://www.linkedin.com/in/mizanur-hasan/`

const usage = `mac-cleaner-cli %s: scan, choose what to remove, then clean or deny.

Usage:
  mac-cleaner-cli [mode] [flags]

Modes (omit to open the menu):
  full    quick + deep + dev in one pass
  quick   app and browser caches, logs, old installers
  deep    device backups, Mail downloads, Trash, large files
  dev     node_modules, build output, package-manager and Xcode caches
  dupes   identical files in Downloads, Documents, Desktop, media
  disk    disk doctor: see what uses space (read-only)

Flags:
`

func main() {
	dry := flag.Bool("dry-run", false, "go through the whole flow but remove nothing")
	plain := flag.Bool("plain", false, "print scan results as text and exit (read-only, no TUI)")
	ver := flag.Bool("version", false, "print version")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, version)
		flag.PrintDefaults()
	}
	flag.CommandLine.Parse(flagsFirst(os.Args[1:]))
	if *ver {
		fmt.Printf("mac-cleaner-cli %s\n%s\n", version, about)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" || home == "/" {
		fmt.Fprintln(os.Stderr, "mac-cleaner-cli: cannot find your home folder")
		os.Exit(1)
	}
	if os.Geteuid() == 0 {
		fmt.Fprintln(os.Stderr, "mac-cleaner-cli: do not run as root. It only cleans your own files.")
		os.Exit(1)
	}

	var mode scan.Mode
	if flag.NArg() > 0 {
		mode = scan.Mode(strings.ToLower(flag.Arg(0)))
		switch mode {
		case scan.Full, scan.Quick, scan.Deep, scan.Dev, scan.Dupes, scan.Disk:
		default:
			fmt.Fprintf(os.Stderr, "mac-cleaner-cli: unknown mode %q\n\n", flag.Arg(0))
			flag.Usage()
			os.Exit(2)
		}
	}

	if *plain {
		if mode == "" || mode == scan.Disk {
			fmt.Fprintln(os.Stderr, "mac-cleaner-cli: --plain needs one of: full quick deep dev dupes")
			os.Exit(2)
		}
		runPlain(home, mode)
		return
	}

	p := tea.NewProgram(ui.New(home, version, *dry, mode), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mac-cleaner-cli:", err)
		os.Exit(1)
	}
}

// flagsFirst moves flags ahead of positional args so "mac-cleaner-cli dev --plain" works
// as well as "mac-cleaner-cli --plain dev". All flags are boolean, so none take a value.
func flagsFirst(args []string) []string {
	var flags, rest []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
		} else {
			rest = append(rest, a)
		}
	}
	return append(flags, rest...)
}

func runPlain(home string, mode scan.Mode) {
	var items []*model.Item
	for ev := range scan.Start(context.Background(), mode, home) {
		items = append(items, ev.Items...)
	}
	items = scan.Finalize(items, home)
	by := map[string][]*model.Item{}
	var cats []string
	for _, it := range items {
		if _, ok := by[it.Category]; !ok {
			cats = append(cats, it.Category)
		}
		by[it.Category] = append(by[it.Category], it)
	}
	sort.Strings(cats)
	var grand int64
	for _, c := range cats {
		list := by[c]
		sort.Slice(list, func(i, j int) bool { return list[i].Size > list[j].Size })
		var sum int64
		for _, it := range list {
			sum += it.Size
		}
		grand += sum
		fmt.Printf("\n%s  (%d items, %s)\n", c, len(list), fmtSize(sum))
		for i, it := range list {
			if i == 8 {
				fmt.Printf("   … %d more\n", len(list)-8)
				break
			}
			fmt.Printf("   %9s  %-6s %-6s %s\n", fmtSize(it.Size), it.Risk, it.Action, it.Label)
		}
	}
	fmt.Printf("\nTotal found: %s (nothing was removed)\n", fmtSize(grand))
}

func fmtSize(n int64) string {
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
