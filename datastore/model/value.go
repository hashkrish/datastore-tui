package model

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// ValueKind discriminates which field of Value is populated.
type ValueKind int

const (
	KindNull ValueKind = iota
	KindBoolean
	KindInteger
	KindDouble
	KindTimestamp
	KindKey
	KindString
	KindBlob
	KindGeoPoint
	KindEntity
	KindArray
)

func (k ValueKind) String() string {
	switch k {
	case KindNull:
		return "null"
	case KindBoolean:
		return "boolean"
	case KindInteger:
		return "integer"
	case KindDouble:
		return "double"
	case KindTimestamp:
		return "timestamp"
	case KindKey:
		return "key"
	case KindString:
		return "string"
	case KindBlob:
		return "blob"
	case KindGeoPoint:
		return "geoPoint"
	case KindEntity:
		return "entity"
	case KindArray:
		return "array"
	default:
		return "unknown"
	}
}

// LatLng is a geographic point, mirroring the REST API's geoPointValue.
type LatLng struct {
	Latitude  float64
	Longitude float64
}

// Value is a tagged union over every Datastore property value type. Exactly
// one of the typed fields is meaningful, selected by Kind.
type Value struct {
	Kind ValueKind

	BooleanValue   bool
	IntegerValue   int64
	DoubleValue    float64
	TimestampValue time.Time
	KeyValue       *Key
	StringValue    string
	BlobValue      []byte
	GeoPointValue  LatLng
	EntityValue    *Entity
	ArrayValue     []Value

	// ExcludeFromIndexes mirrors the REST field of the same name; Datastore
	// indexes every property by default, and long strings/blobs must opt out.
	ExcludeFromIndexes bool
}

func NullValue() Value                 { return Value{Kind: KindNull} }
func BooleanValue(b bool) Value        { return Value{Kind: KindBoolean, BooleanValue: b} }
func IntegerValue(i int64) Value       { return Value{Kind: KindInteger, IntegerValue: i} }
func DoubleValue(f float64) Value      { return Value{Kind: KindDouble, DoubleValue: f} }
func TimestampValue(t time.Time) Value { return Value{Kind: KindTimestamp, TimestampValue: t} }
func StringValue(s string) Value       { return Value{Kind: KindString, StringValue: s} }
func BlobValueOf(b []byte) Value       { return Value{Kind: KindBlob, BlobValue: b} }
func GeoPointValueOf(lat, lng float64) Value {
	return Value{Kind: KindGeoPoint, GeoPointValue: LatLng{Latitude: lat, Longitude: lng}}
}
func KeyValueOf(k *Key) Value       { return Value{Kind: KindKey, KeyValue: k} }
func EntityValueOf(e *Entity) Value { return Value{Kind: KindEntity, EntityValue: e} }
func ArrayValueOf(vs []Value) Value { return Value{Kind: KindArray, ArrayValue: vs} }

// wireValue mirrors the REST API's Value message: a struct with one
// field-per-type populated, all others omitted.
type wireValue struct {
	NullValue          *string         `json:"nullValue,omitempty"`
	BooleanValue       *bool           `json:"booleanValue,omitempty"`
	IntegerValue       *string         `json:"integerValue,omitempty"` // string per API spec, avoids JS float64 precision loss
	DoubleValue        *float64        `json:"doubleValue,omitempty"`
	TimestampValue     *string         `json:"timestampValue,omitempty"` // RFC3339
	KeyValue           *Key            `json:"keyValue,omitempty"`
	StringValue        *string         `json:"stringValue,omitempty"`
	BlobValue          *string         `json:"blobValue,omitempty"` // base64
	GeoPointValue      *wireGeoPoint   `json:"geoPointValue,omitempty"`
	EntityValue        *wireEntity     `json:"entityValue,omitempty"`
	ArrayValue         *wireArrayValue `json:"arrayValue,omitempty"`
	ExcludeFromIndexes bool            `json:"excludeFromIndexes,omitempty"`
}

type wireGeoPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type wireArrayValue struct {
	Values []Value `json:"values"`
}

// MarshalJSON implements the Datastore REST API's Value wire format.
func (v Value) MarshalJSON() ([]byte, error) {
	w := wireValue{ExcludeFromIndexes: v.ExcludeFromIndexes}
	switch v.Kind {
	case KindNull:
		null := "NULL_VALUE"
		w.NullValue = &null
	case KindBoolean:
		w.BooleanValue = &v.BooleanValue
	case KindInteger:
		s := fmt.Sprintf("%d", v.IntegerValue)
		w.IntegerValue = &s
	case KindDouble:
		w.DoubleValue = &v.DoubleValue
	case KindTimestamp:
		s := v.TimestampValue.UTC().Format(time.RFC3339Nano)
		w.TimestampValue = &s
	case KindKey:
		w.KeyValue = v.KeyValue
	case KindString:
		w.StringValue = &v.StringValue
	case KindBlob:
		s := base64.StdEncoding.EncodeToString(v.BlobValue)
		w.BlobValue = &s
	case KindGeoPoint:
		w.GeoPointValue = &wireGeoPoint{Latitude: v.GeoPointValue.Latitude, Longitude: v.GeoPointValue.Longitude}
	case KindEntity:
		if v.EntityValue != nil {
			we := toWireEntity(v.EntityValue)
			w.EntityValue = &we
		}
	case KindArray:
		w.ArrayValue = &wireArrayValue{Values: v.ArrayValue}
	default:
		return nil, fmt.Errorf("model: cannot marshal Value with unknown Kind %d", v.Kind)
	}
	return json.Marshal(w)
}

// UnmarshalJSON parses the Datastore REST API's Value wire format, selecting
// Kind based on which field is present.
func (v *Value) UnmarshalJSON(data []byte) error {
	var w wireValue
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	v.ExcludeFromIndexes = w.ExcludeFromIndexes

	switch {
	case w.NullValue != nil:
		v.Kind = KindNull
	case w.BooleanValue != nil:
		v.Kind = KindBoolean
		v.BooleanValue = *w.BooleanValue
	case w.IntegerValue != nil:
		v.Kind = KindInteger
		var i int64
		if _, err := fmt.Sscanf(*w.IntegerValue, "%d", &i); err != nil {
			return fmt.Errorf("model: invalid integerValue %q: %w", *w.IntegerValue, err)
		}
		v.IntegerValue = i
	case w.DoubleValue != nil:
		v.Kind = KindDouble
		v.DoubleValue = *w.DoubleValue
	case w.TimestampValue != nil:
		v.Kind = KindTimestamp
		t, err := time.Parse(time.RFC3339Nano, *w.TimestampValue)
		if err != nil {
			return fmt.Errorf("model: invalid timestampValue %q: %w", *w.TimestampValue, err)
		}
		v.TimestampValue = t
	case w.KeyValue != nil:
		v.Kind = KindKey
		v.KeyValue = w.KeyValue
	case w.StringValue != nil:
		v.Kind = KindString
		v.StringValue = *w.StringValue
	case w.BlobValue != nil:
		v.Kind = KindBlob
		b, err := base64.StdEncoding.DecodeString(*w.BlobValue)
		if err != nil {
			return fmt.Errorf("model: invalid blobValue: %w", err)
		}
		v.BlobValue = b
	case w.GeoPointValue != nil:
		v.Kind = KindGeoPoint
		v.GeoPointValue = LatLng{Latitude: w.GeoPointValue.Latitude, Longitude: w.GeoPointValue.Longitude}
	case w.EntityValue != nil:
		v.Kind = KindEntity
		v.EntityValue = fromWireEntity(w.EntityValue)
	case w.ArrayValue != nil:
		v.Kind = KindArray
		v.ArrayValue = w.ArrayValue.Values
	default:
		// Datastore omits nullValue's value entirely for a bare null in some
		// contexts; treat an empty object as null rather than erroring.
		v.Kind = KindNull
	}
	return nil
}
