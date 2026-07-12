// Package config resolves runtime configuration from (in increasing
// precedence) built-in defaults, a .env file, process environment variables,
// and command-line flags.
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config is the fully resolved configuration for one run.
type Config struct {
	Input string // positional EPUB path
	Out   string

	// Provider selects the LLM backend: "openrouter" (metered HTTP API,
	// needs OPENROUTER_API_KEY), "claude" (the `claude` CLI in print mode —
	// runs on the user's Claude subscription, no API key, no per-token cost),
	// or "ollama" (a local model served by Ollama's OpenAI-compatible API —
	// free and offline, no key; needs `ollama serve` + a pulled model).
	Provider  string
	ClaudeBin string // claude CLI path; "" = "claude" from $PATH

	APIKey  string
	BaseURL string
	Model   string
	Referer string
	Title   string

	// OpenRouter provider routing (see internal/translate.Options).
	ProviderSort  string   // "throughput" | "latency" | "price" | "" (default routing)
	ProviderOrder []string // pin providers by slug, priority order

	Source  string
	Targets []string

	BatchSize   int
	AlignBatch  int // align-pass batch size; 0 = BatchSize/4 (small batches curb positional drift)
	Concurrency int
	MaxRetries  int
	Temperature float64
	Timeout     time.Duration
	JSONMode    bool

	CacheDir      string
	LimitChapters int
	DryRun        bool
	Force         bool
	Verbose       bool

	// Content extraction.
	KeepMatter    bool     // keep front/back matter (cover page, synopsis, credits, index)
	SkipFiles     []string // extra title/filename patterns to skip
	NoImages      bool     // drop body images
	NoNotes       bool     // drop footnotes entirely
	SkipCitations bool     // don't translate citation-kind notes (kept untranslated)

	// Quality passes.
	Glossary        bool   // build a book glossary and enforce it during translation (default on; --no-glossary disables)
	Judge           bool   // run the semantic verification pass after translating
	JudgeModel      string // model for the judge pass (default: same as Model)
	JudgeScope      string // "flagged" (default): judge only lexcheck-flagged + low-coverage sentences + a small calibration sample | "all"
	JudgeInvalidate bool   // immediately clear cache for judge-flagged sentences
	EscalateModel   string // with --judge/--lexcheck: redo flagged sentences with this stronger model, in place
	Lexcheck        bool   // static dictionary-based drift check
	LexiconDir      string // directory of compact lexicons (tools/fetch-lexicons.sh)

	// StatsPath, when set, appends one JSONL record per LLM request attempt
	// (latency, status, tokens, cost, provider) for offline analysis.
	StatsPath string

	// Alignment pass. "hybrid" (default) = local embedding aligner with LLM
	// fallback for gated sentences; "emb" = embedding aligner only; "llm" =
	// LLM align pass (see the speed report in issue #2 and tools/embalign-setup.sh).
	// AlignModeExplicit records whether the user chose the mode (flag or env):
	// an explicit emb/hybrid fails hard when the aligner can't start, while the
	// hybrid default degrades to llm with a notice.
	AlignMode         string
	AlignModeExplicit bool
	EmbPython string  // interpreter for tools/embalign.py ("" = auto: EMBALIGN_PYTHON, .venv-embalign, python3)
	EmbScript string  // aligner script path
	EmbModel  string  // HF encoder id
	EmbLayer  int     // hidden layer for token embeddings
	EmbMethod string  // argmax (precision-first) | itermax (recall-first)
	EmbQMin   float64 // hybrid gate: coverage threshold below which the LLM re-aligns

	// Invalidate, if set, names a file of source sentences whose cached
	// translation+alignment should be cleared (verify/QA loop); the run then exits.
	Invalidate string
}

// Load parses flags + environment (.env already loaded) into a Config.
func Load(args []string) (*Config, error) {
	// Load .env from the current dir if present; never overrides real env vars.
	_ = godotenv.Load()

	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: convert <book.epub|book.fb2|book.tbook> -o <book.tbook> [flags]\n\n"+
			"Translate an EPUB or FB2 into a .tbook archive — via OpenRouter (metered), the\n"+
			"claude CLI on your Claude subscription (--provider claude), or a local Ollama\n"+
			"model (--provider ollama) — or add a target\n"+
			"language to an existing .tbook (given as input; existing translations kept).\n"+
			"Configuration is read from .env (see .env.example), overridable by these flags.\n\nFlags:\n")
		fs.PrintDefaults()
	}

	var (
		out, outShort       string
		target, targetShort string
		source, sourceShort string
		verbose, verboseSh  bool
	)
	fs.StringVar(&out, "out", "", "output .tbook path (default: input with .tbook extension)")
	fs.StringVar(&outShort, "o", "", "shorthand for --out")
	fs.StringVar(&target, "target", envOr("TARGET_LANG", "ru"), "target language code(s), comma-separated")
	fs.StringVar(&targetShort, "t", "", "shorthand for --target")
	fs.StringVar(&source, "source", envOr("SOURCE_LANG", "en"), "source language code")
	fs.StringVar(&sourceShort, "s", "", "shorthand for --source")
	provider := fs.String("provider", envOr("PROVIDER", "openrouter"),
		"LLM backend: openrouter (metered API) | claude (claude CLI on your Claude subscription; no API key) | ollama (local Ollama server; no key)")
	model := fs.String("model", "",
		"model id (default: MODEL env or "+defaultOpenRouterModel+" for openrouter; CLAUDE_MODEL env or "+defaultClaudeModel+" for claude; OLLAMA_MODEL env or "+defaultOllamaModel+" for ollama)")
	cacheDir := fs.String("cache-dir", envOr("CACHE_DIR", ".tbook_cache"), "translation cache directory")
	batchSize := fs.Int("batch-size", envInt("BATCH_SIZE", 16), "sentences per API request")
	alignBatch := fs.Int("align-batch", envInt("ALIGN_BATCH", 0), "sentences per ALIGN request (0 = batch-size/4; small align batches curb positional drift)")
	concurrency := fs.Int("concurrency", envInt("CONCURRENCY", 32), "parallel API requests (gemini via OpenRouter took 48 with zero 429s; lower for :free models)")
	maxRetries := fs.Int("max-retries", envInt("MAX_RETRIES", 4), "per-request retry attempts")
	limit := fs.Int("limit-chapters", 0, "only convert the first N chapters (0 = all)")
	dryRun := fs.Bool("dry-run", false, "parse + segment only; no API calls, no output")
	force := fs.Bool("force", false, "force re-translation, ignoring cached entries (overwrites cache)")
	invalidate := fs.String("invalidate", "", "verify/QA loop: clear cached translation+alignment for the "+
		"source sentences in FILE (JSON array of strings, or one per line), then exit")
	keepMatter := fs.Bool("keep-matter", false, "keep front/back matter (cover page, synopsis, credits, index)")
	skipFiles := fs.String("skip-files", "", "extra chapter title/filename patterns to skip, comma-separated (case-insensitive regex)")
	noImages := fs.Bool("no-images", false, "drop body images (no figures in the output)")
	noNotes := fs.Bool("no-notes", false, "drop footnotes entirely (markers are still stripped from prose)")
	skipCitations := fs.Bool("skip-citations", false, "leave citation-kind footnotes untranslated (saves tokens)")
	// Glossary is one extra call and keeps recurring terms consistent; it runs
	// by default — --no-glossary opts out (e.g. to reuse a pre-glossary cache,
	// whose keys carry no glossary hash). --glossary is kept as an accepted no-op.
	_ = fs.Bool("glossary", true, "(default; deprecated) build a book glossary and enforce it during translation — on unless --no-glossary")
	noGlossary := fs.Bool("no-glossary", envBool("NO_GLOSSARY", false), "skip the book glossary (also reuses caches from pre-glossary runs)")
	judge := fs.Bool("judge", false, "after translating, run an independent LLM judge over every sentence; write flagged sources to <out>.flagged.json")
	judgeModel := fs.String("judge-model", envOr("JUDGE_MODEL", ""), "model for the judge pass (default: same as --model; see README before pointing it at a stronger model)")
	judgeScope := fs.String("judge-scope", envOr("JUDGE_SCOPE", "flagged"), "with --judge: flagged (default: only lexcheck-flagged + low-coverage sentences, plus a 5% calibration sample) | all (every sentence)")
	judgeInvalidate := fs.Bool("judge-invalidate", false, "with --judge: immediately clear cache for flagged sentences so the next run redoes them")
	escalateModel := fs.String("escalate-model", envOr("ESCALATE_MODEL", ""), "with --judge/--lexcheck: immediately redo flagged sentences with this stronger model (kept in the primary cache namespace)")
	// Lexcheck is free (offline) and runs by default; --no-lexcheck opts out.
	// --lexcheck is kept as an accepted no-op for backward compatibility.
	_ = fs.Bool("lexcheck", true, "(default; deprecated) static lexicon drift check — on unless --no-lexcheck")
	noLexcheck := fs.Bool("no-lexcheck", envBool("NO_LEXCHECK", false), "disable the default static lexicon drift check")
	lexiconDir := fs.String("lexicons", envOr("LEXICON_DIR", "lexicons"), "lexicon directory for the drift check")
	alignMode := fs.String("align-mode", envOr("ALIGN_MODE", AlignHybrid),
		"alignment pass: hybrid (default: local embedding aligner + LLM fallback for gated sentences) | emb (embedding aligner only) | llm (LLM align pass)")
	embPython := fs.String("embalign-python", envOr("EMBALIGN_PYTHON", ""), "python for the embedding aligner (default: .venv-embalign/bin/python if present, else python3; see tools/embalign-setup.sh)")
	embScript := fs.String("embalign-script", envOr("EMBALIGN_SCRIPT", "tools/embalign.py"), "embedding aligner script")
	embModel := fs.String("embalign-model", envOr("EMBALIGN_MODEL", "sentence-transformers/LaBSE"), "embedding aligner encoder (HF model id)")
	embLayer := fs.Int("embalign-layer", envInt("EMBALIGN_LAYER", 8), "embedding aligner hidden layer")
	embMethod := fs.String("embalign-method", envOr("EMBALIGN_METHOD", "argmax"), "embedding aligner matching: argmax (precision-first) | itermax (recall-first)")
	embQ := fs.Float64("embalign-q", envFloat("EMBALIGN_Q", 0.7), "hybrid gate: alignment-coverage threshold below which the LLM align pass redoes the sentence")
	stats := fs.String("stats", envOr("STATS_PATH", ""), "append per-request metrics (latency, status, tokens, cost) as JSONL to this file")
	providerSort := fs.String("provider-sort", envOr("PROVIDER_SORT", ""), "OpenRouter provider routing: throughput (fastest tokens/sec) | latency | price (empty = default routing)")
	providerOrder := fs.String("provider-order", envOr("PROVIDER_ORDER", ""), "pin OpenRouter providers by slug, comma-separated in priority order (e.g. alibaba)")
	fs.BoolVar(&verbose, "verbose", false, "verbose output")
	fs.BoolVar(&verboseSh, "v", false, "shorthand for --verbose")

	// Go's flag package stops at the first non-flag argument, so flags after the
	// positional EPUB path would be ignored. Parse repeatedly, peeling off each
	// positional, so flags and positionals may interleave freely.
	var positionals []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positionals = append(positionals, rest[0])
		rest = rest[1:]
	}
	input := ""
	if len(positionals) > 0 {
		input = positionals[0]
	}

	cfg := &Config{
		Input:           input,
		Out:             pick(outShort, out),
		Provider:        strings.ToLower(strings.TrimSpace(*provider)),
		ClaudeBin:       envOr("CLAUDE_BIN", "claude"),
		APIKey:          os.Getenv("OPENROUTER_API_KEY"),
		BaseURL:         strings.TrimRight(envOr("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"), "/"),
		Model:           *model,
		Referer:         os.Getenv("OPENROUTER_HTTP_REFERER"),
		Title:           envOr("OPENROUTER_X_TITLE", "reader-tbook-converter"),
		ProviderSort:    *providerSort,
		ProviderOrder:   splitCodes(*providerOrder),
		Source:          pick(sourceShort, source),
		Targets:         splitCodes(pick(targetShort, target)),
		BatchSize:       *batchSize,
		AlignBatch:      *alignBatch,
		Concurrency:     *concurrency,
		MaxRetries:      *maxRetries,
		Temperature:     envFloat("TEMPERATURE", 0.3),
		Timeout:         time.Duration(envInt("REQUEST_TIMEOUT_SEC", 120)) * time.Second,
		JSONMode:        envBool("USE_JSON_MODE", true),
		CacheDir:        *cacheDir,
		LimitChapters:   *limit,
		DryRun:          *dryRun,
		Force:           *force,
		Verbose:         verbose || verboseSh,
		KeepMatter:      *keepMatter,
		SkipFiles:       splitCodes(*skipFiles),
		NoImages:        *noImages,
		NoNotes:         *noNotes,
		SkipCitations:   *skipCitations,
		Glossary:        !*noGlossary,
		Judge:           *judge,
		JudgeModel:      *judgeModel,
		JudgeScope:      strings.ToLower(strings.TrimSpace(*judgeScope)),
		JudgeInvalidate: *judgeInvalidate,
		StatsPath:       *stats,
		EscalateModel:   *escalateModel,
		Lexcheck:        !*noLexcheck,
		LexiconDir:      *lexiconDir,
		AlignMode:       strings.ToLower(strings.TrimSpace(*alignMode)),
		EmbPython:       *embPython,
		EmbScript:       *embScript,
		EmbModel:        *embModel,
		EmbLayer:        *embLayer,
		EmbMethod:       *embMethod,
		EmbQMin:         *embQ,
		Invalidate:      *invalidate,
	}
	if cfg.Provider != ProviderOpenRouter && cfg.Provider != ProviderClaude && cfg.Provider != ProviderOllama {
		return nil, fmt.Errorf("unknown --provider %q (want openrouter, claude or ollama)", cfg.Provider)
	}
	// Ollama serves an OpenAI-compatible API locally and needs no key
	// (OLLAMA_API_KEY exists for a reverse proxy that demands one). The
	// OPENROUTER_* endpoint/key from .env must not leak into it.
	if cfg.Provider == ProviderOllama {
		cfg.BaseURL = strings.TrimRight(envOr("OLLAMA_BASE_URL", "http://localhost:11434/v1"), "/")
		cfg.APIKey = os.Getenv("OLLAMA_API_KEY")
	}
	if cfg.AlignMode != AlignLLM && cfg.AlignMode != AlignEmb && cfg.AlignMode != AlignHybrid {
		return nil, fmt.Errorf("unknown --align-mode %q (want llm, emb or hybrid)", cfg.AlignMode)
	}
	// Explicit align-mode choice (flag or env) fails hard when the local
	// aligner is missing; the built-in hybrid default degrades to llm instead.
	cfg.AlignModeExplicit = os.Getenv("ALIGN_MODE") != ""
	concurrencySet := os.Getenv("CONCURRENCY") != ""
	batchSet := os.Getenv("BATCH_SIZE") != ""
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "align-mode":
			cfg.AlignModeExplicit = true
		case "concurrency":
			concurrencySet = true
		case "batch-size":
			batchSet = true
		}
	})
	if cfg.JudgeScope != JudgeScopeAll && cfg.JudgeScope != JudgeScopeFlagged {
		return nil, fmt.Errorf("unknown --judge-scope %q (want all or flagged)", cfg.JudgeScope)
	}
	// The model default follows the provider, each with its own env var — the
	// MODEL in .env is an OpenRouter id and must never leak into the claude
	// CLI or an Ollama server (which would fail every call on an unknown model).
	if cfg.Model == "" {
		switch cfg.Provider {
		case ProviderClaude:
			cfg.Model = envOr("CLAUDE_MODEL", defaultClaudeModel)
		case ProviderOllama:
			cfg.Model = envOr("OLLAMA_MODEL", defaultOllamaModel)
		default:
			cfg.Model = envOr("MODEL", defaultOpenRouterModel)
		}
	}
	// A claude CLI call carries process-startup overhead on top of generation,
	// and local Ollama inference is slow outright; give both a roomier default
	// timeout (explicit REQUEST_TIMEOUT_SEC still wins).
	if (cfg.Provider == ProviderClaude || cfg.Provider == ProviderOllama) && os.Getenv("REQUEST_TIMEOUT_SEC") == "" {
		cfg.Timeout = 300 * time.Second
	}
	// An Ollama server runs OLLAMA_NUM_PARALLEL requests (often 1) and queues
	// the rest, which then share the per-request timeout budget — the default
	// 32 would time batches out in the queue. And local translation fine-tunes
	// (translategemma) silently translate only the first few items of a large
	// batch: measured 16 → 1 answer, 8 → all 8; 4 keeps a margin, and costs
	// little since local generation is output-bound. Explicit flag/env wins.
	if cfg.Provider == ProviderOllama && !concurrencySet {
		cfg.Concurrency = defaultOllamaConcurrency
	}
	if cfg.Provider == ProviderOllama && !batchSet {
		cfg.BatchSize = defaultOllamaBatch
	}
	// OpenRouter ids carry a vendor prefix ("google/…"); Claude model ids never
	// do. Under the claude provider, drop such leftovers from .env rather than
	// failing every judge/escalate call on an unknown model.
	if cfg.Provider == ProviderClaude {
		if strings.Contains(cfg.JudgeModel, "/") {
			cfg.JudgeModel = ""
		}
		if strings.Contains(cfg.EscalateModel, "/") {
			cfg.EscalateModel = ""
		}
	}
	// Same hygiene for Ollama, whose ids are "name[:tag]" — a "/" appears only
	// in registry pulls ("hf.co/org/model:Q4"), which always carry a ":tag".
	// An id with "/" but no ":" is an OpenRouter leftover from .env.
	if cfg.Provider == ProviderOllama {
		if strings.Contains(cfg.JudgeModel, "/") && !strings.Contains(cfg.JudgeModel, ":") {
			cfg.JudgeModel = ""
		}
		if strings.Contains(cfg.EscalateModel, "/") && !strings.Contains(cfg.EscalateModel, ":") {
			cfg.EscalateModel = ""
		}
	}
	// Default judge = translate model. Measured (see README): a stronger judge
	// keeps full drift recall but flags 50-70% of sentences on style pedantry —
	// it even rejects 2/3 of its OWN escalated output — which condemns the
	// escalation loop to mass fallback. The calibrated same-model judge plus
	// lexcheck is the economical gate; --judge-model remains for audits.
	if cfg.JudgeModel == "" {
		cfg.JudgeModel = cfg.Model
	}

	// --invalidate is a cache-maintenance op; it needs no input EPUB.
	if cfg.Input == "" && cfg.Invalidate == "" {
		return nil, fmt.Errorf("missing input .epub/.tbook (usage: convert <book.epub|book.tbook> -o <book.tbook>)")
	}
	if cfg.Out == "" && cfg.Input != "" {
		// A .tbook input is the add-a-language path: default to overwriting it
		// in place (adding the new target). A book input maps to a sibling .tbook.
		if strings.HasSuffix(cfg.Input, ".tbook") {
			cfg.Out = cfg.Input
		} else {
			cfg.Out = trimBookExt(cfg.Input) + ".tbook"
		}
	}
	if len(cfg.Targets) == 0 {
		cfg.Targets = []string{"ru"}
	}
	if cfg.BatchSize < 1 {
		cfg.BatchSize = 1
	}
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
	return cfg, nil
}

// LLM backends (mirrored in internal/translate; kept as plain strings here to
// avoid an import cycle risk — config is a leaf).
const (
	ProviderOpenRouter = "openrouter"
	ProviderClaude     = "claude"
	ProviderOllama     = "ollama"
)

// Alignment-pass modes (mirrored in internal/translate).
const (
	AlignLLM    = "llm"
	AlignEmb    = "emb"
	AlignHybrid = "hybrid"
)

// Judge scopes.
const (
	JudgeScopeAll     = "all"     // judge every translated sentence
	JudgeScopeFlagged = "flagged" // judge lexcheck-flagged + low-coverage + calibration sample
)

// Per-provider model defaults.
const (
	defaultOpenRouterModel = "google/gemini-2.5-flash"
	defaultClaudeModel     = "claude-haiku-4-5"
	defaultOllamaModel     = "translategemma:12b" // Gemma 3 translation fine-tune (ollama.com/library/translategemma)
)

// defaultOllamaConcurrency and defaultOllamaBatch replace the global defaults
// when no flag/env sets them — a local server serializes past
// OLLAMA_NUM_PARALLEL, and local translation fine-tunes drop most of a large
// batch (see Load).
const (
	defaultOllamaConcurrency = 2
	defaultOllamaBatch       = 4
)

// trimBookExt strips a known book extension (case-insensitive) so the default
// output lands next to the input as <name>.tbook.
func trimBookExt(path string) string {
	lower := strings.ToLower(path)
	for _, ext := range []string{".fb2.zip", ".fb2", ".epub"} {
		if strings.HasSuffix(lower, ext) {
			return path[:len(path)-len(ext)]
		}
	}
	return path
}

// pick returns the first non-empty value (used for short/long flag aliases).
func pick(short, long string) string {
	if short != "" {
		return short
	}
	return long
}

func splitCodes(s string) []string {
	var out []string
	seen := map[string]bool{}
	for c := range strings.SplitSeq(s, ",") {
		c = strings.TrimSpace(c)
		if c != "" && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return def
}
