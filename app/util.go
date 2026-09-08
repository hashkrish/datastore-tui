package app

import (
	"errors"
	"strconv"
	"strings"

	"github.com/krishnan/datastore-tui/datastore/query"
	"github.com/krishnan/datastore-tui/ui/panes"
)

var errNewEntityKindRequired = errors.New("app: new entity requires a Kind")
var errQueryPropertyRequired = errors.New("app: query filter requires a property name")
var errQueryUnsupportedValueType = errors.New("app: query filter value type is not editable")
var errOrderPropertyRequired = errors.New("app: order requires a property name")
var errRefNoKinds = errors.New("app: no kinds found in this namespace to query")

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// resolveNamespaceID converts the browse view's namespace label (which uses
// query.DefaultNamespaceLabel for the default namespace) into the actual
// namespace ID Datastore expects ("" for default).
func resolveNamespaceID(namespace string) string {
	if namespace == query.DefaultNamespaceLabel {
		return ""
	}
	return namespace
}

// namespaceLabel converts a Key's actual namespace ID (as returned by the
// server, "" for the default namespace) into the browse view's display
// label. The inverse of resolveNamespaceID.
func namespaceLabel(namespaceID string) string {
	if namespaceID == "" {
		return query.DefaultNamespaceLabel
	}
	return namespaceID
}

// filterDetailRows keeps only the rows whose property name/index or preview
// text contains filter (case-insensitive); an empty filter keeps everything.
func filterDetailRows(rows []panes.DetailRow, filter string) []panes.DetailRow {
	if filter == "" {
		return rows
	}
	filter = strings.ToLower(filter)
	out := make([]panes.DetailRow, 0, len(rows))
	for _, row := range rows {
		haystack := strings.ToLower(row.Segment.String() + " " + row.Preview)
		if strings.Contains(haystack, filter) {
			out = append(out, row)
		}
	}
	return out
}

// halfPage estimates a "half screen" of rows for ctrl+u/ctrl+d scrolling,
// based on the terminal height minus the chrome (column titles, status
// line) that each list view renders around its rows.
func (m *Model) halfPage() int {
	rows := (m.height - 2) / 2
	if rows < 1 {
		return 1
	}
	return rows
}
