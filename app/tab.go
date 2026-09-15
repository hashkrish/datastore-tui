package app

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/edit"
	"github.com/krishnan/datastore-tui/ui/keymap"
	"github.com/krishnan/datastore-tui/ui/nav"
)

// tab holds one independent Miller-column browse/detail session: everything
// Model used to own before multi-tab support, minus the state that's
// genuinely shared across every session (the Datastore client, read-only
// flag, bookmarks, and terminal size — see Model). Model embeds a *tab for
// the active tab, so every existing "m.nav", "m.screen", "m.currentEntity"
// etc. reference across app/*.go keeps compiling and operating on whichever
// tab is active, unchanged. Async command results are the one place that
// can't rely on that shortcut — see tabID in messages.go and tabByID in
// model.go.
type tab struct {
	id int

	namespace string // resolved (non-label) namespace backing the current kind/entity lists

	nav        *nav.State
	detailPath nav.DetailPath

	currentEntity *model.Entity // non-nil while a detail view is open
	dirty         edit.Tracker

	// detailOrigin is the screen ("h"/"esc"/"q" out of the detail view's
	// root) should return to: screenBrowse (the zero value, and every
	// existing way of opening detail) or screenTable, when detail was
	// opened via "enter" on a table-view row (see openEntityDetailFromTable
	// in app/table.go). Deliberately separate from prevScreen, which is
	// reused as scratch state for confirm/help overlays and gets
	// overwritten en route through those flows.
	detailOrigin screen

	tableState *nav.TableState // non-nil while screenTable is active

	// entityStack holds the detail-view state to return to when "h"/"esc"
	// backs out past the root of an entity reached via "ctrl+]" (following a
	// Key-typed property) — see followKeyProperty/goToKey and detailBack.
	// Empty for a detail view opened directly from browse, a bookmark, or a
	// new entity, so backing out of those still lands in browse as before.
	entityStack []entityFrame

	screen         screen
	prevScreen     screen // screen to return to after a confirm/help overlay
	detailSelected int
	detailFilter   string // substring filter over the current scope's rows

	filterInput textinput.Model

	fieldEditor    *edit.FieldEditor
	editingSegment nav.PropSegment // which row's leaf value fieldEditor is editing

	newItemKind      model.ValueKind
	newItemTarget    newItemTarget
	retypeOriginal   model.Value // pre-retype value, reused if the same type is re-picked
	newItemTypeForm  *huh.Form
	newEntityKeyForm *huh.Form
	newEntityKeyKind string
	newEntityKeyID   string
	newEntityKeyName string

	// Query filter ("Q" in browse mode): screenQueryFilter fills in
	// queryProperty/queryOp/queryValueKind via queryFilterForm, then
	// screenQueryValue reuses fieldEditor to fill in the value (optionally
	// via screenQueryPastePicker, for Key-typed values), appending the
	// result to activeFilters. Pressing "Q" again while activeFilters is
	// already non-empty adds another AND-combined filter rather than
	// replacing it; activeFilters is non-empty once a query has run, so
	// refreshing the Entity column re-runs it instead of reloading the plain
	// list.
	queryFilterForm  *huh.Form
	queryProperty    string
	queryOp          client.FilterOp
	queryValueKind   model.ValueKind
	queryPasteCursor int
	activeFilters    []client.PropertyFilter

	// Order by ("O" in browse mode): screenOrder fills in orderProperty/
	// orderDescending via orderForm, then activeOrder is set on completion.
	// Combines with activeFilters when both are set — see
	// query.QueryEntitiesPage.
	orderForm       *huh.Form
	orderProperty   string
	orderDescending bool
	activeOrder     *client.Order

	// Cross-kind reference query ("ctrl+f"/"F" in browse mode on a
	// highlighted entity's key, or in detail mode on a highlighted scalar
	// property): screenRefKind fills in refTargetKind via refKindForm
	// (options loaded non-destructively into refKinds, not nav's real Kind
	// column — the user may cancel), then screenRefProperty fills in
	// refProperty via refPropertyForm, at which point the flow commits — see
	// updateRefPropertyForm. refAddToExisting distinguishes "F" (AND-combine
	// onto the active query, if staying on the same kind) from "ctrl+f"
	// (always clear first).
	refValue         model.Value
	refSourceKind    string
	refAddToExisting bool
	refKinds         []string
	refKindForm      *huh.Form
	refTargetKind    string
	refPropertyForm  *huh.Form
	refProperty      string

	chordG keymap.Chord
	chordD keymap.Chord
	chordY keymap.Chord

	confirmYes func(*Model) (tea.Model, tea.Cmd)

	status string
	err    error
}

// newTab returns a fresh tab (id) starting at the namespace column, mirroring
// what Model.New used to set up for the single implicit session.
func newTab(id int) *tab {
	fi := textinput.New()
	fi.Prompt = "/"
	return &tab{
		id:          id,
		nav:         nav.NewState(),
		filterInput: fi,
	}
}

// label returns the tab bar's display text for t: its current
// namespace/kind location, or "new tab" before anything has loaded.
func (t *tab) label() string {
	ns, ok := t.nav.SelectedNamespace()
	if !ok {
		return "new tab"
	}
	if kind, ok := t.nav.SelectedKind(); ok {
		return ns + "/" + kind
	}
	return ns
}
