package main

import (
	"testing"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/tbook"
)

const pendingModel = "test-model+g:deadbeef"

func pendingSentences() []*tbook.Sentence {
	return []*tbook.Sentence{
		{Src: "The dig was in disarray."},
		{Src: "She wore a greatcoat."},
		{Src: "Nine hundred thousand years had passed since the Event."},
	}
}

func pendingCfg(dir string, repair bool, ctxN int) *config.Config {
	return &config.Config{
		Source: "en", Targets: []string{"ru"}, CacheDir: dir,
		Repair: repair, RepairContext: ctxN, AlignMode: config.AlignHybrid,
	}
}

// seedRaw caches a pass-1 translation under the given text namespace.
func seedRaw(t *testing.T, dir, model string, sentences []*tbook.Sentence) {
	t.Helper()
	for _, s := range sentences {
		if err := cache.Write(dir, cache.TrKey(s.Src, "en", "ru", model),
			tbook.Translation{Text: "перевод: " + s.Src}); err != nil {
			t.Fatalf("seed raw: %v", err)
		}
	}
}

// seedProofread caches a pass-1.5 proofread text under the given namespace.
func seedProofread(t *testing.T, dir, model string, sentences []*tbook.Sentence) {
	t.Helper()
	for _, s := range sentences {
		if err := cache.Write(dir, cache.RepairKey(s.Src, "en", "ru", model),
			tbook.Translation{Text: "вычитано: " + s.Src}); err != nil {
			t.Fatalf("seed proofread: %v", err)
		}
	}
}

// seedFinal caches an aligned entry under the given final namespace.
func seedFinal(t *testing.T, dir, model string, sentences []*tbook.Sentence) {
	t.Helper()
	for _, s := range sentences {
		if err := cache.Write(dir, cache.Key(s.Src, "en", "ru", model),
			tbook.Translation{Text: "перевод: " + s.Src}); err != nil {
			t.Fatalf("seed final: %v", err)
		}
	}
}

// A book translated WITHOUT the proofread pass and re-run WITH it must report
// the pass as pending work. Counting in the raw namespace instead of the one
// assembly reads from made convert announce "All sentences already cached —
// assembling offline", write a book whose every sentence was empty, and exit 0.
func TestPendingSeesProofreadWorkOverAnUnrepairedCache(t *testing.T) {
	dir := t.TempDir()
	sentences := pendingSentences()
	seedRaw(t, dir, pendingModel, sentences)
	seedFinal(t, dir, pendingModel, sentences)

	plain := pendingCfg(dir, false, 0)
	if got := pendingFinal(plain, sentences, pendingModel); got != 0 {
		t.Errorf("without --repair the cache is complete: pendingFinal = %d, want 0", got)
	}

	rep := pendingCfg(dir, true, 0)
	if got := pendingFinal(rep, sentences, pendingModel); got != len(sentences) {
		t.Errorf("with --repair every sentence still needs the pass: pendingFinal = %d, want %d",
			got, len(sentences))
	}
	if got := pendingRaw(rep, sentences, pendingModel); got != 0 {
		t.Errorf("raw translations are cached: pendingRaw = %d, want 0", got)
	}
	if got := pendingRepair(rep, sentences, pendingModel); got != len(sentences) {
		t.Errorf("no proofread text cached yet: pendingRepair = %d, want %d", got, len(sentences))
	}
	// align-mode emb aligns locally, but the proofread pass still needs the LLM.
	if got := pendingText(rep, sentences, pendingModel); got != len(sentences) {
		t.Errorf("pendingText = %d, want %d", got, len(sentences))
	}
	if got := pendingPhaseVerb(rep, sentences, pendingModel); got != "Proofreading" {
		t.Errorf("phase verb = %q, want \"Proofreading\"", got)
	}
}

// Once the proofread text and the alignment of it are cached, a repeat run is
// fully offline again — and a --context run keeps its own namespace, so it does
// not serve alignments built from the context-free variant.
func TestPendingAfterProofreadIsOfflineAndContextScoped(t *testing.T) {
	dir := t.TempDir()
	sentences := pendingSentences()
	rep := pendingCfg(dir, true, 0)
	seedRaw(t, dir, pendingModel, sentences)
	seedProofread(t, dir, repairModel(rep, pendingModel), sentences)
	seedFinal(t, dir, finalModel(rep, pendingModel), sentences)

	if got := pendingFinal(rep, sentences, pendingModel); got != 0 {
		t.Errorf("a repaired cache re-assembles offline: pendingFinal = %d, want 0", got)
	}
	if got := pendingText(rep, sentences, pendingModel); got != 0 {
		t.Errorf("no text work left: pendingText = %d, want 0", got)
	}

	ctx2 := pendingCfg(dir, true, 2)
	if got := pendingFinal(ctx2, sentences, pendingModel); got != len(sentences) {
		t.Errorf("--context 2 is its own namespace: pendingFinal = %d, want %d",
			got, len(sentences))
	}
}

// With the alignment missing but every text pass cached, the announced phase is
// the align pass — not "Translating".
func TestPendingPhaseVerbAligning(t *testing.T) {
	dir := t.TempDir()
	sentences := pendingSentences()
	rep := pendingCfg(dir, true, 0)
	seedRaw(t, dir, pendingModel, sentences)
	seedProofread(t, dir, repairModel(rep, pendingModel), sentences)

	if got := pendingPhaseVerb(rep, sentences, pendingModel); got != "Aligning" {
		t.Errorf("phase verb = %q, want \"Aligning\"", got)
	}
	plain := pendingCfg(dir, false, 0)
	if got := pendingPhaseVerb(plain, sentences, pendingModel); got != "Aligning" {
		t.Errorf("phase verb without repair = %q, want \"Aligning\"", got)
	}
}
