package client

import (
	"context"
	"encoding/base64"

	"github.com/krishnan/datastore-tui/datastore/model"
)

// CommitMode selects transactional vs. non-transactional commits.
type CommitMode string

const (
	NonTransactional CommitMode = "NON_TRANSACTIONAL"
	Transactional    CommitMode = "TRANSACTIONAL"
)

// Mutation is one write operation within a Commit call. Build one with
// Insert, Update, Upsert, or Delete.
type Mutation struct {
	insert *model.Entity
	update *model.Entity
	upsert *model.Entity
	delete *model.Key
}

func InsertMutation(e *model.Entity) Mutation { return Mutation{insert: e} }
func UpdateMutation(e *model.Entity) Mutation { return Mutation{update: e} }
func UpsertMutation(e *model.Entity) Mutation { return Mutation{upsert: e} }
func DeleteMutation(k *model.Key) Mutation    { return Mutation{delete: k} }

type wireMutation struct {
	Insert *model.Entity `json:"insert,omitempty"`
	Update *model.Entity `json:"update,omitempty"`
	Upsert *model.Entity `json:"upsert,omitempty"`
	Delete *model.Key    `json:"delete,omitempty"`
}

func (m Mutation) toWire() wireMutation {
	return wireMutation{Insert: m.insert, Update: m.update, Upsert: m.upsert, Delete: m.delete}
}

type commitRequest struct {
	Mode        CommitMode     `json:"mode"`
	Mutations   []wireMutation `json:"mutations"`
	Transaction string         `json:"transaction,omitempty"` // base64
}

type mutationResult struct {
	Key              *model.Key `json:"key,omitempty"`
	Version          string     `json:"version,omitempty"`
	ConflictDetected bool       `json:"conflictDetected,omitempty"`
}

type commitResponse struct {
	MutationResults []mutationResult `json:"mutationResults"`
	IndexUpdates    int32            `json:"indexUpdates"`
}

// CommitResult mirrors the server's per-mutation results, in the same order
// as the Mutations passed to Commit. Key is populated for inserts/upserts
// that received a server-allocated ID.
type CommitResult struct {
	Keys         []*model.Key
	IndexUpdates int32
}

// Commit applies mutations, batching them into a single round trip. Pass a
// non-empty transaction (from BeginTransaction) for Transactional mode, or
// nil/empty for NonTransactional.
func (c *Client) Commit(ctx context.Context, mode CommitMode, transaction []byte, mutations []Mutation) (*CommitResult, error) {
	req := commitRequest{Mode: mode}
	if len(transaction) > 0 {
		req.Transaction = base64.StdEncoding.EncodeToString(transaction)
	}
	for _, m := range mutations {
		req.Mutations = append(req.Mutations, m.toWire())
	}

	var resp commitResponse
	if err := c.post(ctx, "commit", req, &resp); err != nil {
		return nil, err
	}

	result := &CommitResult{IndexUpdates: resp.IndexUpdates}
	for _, r := range resp.MutationResults {
		result.Keys = append(result.Keys, r.Key)
	}
	return result, nil
}

type beginTransactionResponse struct {
	Transaction string `json:"transaction"` // base64
}

// BeginTransaction starts a new transaction and returns its opaque handle,
// for use as the transaction argument to Commit or Rollback.
func (c *Client) BeginTransaction(ctx context.Context) ([]byte, error) {
	var resp beginTransactionResponse
	if err := c.post(ctx, "beginTransaction", struct{}{}, &resp); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(resp.Transaction)
}

type rollbackRequest struct {
	Transaction string `json:"transaction"`
}

// Rollback aborts a transaction previously started with BeginTransaction.
func (c *Client) Rollback(ctx context.Context, transaction []byte) error {
	req := rollbackRequest{Transaction: base64.StdEncoding.EncodeToString(transaction)}
	return c.post(ctx, "rollback", req, nil)
}
