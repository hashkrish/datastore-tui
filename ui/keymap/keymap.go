// Package keymap centralizes the vim-style key.Binding sets for each UI mode
// (browse/detail/edit/filter) and a small chord tracker for two-key
// sequences like "gg" and "dd" that bubbles/key does not model directly.
package keymap

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
)

// Mode selects which keymap is active; the app model switches modes as the
// user drills into columns, opens the detail view, or starts editing.
type Mode int

const (
	ModeBrowse Mode = iota
	ModeDetail
	ModeEdit
	ModeFilter
)

// BrowseKeyMap covers the ranger-style Miller-column browsing mode.
type BrowseKeyMap struct {
	Up, Down                 key.Binding
	Left, Right              key.Binding
	Top, Bottom              key.Binding
	HalfPageUp, HalfPageDown key.Binding
	Filter                   key.Binding
	DeleteMark               key.Binding // first "d" of "dd"
	Add                      key.Binding
	Refresh                  key.Binding
	Open                     key.Binding
	ListBookmarks            key.Binding // ctrl+l: open the bookmark picker
	ToggleBookmark           key.Binding // ctrl+b: bookmark/unbookmark the highlighted entity
	Query                    key.Binding // f: filter the current kind's entities by a property (AND-combines if pressed again)
	ClearFilters             key.Binding // C: clear all active AND filters
	Order                    key.Binding // O: order the current kind's entities by a property
	FindReferences           key.Binding // ctrl+f: query another kind by this entity's key (clears any active query)
	FindReferencesAdd        key.Binding // F: same, but AND-combines onto the active query instead of clearing it
	Yank                     key.Binding // first "y" of "yy": copy the selected entity's key
	ToggleTable              key.Binding // T: toggle table (spreadsheet) view of the current kind's entities
	Quit                     key.Binding
	Help                     key.Binding
	PendingG                 key.Binding // first "g" of "gg"
}

// DefaultBrowseKeyMap returns the standard vim bindings for browse mode.
func DefaultBrowseKeyMap() BrowseKeyMap {
	return BrowseKeyMap{
		Up:                key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:              key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Left:              key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "back")),
		Right:             key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "drill in")),
		Top:               key.NewBinding(key.WithKeys("g"), key.WithHelp("gg", "top")),
		Bottom:            key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		HalfPageUp:        key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "half page up")),
		HalfPageDown:      key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "half page down")),
		Filter:            key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		DeleteMark:        key.NewBinding(key.WithKeys("d"), key.WithHelp("dd", "delete")),
		Add:               key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "new entity")),
		Refresh:           key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
		Open:              key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		ListBookmarks:     key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "bookmarks")),
		ToggleBookmark:    key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("ctrl+b", "bookmark")),
		Query:             key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "query")),
		ClearFilters:      key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "clear filters")),
		Order:             key.NewBinding(key.WithKeys("O"), key.WithHelp("O", "order")),
		FindReferences:    key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "find references")),
		FindReferencesAdd: key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "find references (add)")),
		Yank:              key.NewBinding(key.WithKeys("y"), key.WithHelp("yy", "copy key")),
		ToggleTable:       key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "table view")),
		Quit:              key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:              key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
}

// DetailKeyMap covers the property-tree detail/edit-entry mode.
type DetailKeyMap struct {
	Up, Down                 key.Binding
	HalfPageUp, HalfPageDown key.Binding
	Expand                   key.Binding // l/enter on array or embedded entity: descend
	Back                     key.Binding // h/esc: back out one nesting level, or to browse
	Edit                     key.Binding // enter on a scalar leaf: open edit form
	Retype                   key.Binding // t: change the selected property's data type
	Filter                   key.Binding // "/" filter the current scope's rows
	AddItem                  key.Binding // o on an array: append item
	DeleteItem               key.Binding // dd on an array item: remove it
	Save                     key.Binding // ctrl+s: commit pending edits
	Refresh                  key.Binding // r: reload the current entity from the database
	GoToKey                  key.Binding // ctrl+]: open the entity a Key-typed property points at
	ToggleBookmark           key.Binding // ctrl+b: bookmark/unbookmark the current entity
	ListBookmarks            key.Binding // ctrl+l: open the bookmark picker
	FindReferences           key.Binding // ctrl+f: query another kind by this property's value (clears any active query)
	FindReferencesAdd        key.Binding // F: same, but AND-combines onto the active query instead of clearing it
	Yank                     key.Binding // first "y" of "yy": copy the selected property's value
	Quit                     key.Binding
	Help                     key.Binding
}

// DefaultDetailKeyMap returns the standard vim bindings for detail mode.
func DefaultDetailKeyMap() DetailKeyMap {
	return DetailKeyMap{
		Up:                key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:              key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		HalfPageUp:        key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "half page up")),
		HalfPageDown:      key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "half page down")),
		Expand:            key.NewBinding(key.WithKeys("l", "right", "enter"), key.WithHelp("l/enter", "expand/edit")),
		Back:              key.NewBinding(key.WithKeys("h", "left", "esc"), key.WithHelp("h/esc", "back")),
		Edit:              key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
		Retype:            key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "change type")),
		Filter:            key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		AddItem:           key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "add item")),
		DeleteItem:        key.NewBinding(key.WithKeys("d"), key.WithHelp("dd", "delete item")),
		Save:              key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "save")),
		Refresh:           key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		GoToKey:           key.NewBinding(key.WithKeys("ctrl+]"), key.WithHelp("ctrl+]", "go to key")),
		ToggleBookmark:    key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("ctrl+b", "bookmark")),
		ListBookmarks:     key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "bookmarks")),
		FindReferences:    key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "find references")),
		FindReferencesAdd: key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "find references (add)")),
		Yank:              key.NewBinding(key.WithKeys("y"), key.WithHelp("yy", "copy value")),
		Quit:              key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "back to browse")),
		Help:              key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
}

// TableKeyMap covers the spreadsheet-like grid mode ("T" from browse):
// entities as rows, properties as columns. h/l scroll columns rather than
// back/drill-in, and there's no Query/Order of its own (v1 scope), which is
// why this is its own struct instead of reusing BrowseKeyMap.
type TableKeyMap struct {
	Up, Down                 key.Binding
	Left, Right              key.Binding
	Top, Bottom              key.Binding
	HalfPageUp, HalfPageDown key.Binding
	Open                     key.Binding // enter: open the selected row's entity in detail view
	Yank                     key.Binding // first "y" of "yy": copy the selected cell's value
	FilterColumns            key.Binding // *: filter which columns (properties) are shown, by name
	Search                   key.Binding // /: search every visible cell's value, jump to the first match
	NextMatch                key.Binding // n: jump to the next search match
	PrevMatch                key.Binding // N: jump to the previous search match
	Toggle                   key.Binding // T: back to browse
	Back                     key.Binding // q/esc: back to browse (table is a child screen of browse, like detail mode — "q" here must not quit the app)
	Quit                     key.Binding // ctrl+c: actually quit the app
	Help                     key.Binding
}

// DefaultTableKeyMap returns the standard vim bindings for table mode.
func DefaultTableKeyMap() TableKeyMap {
	return TableKeyMap{
		Up:            key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:          key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Left:          key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "scroll left")),
		Right:         key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "scroll right")),
		Top:           key.NewBinding(key.WithKeys("g"), key.WithHelp("gg", "top")),
		Bottom:        key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		HalfPageUp:    key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "half page up")),
		HalfPageDown:  key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "half page down")),
		Open:          key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open entity")),
		Yank:          key.NewBinding(key.WithKeys("y"), key.WithHelp("yy", "copy cell value")),
		FilterColumns: key.NewBinding(key.WithKeys("*"), key.WithHelp("*", "filter columns")),
		Search:        key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search cells")),
		NextMatch:     key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next match")),
		PrevMatch:     key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "previous match")),
		Toggle:        key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "back to browse")),
		Back:          key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "back to browse")),
		Quit:          key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		Help:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
}

// TabKeyMap covers tab management (opening/closing/switching independent
// browse sessions), active alongside BrowseKeyMap/DetailKeyMap/TableKeyMap —
// see app.handleTabKey, which only consults it while the active tab sits on
// one of the "stable" screens (browse/detail/table), not mid-form.
type TabKeyMap struct {
	NewTab   key.Binding // ctrl+t: open a new tab
	CloseTab key.Binding // ctrl+w: close the active tab
	NextTab  key.Binding // tab: cycle to the next tab
	PrevTab  key.Binding // shift+tab: cycle to the previous tab
	// Jumping directly to a tab ("1"-"9") is matched by raw digit runes
	// rather than a key.Binding — see app.handleTabKey.
}

// DefaultTabKeyMap returns the standard tab-management bindings.
func DefaultTabKeyMap() TabKeyMap {
	return TabKeyMap{
		NewTab:   key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "new tab")),
		CloseTab: key.NewBinding(key.WithKeys("ctrl+w"), key.WithHelp("ctrl+w", "close tab")),
		NextTab:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
		PrevTab:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous tab")),
	}
}

// FilterKeyMap covers the in-column "/" incremental filter input.
type FilterKeyMap struct {
	Confirm key.Binding
	Cancel  key.Binding
}

// DefaultFilterKeyMap returns the standard bindings for filter-input mode.
func DefaultFilterKeyMap() FilterKeyMap {
	return FilterKeyMap{
		Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}

// chordWindow is how long a leading chord key ("g" or "d") stays armed
// waiting for its second key before it's treated as a stray keystroke.
const chordWindow = 600 * time.Millisecond

// Chord tracks two-key vim sequences ("gg", "dd") that a flat key.Binding
// set can't express: it remembers the last chord-starting key pressed and
// how long ago, so the caller can ask "does this next key complete a chord?"
type Chord struct {
	pending rune
	armedAt time.Time
}

// Arm records key as the first half of a potential chord.
func (c *Chord) Arm(key rune) {
	c.pending = key
	c.armedAt = time.Now()
}

// Complete reports whether key completes the currently armed chord (i.e.
// key == the armed key, pressed within chordWindow), consuming the arm
// state either way.
func (c *Chord) Complete(key rune) bool {
	defer c.Reset()
	if c.pending == 0 {
		return false
	}
	if time.Since(c.armedAt) > chordWindow {
		return false
	}
	return c.pending == key
}

// Reset disarms any pending chord.
func (c *Chord) Reset() {
	c.pending = 0
}
