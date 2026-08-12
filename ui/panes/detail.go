package panes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/nav"
)

// DetailRow is one row of the entity detail/property view: a container
// (embedded entity or array, drill-in-able) or a scalar leaf (editable).
type DetailRow struct {
	Segment     nav.PropSegment
	Kind        model.ValueKind
	Preview     string
	IsContainer bool
}

// IsContainer reports whether v should be drilled into (l/enter) rather
// than edited directly.
func IsContainer(v model.Value) bool {
	return v.Kind == model.KindEntity || v.Kind == model.KindArray
}

// BuildRows lists the rows of the current scope value, which must be a
// KindEntity or KindArray Value (as returned by GetValueAtPath for a
// non-leaf path).
func BuildRows(scope model.Value) ([]DetailRow, error) {
	switch scope.Kind {
	case model.KindEntity:
		return entityRows(scope.EntityValue), nil
	case model.KindArray:
		return arrayRows(scope.ArrayValue), nil
	default:
		return nil, fmt.Errorf("panes: cannot list rows of a %s leaf value", scope.Kind)
	}
}

func entityRows(e *model.Entity) []DetailRow {
	if e == nil {
		return nil
	}
	names := make([]string, 0, len(e.Properties))
	for name := range e.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := make([]DetailRow, len(names))
	for i, name := range names {
		v := e.Properties[name]
		rows[i] = DetailRow{
			Segment:     nav.PropSegment{Name: name},
			Kind:        v.Kind,
			Preview:     preview(v),
			IsContainer: IsContainer(v),
		}
	}
	return rows
}

func arrayRows(values []model.Value) []DetailRow {
	rows := make([]DetailRow, len(values))
	for i, v := range values {
		rows[i] = DetailRow{
			Segment:     nav.PropSegment{IsIndex: true, Index: i},
			Kind:        v.Kind,
			Preview:     preview(v),
			IsContainer: IsContainer(v),
		}
	}
	return rows
}

// preview renders a short human-readable summary of v's value, for display
// alongside its property name in the detail list.
func preview(v model.Value) string {
	switch v.Kind {
	case model.KindNull:
		return "null"
	case model.KindBoolean:
		return fmt.Sprintf("%t", v.BooleanValue)
	case model.KindInteger:
		return fmt.Sprintf("%d", v.IntegerValue)
	case model.KindDouble:
		return fmt.Sprintf("%g", v.DoubleValue)
	case model.KindTimestamp:
		return v.TimestampValue.Format("2006-01-02T15:04:05Z")
	case model.KindString:
		s := v.StringValue
		if len(s) > 60 {
			s = s[:57] + "..."
		}
		return fmt.Sprintf("%q", s)
	case model.KindBlob:
		return fmt.Sprintf("<%d bytes>", len(v.BlobValue))
	case model.KindGeoPoint:
		return fmt.Sprintf("(%g, %g)", v.GeoPointValue.Latitude, v.GeoPointValue.Longitude)
	case model.KindKey:
		return v.KeyValue.String()
	case model.KindEntity:
		n := 0
		if v.EntityValue != nil {
			n = len(v.EntityValue.Properties)
		}
		return fmt.Sprintf("{%d properties}", n)
	case model.KindArray:
		return fmt.Sprintf("[%d items]", len(v.ArrayValue))
	default:
		return ""
	}
}

// FormatBreadcrumb renders namespace/key (key.Last() already includes its
// own kind, e.g. "Person/alice") plus the current detail path as a single
// breadcrumb string.
func FormatBreadcrumb(namespace string, key *model.Key, path *nav.DetailPath) string {
	root := strings.Join([]string{namespace, key.Last().String()}, "/")
	return path.Breadcrumb(root)
}
