package translate

import (
	"strings"
	"testing"
)

// senseLexicon is a tiny bilingual dictionary: source word → known renderings.
type senseLexicon map[string][]string

func (f senseLexicon) Covered(src string) bool { _, ok := f[foldLower(src)]; return ok }

func (f senseLexicon) Supports(src, tgt string) bool {
	for _, c := range f[foldLower(src)] {
		if c == tgt {
			return true
		}
	}
	return false
}

func (f senseLexicon) NearMiss(src, tgt string) (string, bool) {
	if f.Supports(src, tgt) {
		return "", false
	}
	for _, c := range f[foldLower(src)] {
		if editish(c, tgt) {
			return c, true
		}
	}
	return "", false
}

// editish is "within two edits", enough for the fixture.
func editish(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	if len(ra) < len(rb) {
		ra, rb = rb, ra
	}
	if len(ra)-len(rb) > 2 {
		return false
	}
	i, j, diff := 0, 0, 0
	for i < len(ra) && j < len(rb) {
		if ra[i] == rb[j] {
			i, j = i+1, j+1
			continue
		}
		diff++
		if len(ra) == len(rb) {
			i, j = i+1, j+1
		} else {
			i++
		}
	}
	return diff+(len(ra)-i) <= 2
}

func foldLower(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + 32
		}
	}
	return string(out)
}

// TestGateGlossaryDropsForeignRenderings: the entries that taught the leak.
// «Айрин Адлер» is a correct Russian rendering and a wrong German one, and the
// old "contains a target-script letter" rule could not see it, because for a
// Latin target that rule has no opinion at all.
func TestGateGlossaryDropsForeignRenderings(t *testing.T) {
	entries := []GlossEntry{
		{Src: "Irene Adler", Tgt: "Айрин Адлер"},
		{Src: "Sherlock Holmes", Tgt: "Sherlock Holmes"}, // self-equal
		{Src: "JacSol", Tgt: "JacSol"},
		{Src: "Baker Street", Tgt: "Bakerstraße"},
	}
	kept, dropped := GateGlossary(entries, "en", "de", nil)
	if len(kept) != 1 || kept[0].Src != "Baker Street" {
		t.Errorf("kept %+v, want only the German rendering", kept)
	}
	if len(dropped) != 3 {
		t.Fatalf("dropped %d, want 3", len(dropped))
	}
	for _, d := range dropped {
		if d.Why == "" {
			t.Errorf("%q → %q dropped with no reason", d.Src, d.Tgt)
		}
	}
	// Same list for ru: there the Cyrillic entry is the right one.
	keptRu, _ := GateGlossary(entries, "en", "ru", nil)
	if len(keptRu) != 1 || keptRu[0].Src != "Irene Adler" {
		t.Errorf("kept %+v for ru, want the Cyrillic rendering", keptRu)
	}
}

// TestGateGlossaryMarksSuspect is the `escritoire → эскртуар` class: the
// glossary enforced a typo and the proofread prompt protected it, so all 9
// occurrences survived verbatim. The gate marks it and never rewrites it.
func TestGateGlossaryMarksSuspect(t *testing.T) {
	lex := senseLexicon{
		"escritoire": {"эскритуар", "секретер"},
		"sleeve":     {"рукав"},
	}
	entries := []GlossEntry{
		{Src: "escritoire", Tgt: "эскртуар"},
		{Src: "sleeve", Tgt: "оболочка"}, // a book's own sense, nowhere near the dictionary
		{Src: "stack", Tgt: "стек"},      // not in the lexicon at all
	}
	kept, dropped := GateGlossary(entries, "en", "ru", lex)
	if len(dropped) != 0 || len(kept) != 3 {
		t.Fatalf("kept %d, dropped %d: the gate must MARK, never drop, a suspect", len(kept), len(dropped))
	}
	sus := Suspects(kept)
	if len(sus) != 1 || sus[0].Src != "escritoire" || sus[0].Suspect != "эскритуар" {
		t.Fatalf("suspects = %+v, want escritoire with the near spelling", sus)
	}
	// The repair prompt is where it matters: a suspect loses the never-change
	// protection so the proofreader may fix the word.
	prompt := repairSystemPrompt("English", "Russian", kept, 0)
	if strings.Contains(prompt, "эскртуар") {
		t.Error("the suspect rendering is still in the proofreader's protected list")
	}
	if !strings.Contains(prompt, "оболочка") {
		t.Error("an ordinary entry lost its protection")
	}
	// The translate prompt keeps it: consistency across the book is the point,
	// and the lexicon does not get to overrule the glossary.
	if !strings.Contains(TranslateSystemPrompt("English", "Russian", kept), "эскртуар") {
		t.Error("the suspect entry was dropped from the translate prompt")
	}
}
