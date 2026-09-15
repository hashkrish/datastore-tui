package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/krishnan/datastore-tui/datastore/client"
)

// startOrder implements "O" in browse mode: opens the property/direction
// form, scoped to whichever kind is currently highlighted — mirrors
// startQuery's namespace resolution and reuses the same __property__
// metadata lookup for the pick-list.
func (m *Model) startOrder() (tea.Model, tea.Cmd) {
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
	return m, loadOrderPropertiesCmd(m.client, m.id, ns, kind)
}

// openOrderForm builds orderForm once loadOrderPropertiesCmd resolves,
// mirroring openQueryFilterForm's pick-list/free-text fallback. Unlike the
// query filter flow, there's no value-entry step — an order clause is just
// a property and a direction.
func (m *Model) openOrderForm(properties []string) (tea.Model, tea.Cmd) {
	m.orderProperty = ""
	m.orderDescending = false

	var propertyField huh.Field
	if len(properties) > 0 {
		propOpts := make([]huh.Option[string], len(properties))
		for i, p := range properties {
			propOpts[i] = huh.NewOption(p, p)
		}
		m.orderProperty = properties[0]
		propertyField = huh.NewSelect[string]().Title("Property").Options(propOpts...).Value(&m.orderProperty).Filtering(true)
	} else {
		propertyField = huh.NewInput().Title("Property (none found for this kind)").Value(&m.orderProperty)
	}

	m.orderForm = huh.NewForm(huh.NewGroup(
		propertyField,
		huh.NewSelect[bool]().Title("Direction").
			Options(huh.NewOption("Ascending", false), huh.NewOption("Descending", true)).
			Value(&m.orderDescending),
	))
	if m.width > 0 {
		m.orderForm = m.orderForm.WithWidth(m.width)
	}
	if h := formHeight(m.height); h > 0 {
		m.orderForm = m.orderForm.WithHeight(h)
	}
	m.screen = screenOrder
	return m, m.orderForm.Init()
}

// updateOrderForm drives orderForm. On completion it sets activeOrder and
// re-runs the current query — combined with activeFilters if any are set, so
// ordering and filtering compose (see query.QueryEntitiesPage).
func (m *Model) updateOrderForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.orderForm.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.orderForm = f
	}

	switch m.orderForm.State {
	case huh.StateCompleted:
		if m.orderProperty == "" {
			m.err = errOrderPropertyRequired
			m.screen = screenBrowse
			return m, nil
		}
		order := client.Order{Property: m.orderProperty, Descending: m.orderDescending}
		m.activeOrder = &order
		m.screen = screenBrowse
		m.status = "querying..."
		kind, _ := m.nav.SelectedKind()
		if len(m.activeFilters) > 0 {
			return m, runFilteredQueryCmd(m.client, m.id, m.namespace, kind, m.activeFilters, m.activeOrder, "", false)
		}
		return m, loadEntitiesCmd(m.client, m.id, m.namespace, kind, "", false, m.activeOrder)
	case huh.StateAborted:
		m.screen = screenBrowse
		return m, nil
	default:
		return m, cmd
	}
}

// viewOrderForm renders the property/direction form ("O" in browse mode).
func (m *Model) viewOrderForm() string {
	kind, _ := m.nav.SelectedKind()
	return "Order " + kind + "\n\n" + m.orderForm.View()
}
