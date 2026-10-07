package model

import (
	"bytes"
	"encoding/json"
	"unicode"
	"unicode/utf8"
)

// DecodeBlobText decodes b to text when it's displayable — valid UTF-8 with
// no non-whitespace control characters — pretty-printing it with a
// two-space indent first if it's valid JSON. ok is false for binary data.
func DecodeBlobText(b []byte) (text string, ok bool) {
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
