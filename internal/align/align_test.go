package align

import (
	"encoding/json"
	"testing"
)

// chunksFromJSON parses model-style chunk JSON for tests.
func chunksFromJSON(t *testing.T, s string) []Chunk {
	t.Helper()
	var c []Chunk
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		t.Fatalf("unmarshal chunks: %v", err)
	}
	return c
}

func TestBuildTextAlign_WordLevel(t *testing.T) {
	src := "Stan went to the living room."
	words := [][2]int{{0, 4}, {5, 9}, {10, 12}, {13, 16}, {17, 23}, {24, 28}}
	raw := "Стэн прошёл в гостиную."
	chunks := chunksFromJSON(t, `[
		{"tgt":"Стэн","en":"Stan"},
		{"tgt":"прошёл","en":"went"},
		{"tgt":"в","en":"to"},
		{"tgt":"гостиную","en":["living","room"]}
	]`)

	tr := BuildTextAlign(chunks, src, words, raw)
	// The raw translation is canonical — echoed fragments only locate spans.
	if tr.Text != raw {
		t.Fatalf("text = %q, want raw %q", tr.Text, raw)
	}
	// Offsets are RUNE indices into the raw text; spans cover the fragment's
	// letters (not adjacent punctuation).
	wantT := [][2]int{{0, 4}, {5, 11}, {12, 13}, {14, 22}}
	wantW := [][]int{{0}, {1}, {2}, {4, 5}}
	if len(tr.Align) != len(wantT) {
		t.Fatalf("got %d align chunks, want %d", len(tr.Align), len(wantT))
	}
	runes := []rune(tr.Text)
	for i, c := range tr.Align {
		if c.T != wantT[i] {
			t.Errorf("chunk %d span = %v, want %v (frag %q)", i, c.T, wantT[i], string(runes[c.T[0]:c.T[1]]))
		}
		if len(c.W) != len(wantW[i]) {
			t.Errorf("chunk %d w = %v, want %v", i, c.W, wantW[i])
		}
	}
}

func TestBuildTextAlign_InsertedAndRepeated(t *testing.T) {
	// "the" appears twice; an inserted target word has en [].
	src := "the cat and the dog"
	words := [][2]int{{0, 3}, {4, 7}, {8, 11}, {12, 15}, {16, 19}}
	raw := "кот и пёс (тот)"
	chunks := chunksFromJSON(t, `[
		{"tgt":"кот","en":"cat"},
		{"tgt":"и","en":"and"},
		{"tgt":"пёс","en":"dog"},
		{"tgt":"(тот)","en":[]}
	]`)
	tr := BuildTextAlign(chunks, src, words, raw)
	if tr.Text != raw {
		t.Fatalf("text = %q", tr.Text)
	}
	// The inserted "(тот)" must produce no align entry (empty w dropped).
	if len(tr.Align) != 3 {
		t.Fatalf("expected 3 align chunks, got %d: %+v", len(tr.Align), tr.Align)
	}
	// "dog" is index 4.
	if tr.Align[2].W[0] != 4 {
		t.Errorf("dog should map to word 4, got %v", tr.Align[2].W)
	}
}

// TestPunctuationSurvivesSloppyEcho guards the v6 point: an echo that drops
// the raw text's punctuation must not corrupt the shipped text — the raw
// translation is canonical and the echo only places highlights.
func TestPunctuationSurvivesSloppyEcho(t *testing.T) {
	src := "Well, he left."
	words := [][2]int{{0, 4}, {6, 8}, {9, 13}}
	raw := "Что ж, он ушёл."
	chunks := chunksFromJSON(t, `[
		{"tgt":"Что ж","en":"Well"},
		{"tgt":"он","en":"he"},
		{"tgt":"ушёл","en":"left"}
	]`)
	tr := BuildTextAlign(chunks, src, words, raw)
	if tr.Text != raw {
		t.Fatalf("raw text must survive verbatim, got %q", tr.Text)
	}
	if len(tr.Align) != 3 {
		t.Fatalf("expected 3 align chunks, got %+v", tr.Align)
	}
	rs := []rune(raw)
	if got := string(rs[tr.Align[0].T[0]:tr.Align[0].T[1]]); got != "Что ж" {
		t.Errorf("first span = %q, want %q", got, "Что ж")
	}
}

// TestRewrittenEchoRejected: a mapped fragment that does not occur in the raw
// translation means the align pass rewrote the text — the sentence must be
// rejected (zero Translation) so the pipeline retries it.
func TestRewrittenEchoRejected(t *testing.T) {
	src := "the dog"
	words := [][2]int{{0, 3}, {4, 7}}
	raw := "пёс"
	chunks := chunksFromJSON(t, `[{"tgt":"кот","en":"dog"}]`)
	tr := BuildTextAlign(chunks, src, words, raw)
	if tr.Text != "" || tr.Align != nil {
		t.Fatalf("diverged echo must reject the sentence, got %+v", tr)
	}
}

// TestPunctuationOnlyFragmentSkipped: fragments with no letters (").", "—")
// never make highlights — even when the model attached source words to them.
func TestPunctuationOnlyFragmentSkipped(t *testing.T) {
	src := "I want."
	words := [][2]int{{0, 1}, {2, 6}}
	raw := "я хочу)."
	chunks := chunksFromJSON(t, `[
		{"tgt":"я","en":"I"},
		{"tgt":"хочу","en":"want"},
		{"tgt":").","en":"want"}
	]`)
	tr := BuildTextAlign(chunks, src, words, raw)
	if tr.Text != raw {
		t.Fatalf("text = %q", tr.Text)
	}
	if len(tr.Align) != 2 {
		t.Fatalf("punctuation-only chunk must be dropped, got %+v", tr.Align)
	}
}

// TestPartialEchoStillMaps: a chunk the model skipped (word present in raw but
// never echoed) must not reject the sentence — later fragments still locate.
func TestPartialEchoStillMaps(t *testing.T) {
	src := "the black dog"
	words := [][2]int{{0, 3}, {4, 9}, {10, 13}}
	raw := "чёрный большой пёс"
	chunks := chunksFromJSON(t, `[
		{"tgt":"чёрный","en":"black"},
		{"tgt":"пёс","en":"dog"}
	]`)
	tr := BuildTextAlign(chunks, src, words, raw)
	if tr.Text != raw || len(tr.Align) != 2 {
		t.Fatalf("partial echo should map what it can, got %+v", tr)
	}
	rs := []rune(raw)
	if got := string(rs[tr.Align[1].T[0]:tr.Align[1].T[1]]); got != "пёс" {
		t.Errorf("second span = %q, want %q", got, "пёс")
	}
}

func TestEnField(t *testing.T) {
	// Each case decodes into a fresh Chunk (as the pipeline does, one per array
	// element), so an absent "en" yields a nil slice.
	decode := func(s string) Chunk {
		var c Chunk
		if err := json.Unmarshal([]byte(s), &c); err != nil {
			t.Fatalf("unmarshal %s: %v", s, err)
		}
		return c
	}
	if c := decode(`{"tgt":"x","en":"Stan"}`); len(c.En) != 1 || c.En[0] != "Stan" {
		t.Errorf("string en: %v", c.En)
	}
	if c := decode(`{"tgt":"x","en":["a","b"]}`); len(c.En) != 2 {
		t.Errorf("array en: %v", c.En)
	}
	if c := decode(`{"tgt":"x","en":[]}`); len(c.En) != 0 {
		t.Errorf("empty en: %v", c.En)
	}
	if c := decode(`{"tgt":"x"}`); c.En != nil {
		t.Errorf("absent en: %v", c.En)
	}
	// Multi-word "en" strings are whitespace-split into tokens (defensive: models
	// emit "living room" instead of ["living","room"]). Without this they'd match
	// nothing and drop the highlight.
	if c := decode(`{"tgt":"x","en":"living room"}`); len(c.En) != 2 || c.En[0] != "living" || c.En[1] != "room" {
		t.Errorf("multiword string en: %v", c.En)
	}
	if c := decode(`{"tgt":"x","en":["good bye"]}`); len(c.En) != 2 || c.En[0] != "good" || c.En[1] != "bye" {
		t.Errorf("multiword array-element en: %v", c.En)
	}
}

// TestMultiWordEnStringResolves guards the bug this fixed: a multi-word "en"
// STRING must resolve to BOTH source-word indices, not zero.
func TestMultiWordEnStringResolves(t *testing.T) {
	src := "Stan went to the living room."
	words := [][2]int{{0, 4}, {5, 9}, {10, 12}, {13, 16}, {17, 23}, {24, 28}}
	chunks := []Chunk{{Tgt: "гостиную", En: mustEn(t, `"living room"`)}}
	tr := BuildTextAlign(chunks, src, words, "гостиную")
	if len(tr.Align) != 1 || len(tr.Align[0].W) != 2 || tr.Align[0].W[0] != 4 || tr.Align[0].W[1] != 5 {
		t.Errorf("multiword en string should resolve to [4,5], got %+v", tr.Align)
	}
}

func mustEn(t *testing.T, s string) EnField {
	t.Helper()
	var e EnField
	if err := json.Unmarshal([]byte(s), &e); err != nil {
		t.Fatalf("unmarshal en %s: %v", s, err)
	}
	return e
}

func TestNumberedEchoResolution(t *testing.T) {
	src := "la capa más profunda incluye tu identidad"
	words := [][2]int{{0, 2}, {3, 7}, {8, 11}, {12, 20}, {21, 28}, {29, 31}, {32, 41}}
	raw := "самый глубокий слой включает это идентичность"
	chunks := []Chunk{
		{Tgt: "самый", En: EnField{"2:más"}},                // verified index
		{Tgt: "глубокий", En: EnField{"3:profunda"}},        // verified index
		{Tgt: "слой", En: EnField{"5:capa"}},                // index/text mismatch → text fallback → 1
		{Tgt: "включает", En: EnField{"incluye"}},           // plain text still works
		{Tgt: "это", En: EnField{}},                         // inserted
		{Tgt: "идентичность", En: EnField{"99:identidad"}},  // out-of-range index → text fallback
	}
	tr := BuildTextAlign(chunks, src, words, raw)
	want := map[string][]int{
		"самый": {2}, "глубокий": {3}, "слой": {1}, "включает": {4}, "идентичность": {6},
	}
	got := map[string][]int{}
	rs := []rune(tr.Text)
	for _, c := range tr.Align {
		got[string(rs[c.T[0]:c.T[1]])] = c.W
	}
	for frag, w := range want {
		if len(got[frag]) != len(w) || got[frag][0] != w[0] {
			t.Errorf("%s → %v, want %v", frag, got[frag], w)
		}
	}
	if _, ok := got["это"]; ok {
		t.Errorf("inserted word should carry no align chunk")
	}
}

// TestShortFragmentNeedsWordBoundary: when the cursor lags (a preceding chunk
// was skipped), a one-letter fragment must bind to the standalone word, never
// to the same letter inside a longer word.
func TestShortFragmentNeedsWordBoundary(t *testing.T) {
	src := "hello and goodbye"
	words := [][2]int{{0, 5}, {6, 9}, {10, 17}}
	raw := "привет и пока"
	// No chunk for "привет" — the cursor sits at 0 when "и" is located; the
	// "и" inside "прИвет" must not match.
	chunks := chunksFromJSON(t, `[
		{"tgt":"и","en":"and"},
		{"tgt":"пока","en":"goodbye"}
	]`)
	tr := BuildTextAlign(chunks, src, words, raw)
	if len(tr.Align) != 2 {
		t.Fatalf("expected 2 align chunks, got %+v", tr.Align)
	}
	rs := []rune(raw)
	if got := string(rs[tr.Align[0].T[0]:tr.Align[0].T[1]]); got != "и" || tr.Align[0].T[0] != 7 {
		t.Errorf("fragment 'и' bound at %v (%q), want standalone word at offset 7", tr.Align[0].T, got)
	}
}
