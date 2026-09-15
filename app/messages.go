package app

import (
	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
)

// Async results delivered back into Update via tea.Cmd. Each API call runs
// in its own command so the UI never blocks on the network.
//
// Every message carries the tabID of the tab that issued the command, so
// Update can route the result back to that tab (via Model.tabByID) rather
// than to whichever tab happens to be active when the response arrives —
// see app/tab.go and the tabByID/*Cmd plumbing in app/model.go and
// app/commands.go.

type namespacesLoadedMsg struct {
	tabID      int
	namespaces []string
	err        error
}

// namespace records which namespace this page of kinds was fetched for, so
// Update can drop a response that arrives after the highlighted namespace
// has since moved on (e.g. fast j/k scrolling firing overlapping preview
// fetches out of order).
type kindsLoadedMsg struct {
	tabID     int
	namespace string
	kinds     []string
	err       error
}

// namespace/kind record what this page of entities was fetched for, so
// Update can drop a stale response the same way kindsLoadedMsg does.
type entitiesLoadedMsg struct {
	tabID      int
	namespace  string
	kind       string
	page       *client.QueryPage
	appendPage bool
	err        error
}

// namespace/kind record what this list of property names was fetched for,
// so Update can drop a stale response the same way kindsLoadedMsg does —
// e.g. if the user backs out of the kind before the __property__ query
// (triggered by "Q") resolves.
type propertiesLoadedMsg struct {
	tabID      int
	namespace  string
	kind       string
	properties []string
	err        error
}

// orderPropertiesLoadedMsg is loadOrderPropertiesCmd's result ("O" in browse
// mode) — the same property-name lookup propertiesLoadedMsg uses for "Q",
// but tagged separately so Update opens the order form instead of the query
// filter form.
type orderPropertiesLoadedMsg struct {
	tabID      int
	namespace  string
	kind       string
	properties []string
	err        error
}

// refKindsLoadedMsg is loadRefKindsCmd's result ("F" cross-kind reference
// query): the same kind listing kindsLoadedMsg uses, but tagged separately
// so Update doesn't mutate the real Kind column with it — kindsLoadedMsg's
// handler calls nav.State.SetKinds immediately, which this flow must not do
// until the user actually commits to a target kind (they may cancel
// partway through).
type refKindsLoadedMsg struct {
	tabID     int
	namespace string
	kinds     []string
	err       error
}

// refPropertiesLoadedMsg is loadRefPropertiesCmd's result: the same
// __property__ lookup propertiesLoadedMsg uses for "Q", but tagged
// separately so Update opens the reference-query property picker (fixed to
// the already-known filter value) instead of the "Q" flow's value editor,
// and so it isn't dropped as stale by propertiesLoadedMsg's "kind ==
// m.nav.SelectedKind()" guard — the target kind here is deliberately not
// the currently selected one.
type refPropertiesLoadedMsg struct {
	tabID      int
	namespace  string
	kind       string
	properties []string
	err        error
}

type entitySavedMsg struct {
	tabID int
	err   error
}

type entityDeletedMsg struct {
	tabID int
	err   error
}

// keyLookupMsg delivers the result of following a Key-typed property
// (ctrl+]) or opening a bookmark to another entity.
type keyLookupMsg struct {
	tabID  int
	entity *model.Entity
	err    error
}

// bookmarksLookedUpMsg delivers the batched Lookup issued when the bookmark
// picker (ctrl+l) opens, keyed by each found entity's Key.String() so the
// picker can preview any bookmark instantly as the selection moves.
// Bookmarks are global (not per-tab), and the picker is a modal overlay
// that blocks tab-switching while it's open, so this doesn't need a tabID.
type bookmarksLookedUpMsg struct {
	entities map[string]*model.Entity
	err      error
}
