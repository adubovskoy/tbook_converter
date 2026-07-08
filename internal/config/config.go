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
	Glossary        bool   // build a book glossary and enforce it during translation
	Judge           bool   // run the semantic verification pass after translating
	JudgeModel      string // model for the judge pass (default: same as Model)
	JudgeInvalidate bool   // immediately clear cache for judge-flagged sentences
	EscalateModel   string // with --judge/--lexcheck: redo flagged sentences with this stronger model, in place
	Lexcheck        bool   // static dictionary-based drift check
	LexiconDir      string // directory of compact lexicons (tools/fetch-lexicons.sh)

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
		fmt.Fprintf(fs.Output(), "Usage: convert <book.epub|book.tbook> -o <book.tbook> [flags]\n\n"+
			"Translate an EPUB into a .tbook archive via OpenRouter, or add a target\n"+
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
	model := fs.String("model", envOr("MODEL", "google/gemini-2.5-flash"), "OpenRouter model id")
	cacheDir := fs.String("cache-dir", envOr("CACHE_DIR", ".tbook_cache"), "translation cache directory")
	batchSize := fs.Int("batch-size", envInt("BATCH_SIZE", 16), "sentences per API request")
	alignBatch := fs.Int("align-batch", envInt("ALIGN_BATCH", 0), "sentences per ALIGN request (0 = batch-size/4; small align batches curb positional drift)")
	concurrency := fs.Int("concurrency", envInt("CONCURRENCY", 6), "parallel API requests")
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
	glossary := fs.Bool("glossary", false, "build a book glossary first and enforce it during translation for term consistency")
	judge := fs.Bool("judge", false, "after translating, run an independent LLM judge over every sentence; write flagged sources to <out>.flagged.json")
	judgeModel := fs.String("judge-model", envOr("JUDGE_MODEL", ""), "model for the judge pass (default: same as --model; see README before pointing it at a stronger model)")
	judgeInvalidate := fs.Bool("judge-invalidate", false, "with --judge: immediately clear cache for flagged sentences so the next run redoes them")
	escalateModel := fs.String("escalate-model", envOr("ESCALATE_MODEL", ""), "with --judge/--lexcheck: immediately redo flagged sentences with this stronger model (kept in the primary cache namespace)")
	// Lexcheck is free (offline) and runs by default; --no-lexcheck opts out.
	// --lexcheck is kept as an accepted no-op for backward compatibility.
	_ = fs.Bool("lexcheck", true, "(default; deprecated) static lexicon drift check — on unless --no-lexcheck")
	noLexcheck := fs.Bool("no-lexcheck", envBool("NO_LEXCHECK", false), "disable the default static lexicon drift check")
	lexiconDir := fs.String("lexicons", envOr("LEXICON_DIR", "lexicons"), "lexicon directory for the drift check")
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
		Input:         input,
		Out:           pick(outShort, out),
		APIKey:        os.Getenv("OPENROUTER_API_KEY"),
		BaseURL:       strings.TrimRight(envOr("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"), "/"),
		Model:         *model,
		Referer:       os.Getenv("OPENROUTER_HTTP_REFERER"),
		Title:         envOr("OPENROUTER_X_TITLE", "reader-tbook-converter"),
		ProviderSort:  *providerSort,
		ProviderOrder: splitCodes(*providerOrder),
		Source:        pick(sourceShort, source),
		Targets:       splitCodes(pick(targetShort, target)),
		BatchSize:     *batchSize,
		AlignBatch:    *alignBatch,
		Concurrency:   *concurrency,
		MaxRetries:    *maxRetries,
		Temperature:   envFloat("TEMPERATURE", 0.3),
		Timeout:       time.Duration(envInt("REQUEST_TIMEOUT_SEC", 120)) * time.Second,
		JSONMode:      envBool("USE_JSON_MODE", true),
		CacheDir:      *cacheDir,
		LimitChapters: *limit,
		DryRun:        *dryRun,
		Force:         *force,
		Verbose:       verbose || verboseSh,
		KeepMatter:    *keepMatter,
		SkipFiles:     splitCodes(*skipFiles),
		NoImages:      *noImages,
		NoNotes:       *noNotes,
		SkipCitations: *skipCitations,
		Glossary:        *glossary,
		Judge:           *judge,
		JudgeModel:      *judgeModel,
		JudgeInvalidate: *judgeInvalidate,
		EscalateModel:   *escalateModel,
		Lexcheck:        !*noLexcheck,
		LexiconDir:      *lexiconDir,
		Invalidate:    *invalidate,
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
		// in place (adding the new target). An .epub input maps to a sibling .tbook.
		if strings.HasSuffix(cfg.Input, ".tbook") {
			cfg.Out = cfg.Input
		} else {
			cfg.Out = strings.TrimSuffix(cfg.Input, ".epub") + ".tbook"
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
