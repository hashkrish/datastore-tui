// Package edit provides huh-based edit forms for scalar Datastore property
// values, plus the save/commit step that writes an edited entity back.
package edit

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/huh"
	"github.com/krishnan/datastore-tui/datastore/model"
)

// FieldEditor wraps a huh.Form for editing one scalar Value, plus the
// scratch fields the form's inputs are bound to (huh binds by pointer, so
// these must outlive the form).
type FieldEditor struct {
	kind model.ValueKind
	form *huh.Form

	strVal    string
	boolVal   bool
	intVal    string
	doubleVal string
	tsVal     string
	latVal    string
	lngVal    string
	blobVal   string
	// blobShowable is true when the blob's bytes decoded (see
	// decodeBlobText) to displayable text rather than falling back to
	// base64 — Result() must mirror whichever path NewFieldEditor took.
	blobShowable bool
	keyKind      string
	keyID        string
	keyName      string
	// keyBase preserves the ProjectID/NamespaceID and any ancestor path
	// segments of the Key being edited — the kind/ID/name form fields only
	// ever expose the leaf path element, so Result() splices the edited
	// leaf back onto this rather than discarding the rest of the key.
	keyBase *model.Key
}

// NewFieldEditor builds an edit form for v, sized to width (the full
// terminal width, so text/blob editors aren't squeezed to huh's narrower
// default). v must be a scalar leaf (anything but KindEntity/KindArray,
// which are containers navigated via the detail view rather than edited
// directly); ok is false otherwise.
func NewFieldEditor(v model.Value, width int) (*FieldEditor, bool) {
	e := &FieldEditor{kind: v.Kind}
	switch v.Kind {
	case model.KindString:
		e.strVal = v.StringValue
		e.form = huh.NewForm(huh.NewGroup(
			huh.NewText().Title("String value").Value(&e.strVal),
		))
	case model.KindBoolean:
		e.boolVal = v.BooleanValue
		e.form = huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Boolean value").Affirmative("true").Negative("false").Value(&e.boolVal),
		))
	case model.KindInteger:
		e.intVal = strconv.FormatInt(v.IntegerValue, 10)
		e.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Integer value").Value(&e.intVal).Validate(validateInt),
		))
	case model.KindDouble:
		e.doubleVal = strconv.FormatFloat(v.DoubleValue, 'g', -1, 64)
		e.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Double value").Value(&e.doubleVal).Validate(validateFloat),
		))
	case model.KindTimestamp:
		e.tsVal = v.TimestampValue.UTC().Format(time.RFC3339)
		e.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Timestamp (RFC3339)").Value(&e.tsVal).Validate(validateTimestamp),
		))
	case model.KindGeoPoint:
		e.latVal = strconv.FormatFloat(v.GeoPointValue.Latitude, 'g', -1, 64)
		e.lngVal = strconv.FormatFloat(v.GeoPointValue.Longitude, 'g', -1, 64)
		e.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Latitude").Value(&e.latVal).Validate(validateFloat),
			huh.NewInput().Title("Longitude").Value(&e.lngVal).Validate(validateFloat),
		))
	case model.KindBlob:
		if text, ok := decodeBlobText(v.BlobValue); ok {
			e.blobShowable = true
			e.blobVal = text
			e.form = huh.NewForm(huh.NewGroup(
				huh.NewText().Title(blobEditTitle(text)).Value(&e.blobVal),
			))
		} else {
			e.blobVal = base64.StdEncoding.EncodeToString(v.BlobValue)
			e.form = huh.NewForm(huh.NewGroup(
				huh.NewText().Title("Blob value (base64)").Value(&e.blobVal).Validate(validateBase64),
			))
		}
	case model.KindKey:
		e.SetKeyValue(v.KeyValue)
		e.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Key kind").Value(&e.keyKind),
			huh.NewInput().Title("ID (numeric; leave blank to use Name instead)").Value(&e.keyID),
			huh.NewInput().Title("Name (leave blank to use ID instead)").Value(&e.keyName),
		))
	case model.KindNull:
		e.form = huh.NewForm(huh.NewGroup(
			huh.NewNote().Title("Null value").Description("This property has no value to edit."),
		))
	default:
		return nil, false
	}
	if width > 0 {
		e.form = e.form.WithWidth(width)
	}
	return e, true
}

// Form returns the underlying huh.Form to embed as a tea.Model.
func (e *FieldEditor) Form() *huh.Form { return e.form }

// SetKeyValue populates a KindKey editor's kind/ID-or-name fields from k,
// leaving them blank if k is nil. Exported so a caller can fill in a
// Key-typed value editor from a source other than typing — e.g. the query
// filter's paste-from-bookmark picker inserting a bookmarked entity's Key
// without reaching into unexported fields.
func (e *FieldEditor) SetKeyValue(k *model.Key) {
	e.keyBase = k
	e.keyKind, e.keyID, e.keyName = "", "", ""
	if k == nil {
		return
	}
	last := k.Last()
	e.keyKind = last.Kind
	if last.HasID() {
		e.keyID = strconv.FormatInt(last.ID, 10)
	} else {
		e.keyName = last.Name
	}
}

// Result converts the form's current field values back into a model.Value.
// Call only after Form().State() == huh.StateCompleted.
func (e *FieldEditor) Result() (model.Value, error) {
	switch e.kind {
	case model.KindString:
		return model.StringValue(e.strVal), nil
	case model.KindBoolean:
		return model.BooleanValue(e.boolVal), nil
	case model.KindInteger:
		i, err := strconv.ParseInt(e.intVal, 10, 64)
		if err != nil {
			return model.Value{}, err
		}
		return model.IntegerValue(i), nil
	case model.KindDouble:
		f, err := strconv.ParseFloat(e.doubleVal, 64)
		if err != nil {
			return model.Value{}, err
		}
		return model.DoubleValue(f), nil
	case model.KindTimestamp:
		t, err := time.Parse(time.RFC3339, e.tsVal)
		if err != nil {
			return model.Value{}, err
		}
		return model.TimestampValue(t), nil
	case model.KindGeoPoint:
		lat, err := strconv.ParseFloat(e.latVal, 64)
		if err != nil {
			return model.Value{}, err
		}
		lng, err := strconv.ParseFloat(e.lngVal, 64)
		if err != nil {
			return model.Value{}, err
		}
		return model.GeoPointValueOf(lat, lng), nil
	case model.KindBlob:
		if e.blobShowable {
			return model.BlobValueOf(encodeBlobText(e.blobVal)), nil
		}
		b, err := base64.StdEncoding.DecodeString(e.blobVal)
		if err != nil {
			return model.Value{}, err
		}
		return model.BlobValueOf(b), nil
	case model.KindKey:
		if e.keyKind == "" {
			return model.Value{}, fmt.Errorf("edit: key kind is required")
		}
		pe := model.PathElement{Kind: e.keyKind}
		if e.keyID != "" {
			id, err := strconv.ParseInt(e.keyID, 10, 64)
			if err != nil {
				return model.Value{}, err
			}
			pe.ID = id
		} else {
			pe.Name = e.keyName
		}
		key := &model.Key{Path: []model.PathElement{pe}}
		if e.keyBase != nil {
			key.ProjectID = e.keyBase.ProjectID
			key.NamespaceID = e.keyBase.NamespaceID
			if ancestors := len(e.keyBase.Path) - 1; ancestors > 0 {
				key.Path = append(append([]model.PathElement{}, e.keyBase.Path[:ancestors]...), pe)
			}
		}
		return model.KeyValueOf(key), nil
	case model.KindNull:
		return model.NullValue(), nil
	default:
		return model.Value{}, fmt.Errorf("edit: kind %s is not an editable leaf", e.kind)
	}
}

func validateInt(s string) error {
	_, err := strconv.ParseInt(s, 10, 64)
	return err
}

func validateFloat(s string) error {
	_, err := strconv.ParseFloat(s, 64)
	return err
}

func validateTimestamp(s string) error {
	_, err := time.Parse(time.RFC3339, s)
	return err
}

func validateBase64(s string) error {
	_, err := base64.StdEncoding.DecodeString(s)
	return err
}

// decodeBlobText decodes b to text for editing when it's displayable —
// valid UTF-8 with no non-whitespace control characters — pretty-printing
// it with a two-space indent first if it's valid JSON. ok is false for
// binary data, which the caller falls back to editing as base64.
func decodeBlobText(b []byte) (text string, ok bool) {
	if !utf8.Valid(b) || !isPrintableText(b) {
		return "", false
	}
	if json.Valid(b) {
		var buf bytes.Buffer
		if err := json.Indent(&buf, b, "", "  "); err == nil {
			return buf.String(), true
		}
	}
	return string(b), true
}

// encodeBlobText reverses decodeBlobText for saving: valid JSON is compacted
// back down before being stored as the blob's raw bytes, since the
// two-space indent decodeBlobText applies is purely a display convenience.
func encodeBlobText(s string) []byte {
	if json.Valid([]byte(s)) {
		var buf bytes.Buffer
		if err := json.Compact(&buf, []byte(s)); err == nil {
			return buf.Bytes()
		}
	}
	return []byte(s)
}

// blobEditTitle labels the blob edit field by what it actually contains.
func blobEditTitle(text string) string {
	if json.Valid([]byte(text)) {
		return "Blob value (JSON)"
	}
	return "Blob value (text)"
}

// isPrintableText reports whether b contains only displayable characters —
// any control character other than \n, \r, or \t disqualifies it as binary.
func isPrintableText(b []byte) bool {
	for _, r := range string(b) {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
