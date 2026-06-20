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
	chunks := chunksFromJSON(t, `[
		{"tgt":"Стэн","en":"Stan"},
		{"tgt":"прошёл","en":"went"},
		{"tgt":"в","en":"to"},
		{"tgt":"гостиную","en":["living","room"]}
	]`)

	tr := BuildTextAlign(chunks, src, words)
	if tr.Text != "Стэн прошёл в гостиную" {
		t.Fatalf("text = %q", tr.Text)
	}
	// Offsets must be RUNE indices into the Cyrillic text, not bytes.
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
	chunks := chunksFromJSON(t, `[
		{"tgt":"кот","en":"cat"},
		{"tgt":"и","en":"and"},
		{"tgt":"пёс","en":"dog"},
		{"tgt":"(тот)","en":[]}
	]`)
	tr := BuildTextAlign(chunks, src, words)
	if tr.Text != "кот и пёс (тот)" {
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
}

func TestSmartJoinSpacing(t *testing.T) {
	// Punctuation attaches without a leading space; words get one.
	src := "Hello world"
	words := [][2]int{{0, 5}, {6, 11}}
	chunks := chunksFromJSON(t, `[{"tgt":"Привет","en":"Hello"},{"tgt":"мир","en":"world"},{"tgt":"!","en":[]}]`)
	tr := BuildTextAlign(chunks, src, words)
	if tr.Text != "Привет мир!" {
		t.Fatalf("text = %q", tr.Text)
	}
}
