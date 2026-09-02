package app

import (
	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
)

// Async results delivered back into Update via tea.Cmd. Each API call runs
// in its own command so the UI never blocks on the network.

type namespacesLoadedMsg struct {
	namespaces []string
	err        error
}

// namespace records which namespace this page of kinds was fetched for, so
// Update can drop a response that arrives after the highlighted namespace
// has since moved on (e.g. fast j/k scrolling firing overlapping preview
// fetches out of order).
type kindsLoadedMsg struct {
	namespace string
	kinds     []string
	err       error
}

// namespace/kind record what this page of entities was fetched for, so
// Update can drop a stale response the same way kindsLoadedMsg does.
type entitiesLoadedMsg struct {
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
	namespace  string
	kind       string
	properties []string
	err        error
}

type entitySavedMsg struct {
	err error
}

type entityDeletedMsg struct {
	err error
}

// keyLookupMsg delivers the result of following a Key-typed property
// (ctrl+]) or opening a bookmark to another entity.
type keyLookupMsg struct {
	entity *model.Entity
	err    error
}

// bookmarksLookedUpMsg delivers the batched Lookup issued when the bookmark
// picker (ctrl+l) opens, keyed by each found entity's Key.String() so the
// picker can preview any bookmark instantly as the selection moves.
type bookmarksLookedUpMsg struct {
	entities map[string]*model.Entity
	err      error
}
