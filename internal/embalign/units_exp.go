package embalign

// EXPERIMENTAL / RESEARCH-ONLY plumbing for the unit-glue lever.
//
// The aligner can be told which source words form one known multi-word
// expression (see request.Units). Production has no such lexicon yet, so for
// the experiment the spans come from a probe file — the same shape the
// alignment eval already uses (bench-quality/probe-dev.json):
//
//	[{"src": "<exact sentence text>", "expr": "<expression as written in src>"}, ...]
//
// LoadUnitsFile turns that into a per-sentence span lookup. Nothing here runs
// unless --units-file / EMBALIGN_UNITS_FILE is set, and the child ignores the
// spans anyway unless EMBALIGN_UNIT_GLUE is set. Delete both together when the
// experiment concludes, or replace this file with a real lexicon source.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"
)

// unitProbe is the one field pair we need out of a probe record.
type unitProbe struct {
	Src  string `json:"src"`
	Expr string `json:"expr"`
}

// LoadUnitsFile reads a probe JSON file and returns a lookup that maps a source
// sentence (plus its word spans, as rune offsets, exactly as the .tbook stores
// them) to expression spans in SOURCE WORD indices, [start, end) — inclusive
// start, exclusive end, the convention tools/embalign.py expects.
//
// Sentences absent from the file get nil (the request then carries no units).
// Matching mirrors bench-quality/probe_align.py: the first case-insensitive
// occurrence of expr inside src, then every word overlapping that character
// range. A word span is used, so a match is always contiguous.
func LoadUnitsFile(path string) (func(src string, words [][2]int) [][2]int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("units file: %w", err)
	}
	var probes []unitProbe
	if err := json.Unmarshal(b, &probes); err != nil {
		return nil, fmt.Errorf("units file %s: %w", path, err)
	}
	bySrc := make(map[string][]string, len(probes))
	for _, p := range probes {
		key := strings.TrimSpace(p.Src)
		expr := strings.TrimSpace(p.Expr)
		if key == "" || expr == "" {
			continue
		}
		bySrc[key] = append(bySrc[key], expr)
	}
	return func(src string, words [][2]int) [][2]int {
		exprs := bySrc[strings.TrimSpace(src)]
		if len(exprs) == 0 || len(words) == 0 {
			return nil
		}
		runes := []rune(src)
		var out [][2]int
		for _, expr := range exprs {
			lo := indexFoldRunes(runes, []rune(expr))
			if lo < 0 {
				continue
			}
			hi := lo + len([]rune(expr))
			first, last := -1, -1
			for i, w := range words {
				if w[0] < hi && w[1] > lo { // overlap
					if first < 0 {
						first = i
					}
					last = i
				}
			}
			if first < 0 {
				continue
			}
			out = append(out, [2]int{first, last + 1})
		}
		return out
	}, nil
}

// indexFoldRunes returns the rune index of the first case-insensitive
// occurrence of needle in hay, or -1. Rune-wise (not via strings.ToLower) so
// the returned index is always a valid offset into hay — lowercasing can
// change a string's length for some scripts, which would shift the offsets the
// word spans are compared against.
func indexFoldRunes(hay, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(hay) {
		return -1
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		ok := true
		for j, r := range needle {
			if unicode.ToLower(hay[i+j]) != unicode.ToLower(r) {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}
