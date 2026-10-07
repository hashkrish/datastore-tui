package app

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/nav"
)

// consoleBase is the Cloud Console's Datastore Studio root for the default
// database — the only database the client talks to (see client.url).
const consoleBase = "https://console.cloud.google.com/datastore/databases/-default-/entities"

// consoleDefaultNamespace is how the console's ns= matrix parameter spells
// the default (empty) namespace.
const consoleDefaultNamespace = "__$DEFAULT$__"

// consoleBase64 is the Closure library's WEBSAFE_DOT_PADDING alphabet the
// console uses for KEY filter values: URL-safe base64 with "." for padding.
var consoleBase64 = base64.URLEncoding.WithPadding('.')

// webFromBrowse implements "W" (deliver = openWeb) and "yu" (deliver =
// copyWebURL) in browse mode, for the focused kind's entity list in the
// Cloud Console — carrying over the active query filters when the Entity
// column is focused (they only apply to the loaded list).
func (m *Model) webFromBrowse(deliver func(string)) (tea.Model, tea.Cmd) {
	if m.webUnavailable() {
		return m, nil
	}
	if m.nav.Focus == nav.ColumnNamespace {
		m.status = "select a kind first"
		return m, nil
	}
	ns, ok := m.nav.SelectedNamespace()
	if !ok {
		return m, nil
	}
	kind, ok := m.nav.SelectedKind()
	if !ok {
		return m, nil
	}
	var filters []client.PropertyFilter
	if m.nav.Focus == nav.ColumnEntity {
		filters = m.activeFilters
	}
	u, ok := consoleQueryURL(m.consoleProject, resolveNamespaceID(ns), kind, filters)
	deliver(u)
	if !ok && m.err == nil {
		m.status += " (unfiltered: only = on string/key values carries over)"
	}
	return m, nil
}

// webForEntity implements "W"/"yu" in detail/table mode, for e's page in
// the Cloud Console's entity editor; see webFromBrowse.
func (m *Model) webForEntity(e *model.Entity, deliver func(string)) (tea.Model, tea.Cmd) {
	if m.webUnavailable() || e == nil || e.Key == nil || e.Key.IsIncomplete() {
		return m, nil
	}
	deliver(consoleEntityURL(m.consoleProject, e.Key))
	return m, nil
}

// webUnavailable reports whether there's no Cloud Console to link to — the
// case against the emulator — setting a status message if so.
func (m *Model) webUnavailable() bool {
	if m.consoleProject != "" {
		return false
	}
	m.status = "console links are only available against a real GCP project, not the emulator"
	return true
}

// openWeb launches u in the system browser, falling back to copying it to
// the clipboard when no browser opener is available (e.g. over SSH).
func (m *Model) openWeb(u string) {
	if err := openBrowser(u); err != nil {
		if cerr := clipboard.WriteAll(u); cerr != nil {
			m.err = fmt.Errorf("open in web: %w", err)
			return
		}
		m.status = "couldn't launch a browser; copied console URL"
		return
	}
	m.status = "opened in browser"
}

// copyWebURL copies u to the system clipboard.
func (m *Model) copyWebURL(u string) {
	if err := clipboard.WriteAll(u); err != nil {
		m.err = err
		return
	}
	m.status = "copied console URL"
}

// openBrowser starts the platform's URL opener without waiting on it, and
// with no stdio attached so it can't write over the TUI.
func openBrowser(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// consoleEntityURL builds the console's entity-editor link for key. The
// key= parameter is the console's own key serialization (see
// consoleKeyString), URL-encoded twice — once as a value, once more as part
// of the path.
func consoleEntityURL(project string, key *model.Key) string {
	return fmt.Sprintf("%s;kind=%s;ns=%s/edit;key=%s?project=%s",
		consoleBase,
		encodeURIComponent(key.Kind()),
		consoleNamespace(key.NamespaceID),
		encodeURIComponent(encodeURIComponent(consoleKeyString(key))),
		url.QueryEscape(project))
}

// consoleKeyString serializes key the way the console's key= parameter
// does: length-prefixed fields joined by "|" — the namespace, then each
// path element's kind and "id:<n>"/"name:<s>". e.g. a default-namespace
// Task/1234567890 is "0/|4/Task|13/id:1234567890".
func consoleKeyString(key *model.Key) string {
	fields := []string{key.NamespaceID}
	for _, p := range key.Path {
		id := "name:" + p.Name
		if p.Name == "" {
			id = "id:" + strconv.FormatInt(p.ID, 10)
		}
		fields = append(fields, p.Kind, id)
	}
	return joinConsoleFields(fields)
}

// consoleQueryURL builds the console's kind-query link for kind in
// namespace, with filters encoded into its queryModel parameter. ok is
// false when some filter has no known console encoding (see
// consoleFilterClause), in which case the URL lists kind unfiltered rather
// than showing a partially-filtered result.
func consoleQueryURL(project, namespace, kind string, filters []client.PropertyFilter) (u string, ok bool) {
	base := fmt.Sprintf("%s;kind=%s;ns=%s/query/kind",
		consoleBase, encodeURIComponent(kind), consoleNamespace(namespace))
	suffix := "?project=" + url.QueryEscape(project)
	if len(filters) == 0 {
		return base + suffix, true
	}
	clauses := []string{strconv.Itoa(len(filters))}
	for _, f := range filters {
		clause, ok := consoleFilterClause(f)
		if !ok {
			return base + suffix, false
		}
		clauses = append(clauses, clause)
	}
	return base + ";queryModel=" + encodeURIComponent(strings.Join(clauses, "|")) + suffix, true
}

// consoleFilterClause encodes f as one queryModel WHERE clause, e.g.
// `WH|1|6/status|EQ|STR|4/DONE`. Only equality on string
// and key values is supported — the console's codes for other operators
// and value types haven't been confirmed.
func consoleFilterClause(f client.PropertyFilter) (string, bool) {
	if f.Op != client.OpEqual {
		return "", false
	}
	var typ, val string
	switch f.Value.Kind {
	case model.KindString:
		typ, val = "STR", f.Value.StringValue
	case model.KindKey:
		if f.Value.KeyValue == nil {
			return "", false
		}
		typ, val = "KEY", consoleBase64.EncodeToString([]byte(gqlKeyLiteral(f.Value.KeyValue)))
	default:
		return "", false
	}
	return "WH|1|" + lengthPrefixed(f.Property) + "|EQ|" + typ + "|" + lengthPrefixed(val), true
}

// gqlKeyLiteral renders key as a GQL key literal, e.g. `Key(Parent, 1,
// Child, 'name')`.
func gqlKeyLiteral(key *model.Key) string {
	parts := make([]string, 0, 2*len(key.Path))
	for _, p := range key.Path {
		id := strconv.FormatInt(p.ID, 10)
		if p.Name != "" {
			id = "'" + strings.ReplaceAll(p.Name, "'", `\'`) + "'"
		}
		parts = append(parts, p.Kind, id)
	}
	return "Key(" + strings.Join(parts, ", ") + ")"
}

func joinConsoleFields(fields []string) string {
	for i, f := range fields {
		fields[i] = lengthPrefixed(f)
	}
	return strings.Join(fields, "|")
}

func lengthPrefixed(s string) string {
	return strconv.Itoa(len(s)) + "/" + s
}

func consoleNamespace(ns string) string {
	if ns == "" {
		return consoleDefaultNamespace
	}
	return encodeURIComponent(ns)
}

// encodeURIComponent percent-encodes s like JavaScript's function of the
// same name, which the console uses: spaces become %20, not "+".
func encodeURIComponent(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
