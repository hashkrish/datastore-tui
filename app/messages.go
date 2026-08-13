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

type kindsLoadedMsg struct {
	kinds []string
	err   error
}

type entitiesLoadedMsg struct {
	page       *client.QueryPage
	appendPage bool
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
