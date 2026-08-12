package client

import (
	"context"

	"github.com/krishnan/datastore-tui/datastore/model"
)

// Query is a simplified structured query: enough to list a Kind's entities,
// or query the __namespace__/__kind__ metadata kinds, with cursor-based
// pagination. It intentionally does not expose Datastore's full filter
// grammar, which this TUI does not need for browsing/editing.
type Query struct {
	Kind        string
	Limit       int32  // 0 means "server default"
	StartCursor string // "" for the first page
	Order       []Order
}

// Order is a single ASC/DESC ordering clause.
type Order struct {
	Property   string
	Descending bool
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

type wireStructuredQuery struct {
	Kind        []wireKindExpr      `json:"kind,omitempty"`
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
