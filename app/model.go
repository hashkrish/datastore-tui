// Package app wires the datastore client, navigation state, panes, and edit
// forms into a single bubbletea Model.
package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/edit"
	"github.com/krishnan/datastore-tui/ui/keymap"
	"github.com/krishnan/datastore-tui/ui/nav"
	"github.com/krishnan/datastore-tui/ui/panes"
)

// screen selects which full-screen view is active.
type screen int

const (
	screenBrowse screen = iota
	screenDetail
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
	screenBookmarks
	screenQueryFilter
	screenQueryValue
	screenQueryPastePicker
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

// Model is the top-level bubbletea model.
type Model struct {
	client    *client.Client
	namespace string // resolved (non-label) namespace backing the current kind/entity lists

	nav        *nav.State
	detailPath nav.DetailPath

	currentEntity *model.Entity // non-nil while a detail view is open
	dirty         edit.Tracker

	screen         screen
	prevScreen     screen // screen to return to after a confirm/help overlay
	detailSelected int
	detailFilter   string // substring filter over the current scope's rows

	filterInput textinput.Model

	fieldEditor    *edit.FieldEditor
	editingSegment nav.PropSegment // which row's leaf value fieldEditor is editing

	newItemKind      model.ValueKind
	newItemTarget    newItemTarget
	retypeOriginal   model.Value // pre-retype value, reused if the same type is re-picked
	newItemTypeForm  *huh.Form
	newEntityKeyForm *huh.Form
	newEntityKeyKind string
	newEntityKeyID   string
	newEntityKeyName string

	bookmarks        []bookmark
	bookmarkCursor   int
	bookmarkEntities map[string]*model.Entity // key.String() -> fetched entity, nil map while loading

	// Query filter ("Q" in browse mode): screenQueryFilter fills in
	// queryProperty/queryOp/queryValueKind via queryFilterForm, then
	// screenQueryValue reuses fieldEditor to fill in the value (optionally
	// via screenQueryPastePicker, for Key-typed values). activeFilter is
	// non-nil once a query has run, so refreshing the Entity column re-runs
	// it instead of reloading the plain list.
	queryFilterForm  *huh.Form
	queryProperty    string
	queryOp          client.FilterOp
	queryValueKind   model.ValueKind
	queryPasteCursor int
	activeFilter     *client.PropertyFilter

	chordG keymap.Chord
	chordD keymap.Chord

	confirmYes func(*Model) (tea.Model, tea.Cmd)

	width, height int
	status        string
	err           error
}

// New builds a fresh Model against c.
func New(c *client.Client) *Model {
	fi := textinput.New()
	fi.Prompt = "/"
	return &Model{
		client:      c,
		nav:         nav.NewState(),
		filterInput: fi,
		bookmarks:   loadBookmarks(),
	}
}

func (m *Model) Init() tea.Cmd {
	return loadNamespacesCmd(m.client)
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
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.nav.SetNamespaces(msg.namespaces)
		return m, m.previewCmd()

	case kindsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// The namespace this page was fetched for may no longer be the one
		// highlighted (fast j/k scrolling fires overlapping preview fetches
		// that can resolve out of order) — drop it rather than show kinds
		// for the wrong namespace.
		if ns, ok := m.nav.SelectedNamespace(); !ok || ns != msg.namespace {
			return m, nil
		}
		m.nav.SetKinds(msg.kinds)
		// If this landed a real drill-in (Focus is now Kind, not just a
		// Namespace-focused preview fetch), the Entity preview pane needs
		// data for whichever kind SetKinds just selected.
		if m.nav.Focus == nav.ColumnKind {
			return m, m.previewCmd()
		}
		return m, nil

	case entitiesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if kind, ok := m.nav.SelectedKind(); !ok || kind != msg.kind {
			return m, nil
		}
		if ns, ok := m.nav.SelectedNamespace(); !ok || ns != msg.namespace {
			return m, nil
		}
		m.nav.SetEntitiesPage(msg.page, msg.appendPage)
		return m, nil

	case propertiesLoadedMsg:
		m.status = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if kind, ok := m.nav.SelectedKind(); !ok || kind != msg.kind {
			return m, nil
		}
		if ns, ok := m.nav.SelectedNamespace(); !ok || ns != msg.namespace {
			return m, nil
		}
		return m.openQueryFilterForm(msg.properties)

	case entitySavedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = ""
			return m, nil
		}
		m.dirty.Reset()
		m.status = "saved"
		return m, nil

	case entityDeletedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.status = "deleted"
		ns, _ := m.nav.SelectedNamespace()
		kind, _ := m.nav.SelectedKind()
		return m, loadEntitiesCmd(m.client, ns, kind, "", false)

	case keyLookupMsg:
		m.status = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		return m.openEntity(msg.entity)

	case bookmarksLookedUpMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.bookmarkEntities = msg.entities
		return m, nil

	case tea.KeyMsg:
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
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.err = nil
	switch m.screen {
	case screenBrowse:
		return m.updateBrowse(msg)
	case screenFilterInput:
		return m.updateFilterInput(msg)
	case screenDetailFilterInput:
		return m.updateDetailFilterInput(msg)
	case screenDetail:
		return m.updateDetail(msg)
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
	case screenConfirmDeleteEntity, screenConfirmDeleteItem, screenConfirmQuit, screenConfirmRefresh:
		return m.updateConfirm(msg)
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
	body := m.viewBody()
	statusLine := m.viewStatus()
	return lipgloss.JoinVertical(lipgloss.Left, body, statusLine)
}

func (m *Model) viewBody() string {
	contentHeight := m.height - 1
	switch m.screen {
	case screenFilterInput:
		return panes.RenderBrowse(m.nav, m.width, contentHeight-1) + "\n" + m.filterInput.View()
	case screenDetailFilterInput:
		return m.viewDetailScreen(contentHeight-1) + "\n" + m.filterInput.View()
	case screenDetail, screenEditLeaf, screenNewItemType, screenNewItemValue, screenConfirmDeleteItem, screenConfirmRefresh:
		return m.viewDetailScreen(contentHeight)
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
	case screenConfirmDeleteEntity:
		return "Delete entity " + entityLabel(m.nav.SelectedEntity()) + "? Press y to confirm, any other key to cancel."
	case screenConfirmQuit:
		return "Unsaved changes will be lost. Press y to quit anyway, any other key to cancel."
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
	_, rows, err := m.currentScope()
	if err != nil {
		return breadcrumb + "\n\n" + err.Error()
	}
	// Only reserve a separator line when something (a form or confirm
	// prompt) is appended below the row list; plain viewing uses the full
	// available height for rows.
	reserve := 0
	switch m.screen {
	case screenEditLeaf, screenNewItemValue, screenNewItemType, screenConfirmDeleteItem, screenConfirmRefresh:
		reserve = 1
	}
	base := panes.RenderDetail(breadcrumb, rows, m.detailSelected, m.width, height-reserve)

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
		if m.activeFilter != nil {
			info += fmt.Sprintf(" (filtered: %s %s %s)", m.activeFilter.Property, filterOpSymbol(m.activeFilter.Op), formatFilterValue(m.activeFilter.Value))
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

Browse mode:
  j/k, up/down   move
  ctrl+u/ctrl+d  half page up/down
  h/l, left/right back / drill in
  gg / G         jump to top / bottom of column
  /              filter the focused column
  enter          open selected entity
  o              new entity (Entity column)
  dd             delete selected entity (Entity column)
  R              refresh focused column
  Q              query: filter the current kind's entities by a property
  ctrl+l         open bookmarks (jump to a bookmarked entity)
  q, ctrl+c      quit
  (right pane previews the highlighted entity's properties)

Query filter (Q):
  enter          confirm each step (property/operator/type, then value)
  esc            cancel back to browse
  ctrl+p         (Key values only) paste from a bookmark, with live preview

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
