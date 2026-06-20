// Package align turns the model's word-level chunks into a sentence's
// translation text and highlight spans, deterministically — the model never
// emits character offsets, only source-word TEXT (the "v3" match-by-text
// contract). This is a faithful port of tbook.py's resolve_en_to_indices +
// smart_join + build_text_align.
package align

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"unicode"

	"github.com/dimando/reader/converter/internal/tbook"
)

// Chunk is one model-emitted alignment unit: a target fragment plus the source
// word(s) it translates, given as TEXT under "en" (a string or array of
// strings; absent/[] marks an inserted word with no source).
type Chunk struct {
	Tgt string  `json:"tgt"`
	En  EnField `json:"en"`
}

// EnField decodes "en" whether the model emits a string, an array of strings,
// or null/absent. Non-string array members are dropped (mirrors the Python
// isinstance(e, str) filter).
type EnField []string

func (e *EnField) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*e = nil
		return nil
	}
	switch b[0] {
	case '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*e = []string{s}
	case '[':
		var raw []json.RawMessage
		if err := json.Unmarshal(b, &raw); err != nil {
			return err
		}
		out := make([]string, 0, len(raw))
		for _, r := range raw {
			var s string
			if json.Unmarshal(r, &s) == nil {
				out = append(out, s)
			}
		}
		*e = out
	default:
		// Number/object/etc. — not a usable source word; ignore.
		*e = nil
	}
	return nil
}

// Punctuation stripped from the ends of a source-word token before matching.
const enStrip = ".,!?;:\"'()[]{}«»“”‘’…-"

// Spacing rules for joining target fragments. Em-dash (—) is deliberately
// excluded: dialogue in several languages keeps spaces around it.
var (
	noSpaceBefore = runeSet(",.!?;:…)»”’%")
	noSpaceAfter  = runeSet("(«“‘")
)

func runeSet(s string) map[rune]bool {
	m := make(map[rune]bool, len(s))
	for _, r := range s {
		m[r] = true
	}
	return m
}

// NormEn normalizes a source-word token: trim whitespace, strip surrounding
// punctuation, lowercase.
func NormEn(w string) string {
	w = strings.TrimSpace(w)
	w = strings.Trim(w, enStrip)
	return strings.ToLower(w)
}

// resolved is a chunk after its "en" text has been matched to source word
// indices.
type resolved struct {
	Tgt string
	Src []int
}

// resolveEnToIndices maps each chunk's source-word TEXT back to indices into
// wordStrs (the lowercased source words). Repeated words are consumed
// left-to-right; a word reused beyond its occurrences falls back to its last
// occurrence; unmatched tokens are dropped.
func resolveEnToIndices(chunks []Chunk, wordStrs []string) []resolved {
	occ := map[string][]int{}
	for i, w := range wordStrs {
		occ[w] = append(occ[w], i)
	}
	ptr := map[string]int{} // next occurrence to consume per word

	out := make([]resolved, 0, len(chunks))
	for _, ch := range chunks {
		idxSet := map[int]bool{}
		for _, e := range ch.En {
			key := NormEn(e)
			if key == "" {
				continue
			}
			idxs, ok := occ[key]
			if !ok {
				continue
			}
			if p := ptr[key]; p < len(idxs) {
				idxSet[idxs[p]] = true
				ptr[key] = p + 1
			} else {
				idxSet[idxs[len(idxs)-1]] = true // one-to-many: last occurrence
			}
		}
		src := make([]int, 0, len(idxSet))
		for i := range idxSet {
			src = append(src, i)
		}
		sort.Ints(src)
		out = append(out, resolved{Tgt: ch.Tgt, Src: src})
	}
	return out
}

// smartJoin concatenates target fragments with sensible spacing, building the
// translation text and the [start,end) highlight spans. Offsets are rune
// indices into the returned text. Source indices are validated against nWords.
func smartJoin(chunks []resolved, nWords int) (string, []tbook.AlignChunk) {
	var tr []rune
	align := []tbook.AlignChunk{}
	for _, ch := range chunks {
		if ch.Tgt == "" {
			continue
		}
		frag := []rune(ch.Tgt)
		if len(tr) > 0 {
			prev, first := tr[len(tr)-1], frag[0]
			if !(noSpaceBefore[first] || noSpaceAfter[prev] ||
				unicode.IsSpace(prev) || unicode.IsSpace(first)) {
				tr = append(tr, ' ')
			}
		}
		start := len(tr)
		tr = append(tr, frag...)
		end := len(tr)

		w := make([]int, 0, len(ch.Src))
		for _, i := range ch.Src {
			if i >= 0 && i < nWords {
				w = append(w, i)
			}
		}
		if len(w) > 0 {
			align = append(align, tbook.AlignChunk{T: [2]int{start, end}, W: w})
		}
	}
	return string(tr), align
}

// BuildTextAlign resolves a sentence's model chunks into its Translation
// (text + word-level highlight spans). src is the source sentence; words are
// its [start,end) rune offsets.
func BuildTextAlign(chunks []Chunk, src string, words [][2]int) tbook.Translation {
	runes := []rune(src)
	wordStrs := make([]string, len(words))
	for i, wd := range words {
		a, b := wd[0], wd[1]
		if a < 0 {
			a = 0
		}
		if b > len(runes) {
			b = len(runes)
		}
		if a > b {
			a = b
		}
		wordStrs[i] = strings.ToLower(string(runes[a:b]))
	}
	resolved := resolveEnToIndices(chunks, wordStrs)
	text, al := smartJoin(resolved, len(words))
	return tbook.Translation{Text: text, Align: al}
}
