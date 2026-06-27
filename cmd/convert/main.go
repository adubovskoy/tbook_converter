// Command convert turns an EPUB into a .tbook language-learning archive,
// translating every sentence via OpenRouter with word-level alignment.
//
//	convert book.epub -o book.tbook            # English → Russian (default)
//	convert book.epub -t ru,de -o book.tbook   # multiple target languages
//	convert book.epub --dry-run                # parse + segment only, no API
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/epub"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
)

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
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		return err
	}

	// Parse + segment (no API).
	fmt.Printf("Parsing %s ...\n", cfg.Input)
	book, err := epub.Parse(cfg.Input)
	if err != nil {
		return fmt.Errorf("parse epub: %w", err)
	}
	chapters := book.Chapters
	if cfg.LimitChapters > 0 && cfg.LimitChapters < len(chapters) {
		chapters = chapters[:cfg.LimitChapters]
	}
	coverState := "no"
	if book.Cover != nil {
		coverState = "yes"
	}
	fmt.Printf("  %q by %s — %d chapters, cover: %s\n", book.Title, book.Author, len(chapters), coverState)

	outChapters, sentences := segment.BuildSentenceObjects(chapters)
	words := 0
	for _, s := range sentences {
		words += len(s.Words)
	}
	fmt.Printf("  %d sentences, ~%d words\n", len(sentences), words)

	if cfg.DryRun {
		printDryRun(sentences, outChapters)
		return nil
	}

	// Only the untranslated work needs the API; a fully-cached run assembles
	// offline (resume / re-assemble) without a key.
	pending := translate.CountPending(sentences, cfg.Targets, cfg.CacheDir, cfg.Source, cfg.Model, cfg.Force)
	if pending > 0 {
		if cfg.APIKey == "" {
			return fmt.Errorf("%d sentences need translating but OPENROUTER_API_KEY is not set "+
				"(put it in converter/.env — see .env.example)", pending)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		fmt.Printf("Translating %s→%s via %s ...\n", cfg.Source, strings.Join(cfg.Targets, ","), cfg.Model)
		client := translate.NewClient(translate.Options{
			BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
			Referer: cfg.Referer, Title: cfg.Title,
			Temperature: cfg.Temperature, JSONMode: cfg.JSONMode,
			MaxRetries: cfg.MaxRetries, Timeout: cfg.Timeout,
		})
		pipe := &translate.Pipeline{
			Client: client, CacheDir: cfg.CacheDir, Source: cfg.Source,
			BatchSize: cfg.BatchSize, Concurrency: cfg.Concurrency,
			Force: cfg.Force,
		}
		if err := pipe.Run(ctx, sentences, cfg.Targets); err != nil {
			return err
		}
	} else {
		fmt.Println("All sentences already cached — assembling offline (no API calls).")
	}

	// Fill from cache + assemble.
	found, missing := translate.FillFromCache(sentences, cfg.Targets, cfg.CacheDir, cfg.Source, cfg.Model)
	fmt.Printf("Filled %d translations from cache (%d missing).\n", found, missing)

	fmt.Printf("Writing %s ...\n", cfg.Out)
	if err := tbook.Write(cfg.Out, book.Title, book.Author, cfg.Source, cfg.Targets, book.Cover, outChapters); err != nil {
		return fmt.Errorf("assemble: %w", err)
	}
	if fi, err := os.Stat(cfg.Out); err == nil {
		fmt.Printf("Done. %s (%d KB)\n", cfg.Out, fi.Size()/1024)
	}

	// Validate.
	rep := tbook.Validate(outChapters, cfg.Targets)
	fmt.Printf("Validation: %d sentences, %d empty, %d offset_errors — %s\n",
		rep.Sentences, rep.Empty, rep.OffsetErrors, status(rep))
	if rep.OffsetErrors > 0 {
		return fmt.Errorf("validation failed: %d offset errors", rep.OffsetErrors)
	}
	return nil
}

func status(r tbook.Report) string {
	if r.OK() {
		return "OK"
	}
	if r.OffsetErrors > 0 {
		return "PROBLEM"
	}
	return "OK (some sentences untranslated)"
}

func printDryRun(sentences []*tbook.Sentence, chapters []tbook.Chapter) {
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

	// Formatting fidelity: paragraph roles + inline emphasis.
	roles := map[string]int{}
	emphSentences, emphSpans := 0, 0
	for _, c := range chapters {
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
			}
		}
	}
	fmt.Printf("Formatting: %d subtitle, %d heading, %d sceneBreak paragraphs; "+
		"%d sentences with emphasis (%d spans)\n",
		roles[tbook.RoleSubtitle], roles[tbook.RoleHeading], roles[tbook.RoleSceneBreak],
		emphSentences, emphSpans)
}
