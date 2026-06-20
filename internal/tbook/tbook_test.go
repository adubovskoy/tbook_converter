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
	mk := func(spans []Span) []Chapter {
		return []Chapter{{Paragraphs: [][]*Sentence{{{
			Src:   "Hello world", // 11 runes
			Words: [][2]int{{0, 5}, {6, 11}},
			Tr:    map[string]Translation{"ru": {Text: "Привет мир", Align: []AlignChunk{}}},
			Spans: spans,
		}}}}}
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

func TestValidateToleratesSceneBreakParagraph(t *testing.T) {
	chs := []Chapter{{
		Paragraphs:      [][]*Sentence{{}, {{Src: "Hi", Words: [][2]int{{0, 2}}, Tr: map[string]Translation{"ru": {Text: "Привет", Align: []AlignChunk{}}}}}},
		ParagraphStyles: []string{RoleSceneBreak, RoleBody},
	}}
	rep := Validate(chs, []string{"ru"})
	if rep.OffsetErrors != 0 {
		t.Fatalf("scene-break (empty) paragraph caused errors: %+v", rep)
	}
	if rep.Sentences != 1 {
		t.Fatalf("sentences = %d, want 1", rep.Sentences)
	}
}
