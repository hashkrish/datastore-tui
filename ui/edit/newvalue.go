package edit

import (
	"time"

	"github.com/charmbracelet/huh"
	"github.com/krishnan/datastore-tui/datastore/model"
)

// selectableKinds are the value types offered when adding a new array item
// or retyping a property.
var selectableKinds = []model.ValueKind{
	model.KindString,
	model.KindInteger,
	model.KindDouble,
	model.KindBoolean,
	model.KindTimestamp,
	model.KindGeoPoint,
	model.KindBlob,
	model.KindKey,
	model.KindArray,
	model.KindEntity,
	model.KindNull,
}

// TypeSelectField builds just the value-type select field, writing the
// choice into selected — exported so a caller can compose it into a form
// alongside other fields (e.g. the query filter form, which combines a
// property-name input and operator select with this type picker in one
// group) rather than getting a whole standalone form back.
func TypeSelectField(title string, selected *model.ValueKind) huh.Field {
	opts := make([]huh.Option[model.ValueKind], len(selectableKinds))
	for i, k := range selectableKinds {
		opts[i] = huh.NewOption(k.String(), k)
	}
	return huh.NewSelect[model.ValueKind]().Title(title).Options(opts...).Value(selected)
}

// TypeSelectForm builds a form that lets the user pick a value type,
// writing the choice into selected. Used as the first step of "add a new
// array item" (o in an array's detail view), before NewFieldEditor edits
// the zero value for that type.
func TypeSelectForm(selected *model.ValueKind) *huh.Form {
	return huh.NewForm(huh.NewGroup(TypeSelectField("New item type", selected)))
}

// ZeroValue returns a sensible default Value for kind, used to seed a new
// array item or property before the user edits it.
func ZeroValue(kind model.ValueKind) model.Value {
	switch kind {
	case model.KindString:
		return model.StringValue("")
	case model.KindInteger:
		return model.IntegerValue(0)
	case model.KindDouble:
		return model.DoubleValue(0)
	case model.KindBoolean:
		return model.BooleanValue(false)
	case model.KindTimestamp:
		return model.TimestampValue(time.Now().UTC())
	case model.KindGeoPoint:
		return model.GeoPointValueOf(0, 0)
	case model.KindBlob:
		return model.BlobValueOf(nil)
	case model.KindKey:
		return model.KeyValueOf(&model.Key{Path: []model.PathElement{{Kind: ""}}})
	case model.KindArray:
		return model.ArrayValueOf(nil)
	case model.KindEntity:
		return model.EntityValueOf(&model.Entity{Properties: map[string]model.Value{}})
	default:
		return model.NullValue()
	}
}
