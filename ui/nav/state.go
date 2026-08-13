// Package nav holds the navigation state for the Miller-column browse view
// (namespace/kind/entity selection, focus, per-column filters, pagination)
// and the property-path breadcrumb used once an entity's detail view is
// open. It is deliberately UI-framework-agnostic: ui/panes and app read and
// mutate this state, bubbletea's Model/Update/View lives above it.
package nav

import (
	"strings"

	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
)

// Column identifies one of the three Miller columns.
type Column int

const (
	ColumnNamespace Column = iota
	ColumnKind
	ColumnEntity
)

// columnList holds one column's items plus its selection and filter state.
// Namespace/Kind columns hold plain strings; the Entity column also keeps
// pagination state (populated via SetEntitiesPage instead of SetItems).
type columnList struct {
	items    []string
	selected int
	filter   string
}

func (c *columnList) visible() []string {
	if c.filter == "" {
		return c.items
	}
	out := make([]string, 0, len(c.items))
	for _, it := range c.items {
		if strings.Contains(strings.ToLower(it), strings.ToLower(c.filter)) {
			out = append(out, it)
		}
	}
	return out
}

func (c *columnList) selectedItem() (string, bool) {
	v := c.visible()
	if c.selected < 0 || c.selected >= len(v) {
		return "", false
	}
	return v[c.selected], true
}

func (c *columnList) moveBy(delta int) {
	n := len(c.visible())
	if n == 0 {
		c.selected = 0
		return
	}
	c.selected = clamp(c.selected+delta, 0, n-1)
}

func (c *columnList) moveToTop()    { c.selected = 0 }
func (c *columnList) moveToBottom() { c.selected = max(0, len(c.visible())-1) }

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// State is the full Miller-column navigation state.
type State struct {
	Focus Column

	namespaces columnList
	kinds      columnList
	entities   columnList

	loadedEntities []*model.Entity // parallels entities.items, in the same order
	entityCursor   string          // pagination cursor for the next page, "" if none fetched yet
	entityHasMore  bool
}

// NewState returns an empty navigation state, focused on the namespace
// column.
func NewState() *State {
	return &State{Focus: ColumnNamespace}
}

// SetNamespaces replaces the namespace column's contents (e.g. after a
// query.ListNamespaces call) and resets its selection/filter.
func (s *State) SetNamespaces(namespaces []string) {
	s.namespaces = columnList{items: namespaces}
}

// SetKinds replaces the kind column's contents and resets its
// selection/filter; call after the namespace selection changes.
func (s *State) SetKinds(kinds []string) {
	s.kinds = columnList{items: kinds}
}

// SetEntitiesPage replaces (append=false) or extends (append=true) the
// entity column's contents from a query.ListEntitiesPage result.
func (s *State) SetEntitiesPage(page *client.QueryPage, appendPage bool) {
	names := make([]string, len(page.Entities))
	for i, e := range page.Entities {
		names[i] = entityLabel(e)
	}
	if appendPage {
		s.entities.items = append(s.entities.items, names...)
		s.loadedEntities = append(s.loadedEntities, page.Entities...)
	} else {
		s.entities = columnList{items: names}
		s.loadedEntities = page.Entities
	}
	s.entityCursor = page.EndCursor
	s.entityHasMore = page.HasMore
}

// entityLabel renders an Entity's list-row label for the Entity column —
// just its ID/Name, without the "Kind/" prefix PathElement.String() would
// add, since the column is already scoped to one kind.
func entityLabel(e *model.Entity) string {
	return e.Key.Last().IDOrName()
}

// EntityCursor and EntityHasMore expose pagination state for the entity
// column so the caller can decide whether/when to fetch the next page.
func (s *State) EntityCursor() string { return s.entityCursor }
func (s *State) EntityHasMore() bool  { return s.entityHasMore }

// SelectedNamespace returns the currently selected namespace label, or ""
// if none is selected (empty list).
func (s *State) SelectedNamespace() (string, bool) { return s.namespaces.selectedItem() }

// SelectedKind returns the currently selected kind name, or "" if none.
func (s *State) SelectedKind() (string, bool) { return s.kinds.selectedItem() }

// SelectedEntity returns the currently selected entity, or nil if none.
func (s *State) SelectedEntity() *model.Entity {
	label, ok := s.entities.selectedItem()
	if !ok {
		return nil
	}
	for i, item := range s.entities.items {
		if item == label {
			return s.loadedEntities[i]
		}
	}
	return nil
}

// column returns the columnList for the focused column, so movement/filter
// operations can act generically on "whichever column is focused."
func (s *State) column(col Column) *columnList {
	switch col {
	case ColumnNamespace:
		return &s.namespaces
	case ColumnKind:
		return &s.kinds
	default:
		return &s.entities
	}
}

// MoveBy moves the focused column's selection by delta rows (negative is up).
func (s *State) MoveBy(delta int) { s.column(s.Focus).moveBy(delta) }

// MoveToTop moves the focused column's selection to its first row ("gg").
func (s *State) MoveToTop() { s.column(s.Focus).moveToTop() }

// MoveToBottom moves the focused column's selection to its last row ("G").
func (s *State) MoveToBottom() { s.column(s.Focus).moveToBottom() }

// SetFilter sets the focused column's substring filter ("/").
func (s *State) SetFilter(text string) { s.column(s.Focus).filter = text }

// Filter returns the focused column's current filter text.
func (s *State) Filter() string { return s.column(s.Focus).filter }

// VisibleItems returns the given column's currently filtered item list, for
// rendering.
func (s *State) VisibleItems(col Column) []string { return s.column(col).visible() }

// SelectedIndex returns the given column's selected row index into its
// VisibleItems (not its unfiltered items).
func (s *State) SelectedIndex(col Column) int { return s.column(col).selected }

// FocusRight moves focus one column to the right ("l"/ranger drill-in),
// stopping at the entity column. It does not fetch data; the caller is
// responsible for loading the newly-focused column's contents based on the
// now-fixed selection to its left.
func (s *State) FocusRight() bool {
	if s.Focus == ColumnEntity {
		return false
	}
	s.Focus++
	return true
}

// FocusLeft moves focus one column to the left ("h"/ranger back-out),
// stopping at the namespace column.
func (s *State) FocusLeft() bool {
	if s.Focus == ColumnNamespace {
		return false
	}
	s.Focus--
	return true
}
