package app

import (
	"context"
	"fmt"

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
		return kindsLoadedMsg{namespace: namespace, kinds: kinds, err: err}
	}
}

// loadPropertiesCmd fetches kind's property names ("Q" in browse mode, so
// the query filter form can offer a pick-list instead of a free-text name).
func loadPropertiesCmd(c *client.Client, namespace, kind string) tea.Cmd {
	return func() tea.Msg {
		properties, err := query.ListProperties(context.Background(), c, namespace, kind)
		return propertiesLoadedMsg{namespace: namespace, kind: kind, properties: properties, err: err}
	}
}

// loadOrderPropertiesCmd is loadPropertiesCmd's counterpart for "O" in
// browse mode: same __property__ lookup, tagged with a distinct message
// type so Update knows to open the order form rather than the query filter
// form.
func loadOrderPropertiesCmd(c *client.Client, namespace, kind string) tea.Cmd {
	return func() tea.Msg {
		properties, err := query.ListProperties(context.Background(), c, namespace, kind)
		return orderPropertiesLoadedMsg{namespace: namespace, kind: kind, properties: properties, err: err}
	}
}

func loadEntitiesCmd(c *client.Client, namespace, kind, cursor string, appendPage bool, order *client.Order) tea.Cmd {
	return func() tea.Msg {
		page, err := query.ListEntitiesPage(context.Background(), c, namespace, kind, cursor, entityPageSize, order)
		return entitiesLoadedMsg{namespace: namespace, kind: kind, page: page, appendPage: appendPage, err: err}
	}
}

// runFilteredQueryCmd is loadEntitiesCmd's counterpart for a property-filtered
// query ("Q" in browse mode, AND-combining every filter in filters): same
// entitiesLoadedMsg result, so Update's existing handler and staleness guard
// apply unchanged regardless of whether the page came from the plain list or
// a filter.
func runFilteredQueryCmd(c *client.Client, namespace, kind string, filters []client.PropertyFilter, order *client.Order, cursor string, appendPage bool) tea.Cmd {
	return func() tea.Msg {
		page, err := query.QueryEntitiesPage(context.Background(), c, namespace, kind, filters, cursor, entityPageSize, order)
		return entitiesLoadedMsg{namespace: namespace, kind: kind, page: page, appendPage: appendPage, err: err}
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

func lookupKeyCmd(c *client.Client, key *model.Key) tea.Cmd {
	return func() tea.Msg {
		result, err := c.Lookup(context.Background(), []*model.Key{key})
		if err != nil {
			return keyLookupMsg{err: err}
		}
		if len(result.Found) == 0 {
			return keyLookupMsg{err: fmt.Errorf("entity not found: %s", key.String())}
		}
		return keyLookupMsg{entity: result.Found[0]}
	}
}

// lookupBookmarksCmd fetches every bookmarked entity in one batched Lookup
// call, for the ctrl+l picker's live preview.
func lookupBookmarksCmd(c *client.Client, bs []bookmark) tea.Cmd {
	keys := make([]*model.Key, len(bs))
	for i, b := range bs {
		keys[i] = b.Key
	}
	return func() tea.Msg {
		result, err := c.Lookup(context.Background(), keys)
		if err != nil {
			return bookmarksLookedUpMsg{err: err}
		}
		entities := make(map[string]*model.Entity, len(result.Found))
		for _, e := range result.Found {
			entities[e.Key.String()] = e
		}
		return bookmarksLookedUpMsg{entities: entities}
	}
}
