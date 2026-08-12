package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/datastore/query"
	"github.com/krishnan/datastore-tui/ui/edit"
)

const entityPageSize = 200

func loadNamespacesCmd(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		ns, err := query.ListNamespaces(context.Background(), c)
		return namespacesLoadedMsg{namespaces: ns, err: err}
	}
}

func loadKindsCmd(c *client.Client, namespace string) tea.Cmd {
	return func() tea.Msg {
		kinds, err := query.ListKinds(context.Background(), c, namespace)
		return kindsLoadedMsg{kinds: kinds, err: err}
	}
}

func loadEntitiesCmd(c *client.Client, namespace, kind, cursor string, appendPage bool) tea.Cmd {
	return func() tea.Msg {
		page, err := query.ListEntitiesPage(context.Background(), c, namespace, kind, cursor, entityPageSize)
		return entitiesLoadedMsg{page: page, appendPage: appendPage, err: err}
	}
}

func saveEntityCmd(c *client.Client, e *model.Entity) tea.Cmd {
	return func() tea.Msg {
		err := edit.Save(context.Background(), c, e)
		return entitySavedMsg{err: err}
	}
}

func deleteEntityCmd(c *client.Client, key *model.Key) tea.Cmd {
	return func() tea.Msg {
		err := edit.Delete(context.Background(), c, key)
		return entityDeletedMsg{err: err}
	}
}
