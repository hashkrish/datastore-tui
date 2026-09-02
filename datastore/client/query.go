package client

import (
	"context"

	"github.com/krishnan/datastore-tui/datastore/model"
)

// Query is a simplified structured query: enough to list a Kind's entities,
// or query the __namespace__/__kind__ metadata kinds, with cursor-based
// pagination, plus any number of property filters AND-combined together. It
// intentionally does not expose Datastore's full filter grammar (OR, nested
// composites) — this TUI only ever needs a flat AND of property filters.
type Query struct {
	Kind        string
	Limit       int32  // 0 means "server default"
	StartCursor string // "" for the first page
	Order       []Order
	Filters     []PropertyFilter // empty means unfiltered; more than one is AND-combined
}

// Order is a single ASC/DESC ordering clause.
type Order struct {
	Property   string
	Descending bool
}

// FilterOp is one of Datastore's property-filter comparison operators.
type FilterOp string

const (
	OpEqual              FilterOp = "EQUAL"
	OpLessThan           FilterOp = "LESS_THAN"
	OpGreaterThan        FilterOp = "GREATER_THAN"
	OpLessThanOrEqual    FilterOp = "LESS_THAN_OR_EQUAL"
	OpGreaterThanOrEqual FilterOp = "GREATER_THAN_OR_EQUAL"
	// OpHasAncestor is Datastore's ancestor-query operator: property must be
	// "__key__" and Value a Key-typed value naming the ancestor. Used to
	// scope a query against the __property__ metadata kind to one __kind__.
	OpHasAncestor FilterOp = "HAS_ANCESTOR"
)

// PropertyFilter restricts a query to entities where Property compares to
// Value via Op. Datastore requires that inequality operators (anything but
// OpEqual) target only one property across the whole query and be the first
// sort order; callers building a multi-filter Query are responsible for
// respecting that (Datastore itself rejects a query that doesn't).
type PropertyFilter struct {
	Property string
	Op       FilterOp
	Value    model.Value
}

type wireKindExpr struct {
	Name string `json:"name"`
}

type wirePropertyRef struct {
	Name string `json:"name"`
}

type wirePropertyOrder struct {
	Property  wirePropertyRef `json:"property"`
	Direction string          `json:"direction"`
}

type wirePropertyFilter struct {
	Property wirePropertyRef `json:"property"`
	Op       string          `json:"op"`
	Value    model.Value     `json:"value"`
}

// wireCompositeFilter mirrors the REST API's CompositeFilter: Op is always
// "AND" here (the only combinator Query exposes), applied across Filters.
type wireCompositeFilter struct {
	Op      string       `json:"op"`
	Filters []wireFilter `json:"filters"`
}

// wireFilter mirrors the REST API's Filter union: exactly one of
// PropertyFilter (a single q.Filters entry) or CompositeFilter (two or
// more, AND-combined) is populated.
type wireFilter struct {
	PropertyFilter  *wirePropertyFilter  `json:"propertyFilter,omitempty"`
	CompositeFilter *wireCompositeFilter `json:"compositeFilter,omitempty"`
}

type wireStructuredQuery struct {
	Kind        []wireKindExpr      `json:"kind,omitempty"`
	Filter      *wireFilter         `json:"filter,omitempty"`
	Order       []wirePropertyOrder `json:"order,omitempty"`
	StartCursor string              `json:"startCursor,omitempty"`
	Limit       *int32              `json:"limit,omitempty"`
}

type runQueryRequest struct {
	PartitionID *wirePartitionIDReq `json:"partitionId,omitempty"`
	Query       wireStructuredQuery `json:"query"`
}

// wirePartitionIDReq mirrors model's internal partition ID shape; duplicated
// here since model does not export it.
type wirePartitionIDReq struct {
	ProjectID   string `json:"projectId"`
	NamespaceID string `json:"namespaceId,omitempty"`
}

type entityResultBatchItem struct {
	Entity  model.Entity `json:"entity"`
	Cursor  string       `json:"cursor"`
	Version string       `json:"version"`
}

type queryBatch struct {
	EntityResults []entityResultBatchItem `json:"entityResults"`
	EndCursor     string                  `json:"endCursor"`
	MoreResults   string                  `json:"moreResults"`
}

type runQueryResponse struct {
	Batch queryBatch `json:"batch"`
}

// QueryPage is one page of RunQuery results.
type QueryPage struct {
	Entities  []*model.Entity
	EndCursor string
	// HasMore reports whether another page may exist; when false, EndCursor
	// should not be used to fetch further pages.
	HasMore bool
}

// propertyFilterWire converts a single PropertyFilter to its REST wire shape,
// shared by RunQuery's single-filter and composite-filter (AND) cases.
func propertyFilterWire(f PropertyFilter) *wirePropertyFilter {
	return &wirePropertyFilter{
		Property: wirePropertyRef{Name: f.Property},
		Op:       string(f.Op),
		Value:    f.Value,
	}
}

// RunQuery executes q against namespace (use "" for the default namespace)
// and returns one page of results.
func (c *Client) RunQuery(ctx context.Context, namespace string, q Query) (*QueryPage, error) {
	sq := wireStructuredQuery{
		Kind:        []wireKindExpr{{Name: q.Kind}},
		StartCursor: q.StartCursor,
	}
	if q.Limit > 0 {
		sq.Limit = &q.Limit
	}
	switch len(q.Filters) {
	case 0:
		// unfiltered
	case 1:
		sq.Filter = &wireFilter{PropertyFilter: propertyFilterWire(q.Filters[0])}
	default:
		subs := make([]wireFilter, len(q.Filters))
		for i, f := range q.Filters {
			subs[i] = wireFilter{PropertyFilter: propertyFilterWire(f)}
		}
		sq.Filter = &wireFilter{CompositeFilter: &wireCompositeFilter{Op: "AND", Filters: subs}}
	}
	for _, o := range q.Order {
		dir := "ASCENDING"
		if o.Descending {
			dir = "DESCENDING"
		}
		sq.Order = append(sq.Order, wirePropertyOrder{
			Property:  wirePropertyRef{Name: o.Property},
			Direction: dir,
		})
	}

	req := runQueryRequest{Query: sq}
	if namespace != "" {
		req.PartitionID = &wirePartitionIDReq{ProjectID: c.cfg.ProjectID, NamespaceID: namespace}
	}

	var resp runQueryResponse
	if err := c.post(ctx, "runQuery", req, &resp); err != nil {
		return nil, err
	}

	page := &QueryPage{
		EndCursor: resp.Batch.EndCursor,
		HasMore:   resp.Batch.MoreResults == "NOT_FINISHED",
	}
	for _, r := range resp.Batch.EntityResults {
		e := r.Entity
		page.Entities = append(page.Entities, &e)
	}
	return page, nil
}
