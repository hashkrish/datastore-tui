package model

import (
	"encoding/json"
	"testing"
)

func TestEntityRoundTrip(t *testing.T) {
	in := Entity{
		Key: &Key{Path: []PathElement{{Kind: "Person", Name: "alice"}}},
		Properties: map[string]Value{
			"name": StringValue("Alice"),
			"age":  IntegerValue(30),
			"tags": ArrayValueOf([]Value{StringValue("admin"), StringValue("vip")}),
			"address": EntityValueOf(&Entity{Properties: map[string]Value{
				"city": StringValue("Springfield"),
			}}),
		},
	}

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Entity
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal(%s): %v", data, err)
	}

	if got.Key.String() != in.Key.String() {
		t.Fatalf("Key = %v, want %v", got.Key, in.Key)
	}
	assertEntityEqual(t, &in, &got)
}

func TestEntityRoundTrip_NoProperties(t *testing.T) {
	in := Entity{Key: &Key{Path: []PathElement{{Kind: "Person", Name: "empty"}}}}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Entity
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal(%s): %v", data, err)
	}
	if len(got.Properties) != 0 {
		t.Fatalf("Properties = %v, want empty", got.Properties)
	}
}
