package ui

import (
	"sort"

	"mac-cleaner-cli/internal/model"
)

type kind int

const (
	kCat kind = iota
	kGroup
	kItem
)

type node struct {
	kind   kind
	title  string
	item   *model.Item
	kids   []*node
	open   bool
	parent *node
}

var catOrder = []string{"App caches", "Browsers", "Logs", "Old installers", "Developer caches", "Xcode",
	"Project artifacts", "Device backups", "Mail downloads", "Large files", "Trash", "Duplicates"}

func buildTree(items []*model.Item) []*node {
	by := map[string][]*model.Item{}
	for _, it := range items {
		by[it.Category] = append(by[it.Category], it)
	}
	var cats []string
	seen := map[string]bool{}
	for _, c := range catOrder {
		if len(by[c]) > 0 {
			cats = append(cats, c)
			seen[c] = true
		}
	}
	var rest []string
	for c := range by {
		if !seen[c] {
			rest = append(rest, c)
		}
	}
	sort.Strings(rest)
	cats = append(cats, rest...)

	var roots []*node
	for _, c := range cats {
		root := &node{kind: kCat, title: c, open: true}
		list := by[c]
		if c == "Duplicates" {
			groups := map[string][]*model.Item{}
			var keys []string
			for _, it := range list {
				if _, ok := groups[it.GroupKey]; !ok {
					keys = append(keys, it.GroupKey)
				}
				groups[it.GroupKey] = append(groups[it.GroupKey], it)
			}
			reclaim := func(k string) int64 { g := groups[k]; return g[0].Size * int64(len(g)-1) }
			sort.Slice(keys, func(i, j int) bool { return reclaim(keys[i]) > reclaim(keys[j]) })
			for _, k := range keys {
				g := groups[k]
				sort.SliceStable(g, func(i, j int) bool { return g[i].ModTime.Before(g[j].ModTime) })
				gn := &node{kind: kGroup, title: g[0].Group, parent: root}
				for _, it := range g {
					gn.kids = append(gn.kids, &node{kind: kItem, title: it.Label, item: it, parent: gn})
				}
				root.kids = append(root.kids, gn)
			}
		} else {
			sort.SliceStable(list, func(i, j int) bool { return list[i].Size > list[j].Size })
			for _, it := range list {
				root.kids = append(root.kids, &node{kind: kItem, title: it.Label, item: it, parent: root})
			}
		}
		roots = append(roots, root)
	}
	return roots
}

func flatten(roots []*node) []*node {
	var out []*node
	var walk func(n *node)
	walk = func(n *node) {
		out = append(out, n)
		if n.kind != kItem && n.open {
			for _, k := range n.kids {
				walk(k)
			}
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return out
}

// leaves returns every item under n.
func (n *node) leaves() []*model.Item {
	if n.kind == kItem {
		return []*model.Item{n.item}
	}
	var out []*model.Item
	for _, k := range n.kids {
		out = append(out, k.leaves()...)
	}
	return out
}

// reclaim is what removing the whole node would free. A duplicate group keeps one copy.
func (n *node) reclaim() int64 {
	switch n.kind {
	case kItem:
		return n.item.Size
	case kGroup:
		var s int64
		for _, k := range n.kids {
			s += k.item.Size
		}
		return s - n.kids[0].item.Size
	}
	var s int64
	for _, k := range n.kids {
		s += k.reclaim()
	}
	return s
}

// state: 0 none, 1 some, 2 all selected.
func (n *node) state() int {
	ls := n.leaves()
	c := 0
	for _, it := range ls {
		if it.Selected {
			c++
		}
	}
	switch {
	case c == 0:
		return 0
	case c == len(ls):
		return 2
	}
	return 1
}

// lastCopy reports whether selecting it would remove every copy in its duplicate group.
func lastCopy(n *node) bool {
	if n.kind != kItem || n.parent == nil || n.parent.kind != kGroup || n.item.Selected {
		return false
	}
	sel := 0
	for _, k := range n.parent.kids {
		if k.item.Selected {
			sel++
		}
	}
	return sel >= len(n.parent.kids)-1
}

// toggle flips a node. It returns a message when part of the request was refused.
func toggle(n *node) string {
	switch n.kind {
	case kItem:
		if lastCopy(n) {
			return "Keep at least one copy of each duplicate"
		}
		n.item.Selected = !n.item.Selected
		return ""
	case kGroup:
		if n.state() == 2 || n.state() == 1 && selectedCount(n) == len(n.kids)-1 {
			setAll(n, false)
		} else {
			for i, k := range n.kids {
				k.item.Selected = i > 0 // the oldest copy is kept
			}
		}
		return ""
	}
	if n.state() == 2 {
		setAll(n, false)
		return ""
	}
	for _, k := range n.kids {
		if k.kind == kGroup {
			for i, g := range k.kids {
				g.item.Selected = i > 0
			}
		} else {
			k.item.Selected = true
		}
	}
	return ""
}

func selectedCount(n *node) int {
	c := 0
	for _, it := range n.leaves() {
		if it.Selected {
			c++
		}
	}
	return c
}

func setAll(n *node, v bool) {
	for _, it := range n.leaves() {
		it.Selected = v
	}
}

// selectSafe selects every Safe item (never duplicates, which are always Review).
func selectSafe(roots []*node) {
	for _, r := range roots {
		for _, it := range r.leaves() {
			if it.Risk == model.Safe {
				it.Selected = true
			}
		}
	}
}

func selectEverything(roots []*node) {
	for _, r := range roots {
		for _, k := range r.kids {
			if k.kind == kGroup {
				for i, g := range k.kids {
					g.item.Selected = i > 0
				}
			} else {
				k.item.Selected = true
			}
		}
	}
}

func clearAll(roots []*node) {
	for _, r := range roots {
		setAll(r, false)
	}
}

type totals struct {
	trashN, delN, riskyDelN int
	trash, del              int64
}

func (t totals) count() int  { return t.trashN + t.delN }
func (t totals) size() int64 { return t.trash + t.del }

func sumSelected(roots []*node) totals {
	var t totals
	for _, r := range roots {
		for _, it := range r.leaves() {
			if !it.Selected {
				continue
			}
			if it.Action == model.Trash {
				t.trashN++
				t.trash += it.Size
			} else {
				t.delN++
				t.del += it.Size
				if it.Risk == model.Review {
					t.riskyDelN++
				}
			}
		}
	}
	return t
}
