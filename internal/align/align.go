// Package align turns the model's word-level chunks into a sentence's
// highlight spans, deterministically — the model never emits character
// offsets, only source-word TEXT (echoed as "index:text" under the v5+
// numbered contract). Since v6 the pass-1 raw translation is the CANONICAL
// text: the align pass only locates each echoed fragment inside it, so a
// sloppy echo (dropped commas, case slips) can no longer corrupt the shipped
// text — the worst it can do is fail to place a highlight.
package align

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
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

// EnField decodes "en" into a flat list of source-word TOKENS, whether the model
// emits a string, an array of strings, or null/absent. Each string is
// whitespace-split: models often emit a multi-word source unit as ONE string
// ("living room") instead of the array form (["living","room"]); without the
// split, resolveEnToIndices would look up the whole phrase as a single token,
// match nothing, and silently drop the highlight. Each source word in Words is a
// single token, so matching token-by-token is always correct.
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
		*e = strings.Fields(s)
	case '[':
		var raw []json.RawMessage
		if err := json.Unmarshal(b, &raw); err != nil {
			return err
		}
		out := []string{}
		for _, r := range raw {
			var s string
			if json.Unmarshal(r, &s) == nil {
				out = append(out, strings.Fields(s)...)
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

// numberedEnRE matches the v5 numbered-echo token form "index:text".
var numberedEnRE = regexp.MustCompile(`^(\d{1,3}):(.+)$`)

// resolveEnToIndices maps each chunk's source-word tokens back to indices into
// wordStrs (the lowercased source words). A "index:text" token (the v5
// numbered-echo contract) resolves to its index when the echoed text matches
// that word — a numbering slip falls back to the text. Plain-text tokens (and
// fallbacks) match by text: repeated words are consumed left-to-right; a word
// reused beyond its occurrences falls back to its last occurrence; unmatched
// tokens are dropped.
func resolveEnToIndices(chunks []Chunk, wordStrs []string) []resolved {
	occ := map[string][]int{}
	for i, w := range wordStrs {
		occ[w] = append(occ[w], i)
	}
	ptr := map[string]int{} // next occurrence to consume per word

	byText := func(key string, idxSet map[int]bool) {
		idxs, ok := occ[key]
		if !ok {
			return
		}
		if p := ptr[key]; p < len(idxs) {
			idxSet[idxs[p]] = true
			ptr[key] = p + 1
		} else {
			idxSet[idxs[len(idxs)-1]] = true // one-to-many: last occurrence
		}
	}

	out := make([]resolved, 0, len(chunks))
	for _, ch := range chunks {
		idxSet := map[int]bool{}
		for _, e := range ch.En {
			token := e
			if m := numberedEnRE.FindStringSubmatch(strings.TrimSpace(e)); m != nil {
				idx, _ := strconv.Atoi(m[1])
				key := NormEn(m[2])
				if idx >= 0 && idx < len(wordStrs) && key != "" && wordStrs[idx] == key {
					idxSet[idx] = true // index verified by its echoed text
					continue
				}
				token = m[2] // numbering slipped — fall back to the text
			}
			if key := NormEn(token); key != "" {
				byText(key, idxSet)
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

// foldedText is rawText reduced to lowercased letters/digits, with each folded
// rune's index in the original rune slice, so a punctuation- and
// case-insensitive fragment match maps back to exact raw offsets.
type foldedText struct {
	runes []rune
	at    []int // at[i] = rune index in raw of runes[i]
}

func foldText(raw []rune) foldedText {
	f := foldedText{}
	for i, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			f.runes = append(f.runes, unicode.ToLower(r))
			f.at = append(f.at, i)
		}
	}
	return f
}

// locate finds the folded fragment ff in f starting at folded index from,
// returning the folded [start,end) match or (-1,-1). Fragments arrive in
// target order, so the scan is a moving cursor: repeated words resolve to
// their next occurrence. Only word-boundary matches count — fragments are
// whole target words, so "и" must match the standalone word, never the "и"
// inside a longer word the cursor happens to sit before.
func (f foldedText) locate(ff []rune, from int) (int, int) {
	for i := from; i+len(ff) <= len(f.runes); i++ {
		if !f.boundaryBefore(i) || !f.boundaryAfter(i+len(ff)) {
			continue
		}
		match := true
		for j, r := range ff {
			if f.runes[i+j] != r {
				match = false
				break
			}
		}
		if match {
			return i, i + len(ff)
		}
	}
	return -1, -1
}

// boundaryBefore reports whether folded index i starts a word in the raw text:
// the previous folded rune is absent or not raw-adjacent (punctuation or space
// sits between them).
func (f foldedText) boundaryBefore(i int) bool {
	return i == 0 || f.at[i-1] < f.at[i]-1
}

// boundaryAfter reports whether folded index i (exclusive end) ends a word.
func (f foldedText) boundaryAfter(i int) bool {
	return i == len(f.runes) || f.at[i] > f.at[i-1]+1
}

// BuildTextAlign resolves a sentence's model chunks against the FINISHED raw
// translation: the returned Translation carries rawText verbatim, with each
// chunk's fragment located inside it (case/punctuation-insensitive, in order)
// to produce the [start,end) highlight spans. Highlights cover the fragment's
// letters, not its surrounding punctuation. Chunks that map no source word
// (inserted words, punctuation-only fragments) emit no align entry. A mapped
// fragment that cannot be located means the echo rewrote the translation —
// the zero Translation is returned so the caller can retry the sentence.
func BuildTextAlign(chunks []Chunk, src string, words [][2]int, rawText string) tbook.Translation {
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

	raw := []rune(rawText)
	folded := foldText(raw)
	al := []tbook.AlignChunk{}
	cursor := 0
	for _, ch := range resolved {
		w := make([]int, 0, len(ch.Src))
		for _, i := range ch.Src {
			if i >= 0 && i < len(words) {
				w = append(w, i)
			}
		}
		ff := foldText([]rune(ch.Tgt)).runes
		if len(ff) == 0 {
			continue // punctuation-only fragment: never a highlight
		}
		fs, fe := folded.locate(ff, cursor)
		if fs < 0 {
			if len(w) == 0 {
				continue // unlocatable inserted word: no claim lost
			}
			return tbook.Translation{} // echo diverged from the translation — retry
		}
		if len(w) > 0 {
			al = append(al, tbook.AlignChunk{
				T: [2]int{folded.at[fs], folded.at[fe-1] + 1}, W: w,
			})
		}
		cursor = fe
	}
	return tbook.Translation{Text: rawText, Align: al}
}
