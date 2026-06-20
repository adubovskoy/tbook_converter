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

	Source  string
	Targets []string

	BatchSize   int
	Concurrency int
	MaxRetries  int
	Temperature float64
	Timeout     time.Duration
	JSONMode    bool

	CacheDir      string
	LimitChapters int
	DryRun        bool
	Verbose       bool
}

// Load parses flags + environment (.env already loaded) into a Config.
func Load(args []string) (*Config, error) {
	// Load .env from the current dir if present; never overrides real env vars.
	_ = godotenv.Load()

	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: convert <book.epub> -o <book.tbook> [flags]\n\n"+
			"Translate an EPUB into a .tbook archive via OpenRouter.\n"+
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
	concurrency := fs.Int("concurrency", envInt("CONCURRENCY", 6), "parallel API requests")
	maxRetries := fs.Int("max-retries", envInt("MAX_RETRIES", 4), "per-request retry attempts")
	limit := fs.Int("limit-chapters", 0, "only convert the first N chapters (0 = all)")
	dryRun := fs.Bool("dry-run", false, "parse + segment only; no API calls, no output")
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
		Source:        pick(sourceShort, source),
		Targets:       splitCodes(pick(targetShort, target)),
		BatchSize:     *batchSize,
		Concurrency:   *concurrency,
		MaxRetries:    *maxRetries,
		Temperature:   envFloat("TEMPERATURE", 0.3),
		Timeout:       time.Duration(envInt("REQUEST_TIMEOUT_SEC", 120)) * time.Second,
		JSONMode:      envBool("USE_JSON_MODE", true),
		CacheDir:      *cacheDir,
		LimitChapters: *limit,
		DryRun:        *dryRun,
		Verbose:       verbose || verboseSh,
	}

	if cfg.Input == "" {
		return nil, fmt.Errorf("missing input .epub (usage: convert <book.epub> -o <book.tbook>)")
	}
	if cfg.Out == "" {
		cfg.Out = strings.TrimSuffix(cfg.Input, ".epub") + ".tbook"
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
