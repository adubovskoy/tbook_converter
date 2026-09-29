package learner

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/tbook/v2/migrate"
)

// consensusBook has one sentence of four words whose highlight sets cover all
// four consensus outcomes: unanimous, solo miss, minority miss, unanimous miss.
//
//	the     de ✓ fr ✓ ru ✓  unanimous   (function word)
//	garden  de ✓ fr ✗ ru ✓  solo miss, fr
//	deal    de ✗ fr ✗ ru ✗  unanimous miss
//	was     de ✗ fr ✗ ru ✓  minority miss, k = 2 (function word)
func consensusBook() *tbook.Book {
	ch := func(a, b int, w ...int) tbook.AlignChunk { return tbook.AlignChunk{T: [2]int{a, b}, W: w} }
	s := &tbook.Sentence{
		Src:   "The garden deal was.",
		Words: [][2]int{{0, 3}, {4, 10}, {11, 15}, {16, 19}},
		Tr: map[string]tbook.Translation{
			"de": {Text: "Der Gartentisch stand.", Align: []tbook.AlignChunk{ch(0, 3, 0), ch(4, 15, 1)}},
			"fr": {Text: "La table est là.", Align: []tbook.AlignChunk{ch(0, 2, 0)}},
			"ru": {Text: "Стол в саду был.", Align: []tbook.AlignChunk{ch(0, 4, 0), ch(7, 11, 1), ch(12, 15, 3)}},
		},
	}
	return &tbook.Book{
		Title: "Consensus", Author: "Test", Source: "en", Targets: []string{"de", "fr", "ru"},
		Chapters: []tbook.Chapter{{Title: "One", Paragraphs: [][]*tbook.Sentence{{s}}}},
	}
}

func writeBook(t *testing.T, b *tbook.Book, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := tbook.Write(path, b); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func checkConsensus(t *testing.T, path string) {
	t.Helper()
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := b.Consensus(ConsensusOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Sentences != 1 || r.Cells != 3 || r.Slots != 4 {
		t.Fatalf("sentences %d cells %d slots %d, want 1/3/4", r.Sentences, r.Cells, r.Slots)
	}
	if r.Unanimous != 1 || r.Missed != 3 {
		t.Errorf("unanimous %d missed %d, want 1 and 3", r.Unanimous, r.Missed)
	}
	for k, want := range map[int]int{0: 1, 1: 1, 2: 1, 3: 1} {
		if r.Hist[k] != want {
			t.Errorf("hist[%d] = %d, want %d", k, r.Hist[k], want)
		}
	}
	if r.Solo != 1 || r.SoloByLang["fr"] != 1 {
		t.Errorf("solo %d by lang %v, want 1 and fr:1", r.Solo, r.SoloByLang)
	}
	if r.SoloSentences != 1 {
		t.Errorf("solo sentences = %d, want 1", r.SoloSentences)
	}
	if r.UnanimousMiss != 1 || len(r.TopUnanimousMiss) != 1 || r.TopUnanimousMiss[0].Word != "deal" {
		t.Errorf("unanimous miss %d %v, want 1 on %q", r.UnanimousMiss, r.TopUnanimousMiss, "deal")
	}

	// Content-only drops the two function-word slots and keeps the verdicts.
	rc, err := b.Consensus(ConsensusOpts{ContentOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if rc.Slots != 2 || rc.Unanimous != 0 || rc.Solo != 1 || rc.UnanimousMiss != 1 {
		t.Errorf("content-only: slots %d unanimous %d solo %d unanimous-miss %d, want 2/0/1/1",
			rc.Slots, rc.Unanimous, rc.Solo, rc.UnanimousMiss)
	}
}

func TestConsensusV1(t *testing.T) { checkConsensus(t, writeBook(t, consensusBook(), "c.tbook")) }

func TestConsensusV2(t *testing.T) {
	out := filepath.Join(t.TempDir(), "c-v2.tbook")
	if _, err := migrate.File(writeBook(t, consensusBook(), "c.tbook"), out, migrate.Options{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	checkConsensus(t, out)
}

// A book with fewer than three targets carries no consensus signal at all, and
// the tool must say so instead of printing zeros.
func TestConsensusRefusesTwoTargets(t *testing.T) {
	b := consensusBook()
	b.Targets = []string{"de", "fr"}
	for _, ch := range b.Chapters {
		for _, para := range ch.Paragraphs {
			for _, s := range para {
				delete(s.Tr, "ru")
			}
		}
	}
	book, err := Load(writeBook(t, b, "two.tbook"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := book.Consensus(ConsensusOpts{}); err == nil {
		t.Fatal("expected a refusal for a 2-target book")
	} else if !strings.Contains(err.Error(), "at least 3") {
		t.Errorf("unclear refusal: %v", err)
	}
}

// A sentence that does not reach --min-langs aligned languages never votes.
func TestConsensusMinLangs(t *testing.T) {
	book, err := Load(writeBook(t, consensusBook(), "c.tbook"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := book.Consensus(ConsensusOpts{MinLangs: 4})
	if err != nil {
		t.Fatal(err)
	}
	if r.Sentences != 0 || r.Slots != 0 {
		t.Errorf("sentences %d slots %d, want 0 and 0", r.Sentences, r.Slots)
	}
}
