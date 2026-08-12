package model

import (
	"encoding/json"
	"testing"
)

func TestKeyRoundTrip_ByName(t *testing.T) {
	in := Key{
		ProjectID:   "my-project",
		NamespaceID: "tenant-a",
		Path:        []PathElement{{Kind: "Person", Name: "alice"}},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Key
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal(%s): %v", data, err)
	}
	if got.ProjectID != in.ProjectID || got.NamespaceID != in.NamespaceID {
		t.Fatalf("got %+v, want %+v", got, in)
	}
	if len(got.Path) != 1 || got.Path[0].Kind != "Person" || got.Path[0].Name != "alice" || got.Path[0].HasID() {
		t.Fatalf("Path = %+v, want [{Kind:Person Name:alice}]", got.Path)
	}
}

func TestKeyRoundTrip_ByID(t *testing.T) {
	in := Key{Path: []PathElement{{Kind: "Person", ID: 4548638770462720}}}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Key
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal(%s): %v", data, err)
	}
	if !got.Path[0].HasID() || got.Path[0].ID != 4548638770462720 {
		t.Fatalf("Path[0] = %+v, want ID 4548638770462720", got.Path[0])
	}
}

func TestKeyRoundTrip_AncestorPath(t *testing.T) {
	in := Key{Path: []PathElement{
		{Kind: "Company", Name: "acme"},
		{Kind: "Employee", ID: 42},
	}}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Key
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal(%s): %v", data, err)
	}
	if len(got.Path) != 2 {
		t.Fatalf("Path len = %d, want 2", len(got.Path))
	}
	if got.Kind() != "Employee" {
		t.Fatalf("Kind() = %q, want Employee", got.Kind())
	}
	ancestors := got.Ancestors()
	if len(ancestors) != 1 || ancestors[0].Kind != "Company" || ancestors[0].Name != "acme" {
		t.Fatalf("Ancestors() = %+v, want [{Kind:Company Name:acme}]", ancestors)
	}
}

func TestKeyMarshal_IDTakesPrecedenceOverName(t *testing.T) {
	// A PathElement should never have both set in practice, but if ID is
	// non-zero, HasID must win so the wire form doesn't emit both fields.
	p := PathElement{Kind: "Person", ID: 1, Name: "should-be-ignored"}
	if !p.HasID() {
		t.Fatalf("HasID() = false, want true when ID is set")
	}
}

func TestKey_IsIncomplete(t *testing.T) {
	incomplete := &Key{Path: []PathElement{{Kind: "Person"}}}
	if !incomplete.IsIncomplete() {
		t.Fatalf("IsIncomplete() = false, want true for a key with no ID/Name")
	}
	complete := &Key{Path: []PathElement{{Kind: "Person", Name: "alice"}}}
	if complete.IsIncomplete() {
		t.Fatalf("IsIncomplete() = true, want false for a key with a Name")
	}
}

func TestKey_String(t *testing.T) {
	k := &Key{
		NamespaceID: "tenant-a",
		Path: []PathElement{
			{Kind: "Company", Name: "acme"},
			{Kind: "Employee", ID: 42},
		},
	}
	want := "tenant-a/Company/acme/Employee/42"
	if got := k.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestKey_StringNilSafe(t *testing.T) {
	var k *Key
	if got := k.String(); got != "" {
		t.Fatalf("String() on nil Key = %q, want empty string", got)
	}
	if got := k.Kind(); got != "" {
		t.Fatalf("Kind() on nil Key = %q, want empty string", got)
	}
	if got := k.Last(); got != (PathElement{}) {
		t.Fatalf("Last() on nil Key = %+v, want zero value", got)
	}
}
