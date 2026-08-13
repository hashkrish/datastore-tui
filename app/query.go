package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/edit"
	"github.com/krishnan/datastore-tui/ui/nav"
)

// queryValueKinds are the value types offered for a filter's value — a
// scalar Datastore can actually compare a property against. Unlike
// edit.TypeSelectField's full list (used for array items/retyping), Array,
// Entity, and Null are excluded: none is a meaningful filter value for the
// EQUAL/LESS_THAN/etc. operators PropertyFilter supports.
var queryValueKinds = []model.ValueKind{
	model.KindString,
	model.KindInteger,
	model.KindDouble,
	model.KindBoolean,
	model.KindTimestamp,
	model.KindGeoPoint,
	model.KindBlob,
	model.KindKey,
}

var queryOps = []client.FilterOp{
	client.OpEqual,
	client.OpLessThan,
	client.OpGreaterThan,
	client.OpLessThanOrEqual,
	client.OpGreaterThanOrEqual,
}

// filterOpSymbol renders a FilterOp as the comparison symbol a user typed
// (huh's select shows these as the option labels too).
func filterOpSymbol(op client.FilterOp) string {
	switch op {
	case client.OpEqual:
		return "="
	case client.OpLessThan:
		return "<"
	case client.OpGreaterThan:
		return ">"
	case client.OpLessThanOrEqual:
		return "<="
	case client.OpGreaterThanOrEqual:
		return ">="
	default:
		return string(op)
	}
}

// formatFilterValue renders a filter's value for display in the breadcrumb
// and value-entry header — just the scalar kinds queryValueKinds offers.
func formatFilterValue(v model.Value) string {
	switch v.Kind {
	case model.KindString:
		return fmt.Sprintf("%q", v.StringValue)
	case model.KindInteger:
		return fmt.Sprintf("%d", v.IntegerValue)
	case model.KindDouble:
		return fmt.Sprintf("%g", v.DoubleValue)
	case model.KindBoolean:
		return fmt.Sprintf("%t", v.BooleanValue)
	case model.KindTimestamp:
		return v.TimestampValue.Format("2006-01-02T15:04:05Z")
	case model.KindGeoPoint:
		return fmt.Sprintf("(%g, %g)", v.GeoPointValue.Latitude, v.GeoPointValue.Longitude)
	case model.KindBlob:
		return fmt.Sprintf("<%d bytes>", len(v.BlobValue))
	case model.KindKey:
		if v.KeyValue == nil {
			return ""
		}
		return v.KeyValue.String()
	default:
		return ""
	}
}

// startQuery implements "Q" in browse mode: opens the property/operator/type
// form (step 1 of the query filter flow), scoped to whichever kind is
// currently highlighted — which may not yet be m.namespace's concern if the
// user only ever used live cursor-preview to reach this kind without
// drilling in, so namespace is resolved fresh here and stamped onto
// m.namespace, mirroring what drillIn does when entering a kind normally.
// The form itself isn't built until the __property__ metadata query
// (loadPropertiesCmd) resolves, so the property field can offer a pick-list
// instead of a free-text name — see openQueryFilterForm.
func (m *Model) startQuery() (tea.Model, tea.Cmd) {
	ns, ok := m.nav.SelectedNamespace()
	if !ok {
		m.status = "select a namespace first"
		return m, nil
	}
	kind, ok := m.nav.SelectedKind()
	if !ok {
		m.status = "select a kind first"
		return m, nil
	}
	m.namespace = ns
	m.status = "loading properties..."
	return m, loadPropertiesCmd(m.client, ns, kind)
}

// openQueryFilterForm builds queryFilterForm once loadPropertiesCmd
// resolves. properties selects the property field from a pick-list; if the
// __property__ metadata query came back empty (e.g. the kind has no typed
// properties Datastore tracks yet), it falls back to a free-text input so
// the query feature still works.
func (m *Model) openQueryFilterForm(properties []string) (tea.Model, tea.Cmd) {
	m.queryProperty = ""
	m.queryOp = client.OpEqual
	m.queryValueKind = model.KindString

	var propertyField huh.Field
	if len(properties) > 0 {
		propOpts := make([]huh.Option[string], len(properties))
		for i, p := range properties {
			propOpts[i] = huh.NewOption(p, p)
		}
		m.queryProperty = properties[0]
		propertyField = huh.NewSelect[string]().Title("Property").Options(propOpts...).Value(&m.queryProperty)
	} else {
		propertyField = huh.NewInput().Title("Property (none found for this kind)").Value(&m.queryProperty)
	}

	opOpts := make([]huh.Option[client.FilterOp], len(queryOps))
	for i, op := range queryOps {
		opOpts[i] = huh.NewOption(filterOpSymbol(op), op)
	}
	typeOpts := make([]huh.Option[model.ValueKind], len(queryValueKinds))
	for i, k := range queryValueKinds {
		typeOpts[i] = huh.NewOption(k.String(), k)
	}

	m.queryFilterForm = huh.NewForm(huh.NewGroup(
		propertyField,
		huh.NewSelect[client.FilterOp]().Title("Operator").Options(opOpts...).Value(&m.queryOp),
		huh.NewSelect[model.ValueKind]().Title("Value type").Options(typeOpts...).Value(&m.queryValueKind),
	))
	if m.width > 0 {
		m.queryFilterForm = m.queryFilterForm.WithWidth(m.width)
	}
	m.screen = screenQueryFilter
	return m, m.queryFilterForm.Init()
}

// updateQueryFilter drives queryFilterForm (query flow step 1).
func (m *Model) updateQueryFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.queryFilterForm.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.queryFilterForm = f
	}

	switch m.queryFilterForm.State {
	case huh.StateCompleted:
		if m.queryProperty == "" {
			m.err = errQueryPropertyRequired
			m.screen = screenBrowse
			return m, nil
		}
		fe, ok := edit.NewFieldEditor(edit.ZeroValue(m.queryValueKind), m.width)
		if !ok {
			m.err = errQueryUnsupportedValueType
			m.screen = screenBrowse
			return m, nil
		}
		m.fieldEditor = fe
		m.screen = screenQueryValue
		return m, fe.Form().Init()
	case huh.StateAborted:
		m.screen = screenBrowse
		return m, nil
	default:
		return m, cmd
	}
}

// updateQueryValue drives the value editor (query flow step 2), mirroring
// updateEditLeaf, but first intercepting ctrl+p (only meaningful for a Key
// value, the only type a bookmark can supply) before forwarding to the
// form — the same "check for a specific key before forwarding" pattern
// updateFilterInput uses.
func (m *Model) updateQueryValue(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && m.queryValueKind == model.KindKey && keyMsg.String() == "ctrl+p" {
		return m.startQueryPastePicker()
	}

	updated, cmd := m.fieldEditor.Form().Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		_ = f
	}

	switch m.fieldEditor.Form().State {
	case huh.StateCompleted:
		value, err := m.fieldEditor.Result()
		if err != nil {
			m.err = err
			m.screen = screenBrowse
			m.fieldEditor = nil
			return m, nil
		}
		filter := client.PropertyFilter{Property: m.queryProperty, Op: m.queryOp, Value: value}
		m.activeFilter = &filter
		m.fieldEditor = nil
		m.nav.Focus = nav.ColumnEntity
		m.screen = screenBrowse
		m.status = "querying..."
		kind, _ := m.nav.SelectedKind()
		return m, runFilteredQueryCmd(m.client, m.namespace, kind, filter, "", false)
	case huh.StateAborted:
		m.fieldEditor = nil
		m.screen = screenBrowse
		return m, nil
	default:
		return m, cmd
	}
}

// startQueryPastePicker implements ctrl+p inside the query value editor:
// opens a standalone picker over the saved bookmarks (distinct from the
// ctrl+l picker) so the user can fill in a Key-typed filter value from a
// bookmark instead of typing kind/ID/name by hand. Reuses the same batched
// Lookup the ctrl+l picker uses for its live preview.
func (m *Model) startQueryPastePicker() (tea.Model, tea.Cmd) {
	if len(m.bookmarks) == 0 {
		m.status = "no bookmarks"
		return m, nil
	}
	m.queryPasteCursor = 0
	m.bookmarkEntities = nil
	m.screen = screenQueryPastePicker
	return m, lookupBookmarksCmd(m.client, m.bookmarks)
}

// updateQueryPastePicker drives the paste picker: j/k move (live preview
// comes for free from the already-fetched m.bookmarkEntities map, as with
// the ctrl+l picker), enter fills the selected bookmark's Key into the value
// editor and returns to it, esc/q returns unchanged.
//
// Filling it in means rebuilding fieldEditor from scratch via
// NewFieldEditor/SetKeyValue rather than mutating the live one: huh's
// Input.Value(ptr) copies the pointed-to string into its own internal
// textinput.Model once, at construction — it never re-reads the pointer
// afterward, so mutating keyKind/keyID/keyName on the already-built form
// wouldn't be reflected in what's rendered.
func (m *Model) updateQueryPastePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.queryPasteCursor < len(m.bookmarks)-1 {
			m.queryPasteCursor++
		}
		return m, nil
	case "k", "up":
		if m.queryPasteCursor > 0 {
			m.queryPasteCursor--
		}
		return m, nil
	case "enter", "l", "right":
		var cmd tea.Cmd
		if m.queryPasteCursor >= 0 && m.queryPasteCursor < len(m.bookmarks) {
			key := m.bookmarks[m.queryPasteCursor].Key
			if fe, ok := edit.NewFieldEditor(model.KeyValueOf(key), m.width); ok {
				m.fieldEditor = fe
				cmd = fe.Form().Init()
			}
		}
		m.screen = screenQueryValue
		return m, cmd
	case "esc", "q":
		m.screen = screenQueryValue
		return m, nil
	}
	return m, nil
}
