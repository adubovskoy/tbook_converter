package translate

import (
	"sort"
	"strings"
	"unicode"

	"github.com/dimando/reader/converter/internal/embalign"
	"github.com/dimando/reader/converter/internal/stopwords"
	"github.com/dimando/reader/converter/internal/tbook"
)

// Local, deterministic glossary mining. The model-driven sample pass cannot
// enumerate — on two books it returned 18-34 entries whatever cap it was given
// and missed characters with 147 and 1031 occurrences — so the candidate list
// is produced here, for free, from the whole book. The model then only filters
// and translates the candidates (glossrender.go).
//
// Two candidate classes:
//
//	name   — capitalised mid-sentence at least twice and (almost) never seen
//	         lowercase, plus capitalised bigrams ("Leila Begin", "Chasm City")
//	coined — a frequent lowercase word the bilingual lexicon does not know:
//	         invented terminology (neurachem, needlecast, bubblefab)
//
// Measured on Altered Carbon: 312 candidates, of which the render pass keeps
// ~190 — 175 single-word terms the sample pass never proposed, including every
// major character.

const (
	// glossMineMinFreq is how often a term must occur to be a candidate. 4 is
	// where the tail turns into noise: freq>=4 gives ~310 candidates on a
	// 15k-sentence novel, freq>=2 gives ~500 with no new frequent terms.
	glossMineMinFreq = 4
	// glossMineBigramMinFreq is lower: a full name repeats less often than its
	// parts, and it is exactly what single-token mining gets wrong ("San" for
	// "San Diego", "Bay" for "Bay City").
	glossMineBigramMinFreq = 3
	// glossCandidateMax bounds the render pass at ~8 requests per book.
	glossCandidateMax = 400
	// glossExampleMin/Max are the length window for the example sentence sent
	// with a candidate: long enough to show the sense, short enough to be free.
	glossExampleMin = 40
	glossExampleMax = 220
)

// glossCandidate is one mined term awaiting a rendering.
type glossCandidate struct {
	Term     string   `json:"term"`
	Kind     string   `json:"kind"` // "name" | "coined"
	Freq     int      `json:"freq"`
	Examples []string `json:"examples,omitempty"`
}

// glossExtraStop fills the gaps of internal/stopwords for this job. That
// package is deliberately conservative — it lists only closed-class words, since
// there a false stopword weakens the alignment gate. Here the cost is reversed:
// a function word that slips through becomes a glossary entry, so these
// (measured as false candidates on two books) are added to whatever the source
// language's list already covers.
var glossExtraStop = words(`
beyond beside besides during unless towards toward throughout amongst albeit
whilst somewhere someone something anywhere anyone anything everywhere everyone
everything nowhere nothing whoever whatever whenever wherever however therefore
otherwise instead perhaps maybe almost enough rather indeed though although
since while until once twice ain gonna gotta
mr mister mrs ms miss sir madam
`)

// stopSet is the source language's closed-class list plus glossExtraStop.
func stopSet(source string) map[string]bool {
	out := make(map[string]bool, 160)
	for w := range stopwords.Set(source) {
		out[w] = true
	}
	for w := range glossExtraStop {
		out[w] = true
	}
	return out
}

// genderPronounM / genderPronounF and the honorifics are the only patterns used
// to derive a character's gender from the source: they bind to ONE name.
// Counting he/she anywhere in the sentence is not enough — sentences hold
// several characters, and that noise put Laurens Bancroft at 0.52 female.
var (
	genderPronounM = words("he him his himself")
	genderPronounF = words("she her hers herself")
	genderHonorM   = words("mr mister sir lord father brother uncle son")
	genderHonorF   = words("ms mrs miss madam lady mother sister aunt daughter")
)

func words(s string) map[string]bool {
	m := make(map[string]bool)
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// capitalisesNouns reports source languages whose orthography capitalises every
// noun. There the never-lowercase test cannot tell a name from an ordinary noun,
// so name mining is skipped and only coined terms are mined.
func capitalisesNouns(source string) bool { return source == "de" || source == "lb" }

// sentenceWords splits one sentence into its word tokens using the offsets the
// segmenter already computed, so mining sees exactly the tokens the aligner and
// the reader do. Sentence.Words holds RUNE offsets (spec §4.2) — slicing Src by
// them as bytes shifts every token after the first non-ASCII character and mines
// fragments like "e amarant" out of "the Amarantin".
func sentenceWords(s *tbook.Sentence) []string {
	out := embalign.WordStrings(s.Src, s.Words)
	for i, w := range out {
		out[i] = strings.Trim(w, `.,;:!?"'“”„«»()[]`)
	}
	return out
}

// stripPossessive folds "Bancroft's" into "Bancroft".
func stripPossessive(w string) string {
	for _, suf := range []string{"’s", "'s", "’S", "'S"} {
		if strings.HasSuffix(w, suf) {
			return w[:len(w)-len(suf)]
		}
	}
	return w
}

func isUpperFirst(w string) bool {
	for _, r := range w {
		return unicode.IsUpper(r)
	}
	return false
}

func isLowerFirst(w string) bool {
	for _, r := range w {
		return unicode.IsLower(r)
	}
	return false
}

// pronounSelfRef reports the "I'm"/"I'd" family, capitalised only because of the
// pronoun and otherwise a top-frequency false candidate.
func pronounSelfRef(w string) bool {
	return w == "I" || strings.HasPrefix(w, "I’") || strings.HasPrefix(w, "I'")
}

// mineGlossCandidates enumerates glossary candidates over the whole book.
func mineGlossCandidates(sentences []*tbook.Sentence, source string, lex Lexicon) []glossCandidate {
	stop := stopSet(source)
	caps := map[string]int{}   // capitalised occurrences
	mid := map[string]int{}    // …of which not sentence-initial
	lower := map[string]int{}  // same token seen lowercase
	bigram := map[string]int{} // capitalised pairs
	example := map[string]string{}

	keepExample := func(term, src string) {
		if _, ok := example[term]; ok {
			return
		}
		if len(src) >= glossExampleMin && len(src) <= glossExampleMax {
			example[term] = src
		}
	}

	for _, s := range sentences {
		toks := sentenceWords(s)
		for i, raw := range toks {
			w := stripPossessive(raw)
			if len(w) < 3 || pronounSelfRef(w) {
				continue
			}
			switch {
			case isUpperFirst(w):
				caps[w]++
				if i > 0 {
					mid[w]++
				}
				keepExample(w, s.Src)
				// A capitalised pair, both parts content words: a full name or a
				// two-word place. Only mid-sentence, so a sentence-initial word
				// cannot start one.
				// A possessive first part would mine a pair that never occurs in
				// the book ("Sky’s Edge" -> "Sky Edge"), so skip it.
				if i > 0 && i+1 < len(toks) && raw == w {
					nxt := stripPossessive(toks[i+1])
					if len(nxt) >= 3 && isUpperFirst(nxt) && !pronounSelfRef(nxt) &&
						!stop[strings.ToLower(w)] && !stop[strings.ToLower(nxt)] {
						pair := w + " " + nxt
						bigram[pair]++
						keepExample(pair, s.Src)
					}
				}
			case isLowerFirst(w):
				lw := strings.ToLower(w)
				lower[lw]++
				keepExample(lw, s.Src)
			}
		}
	}

	var out []glossCandidate
	if !capitalisesNouns(source) {
		for w, c := range caps {
			// A proper noun is never (or almost never) written lowercase in the
			// same book; "World"/"Good"/"Real" are capitalised common words and
			// drift for reasons a glossary cannot fix.
			if mid[w] < 2 || c < glossMineMinFreq || stop[strings.ToLower(w)] {
				continue
			}
			if lower[strings.ToLower(w)] > maxInt(1, c/20) {
				continue
			}
			out = append(out, glossCandidate{Term: w, Kind: "name", Freq: c,
				Examples: exampleOf(example, w)})
		}
		for pair, c := range bigram {
			if c >= glossMineBigramMinFreq {
				out = append(out, glossCandidate{Term: pair, Kind: "name", Freq: c,
					Examples: exampleOf(example, pair)})
			}
		}
	}
	if lex != nil {
		for w, c := range lower {
			if c < glossMineMinFreq || len(w) < 4 || stop[w] || strings.ContainsAny(w, "’'") {
				continue
			}
			if lexKnows(lex, w) {
				continue
			}
			out = append(out, glossCandidate{Term: w, Kind: "coined", Freq: c,
				Examples: exampleOf(example, w)})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Freq != out[j].Freq {
			return out[i].Freq > out[j].Freq
		}
		return out[i].Term < out[j].Term
	})
	if len(out) > glossCandidateMax {
		out = out[:glossCandidateMax]
	}
	return out
}

func exampleOf(m map[string]string, term string) []string {
	if s, ok := m[term]; ok {
		return []string{s}
	}
	return nil
}

// lexKnows reports whether the bilingual lexicon knows the word, trying light
// de-inflections first. The lexicon is a translation dictionary, not a wordlist,
// so it lists "shrug" but not "shrugged" — without this, 60% of the coined-term
// candidates are ordinary inflected verbs.
func lexKnows(lex Lexicon, w string) bool {
	for _, form := range deinflect(w) {
		if lex.Covered(form) {
			return true
		}
	}
	if i := strings.IndexByte(w, '-'); i > 0 { // re-sleeved, trode-cable
		return lexKnows(lex, w[:i]) && lexKnows(lex, w[i+1:])
	}
	return false
}

// deinflect returns w plus light English reductions of it.
func deinflect(w string) []string {
	out := []string{w}
	add := func(s string) {
		if len(s) >= 3 {
			out = append(out, s)
		}
	}
	for _, suf := range []string{"ing", "ed", "es", "s", "ly", "er", "est"} {
		if !strings.HasSuffix(w, suf) || len(w)-len(suf) < 3 {
			continue
		}
		base := w[:len(w)-len(suf)]
		add(base)
		add(base + "e") // moving -> move, forced -> force
		if n := len(base); n > 3 && base[n-1] == base[n-2] {
			add(base[:n-1]) // grinned -> grin
		}
	}
	return out
}

// genderEvidence is what the source itself says about one name's gender.
type genderEvidence struct {
	Gender     string  // "m" | "f"
	Evidence   int     // weighted observations
	Confidence float64 // share of them pointing that way
}

// mineGender derives each name's gender from the source with two patterns that
// bind to a single name: an honorific directly before it, and the first gendered
// pronoun after it with no other name in between. Validated by hand on two
// books: at confidence >= 0.8 it made no errors, and its disagreement with the
// model is the signal that a surname belongs to characters of both genders
// (Bancroft: Laurens and Miriam; Elliott: Victor, Elizabeth and Irene).
// known is the set of mined names, so a name at the START of a sentence counts
// too. The candidate miner has to ignore sentence-initial capitals (they cannot
// be told from a capitalised common word), but once a term is established as a
// name, "Ortega lifted her hand." is evidence like any other — and most
// narration puts the name first.
func mineGender(sentences []*tbook.Sentence, source string, known map[string]bool) map[string]genderEvidence {
	type tally struct{ m, f int }
	ev := map[string]*tally{}
	bump := func(name, g string, weight int) {
		t := ev[name]
		if t == nil {
			t = &tally{}
			ev[name] = t
		}
		if g == "m" {
			t.m += weight
		} else {
			t.f += weight
		}
	}
	if capitalisesNouns(source) {
		return nil // the name test below cannot work there
	}
	stop := stopSet(source)

	for _, s := range sentences {
		toks := sentenceWords(s)
		names := make([]string, len(toks))
		low := make([]string, len(toks))
		for i, raw := range toks {
			w := stripPossessive(raw)
			low[i] = strings.ToLower(w)
			if len(w) >= 3 && isUpperFirst(w) && !pronounSelfRef(w) && !stop[low[i]] &&
				(i > 0 || known[w]) {
				names[i] = w
			}
		}
		for i, name := range names {
			if name == "" {
				continue
			}
			if i > 0 {
				if genderHonorM[low[i-1]] {
					bump(name, "m", 3)
				} else if genderHonorF[low[i-1]] {
					bump(name, "f", 3)
				}
			}
			for j := i + 1; j < len(low); j++ {
				if names[j] != "" && names[j] != name {
					break // another character intervenes
				}
				if genderPronounM[low[j]] {
					bump(name, "m", 1)
					break
				}
				if genderPronounF[low[j]] {
					bump(name, "f", 1)
					break
				}
			}
		}
	}

	out := make(map[string]genderEvidence, len(ev))
	for name, t := range ev {
		total := t.m + t.f
		if total == 0 {
			continue
		}
		g, hits := "f", t.f
		if t.m > t.f {
			g, hits = "m", t.m
		}
		out[name] = genderEvidence{Gender: g, Evidence: total,
			Confidence: float64(hits) / float64(total)}
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
