package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/epub"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
	"github.com/dimando/reader/converter/internal/translate"
)

// alignedTr renders text as a translation whose i-th word maps to source word i
// — enough to exercise the link mapping without an aligner.
func alignedTr(text string, srcWords int) tbook.Translation {
	tr := tbook.Translation{Text: text, Align: []tbook.AlignChunk{}}
	for i, w := range segment.Tokenize(text) {
		if i >= srcWords {
			break
		}
		tr.Align = append(tr.Align, tbook.AlignChunk{T: w, W: []int{i}})
	}
	return tr
}

func v1Sentence(src string, tr tbook.Translation) *tbook.Sentence {
	return &tbook.Sentence{Src: src, Words: segment.Tokenize(src),
		Tr: map[string]tbook.Translation{"ru": tr}}
}

// TestWriteV2Statuses drives the --format v2 assembly over a model that carries
// one clean cell, one the language check rejects, one the judge flagged, and
// one whose alignment failed — and checks that each ships with the status and
// verdict the pipeline actually recorded, that gates names what ran, that
// manifest.source survives, and that the file passes §11.1 validation.
func TestWriteV2Statuses(t *testing.T) {
	cacheDir := t.TempDir()
	out := filepath.Join(t.TempDir(), "out.tbook")

	const untranslated = "He walked slowly down the long dark street"
	clean := v1Sentence("The dog ran home.", alignedTr("The dog ran home", 4))
	// The source copied through: flagged, but the kind has no measured
	// precision, so it badges as unverified and claims no language.
	copied := v1Sentence(untranslated+".", alignedTr(untranslated, 8))
	// Another language outright: this is what wrongLang is for.
	wrong := v1Sentence("It is a hobby of mine to have an exact knowledge of London.",
		tbook.Translation{Text: "我的一个爱好是对伦敦了如指掌。", Align: []tbook.AlignChunk{}})
	judged := v1Sentence("She opened the window.", alignedTr("Она открыла окно.", 4))
	// The align pass produced nothing: translated, shipped without links.
	unaligned := v1Sentence("A bird sang.", tbook.Translation{Text: "Птица пела.", Align: []tbook.AlignChunk{}})

	sents := []*tbook.Sentence{clean, copied, wrong, judged, unaligned}
	book := &tbook.Book{
		Title: "Fixture", Author: "Nobody", Source: "en", Targets: []string{"ru"},
		Chapters: []tbook.Chapter{{Title: "One", Paragraphs: [][]*tbook.Sentence{
			{clean}, {copied}, {wrong}, {judged}, {unaligned},
		}}},
	}

	cfg := &config.Config{
		Source: "en", Targets: []string{"ru"}, Out: out, Format: config.FormatV2,
		CacheDir: cacheDir, Judge: true, JudgeModel: "judge-model",
		JudgeScope: config.JudgeScopeAll,
	}
	verdict := translate.Verdict{OK: false, Why: "mistranslation: wrong subject"}
	if err := translate.SaveVerdict(cacheDir, cfg.JudgeModel, "en", "ru", judged, verdict); err != nil {
		t.Fatalf("save verdict: %v", err)
	}

	src := &v2.Source{Kind: "epub", SHA256: strings.Repeat("ab", 32),
		Identifier: "urn:uuid:fixture", Title: "Fixture", Language: "en-GB"}
	run := tbook.RunRecord{At: time.Now().UTC().Format(time.RFC3339),
		Targets: []string{"ru"}, Options: map[string]any{"tokenizer": v2.TokenizerID}}

	g := newGateState(cfg, sents, []string{"ru"}, nil, nil)
	if got := g.gatesFor("ru"); strings.Join(got, ",") != "langcheck,judge" {
		t.Fatalf("gates = %v, want [langcheck judge]", got)
	}
	if err := writeV2(cfg, book, src, run, g); err != nil {
		t.Fatalf("writeV2: %v", err)
	}

	f, err := v2.Open(out)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	if f.Manifest.Source == nil || f.Manifest.Source.Identifier != "urn:uuid:fixture" ||
		f.Manifest.Source.SHA256 != src.SHA256 || f.Manifest.Source.Language != "en-GB" ||
		f.Manifest.Source.Kind != "epub" {
		t.Errorf("manifest.source = %+v", f.Manifest.Source)
	}
	if f.Manifest.Source.TextSHA256 != "" || len(f.Manifest.Source.Docs) != 0 {
		t.Errorf("textSha256/docs must be omitted, got %+v", f.Manifest.Source)
	}
	if f.Manifest.Meta == nil || len(f.Manifest.Meta.Runs) == 0 {
		t.Fatal("no run record")
	}
	last := f.Manifest.Meta.Runs[len(f.Manifest.Meta.Runs)-1]
	if last.Options["tokenizer"] != v2.TokenizerID {
		t.Errorf("run options tokenizer = %v, want %s", last.Options["tokenizer"], v2.TokenizerID)
	}

	ov, ok, err := f.Overlay(0, "ru")
	if err != nil || !ok {
		t.Fatalf("overlay: %v (found %v)", err, ok)
	}
	if strings.Join(ov.Gates, ",") != "langcheck,judge" {
		t.Errorf("overlay gates = %v", ov.Gates)
	}
	if len(ov.Paragraphs) != 5 {
		t.Fatalf("%d overlay paragraphs, want 5", len(ov.Paragraphs))
	}
	cell := func(i int) *v2.Translation {
		if len(ov.Paragraphs[i].S) != 1 {
			t.Fatalf("paragraph %d has %d translations", i, len(ov.Paragraphs[i].S))
		}
		return ov.Paragraphs[i].S[0]
	}

	if c := cell(0); c.S != "" || c.V != nil {
		t.Errorf("clean cell: s=%q v=%+v, want neither", c.S, c.V)
	}
	c := cell(1)
	if c.S != v2.StatusUnverified {
		t.Errorf("copied-through cell: s=%q, want %q", c.S, v2.StatusUnverified)
	}
	if c.Lang != "" {
		t.Errorf("copied-through cell: lang=%q, want none — the kind names no language", c.Lang)
	}
	if c.V == nil || c.V.OK || c.V.By != v2.GateLangcheck || c.V.Why != "untranslated" {
		t.Errorf("copied-through cell: v=%+v", c.V)
	}
	c = cell(2)
	if c.S != v2.StatusWrongLang {
		t.Errorf("wrong-language cell: s=%q, want %q", c.S, v2.StatusWrongLang)
	}
	if c.Lang != "zh" {
		t.Errorf("wrong-language cell: lang=%q, want zh", c.Lang)
	}
	if c.V == nil || c.V.OK || c.V.By != v2.GateLangcheck || c.V.Why != "foreign-script" {
		t.Errorf("wrong-language cell: v=%+v", c.V)
	}
	c = cell(3)
	if c.S != v2.StatusUnverified {
		t.Errorf("judged cell: s=%q, want %q", c.S, v2.StatusUnverified)
	}
	if c.V == nil || c.V.OK || c.V.By != v2.GateJudge || !strings.Contains(c.V.Why, "mistranslation") {
		t.Errorf("judged cell: v=%+v", c.V)
	}
	c = cell(4)
	if c.S != v2.StatusRaw {
		t.Errorf("unaligned cell: s=%q, want %q", c.S, v2.StatusRaw)
	}
	if len(c.A) != 0 || c.V != nil {
		t.Errorf("unaligned cell: a=%v v=%+v", c.A, c.V)
	}

	// writeV2 validates before reporting success; assert it independently too.
	rep, err := v2.Validate(out)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !rep.OK() {
		t.Fatalf("invalid archive: %v", rep.Violations)
	}

	// The version-1 reader must refuse the file rather than half-read it.
	if _, err := tbook.Read(out); err == nil || !strings.Contains(err.Error(), "formatVersion") {
		t.Errorf("tbook.Read on a v2 file: err = %v, want a formatVersion refusal", err)
	}
}

// TestGateStateScopedJudge: a judge that saw only a scoped subset must not claim
// the registered `judge` gate, or an absent verdict would read as "passed".
func TestGateStateScopedJudge(t *testing.T) {
	cfg := &config.Config{Source: "en", Targets: []string{"ru"}, CacheDir: t.TempDir(),
		Judge: true, JudgeModel: "m", JudgeScope: config.JudgeScopeFlagged}
	g := newGateState(cfg, nil, []string{"ru"}, nil, nil)
	if got := g.gatesFor("ru"); strings.Join(got, ",") != "langcheck,"+gateJudgeScoped {
		t.Errorf("gates = %v, want [langcheck %s]", got, gateJudgeScoped)
	}
}

// TestGateStateCarriedOverLanguage: a language this run did not translate was
// not judged, so its overlay claims no judge gate and its missing links are
// `unaligned` (cause unrecorded), never `raw`.
func TestGateStateCarriedOverLanguage(t *testing.T) {
	cfg := &config.Config{Source: "en", Targets: []string{"ru"}, CacheDir: t.TempDir(),
		Judge: true, JudgeModel: "m", JudgeScope: config.JudgeScopeAll}
	g := newGateState(cfg, nil, []string{"ru", "de"}, nil, nil)
	if got := g.gatesFor("de"); strings.Join(got, ",") != "langcheck" {
		t.Errorf("de gates = %v, want [langcheck]", got)
	}
	s := v1Sentence("A bird sang.", tbook.Translation{Text: "Ein Vogel sang.", Align: []tbook.AlignChunk{}})
	s.Tr["de"] = s.Tr["ru"]
	tr := &v2.Translation{T: "Ein Vogel sang."}
	g.annotate(s, "de", tr, false)
	if tr.S != v2.StatusUnaligned {
		t.Errorf("carried-over cell: s=%q, want %q", tr.S, v2.StatusUnaligned)
	}
}

func TestSourceIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.epub")
	if err := os.WriteFile(path, []byte("not really an epub"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := sourceIdentity(path, &epub.Book{Title: "T", Language: "fr", Identifier: "id:1"})
	if got.Kind != "epub" || got.Title != "T" || got.Language != "fr" || got.Identifier != "id:1" {
		t.Errorf("source = %+v", got)
	}
	if len(got.SHA256) != 64 {
		t.Errorf("sha256 = %q", got.SHA256)
	}
	if k := sourceKind("/tmp/x.FB2.zip"); k != "fb2" {
		t.Errorf("sourceKind(.fb2.zip) = %q", k)
	}
}
