package tbook

import (
	"reflect"
	"testing"
)

func TestTrimStyles(t *testing.T) {
	cases := []struct {
		name   string
		styles []string
		n      int
		want   []string
	}{
		{"all body → nil", []string{"body", "body"}, 2, nil},
		{"empty → nil", nil, 3, nil},
		{"trailing body dropped", []string{"subtitle", "body", "body"}, 3, []string{"subtitle"}},
		{"dense up to last non-body", []string{"subtitle", "body", "sceneBreak"}, 3, []string{"subtitle", "body", "sceneBreak"}},
		{"short input padded with body", []string{"sceneBreak"}, 3, []string{"sceneBreak"}},
		{"blank treated as body", []string{"", "heading"}, 2, []string{"body", "heading"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := trimStyles(c.styles, c.n); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("trimStyles(%v, %d) = %v, want %v", c.styles, c.n, got, c.want)
			}
		})
	}
}

func TestValidateSpans(t *testing.T) {
	mk := func(spans []Span) *Book {
		return &Book{Chapters: []Chapter{{Paragraphs: [][]*Sentence{{{
			Src:   "Hello world", // 11 runes
			Words: [][2]int{{0, 5}, {6, 11}},
			Tr:    map[string]Translation{"ru": {Text: "Привет мир", Align: []AlignChunk{}}},
			Spans: spans,
		}}}}}}
	}
	if rep := Validate(mk([]Span{{S: 0, E: 5, K: SpanItalic}}), []string{"ru"}); !rep.OK() {
		t.Fatalf("valid span rejected: %+v", rep)
	}
	if rep := Validate(mk([]Span{{S: 6, E: 99, K: SpanItalic}}), []string{"ru"}); rep.OffsetErrors == 0 {
		t.Fatalf("out-of-range span should error")
	}
	if rep := Validate(mk([]Span{{S: 0, E: 5, K: "x"}}), []string{"ru"}); rep.OffsetErrors == 0 {
		t.Fatalf("bad span kind should error")
	}
	if rep := Validate(mk([]Span{{S: 5, E: 5, K: SpanItalic}}), []string{"ru"}); rep.OffsetErrors == 0 {
		t.Fatalf("empty span should error")
	}
}

func TestValidateNotesFiguresTables(t *testing.T) {
	b := &Book{
		Images: map[string][]byte{"images/img1.jpg": {1}},
		Notes: map[string]*Note{
			"n1": {Label: "1", Kind: NoteKindNote, Paragraphs: [][]*Sentence{{
				{Src: "Note body.", Words: [][2]int{{0, 4}}, Tr: map[string]Translation{}},
			}}},
		},
		Chapters: []Chapter{{
			Paragraphs: [][]*Sentence{
				{{Src: "Hello.", Words: [][2]int{{0, 5}}, Tr: map[string]Translation{},
					Notes: []NoteRef{{P: 6, ID: "n1", Label: "1"}}}},
				{{Src: "Caption.", Words: [][2]int{{0, 7}}, Tr: map[string]Translation{}}},
				{},
			},
			ParagraphStyles: []string{RoleBody, RoleFigure, RoleTable},
			Figures:         []Figure{{Para: 1, Image: "images/img1.jpg"}},
			Tables: []Table{{Para: 2, Rows: [][]TableCell{{
				{Sentences: []*Sentence{{Src: "Cell", Words: [][2]int{{0, 4}}, Tr: map[string]Translation{}}}, Header: true},
			}}}},
		}},
	}
	rep := Validate(b, nil)
	if rep.OffsetErrors != 0 || rep.StructErrors != 0 {
		t.Fatalf("valid book rejected: %+v", rep)
	}
	if rep.Sentences != 4 { // body + caption + cell + note body
		t.Fatalf("sentences = %d, want 4", rep.Sentences)
	}

	// Marker past the end of src → offset error.
	bad := *b
	bad.Chapters[0].Paragraphs[0][0].Notes = []NoteRef{{P: 99, ID: "n1", Label: "1"}}
	if rep := Validate(&bad, nil); rep.OffsetErrors == 0 {
		t.Fatal("out-of-range marker should error")
	}
	bad.Chapters[0].Paragraphs[0][0].Notes = []NoteRef{{P: 6, ID: "nX", Label: "1"}}
	if rep := Validate(&bad, nil); rep.StructErrors == 0 {
		t.Fatal("unresolved note id should error")
	}
	bad.Chapters[0].Paragraphs[0][0].Notes = nil
	bad.Chapters[0].Figures = []Figure{{Para: 1, Image: "images/missing.jpg"}}
	if rep := Validate(&bad, nil); rep.StructErrors == 0 {
		t.Fatal("missing image entry should error")
	}
}

func TestValidateToleratesSceneBreakParagraph(t *testing.T) {
	chs := []Chapter{{
		Paragraphs:      [][]*Sentence{{}, {{Src: "Hi", Words: [][2]int{{0, 2}}, Tr: map[string]Translation{"ru": {Text: "Привет", Align: []AlignChunk{}}}}}},
		ParagraphStyles: []string{RoleSceneBreak, RoleBody},
	}}
	rep := Validate(&Book{Chapters: chs}, []string{"ru"})
	if rep.OffsetErrors != 0 {
		t.Fatalf("scene-break (empty) paragraph caused errors: %+v", rep)
	}
	if rep.Sentences != 1 {
		t.Fatalf("sentences = %d, want 1", rep.Sentences)
	}
}

func TestCoverageQ(t *testing.T) {
	tr := Translation{
		Text: "Стэн прошёл в гостиную.",
		Align: []AlignChunk{
			{T: [2]int{0, 4}, W: []int{0}},   // Стэн — aligned
			{T: [2]int{5, 11}, W: []int{1}},  // прошёл — aligned
			{T: [2]int{12, 13}, W: []int{}},  // в — unaligned
			{T: [2]int{14, 22}, W: []int{2}}, // гостиную — aligned
		},
	}
	if q := coverageQ(tr); q != 0.75 {
		t.Fatalf("q = %v, want 0.75", q)
	}
	if q := coverageQ(Translation{Text: "", Align: nil}); q != 0 {
		t.Fatalf("empty q = %v, want 0", q)
	}
}
