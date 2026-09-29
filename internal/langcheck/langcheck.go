// Package langcheck flags translations that are not in the target language.
//
// Nothing else in the pipeline sees this class of defect. Validation checks
// structure and offsets; the alignment q-score measures how well the words line
// up with whatever text arrived, so a fluent answer in the wrong language
// scores as well as a right one; and lexcheck needs dictionary lookups, so a
// sentence it cannot look up at all yields no evidence either way. Two measured
// cases, both of which shipped with "OK" and a 0.97 mean q
// (bench-quality/reports/four-model-bench-2026-08.md §2.3, §7):
//
//   - a whole batch answered in another language — DeepSeek-V4-Flash returned 16
//     consecutive en→ru sentences in Chinese;
//   - an enforced glossary in the wrong language — a multi-target run built one
//     en→ru glossary and enforced it on en→de … en→tr, which put Russian names,
//     and often whole Russian sentences, into 50–73% of six Latin targets.
//
// Three signals, all offline and free:
//
//	foreign-script    the text contains words in a script neither the source nor
//	                  the target language writes in (and which are not quoted
//	                  from the source, so a macaronic original stays clean);
//	untranslated      the text is the source sentence, or almost all of its words
//	                  are source words — the pivot copied through;
//	duplicate-target  two targets carry the identical sentence, which for any
//	                  pair of distinct languages means one answer was served for
//	                  both (same-script leaks like ru→uk or es→pt, which the
//	                  script signal cannot see).
package langcheck

import (
	"sort"
	"strings"
	"unicode"

	"github.com/dimando/reader/converter/internal/tbook"
)

// Kind names a defect. The values are stable: they go into the report file.
const (
	KindForeignScript   = "foreign-script"
	KindUntranslated    = "untranslated"
	KindDuplicateTarget = "duplicate-target"
)

// Gating reports whether a kind is strong enough to act on — to throw a
// translation away, or to badge the cell as being in the wrong language.
//
// Only two of the three are. A whole leaked sentence is caught 78-88% of the
// time by foreign-script on four independent targets, at a 1.0-4.0%
// false-positive rate on clean text, and a duplicate target is one answer
// served for two languages by construction. `untranslated` has no measured
// precision and a known failure shape: a sentence whose content is proper
// nouns the target legitimately keeps reads as the source copied through.
// Measured, verbatim, on Sherlock ch.1 en→de:
//
//	src  “Is Briony Lodge, Serpentine Avenue, St. John’s Wood.”
//	de   „Ist Briony Lodge, Serpentine Avenue, St. John’s Wood.“
//
// That is correct German. Acting on it deletes a good translation, which is a
// worse outcome than shipping a suspect one, so the kind stays a report.
func Gating(kind string) bool {
	return kind == KindForeignScript || kind == KindDuplicateTarget
}

// Flag is one suspect (sentence, target) pair.
type Flag struct {
	Target string   `json:"target"`
	Kind   string   `json:"kind"`
	Src    string   `json:"src"`
	Text   string   `json:"text"`
	Words  []string `json:"words,omitempty"` // the offending words (foreign-script)
	Share  float64  `json:"share,omitempty"` // share of letters that are foreign
	Other  string   `json:"other,omitempty"` // the other target (duplicate-target)
	// Lang is the language of the foreign script when the script identifies
	// one (Han → zh); empty for scripts many languages share (Latin,
	// Cyrillic), where naming one would be a guess.
	Lang string `json:"lang,omitempty"`
}

// scripts a language is written in. A language absent from this table disables
// the script signal for it rather than guessing — a wrong guess would flag a
// whole correct book.
var langScripts = map[string][]*unicode.RangeTable{
	// Latin
	"en": {unicode.Latin}, "de": {unicode.Latin}, "es": {unicode.Latin},
	"fr": {unicode.Latin}, "it": {unicode.Latin}, "pt": {unicode.Latin},
	"nl": {unicode.Latin}, "pl": {unicode.Latin}, "cs": {unicode.Latin},
	"sk": {unicode.Latin}, "sv": {unicode.Latin}, "da": {unicode.Latin},
	"no": {unicode.Latin}, "nb": {unicode.Latin}, "fi": {unicode.Latin},
	"hu": {unicode.Latin}, "ro": {unicode.Latin}, "hr": {unicode.Latin},
	"sl": {unicode.Latin}, "et": {unicode.Latin}, "lv": {unicode.Latin},
	"lt": {unicode.Latin}, "tr": {unicode.Latin}, "az": {unicode.Latin},
	"vi": {unicode.Latin}, "id": {unicode.Latin}, "ms": {unicode.Latin},
	"sw": {unicode.Latin}, "af": {unicode.Latin}, "ca": {unicode.Latin},
	"eu": {unicode.Latin}, "gl": {unicode.Latin}, "tl": {unicode.Latin},
	// Cyrillic
	"ru": {unicode.Cyrillic}, "uk": {unicode.Cyrillic}, "be": {unicode.Cyrillic},
	"bg": {unicode.Cyrillic}, "mk": {unicode.Cyrillic}, "kk": {unicode.Cyrillic},
	"ky": {unicode.Cyrillic}, "tg": {unicode.Cyrillic}, "mn": {unicode.Cyrillic},
	"sr": {unicode.Cyrillic, unicode.Latin}, // both alphabets are standard
	// Other scripts
	"el": {unicode.Greek},
	"he": {unicode.Hebrew}, "yi": {unicode.Hebrew},
	"ar": {unicode.Arabic}, "fa": {unicode.Arabic}, "ur": {unicode.Arabic},
	"ps": {unicode.Arabic}, "ku": {unicode.Arabic, unicode.Latin},
	"zh": {unicode.Han}, "yue": {unicode.Han},
	"ja": {unicode.Han, unicode.Hiragana, unicode.Katakana},
	"ko": {unicode.Hangul, unicode.Han},
	"hi": {unicode.Devanagari}, "mr": {unicode.Devanagari},
	"ne": {unicode.Devanagari}, "sa": {unicode.Devanagari},
	"bn": {unicode.Bengali}, "ta": {unicode.Tamil}, "te": {unicode.Telugu},
	"th": {unicode.Thai}, "hy": {unicode.Armenian}, "ka": {unicode.Georgian},
}

// Known reports whether the script signal can run for this language pair.
func Known(source, target string) bool {
	_, okS := langScripts[strings.ToLower(source)]
	_, okT := langScripts[strings.ToLower(target)]
	return okS && okT
}

// Options tunes the signals. The zero value uses the defaults below.
type Options struct {
	MinWords        int     // shortest sentence worth judging (default 3)
	UntranslatedMin float64 // share of target words that are source words (default 0.85)
	// UntranslatedWords is the shortest sentence the untranslated signal will
	// judge (default 6). It is deliberately higher than MinWords: a short
	// sentence legitimately survives translation unchanged — a name, a date, an
	// interjection, a cognate phrase ("Kansas, 1899", "Hallo, Toto!").
	UntranslatedWords int
}

const (
	defaultMinWords          = 3
	defaultUntranslatedMin   = 0.85
	defaultUntranslatedWords = 6
)

func (o Options) minWords() int {
	if o.MinWords > 0 {
		return o.MinWords
	}
	return defaultMinWords
}

func (o Options) untranslatedWords() int {
	if o.UntranslatedWords > 0 {
		return o.UntranslatedWords
	}
	return max(defaultUntranslatedWords, o.minWords())
}

func (o Options) untranslatedMin() float64 {
	if o.UntranslatedMin > 0 {
		return o.UntranslatedMin
	}
	return defaultUntranslatedMin
}

// Check runs every signal over the translated sentences and returns the flags,
// ordered by target then by book order. Targets whose language is unknown to
// the script table still get the untranslated and duplicate-target signals.
func Check(sentences []*tbook.Sentence, source string, targets []string, opts Options) []Flag {
	var flags []Flag
	for _, target := range targets {
		for _, s := range sentences {
			tr, ok := s.Tr[target]
			if !ok || strings.TrimSpace(tr.Text) == "" {
				continue
			}
			if f, ok := checkOne(s.Src, tr.Text, source, target, opts); ok {
				flags = append(flags, f)
			}
		}
	}
	return append(flags, duplicates(sentences, targets, opts)...)
}

// checkOne applies the per-sentence signals; the first that fires wins, so one
// sentence never produces two flags for the same target.
func checkOne(src, text, source, target string, opts Options) (Flag, bool) {
	srcWords := words(src)
	if len(srcWords) < opts.minWords() {
		return Flag{}, false
	}
	if fw, share := foreignWords(src, text, source, target); len(fw) > 0 {
		return Flag{Target: target, Kind: KindForeignScript, Src: src, Text: text,
			Words: fw, Share: round3(share), Lang: dominantLang(fw)}, true
	}
	if len(srcWords) >= opts.untranslatedWords() && untranslated(srcWords, words(text), opts) {
		return Flag{Target: target, Kind: KindUntranslated, Src: src, Text: text}, true
	}
	return Flag{}, false
}

// foreignWords returns the words of text written in a script neither language
// uses, plus the share of the text's letters those words hold. Words that occur
// in the source sentence are skipped: an original that quotes another script
// (a Russian line in an English novel) is legitimately carried over.
func foreignWords(src, text, source, target string) ([]string, float64) {
	allowed := append(langScripts[strings.ToLower(source)], langScripts[strings.ToLower(target)]...)
	if len(langScripts[strings.ToLower(source)]) == 0 || len(langScripts[strings.ToLower(target)]) == 0 {
		return nil, 0 // unknown language: no script opinion
	}
	quoted := map[string]bool{}
	for _, w := range words(src) {
		quoted[w] = true
	}
	var foreign []string
	foreignLetters, totalLetters := 0, 0
	for _, w := range words(text) {
		letters := countLetters(w)
		totalLetters += letters
		if inScripts(w, allowed) || quoted[w] {
			continue
		}
		foreignLetters += letters
		foreign = append(foreign, w)
	}
	if totalLetters == 0 {
		return nil, 0
	}
	return dedupe(foreign), float64(foreignLetters) / float64(totalLetters)
}

// WrongScriptShare is how much of a sentence must sit outside the target's own
// scripts before the batch guard calls it wrong-language. A majority, so a
// rendering that keeps a Latin brand or a formula inside Russian stays clean
// while a whole sentence in another language does not.
const WrongScriptShare = 0.5

// WrongScript reports whether text is written in a script the TARGET language
// does not use — the defect nothing else in the pipeline sees: a well-formed,
// fully covered, q = 1.000 answer in the wrong language (a whole batch came
// back in Chinese from an en→ru run; Cyrillic filled 73% of a German overlay).
//
// It differs from the foreign-script report signal in two ways, both because it
// runs on a reply the pipeline can still throw away: the source's script is NOT
// allowed (Latin inside Russian is a leak, not a quotation) and a majority, not
// a single word, is required. Words that occur in the source sentence are still
// exempt — an original that quotes another script is legitimately carried over.
//
// The second return names the language of the offending script when the script
// identifies one; it is "" for Latin and Cyrillic, which many languages share.
// An unknown target disables the check rather than guessing.
func WrongScript(src, text, source, target string) (bool, string) {
	tgt := langScripts[strings.ToLower(target)]
	if len(tgt) == 0 {
		return false, ""
	}
	quoted := map[string]bool{}
	for _, w := range words(src) {
		quoted[w] = true
	}
	var foreign []string
	foreignLetters, totalLetters := 0, 0
	for _, w := range words(text) {
		letters := countLetters(w)
		totalLetters += letters
		if letters == 0 || quoted[w] || inScripts(w, tgt) {
			continue
		}
		foreignLetters += letters
		foreign = append(foreign, w)
	}
	if totalLetters == 0 || float64(foreignLetters)/float64(totalLetters) <= WrongScriptShare {
		return false, ""
	}
	return true, dominantLang(foreign)
}

// OutsideScript reports whether the majority of text's letters sit outside the
// scripts the target language writes — the glossary-entry form of the same
// question, with no source sentence to exempt anything. It is what rejects a
// Cyrillic rendering from a German glossary (`Айрин Адлер`), which the
// "contains one target-script letter" rule cannot see for a Latin target.
func OutsideScript(text, target string) bool {
	tgt := langScripts[strings.ToLower(target)]
	if len(tgt) == 0 {
		return false
	}
	foreign, total := 0, 0
	for _, w := range words(text) {
		letters := countLetters(w)
		total += letters
		if letters > 0 && !inScripts(w, tgt) {
			foreign += letters
		}
	}
	return total > 0 && float64(foreign)/float64(total) > WrongScriptShare
}

// namedScripts is the script table the detector reports from, most specific
// first. lang is the language a script identifies on its own; "" where several
// languages share it and naming one would be a guess (spec §6.9).
var namedScripts = []struct {
	table *unicode.RangeTable
	lang  string
}{
	{unicode.Hiragana, "ja"}, {unicode.Katakana, "ja"}, {unicode.Hangul, "ko"},
	{unicode.Han, "zh"}, {unicode.Cyrillic, ""}, {unicode.Latin, ""},
	{unicode.Greek, "el"}, {unicode.Hebrew, "he"}, {unicode.Arabic, ""},
	{unicode.Devanagari, ""}, {unicode.Bengali, "bn"}, {unicode.Tamil, "ta"},
	{unicode.Telugu, "te"}, {unicode.Thai, "th"}, {unicode.Armenian, "hy"},
	{unicode.Georgian, "ka"},
}

// dominantLang names the language of the script most of these words are in.
// Japanese wins over Chinese whenever a kana appears: kanji alone reads as Han.
func dominantLang(ws []string) string {
	counts := make([]int, len(namedScripts))
	for _, w := range ws {
		for _, r := range w {
			if !unicode.IsLetter(r) {
				continue
			}
			for i, s := range namedScripts {
				if unicode.Is(s.table, r) {
					counts[i]++
					break
				}
			}
		}
	}
	best := -1
	for i, n := range counts {
		if n > 0 && (best < 0 || n > counts[best]) {
			best = i
		}
	}
	if best < 0 {
		return ""
	}
	if namedScripts[best].lang == "zh" && (counts[0] > 0 || counts[1] > 0) {
		return "ja"
	}
	return namedScripts[best].lang
}

func countLetters(w string) int {
	n := 0
	for _, r := range w {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

// inScripts reports whether every letter of w belongs to one of the tables.
func inScripts(w string, tables []*unicode.RangeTable) bool {
	for _, r := range w {
		if !unicode.IsLetter(r) {
			continue
		}
		if !unicode.In(r, tables...) {
			return false
		}
	}
	return true
}

// untranslated reports whether the target text is the source text copied
// through: identical, or almost every one of its words is a source word.
func untranslated(srcWords, trWords []string, opts Options) bool {
	if len(trWords) < opts.untranslatedWords() {
		return false
	}
	src := map[string]bool{}
	for _, w := range srcWords {
		src[w] = true
	}
	hit := 0
	for _, w := range trWords {
		if src[w] {
			hit++
		}
	}
	return float64(hit)/float64(len(trWords)) >= opts.untranslatedMin()
}

// duplicates flags sentences whose translation is byte-identical for two
// different targets — one answer served for both languages. Short sentences are
// skipped: a name, a number or an interjection is legitimately the same in two
// languages.
func duplicates(sentences []*tbook.Sentence, targets []string, opts Options) []Flag {
	if len(targets) < 2 {
		return nil
	}
	// The run's own order decides which target keeps the text and which ones are
	// flagged as copies of it: the first target is the one whose language the
	// answer was actually generated for (it is the one whose glossary and prompt
	// produced it), so alphabetical order would blame the victim.
	rank := make(map[string]int, len(targets))
	for i, t := range targets {
		rank[t] = i
	}
	minWords := max(opts.minWords(), 5)
	var flags []Flag
	for _, s := range sentences {
		byText := map[string][]string{}
		for _, t := range targets {
			tr, ok := s.Tr[t]
			if !ok {
				continue
			}
			txt := strings.TrimSpace(tr.Text)
			if txt == "" || len(words(txt)) < minWords {
				continue
			}
			byText[txt] = append(byText[txt], t)
		}
		for txt, ts := range byText {
			if len(ts) < 2 {
				continue
			}
			sort.Slice(ts, func(i, j int) bool { return rank[ts[i]] < rank[ts[j]] })
			// One flag per extra target, naming the one it duplicates.
			for _, t := range ts[1:] {
				flags = append(flags, Flag{Target: t, Kind: KindDuplicateTarget,
					Src: s.Src, Text: txt, Other: ts[0]})
			}
		}
	}
	return flags
}

// Summary counts flags per target and kind, for the one-line run report.
type Summary struct {
	Target string
	Kinds  map[string]int
	Total  int
}

// Summarize groups flags by target, in the order the targets were given.
func Summarize(flags []Flag, targets []string) []Summary {
	byTarget := map[string]*Summary{}
	for _, f := range flags {
		s := byTarget[f.Target]
		if s == nil {
			s = &Summary{Target: f.Target, Kinds: map[string]int{}}
			byTarget[f.Target] = s
		}
		s.Kinds[f.Kind]++
		s.Total++
	}
	var out []Summary
	for _, t := range targets {
		if s := byTarget[t]; s != nil {
			out = append(out, *s)
		}
	}
	return out
}

// SrcsByTarget groups the flagged source sentences per target — the shape
// --invalidate consumes, so a flagged book is repaired by re-translating
// exactly those sentences in exactly those languages.
func SrcsByTarget(flags []Flag) map[string][]string {
	out := map[string][]string{}
	seen := map[string]bool{}
	for _, f := range flags {
		k := f.Target + "\x00" + f.Src
		if seen[k] {
			continue
		}
		seen[k] = true
		out[f.Target] = append(out[f.Target], f.Src)
	}
	return out
}

// words splits on non-letters/digits and lowercases, so comparisons ignore case
// and punctuation.
func words(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, strings.ToLower(f))
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func round3(f float64) float64 { return float64(int(f*1000+0.5)) / 1000 }
