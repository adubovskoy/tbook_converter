package segment

import (
	"reflect"
	"testing"

	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/sentencizer/sentencizer"
)

func TestCleanWithSpansMapsThroughWhitespace(t *testing.T) {
	// raw "He  was here" — two spaces collapse to one, so "was" shifts left by one.
	raw := "He  was here"
	clean, spans := CleanWithSpans(raw, []tbook.Span{{S: 4, E: 7, K: tbook.SpanItalic}}) // "was" in raw
	if clean != "He was here" {
		t.Fatalf("clean = %q", clean)
	}
	want := []tbook.Span{{S: 3, E: 6, K: tbook.SpanItalic}} // "was" in clean
	if !reflect.DeepEqual(spans, want) {
		t.Fatalf("spans = %+v, want %+v", spans, want)
	}
	if got := string([]rune(clean)[spans[0].S:spans[0].E]); got != "was" {
		t.Fatalf("emphasized text = %q, want \"was\"", got)
	}
}

func TestSegmentParagraphDistributesSpans(t *testing.T) {
	seg := sentencizer.NewSegmenter("en")
	// "very" (offset 7..11) is emphasized; it must land on the first sentence only,
	// rebased to that sentence's own coordinates (which start at 0 here).
	text := "He was very tired. Stan left."
	res := segmentParagraph(text, []tbook.Span{{S: 7, E: 11, K: tbook.SpanItalic}}, seg)
	if len(res) != 2 {
		t.Fatalf("got %d sentences, want 2: %+v", len(res), res)
	}
	if len(res[0].Spans) != 1 {
		t.Fatalf("sentence 0 spans = %+v, want 1", res[0].Spans)
	}
	sp := res[0].Spans[0]
	if got := string([]rune(res[0].Src)[sp.S:sp.E]); got != "very" {
		t.Fatalf("emphasized = %q, want \"very\"", got)
	}
	if len(res[1].Spans) != 0 {
		t.Fatalf("sentence 1 should have no spans, got %+v", res[1].Spans)
	}
}

func TestSegmentParagraphSpanCrossingSentences(t *testing.T) {
	seg := sentencizer.NewSegmenter("en")
	// A span covering the join of two sentences must be clipped to each.
	text := "One done. Two next."
	// emphasize "done. Two" → rune [4,13)
	res := segmentParagraph(text, []tbook.Span{{S: 4, E: 13, K: tbook.SpanBold}}, seg)
	if len(res) != 2 {
		t.Fatalf("got %d sentences, want 2", len(res))
	}
	if got := string([]rune(res[0].Src)[res[0].Spans[0].S:res[0].Spans[0].E]); got != "done." {
		t.Fatalf("sentence 0 emphasized = %q, want \"done.\"", got)
	}
	if got := string([]rune(res[1].Src)[res[1].Spans[0].S:res[1].Spans[0].E]); got != "Two" {
		t.Fatalf("sentence 1 emphasized = %q, want \"Two\"", got)
	}
}

func TestBuildSentenceObjectsKeepsSceneBreakAndStyles(t *testing.T) {
	chapters := []ParsedChapter{{
		Title: "C1",
		Paragraphs: []ParsedParagraph{
			{Text: "First body.", Role: tbook.RoleBody},
			{Text: "", Role: tbook.RoleSceneBreak},
			{Text: "Sub", Role: tbook.RoleSubtitle},
		},
	}}
	out, all := BuildSentenceObjects(chapters)
	if len(out) != 1 {
		t.Fatalf("chapters = %d", len(out))
	}
	ch := out[0]
	if len(ch.Paragraphs) != 3 {
		t.Fatalf("paragraphs = %d, want 3 (scene break kept)", len(ch.Paragraphs))
	}
	if len(ch.Paragraphs[1]) != 0 {
		t.Fatalf("scene-break paragraph should be empty, got %d sentences", len(ch.Paragraphs[1]))
	}
	wantStyles := []string{tbook.RoleBody, tbook.RoleSceneBreak, tbook.RoleSubtitle}
	if !reflect.DeepEqual(ch.ParagraphStyles, wantStyles) {
		t.Fatalf("styles = %v, want %v", ch.ParagraphStyles, wantStyles)
	}
	if len(all) != 2 { // body + subtitle sentences; scene break contributes none
		t.Fatalf("flat sentences = %d, want 2", len(all))
	}
}

func TestSplitSentencesUnchanged(t *testing.T) {
	seg := sentencizer.NewSegmenter("en")
	got := SplitSentences("One sentence. Two sentence.", seg)
	want := []string{"One sentence.", "Two sentence."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitSentences = %v, want %v", got, want)
	}
	if SplitSentences("   ", seg) != nil {
		t.Fatalf("blank paragraph should yield nil")
	}
}
