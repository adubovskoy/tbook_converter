package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/langcheck"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
)

// langGateAttempts is how many times a flagged cell is re-translated before the
// run gives up on it. Two: the first attempt removes the glossary entries that
// teach the leak and adds the language reminder, which is the whole hypothesis;
// a third attempt of the same prompt has nothing new to offer.
const langGateAttempts = 2

// runLangGate turns the wrong-language check into a gate.
//
// langcheck's three signals are free, offline and the only ones that see a
// fluent answer in the wrong language: the alignment scores how the words line
// up with whatever text arrived (16 Chinese sentences at q = 1.000), the
// archive validates, and lexcheck reports a leaked sentence as positional
// drift when it sees it at all. Until now it was a report — the shipped flow
// was to re-run the whole conversion with --invalidate — so a first run always
// shipped the leak.
//
// Here the flagged cells are re-translated in place, with the glossary entries
// whose renderings are in a foreign script removed from that batch's prompt
// (they are what teaches the leak: an en→ru glossary enforced on Latin targets
// put Russian into 55-73% of two overlays) and the target language named once
// in the system prompt. What still fails after langGateAttempts ships
// untranslated in version 1 — an empty cell is honest, a fluent Chinese
// sentence beside an English one is not — and keeps its text in version 2,
// where §6.7 has a status for exactly this (`s: "wrongLang"`, written by
// newGateState at assembly) and a reader can badge it.
func runLangGate(ctx context.Context, cfg *config.Config, glossaries []*glossaryNS,
	sentences []*tbook.Sentence, pipe *translate.Pipeline) error {

	if !cfg.LangcheckGate {
		return nil
	}
	first := langGateFlags(cfg, sentences)
	if len(first) == 0 {
		return nil
	}
	fmt.Printf("Language gate: %d flagged cell(s) — re-translating (up to %d attempts)\n",
		countFlags(first), langGateAttempts)

	spent := 0
	flags := first
	for attempt := 0; attempt < langGateAttempts && len(flags) > 0; attempt++ {
		n, err := langGateRetranslate(ctx, cfg, glossaries, sentences, flags, pipe)
		if err != nil {
			return err
		}
		spent += n
		flags = langGateFlags(cfg, sentences)
	}

	fixed := countFlags(first) - countFlags(flags)
	fmt.Printf("Language gate: %d fixed, %d still flagged; %d re-translations spent\n",
		fixed, countFlags(flags), spent)
	if len(flags) == 0 {
		return nil
	}
	for _, sum := range langcheck.Summarize(flatten(flags), cfg.Targets) {
		fmt.Printf("Language gate [%s]: %d residual of %d sentences (%.2f%%)\n",
			sum.Target, sum.Total, len(sentences), 100*float64(sum.Total)/float64(max(1, len(sentences))))
	}
	// Only the two kinds with measured precision act on the cell (see
	// langcheck.Gating). An untranslated-only flag is reported and nothing more:
	// deleting a correct translation is worse than shipping a suspect one.
	reported := 0
	for _, byTarget := range flags {
		for _, kind := range byTarget {
			if !langcheck.Gating(kind) {
				reported++
			}
		}
	}
	if reported > 0 {
		fmt.Printf("Language gate: %d of those are %s only — reported, not acted on\n",
			reported, langcheck.KindUntranslated)
	}
	if cfg.Format == config.FormatV2 {
		fmt.Println("Language gate: residual cells keep their text; the assembly badges them (spec §6.7).")
		return nil
	}
	// Version 1 has no status slot, so the only honest shape for a cell that IS
	// in the wrong language is no translation at all.
	dropped := 0
	for _, s := range sentences {
		for target, byTarget := range flags {
			if kind, ok := byTarget[s.Src]; ok && langcheck.Gating(kind) {
				s.Tr[target] = tbook.Translation{Text: "", Align: []tbook.AlignChunk{}}
				dropped++
			}
		}
	}
	fmt.Printf("Language gate: %d cell(s) ship untranslated (version 1 has no status slot)\n", dropped)
	return nil
}

// langGateFlags is target → flagged source sentence → kind, from all three
// signals over the text as it currently stands in memory. A sentence can draw
// two flags for one target (a foreign script AND a duplicate); the kind that
// acts on the cell wins, so the record never understates it.
func langGateFlags(cfg *config.Config, sentences []*tbook.Sentence) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, f := range langcheck.Check(sentences, cfg.Source, cfg.Targets, langcheck.Options{}) {
		if out[f.Target] == nil {
			out[f.Target] = map[string]string{}
		}
		if prev, seen := out[f.Target][f.Src]; seen && langcheck.Gating(prev) {
			continue
		}
		out[f.Target][f.Src] = f.Kind
	}
	return out
}

func countFlags(flags map[string]map[string]string) int {
	n := 0
	for _, byTarget := range flags {
		n += len(byTarget)
	}
	return n
}

// flatten turns the gate's map back into flags for langcheck.Summarize; only
// the target and kind are read, so the other fields stay empty.
func flatten(flags map[string]map[string]string) []langcheck.Flag {
	var out []langcheck.Flag
	for target, byTarget := range flags {
		for src, kind := range byTarget {
			out = append(out, langcheck.Flag{Target: target, Src: src, Kind: kind})
		}
	}
	return out
}

// langGateRetranslate redoes the flagged cells of every target and refreshes
// them from the cache. Returns how many (sentence, language) pairs it spent.
func langGateRetranslate(ctx context.Context, cfg *config.Config, glossaries []*glossaryNS,
	sentences []*tbook.Sentence, flags map[string]map[string]string, pipe *translate.Pipeline) (int, error) {

	if pipe == nil {
		// A fully-cached run assembles offline and never built one; the gate is
		// the first thing to need the model, so build it here with the same
		// settings (no local aligner: the LLM align pass is its fallback).
		pipe = &translate.Pipeline{
			Client:   translate.NewClient(clientOptions(cfg, cfg.Model, cfg.Temperature)),
			CacheDir: cfg.CacheDir, Source: cfg.Source,
			BatchSize: cfg.BatchSize, AlignBatch: cfg.AlignBatch,
			Concurrency: cfg.Concurrency,
			Repair:      cfg.Repair, RepairContext: cfg.RepairContext,
			AlignMode: config.AlignLLM, EmbQMin: cfg.EmbQMin,
		}
	}
	// The retry redoes the cell from scratch and names the target language from
	// the first attempt: the gonka gateway answers an identical request body
	// from cache, so an unchanged prompt would buy the same wrong answer back.
	force, nudge := pipe.Force, pipe.LangNudge
	gloss, ns := pipe.Glossary, pipe.CacheModel
	pipe.Force, pipe.LangNudge = true, true
	defer func() {
		pipe.Force, pipe.LangNudge = force, nudge
		pipe.Glossary, pipe.CacheModel = gloss, ns
	}()

	spent := 0
	for _, g := range glossaries {
		byTarget := flags[g.target]
		if len(byTarget) == 0 {
			continue
		}
		srcs := make([]string, 0, len(byTarget))
		for src := range byTarget {
			srcs = append(srcs, src)
		}
		sort.Strings(srcs)
		subset := subsetBySrc(sentences, srcs)
		if len(subset) == 0 {
			continue
		}
		// Clear the cell's translation, proofread text and alignment: the
		// pipeline's own Force flag would do the same, but clearing first also
		// drops a stale alignment of the text being replaced.
		cache.Invalidate(cfg.CacheDir, srcs, []string{g.target}, cfg.Source,
			g.cacheModel, finalModel(cfg, g.cacheModel), repairModel(cfg, g.cacheModel))

		clean := translate.DropForeignRenderings(g.entries, g.target)
		if n := len(g.entries) - len(clean); n > 0 {
			fmt.Printf("Language gate [%s]: %d glossary entr(ies) held back from the retry prompt\n",
				g.target, n)
		}
		// The retry keeps the run's cache namespace: holding an entry back from
		// one batch's prompt is not a different glossary, and a new namespace
		// would orphan every cell the gate did not touch.
		pipe.Glossary, pipe.CacheModel = clean, g.cacheModel
		if err := pipe.Run(ctx, subset, []string{g.target}); err != nil {
			return spent, err
		}
		translate.FillFromCache(subset, []string{g.target}, cfg.CacheDir, cfg.Source,
			finalModel(cfg, g.cacheModel), g.cacheModel, repairModel(cfg, g.cacheModel))
		spent += len(subset)
	}
	return spent, nil
}
