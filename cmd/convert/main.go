// Command convert turns an EPUB or FB2 into a .tbook language-learning
// archive, translating every sentence with word-level alignment via OpenRouter
// (metered) or the claude CLI (the user's Claude subscription).
//
//	convert book.epub -o book.tbook            # English → Russian (default)
//	convert book.epub --provider claude        # translate on the Claude subscription
//	convert book.fb2 -t ru,de -o book.tbook    # FB2 input, multiple targets
//	convert book.epub --dry-run                # parse + segment only, no API
//	convert book.epub --glossary --judge       # quality passes (see README)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/embalign"
	"github.com/dimando/reader/converter/internal/epub"
	"github.com/dimando/reader/converter/internal/fb2"
	"github.com/dimando/reader/converter/internal/lexcheck"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
)

// statsLog is the shared per-request JSONL sink (--stats); nil = disabled.
// Package-level so every pass's client (translate, judge, escalate) shares it.
var statsLog *translate.Stats

func main() {
	if err := run(); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return // usage already printed
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	runStart := time.Now()
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		return err
	}
	if cfg.StatsPath != "" {
		statsLog, err = translate.OpenStats(cfg.StatsPath)
		if err != nil {
			return fmt.Errorf("open --stats file: %w", err)
		}
		defer statsLog.Close()
		fmt.Printf("Per-request metrics → %s\n", cfg.StatsPath)
	}

	// Verify/QA loop: clear cached translation+alignment for flagged sentences,
	// then exit. A following run redoes exactly those (e.g. with a stronger model).
	if cfg.Invalidate != "" {
		srcs, err := loadSrcList(cfg.Invalidate)
		if err != nil {
			return fmt.Errorf("read --invalidate file: %w", err)
		}
		if err := ensureLlamaCppModel(cfg); err != nil {
			return err
		}
		n := cache.Invalidate(cfg.CacheDir, srcs, cfg.Targets, cfg.Source, cfg.Model)
		fmt.Printf("Invalidated %d cache file(s) for %d source sentence(s) — re-run to redo them.\n",
			n, len(srcs))
		return nil
	}

	// Load the source book (no API). Two inputs are accepted:
	//   • an .epub  — parsed + segmented (the fresh-conversion path);
	//   • a .tbook — read back so a new target language can be added without the
	//     source EPUB. Existing translations are preserved verbatim; only the
	//     new target(s) in --target are translated and merged in.
	var (
		title, author string
		cover         []byte
		images        map[string][]byte
		notes         map[string]*tbook.Note
		outChapters   []tbook.Chapter
		sentences     []*tbook.Sentence // chapter prose + figure captions + table cells
		noteSents     []*tbook.Sentence
		citeSents     []*tbook.Sentence
		writeTargets  []string // final manifest targetLangs
	)

	if strings.HasSuffix(cfg.Input, ".tbook") {
		fmt.Printf("Reading %s ...\n", cfg.Input)
		rb, err := tbook.Read(cfg.Input)
		if err != nil {
			return fmt.Errorf("read tbook: %w", err)
		}
		cfg.Source = rb.Source // the archive is authoritative for the source language
		title, author = rb.Title, rb.Author
		cover, images, notes = rb.Cover, rb.Images, rb.Notes
		if cfg.LimitChapters > 0 && cfg.LimitChapters < len(rb.Chapters) {
			rb.Chapters = rb.Chapters[:cfg.LimitChapters]
		}
		outChapters = rb.Chapters
		sentences, noteSents, citeSents = rb.Sentences()
		writeTargets = mergeTargets(rb.Targets, cfg.Targets)
		fmt.Printf("  %q by %s — %d chapters, %d images, %d notes; have %v, adding %v\n",
			title, author, len(outChapters), len(images), len(notes), rb.Targets, cfg.Targets)
	} else {
		fmt.Printf("Parsing %s ...\n", cfg.Input)
		var book *epub.Book
		if lower := strings.ToLower(cfg.Input); strings.HasSuffix(lower, ".fb2") ||
			strings.HasSuffix(lower, ".fb2.zip") {
			book, err = fb2.Parse(cfg.Input)
			if err != nil {
				return fmt.Errorf("parse fb2: %w", err)
			}
		} else {
			book, err = epub.ParseOpts(cfg.Input, epub.Options{
				SkipMatter: !cfg.KeepMatter,
				SkipExtra:  cfg.SkipFiles,
				NoImages:   cfg.NoImages,
				NoNotes:    cfg.NoNotes,
			})
			if err != nil {
				return fmt.Errorf("parse epub: %w", err)
			}
		}
		chapters := book.Chapters
		if cfg.LimitChapters > 0 && cfg.LimitChapters < len(chapters) {
			chapters = chapters[:cfg.LimitChapters]
		}
		coverState := "no"
		if book.Cover != nil {
			coverState = "yes"
		}
		fmt.Printf("  %q by %s — %d chapters, cover: %s, %d images, %d notes\n",
			book.Title, book.Author, len(chapters), coverState, len(book.Images), len(book.Notes))

		outChapters, sentences = segment.BuildSentenceObjects(chapters, cfg.Source)
		notes, noteSents, citeSents = segment.BuildNotes(book.Notes, cfg.Source)
		title, author, cover, images = book.Title, book.Author, book.Cover, book.Images
		writeTargets = cfg.Targets
	}

	// Everything below translates `translatable`; citations join it unless
	// skipped (untranslated citations are legal — empty tr).
	translatable := append(append([]*tbook.Sentence{}, sentences...), noteSents...)
	if !cfg.SkipCitations {
		translatable = append(translatable, citeSents...)
	}
	allSents := append(append([]*tbook.Sentence{}, sentences...), noteSents...)
	allSents = append(allSents, citeSents...)

	words := 0
	for _, s := range sentences {
		words += len(s.Words)
	}
	fmt.Printf("  %d sentences (~%d words) + %d note sentences (%d in citations%s)\n",
		len(sentences), words, len(noteSents)+len(citeSents), len(citeSents),
		map[bool]string{true: ", skipped", false: ""}[cfg.SkipCitations])

	if cfg.DryRun {
		printDryRun(sentences, outChapters, notes)
		return nil
	}

	// Must precede any cache math: cache keys embed the model id.
	if err := ensureLlamaCppModel(cfg); err != nil {
		return err
	}

	cacheModel := cfg.Model
	var glossary []translate.GlossEntry

	// Only the untranslated work needs the LLM; a fully-cached run assembles
	// offline (resume / re-assemble) without a key or CLI.
	countPending := func() int {
		return translate.CountPending(translatable, cfg.Targets, cfg.CacheDir, cfg.Source, cacheModel, cfg.Force)
	}
	needAPI := countPending() > 0 || cfg.Glossary || cfg.Judge
	if cfg.AlignMode == config.AlignEmb && !cfg.Glossary && !cfg.Judge {
		// emb aligns locally: only the translate phase needs the LLM.
		needAPI = translate.CountPendingTranslate(translatable, cfg.Targets, cfg.CacheDir,
			cfg.Source, cacheModel, cfg.Force) > 0
	}
	var client *translate.Client
	if needAPI {
		switch cfg.Provider {
		case config.ProviderClaude:
			if _, err := exec.LookPath(cfg.ClaudeBin); err != nil {
				return fmt.Errorf("--provider claude needs the Claude Code CLI (%q not found in PATH); "+
					"install it or set CLAUDE_BIN", cfg.ClaudeBin)
			}
		case config.ProviderOllama:
			models := []string{cfg.Model}
			if cfg.Judge && cfg.JudgeModel != cfg.Model {
				models = append(models, cfg.JudgeModel)
			}
			if cfg.EscalateModel != "" && cfg.EscalateModel != cfg.Model {
				models = append(models, cfg.EscalateModel)
			}
			for _, m := range models {
				if err := translate.CheckOllama(cfg.BaseURL, m); err != nil {
					return err
				}
			}
		case config.ProviderLlamaCpp:
			served, err := translate.ServedModels(cfg.BaseURL, cfg.APIKey)
			if err != nil {
				return fmt.Errorf("can't reach the llama.cpp server at %s (%v) — is llama-server running?",
					cfg.BaseURL, err)
			}
			// llama-server ignores the requested id, so a mismatch cannot fail
			// a call — it would only mislabel the cache; say so and continue.
			if !translate.ServedBy(served, cfg.Model) {
				fmt.Printf("note: the llama.cpp server serves %s, not %q — requests use the served model; cache keys keep %q\n",
					strings.Join(served, ", "), cfg.Model, cfg.Model)
			}
		default:
			if cfg.APIKey == "" && countPending() > 0 {
				return fmt.Errorf("%d sentences need translating but OPENROUTER_API_KEY is not set "+
					"(put it in converter/.env — see .env.example — use --provider claude "+
					"to run on your Claude subscription, or --provider ollama for a local model)", countPending())
			}
		}
		client = translate.NewClient(clientOptions(cfg, cfg.Model, cfg.Temperature))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Glossary pass: build (or load) the book glossary and namespace the cache
	// with its hash — translations made under a different glossary never mix.
	if cfg.Glossary {
		var ghash string
		glossary, ghash, err = translate.BuildGlossary(ctx, client, cfg.CacheDir, sentences,
			cfg.Source, cfg.Targets[0], title, author)
		if err != nil {
			return fmt.Errorf("glossary: %w", err)
		}
		cacheModel = translate.CacheKeyModel(cfg.Model, ghash)
		fmt.Printf("Glossary: %d enforced terms (cache namespace %s)\n", len(glossary), cacheModel)
	}

	if pending := countPending(); pending > 0 {
		via := cfg.Model
		switch cfg.Provider {
		case config.ProviderClaude:
			via = "claude CLI (subscription) / " + cfg.Model
		case config.ProviderOllama:
			via = cfg.Model + " (ollama at " + cfg.BaseURL + ")"
		case config.ProviderLlamaCpp:
			via = cfg.Model + " (llama.cpp at " + cfg.BaseURL + ")"
		}
		fmt.Printf("Translating %s→%s via %s ...\n", cfg.Source, strings.Join(cfg.Targets, ","), via)
		pipe := &translate.Pipeline{
			Client: client, CacheDir: cfg.CacheDir, Source: cfg.Source,
			BatchSize: cfg.BatchSize, AlignBatch: cfg.AlignBatch,
			Concurrency: cfg.Concurrency,
			Force:       cfg.Force,
			Glossary:    glossary, CacheModel: cacheModel,
			AlignMode: cfg.AlignMode, EmbQMin: cfg.EmbQMin,
		}
		if cfg.AlignMode != config.AlignLLM {
			fmt.Printf("Starting embedding aligner (%s, %s) ...\n", cfg.EmbModel, cfg.EmbMethod)
			aligner, err := embalign.Start(embalign.Options{
				Python: cfg.EmbPython, Script: cfg.EmbScript,
				Model: cfg.EmbModel, Layer: cfg.EmbLayer, Method: cfg.EmbMethod,
			})
			switch {
			case err == nil:
				defer aligner.Close()
				pipe.EmbAligner = aligner
				pipe.LexDicts = lexDictLoader(cfg)
			case cfg.AlignModeExplicit:
				return err
			default:
				// The hybrid default must not break a machine without the local
				// aligner: degrade to the LLM align pass and say how to get the
				// free one back.
				fmt.Printf("Embedding aligner unavailable (%v)\n"+
					"  — falling back to --align-mode llm for this run; "+
					"run tools/embalign-setup.sh to enable the free local align pass.\n", err)
				cfg.AlignMode = config.AlignLLM
				pipe.AlignMode = config.AlignLLM
			}
		}
		t0 := time.Now()
		if err := pipe.Run(ctx, translatable, cfg.Targets); err != nil {
			return err
		}
		fmt.Printf("Pipeline (translate+align) wall time: %s\n", time.Since(t0).Round(time.Second))
	} else {
		fmt.Println("All sentences already cached — assembling offline (no API calls).")
	}

	// Fill from cache + assemble. Citation sentences are filled too (they may
	// be cached from earlier runs); missing ones stay empty.
	found, missing := translate.FillFromCache(allSents, cfg.Targets, cfg.CacheDir, cfg.Source, cacheModel)
	fmt.Printf("Filled %d translations from cache (%d missing).\n", found, missing)

	// Verification passes. Lexcheck is the free, offline gate: a bilingual
	// dictionary statically catches positional-drift signatures. The judge
	// (spec §10.4) is the semantic gate: an independent LLM catches wrong-word
	// mappings and mistranslations the dictionary cannot see. With
	// --escalate-model, everything flagged is immediately redone by the
	// stronger model — in the primary cache namespace, so the book stays one
	// cheap-model file with only the hard few percent escalated.
	flaggedSet := map[string]bool{}
	if cfg.Lexcheck {
		for _, src := range runLexcheck(cfg, translatable) {
			flaggedSet[src] = true
		}
	}
	if cfg.Judge {
		judgeInput := translatable
		var sample []string
		if cfg.JudgeScope == config.JudgeScopeFlagged {
			judgeInput, sample = judgeScopeFlagged(cfg, translatable, flaggedSet)
		}
		t0 := time.Now()
		flagged, err := runJudge(ctx, cfg, judgeInput)
		if err != nil {
			return err
		}
		fmt.Printf("Judge wall time: %s\n", time.Since(t0).Round(time.Second))
		for _, src := range flagged {
			flaggedSet[src] = true
		}
		reportCalibration(sample, flagged)
	}
	if cfg.EscalateModel != "" && len(flaggedSet) > 0 {
		flagged := make([]string, 0, len(flaggedSet))
		for src := range flaggedSet {
			flagged = append(flagged, src)
		}
		sort.Strings(flagged)
		t0 := time.Now()
		if err := escalate(ctx, cfg, glossary, cacheModel, translatable, flagged); err != nil {
			return err
		}
		// A stronger model is not a verified model: re-run the same gates over
		// the escalated output, give it one redo, and fall back for whatever
		// still fails — never ship an unvetted rewrite.
		if err := reverifyEscalated(ctx, cfg, glossary, cacheModel, translatable, flagged); err != nil {
			return err
		}
		fmt.Printf("Escalation wall time: %s\n", time.Since(t0).Round(time.Second))
	}

	fmt.Printf("Writing %s ...\n", cfg.Out)
	out := &tbook.Book{
		Title: title, Author: author,
		Source: cfg.Source, Targets: writeTargets,
		Cover: cover, Images: images, Notes: notes,
		Chapters: outChapters,
	}
	if err := tbook.Write(cfg.Out, out); err != nil {
		return fmt.Errorf("assemble: %w", err)
	}
	if fi, err := os.Stat(cfg.Out); err == nil {
		fmt.Printf("Done. %s (%d KB)\n", cfg.Out, fi.Size()/1024)
	}

	// Validate against the full manifest target set (existing + newly added).
	rep := tbook.Validate(out, writeTargets)
	fmt.Printf("Validation: %d sentences, %d empty, %d offset_errors, %d struct_errors — %s\n",
		rep.Sentences, rep.Empty, rep.OffsetErrors, rep.StructErrors, status(rep))
	unaligned := ""
	if rep.Unaligned > 0 {
		unaligned = fmt.Sprintf(" (%d sentences carry no alignment — raw fallback / skipped citations)", rep.Unaligned)
	}
	fmt.Printf("Coverage: %.0f%% of target words aligned, %.0f%% of source words highlighted%s\n",
		100*rep.TgtCoverage(), 100*rep.SrcCoverage(), unaligned)
	if rep.TgtCoverage() < tbook.CoverageWarn && rep.Empty == 0 {
		fmt.Printf("  WARNING: low alignment coverage (%.0f%% < %.0f%%) — alignment may be "+
			"positionally drifted or mostly empty. Note: coverage cannot detect a "+
			"partial drift or wrong-word mapping; run --judge to verify semantically.\n",
			100*rep.TgtCoverage(), 100*tbook.CoverageWarn)
	}
	if rep.OffsetErrors > 0 || rep.StructErrors > 0 {
		return fmt.Errorf("validation failed: %d offset errors, %d struct errors",
			rep.OffsetErrors, rep.StructErrors)
	}
	fmt.Printf("Total wall time: %s\n", time.Since(runStart).Round(time.Second))
	return nil
}

// judgeScopeFlagged selects the sentences worth judging when --judge-scope
// flagged: everything lexcheck flagged, everything with low alignment coverage
// (embalign Q below the hybrid threshold, or — for entries with no stored Q —
// locally computed chunk coverage; raw-fallback sentences with no alignment
// land here too), plus a deterministic 5% calibration sample of the rest so a
// judge-miss drift in the skipped population stays measurable. Returns the
// subset and the sample's sources.
func judgeScopeFlagged(cfg *config.Config, sentences []*tbook.Sentence, flaggedSet map[string]bool) ([]*tbook.Sentence, []string) {
	qmin := cfg.EmbQMin
	if qmin == 0 {
		qmin = translate.DefaultEmbQMin
	}
	suspect := map[string]bool{}
	var rest []*tbook.Sentence
	seen := map[string]bool{}
	for _, s := range sentences {
		if seen[s.Src] {
			continue
		}
		seen[s.Src] = true
		if flaggedSet[s.Src] {
			suspect[s.Src] = true
			continue
		}
		lowQ := false
		for _, target := range cfg.Targets {
			tr, ok := s.Tr[target]
			if !ok || tr.Text == "" {
				continue
			}
			q := tr.Q
			if q == 0 {
				q = localCoverage(tr)
			}
			if q < qmin {
				lowQ = true
				break
			}
		}
		if lowQ {
			suspect[s.Src] = true
		} else {
			rest = append(rest, s)
		}
	}
	// Deterministic sample: same book + flags → same sample across runs, so
	// cached verdicts stay valid.
	rng := rand.New(rand.NewSource(1))
	rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	nSample := (len(rest) + 19) / 20 // 5%, rounded up
	var sample []string
	for _, s := range rest[:min(nSample, len(rest))] {
		suspect[s.Src] = true
		sample = append(sample, s.Src)
	}
	subset := make([]string, 0, len(suspect))
	for src := range suspect {
		subset = append(subset, src)
	}
	fmt.Printf("Judge scope: %d suspect (lexcheck/low-coverage) + %d calibration sample of %d clean = %d of %d sentences\n",
		len(suspect)-len(sample), len(sample), len(rest), len(suspect), len(seen))
	return subsetBySrc(sentences, subset), sample
}

// reportCalibration prints the judge flag rate inside the random calibration
// sample — the estimate of what the scoped judge is missing in the skipped
// population. A rate well above the suspect set's residual means the scope
// filter is too narrow.
func reportCalibration(sample, flagged []string) {
	if len(sample) == 0 {
		return
	}
	fset := make(map[string]bool, len(flagged))
	for _, src := range flagged {
		fset[src] = true
	}
	n := 0
	for _, src := range sample {
		if fset[src] {
			n++
		}
	}
	fmt.Printf("Judge calibration sample: %d/%d flagged (%.1f%%) among sentences the scope filter considered clean\n",
		n, len(sample), 100*float64(n)/float64(len(sample)))
}

// localCoverage mirrors tbook's assembly-time coverage score for cache entries
// that carry no Q (LLM-aligned ones): the fraction of rendered translation
// words whose chunk maps to ≥1 source word. 0 when nothing is aligned.
func localCoverage(tr tbook.Translation) float64 {
	if tr.Text == "" || len(tr.Align) == 0 {
		return 0
	}
	runes := []rune(tr.Text)
	words, aligned := 0, 0
	for _, c := range tr.Align {
		a, b := c.T[0], c.T[1]
		if a < 0 {
			a = 0
		}
		if b > len(runes) {
			b = len(runes)
		}
		if a >= b {
			continue
		}
		hasLD := false
		for _, r := range runes[a:b] {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				hasLD = true
				break
			}
		}
		if !hasLD {
			continue
		}
		words++
		if len(c.W) > 0 {
			aligned++
		}
	}
	if words == 0 {
		return 0
	}
	return float64(aligned) / float64(words)
}

// ensureLlamaCppModel resolves an empty --model/LLAMACPP_MODEL against the
// llama.cpp server before any cache math: llama-server serves ONE model and
// ignores the requested id, but cache keys and logs need the real name.
func ensureLlamaCppModel(cfg *config.Config) error {
	if cfg.Provider != config.ProviderLlamaCpp || cfg.Model != "" {
		return nil
	}
	m, err := translate.AdoptServedModel(cfg.BaseURL, cfg.APIKey)
	if err != nil {
		return fmt.Errorf("resolving the llama.cpp model at %s: %w — start "+
			"`llama-server -hf <model>`, or set LLAMACPP_BASE_URL/LLAMACPP_MODEL", cfg.BaseURL, err)
	}
	cfg.Model = m
	if cfg.JudgeModel == "" { // the config-time default ran while Model was empty
		cfg.JudgeModel = m
	}
	fmt.Printf("llama.cpp serves %q — adopted as the model id (set LLAMACPP_MODEL to pin cache keys)\n", m)
	return nil
}

// clientOptions builds the LLM client options shared by every pass
// (translate, escalate, judge), including the backend selection. model and
// temperature vary per pass; everything else comes from the config.
func clientOptions(cfg *config.Config, model string, temperature float64) translate.Options {
	return translate.Options{
		Provider: cfg.Provider, ClaudeBin: cfg.ClaudeBin,
		BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: model,
		Referer: cfg.Referer, Title: cfg.Title,
		Temperature: temperature, JSONMode: cfg.JSONMode,
		MaxRetries: cfg.MaxRetries, Timeout: cfg.Timeout,
		ProviderSort: cfg.ProviderSort, ProviderOrder: cfg.ProviderOrder,
		Stats: statsLog,
	}
}

// lexDictLoader supplies per-target lexcheck dictionaries for the hybrid
// align gate, caching loads. A missing lexicon disables the lexcheck part of
// the gate for that target (the coverage threshold still applies).
func lexDictLoader(cfg *config.Config) func(target string) *lexcheck.Dict {
	dicts := map[string]*lexcheck.Dict{}
	return func(target string) *lexcheck.Dict {
		if d, ok := dicts[target]; ok {
			return d
		}
		d, err := lexcheck.Load(cfg.LexiconDir, cfg.Source, target)
		if err != nil || d == nil {
			fmt.Printf("[%s] hybrid gate: no lexicon (%v) — gating on alignment coverage only\n", target, err)
			d = nil
		}
		dicts[target] = d
		return d
	}
}

// mergeTargets returns the existing target list followed by any requested
// targets not already present — the final manifest targetLangs when adding a
// language to an existing .tbook. Order is stable: existing first, then new.
func mergeTargets(existing, requested []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(existing)+len(requested))
	for _, group := range [][]string{existing, requested} {
		for _, t := range group {
			if t != "" && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// runLexcheck statically scores every translated sentence against the
// bilingual lexicon (free, offline; see internal/lexcheck) and returns the
// flagged sources. Skipped with a notice when no lexicon file exists for the
// language pair.
func runLexcheck(cfg *config.Config, sentences []*tbook.Sentence) []string {
	target := cfg.Targets[0]
	dict, err := lexcheck.Load(cfg.LexiconDir, cfg.Source, target)
	if err != nil {
		fmt.Printf("lexcheck: %v — skipped\n", err)
		return nil
	}
	if dict == nil {
		fmt.Printf("lexcheck: no lexicon %s — skipped (run tools/fetch-lexicons.sh)\n",
			lexcheck.Path(cfg.LexiconDir, cfg.Source, target))
		return nil
	}
	var flagged []string
	seen := map[string]bool{}
	checked, lowSupport, shiftPattern := 0, 0, 0
	for _, s := range sentences {
		if seen[s.Src] {
			continue
		}
		seen[s.Src] = true
		r := dict.CheckSentence(s, target)
		if r.Covered == 0 {
			continue
		}
		checked++
		if !r.Flagged {
			continue
		}
		if r.Reason == "low-support" {
			lowSupport++
		} else {
			shiftPattern++
		}
		flagged = append(flagged, s.Src)
	}
	fmt.Printf("Lexcheck (%d headwords): %d checked, %d flagged (%d low-support, %d shift-pattern)\n",
		dict.Entries(), checked, len(flagged), lowSupport, shiftPattern)
	if len(flagged) > 0 {
		sort.Strings(flagged)
		path := cfg.Out + ".lexflagged.json"
		if err := translate.WriteFlagged(path, flagged); err == nil {
			fmt.Printf("Lexcheck flags written to %s\n", path)
		}
	}
	return flagged
}

// subsetBySrc returns the unique sentences whose source is in srcs.
func subsetBySrc(sentences []*tbook.Sentence, srcs []string) []*tbook.Sentence {
	set := make(map[string]bool, len(srcs))
	for _, src := range srcs {
		set[src] = true
	}
	var subset []*tbook.Sentence
	seen := map[*tbook.Sentence]bool{}
	for _, s := range sentences {
		if set[s.Src] && !seen[s] {
			seen[s] = true
			subset = append(subset, s)
		}
	}
	return subset
}

// escalateRun redoes a subset (translate + align) with the stronger
// --escalate-model, overwriting their entries in the PRIMARY model's cache
// namespace, then refreshes them from cache.
func escalateRun(ctx context.Context, cfg *config.Config, glossary []translate.GlossEntry,
	cacheModel string, subset []*tbook.Sentence) error {

	ec := translate.NewClient(clientOptions(cfg, cfg.EscalateModel, cfg.Temperature))
	pipe := &translate.Pipeline{
		Client: ec, CacheDir: cfg.CacheDir, Source: cfg.Source,
		BatchSize: cfg.BatchSize, AlignBatch: cfg.AlignBatch,
		Concurrency: cfg.Concurrency,
		Force:       true, // redo even though (bad) entries are cached
		Glossary:    glossary, CacheModel: cacheModel,
	}
	if err := pipe.Run(ctx, subset, cfg.Targets); err != nil {
		return err
	}
	found, missing := translate.FillFromCache(subset, cfg.Targets, cfg.CacheDir, cfg.Source, cacheModel)
	fmt.Printf("Escalation done: %d refreshed (%d missing).\n", found, missing)
	return nil
}

func escalate(ctx context.Context, cfg *config.Config, glossary []translate.GlossEntry,
	cacheModel string, sentences []*tbook.Sentence, flagged []string) error {

	subset := subsetBySrc(sentences, flagged)
	if len(subset) == 0 {
		return nil
	}
	fmt.Printf("Escalating %d flagged sentences to %s ...\n", len(subset), cfg.EscalateModel)
	return escalateRun(ctx, cfg, glossary, cacheModel, subset)
}

// reverifyEscalated closes the loop escalation used to leave open: the same
// gates (lexcheck + judge) re-check the escalated output, anything still
// flagged gets ONE more escalation attempt, and persistent failures fall back
// — a judged mistranslation loses its translation entirely (re-translated on
// the next run), an alignment failure keeps the raw translation with no
// highlights. Nothing the gates rejected ships silently.
func reverifyEscalated(ctx context.Context, cfg *config.Config, glossary []translate.GlossEntry,
	cacheModel string, sentences []*tbook.Sentence, flagged []string) error {

	subset := subsetBySrc(sentences, flagged)
	if len(subset) == 0 || (!cfg.Lexcheck && !cfg.Judge) {
		return nil
	}
	verify := func(set []*tbook.Sentence) ([]string, error) {
		bad := map[string]bool{}
		if cfg.Lexcheck {
			if dict, err := lexcheck.Load(cfg.LexiconDir, cfg.Source, cfg.Targets[0]); err == nil && dict != nil {
				for _, s := range set {
					if dict.CheckSentence(s, cfg.Targets[0]).Flagged {
						bad[s.Src] = true
					}
				}
			}
		}
		if cfg.Judge {
			jc := translate.NewClient(clientOptions(cfg, cfg.JudgeModel, 0))
			rep, err := translate.Judge(ctx, jc, cfg.CacheDir, cfg.Source, cfg.Targets,
				set, max(1, cfg.BatchSize/2), cfg.Concurrency)
			if err != nil {
				return nil, err
			}
			for _, src := range rep.FlaggedSrcs {
				bad[src] = true
			}
		}
		out := make([]string, 0, len(bad))
		for src := range bad {
			out = append(out, src)
		}
		sort.Strings(out)
		return out, nil
	}

	fmt.Printf("Re-verifying %d escalated sentences ...\n", len(subset))
	still, err := verify(subset)
	if err != nil {
		return err
	}
	if len(still) > 0 { // one redo, then re-check
		fmt.Printf("Re-escalating %d still-flagged sentences ...\n", len(still))
		if err := escalateRun(ctx, cfg, glossary, cacheModel, subsetBySrc(subset, still)); err != nil {
			return err
		}
		if still, err = verify(subsetBySrc(subset, still)); err != nil {
			return err
		}
	}
	if len(still) == 0 {
		fmt.Println("Escalation verified clean.")
		return nil
	}

	// Persistent failures: strip what the gates rejected and refill from cache.
	dropped, kept := 0, 0
	for _, s := range subsetBySrc(subset, still) {
		for _, target := range cfg.Targets {
			mistranslated := false
			if v, ok := translate.VerdictFor(cfg.CacheDir, cfg.JudgeModel, cfg.Source, target, s); ok {
				mistranslated = strings.Contains(v.Why, "mistranslation")
			}
			cache.Remove(cfg.CacheDir, cache.Key(s.Src, cfg.Source, target, cacheModel))
			if mistranslated {
				cache.Remove(cfg.CacheDir, cache.TrKey(s.Src, cfg.Source, target, cacheModel))
				dropped++
			} else {
				kept++
			}
		}
	}
	translate.FillFromCache(subsetBySrc(subset, still), cfg.Targets, cfg.CacheDir, cfg.Source, cacheModel)
	path := cfg.Out + ".unverified.json"
	_ = translate.WriteFlagged(path, still)
	fmt.Printf("Escalation left %d sentences unverified (%d ship raw with no highlights, "+
		"%d ship untranslated) — listed in %s; re-run to retry them.\n",
		len(still), kept, dropped, path)
	return nil
}

// runJudge judges every translated sentence, reports the verdict spread, and
// writes flagged sources to <out>.flagged.json (the --invalidate format). With
// --judge-invalidate the flagged cache entries are cleared immediately, so the
// very next run re-translates them (e.g. with a stronger --model). Returns the
// flagged source sentences.
func runJudge(ctx context.Context, cfg *config.Config, sentences []*tbook.Sentence) ([]string, error) {
	jc := translate.NewClient(clientOptions(cfg, cfg.JudgeModel, 0))
	fmt.Printf("Judging translations via %s ...\n", cfg.JudgeModel)
	// Judge batches are smaller than translate batches: each item carries the
	// full alignment, and verdict quality drops with oversized prompts (capped
	// at 16 so a big --batch-size doesn't degrade verdicts).
	batch := max(1, min(16, cfg.BatchSize/2))
	rep, err := translate.Judge(ctx, jc, cfg.CacheDir, cfg.Source, cfg.Targets, sentences, batch, cfg.Concurrency)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Judge: %d checked, %d flagged, %d unjudged\n", rep.Checked, rep.Flagged, rep.Unjudged)
	if len(rep.Reasons) > 0 {
		type rc struct {
			why string
			n   int
		}
		var rcs []rc
		for w, n := range rep.Reasons {
			rcs = append(rcs, rc{w, n})
		}
		sort.Slice(rcs, func(i, j int) bool { return rcs[i].n > rcs[j].n })
		for _, r := range rcs {
			fmt.Printf("  %5d × %s\n", r.n, r.why)
		}
	}
	if len(rep.FlaggedSrcs) == 0 {
		return nil, nil
	}
	sort.Strings(rep.FlaggedSrcs)
	flaggedPath := cfg.Out + ".flagged.json"
	if err := translate.WriteFlagged(flaggedPath, rep.FlaggedSrcs); err != nil {
		return nil, fmt.Errorf("write flagged list: %w", err)
	}
	fmt.Printf("Flagged sources written to %s\n", flaggedPath)
	if cfg.JudgeInvalidate {
		n := cache.Invalidate(cfg.CacheDir, rep.FlaggedSrcs, cfg.Targets, cfg.Source, cfg.Model)
		fmt.Printf("Invalidated %d cache file(s) — re-run (optionally with a stronger --model) to redo them.\n", n)
	} else if cfg.EscalateModel == "" {
		fmt.Printf("Re-do them with: convert --invalidate %s && convert …\n", flaggedPath)
	}
	return rep.FlaggedSrcs, nil
}

// loadSrcList reads source sentences from a file: a JSON array of strings, or
// (fallback) one non-empty source sentence per line.
func loadSrcList(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var arr []string
	if json.Unmarshal(b, &arr) == nil {
		out := arr[:0]
		for _, s := range arr {
			if s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	}
	var lines []string
	for ln := range strings.SplitSeq(string(b), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			lines = append(lines, ln)
		}
	}
	return lines, nil
}

func status(r tbook.Report) string {
	if r.OK() {
		return "OK"
	}
	if r.OffsetErrors > 0 || r.StructErrors > 0 {
		return "PROBLEM"
	}
	return "OK (some sentences untranslated)"
}

func printDryRun(sentences []*tbook.Sentence, chapters []tbook.Chapter, notes map[string]*tbook.Note) {
	fmt.Println("\n--- sample (first 6 sentences) ---")
	for i, s := range sentences {
		if i >= 6 {
			break
		}
		fmt.Printf("  • %s\n", s.Src)
	}
	var glued []*tbook.Sentence
	for _, s := range sentences {
		if strings.Contains(s.Src, "…") {
			glued = append(glued, s)
			if len(glued) >= 3 {
				break
			}
		}
	}
	if len(glued) > 0 {
		fmt.Println("\n--- ellipsis sentences (missing-space check) ---")
		for _, s := range glued {
			fmt.Printf("  • %s\n", s.Src)
		}
	}
	titles := make([]string, 0, len(chapters))
	for i, c := range chapters {
		if i >= 8 {
			titles = append(titles, "...")
			break
		}
		titles = append(titles, c.Title)
	}
	fmt.Printf("\nChapters: %s\n", strings.Join(titles, ", "))

	// Formatting fidelity: paragraph roles, inline emphasis, figures, tables,
	// footnote markers.
	roles := map[string]int{}
	emphSentences, emphSpans, markers := 0, 0, 0
	figures, tables := 0, 0
	for _, c := range chapters {
		figures += len(c.Figures)
		tables += len(c.Tables)
		for i, para := range c.Paragraphs {
			role := tbook.RoleBody
			if i < len(c.ParagraphStyles) && c.ParagraphStyles[i] != "" {
				role = c.ParagraphStyles[i]
			}
			roles[role]++
			for _, s := range para {
				if len(s.Spans) > 0 {
					emphSentences++
					emphSpans += len(s.Spans)
				}
				markers += len(s.Notes)
			}
		}
	}
	citations := 0
	for _, n := range notes {
		if n.Kind == tbook.NoteKindCitation {
			citations++
		}
	}
	fmt.Printf("Formatting: %d subtitle, %d heading, %d sceneBreak paragraphs; "+
		"%d sentences with emphasis (%d spans)\n",
		roles[tbook.RoleSubtitle], roles[tbook.RoleHeading], roles[tbook.RoleSceneBreak],
		emphSentences, emphSpans)
	fmt.Printf("Content: %d figures, %d tables, %d note markers → %d notes (%d citations)\n",
		figures, tables, markers, len(notes), citations)
}
