// Package model holds the types shared between scanners, the delete engine and the UI.
package model

import "time"

// Risk tells the user how careful to be with an item.
type Risk int

const (
	Safe   Risk = iota // regenerates by itself; removing it loses nothing
	Review             // may hold something the user still wants
)

func (r Risk) String() string {
	if r == Safe {
		return "safe"
	}
	return "review"
}

// Action is what happens to an item when the user confirms.
type Action int

const (
	Trash  Action = iota // move to ~/.Trash, restorable
	Delete               // remove for good
)

func (a Action) String() string {
	if a == Trash {
		return "trash"
	}
	return "delete"
}

// Toggle flips between Trash and Delete.
func (a Action) Toggle() Action {
	if a == Trash {
		return Delete
	}
	return Trash
}

// Item is one removable thing: a folder, a file, or (for the Trash) a container.
type Item struct {
	Path     string
	Label    string
	Category string
	Note     string
	Size     int64
	Risk     Risk
	Action   Action
	Selected bool
	ModTime  time.Time

	// Duplicates only: items sharing GroupKey are copies of the same content.
	GroupKey string
	Group    string
}
