package client

import (
	"context"

	"github.com/krishnan/datastore-tui/datastore/model"
)

type lookupRequest struct {
	Keys []*model.Key `json:"keys"`
}

type entityResult struct {
	Entity model.Entity `json:"entity"`
}

type lookupResponse struct {
	Found    []entityResult `json:"found"`
	Missing  []entityResult `json:"missing"`
	Deferred []*model.Key   `json:"deferred"`
}

// LookupResult holds the outcome of a Lookup call.
type LookupResult struct {
	Found    []*model.Entity
	Missing  []*model.Key
	Deferred []*model.Key // keys the server deferred; re-issue a Lookup for these
}

// Lookup fetches entities by key.
func (c *Client) Lookup(ctx context.Context, keys []*model.Key) (*LookupResult, error) {
	var resp lookupResponse
	if err := c.post(ctx, "lookup", lookupRequest{Keys: keys}, &resp); err != nil {
		return nil, err
	}

	result := &LookupResult{Deferred: resp.Deferred}
	for _, f := range resp.Found {
		e := f.Entity
		result.Found = append(result.Found, &e)
	}
	for _, m := range resp.Missing {
		result.Missing = append(result.Missing, m.Entity.Key)
	}
	return result, nil
}
