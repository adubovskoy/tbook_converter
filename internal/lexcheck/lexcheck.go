// Package lexcheck statically detects alignment drift using bilingual
// dictionaries — no API calls. For every align chunk it asks: is the target
// fragment a dictionary-plausible translation of the source word(s) it claims
// to translate? Isolated misses prove nothing (dictionaries are register-biased
// and words are polysemous — a literary synonym is often absent), so a sentence
// is flagged only on AGGREGATE evidence:
//
//   - low-support: enough covered pairs, few of them plausible; or
//   - shift-pattern: several pairs implausible for their own source word but
//     plausible for its NEIGHBOR — the direct signature of an off-by-one
//     positional cascade, and robust to polysemy (a random sense gap does not
//     systematically match the neighboring word).
//
// Morphology is handled two ways: OPUS dictionaries store surface forms (many
// inflections are present verbatim), and matching falls back to conservative
// prefix comparison plus light source-side suffix stripping.
package lexcheck

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/dimando/reader/converter/internal/tbook"
)

// Tunables — defaults chosen by sweeping against LLM-judge verdicts on a real
// book (see cmd/lexeval): they favor precision over recall, since flagged
// sentences get re-aligned/escalated and false positives waste money.
const (
	MinCoveredPairs = 4    // fewer covered pairs → not enough evidence, skip sentence
	LowSupportRate  = 0.34 // flag when ≤ this fraction of covered pairs is plausible
	MinShiftHits    = 2    // flag when ≥ this many pairs fit a neighbor but not their own word
	prefixMinLen    = 4    // prefix matching applies to words at least this long
	prefixMinCommon = 3    // …sharing at least this long a common prefix
	prefixSlack     = 3    // …and differing only in the last ≤ this many runes
)

// Dict is a one-direction bilingual dictionary: folded source word → plausible
// folded target words (multiple senses included, most frequent first).
type Dict struct {
	m      map[string][]string
	source string
	target string
}

// Path returns the conventional lexicon file path for a language pair.
func Path(dir, source, target string) string {
	return filepath.Join(dir, source+"-"+target+".tsv.gz")
}

// Load reads a compact lexicon ("src\ttgt1|tgt2|…" gzip lines) for the pair.
// Returns (nil, nil) when the file does not exist — lexcheck is optional.
func Load(dir, source, target string) (*Dict, error) {
	path := Path(dir, source, target)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	defer zr.Close()

	d := &Dict{m: make(map[string][]string, 1<<17), source: source, target: target}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		i := strings.IndexByte(line, '\t')
		if i <= 0 {
			continue
		}
		d.m[line[:i]] = strings.Split(line[i+1:], "|")
	}
	return d, sc.Err()
}

// Entries reports the number of source headwords.
func (d *Dict) Entries() int { return len(d.m) }

// fold lowercases and keeps letters only.
func fold(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// stemVariants returns light source-side reductions of a folded word (plural /
// frequent inflection endings) so "metas" also consults "meta". Deliberately
// tiny and language-generic: dictionary surface forms carry most inflections.
func stemVariants(w string) []string {
	var out []string
	rs := []rune(w)
	n := len(rs)
	if n >= 4 && rs[n-1] == 's' {
		out = append(out, string(rs[:n-1]))
		if n >= 5 && rs[n-2] == 'e' {
			out = append(out, string(rs[:n-2]))
		}
	}
	return out
}

// prefixPlausible reports whether two folded words are inflection-close: equal,
// or long words sharing a long common prefix with only a short tail differing.
func prefixPlausible(a, b string) bool {
	if a == b {
		return a != ""
	}
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la < prefixMinLen || lb < prefixMinLen {
		return false
	}
	minLen := min(la, lb)
	common := 0
	for common < minLen && ra[common] == rb[common] {
		common++
	}
	// Short words may only shed a short tail (цель/целей); long words a longer
	// one (совершенствование/совершенствования). Erring permissive is safe: a
	// looser match means FEWER drift flags, never more.
	slack := prefixSlack
	if minLen < 6 {
		slack = minLen - prefixMinCommon
	}
	return common >= prefixMinCommon && common >= minLen-slack
}

// covered reports whether the dictionary knows the source word at all (directly
// or via a stem variant). Uncovered words carry no evidence either way.
func (d *Dict) covered(srcWord string) bool {
	w := fold(srcWord)
	if _, ok := d.m[w]; ok {
		return true
	}
	for _, v := range stemVariants(w) {
		if _, ok := d.m[v]; ok {
			return true
		}
	}
	return false
}

// Covered is the exported form of covered, for diagnostic tools.
func (d *Dict) Covered(srcWord string) bool { return d.covered(srcWord) }

// Supports reports whether tgt is a dictionary-plausible rendering of srcWord.
func (d *Dict) Supports(srcWord, tgt string) bool {
	w := fold(srcWord)
	tf := fold(tgt)
	if tf == "" {
		return false
	}
	keys := append([]string{w}, stemVariants(w)...)
	for _, k := range keys {
		for _, cand := range d.m[k] {
			if prefixPlausible(cand, tf) {
				return true
			}
		}
	}
	return false
}

// Result is the static verdict for one sentence.
type Result struct {
	Covered   int    // pairs whose source word the dictionary knows
	Supported int    // of those, pairs that are dictionary-plausible
	ShiftHits int    // pairs implausible for their word but plausible for a neighbor
	Flagged   bool
	Reason    string // "low-support" | "shift-pattern" | ""
}

// SupportRate is Supported/Covered (1 when nothing is covered).
func (r Result) SupportRate() float64 {
	if r.Covered == 0 {
		return 1
	}
	return float64(r.Supported) / float64(r.Covered)
}

// CheckSentence statically scores one sentence's alignment for target lang.
func (d *Dict) CheckSentence(s *tbook.Sentence, target string) Result {
	var res Result
	tr, ok := s.Tr[target]
	if !ok || tr.Text == "" || len(tr.Align) == 0 {
		return res
	}
	srcRunes := []rune(s.Src)
	trRunes := []rune(tr.Text)
	word := func(i int) string {
		if i < 0 || i >= len(s.Words) {
			return ""
		}
		a, b := s.Words[i][0], s.Words[i][1]
		if a < 0 || b > len(srcRunes) || a >= b {
			return ""
		}
		return string(srcRunes[a:b])
	}

	for _, c := range tr.Align {
		if len(c.W) == 0 {
			continue // inserted word: no claim to verify
		}
		a, b := c.T[0], c.T[1]
		if a < 0 || b > len(trRunes) || a >= b {
			continue
		}
		frag := string(trRunes[a:b])
		if len([]rune(fold(frag))) < 3 {
			continue // function words / punctuation: too noisy to score
		}

		anyCovered, supported := false, false
		for _, wi := range c.W {
			w := word(wi)
			if len([]rune(fold(w))) < 3 || !d.covered(w) {
				continue
			}
			anyCovered = true
			if d.Supports(w, frag) {
				supported = true
				break
			}
		}
		if !anyCovered {
			continue
		}
		res.Covered++
		if supported {
			res.Supported++
			continue
		}
		// Not plausible for its own word — does it fit a neighboring word?
		// That is the off-by-one signature.
		lo, hi := c.W[0], c.W[len(c.W)-1]
		for _, ni := range []int{lo - 1, hi + 1} {
			w := word(ni)
			if w != "" && d.covered(w) && d.Supports(w, frag) {
				res.ShiftHits++
				break
			}
		}
	}

	if res.Covered >= MinCoveredPairs && res.SupportRate() <= LowSupportRate {
		res.Flagged, res.Reason = true, "low-support"
	} else if res.ShiftHits >= MinShiftHits {
		res.Flagged, res.Reason = true, "shift-pattern"
	}
	return res
}
