package v2

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// Link is one entry of Translation.A: the source word index(es) a target token
// renders. The zero value is the unaligned marker (JSON null); Inserted marks
// a token with no source counterpart (JSON []). Words holds one index for an
// int link and several, ascending, for an int[] link (spec §6.4).
type Link struct {
	Words    []int
	Inserted bool
}

// Null, Insert and Single build the three simple link forms.
func NullLink() Link        { return Link{} }
func InsertedLink() Link    { return Link{Inserted: true} }
func Single(i int) Link     { return Link{Words: []int{i}} }
func Multi(idx ...int) Link { return Link{Words: idx} }

// IsNull reports the unaligned marker.
func (l Link) IsNull() bool { return !l.Inserted && len(l.Words) == 0 }

// Linked reports whether the link names at least one source word.
func (l Link) Linked() bool { return len(l.Words) > 0 }

// Contains reports whether the link names source word i.
func (l Link) Contains(i int) bool {
	for _, w := range l.Words {
		if w == i {
			return true
		}
	}
	return false
}

func (l Link) MarshalJSON() ([]byte, error) {
	switch {
	case l.Inserted:
		return []byte("[]"), nil
	case len(l.Words) == 0:
		return []byte("null"), nil
	case len(l.Words) == 1:
		return []byte(strconv.Itoa(l.Words[0])), nil
	}
	return json.Marshal(l.Words)
}

func (l *Link) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case bytes.Equal(b, []byte("null")):
		*l = Link{}
		return nil
	case len(b) > 0 && b[0] == '[':
		var idx []int
		if err := json.Unmarshal(b, &idx); err != nil {
			return err
		}
		if len(idx) == 0 {
			*l = Link{Inserted: true}
		} else {
			*l = Link{Words: idx}
		}
		return nil
	}
	var i int
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*l = Link{Words: []int{i}}
	return nil
}
