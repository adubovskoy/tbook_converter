package cli

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/lexcheck"
	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
	"github.com/dimando/reader/converter/internal/translate"
)

func writeLexicon(t *testing.T, dir, source, target string, lines ...string) {
	t.Helper()
	f, err := os.Create(lexcheck.Path(dir, source, target))
	if err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(f)
	if _, err := zw.Write([]byte(strings.Join(lines, "\n") + "\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func frSentence(src string, tr tbook.Translation) *tbook.Sentence {
	s := v1Sentence(src, tr)
	s.Tr = map[string]tbook.Translation{"fr": tr}
	return s
}

// TestGatesSurviveTypography: the typography pass rewrites the text the gates
// judged, and the verdict cache keys on that exact text. Gate state must be
// resolved against the pre-typography text and still ship with the rewritten
// cell: a judge-flagged and a lexcheck-flagged cell keep their failing
// verdicts, a clean one stays clean.
func TestGatesSurviveTypography(t *testing.T) {
	cacheDir, lexDir := t.TempDir(), t.TempDir()
	out := filepath.Join(t.TempDir(), "out.tbook")

	judged := frSentence(`"She opened the window," he said.`,
		alignedTr(`"Elle a ouvert la fenêtre", dit-il.`, 6))
	// Every covered pair is dictionary-implausible: lexcheck low-support.
	drifted := frSentence(`"Dog cat bird fish horse," she said.`,
		alignedTr(`"Maison voiture arbre table fenêtre", dit-elle.`, 7))
	clean := frSentence(`"The dog ran home," he said.`,
		alignedTr(`"Le chien a couru", dit-il.`, 5))
	sents := []*tbook.Sentence{judged, drifted, clean}
	before := map[*tbook.Sentence]string{}
	for _, s := range sents {
		before[s] = s.Tr["fr"].Text
	}

	writeLexicon(t, lexDir, "en", "fr",
		"dog\tchien", "cat\tchat", "bird\toiseau", "fish\tpoisson", "horse\tcheval")

	cfg := &config.Config{
		Source: "en", Targets: []string{"fr"}, Out: out, Format: config.FormatV2,
		CacheDir: cacheDir, Judge: true, JudgeModel: "judge-model",
		JudgeScope: config.JudgeScopeAll,
		Lexcheck:   true, LexiconDir: lexDir,
		Typography: true,
	}
	if err := translate.SaveVerdict(cacheDir, cfg.JudgeModel, "en", "fr", judged,
		translate.Verdict{OK: false, Why: "mistranslation: wrong subject"}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []*tbook.Sentence{drifted, clean} {
		if err := translate.SaveVerdict(cacheDir, cfg.JudgeModel, "en", "fr", s,
			translate.Verdict{OK: true}); err != nil {
			t.Fatal(err)
		}
	}

	book := &tbook.Book{
		Title: "Fixture", Author: "Nobody", Source: "en", Targets: []string{"fr"},
		Chapters: []tbook.Chapter{{Title: "One", Paragraphs: [][]*tbook.Sentence{
			{judged}, {drifted}, {clean},
		}}},
	}
	g := finalizeCells(cfg, sents, []string{"fr"}, nil, nil, book.Chapters, nil, true)

	for _, s := range sents {
		if s.Tr["fr"].Text == before[s] {
			t.Fatalf("typography left %q unchanged; the fixture no longer exercises the pass", before[s])
		}
	}
	// The defect's precondition: the cache no longer answers for the new text.
	if _, ok := translate.VerdictFor(cacheDir, cfg.JudgeModel, "en", "fr", judged); ok {
		t.Fatal("verdict cache still matches the post-typography text")
	}

	run := tbook.RunRecord{At: time.Now().UTC().Format(time.RFC3339),
		Targets: []string{"fr"}, Options: map[string]any{"tokenizer": v2.TokenizerID}}
	if err := writeV2(cfg, book, &v2.Source{Kind: "epub"}, run, g); err != nil {
		t.Fatalf("writeV2: %v", err)
	}
	f, err := v2.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ov, ok, err := f.Overlay(0, "fr")
	if err != nil || !ok {
		t.Fatalf("overlay: %v (found %v)", err, ok)
	}
	if got := strings.Join(ov.Gates, ","); got != "langcheck,lexcheck,judge" {
		t.Fatalf("gates = %s", got)
	}
	cell := func(i int) *v2.Translation {
		if len(ov.Paragraphs[i].S) != 1 {
			t.Fatalf("paragraph %d has %d translations", i, len(ov.Paragraphs[i].S))
		}
		c := ov.Paragraphs[i].S[0]
		if c.T != sents[i].Tr["fr"].Text {
			t.Errorf("cell %d ships %q, want the post-typography %q", i, c.T, sents[i].Tr["fr"].Text)
		}
		return c
	}

	if c := cell(0); c.S != v2.StatusUnverified || c.V == nil || c.V.OK ||
		c.V.By != v2.GateJudge || !strings.Contains(c.V.Why, "mistranslation") {
		t.Errorf("judged cell: s=%q v=%+v, want unverified with the judge verdict", c.S, c.V)
	}
	// A lexcheck flag is a verdict only; it sets no status of its own.
	if c := cell(1); c.S != "" || c.V == nil || c.V.OK ||
		c.V.By != v2.GateLexcheck || c.V.Why != "low-support" {
		t.Errorf("lexcheck cell: s=%q v=%+v, want the lexcheck low-support verdict", c.S, c.V)
	}
	if c := cell(2); c.S != "" || c.V != nil {
		t.Errorf("clean cell: s=%q v=%+v, want neither", c.S, c.V)
	}
}

// TestGatesWithoutTypographyUnchanged: with the pass off the cells are
// written untouched and the flagged one still carries its verdict.
func TestGatesWithoutTypographyUnchanged(t *testing.T) {
	cacheDir := t.TempDir()
	judged := frSentence(`"She opened the window," he said.`,
		alignedTr(`"Elle a ouvert la fenêtre", dit-il.`, 6))
	orig := judged.Tr["fr"].Text
	cfg := &config.Config{Source: "en", Targets: []string{"fr"}, CacheDir: cacheDir,
		Judge: true, JudgeModel: "judge-model", JudgeScope: config.JudgeScopeAll}
	if err := translate.SaveVerdict(cacheDir, cfg.JudgeModel, "en", "fr", judged,
		translate.Verdict{OK: false, Why: "wrong"}); err != nil {
		t.Fatal(err)
	}
	chapters := []tbook.Chapter{{Paragraphs: [][]*tbook.Sentence{{judged}}}}
	g := finalizeCells(cfg, []*tbook.Sentence{judged}, []string{"fr"}, nil, nil, chapters, nil, true)
	if judged.Tr["fr"].Text != orig {
		t.Errorf("text changed with typography off: %q", judged.Tr["fr"].Text)
	}
	if !g.judgeFlagged("fr", judged.Src) {
		t.Error("judge verdict lost")
	}
	if g := finalizeCells(cfg, nil, nil, nil, nil, nil, nil, false); g != nil {
		t.Error("gate state built for a v1 write")
	}
}
