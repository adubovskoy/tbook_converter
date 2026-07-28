package main

import (
	"time"

	"github.com/dimando/reader/converter/internal/buildinfo"
	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
)

// metaRun builds this run's provenance record for manifest.meta (spec §3.4):
// which binary produced the file, on which backend, with which per-pass models
// and prompt-contract versions, and under which settings — enough to answer
// "what made this translation, and how do I reproduce it?" months later.
//
// Called at assembly time, so it records what the run actually DID, not what
// was requested: cfg carries the align mode after a possible fallback to llm and
// the model id adopted from a llama.cpp server.
//
// Options is a curated allowlist, not a config dump. A .tbook is a shared file,
// and §3.4 forbids credentials and machine identity in meta — so no API keys, no
// base URLs, no input/output/cache/lexicon paths, no interpreter locations.
func metaRun(cfg *config.Config, targets []string, glossTerms int, at time.Time) tbook.RunRecord {
	r := tbook.RunRecord{
		At: at.UTC().Format(time.RFC3339),
		Producer: &tbook.Producer{
			Name:    buildinfo.Name,
			Version: buildinfo.Version,
			Commit:  buildinfo.Commit(),
			URL:     buildinfo.URL,
		},
		Targets:  targets,
		Provider: cfg.Provider,
		Models:   map[string]string{"translate": cfg.Model},
		Prompts:  map[string]string{"translate": cache.TrPromptVersion},
	}
	// The align pass: the LLM contract unless alignment is purely local, plus
	// the encoder whenever the local aligner is in play (hybrid uses both).
	if cfg.AlignMode != config.AlignEmb {
		r.Models["align"] = cfg.Model
		r.Prompts["align"] = cache.PromptVersion
	}
	if cfg.AlignMode != config.AlignLLM {
		r.Models["embed"] = cfg.EmbModel
	}
	if cfg.Repair {
		r.Models["repair"] = cfg.Model
		r.Prompts["repair"] = cache.RepairPromptVersion
	}
	if cfg.Judge {
		r.Models["judge"] = cfg.JudgeModel
		r.Prompts["judge"] = translate.JudgeVersion
	}
	if cfg.EscalateModel != "" {
		r.Models["escalate"] = cfg.EscalateModel
	}

	opts := map[string]any{
		"alignMode":   cfg.AlignMode,
		"batchSize":   cfg.BatchSize,
		"concurrency": cfg.Concurrency,
		"temperature": cfg.Temperature,
		"glossary":    cfg.Glossary,
		"repair":      cfg.Repair,
		"judge":       cfg.Judge,
		"lexcheck":    cfg.Lexcheck,
	}
	if cfg.Glossary {
		// The glossary is user-editable and reshapes every translation, so the
		// term count belongs in the provenance even though the file does not.
		opts["glossaryTerms"] = glossTerms
	}
	if cfg.AlignBatch > 0 {
		opts["alignBatch"] = cfg.AlignBatch
	}
	if cfg.AlignMode != config.AlignLLM {
		opts["embMethod"] = cfg.EmbMethod
		if cfg.EmbQMin > 0 {
			opts["embQMin"] = cfg.EmbQMin
		}
	}
	if cfg.Judge {
		opts["judgeScope"] = cfg.JudgeScope
	}
	if cfg.JSONMode {
		opts["jsonMode"] = true
	}
	if cfg.KeepMatter {
		opts["keepMatter"] = true
	}
	if cfg.NoImages {
		opts["noImages"] = true
	}
	if cfg.NoNotes {
		opts["noNotes"] = true
	}
	if cfg.SkipCitations {
		opts["skipCitations"] = true
	}
	if cfg.LimitChapters > 0 {
		opts["limitChapters"] = cfg.LimitChapters
	}
	r.Options = opts
	return r
}
