// Package jsonx marshals JSON as UTF-8 without escaping <, >, & — matching the
// Python producer's ensure_ascii=False output and keeping non-ASCII text
// (Cyrillic, etc.) readable in the archive.
package jsonx

import (
	"bytes"
	"encoding/json"
)

// Marshal encodes v compactly without HTML escaping.
func Marshal(v any) ([]byte, error) { return encode(v, "") }

// MarshalIndent encodes v with the given indent, without HTML escaping.
func MarshalIndent(v any, indent string) ([]byte, error) { return encode(v, indent) }

func encode(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	b := buf.Bytes()
	if n := len(b); n > 0 && b[n-1] == '\n' { // Encoder appends a newline.
		b = b[:n-1]
	}
	return b, nil
}
