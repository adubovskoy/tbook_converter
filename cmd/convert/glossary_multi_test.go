package main

import (
	"path/filepath"
	"testing"

	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/translate"
)

// A glossary lists source→TARGET terms, so a multi-target run needs one per
// language. Enforcing the first target's list on the others tells the model,
// term by term, to write in the wrong language: a book converted that way
// carried Russian names — and often whole Russian sentences — in 50–73% of its
// six Latin targets, while validation, the alignment q-score and lexcheck all
// reported it fine (bench-quality/reports/four-model-bench-2026-08.md §8).
func TestResolveGlossariesIsPerTarget(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "book.tbook")
	cfg := &config.Config{
		Source: "en", Targets: []string{"ru", "de", "fr"},
		Out: out, Model: "test-model", Glossary: true,
	}

	got := resolveGlossaries(cfg, "Revelation Space", "Alastair Reynolds", 1575)
	if len(got) != 3 {
		t.Fatalf("resolved %d namespaces, want one per target", len(got))
	}
	for i, target := range cfg.Targets {
		g := got[i]
		if g.target != target {
			t.Errorf("namespace %d is for %q, want %q", i, g.target, target)
		}
		if g.scope.Target != target {
			t.Errorf("%s: sidecar scope target = %q", target, g.scope.Target)
		}
		want := out + ".glossary.en-" + target + ".json"
		if g.path != want {
			t.Errorf("%s: sidecar path = %q, want %q", target, g.path, want)
		}
		if !g.needsBuild {
			t.Errorf("%s: no sidecar exists yet, so the glossary pass must run", target)
		}
	}
}

// A sidecar supplies one target's glossary without touching the others, and the
// cache namespace it implies belongs to that target alone — so re-running with
// an edited de glossary never re-translates ru, and vice versa.
func TestResolveGlossariesLoadsEachSidecarSeparately(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "book.tbook")
	cfg := &config.Config{
		Source: "en", Targets: []string{"ru", "de"},
		Out: out, Model: "test-model", Glossary: true,
	}
	scope := translate.GlossaryScope{
		Source: "en", Target: "ru", Title: "Oz", Author: "Baum", Sentences: 2143,
	}
	entries := []translate.GlossEntry{{Src: "Scarecrow", Tgt: "Страшила"}}
	if err := translate.WriteGlossaryFile(translate.GlossaryFilePath(out, scope), scope, entries); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	got := resolveGlossaries(cfg, "Oz", "Baum", 2143)
	ru, de := got[0], got[1]
	if ru.needsBuild || len(ru.entries) != 1 || ru.entries[0].Tgt != "Страшила" {
		t.Errorf("ru should come from the sidecar: %+v", ru)
	}
	if !de.needsBuild || len(de.entries) != 0 {
		t.Errorf("de has no sidecar and must be built, not inherited from ru: %+v", de)
	}
	if ru.cacheModel == de.cacheModel {
		t.Errorf("both targets share the cache namespace %q — an edit to one would "+
			"invalidate the other", ru.cacheModel)
	}
	if want := translate.CacheKeyModel(cfg.Model, translate.GlossHash(entries)); ru.cacheModel != want {
		t.Errorf("ru namespace = %q, want %q", ru.cacheModel, want)
	}
	if de.cacheModel != cfg.Model {
		t.Errorf("de namespace = %q, want the plain model until its glossary is built", de.cacheModel)
	}
}

// --no-glossary leaves every target on the plain model namespace and asks for no
// glossary pass.
func TestResolveGlossariesWithoutGlossary(t *testing.T) {
	cfg := &config.Config{
		Source: "en", Targets: []string{"ru", "de"},
		Out: filepath.Join(t.TempDir(), "book.tbook"), Model: "test-model",
	}
	for _, g := range resolveGlossaries(cfg, "Oz", "Baum", 10) {
		if g.needsBuild || len(g.entries) != 0 || g.cacheModel != cfg.Model {
			t.Errorf("%s: unexpected namespace %+v", g.target, g)
		}
	}
}
