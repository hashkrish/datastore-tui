package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/nav"
	"github.com/krishnan/datastore-tui/ui/panes"
)

// startRefQueryFromBrowse implements "ctrl+f"/"F" in browse mode: queries
// another kind for entities whose chosen property equals the highlighted
// entity's key. Only meaningful with the Entity column focused and an
// entity highlighted. addToExisting selects "F"'s behavior (AND-combine
// onto the active query when staying on the same kind) over "ctrl+f"'s
// (always clear first) — see updateRefPropertyForm.
func (m *Model) startRefQueryFromBrowse(addToExisting bool) (tea.Model, tea.Cmd) {
	if m.nav.Focus != nav.ColumnEntity {
		return m, nil
	}
	e := m.nav.SelectedEntity()
	if e == nil {
		return m, nil
	}
	m.prevScreen = screenBrowse
	return m.startRefQuery(model.KeyValueOf(e.Key), addToExisting)
}

// startRefQueryFromDetail implements "ctrl+f"/"F" in detail mode: queries
// another kind for entities whose chosen property equals the highlighted
// scalar property's current value. Rejects a container row (array/embedded
// entity) the same way startAddItem/confirmDeleteItem do. Since this leaves
// detail view, it prompts first if there are unsaved edits, mirroring
// exitDetailToBrowse. See startRefQueryFromBrowse for addToExisting.
func (m *Model) startRefQueryFromDetail(addToExisting bool) (tea.Model, tea.Cmd) {
	_, rows, err := m.currentScope()
	if err != nil {
		m.err = err
		return m, nil
	}
	if len(rows) == 0 {
		return m, nil
	}
	row := rows[m.detailSelected]
	if row.IsContainer {
		m.status = "select a scalar property to find references by"
		return m, nil
	}
	segs := append(m.detailPath.Segments(), row.Segment)
	value, err := panes.GetValueAtPath(m.currentEntity, segs)
	if err != nil {
		m.err = err
		return m, nil
	}

	if m.dirty.Dirty() {
		m.prevScreen = screenDetail
		m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
			mm.currentEntity = nil
			mm.dirty.Reset()
			mm.screen = screenBrowse
			mm.prevScreen = screenBrowse
			return mm.startRefQuery(value, addToExisting)
		}
		m.screen = screenConfirmQuit
		return m, nil
	}
	m.currentEntity = nil
	m.screen = screenBrowse
	m.prevScreen = screenBrowse
	return m.startRefQuery(value, addToExisting)
}

// startRefQuery is the shared entry point for the cross-kind reference
// query flow: captures value (the filter's fixed comparison value),
// refSourceKind (the kind being browsed when the flow started, needed at
// commit time to tell whether the target kind is the one already being
// queried), and addToExisting ("F"'s AND-combine vs "ctrl+f"'s always-clear
// — see updateRefPropertyForm), then loads the namespace's kinds for the
// target-kind picker.
func (m *Model) startRefQuery(value model.Value, addToExisting bool) (tea.Model, tea.Cmd) {
	m.refValue = value
	m.refSourceKind, _ = m.nav.SelectedKind()
	m.refAddToExisting = addToExisting
	m.refTargetKind = ""
	m.refProperty = ""
	m.status = "loading kinds..."
	return m, loadRefKindsCmd(m.client, m.id, m.namespace)
}

// openRefKindForm builds refKindForm once loadRefKindsCmd resolves — a
// single-field pick-list of every kind in the namespace, mirroring
// openQueryFilterForm's property field.
func (m *Model) openRefKindForm(kinds []string) (tea.Model, tea.Cmd) {
	if len(kinds) == 0 {
		m.err = errRefNoKinds
		m.screen = m.prevScreen
		return m, nil
	}
	m.refKinds = kinds
	m.refTargetKind = kinds[0]

	opts := make([]huh.Option[string], len(kinds))
	for i, k := range kinds {
		opts[i] = huh.NewOption(k, k)
	}
	m.refKindForm = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Target kind").Options(opts...).Value(&m.refTargetKind).Filtering(true),
	))
	if m.width > 0 {
		m.refKindForm = m.refKindForm.WithWidth(m.width)
	}
	if h := formHeight(m.height); h > 0 {
		m.refKindForm = m.refKindForm.WithHeight(h)
	}
	m.screen = screenRefKind
	return m, m.refKindForm.Init()
}

// updateRefKindForm drives refKindForm (reference query flow step 1).
func (m *Model) updateRefKindForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.refKindForm.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.refKindForm = f
	}

	switch m.refKindForm.State {
	case huh.StateCompleted:
		m.status = "loading properties..."
		return m, loadRefPropertiesCmd(m.client, m.id, m.namespace, m.refTargetKind)
	case huh.StateAborted:
		m.screen = m.prevScreen
		return m, nil
	default:
		return m, cmd
	}
}

// openRefPropertyForm builds refPropertyForm once loadRefPropertiesCmd
// resolves, falling back to free-text entry if the target kind has no
// __property__ metadata yet, mirroring openQueryFilterForm.
func (m *Model) openRefPropertyForm(properties []string) (tea.Model, tea.Cmd) {
	m.refProperty = ""

	var propertyField huh.Field
	if len(properties) > 0 {
		propOpts := make([]huh.Option[string], len(properties))
		for i, p := range properties {
			propOpts[i] = huh.NewOption(p, p)
		}
		m.refProperty = properties[0]
		propertyField = huh.NewSelect[string]().Title("Property").Options(propOpts...).Value(&m.refProperty).Filtering(true)
	} else {
		propertyField = huh.NewInput().Title("Property (none found for this kind)").Value(&m.refProperty)
	}

	m.refPropertyForm = huh.NewForm(huh.NewGroup(propertyField))
	if m.width > 0 {
		m.refPropertyForm = m.refPropertyForm.WithWidth(m.width)
	}
	if h := formHeight(m.height); h > 0 {
		m.refPropertyForm = m.refPropertyForm.WithHeight(h)
	}
	m.screen = screenRefProperty
	return m, m.refPropertyForm.Init()
}

// updateRefPropertyForm drives refPropertyForm (reference query flow step
// 2) and, on completion, commits the flow: switches the Kind column to the
// target kind and runs the filtered query. If refAddToExisting ("F") and
// the target kind is the same kind already being browsed, the new filter
// AND-combines onto the active query (like pressing "f" again); otherwise
// ("ctrl+f", or a different target kind) any prior filter/order is
// cleared first.
func (m *Model) updateRefPropertyForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.refPropertyForm.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.refPropertyForm = f
	}

	switch m.refPropertyForm.State {
	case huh.StateCompleted:
		if m.refProperty == "" {
			m.err = errQueryPropertyRequired
			m.screen = screenBrowse
			return m, nil
		}
		filter := client.PropertyFilter{Property: m.refProperty, Op: client.OpEqual, Value: m.refValue}

		m.nav.SetKinds(m.refKinds)
		m.nav.SelectKind(m.refTargetKind)
		m.nav.Focus = nav.ColumnEntity

		if m.refAddToExisting && m.refTargetKind == m.refSourceKind {
			m.activeFilters = append(m.activeFilters, filter)
		} else {
			// Either "ctrl+f" (always start fresh) or the target kind
			// differs from the source kind — the prior filters/order
			// referenced a different kind's properties and can't combine
			// with this one regardless of which key was pressed.
			m.activeFilters = []client.PropertyFilter{filter}
			m.activeOrder = nil
		}

		m.screen = screenBrowse
		m.status = "querying..."
		return m, runFilteredQueryCmd(m.client, m.id, m.namespace, m.refTargetKind, m.activeFilters, m.activeOrder, "", false)
	case huh.StateAborted:
		m.screen = screenBrowse
		return m, nil
	default:
		return m, cmd
	}
}
