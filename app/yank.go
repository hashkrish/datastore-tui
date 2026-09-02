package app

import (
	"encoding/base64"
	"fmt"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/panes"
)

// yankSelectedKey implements "yy" in browse mode (Entity column): copies the
// highlighted entity's key to the system clipboard.
func (m *Model) yankSelectedKey() (tea.Model, tea.Cmd) {
	e := m.nav.SelectedEntity()
	if e == nil || e.Key == nil {
		return m, nil
	}
	if err := clipboard.WriteAll(e.Key.String()); err != nil {
		m.err = err
		return m, nil
	}
	m.status = "copied key"
	return m, nil
}

// yankSelectedValue implements "yy" in detail mode: copies the highlighted
// property's value to the system clipboard. A no-op on a container row
// (array/embedded entity) — there's no single scalar value to copy.
func (m *Model) yankSelectedValue() (tea.Model, tea.Cmd) {
	scope, rows, err := m.currentScope()
	if err != nil || len(rows) == 0 {
		return m, nil
	}
	row := rows[m.detailSelected]
	if row.IsContainer {
		return m, nil
	}
	leaf, err := panes.LeafRowValue(scope, row)
	if err != nil {
		m.err = err
		return m, nil
	}
	if err := clipboard.WriteAll(yankText(leaf)); err != nil {
		m.err = err
		return m, nil
	}
	m.status = "copied value"
	return m, nil
}

// yankText renders v as the plain text a clipboard paste should produce —
// unlike panes.preview (truncated/quoted for on-screen display), this is the
// full, unquoted value.
func yankText(v model.Value) string {
	switch v.Kind {
	case model.KindNull:
		return ""
	case model.KindBoolean:
		return fmt.Sprintf("%t", v.BooleanValue)
	case model.KindInteger:
		return fmt.Sprintf("%d", v.IntegerValue)
	case model.KindDouble:
		return fmt.Sprintf("%g", v.DoubleValue)
	case model.KindTimestamp:
		return v.TimestampValue.Format("2006-01-02T15:04:05Z")
	case model.KindString:
		return v.StringValue
	case model.KindBlob:
		return base64.StdEncoding.EncodeToString(v.BlobValue)
	case model.KindGeoPoint:
		return fmt.Sprintf("%g,%g", v.GeoPointValue.Latitude, v.GeoPointValue.Longitude)
	case model.KindKey:
		if v.KeyValue == nil {
			return ""
		}
		return v.KeyValue.String()
	default:
		return ""
	}
}
