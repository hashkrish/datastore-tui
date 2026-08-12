package panes

import (
	"testing"

	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/nav"
)

// fixtureEntity builds a Person entity with a mix of scalar, array, and
// nested-embedded-entity properties for exercising path get/set/append.
func fixtureEntity() *model.Entity {
	return &model.Entity{
		Key: &model.Key{Path: []model.PathElement{{Kind: "Person", Name: "alice"}}},
		Properties: map[string]model.Value{
			"name": model.StringValue("Alice"),
			"age":  model.IntegerValue(30),
			"tags": model.ArrayValueOf([]model.Value{
				model.StringValue("admin"),
				model.StringValue("vip"),
			}),
			"address": model.EntityValueOf(&model.Entity{
				Properties: map[string]model.Value{
					"city": model.StringValue("Springfield"),
					"zip":  model.StringValue("12345"),
				},
			}),
			"contacts": model.ArrayValueOf([]model.Value{
				model.EntityValueOf(&model.Entity{Properties: map[string]model.Value{
					"phone": model.StringValue("555-1234"),
				}}),
			}),
		},
	}
}

func name(n string) nav.PropSegment { return nav.PropSegment{Name: n} }
func index(i int) nav.PropSegment   { return nav.PropSegment{IsIndex: true, Index: i} }

func TestGetValueAtPath_Root(t *testing.T) {
	e := fixtureEntity()
	v, err := GetValueAtPath(e, nil)
	if err != nil {
		t.Fatalf("GetValueAtPath: %v", err)
	}
	if v.Kind != model.KindEntity || v.EntityValue != e {
		t.Fatalf("GetValueAtPath(nil) = %+v, want the root entity wrapped as KindEntity", v)
	}
}

func TestGetValueAtPath_TopLevelScalar(t *testing.T) {
	e := fixtureEntity()
	v, err := GetValueAtPath(e, []nav.PropSegment{name("name")})
	if err != nil {
		t.Fatalf("GetValueAtPath: %v", err)
	}
	if v.Kind != model.KindString || v.StringValue != "Alice" {
		t.Fatalf("GetValueAtPath(name) = %+v, want StringValue Alice", v)
	}
}

func TestGetValueAtPath_ArrayIndex(t *testing.T) {
	e := fixtureEntity()
	v, err := GetValueAtPath(e, []nav.PropSegment{name("tags"), index(1)})
	if err != nil {
		t.Fatalf("GetValueAtPath: %v", err)
	}
	if v.Kind != model.KindString || v.StringValue != "vip" {
		t.Fatalf("GetValueAtPath(tags[1]) = %+v, want StringValue vip", v)
	}
}

func TestGetValueAtPath_NestedEntity(t *testing.T) {
	e := fixtureEntity()
	v, err := GetValueAtPath(e, []nav.PropSegment{name("address"), name("city")})
	if err != nil {
		t.Fatalf("GetValueAtPath: %v", err)
	}
	if v.Kind != model.KindString || v.StringValue != "Springfield" {
		t.Fatalf("GetValueAtPath(address/city) = %+v, want StringValue Springfield", v)
	}
}

func TestGetValueAtPath_ArrayOfEntities(t *testing.T) {
	e := fixtureEntity()
	v, err := GetValueAtPath(e, []nav.PropSegment{name("contacts"), index(0), name("phone")})
	if err != nil {
		t.Fatalf("GetValueAtPath: %v", err)
	}
	if v.Kind != model.KindString || v.StringValue != "555-1234" {
		t.Fatalf("GetValueAtPath(contacts[0]/phone) = %+v, want StringValue 555-1234", v)
	}
}

func TestGetValueAtPath_Errors(t *testing.T) {
	e := fixtureEntity()
	cases := []struct {
		name string
		segs []nav.PropSegment
	}{
		{"missing property", []nav.PropSegment{name("nope")}},
		{"index on non-array", []nav.PropSegment{name("name"), index(0)}},
		{"name on non-entity", []nav.PropSegment{name("tags"), name("nope")}},
		{"array index out of range", []nav.PropSegment{name("tags"), index(99)}},
		{"root segment is index", []nav.PropSegment{index(0)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := GetValueAtPath(e, c.segs); err == nil {
				t.Fatalf("GetValueAtPath(%v) succeeded, want an error", c.segs)
			}
		})
	}
}

func TestSetValueAtPath_TopLevel(t *testing.T) {
	e := fixtureEntity()
	if err := SetValueAtPath(e, []nav.PropSegment{name("name")}, model.StringValue("Bob")); err != nil {
		t.Fatalf("SetValueAtPath: %v", err)
	}
	if e.Properties["name"].StringValue != "Bob" {
		t.Fatalf("Properties[name] = %+v, want Bob", e.Properties["name"])
	}
}

func TestSetValueAtPath_Nested(t *testing.T) {
	e := fixtureEntity()
	segs := []nav.PropSegment{name("address"), name("city")}
	if err := SetValueAtPath(e, segs, model.StringValue("Shelbyville")); err != nil {
		t.Fatalf("SetValueAtPath: %v", err)
	}
	got, err := GetValueAtPath(e, segs)
	if err != nil {
		t.Fatalf("GetValueAtPath: %v", err)
	}
	if got.StringValue != "Shelbyville" {
		t.Fatalf("address/city = %q, want Shelbyville", got.StringValue)
	}
	// Sibling property must survive the nested write untouched.
	zip, err := GetValueAtPath(e, []nav.PropSegment{name("address"), name("zip")})
	if err != nil {
		t.Fatalf("GetValueAtPath(zip): %v", err)
	}
	if zip.StringValue != "12345" {
		t.Fatalf("address/zip = %q, want unchanged 12345", zip.StringValue)
	}
}

func TestSetValueAtPath_ArrayElement(t *testing.T) {
	e := fixtureEntity()
	segs := []nav.PropSegment{name("tags"), index(0)}
	if err := SetValueAtPath(e, segs, model.StringValue("owner")); err != nil {
		t.Fatalf("SetValueAtPath: %v", err)
	}
	tags := e.Properties["tags"].ArrayValue
	if tags[0].StringValue != "owner" || tags[1].StringValue != "vip" {
		t.Fatalf("tags = %v, want [owner vip]", tags)
	}
}

func TestSetValueAtPath_EmptyPathErrors(t *testing.T) {
	e := fixtureEntity()
	if err := SetValueAtPath(e, nil, model.StringValue("x")); err == nil {
		t.Fatalf("SetValueAtPath(nil path) succeeded, want an error")
	}
}

func TestAppendArrayItem(t *testing.T) {
	e := fixtureEntity()
	segs := []nav.PropSegment{name("tags")}
	if err := AppendArrayItem(e, segs, model.StringValue("editor")); err != nil {
		t.Fatalf("AppendArrayItem: %v", err)
	}
	tags := e.Properties["tags"].ArrayValue
	if len(tags) != 3 || tags[2].StringValue != "editor" {
		t.Fatalf("tags = %v, want [admin vip editor]", tags)
	}
}

func TestAppendArrayItem_NotAnArray(t *testing.T) {
	e := fixtureEntity()
	if err := AppendArrayItem(e, []nav.PropSegment{name("name")}, model.StringValue("x")); err == nil {
		t.Fatalf("AppendArrayItem on a non-array succeeded, want an error")
	}
}

func TestRemoveArrayItem(t *testing.T) {
	e := fixtureEntity()
	if err := RemoveArrayItem(e, []nav.PropSegment{name("tags")}, 0); err != nil {
		t.Fatalf("RemoveArrayItem: %v", err)
	}
	tags := e.Properties["tags"].ArrayValue
	if len(tags) != 1 || tags[0].StringValue != "vip" {
		t.Fatalf("tags = %v, want [vip]", tags)
	}
}

func TestRemoveArrayItem_OutOfRange(t *testing.T) {
	e := fixtureEntity()
	if err := RemoveArrayItem(e, []nav.PropSegment{name("tags")}, 99); err == nil {
		t.Fatalf("RemoveArrayItem(out of range) succeeded, want an error")
	}
}

func TestRemoveArrayItem_NestedArrayOfEntities(t *testing.T) {
	e := fixtureEntity()
	segs := []nav.PropSegment{name("contacts")}
	if err := AppendArrayItem(e, segs, model.EntityValueOf(&model.Entity{
		Properties: map[string]model.Value{"phone": model.StringValue("555-9999")},
	})); err != nil {
		t.Fatalf("AppendArrayItem: %v", err)
	}
	if err := RemoveArrayItem(e, segs, 0); err != nil {
		t.Fatalf("RemoveArrayItem: %v", err)
	}
	contacts := e.Properties["contacts"].ArrayValue
	if len(contacts) != 1 {
		t.Fatalf("contacts len = %d, want 1", len(contacts))
	}
	phone := contacts[0].EntityValue.Properties["phone"].StringValue
	if phone != "555-9999" {
		t.Fatalf("remaining contact phone = %q, want 555-9999", phone)
	}
}
