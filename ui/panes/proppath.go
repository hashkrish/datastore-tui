// Package panes renders the Miller-column browse view and the entity
// detail/property-tree view, and provides the path-addressed read/write
// helpers the detail view and edit forms use to navigate and mutate a
// entity's (possibly nested) properties in place.
package panes

import (
	"fmt"

	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/nav"
)

// GetValueAtPath resolves the container (entity or array) or leaf value at
// segs within root. An empty segs resolves to root itself, wrapped as a
// KindEntity Value so callers can treat "the whole entity" and "an embedded
// entity" uniformly.
func GetValueAtPath(root *model.Entity, segs []nav.PropSegment) (model.Value, error) {
	if len(segs) == 0 {
		return model.Value{Kind: model.KindEntity, EntityValue: root}, nil
	}
	first := segs[0]
	if first.IsIndex {
		return model.Value{}, fmt.Errorf("panes: root path segment must be a property name, got an index")
	}
	v, ok := root.Properties[first.Name]
	if !ok {
		return model.Value{}, fmt.Errorf("panes: no such property %q", first.Name)
	}
	for _, seg := range segs[1:] {
		var err error
		v, err = getChild(v, seg)
		if err != nil {
			return model.Value{}, err
		}
	}
	return v, nil
}

// SetValueAtPath writes newValue at segs within root. segs must be
// non-empty and name a property (directly, or nested through
// arrays/embedded entities).
func SetValueAtPath(root *model.Entity, segs []nav.PropSegment, newValue model.Value) error {
	if len(segs) == 0 {
		return fmt.Errorf("panes: cannot set at an empty path")
	}
	first := segs[0]
	if first.IsIndex {
		return fmt.Errorf("panes: root path segment must be a property name, got an index")
	}
	if len(segs) == 1 {
		root.Properties[first.Name] = newValue
		return nil
	}
	child, ok := root.Properties[first.Name]
	if !ok {
		return fmt.Errorf("panes: no such property %q", first.Name)
	}
	updated, err := setInValue(child, segs[1:], newValue)
	if err != nil {
		return err
	}
	root.Properties[first.Name] = updated
	return nil
}

// AppendArrayItem appends item to the array at arraySegs (a path resolving
// to a KindArray value).
func AppendArrayItem(root *model.Entity, arraySegs []nav.PropSegment, item model.Value) error {
	v, err := GetValueAtPath(root, arraySegs)
	if err != nil {
		return err
	}
	if v.Kind != model.KindArray {
		return fmt.Errorf("panes: cannot append: not an array")
	}
	v.ArrayValue = append(v.ArrayValue, item)
	return SetValueAtPath(root, arraySegs, v)
}

// RemoveArrayItem removes the item at index from the array at arraySegs.
func RemoveArrayItem(root *model.Entity, arraySegs []nav.PropSegment, index int) error {
	v, err := GetValueAtPath(root, arraySegs)
	if err != nil {
		return err
	}
	if v.Kind != model.KindArray {
		return fmt.Errorf("panes: cannot remove item: not an array")
	}
	if index < 0 || index >= len(v.ArrayValue) {
		return fmt.Errorf("panes: array index %d out of range", index)
	}
	v.ArrayValue = append(v.ArrayValue[:index], v.ArrayValue[index+1:]...)
	return SetValueAtPath(root, arraySegs, v)
}

func getChild(v model.Value, seg nav.PropSegment) (model.Value, error) {
	if seg.IsIndex {
		if v.Kind != model.KindArray {
			return model.Value{}, fmt.Errorf("panes: not an array")
		}
		if seg.Index < 0 || seg.Index >= len(v.ArrayValue) {
			return model.Value{}, fmt.Errorf("panes: array index %d out of range", seg.Index)
		}
		return v.ArrayValue[seg.Index], nil
	}
	if v.Kind != model.KindEntity || v.EntityValue == nil {
		return model.Value{}, fmt.Errorf("panes: not an entity")
	}
	child, ok := v.EntityValue.Properties[seg.Name]
	if !ok {
		return model.Value{}, fmt.Errorf("panes: no such property %q", seg.Name)
	}
	return child, nil
}

func setChild(v model.Value, seg nav.PropSegment, newChild model.Value) (model.Value, error) {
	if seg.IsIndex {
		if v.Kind != model.KindArray {
			return model.Value{}, fmt.Errorf("panes: not an array")
		}
		if seg.Index < 0 || seg.Index >= len(v.ArrayValue) {
			return model.Value{}, fmt.Errorf("panes: array index %d out of range", seg.Index)
		}
		v.ArrayValue[seg.Index] = newChild
		return v, nil
	}
	if v.Kind != model.KindEntity || v.EntityValue == nil {
		return model.Value{}, fmt.Errorf("panes: not an entity")
	}
	v.EntityValue.Properties[seg.Name] = newChild
	return v, nil
}

func setInValue(v model.Value, segs []nav.PropSegment, newValue model.Value) (model.Value, error) {
	if len(segs) == 0 {
		return newValue, nil
	}
	seg := segs[0]
	child, err := getChild(v, seg)
	if err != nil {
		return model.Value{}, err
	}
	updatedChild, err := setInValue(child, segs[1:], newValue)
	if err != nil {
		return model.Value{}, err
	}
	return setChild(v, seg, updatedChild)
}
