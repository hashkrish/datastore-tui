// Package model holds the typed Datastore domain types (Key, Entity, Value)
// and their JSON (de)serialization matching the Cloud Datastore REST API v1
// wire format (https://cloud.google.com/datastore/docs/reference/data/rest/v1/Key).
package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PathElement is one segment of a Key's ancestor path: a Kind plus either a
// numeric ID or a string Name (never both, per the Datastore data model).
type PathElement struct {
	Kind string
	ID   int64  // 0 if unset (use HasID to distinguish from a real ID of 0... Datastore IDs are always >0, so 0 means unset)
	Name string // "" if unset
}

// HasID reports whether this path element is identified by numeric ID rather
// than by Name.
func (p PathElement) HasID() bool {
	return p.ID != 0
}

func (p PathElement) String() string {
	if p.HasID() {
		return fmt.Sprintf("%s/%d", p.Kind, p.ID)
	}
	return fmt.Sprintf("%s/%s", p.Kind, p.Name)
}

// Key identifies an Entity: a namespaced project plus an ancestor Path, the
// last element of which is the entity's own Kind/ID-or-Name.
type Key struct {
	ProjectID   string
	NamespaceID string // "" means the default namespace
	Path        []PathElement
}

// Kind returns this key's own kind (the last path element's kind), or "" if
// the key has no path elements.
func (k *Key) Kind() string {
	if k == nil || len(k.Path) == 0 {
		return ""
	}
	return k.Path[len(k.Path)-1].Kind
}

// Last returns this key's own path element (its kind + ID/Name).
func (k *Key) Last() PathElement {
	if k == nil || len(k.Path) == 0 {
		return PathElement{}
	}
	return k.Path[len(k.Path)-1]
}

// IsIncomplete reports whether the key's own path element has no ID or Name
// assigned yet (the case for a not-yet-inserted entity awaiting an
// auto-allocated ID from the server).
func (k *Key) IsIncomplete() bool {
	last := k.Last()
	return last.ID == 0 && last.Name == ""
}

// Ancestors returns the path elements above this key's own element, root
// first.
func (k *Key) Ancestors() []PathElement {
	if k == nil || len(k.Path) <= 1 {
		return nil
	}
	return k.Path[:len(k.Path)-1]
}

// String renders a breadcrumb-style path, e.g. "Namespace/Company/1/Employee/42".
func (k *Key) String() string {
	if k == nil {
		return ""
	}
	parts := make([]string, 0, len(k.Path)+1)
	if k.NamespaceID != "" {
		parts = append(parts, k.NamespaceID)
	}
	for _, p := range k.Path {
		parts = append(parts, p.String())
	}
	return strings.Join(parts, "/")
}

// wire structs mirror the REST API JSON shape exactly.

type wirePathElement struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"` // int64 encoded as string to avoid JS precision loss
	Name string `json:"name,omitempty"`
}

type wirePartitionID struct {
	ProjectID   string `json:"projectId"`
	NamespaceID string `json:"namespaceId,omitempty"`
}

type wireKey struct {
	PartitionID *wirePartitionID  `json:"partitionId,omitempty"`
	Path        []wirePathElement `json:"path"`
}

// MarshalJSON implements the Datastore REST API's Key wire format.
func (k Key) MarshalJSON() ([]byte, error) {
	w := wireKey{
		Path: make([]wirePathElement, len(k.Path)),
	}
	if k.ProjectID != "" || k.NamespaceID != "" {
		w.PartitionID = &wirePartitionID{ProjectID: k.ProjectID, NamespaceID: k.NamespaceID}
	}
	for i, p := range k.Path {
		wp := wirePathElement{Kind: p.Kind}
		if p.HasID() {
			wp.ID = fmt.Sprintf("%d", p.ID)
		} else {
			wp.Name = p.Name
		}
		w.Path[i] = wp
	}
	return json.Marshal(w)
}

// UnmarshalJSON parses the Datastore REST API's Key wire format.
func (k *Key) UnmarshalJSON(data []byte) error {
	var w wireKey
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	if w.PartitionID != nil {
		k.ProjectID = w.PartitionID.ProjectID
		k.NamespaceID = w.PartitionID.NamespaceID
	}
	k.Path = make([]PathElement, len(w.Path))
	for i, wp := range w.Path {
		p := PathElement{Kind: wp.Kind, Name: wp.Name}
		if wp.ID != "" {
			var id int64
			if _, err := fmt.Sscanf(wp.ID, "%d", &id); err != nil {
				return fmt.Errorf("key path element %d: invalid id %q: %w", i, wp.ID, err)
			}
			p.ID = id
		}
		k.Path[i] = p
	}
	return nil
}
