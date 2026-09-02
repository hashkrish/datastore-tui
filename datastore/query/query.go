// Package query provides the metadata-kind lookups (namespaces, kinds) and
// paginated entity listing the browse UI needs on top of the raw
// datastore/client REST client.
package query

import (
	"context"
	"sort"

	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
)

// DefaultNamespaceLabel is the display label used for the default (empty)
// namespace, since Datastore itself has no name for it.
const DefaultNamespaceLabel = "[default]"

// ListNamespaces returns every namespace in the project, sorted with the
// default namespace first. Backed by a query against the reserved
// __namespace__ metadata kind, which must be queried without a namespace
// partition.
func ListNamespaces(ctx context.Context, c *client.Client) ([]string, error) {
	page, err := c.RunQuery(ctx, "", client.Query{Kind: "__namespace__"})
	if err != nil {
		return nil, err
	}

	namespaces := []string{DefaultNamespaceLabel}
	for _, e := range page.Entities {
		if name := e.Key.Last().Name; name != "" {
			namespaces = append(namespaces, name)
		}
	}
	sort.Strings(namespaces[1:])
	return namespaces, nil
}

// ListKinds returns every Kind present in namespace (pass
// DefaultNamespaceLabel or "" for the default namespace), backed by a query
// against the reserved __kind__ metadata kind.
func ListKinds(ctx context.Context, c *client.Client, namespace string) ([]string, error) {
	page, err := c.RunQuery(ctx, resolveNamespace(namespace), client.Query{Kind: "__kind__"})
	if err != nil {
		return nil, err
	}

	kinds := make([]string, 0, len(page.Entities))
	for _, e := range page.Entities {
		kinds = append(kinds, e.Key.Last().Name)
	}
	sort.Strings(kinds)
	return kinds, nil
}

// ListProperties returns every property name defined on kind within
// namespace, backed by an ancestor query against the reserved __property__
// metadata kind (ancestored under __kind__/kind, per Datastore's metadata
// query convention: https://cloud.google.com/datastore/docs/concepts/metadataqueries).
// A property can appear more than once in __property__ if it has multiple
// representations (e.g. both indexed as a string and as an array) — deduped
// here since callers just want a flat pick-list of names.
func ListProperties(ctx context.Context, c *client.Client, namespace, kind string) ([]string, error) {
	// The ancestor key's namespace must match the query's namespace
	// partition — Datastore rejects the request otherwise ("query namespace
	// is X but ancestor namespace is Y").
	ns := resolveNamespace(namespace)
	ancestor := &model.Key{NamespaceID: ns, Path: []model.PathElement{{Kind: "__kind__", Name: kind}}}
	page, err := c.RunQuery(ctx, ns, client.Query{
		Kind: "__property__",
		Filters: []client.PropertyFilter{{
			Property: "__key__",
			Op:       client.OpHasAncestor,
			Value:    model.KeyValueOf(ancestor),
		}},
	})
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(page.Entities))
	properties := make([]string, 0, len(page.Entities))
	for _, e := range page.Entities {
		name := e.Key.Last().Name
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		properties = append(properties, name)
	}
	sort.Strings(properties)
	return properties, nil
}

// ListEntitiesPage fetches one page of entities of kind within namespace,
// ordered by order if given (falling back to __key__ as a tiebreaker for
// stable pagination), or by key alone otherwise. Pass cursor="" for the
// first page; subsequent pages use the EndCursor from the previous
// QueryPage.
func ListEntitiesPage(ctx context.Context, c *client.Client, namespace, kind, cursor string, pageSize int32, order *client.Order) (*client.QueryPage, error) {
	clauses := []client.Order{{Property: "__key__"}}
	if order != nil {
		clauses = []client.Order{*order, {Property: "__key__"}}
	}
	return c.RunQuery(ctx, resolveNamespace(namespace), client.Query{
		Kind:        kind,
		StartCursor: cursor,
		Limit:       pageSize,
		Order:       clauses,
	})
}

// QueryEntitiesPage fetches one page of entities of kind within namespace
// matching filters (AND-combined; must be non-empty — callers with no
// filter should use ListEntitiesPage instead), the same shape as
// ListEntitiesPage but with a property filter applied, plus an optional
// user-chosen order. Ordering by key isn't possible when one of filters is
// an inequality (Datastore requires the query's first sort order to match
// the inequality-filtered property), so those queries order by that
// property first, with order (if it names a different property) as a
// secondary sort; all-equality filters put order (or __key__, if none)
// first, same as ListEntitiesPage.
func QueryEntitiesPage(ctx context.Context, c *client.Client, namespace, kind string, filters []client.PropertyFilter, cursor string, pageSize int32, order *client.Order) (*client.QueryPage, error) {
	inequalityProperty := ""
	for _, f := range filters {
		if f.Op != client.OpEqual {
			inequalityProperty = f.Property
			break
		}
	}

	var clauses []client.Order
	switch {
	case inequalityProperty != "":
		clauses = []client.Order{{Property: inequalityProperty}}
		if order != nil && order.Property != inequalityProperty {
			clauses = append(clauses, *order)
		}
	case order != nil:
		clauses = []client.Order{*order, {Property: "__key__"}}
	default:
		clauses = []client.Order{{Property: "__key__"}}
	}
	return c.RunQuery(ctx, resolveNamespace(namespace), client.Query{
		Kind:        kind,
		StartCursor: cursor,
		Limit:       pageSize,
		Order:       clauses,
		Filters:     filters,
	})
}

func resolveNamespace(namespace string) string {
	if namespace == DefaultNamespaceLabel {
		return ""
	}
	return namespace
}

// EntityKeys extracts the Key of each entity, a common need when building a
// list pane from a QueryPage.
func EntityKeys(entities []*model.Entity) []*model.Key {
	keys := make([]*model.Key, len(entities))
	for i, e := range entities {
		keys[i] = e.Key
	}
	return keys
}
