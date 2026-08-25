package translate

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
)

// fakeLexicon knows exactly the words it is given, so a test can decide what
// counts as invented terminology.
type fakeLexicon map[string]bool

func (f fakeLexicon) Covered(w string) bool { return f[strings.ToLower(w)] }

// mineFixture segments real sentences the way the converter does, so mining is
// exercised on the same tokens production sees.
func mineFixture(t *testing.T, paragraphs ...string) []*tbook.Sentence {
	t.Helper()
	paras := make([]segment.ParsedParagraph, 0, len(paragraphs))
	for _, p := range paragraphs {
		paras = append(paras, segment.ParsedParagraph{Text: p})
	}
	_, sents := segment.BuildSentenceObjects([]segment.ParsedChapter{{Paragraphs: paras}}, "en")
	if len(sents) == 0 {
		t.Fatal("fixture produced no sentences")
	}
	return sents
}

func candidateSet(cands []glossCandidate) map[string]glossCandidate {
	m := make(map[string]glossCandidate, len(cands))
	for _, c := range cands {
		m[c.Term] = c
	}
	return m
}

func TestMineGlossCandidatesFindsNamesAndCoinedTerms(t *testing.T) {
	// Ortega recurs mid-sentence and never lowercase -> a name.
	// "world" recurs but is also written lowercase -> a capitalised common word.
	// "neurachem" is frequent and unknown to the lexicon -> coined.
	// "walked" is frequent but the lexicon knows "walk" -> ordinary.
	// "I'm" is capitalised only because of the pronoun.
	var b strings.Builder
	for i := 0; i < 5; i++ {
		b.WriteString("The street was quiet when Ortega walked in. ")
		b.WriteString("I'm told the neurachem was still online. ")
		b.WriteString("The World spun on, and the world spun back. ")
	}
	sents := mineFixture(t, b.String())
	lex := fakeLexicon{"walk": true, "street": true, "quiet": true, "world": true,
		"online": true, "spun": true, "told": true, "still": true}

	got := candidateSet(mineGlossCandidates(sents, "en", lex))
	if c, ok := got["Ortega"]; !ok || c.Kind != "name" || c.Freq < 5 {
		t.Errorf("Ortega should be mined as a name with freq>=5, got %+v (ok=%v)", c, ok)
	}
	if c, ok := got["neurachem"]; !ok || c.Kind != "coined" {
		t.Errorf("neurachem should be mined as coined, got %+v (ok=%v)", c, ok)
	}
	for _, bad := range []string{"World", "walked", "I'm", "I’m", "The"} {
		if _, ok := got[bad]; ok {
			t.Errorf("%q must not be a candidate", bad)
		}
	}
}

// Sentence.Words are rune offsets: slicing them as bytes mines mojibake
// fragments out of any sentence containing a non-ASCII character.
func TestMineGlossCandidatesHandlesNonASCII(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 5; i++ {
		b.WriteString("Sky’s Edge might have been Sylveste’s home — Sylveste said so. ")
	}
	for _, c := range mineGlossCandidates(mineFixture(t, b.String()), "en", nil) {
		if !utf8.ValidString(c.Term) || strings.ContainsRune(c.Term, utf8.RuneError) {
			t.Errorf("mined a broken token: %q", c.Term)
		}
	}
	got := candidateSet(mineGlossCandidates(mineFixture(t, b.String()), "en", nil))
	if _, ok := got["Sylveste"]; !ok {
		t.Errorf("possessive should fold into the bare name; got %v", keysOf(got))
	}
	// "Sky’s Edge" must not be mined as the pair "Sky Edge", which never occurs.
	if _, ok := got["Sky Edge"]; ok {
		t.Error("a bigram must not be built across a stripped possessive")
	}
}

func TestMineGlossCandidatesFindsBigrams(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 4; i++ {
		b.WriteString("They met Leila Begin outside. ")
	}
	got := candidateSet(mineGlossCandidates(mineFixture(t, b.String()), "en", nil))
	if _, ok := got["Leila Begin"]; !ok {
		t.Errorf("the full name should be mined as a bigram; got %v", keysOf(got))
	}
}

func TestMineGlossCandidatesSkipsNamesForNounCapitalisingSource(t *testing.T) {
	sents := mineFixture(t, strings.Repeat("Der Wagen stand vor dem Haus. ", 5))
	for _, c := range mineGlossCandidates(sents, "de", nil) {
		if c.Kind == "name" {
			t.Errorf("German capitalises every noun; %q must not be mined as a name", c.Term)
		}
	}
}

func TestMineGenderUsesBindingPatternsOnly(t *testing.T) {
	sents := mineFixture(t,
		"Ortega lifted her hand. "+ // pronoun after the name
			"Mr Bancroft closed the door. "+ // honorific before the name
			"Kadmin watched Ortega and told her nothing. ", // Ortega, not Kadmin
	)
	g := mineGender(sents, "en", map[string]bool{"Ortega": true, "Bancroft": true, "Kadmin": true})
	if got := g["Ortega"]; got.Gender != "f" {
		t.Errorf("Ortega: want f, got %+v", got)
	}
	if got := g["Bancroft"]; got.Gender != "m" {
		t.Errorf("Bancroft: want m, got %+v", got)
	}
	// "her" in the third sentence follows Ortega, so it must not reach Kadmin.
	if got, ok := g["Kadmin"]; ok && got.Gender == "f" {
		t.Errorf("Kadmin must not inherit Ortega's pronoun, got %+v", got)
	}
}

func TestMineGenderSplitSurnameIsNotConfident(t *testing.T) {
	// The same surname for a man and a woman: the evidence must come out split,
	// which is what lets acceptGender refuse to tag it.
	var b strings.Builder
	for i := 0; i < 8; i++ {
		b.WriteString("Bancroft raised his glass. Bancroft turned her head away. ")
	}
	g := mineGender(mineFixture(t, b.String()), "en", map[string]bool{"Bancroft": true})
	ev := g["Bancroft"]
	if ev.Evidence < genderMinEvidence {
		t.Fatalf("want at least %d observations, got %+v", genderMinEvidence, ev)
	}
	if ev.Confidence > 0.7 {
		t.Errorf("a surname shared by both genders must not look confident: %+v", ev)
	}
}

func TestLexKnowsDeinflects(t *testing.T) {
	lex := fakeLexicon{"walk": true, "shrug": true, "force": true, "sleeve": true}
	for _, w := range []string{"walk", "walked", "walking", "walks", "shrugged", "forced"} {
		if !lexKnows(lex, w) {
			t.Errorf("lexKnows(%q) = false, want true", w)
		}
	}
	for _, w := range []string{"neurachem", "needlecast", "bubblefab"} {
		if lexKnows(lex, w) {
			t.Errorf("lexKnows(%q) = true, want false", w)
		}
	}
	// A hyphenated compound counts as known only if both halves are.
	if !lexKnows(lex, "re-sleeve") && lexKnows(lex, "re") {
		t.Errorf("hyphen handling: unexpected verdict for re-sleeve")
	}
}

func keysOf(m map[string]glossCandidate) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
