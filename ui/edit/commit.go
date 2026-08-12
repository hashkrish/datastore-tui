package edit

import (
	"context"

	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
)

// Tracker records whether an opened entity has unsaved edits, so the app
// can prompt before discarding them.
type Tracker struct {
	dirty bool
}

func (t *Tracker) MarkDirty()  { t.dirty = true }
func (t *Tracker) Dirty() bool { return t.dirty }
func (t *Tracker) Reset()      { t.dirty = false }

// Save commits e as a single upsert. Property edits are applied in place to
// the in-memory entity as the user confirms each field (see
// ui/panes.SetValueAtPath), so by the time Save runs there is exactly one
// round trip regardless of how many properties changed.
func Save(ctx context.Context, c *client.Client, e *model.Entity) error {
	_, err := c.Commit(ctx, client.NonTransactional, nil, []client.Mutation{client.UpsertMutation(e)})
	return err
}

// Delete commits a single delete mutation for key.
func Delete(ctx context.Context, c *client.Client, key *model.Key) error {
	_, err := c.Commit(ctx, client.NonTransactional, nil, []client.Mutation{client.DeleteMutation(key)})
	return err
}
