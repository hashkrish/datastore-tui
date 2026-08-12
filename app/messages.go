package app

import (
	"github.com/krishnan/datastore-tui/datastore/client"
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
