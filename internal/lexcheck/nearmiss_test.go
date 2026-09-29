package lexcheck

import "testing"

// The measured class: the glossary enforced «эскртуар» for escritoire and the
// proofread prompt protected it, so all 9 occurrences shipped verbatim. The
// lexicon knows «эскритуар», one deletion away.
func TestNearMiss(t *testing.T) {
	d := &Dict{m: map[string][]string{
		"escritoire": {"эскритуар", "секретер"},
		"sleeve":     {"рукав"},
		"stack":      {"стопка", "стек"},
	}, source: "en", target: "ru"}

	if near, ok := d.NearMiss("escritoire", "эскртуар"); !ok || near != "эскритуар" {
		t.Errorf("NearMiss(escritoire, эскртуар) = %q, %v; want эскритуар, true", near, ok)
	}
	// A book's own sense is not a typo: «оболочка» is nowhere near «рукав».
	if near, ok := d.NearMiss("sleeve", "оболочка"); ok {
		t.Errorf("NearMiss(sleeve, оболочка) = %q; a free rendering must not read as a typo", near)
	}
	// A plausible rendering never reaches the check.
	if _, ok := d.NearMiss("escritoire", "секретер"); ok {
		t.Error("a supported rendering was called a near miss")
	}
	// Short words are below the floor: two edits there reach a quarter of the
	// word and any two words look alike.
	if _, ok := d.NearMiss("stack", "стек"); ok {
		t.Error("a short rendering was judged")
	}
	// An unknown source word carries no evidence either way.
	if _, ok := d.NearMiss("needlecast", "иглокаст"); ok {
		t.Error("an uncovered source word produced a verdict")
	}
}

func TestEditDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"эскритуар", "эскритуар", 0},
		{"эскритуар", "эскртуар", 1},
		{"abcdef", "abcdef", 0},
		{"abcdef", "abcxyz", 3}, // above the bound: reported as bound+1
	}
	for _, c := range cases {
		if got := editDistance(c.a, c.b, nearMaxEdits); got != c.want && !(c.want > nearMaxEdits && got > nearMaxEdits) {
			t.Errorf("editDistance(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
