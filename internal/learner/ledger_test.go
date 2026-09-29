package learner

import (
	"path/filepath"
	"testing"

	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/tbook/v2/migrate"
)

// synthetic builds a two-paragraph chapter whose taps cover every class of the
// ledger. Offsets are code points, as in a real file.
func synthetic() *tbook.Book {
	de := func(text string, chunks ...tbook.AlignChunk) map[string]tbook.Translation {
		return map[string]tbook.Translation{"de": {Text: text, Align: chunks}}
	}
	ch := func(a, b int, w ...int) tbook.AlignChunk { return tbook.AlignChunk{T: [2]int{a, b}, W: w} }

	// clean: one exclusive chunk of one token per word.
	s1 := &tbook.Sentence{
		Src: "The cat sleeps.", Words: [][2]int{{0, 3}, {4, 7}, {8, 14}},
		Tr: de("Der Kater schläft.", ch(0, 3, 0), ch(4, 9, 1), ch(10, 17, 2)),
	}
	// clean-multi on "not" (two exclusive chunks), none on "does".
	s2 := &tbook.Sentence{
		Src: "He does not run.", Words: [][2]int{{0, 2}, {3, 7}, {8, 11}, {12, 15}},
		Tr: de("Il ne court pas.", ch(0, 2, 0), ch(3, 5, 2), ch(6, 11, 3), ch(12, 15, 2)),
	}
	// shared: one token stands for three source words.
	s3 := &tbook.Sentence{
		Src: "I can wait.", Words: [][2]int{{0, 1}, {2, 5}, {6, 10}},
		Tr: de("Bekleyebilirim.", ch(0, 14, 0, 1, 2)),
	}
	// smear on "tonight" (three target tokens light), shared on the rest.
	s4 := &tbook.Sentence{
		Src: "Lock yourself up tonight.", Words: [][2]int{{0, 4}, {5, 13}, {14, 16}, {17, 24}},
		Tr: de("Encerrarse esta noche para siempre.", ch(0, 10, 0, 1, 2), ch(11, 26, 3)),
	}
	// dead-sentence: a translation with no alignment at all.
	s5 := &tbook.Sentence{
		Src: "Wedlock suits you.", Words: [][2]int{{0, 7}, {8, 13}, {14, 17}},
		Tr: de("Die Ehe steht Ihnen gut."),
	}
	// no translation: not a tap class, but a coarse no-translation slot.
	s6 := &tbook.Sentence{Src: "Nothing here.", Words: [][2]int{{0, 7}, {8, 12}}}

	return &tbook.Book{
		Title: "Synthetic", Author: "Test", Source: "en", Targets: []string{"de"},
		Chapters: []tbook.Chapter{{
			Title:      "One",
			Paragraphs: [][]*tbook.Sentence{{s1, s2, s3}, {s4, s5, s6}},
		}},
	}
}

// wantClasses is the expected outcome of every tap of the synthetic chapter.
var wantClasses = [][]Class{
	{Clean, Clean, Clean},
	{Clean, None, CleanMulti, Clean},
	{Shared, Shared, Shared},
	{Shared, Shared, Shared, Smear},
	{Dead, Dead, Dead},
	nil, // no translation: no taps
}

func writeSynthetic(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic.tbook")
	if err := tbook.Write(path, synthetic()); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func checkClasses(t *testing.T, b *Book) {
	t.Helper()
	if len(b.Sents) != len(wantClasses) {
		t.Fatalf("sentences = %d, want %d", len(b.Sents), len(wantClasses))
	}
	for si, want := range wantClasses {
		s := b.Sents[si]
		cell := s.Tr["de"]
		if want == nil {
			if cell != nil {
				t.Errorf("sentence %d: expected no translation", si)
			}
			continue
		}
		if cell == nil {
			t.Fatalf("sentence %d: no de translation", si)
		}
		if len(s.Words) != len(want) {
			t.Fatalf("sentence %d: %d words, want %d", si, len(s.Words), len(want))
		}
		for i, w := range want {
			if got := cell.Classify(i); got != w {
				t.Errorf("sentence %d word %d (%q): class %s, want %s", si, i, s.Words[i], got, w)
			}
		}
	}
}

func TestClassesV1(t *testing.T) {
	b, err := Load(writeSynthetic(t))
	if err != nil {
		t.Fatal(err)
	}
	if b.Version != 1 {
		t.Fatalf("version = %d, want 1", b.Version)
	}
	checkClasses(t, b)
}

// The migrated file must classify identically: the ledger scores a version-2
// book through the version-2 tap algorithm, and the two must agree.
func TestClassesV2(t *testing.T) {
	v2path := filepath.Join(t.TempDir(), "synthetic-v2.tbook")
	if _, err := migrate.File(writeSynthetic(t), v2path, migrate.Options{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	b, err := Load(v2path)
	if err != nil {
		t.Fatal(err)
	}
	if b.Version != 2 {
		t.Fatalf("version = %d, want 2", b.Version)
	}
	checkClasses(t, b)
}

func TestLedgerAggregates(t *testing.T) {
	b, err := Load(writeSynthetic(t))
	if err != nil {
		t.Fatal(err)
	}
	rep := b.TapLedger(LedgerOpts{})
	if len(rep.Languages) != 1 {
		t.Fatalf("languages = %d, want 1", len(rep.Languages))
	}
	row := rep.Languages[0]
	want := map[string]int{"clean": 5, "clean-multi": 1, "shared": 6, "smear": 1, "none": 1, "dead-sentence": 3}
	for k, v := range want {
		if row.Classes[k] != v {
			t.Errorf("%s = %d, want %d", k, row.Classes[k], v)
		}
	}
	if row.Taps != 17 {
		t.Errorf("taps = %d, want 17", row.Taps)
	}
	// content words: sleeps, wait, Lock, yourself, tonight, Wedlock, suits.
	if row.ContentTaps != 7 || row.ContentSilent != 2 {
		t.Errorf("content taps %d silent %d, want 7 and 2", row.ContentTaps, row.ContentSilent)
	}
	// The coarse collapse counts the untranslated sentence and calls the smear
	// tap clean (its span is claimed by that word alone).
	c := row.Coarse
	if c.Taps != 19 || c.Clean != 7 || c.Shared != 6 || c.Dead != 4 || c.NoTranslation != 2 {
		t.Errorf("coarse = %+v, want taps 19 clean 7 shared 6 dead 4 noTranslation 2", c)
	}
}

func TestLedgerContentOnly(t *testing.T) {
	b, err := Load(writeSynthetic(t))
	if err != nil {
		t.Fatal(err)
	}
	row := b.TapLedger(LedgerOpts{ContentOnly: true}).Languages[0]
	if row.Taps != 7 {
		t.Fatalf("content-only taps = %d, want 7", row.Taps)
	}
	if row.Silent != 2 || row.SilentPct != row.ContentSilentPct {
		t.Errorf("silent = %d (%.2f%%), content silent %.2f%%", row.Silent, row.SilentPct, row.ContentSilentPct)
	}
}

func TestCountWords(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{
		{"", 0}, {" — ", 0}, {"esta noche para", 3}, {"don’t", 1}, {"well-known", 1},
		{"schläft.", 1}, {"«конечно»", 1}, {"4th", 1}, {"a b c", 3},
	} {
		rs := []rune(c.in)
		if got := countWords(rs, 0, len(rs)); got != c.want {
			t.Errorf("countWords(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
