// Package app wires the datastore client, navigation state, panes, and edit
// forms into a single bubbletea Model.
package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/nav"
	"github.com/krishnan/datastore-tui/ui/panes"
)

// screen selects which full-screen view is active.
type screen int

const (
	screenBrowse screen = iota
	screenDetail
	screenTable
	screenTableColumnFilter
	screenTableSearch
	screenFilterInput
	screenDetailFilterInput
	screenEditLeaf
	screenNewItemType
	screenNewItemValue
	screenNewEntityKey
	screenConfirmDeleteEntity
	screenConfirmDeleteItem
	screenConfirmQuit
	screenConfirmRefresh
	screenConfirmClearBookmarks
	screenBookmarks
	screenQueryFilter
	screenQueryValue
	screenQueryPastePicker
	screenOrder
	screenRefKind
	screenRefProperty
	screenKindJump
	screenHelp
)

// newItemTarget selects what the type-select + value-edit flow (triggered
// either by "o" on an array, or by opening a null leaf for retyping) does
// once the user finishes picking a type and a value.
type newItemTarget int

const (
	targetArrayAppend newItemTarget = iota
	targetLeafRetype
)

// Model is the top-level bubbletea model. It owns the state genuinely shared
// across every tab (the Datastore client, read-only flag, bookmarks, and
// terminal size) plus the tab list itself; everything else — Miller-column
// navigation, the open detail view, active filters/order, in-flight forms —
// lives on *tab (see app/tab.go). Model embeds a *tab pointing at the active
// tab, so the bulk of app/*.go keeps referencing "m.nav", "m.screen", etc.
// unchanged, always resolving to whichever tab is active.
type Model struct {
	client   *client.Client
	readOnly bool // disables every mutating action; see blockReadOnly

	// consoleProject is the GCP project "W" opens Cloud Console links in;
	// "" against the emulator, which has no console (see webUnavailable).
	consoleProject string

	bookmarks        []bookmark
	bookmarkCursor   int
	bookmarkEntities map[string]*model.Entity // key.String() -> fetched entity, nil map while loading

	tabs      []*tab
	active    int
	nextTabID int
	*tab      // the active tab; kept in sync with tabs[active] — see app/tabs.go

	confirmingQuit bool // cross-tab unsaved-edits guard on app quit, see requestQuit

	width, height int
}

// New builds a fresh Model against c, with a single starting tab. When
// readOnly is true, every mutating action (new/delete entity,
// retype/add/delete property, save) is blocked; see blockReadOnly.
// consoleProject enables "W" (open in the Cloud Console) for that project;
// pass "" when targeting the emulator.
func New(c *client.Client, readOnly bool, consoleProject string) *Model {
	first := newTab(1)
	return &Model{
		client:         c,
		readOnly:       readOnly,
		consoleProject: consoleProject,
		bookmarks:      loadBookmarks(),
		tabs:           []*tab{first},
		active:         0,
		tab:            first,
		nextTabID:      2,
	}
}

// blockReadOnly reports whether m is in read-only mode, setting a status
// message if so. Callers that trigger a mutating action must check this
// first, before opening any form or confirmation dialog:
//
//	if m.blockReadOnly() {
//		return m, nil
//	}
func (m *Model) blockReadOnly() bool {
	if !m.readOnly {
		return false
	}
	m.status = "read-only mode: mutations disabled (-read-only)"
	return true
}

func (m *Model) Init() tea.Cmd {
	return loadNamespacesCmd(m.client, m.id)
}

// tabByID finds the tab an async command result belongs to, by the stable
// id its issuing *Cmd was tagged with — not m.active/m.tab, which may have
// moved on to a different tab by the time the result arrives. Reports false
// if that tab has since been closed, in which case the result is dropped.
func (m *Model) tabByID(id int) (*tab, bool) {
	for _, t := range m.tabs {
		if t.id == id {
			return t, true
		}
	}
	return nil, false
}

func (m *Model) currentScope() (model.Value, []panes.DetailRow, error) {
	scope, err := panes.GetValueAtPath(m.currentEntity, m.detailPath.Segments())
	if err != nil {
		return model.Value{}, nil, err
	}
	rows, err := panes.BuildRows(scope)
	if err != nil {
		return scope, nil, err
	}
	return scope, filterDetailRows(rows, m.detailFilter), nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case namespacesLoadedMsg:
		t, ok := m.tabByID(msg.tabID)
		if !ok {
			return m, nil
		}
		if msg.err != nil {
			t.err = msg.err
			return m, nil
		}
		t.nav.SetNamespaces(msg.namespaces)
		return m, m.previewCmd(t)

	case kindsLoadedMsg:
		t, ok := m.tabByID(msg.tabID)
		if !ok {
			return m, nil
		}
		if msg.err != nil {
			t.err = msg.err
			return m, nil
		}
		// The namespace this page was fetched for may no longer be the one
		// highlighted (fast j/k scrolling fires overlapping preview fetches
		// that can resolve out of order) — drop it rather than show kinds
		// for the wrong namespace.
		if ns, ok := t.nav.SelectedNamespace(); !ok || ns != msg.namespace {
			return m, nil
		}
		t.nav.SetKinds(msg.kinds)
		// If this landed a real drill-in (Focus is now Kind, not just a
		// Namespace-focused preview fetch), the Entity preview pane needs
		// data for whichever kind SetKinds just selected.
		if t.nav.Focus == nav.ColumnKind {
			return m, m.previewCmd(t)
		}
		return m, nil

	case entitiesLoadedMsg:
		t, ok := m.tabByID(msg.tabID)
		if !ok {
			return m, nil
		}
		if msg.err != nil {
			t.err = msg.err
			return m, nil
		}
		if kind, ok := t.nav.SelectedKind(); !ok || kind != msg.kind {
			return m, nil
		}
		if ns, ok := t.nav.SelectedNamespace(); !ok || ns != msg.namespace {
			return m, nil
		}
		t.nav.SetEntitiesPage(msg.page, msg.appendPage)
		return m, nil

	case propertiesLoadedMsg:
		t, ok := m.tabByID(msg.tabID)
		if !ok {
			return m, nil
		}
		t.status = ""
		if msg.err != nil {
			t.err = msg.err
			return m, nil
		}
		if kind, ok := t.nav.SelectedKind(); !ok || kind != msg.kind {
			return m, nil
		}
		if ns, ok := t.nav.SelectedNamespace(); !ok || ns != msg.namespace {
			return m, nil
		}
		return m.openQueryFilterForm(msg.properties)

	case orderPropertiesLoadedMsg:
		t, ok := m.tabByID(msg.tabID)
		if !ok {
			return m, nil
		}
		t.status = ""
		if msg.err != nil {
			t.err = msg.err
			return m, nil
		}
		if kind, ok := t.nav.SelectedKind(); !ok || kind != msg.kind {
			return m, nil
		}
		if ns, ok := t.nav.SelectedNamespace(); !ok || ns != msg.namespace {
			return m, nil
		}
		return m.openOrderForm(msg.properties)

	case refKindsLoadedMsg:
		t, ok := m.tabByID(msg.tabID)
		if !ok {
			return m, nil
		}
		t.status = ""
		if msg.err != nil {
			t.err = msg.err
			return m, nil
		}
		if msg.namespace != t.namespace {
			return m, nil
		}
		return m.openRefKindForm(msg.kinds)

	case kindJumpKindsLoadedMsg:
		t, ok := m.tabByID(msg.tabID)
		if !ok {
			return m, nil
		}
		t.status = ""
		if msg.err != nil {
			t.err = msg.err
			return m, nil
		}
		if msg.namespace != t.namespace {
			return m, nil
		}
		return m.openKindJumpForm(msg.kinds)

	case refPropertiesLoadedMsg:
		t, ok := m.tabByID(msg.tabID)
		if !ok {
			return m, nil
		}
		t.status = ""
		if msg.err != nil {
			t.err = msg.err
			return m, nil
		}
		if msg.namespace != t.namespace || msg.kind != t.refTargetKind {
			return m, nil
		}
		return m.openRefPropertyForm(msg.properties)

	case entitySavedMsg:
		t, ok := m.tabByID(msg.tabID)
		if !ok {
			return m, nil
		}
		if msg.err != nil {
			t.err = msg.err
			t.status = ""
			return m, nil
		}
		t.dirty.Reset()
		t.status = "saved"
		return m, nil

	case entityDeletedMsg:
		t, ok := m.tabByID(msg.tabID)
		if !ok {
			return m, nil
		}
		if msg.err != nil {
			t.err = msg.err
			return m, nil
		}
		t.status = "deleted"
		ns, _ := t.nav.SelectedNamespace()
		kind, _ := t.nav.SelectedKind()
		return m, loadEntitiesCmd(m.client, t.id, ns, kind, "", false, nil)

	case keyLookupMsg:
		t, ok := m.tabByID(msg.tabID)
		if !ok {
			return m, nil
		}
		t.status = ""
		if msg.err != nil {
			t.err = msg.err
			return m, nil
		}
		return m.openEntity(t, msg.entity)

	case bookmarksLookedUpMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.bookmarkEntities = msg.entities
		return m, nil

	case tea.KeyMsg:
		if m.confirmingQuit {
			return m.updateConfirmingQuit(msg)
		}
		return m.handleKey(msg)
	}

	// huh.Form's Next/Prev/Submit key handling queues an internal follow-up
	// tea.Cmd (e.g. to advance groups); the resulting message isn't a
	// tea.KeyMsg, so it must be forwarded here rather than in handleKey, or
	// the form would visibly "eat" the keystroke and never actually advance.
	switch m.screen {
	case screenEditLeaf, screenNewItemValue:
		return m.updateEditLeaf(msg)
	case screenNewItemType:
		return m.updateNewItemType(msg)
	case screenNewEntityKey:
		return m.updateNewEntityKey(msg)
	case screenQueryFilter:
		return m.updateQueryFilter(msg)
	case screenQueryValue:
		return m.updateQueryValue(msg)
	case screenOrder:
		return m.updateOrderForm(msg)
	case screenRefKind:
		return m.updateRefKindForm(msg)
	case screenRefProperty:
		return m.updateRefPropertyForm(msg)
	case screenKindJump:
		return m.updateKindJumpForm(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.err = nil
	if isStableScreen(m.screen) {
		if handled, mm, cmd := m.handleTabKey(msg); handled {
			return mm, cmd
		}
	}
	switch m.screen {
	case screenBrowse:
		return m.updateBrowse(msg)
	case screenFilterInput:
		return m.updateFilterInput(msg)
	case screenDetailFilterInput:
		return m.updateDetailFilterInput(msg)
	case screenDetail:
		return m.updateDetail(msg)
	case screenTable:
		return m.updateTable(msg)
	case screenTableColumnFilter:
		return m.updateTableColumnFilter(msg)
	case screenTableSearch:
		return m.updateTableSearch(msg)
	case screenEditLeaf:
		return m.updateEditLeaf(msg)
	case screenNewItemType:
		return m.updateNewItemType(msg)
	case screenNewItemValue:
		return m.updateEditLeaf(msg) // same form-driving logic, different completion handler below
	case screenNewEntityKey:
		return m.updateNewEntityKey(msg)
	case screenBookmarks:
		return m.updateBookmarkList(msg)
	case screenQueryFilter:
		return m.updateQueryFilter(msg)
	case screenQueryValue:
		return m.updateQueryValue(msg)
	case screenQueryPastePicker:
		return m.updateQueryPastePicker(msg)
	case screenOrder:
		return m.updateOrderForm(msg)
	case screenRefKind:
		return m.updateRefKindForm(msg)
	case screenRefProperty:
		return m.updateRefPropertyForm(msg)
	case screenKindJump:
		return m.updateKindJumpForm(msg)
	case screenConfirmDeleteEntity, screenConfirmDeleteItem, screenConfirmQuit, screenConfirmRefresh:
		return m.updateConfirm(msg)
	case screenConfirmClearBookmarks:
		return m.updateConfirmClearBookmarks(msg)
	case screenHelp:
		m.screen = m.prevScreen
		return m, nil
	}
	return m, nil
}

func (m *Model) View() string {
	if m.width == 0 {
		return "loading..."
	}
	parts := make([]string, 0, 3)
	if len(m.tabs) > 1 {
		labels := make([]string, len(m.tabs))
		for i, t := range m.tabs {
			labels[i] = t.label()
		}
		parts = append(parts, panes.RenderTabBar(labels, m.active, m.width))
	}
	if m.confirmingQuit {
		parts = append(parts, m.confirmingQuitMessage())
	} else {
		parts = append(parts, m.viewBody())
	}
	parts = append(parts, m.viewStatus())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// tabBarHeight is how many rows View reserves for the tab bar — one when
// there's more than one tab, zero otherwise, so a single-tab session (the
// common case) looks exactly as it did before tabs existed.
func (m *Model) tabBarHeight() int {
	if len(m.tabs) > 1 {
		return 1
	}
	return 0
}

func (m *Model) viewBody() string {
	contentHeight := m.height - 1 - m.tabBarHeight()
	switch m.screen {
	case screenFilterInput:
		return panes.RenderBrowse(m.nav, m.width, contentHeight-1) + "\n" + m.filterInput.View()
	case screenDetailFilterInput:
		return m.viewDetailScreen(contentHeight-1) + "\n" + m.filterInput.View()
	case screenDetail, screenEditLeaf, screenNewItemType, screenNewItemValue, screenConfirmDeleteItem, screenConfirmRefresh:
		return m.viewDetailScreen(contentHeight)
	case screenTable:
		return panes.RenderTable(m.nav.SelectedEntities(), m.tableState, m.width, contentHeight)
	case screenTableColumnFilter, screenTableSearch:
		return panes.RenderTable(m.nav.SelectedEntities(), m.tableState, m.width, contentHeight-1) + "\n" + m.filterInput.View()
	case screenNewEntityKey:
		return m.newEntityKeyForm.View()
	case screenBookmarks:
		return m.viewBookmarks(contentHeight)
	case screenQueryFilter:
		return m.viewQueryFilter()
	case screenQueryValue:
		return m.viewQueryValue()
	case screenQueryPastePicker:
		return m.viewQueryPastePicker(contentHeight)
	case screenOrder:
		return m.viewOrderForm()
	case screenRefKind:
		return "Find references — target kind\n\n" + m.refKindForm.View()
	case screenRefProperty:
		return "Find references — property to match\n\n" + m.refPropertyForm.View()
	case screenKindJump:
		return m.kindJumpForm.View()
	case screenConfirmDeleteEntity:
		return "Delete entity " + entityLabel(m.nav.SelectedEntity()) + "? Press y to confirm, any other key to cancel."
	case screenConfirmQuit:
		return "Unsaved changes will be lost. Press y to quit anyway, any other key to cancel."
	case screenConfirmClearBookmarks:
		return "Clear all bookmarks? Press y to confirm, any other key to cancel."
	case screenHelp:
		return helpText()
	default:
		return panes.RenderBrowse(m.nav, m.width, contentHeight)
	}
}

// viewBookmarks renders the bookmark picker (ctrl+l): a scrollable list of
// bookmark labels plus a live preview of the highlighted entry's entity
// properties, fetched in one batched Lookup when the picker opens.
func (m *Model) viewBookmarks(height int) string {
	var preview *model.Entity
	if m.bookmarkCursor >= 0 && m.bookmarkCursor < len(m.bookmarks) {
		preview = m.bookmarkEntities[m.bookmarks[m.bookmarkCursor].Key.String()]
	}
	return panes.RenderBookmarks(m.bookmarkLabels(), m.bookmarkCursor, preview, m.bookmarkEntities == nil, m.width, height)
}

// bookmarkLabels returns each bookmark's display label, shared by the ctrl+l
// picker (viewBookmarks) and the query filter's paste-from-bookmark picker
// (viewQueryPastePicker) — both list the same m.bookmarks.
func (m *Model) bookmarkLabels() []string {
	labels := make([]string, len(m.bookmarks))
	for i, b := range m.bookmarks {
		labels[i] = b.Label
	}
	return labels
}

// viewQueryFilter renders the property/operator/value-type form ("Q" in
// browse mode, step 1). Unlike screenNewItemValue's type-select step, this
// doesn't reuse viewDetailScreen — there's no open entity/breadcrumb here,
// since a query is launched from browse, not detail, mode.
// formHeight returns the row budget for a huh.Form rendered under a
// "<Title> <kind>\n\n" header (viewQueryFilter, viewOrderForm), accounting
// for that two-line header plus the one-line status bar View always appends
// below the body. Without a height, huh's Select fields render every
// option unbounded, so a kind with many properties pushes the header (and
// the form's own top) off the top of the terminal instead of scrolling.
func formHeight(termHeight int) int {
	h := termHeight - 3
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) viewQueryFilter() string {
	kind, _ := m.nav.SelectedKind()
	return "Query " + kind + "\n\n" + m.queryFilterForm.View()
}

// viewQueryValue renders the value editor (query filter step 2), reusing
// the same fieldEditor screenNewItemValue/screenEditLeaf drive. Below it, a
// reminder that ctrl+p opens the paste-from-bookmark picker, but only when
// the value being edited is a Key (the only type a bookmark can supply).
func (m *Model) viewQueryValue() string {
	kind, _ := m.nav.SelectedKind()
	header := fmt.Sprintf("Query %s: %s %s ?", kind, m.queryProperty, filterOpSymbol(m.queryOp))
	view := header + "\n\n" + m.fieldEditor.Form().View()
	if m.queryValueKind == model.KindKey {
		view += "\n(ctrl+p: paste key from a bookmark)"
	}
	return view
}

// viewQueryPastePicker renders the query value's paste-from-bookmark picker
// — same live-preview shape as viewBookmarks (a batched Lookup already
// fetched every bookmarked entity into m.bookmarkEntities), but a standalone
// screen: picking one fills in the value form instead of opening the entity.
func (m *Model) viewQueryPastePicker(height int) string {
	var preview *model.Entity
	if m.queryPasteCursor >= 0 && m.queryPasteCursor < len(m.bookmarks) {
		preview = m.bookmarkEntities[m.bookmarks[m.queryPasteCursor].Key.String()]
	}
	return panes.RenderBookmarks(m.bookmarkLabels(), m.queryPasteCursor, preview, m.bookmarkEntities == nil, m.width, height)
}

func (m *Model) viewDetailScreen(height int) string {
	breadcrumb := panes.FormatBreadcrumb(m.namespace, m.currentEntity.Key, &m.detailPath)
	scope, rows, err := m.currentScope()
	if err != nil {
		return breadcrumb + "\n\n" + err.Error()
	}
	preview, hasPreview := panes.BlobPreview(scope, rows, m.detailSelected)
	// Only reserve a separator line when something (a form or confirm
	// prompt) is appended below the row list; plain viewing uses the full
	// available height for rows.
	reserve := 0
	switch m.screen {
	case screenEditLeaf, screenNewItemValue, screenNewItemType, screenConfirmDeleteItem, screenConfirmRefresh:
		reserve = 1
	}
	base := panes.RenderDetail(breadcrumb, rows, m.detailSelected, preview, hasPreview, m.width, height-reserve)

	switch m.screen {
	case screenEditLeaf, screenNewItemValue:
		if m.fieldEditor != nil {
			return base + "\n" + m.fieldEditor.Form().View()
		}
	case screenNewItemType:
		return base + "\n" + m.newItemTypeForm.View()
	case screenConfirmDeleteItem:
		return base + "\n\nDelete this item? Press y to confirm, any other key to cancel."
	case screenConfirmRefresh:
		return base + "\n\nReloading will discard your unsaved changes. Press y to reload from the database, any other key to cancel."
	}
	return base
}

func (m *Model) viewStatus() string {
	// MaxWidth clips to exactly one line instead of word-wrapping: the
	// status/help text (or an error message) routinely exceeds narrower
	// terminal widths, and a wrapped second line silently pushes the whole
	// frame one row past the terminal height, scrolling the top border out
	// of view.
	style := lipgloss.NewStyle().Padding(0, 1).MaxWidth(m.width)
	if m.err != nil {
		return style.Foreground(lipgloss.Color("196")).Render("error: " + m.err.Error())
	}
	dirtyMark := ""
	if m.dirty.Dirty() {
		dirtyMark = " [modified]"
	}

	segs := make([]string, 0, 3)
	if info := m.currentInfo(); info != "" {
		segs = append(segs, info)
	}
	if m.status != "" {
		segs = append(segs, m.status+dirtyMark)
	} else if dirtyMark != "" {
		segs = append(segs, strings.TrimSpace(dirtyMark))
	}
	if m.readOnly {
		segs = append(segs, lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("[read-only]"))
	}
	segs = append(segs, "?")

	return style.Render(strings.Join(segs, "  "))
}

// currentInfo returns the bottom status line's "where am I" text: the
// namespace/kind/entity breadcrumb while browsing, replacing the job the
// now-removed column headers used to do. Other screens already show their
// own context (e.g. the detail view's breadcrumb at the top), so this is
// empty there.
func (m *Model) currentInfo() string {
	switch m.screen {
	case screenBrowse, screenFilterInput:
		info := panes.FormatBrowseBreadcrumb(m.nav)
		if len(m.activeFilters) > 0 {
			parts := make([]string, len(m.activeFilters))
			for i, f := range m.activeFilters {
				parts[i] = fmt.Sprintf("%s %s %s", f.Property, filterOpSymbol(f.Op), formatFilterValue(f.Value))
			}
			info += fmt.Sprintf(" (filtered: %s)", strings.Join(parts, " AND "))
		}
		if m.activeOrder != nil {
			dir := "asc"
			if m.activeOrder.Descending {
				dir = "desc"
			}
			info += fmt.Sprintf(" (ordered by: %s %s)", m.activeOrder.Property, dir)
		}
		return info
	default:
		return ""
	}
}

func entityLabel(e *model.Entity) string {
	if e == nil {
		return ""
	}
	return e.Key.Last().String()
}

func helpText() string {
	return `Datastore TUI — Help

Tabs (from browse, detail, or table view):
  ctrl+t         open a new tab (starts fresh at the namespace list)
  ctrl+w         close the active tab (confirms if it has unsaved edits;
                 no-op on the last remaining tab)
  tab/shift+tab  cycle to the next/previous tab
  1-9            jump directly to a tab

Browse mode:
  j/k, up/down   move
  ctrl+u/ctrl+d  half page up/down
  h/l, left/right back / drill in
  gg / G         jump to top / bottom of column
  /              filter the focused column
  enter          open selected entity
  o              new entity (Entity column)
  dd             delete selected entity (Entity column)
  ctrl+b         bookmark/unbookmark selected entity (Entity column)
  R              refresh focused column
  f              query: filter the current kind's entities by a property
                 (press again to AND another filter onto the current query;
                 the property picker also offers __key__, to filter/lookup
                 by the entity's own key instead of a regular property)
  C              clear all active filters
  O              order: sort the current kind's entities by a property
  ctrl+f         find references: query another kind by this entity's key
                 (clears any active query first)
  F              same as ctrl+f, but AND-combines onto the active query
                 instead of clearing it (only when staying on the same kind)
  yy             copy the selected entity's key to the clipboard
  T              toggle table (spreadsheet) view of the current kind's entities
  W              open the current kind's query (with its = filters on
                 string/key values) in the Cloud Console (not the emulator)
  yu             copy that Cloud Console URL to the clipboard
  :              jump straight to a kind's entities in the current namespace,
                 via a filterable picker
  ctrl+l         open bookmarks (jump to a bookmarked entity)
  q, ctrl+c      quit
  (right pane previews the highlighted entity's properties)

Table view (T):
  j/k, up/down   move selected row
  h/l, left/right move selected column
  gg / G         jump to top / bottom row
  ctrl+u/ctrl+d  half page up/down
  enter          open the selected row's entity in detail view
  yy             copy the selected cell's value to the clipboard
  W              open the selected row's entity in the Cloud Console
  yu             copy that Cloud Console URL to the clipboard
  *              filter which columns (properties) are shown, by name
  /              search every visible cell's value, jump to the first match
  n / N          jump to the next / previous search match
  T, q, esc      back to browse
  ctrl+c         quit

Bookmark picker (ctrl+l):
  j/k, up/down, ctrl+n/ctrl+p move
  enter, l       open the selected bookmark
  dd             delete the selected bookmark (no confirmation)
  C              clear all bookmarks (with confirmation)
  esc, q         back

Query filter (f):
  enter          confirm each step (property/operator/type, then value)
  esc            cancel back to browse
  ctrl+p         (Key values only) paste from a bookmark, with live preview

Order by (O):
  enter          confirm each step (property, then direction)
  esc            cancel back to browse

Detail mode (viewing/editing an entity):
  j/k            move between properties
  ctrl+u/ctrl+d  half page up/down
  l/enter        expand array/embedded entity, or edit a scalar
                 (a null property has nothing to edit this way)
  t              change the selected property's data type
  h/esc          back out one level, or to browse
  /              filter the current scope's properties
  o              add array item (when viewing an array)
  dd             delete array item (when viewing an array)
  r              reload from the database (confirms first if you have unsaved edits)
  ctrl+]         open the entity a selected Key property points at
                 (h/esc backs out to the entity you followed it from)
  ctrl+f         find references: query another kind by this property's value
                 (clears any active query first; prompts if you have unsaved edits)
  F              same as ctrl+f, but AND-combines onto the active query
                 instead of clearing it (only when staying on the same kind)
  yy             copy the selected property's value to the clipboard
  W              open this entity in the Cloud Console (not the emulator)
  yu             copy that Cloud Console URL to the clipboard
  ctrl+b         bookmark/unbookmark the current entity
  ctrl+l         open bookmarks (jump to a bookmarked entity)
  w              save pending edits
  q/esc          back to browse (prompts if unsaved)

Bookmark picker (ctrl+l):
  j/k            move (right pane previews the highlighted bookmark's entity)
  enter/l        open the highlighted bookmark
  esc/q          cancel back out

Press any key to close this help.`
}
