package v2

import "testing"

const (
	fixS1 = "“Then I can wait in the next room.”"
	fixS2 = "That is just my point."
	fixS3 = "“That’s the worst of it, Mr. Holmes, I don’t know.”"
	fixS9 = "“‘Oh, at his new offices. He did tell me the address. Yes, 17 King Edward Street, near St. Paul’s.’"
)

func TestParagraphID(t *testing.T) {
	cases := map[string]string{fixS1: "04537723", fixS9: "781304bb", fixS2: "2b606e75", fixS3: "1bfb4bea"}
	for text, want := range cases {
		if got := ParagraphID(text); got != want {
			t.Errorf("ParagraphID(%q) = %s, want %s", text, got, want)
		}
	}
	// Decomposed input normalizes to the same id as its NFC form.
	if ParagraphID("café") != ParagraphID("café") {
		t.Error("ParagraphID must NFC-normalize")
	}
}

func TestChapterID(t *testing.T) {
	if got := ChapterID("Canonical sentences", []string{fixS1, fixS2, fixS3}); got != "cbedb517c" {
		t.Fatalf("ChapterID = %s, want cbedb517c", got)
	}
	// Only the first three body paragraphs count.
	if got := ChapterID("Canonical sentences", []string{fixS1, fixS2, fixS3, "extra"}); got != "cbedb517c" {
		t.Fatalf("fourth body paragraph must be ignored, got %s", got)
	}
	skel := &Chapter{Paragraphs: []Block{
		{Role: RoleHeading, Text: "Canonical sentences"},
		{Text: fixS1}, {Text: fixS2}, {Text: fixS3}, {Text: "more"},
	}}
	if got := ChapterIDOf("Canonical sentences", skel); got != "cbedb517c" {
		t.Fatalf("ChapterIDOf = %s", got)
	}
	if NormTitle("  Canonical \tSENTENCES ") != "canonical sentences" {
		t.Fatalf("NormTitle = %q", NormTitle("  Canonical \tSENTENCES "))
	}
	if NormTitle("Straße") != "strasse" {
		t.Fatalf("full case folding expected, got %q", NormTitle("Straße"))
	}
}

func TestIDSetSuffixes(t *testing.T) {
	s := IDSet{}
	got := []string{s.Add("a"), s.Add("b"), s.Add("a"), s.Add("a")}
	want := []string{"a", "b", "a-2", "a-3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}
