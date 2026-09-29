package typography

import (
	"reflect"
	"testing"

	"github.com/dimando/reader/converter/internal/tbook"
)

func TestRemap(t *testing.T) {
	before := `"Совершенно верно," — ответил он.`
	after := `«Совершенно верно», — ответил он.`
	// `"Совершенно` (quote included), `верно,"`, `ответил`, `—` (no letters)
	got, ok := Remap(before, after, [][2]int{{0, 11}, {12, 19}, {22, 29}, {20, 21}})
	if !ok {
		t.Fatal("remap failed")
	}
	want := [][2]int{{1, 11}, {12, 17}, {22, 29}, {-1, -1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if _, ok := Remap("abc", "abd", nil); ok {
		t.Fatal("letter change must fail")
	}
	if _, ok := Remap("abc", "abc", [][2]int{{0, 9}}); ok {
		t.Fatal("out-of-range span must fail")
	}
}

func TestApplyParagraphs(t *testing.T) {
	s1 := &tbook.Sentence{Src: "“Quite so,” he answered.", Tr: map[string]tbook.Translation{
		"ru": {Text: `"Совершенно верно," — ответил он.`, Align: []tbook.AlignChunk{
			{T: [2]int{1, 11}, W: []int{0}}, {T: [2]int{12, 17}, W: []int{1}}, {T: [2]int{22, 29}, W: []int{3}},
		}},
		"xx": {Text: `"x"`},
	}}
	s2 := &tbook.Sentence{Src: "I answered.", Tr: map[string]tbook.Translation{
		"ru": {Text: "ответил я.", Align: []tbook.AlignChunk{{T: [2]int{0, 7}, W: []int{1}}}},
	}}
	paras := [][]*tbook.Sentence{{s1, s2}}
	if n := ApplyParagraphs(paras, "ru"); n != 1 {
		t.Fatalf("changed %d, want 1", n)
	}
	tr := s1.Tr["ru"]
	if tr.Text != "«Совершенно верно», — ответил он." {
		t.Fatalf("text %q", tr.Text)
	}
	r := []rune(tr.Text)
	if w := string(r[tr.Align[2].T[0]:tr.Align[2].T[1]]); w != "ответил" {
		t.Fatalf("chunk 2 -> %q", w)
	}
	if s2.Tr["ru"].Text != "ответил я." {
		t.Fatalf("tag dash must need a quoted previous source: %q", s2.Tr["ru"].Text)
	}
	if ApplyParagraphs(paras, "ru") != 0 {
		t.Fatal("not idempotent")
	}
	if ApplyParagraphs(paras, "xx") != 0 || s1.Tr["xx"].Text != `"x"` {
		t.Fatal("unsupported language changed")
	}
}
