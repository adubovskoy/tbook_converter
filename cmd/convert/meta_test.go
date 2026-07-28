package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/translate"
)

func fullConfig() *config.Config {
	return &config.Config{
		Input: "/home/someone/books/secret-manuscript.epub",
		Out:   "/home/someone/books/secret-manuscript.tbook",

		Provider:  config.ProviderOpenRouter,
		ClaudeBin: "/opt/homebrew/bin/claude",
		APIKey:    "sk-or-v1-DEADBEEFdeadbeef",
		BaseURL:   "https://llm.internal.example:8443/v1",
		Model:     "google/gemini-2.5-flash",
		Referer:   "https://tbook.dev",

		Source: "en", Targets: []string{"ru", "de"},

		BatchSize: 24, AlignBatch: 6, Concurrency: 8, Temperature: 0.2,
		Timeout: time.Minute, JSONMode: true,

		CacheDir: "/home/someone/.cache/tbook",

		Glossary: true, Repair: true,
		Judge: true, JudgeModel: "anthropic/claude-opus-5", JudgeScope: config.JudgeScopeFlagged,
		EscalateModel: "anthropic/claude-sonnet-5",
		Lexcheck:      true, LexiconDir: "/home/someone/lexicons",

		StatsPath: "/home/someone/stats.jsonl", ProgressFile: "/tmp/progress.ndjson",

		AlignMode: config.AlignHybrid,
		EmbPython: "/home/someone/.venv-embalign/bin/python",
		EmbScript: "/home/someone/tools/embalign.py",
		EmbModel:  "sentence-transformers/LaBSE",
		EmbMethod: "argmax", EmbQMin: 0.6,

		SkipCitations: true, LimitChapters: 3,
	}
}

func TestMetaRunRecordsProvenance(t *testing.T) {
	at := time.Date(2026, 7, 28, 9, 11, 4, 0, time.UTC)
	r := metaRun(fullConfig(), []string{"ru", "de"}, 42, at)

	if r.At != "2026-07-28T09:11:04Z" {
		t.Fatalf("at = %q, want an RFC 3339 UTC stamp", r.At)
	}
	if r.Provider != config.ProviderOpenRouter {
		t.Fatalf("provider = %q", r.Provider)
	}
	if r.Producer == nil || r.Producer.Name != "tbook-converter" || r.Producer.Version == "" {
		t.Fatalf("producer = %+v, want name + version", r.Producer)
	}
	if got := strings.Join(r.Targets, ","); got != "ru,de" {
		t.Fatalf("targets = %q", got)
	}
	wantModels := map[string]string{
		"translate": "google/gemini-2.5-flash",
		"align":     "google/gemini-2.5-flash",
		"repair":    "google/gemini-2.5-flash",
		"judge":     "anthropic/claude-opus-5",
		"escalate":  "anthropic/claude-sonnet-5",
		"embed":     "sentence-transformers/LaBSE",
	}
	for pass, want := range wantModels {
		if r.Models[pass] != want {
			t.Errorf("models[%q] = %q, want %q", pass, r.Models[pass], want)
		}
	}
	wantPrompts := map[string]string{
		"translate": cache.TrPromptVersion,
		"align":     cache.PromptVersion,
		"repair":    cache.RepairPromptVersion,
		"judge":     translate.JudgeVersion,
	}
	for pass, want := range wantPrompts {
		if r.Prompts[pass] != want {
			t.Errorf("prompts[%q] = %q, want %q", pass, r.Prompts[pass], want)
		}
	}
	for key, want := range map[string]any{
		"alignMode": config.AlignHybrid, "batchSize": 24, "concurrency": 8,
		"glossary": true, "glossaryTerms": 42, "judge": true, "judgeScope": config.JudgeScopeFlagged,
		"embMethod": "argmax", "skipCitations": true, "limitChapters": 3,
	} {
		if r.Options[key] != want {
			t.Errorf("options[%q] = %v, want %v", key, r.Options[key], want)
		}
	}
}

// Spec §3.4: no credentials, and no paths/hosts identifying the machine that
// ran the conversion — a .tbook is a file people share.
func TestMetaRunLeaksNoSecretsOrPaths(t *testing.T) {
	cfg := fullConfig()
	b, err := json.Marshal(metaRun(cfg, cfg.Targets, 42, time.Now()))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, secret := range []string{
		cfg.APIKey, cfg.BaseURL, cfg.Input, cfg.Out, cfg.CacheDir, cfg.LexiconDir,
		cfg.EmbPython, cfg.EmbScript, cfg.ClaudeBin, cfg.StatsPath, cfg.ProgressFile,
		"/home/someone", "sk-or-",
	} {
		if strings.Contains(got, secret) {
			t.Errorf("run record leaks %q:\n%s", secret, got)
		}
	}
}

// The align mode decides which passes exist: emb aligns locally (no LLM align
// contract), llm uses no encoder.
func TestMetaRunAlignModes(t *testing.T) {
	cfg := fullConfig()
	cfg.AlignMode = config.AlignEmb
	r := metaRun(cfg, cfg.Targets, 0, time.Now())
	if _, ok := r.Models["align"]; ok {
		t.Errorf("emb mode should record no LLM align model: %v", r.Models)
	}
	if _, ok := r.Prompts["align"]; ok {
		t.Errorf("emb mode should record no align prompt version: %v", r.Prompts)
	}
	if r.Models["embed"] != "sentence-transformers/LaBSE" {
		t.Errorf("emb mode should record the encoder: %v", r.Models)
	}

	cfg.AlignMode = config.AlignLLM
	r = metaRun(cfg, cfg.Targets, 0, time.Now())
	if r.Models["align"] == "" || r.Prompts["align"] == "" {
		t.Errorf("llm mode should record the align model + prompt: %v / %v", r.Models, r.Prompts)
	}
	if _, ok := r.Models["embed"]; ok {
		t.Errorf("llm mode should record no encoder: %v", r.Models)
	}
	if _, ok := r.Options["embMethod"]; ok {
		t.Errorf("llm mode should record no embMethod: %v", r.Options)
	}
}

// Quality passes that did not run must not appear at all — an absent judge is
// not the same as a judge that approved.
func TestMetaRunOmitsPassesThatDidNotRun(t *testing.T) {
	cfg := fullConfig()
	cfg.Repair, cfg.Judge, cfg.EscalateModel, cfg.Glossary = false, false, "", false
	r := metaRun(cfg, cfg.Targets, 0, time.Now())
	for _, pass := range []string{"repair", "judge", "escalate"} {
		if _, ok := r.Models[pass]; ok {
			t.Errorf("models carries %q though the pass was off: %v", pass, r.Models)
		}
		if _, ok := r.Prompts[pass]; ok {
			t.Errorf("prompts carries %q though the pass was off: %v", pass, r.Prompts)
		}
	}
	if _, ok := r.Options["glossaryTerms"]; ok {
		t.Errorf("glossaryTerms recorded with no glossary: %v", r.Options)
	}
	if _, ok := r.Options["judgeScope"]; ok {
		t.Errorf("judgeScope recorded with no judge pass: %v", r.Options)
	}
}
