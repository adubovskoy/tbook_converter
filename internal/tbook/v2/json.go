package v2

import (
	"bytes"
	"encoding/json"
)

// marshal encodes compact JSON without HTML escaping: entries stay readable
// UTF-8 and byte-identical to the spec's examples.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func unmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
