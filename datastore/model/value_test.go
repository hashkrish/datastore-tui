package model

import (
	"encoding/json"
	"testing"
	"time"
)

// roundTrip marshals v, unmarshals the result into a fresh Value, and
// returns it alongside the raw JSON (useful for assertions on wire shape).
func roundTrip(t *testing.T, v Value) (Value, []byte) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Value
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal(%s): %v", data, err)
	}
	return got, data
}

func TestValueRoundTrip_Scalars(t *testing.T) {
	ts := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)

	cases := []struct {
		name string
		in   Value
	}{
		{"null", NullValue()},
		{"boolean true", BooleanValue(true)},
		{"boolean false", BooleanValue(false)},
		{"integer zero", IntegerValue(0)},
		{"integer negative", IntegerValue(-42)},
		{"integer max int64", IntegerValue(9223372036854775807)},
		{"integer min int64", IntegerValue(-9223372036854775808)},
		{"double", DoubleValue(3.14159)},
		{"double negative", DoubleValue(-2.5)},
		{"timestamp", TimestampValue(ts)},
		{"string", StringValue("hello, 世界")},
		{"string empty", StringValue("")},
		{"blob", BlobValueOf([]byte{0x00, 0xFF, 0x10, 0xAB})},
		{"blob empty", BlobValueOf(nil)},
		{"geoPoint", GeoPointValueOf(37.4224, -122.0841)},
		{"key", KeyValueOf(&Key{ProjectID: "proj", Path: []PathElement{{Kind: "Kind", Name: "name1"}}})},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := roundTrip(t, c.in)
			assertValueEqual(t, c.in, got)
		})
	}
}

func TestValueRoundTrip_Array(t *testing.T) {
	in := ArrayValueOf([]Value{
		StringValue("a"),
		IntegerValue(1),
		BooleanValue(true),
		ArrayValueOf([]Value{StringValue("nested")}),
	})
	got, _ := roundTrip(t, in)
	assertValueEqual(t, in, got)
}

func TestValueRoundTrip_EmptyArray(t *testing.T) {
	in := ArrayValueOf(nil)
	got, _ := roundTrip(t, in)
	if got.Kind != KindArray {
		t.Fatalf("Kind = %v, want KindArray", got.Kind)
	}
	if len(got.ArrayValue) != 0 {
		t.Fatalf("ArrayValue = %v, want empty", got.ArrayValue)
	}
}

func TestValueRoundTrip_Entity(t *testing.T) {
	in := EntityValueOf(&Entity{
		Key: &Key{Path: []PathElement{{Kind: "Address", Name: "home"}}},
		Properties: map[string]Value{
			"city": StringValue("Springfield"),
			"zip":  StringValue("12345"),
		},
	})
	got, _ := roundTrip(t, in)
	assertValueEqual(t, in, got)
}

func TestValueRoundTrip_NestedArrayOfEntities(t *testing.T) {
	in := ArrayValueOf([]Value{
		EntityValueOf(&Entity{Properties: map[string]Value{"n": IntegerValue(1)}}),
		EntityValueOf(&Entity{Properties: map[string]Value{"n": IntegerValue(2)}}),
	})
	got, _ := roundTrip(t, in)
	assertValueEqual(t, in, got)
}

func TestValueRoundTrip_ExcludeFromIndexes(t *testing.T) {
	in := StringValue("long text")
	in.ExcludeFromIndexes = true
	got, _ := roundTrip(t, in)
	if !got.ExcludeFromIndexes {
		t.Fatalf("ExcludeFromIndexes not preserved across round trip")
	}
}

// TestValueWireFormat_IntegerIsJSONString locks in the REST API's
// requirement that integerValue be encoded as a JSON string, not a number,
// to avoid float64 precision loss for large int64s in JS-based clients.
func TestValueWireFormat_IntegerIsJSONString(t *testing.T) {
	_, data := roundTrip(t, IntegerValue(9223372036854775807))
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	iv, ok := raw["integerValue"]
	if !ok {
		t.Fatalf("wire JSON missing integerValue field: %s", data)
	}
	if iv[0] != '"' {
		t.Fatalf("integerValue = %s, want a JSON string (quoted), not a bare number", iv)
	}
}

// TestValueWireFormat_RunQueryFieldName guards against regressing the bug
// where the REST client used to send "structuredQuery" instead of the
// actual API field name "query" (see datastore/client/query.go).
func TestValueWireFormat_NullValueField(t *testing.T) {
	_, data := roundTrip(t, NullValue())
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := raw["nullValue"]; !ok {
		t.Fatalf("wire JSON missing nullValue field: %s", data)
	}
}

func TestValueUnmarshal_UnknownShapeDefaultsToNull(t *testing.T) {
	var v Value
	if err := json.Unmarshal([]byte(`{}`), &v); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if v.Kind != KindNull {
		t.Fatalf("Kind = %v, want KindNull for an empty object", v.Kind)
	}
}

func TestValueMarshal_InvalidKind(t *testing.T) {
	v := Value{Kind: ValueKind(999)}
	if _, err := json.Marshal(v); err == nil {
		t.Fatalf("Marshal succeeded for an invalid Kind, want an error")
	}
}

// assertValueEqual compares the fields relevant to want.Kind; it does not
// do a blanket reflect.DeepEqual since irrelevant zero-value fields (e.g.
// IntegerValue on a StringValue) are expected to differ across construction
// paths.
func assertValueEqual(t *testing.T, want, got Value) {
	t.Helper()
	if want.Kind != got.Kind {
		t.Fatalf("Kind = %v, want %v", got.Kind, want.Kind)
	}
	if want.ExcludeFromIndexes != got.ExcludeFromIndexes {
		t.Fatalf("ExcludeFromIndexes = %v, want %v", got.ExcludeFromIndexes, want.ExcludeFromIndexes)
	}
	switch want.Kind {
	case KindNull:
		// nothing further to compare
	case KindBoolean:
		if want.BooleanValue != got.BooleanValue {
			t.Fatalf("BooleanValue = %v, want %v", got.BooleanValue, want.BooleanValue)
		}
	case KindInteger:
		if want.IntegerValue != got.IntegerValue {
			t.Fatalf("IntegerValue = %v, want %v", got.IntegerValue, want.IntegerValue)
		}
	case KindDouble:
		if want.DoubleValue != got.DoubleValue {
			t.Fatalf("DoubleValue = %v, want %v", got.DoubleValue, want.DoubleValue)
		}
	case KindTimestamp:
		if !want.TimestampValue.Equal(got.TimestampValue) {
			t.Fatalf("TimestampValue = %v, want %v", got.TimestampValue, want.TimestampValue)
		}
	case KindString:
		if want.StringValue != got.StringValue {
			t.Fatalf("StringValue = %q, want %q", got.StringValue, want.StringValue)
		}
	case KindBlob:
		if string(want.BlobValue) != string(got.BlobValue) {
			t.Fatalf("BlobValue = %v, want %v", got.BlobValue, want.BlobValue)
		}
	case KindGeoPoint:
		if want.GeoPointValue != got.GeoPointValue {
			t.Fatalf("GeoPointValue = %v, want %v", got.GeoPointValue, want.GeoPointValue)
		}
	case KindKey:
		if want.KeyValue.String() != got.KeyValue.String() {
			t.Fatalf("KeyValue = %v, want %v", got.KeyValue, want.KeyValue)
		}
	case KindEntity:
		assertEntityEqual(t, want.EntityValue, got.EntityValue)
	case KindArray:
		if len(want.ArrayValue) != len(got.ArrayValue) {
			t.Fatalf("ArrayValue len = %d, want %d", len(got.ArrayValue), len(want.ArrayValue))
		}
		for i := range want.ArrayValue {
			assertValueEqual(t, want.ArrayValue[i], got.ArrayValue[i])
		}
	}
}

func assertEntityEqual(t *testing.T, want, got *Entity) {
	t.Helper()
	if want == nil || got == nil {
		if want != got {
			t.Fatalf("Entity = %v, want %v", got, want)
		}
		return
	}
	if len(want.Properties) != len(got.Properties) {
		t.Fatalf("Properties len = %d, want %d", len(got.Properties), len(want.Properties))
	}
	for name, wv := range want.Properties {
		gv, ok := got.Properties[name]
		if !ok {
			t.Fatalf("Properties missing key %q", name)
		}
		assertValueEqual(t, wv, gv)
	}
}
