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

// ListEntitiesPage fetches one page of entities of kind within namespace,
// ordered by key for stable pagination. Pass cursor="" for the first page;
// subsequent pages use the EndCursor from the previous QueryPage.
func ListEntitiesPage(ctx context.Context, c *client.Client, namespace, kind, cursor string, pageSize int32) (*client.QueryPage, error) {
	return c.RunQuery(ctx, resolveNamespace(namespace), client.Query{
		Kind:        kind,
		StartCursor: cursor,
		Limit:       pageSize,
		Order:       []client.Order{{Property: "__key__"}},
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
