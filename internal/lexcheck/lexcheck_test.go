package lexcheck

import (
	"testing"

	"github.com/dimando/reader/converter/internal/tbook"
)

// dict returns a small es→ru dictionary exercising polysemy (capa) and
// inflection (surface forms + prefix fallback).
func dict() *Dict {
	return &Dict{source: "es", target: "ru", m: map[string][]string{
		"capa":      {"плащ", "слой", "плаща"}, // polysemous: cloak AND layer
		"profunda":  {"глубокая", "глубокий"},
		"tercera":   {"третья", "третий"},
		"incluye":   {"включает"},
		"identidad": {"идентичность"},
		"meta":      {"цель"}, // singular only — "metas" resolves via stem variant
		"nivel":     {"уровень"},
		"eleva":     {"поднимает"},
	}}
}

// sentence builds a Sentence with one word per space-separated token and one
// align chunk per (fragment → word index) pair.
func sentence(src string, tr string, mapping [][2]any) *tbook.Sentence {
	words := [][2]int{}
	start := 0
	rs := []rune(src)
	for i := 0; i <= len(rs); i++ {
		if i == len(rs) || rs[i] == ' ' {
			if i > start {
				words = append(words, [2]int{start, i})
			}
			start = i + 1
		}
	}
	trRunes := []rune(tr)
	al := []tbook.AlignChunk{}
	pos := 0
	for _, m := range mapping {
		frag := m[0].(string)
		n := len([]rune(frag))
		// locate frag from pos
		for pos < len(trRunes) && string(trRunes[pos:min(pos+n, len(trRunes))]) != frag {
			pos++
		}
		var w []int
		switch v := m[1].(type) {
		case int:
			if v >= 0 {
				w = []int{v}
			}
		case []int:
			w = v
		}
		al = append(al, tbook.AlignChunk{T: [2]int{pos, pos + n}, W: w})
		pos += n
	}
	return &tbook.Sentence{
		Src: src, Words: words,
		Tr: map[string]tbook.Translation{"ru": {Text: tr, Align: al}},
	}
}

func TestCorrectAlignmentNotFlagged(t *testing.T) {
	// Crossing word order + polysemy: слой must be accepted for capa even
	// though плащ is the more frequent sense.
	s := sentence("la capa tercera profunda incluye tu identidad",
		"третий самый глубокий слой включает вашу идентичность",
		[][2]any{
			{"третий", 2}, {"глубокий", 3}, {"слой", 1}, {"включает", 4}, {"идентичность", 6},
		})
	r := dict().CheckSentence(s, "ru")
	if r.Flagged {
		t.Fatalf("correct alignment flagged: %+v", r)
	}
	if r.Covered < 4 || r.Supported != r.Covered {
		t.Fatalf("expected full support, got %+v", r)
	}
}

func TestFullDriftFlagged(t *testing.T) {
	// Positional pairing: each fragment mapped to the word at its own position,
	// not its meaning (the real v4 defect).
	s := sentence("la capa tercera profunda incluye tu identidad",
		"третий самый глубокий слой включает вашу идентичность",
		[][2]any{
			{"третий", 1}, {"глубокий", 2}, {"слой", 3}, {"включает", 5}, {"идентичность", 4},
		})
	r := dict().CheckSentence(s, "ru")
	if !r.Flagged {
		t.Fatalf("drifted alignment not flagged: %+v", r)
	}
}

func TestStemVariantCoversPlural(t *testing.T) {
	d := dict()
	if !d.Supports("metas", "целей") {
		t.Fatal("plural source word should resolve via stem variant + prefix match")
	}
	if !d.covered("metas") {
		t.Fatal("metas should count as covered via stem variant")
	}
}

func TestSparseEvidenceNotFlagged(t *testing.T) {
	// Only two covered pairs — below MinCoveredPairs; one miss must not flag.
	s := sentence("la capa xyzzy grault",
		"слой фубар",
		[][2]any{{"слой", 1}, {"фубар", 2}})
	r := dict().CheckSentence(s, "ru")
	if r.Flagged {
		t.Fatalf("sparse sentence flagged on thin evidence: %+v", r)
	}
}
