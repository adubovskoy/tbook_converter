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
	clean, spans, _ := CleanWithSpans(raw, []tbook.Span{{S: 4, E: 7, K: tbook.SpanItalic}}, nil) // "was" in raw
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
	res := segmentParagraph(text, []tbook.Span{{S: 7, E: 11, K: tbook.SpanItalic}}, nil, seg)
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
	res := segmentParagraph(text, []tbook.Span{{S: 4, E: 13, K: tbook.SpanBold}}, nil, seg)
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
	out, all := BuildSentenceObjects(chapters, "en")
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

func TestSegmentParagraphDistributesMarks(t *testing.T) {
	seg := sentencizer.NewSegmenter("en")
	// Marker 1 sits right after "tired." (end of sentence 1); marker 2 mid-word
	// region of sentence 2; marker 3 at the very end.
	text := "He was very tired. Stan left the room."
	marks := []Mark{
		{Pos: 18, ID: "n1", Label: "1"},  // after "tired."
		{Pos: 28, ID: "n2", Label: "*"},  // after "left" in sentence 2
		{Pos: 38, ID: "n3", Label: "2"},  // end of text
	}
	res := segmentParagraph(text, nil, marks, seg)
	if len(res) != 2 {
		t.Fatalf("got %d sentences, want 2", len(res))
	}
	if len(res[0].Marks) != 1 || res[0].Marks[0].ID != "n1" {
		t.Fatalf("sentence 0 marks = %+v, want n1", res[0].Marks)
	}
	if got := res[0].Marks[0].Pos; got != len([]rune(res[0].Src)) {
		t.Errorf("n1 pos = %d, want end of sentence (%d)", got, len([]rune(res[0].Src)))
	}
	if len(res[1].Marks) != 2 || res[1].Marks[0].ID != "n2" || res[1].Marks[1].ID != "n3" {
		t.Fatalf("sentence 1 marks = %+v, want n2,n3", res[1].Marks)
	}
	// "Stan left| the room." — n2 lands right after "left" (local offset 9).
	if got := res[1].Marks[0].Pos; got != 9 {
		t.Errorf("n2 local pos = %d, want 9", got)
	}
	if got := res[1].Marks[1].Pos; got != len([]rune(res[1].Src)) {
		t.Errorf("n3 pos = %d, want end", got)
	}
}

func TestBuildNotesSeparatesCitations(t *testing.T) {
	notes := []ParsedNote{
		{ID: "n1", Label: "1", Kind: tbook.NoteKindCitation,
			Paragraphs: []ParsedParagraph{{Text: "Duhigg, The Power of Habit, 2014.", Role: tbook.RoleBody}}},
		{ID: "n2", Label: "*", Kind: tbook.NoteKindNote,
			Paragraphs: []ParsedParagraph{{Text: "A content note. With two sentences.", Role: tbook.RoleBody}}},
	}
	out, noteSents, citeSents := BuildNotes(notes, "en")
	if len(out) != 2 {
		t.Fatalf("notes = %d, want 2", len(out))
	}
	if len(citeSents) != 1 {
		t.Fatalf("citation sentences = %d, want 1", len(citeSents))
	}
	if len(noteSents) != 2 {
		t.Fatalf("note sentences = %d, want 2", len(noteSents))
	}
	if len(out["n2"].Paragraphs) != 1 || len(out["n2"].Paragraphs[0]) != 2 {
		t.Fatalf("n2 paragraphs = %+v", out["n2"].Paragraphs)
	}
}

func TestInsertTitleHeading(t *testing.T) {
	ch := ParsedChapter{
		Title: "I. A SCANDAL IN BOHEMIA",
		Paragraphs: []ParsedParagraph{
			{Text: "To Sherlock Holmes she is always the woman.", Role: tbook.RoleBody},
		},
	}
	InsertTitleHeading(&ch)
	if len(ch.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2 (title heading + body)", len(ch.Paragraphs))
	}
	if h := ch.Paragraphs[0]; h.Role != tbook.RoleHeading || h.Text != ch.Title {
		t.Fatalf("paragraph 0 = %+v, want the title as heading", h)
	}

	// Idempotent: an equal leading heading is not duplicated.
	InsertTitleHeading(&ch)
	if len(ch.Paragraphs) != 2 {
		t.Fatalf("paragraphs after second insert = %d, want 2", len(ch.Paragraphs))
	}

	// A different leading heading (a section title) stays below the inserted title.
	ch2 := ParsedChapter{
		Title:      "Chapter One",
		Paragraphs: []ParsedParagraph{{Text: "PART ONE", Role: tbook.RoleHeading}},
	}
	InsertTitleHeading(&ch2)
	if len(ch2.Paragraphs) != 2 || ch2.Paragraphs[0].Text != "Chapter One" {
		t.Fatalf("paragraphs = %+v, want inserted title above section heading", ch2.Paragraphs)
	}

	// Blank titles insert nothing.
	empty := ParsedChapter{Paragraphs: []ParsedParagraph{{Text: "x", Role: tbook.RoleBody}}}
	InsertTitleHeading(&empty)
	if len(empty.Paragraphs) != 1 {
		t.Fatalf("blank title inserted a heading: %+v", empty.Paragraphs)
	}
}
