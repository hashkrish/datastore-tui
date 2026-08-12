package model

import "encoding/json"

// Entity is a Datastore entity: a Key plus its typed properties.
type Entity struct {
	Key        *Key
	Properties map[string]Value
}

type wireEntity struct {
	Key        *Key             `json:"key,omitempty"`
	Properties map[string]Value `json:"properties,omitempty"`
}

func toWireEntity(e *Entity) wireEntity {
	return wireEntity{Key: e.Key, Properties: e.Properties}
}

func fromWireEntity(w *wireEntity) *Entity {
	if w == nil {
		return nil
	}
	return &Entity{Key: w.Key, Properties: w.Properties}
}

// MarshalJSON implements the Datastore REST API's Entity wire format.
func (e Entity) MarshalJSON() ([]byte, error) {
	return json.Marshal(toWireEntity(&e))
}

// UnmarshalJSON parses the Datastore REST API's Entity wire format.
func (e *Entity) UnmarshalJSON(data []byte) error {
	var w wireEntity
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	e.Key = w.Key
	e.Properties = w.Properties
	return nil
}
