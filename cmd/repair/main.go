// PRODUCTION NOTE: convert runs this pass itself (`--repair`, default-on for
// gonka) — see the phaseRepair phase and repairSystemPrompt in
// internal/translate. This command stays the RESEARCH harness: it keeps the
// knobs the pipeline deliberately does not expose (strict/--fluent/--bold
// mandates, --context N neighbouring sentences, --no-gloss-guard, --diff) so an
// arm can be measured against a frozen baseline cache without touching the
// production path.
//
// Command repair is a research harness for the proofread/repair pass: it runs a
// second LLM pass over the RAW translations a previous run already cached and
// writes the result into a DIFFERENT cache dir, under the same raw-translation
// key. Nothing is aligned here — a following `convert --cache-dir <dst>` aligns
// and assembles the repaired text without re-translating a single sentence, so
// repaired and baseline books differ by exactly one variable.
//
//	repair book.epub --src-cache .cache-exp-kimi-a --dst-cache .cache-exp-kimi-rep \
//	       -s en -t ru --provider gonka --gloss-hash 6c8e9c945df1 \
//	       --limit-chapters 5 --diff repair-diff.json
//
// The cache namespace must match the run that produced --src-cache: pass the
// glossary hash it printed (`cache namespace <model>+g:<hash>`) as --gloss-hash,
// or the whole namespace string as --cache-model (needed when --model repairs
// with a different model than the one that translated). Everything else —
// provider, endpoint, key, model defaults, --batch-size, --concurrency,
// --limit-chapters, --stats, --dry-run, the parse/segment options — is convert's
// own configuration, resolved by internal/config from the same .env and flags.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/epub"
	"github.com/dimando/reader/converter/internal/fb2"
	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"
)

// repairFlagNames are the flags this tool owns; every other argument goes to
// config.Load, so a repair run is configured exactly like the convert run that
// produced the cache. The value here says whether the flag takes an argument —
// splitArgs must not swallow the next token for a boolean one.
var repairFlagNames = map[string]bool{
	"src-cache": true, "dst-cache": true, "cache-model": true,
	"gloss-hash": true, "diff": true,
	"fluent": false, "no-gloss-guard": false, "bold": false, "context": true,
}

const usage = `Usage: repair <book.epub|book.fb2> --src-cache DIR --dst-cache DIR [flags]

Proofread the RAW translations cached in --src-cache with one more LLM pass and
write them into --dst-cache under the same raw-translation key, so
` + "`convert --cache-dir <dst>`" + ` aligns and assembles the repaired text with no
re-translation. Alignment entries are never written here.

repair flags:
  --src-cache DIR    cache dir holding the raw translations to proofread (default: --cache-dir)
  --dst-cache DIR    cache dir to write the repaired raw translations into (required, != src)
  --cache-model M    full cache-key namespace of the source cache ("model+g:hash");
                     overrides --model/--gloss-hash for keying (not for the repair call)
  --gloss-hash H     glossary hash the source cache was built with (the run printed
                     "cache namespace <model>+g:<hash>"); combined with --model
  --diff FILE        write [{src,before,after}] for every CHANGED sentence (research artifact)

Everything else is convert's own configuration (see convert -h): -s, -t,
--provider, --model, --batch-size, --concurrency, --limit-chapters, --stats,
--dry-run, --keep-matter, --skip-citations, …
`

func main() {
	for _, a := range os.Args[1:] {
		if a == "-h" || a == "--help" || a == "-help" {
			fmt.Print(usage)
			return
		}
	}
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

	mine, rest := splitArgs(os.Args[1:])
	fs := flag.NewFlagSet("repair", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(fs.Output(), usage) }
	srcCache := fs.String("src-cache", "", "cache dir holding the raw translations to proofread")
	dstCache := fs.String("dst-cache", "", "cache dir for the repaired raw translations")
	cacheModel := fs.String("cache-model", "", "full cache-key namespace of the source cache (\"model+g:hash\")")
	glossHash := fs.String("gloss-hash", "", "glossary hash the source cache was built with")
	diffPath := fs.String("diff", "", "write [{src,before,after}] for every changed sentence to this file")
	fluent := fs.Bool("fluent", false, "also rewrite grammatical but unidiomatic phrasing (wider mandate, freer edits)")
	noGloss := fs.Bool("no-gloss-guard", false, "do not show the source cache's glossary to the proofreader")
	bold := fs.Bool("bold", false, "drop the \"most items need no change\" anchor (higher recall, more edits)")
	ctxN := fs.Int("context", 0, "send the N preceding sentences (source + finished translation) as read-only context")
	if err := fs.Parse(mine); err != nil {
		return err
	}
	cfg, err := config.Load(rest)
	if err != nil {
		return err
	}
	if strings.HasSuffix(cfg.Input, ".tbook") {
		return fmt.Errorf("repair needs the source book (.epub/.fb2) so it sees the same sentence set as convert, not a .tbook")
	}
	if *srcCache == "" {
		*srcCache = cfg.CacheDir
	}
	if *dstCache == "" {
		return fmt.Errorf("--dst-cache is required (repairing into --src-cache would overwrite the baseline)")
	}
	if abs(*dstCache) == abs(*srcCache) {
		return fmt.Errorf("--dst-cache must differ from --src-cache (%s) — the baseline must stay intact", *srcCache)
	}
	if len(cfg.Targets) != 1 {
		return fmt.Errorf("repair does one target per run (got %v) — its counters and --diff are per language pair",
			cfg.Targets)
	}
	target := cfg.Targets[0]
	// The cache namespace of the run being repaired. --cache-model is the whole
	// string (the only way to key a cache the repair model didn't produce);
	// --gloss-hash is the convenience form for repairing with the same model.
	keyModel := *cacheModel
	if keyModel == "" {
		keyModel = translate.CacheKeyModel(cfg.Model, *glossHash)
	}

	var statsLog *translate.Stats
	if cfg.StatsPath != "" {
		statsLog, err = translate.OpenStats(cfg.StatsPath)
		if err != nil {
			return fmt.Errorf("open --stats file: %w", err)
		}
		defer statsLog.Close()
		fmt.Printf("Per-request metrics → %s\n", cfg.StatsPath)
	}

	sentences, err := bookSentences(cfg)
	if err != nil {
		return err
	}

	// Unique source sentences in book order, each with the raw translation
	// cached under the source namespace. A sentence with no cached raw
	// translation has nothing to proofread: it is skipped (and reported) — the
	// following convert run will translate it itself.
	type item struct {
		src string
		key string // raw-translation key; identical in both cache dirs
		tr  string // raw translation as cached in --src-cache
	}
	var items []item
	seen := map[string]bool{}
	uncached := 0
	for _, s := range sentences {
		if seen[s.Src] {
			continue
		}
		seen[s.Src] = true
		key := cache.TrKey(s.Src, cfg.Source, target, keyModel)
		raw, ok := cache.Read(*srcCache, key)
		if !ok || strings.TrimSpace(raw.Text) == "" {
			uncached++
			continue
		}
		items = append(items, item{src: s.Src, key: key, tr: strings.TrimSpace(raw.Text)})
	}
	// Parallel to items, so a batch can attach its neighbours without carrying
	// the whole item type around. Book order, duplicates already collapsed.
	itemSrcTr := make([]prevItem, len(items))
	for i, it := range items {
		itemSrcTr[i] = prevItem{Src: it.src, Tr: it.tr}
	}

	fmt.Printf("Cache namespace %s (%s→%s)\n", keyModel, cfg.Source, target)
	fmt.Printf("  %d unique sentences — %d with a cached raw translation in %s, %d without (skipped)\n",
		len(seen), len(items), *srcCache, uncached)
	if len(items) == 0 {
		return fmt.Errorf("no cached raw translations found in %s under namespace %s — "+
			"wrong --cache-model/--gloss-hash, --source/--target, or cache dir", *srcCache, keyModel)
	}

	// Index ranges, not slices of slices: each batch fills its own stripe of the
	// result arrays, so nothing but the failure counters needs a lock.
	size := max(1, cfg.BatchSize)
	var starts []int
	for i := 0; i < len(items); i += size {
		starts = append(starts, i)
	}
	var gloss []translate.GlossEntry
	if !*noGloss {
		gloss = srcGlossary(*srcCache)
	}
	sys := repairSystemPrompt(translate.LangName(cfg.Source), translate.LangName(target), gloss, *fluent, *bold, *ctxN)
	mode := "strict"
	if *fluent {
		mode = "fluent"
	}
	if *bold {
		mode += "+bold"
	}
	if *ctxN > 0 {
		mode += fmt.Sprintf("+ctx%d", *ctxN)
	}
	fmt.Printf("Proofreading %s→%s via %s (%s): %d sentences in %d batches (batch %d, concurrency %d, mode %s, glossary %d terms)\n",
		cfg.Source, target, cfg.Model, cfg.Provider, len(items), len(starts), size, cfg.Concurrency,
		mode, len(gloss))

	if cfg.DryRun {
		end := min(size, len(items))
		inputs := make([]repairInput, 0, end)
		for i, it := range items[:end] {
			inputs = append(inputs, repairInput{ID: strconv.Itoa(i + 1), Src: it.src, Tr: it.tr,
				Prev: prevContext(itemSrcTr, i, *ctxN)})
		}
		payload, err := jsonx.MarshalIndent(inputs, "  ")
		if err != nil {
			return err
		}
		fmt.Printf("\n--- dry run: nothing sent, nothing written ---\n")
		fmt.Printf("system prompt (%d bytes):\n%s\n\nfirst batch payload:\n%s\n",
			len(sys), sys, payload)
		fmt.Printf("\nwould send %d batches, write %d entries to %s%s\n",
			len(starts), len(items), *dstCache, glossaryNote(*srcCache, *dstCache))
		return nil
	}

	if err := needKey(cfg); err != nil {
		return err
	}
	client := translate.NewClient(translate.Options{
		Provider: cfg.Provider, ClaudeBin: cfg.ClaudeBin,
		BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
		Referer: cfg.Referer, Title: cfg.Title,
		// Temperature 0: proofreading is a verification pass, like the judge —
		// the same input must not drift between runs.
		Temperature: 0, JSONMode: cfg.JSONMode,
		MaxRetries: cfg.MaxRetries, Timeout: cfg.Timeout,
		ProviderSort: cfg.ProviderSort, ProviderOrder: cfg.ProviderOrder,
		Stats: statsLog,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = translate.WithPhase(ctx, "repair")

	repaired := make([]string, len(items)) // "" = keep the original (dropped id or failed batch)
	kept := make([]bool, len(items))       // the reply carried no usable text for this id
	var (
		mu      sync.Mutex
		failed  int // batches the client gave up on (retries exhausted)
		lastErr error
		bar     = progressbar.Default(int64(len(starts)), "repair")
		t0      = time.Now()
		g, gctx = errgroup.WithContext(ctx)
	)
	g.SetLimit(max(1, cfg.Concurrency))
	for _, start := range starts {
		g.Go(func() error {
			defer func() { _ = bar.Add(1) }()
			end := min(start+size, len(items))
			inputs := make([]repairInput, 0, end-start)
			for i, it := range items[start:end] {
				inputs = append(inputs, repairInput{ID: strconv.Itoa(i + 1), Src: it.src, Tr: it.tr,
					Prev: prevContext(itemSrcTr, start+i, *ctxN)})
			}
			payload, err := jsonx.Marshal(inputs)
			if err != nil {
				return err
			}
			var out map[string]string
			if err := client.ChatJSON(gctx, sys, string(payload), &out); err != nil {
				mu.Lock()
				failed++
				lastErr = err
				mu.Unlock()
				// A batch that never came back keeps its originals — never drop a
				// sentence — except when the run itself must stop.
				if _, ok := errors.AsType[*translate.UsageLimitError](err); ok {
					return err
				}
				return nil
			}
			for i := range items[start:end] {
				text := strings.TrimSpace(out[strconv.Itoa(i+1)])
				if text == "" { // dropped or garbled id
					kept[start+i] = true
					continue
				}
				repaired[start+i] = text
			}
			return nil
		})
	}
	runErr := g.Wait()
	_ = bar.Finish()
	elapsed := time.Since(t0)
	if runErr != nil {
		// Nothing is written on an abort (interrupt, usage limit): a cache where
		// only some batches were proofread is COMPLETE as far as convert can
		// tell, and would silently pass for a fully repaired one.
		return fmt.Errorf("aborted after %s — nothing written to %s: %w",
			elapsed.Round(time.Second), *dstCache, runErr)
	}

	// Everything below is serial and in book order: counts, cache writes and the
	// diff artifact are identical whatever order the batches finished in.
	type diffEntry struct {
		Src    string `json:"src"`
		Before string `json:"before"`
		After  string `json:"after"`
	}
	var (
		diffs     []diffEntry
		changed   int
		unchanged int
		keptIDs   int
		writeErrs int
	)
	for i, it := range items {
		text := repaired[i]
		if text == "" {
			text = it.tr
		}
		if kept[i] {
			keptIDs++
		}
		if text == it.tr {
			unchanged++
		} else {
			changed++
			diffs = append(diffs, diffEntry{Src: it.src, Before: it.tr, After: text})
		}
		// Written even when unchanged: --dst-cache must be a COMPLETE raw-
		// translation cache, or the following convert re-translates the gaps.
		if err := cache.Write(*dstCache, it.key, tbook.Translation{Text: text}); err != nil {
			writeErrs++
			lastErr = err
		}
	}
	copied, err := copyGlossaries(*srcCache, *dstCache)
	if err != nil {
		return err
	}

	fmt.Printf("\nRepair %s→%s via %s\n", cfg.Source, target, cfg.Model)
	fmt.Printf("  sentences:  %d unique, %d cached, %d sent in %d batches\n",
		len(seen), len(items), len(items), len(starts))
	fmt.Printf("  changed:    %d (%.1f%%)\n", changed, pct(changed, len(items)))
	fmt.Printf("  unchanged:  %d (%.1f%%), of which %d kept because the reply dropped the id\n",
		unchanged, pct(unchanged, len(items)), keptIDs)
	if failed > 0 {
		fmt.Printf("  WARNING: %d/%d batches failed after retries — their originals were kept (last error: %v)\n",
			failed, len(starts), lastErr)
	}
	if writeErrs > 0 {
		fmt.Printf("  WARNING: %d cache writes failed (last error: %v)\n", writeErrs, lastErr)
	}
	fmt.Printf("  wrote:      %d raw entries to %s (%d glossary cache file(s) copied)\n",
		len(items)-writeErrs, *dstCache, copied)
	fmt.Printf("  wall:       %s (%.1f sent/s), total %s\n",
		elapsed.Round(time.Second), float64(len(items))/elapsed.Seconds(),
		time.Since(runStart).Round(time.Second))
	if *diffPath != "" {
		b, err := jsonx.MarshalIndent(diffs, "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*diffPath, b, 0o644); err != nil {
			return fmt.Errorf("write --diff: %w", err)
		}
		fmt.Printf("  diff:       %d changed sentences → %s\n", len(diffs), *diffPath)
	}
	fmt.Printf("Next: convert %q --cache-dir %s -s %s -t %s (aligns the repaired text; no translation)\n",
		cfg.Input, *dstCache, cfg.Source, target)
	return nil
}

// repairInput is one sentence as sent to the proofreader: the source, its
// current translation, and the short per-batch id ("1".."n") the reply echoes —
// the same convention the translate/align/judge passes use, for the same reason
// (a cheap model mangles long ids, and ids are echoed twice per item).
type repairInput struct {
	ID   string     `json:"id"`
	Src  string     `json:"src"`
	Tr   string     `json:"tr"`
	Prev []prevItem `json:"prev,omitempty"`
}

// prevItem is a finished neighbouring sentence sent as read-only context. Input
// tokens are free on gonka, and the pass's residual losses were context-free
// guesses — a character's gender where the source is genderless, a pronoun's
// referent — which the preceding sentence usually settles.
type prevItem struct {
	Src string `json:"src"`
	Tr  string `json:"tr"`
}

// repairSystemPrompt is the proofread pass: fix what is WRONG in an existing
// translation (grammar, calqued idioms, register, fidelity) and leave everything
// else byte-identical. It never re-translates: the align pass downstream locates
// fragments inside this text, and a rewritten-to-taste sentence would make the
// baseline/repaired comparison measure two variables instead of one.
//
// The book glossary is appended when available: the first live run "fixed" the
// book's own term «оболочка» (a sleeve is a BODY here) into the literal «рукав»,
// because a proofreader that cannot see the enforced terminology treats a
// deliberate term as a mistranslation.
//
// fluent widens the mandate from hard errors to unidiomatic phrasing — measured
// separately, since freer edits trade fidelity and alignability for naturalness.
func repairSystemPrompt(sourceName, targetName string, glossary []translate.GlossEntry, fluent, bold bool, ctxN int) string {
	r := strings.NewReplacer("{SRC}", sourceName, "{TGT}", targetName)
	base := r.Replace(`You proofread {TGT} translations of {SRC} sentences for a language-learning reader: the app
shows the translation beside the original and highlights matching word pairs, so every
{TGT} word must be correct {TGT} that a learner can safely imitate.

You receive a JSON array of items {id, src, tr}: src is the {SRC} original, tr its {TGT} translation.

For EACH item, return tr FIXED — but ONLY where it is genuinely wrong:
- GRAMMAR: gender/number/case agreement, verb conjugation, verb and preposition government.
  A malformed or non-existent {TGT} word is always an error.
- CALQUES: a word-by-word rendering of a {SRC} idiom, phrasal verb, slang or fixed expression
  that is not real {TGT} — replace it with the natural {TGT} expression of the same meaning
  and the same register.
- REGISTER: slang stays slang, formal stays formal.
- FIDELITY: restore a dropped meaning element of src, delete anything invented, and never
  leave {SRC} words in the translation.`)
	if fluent {
		base += r.Replace(`
- PHRASING: where tr is grammatical but reads as translated-from-{SRC} rather than written
  in {TGT} — unidiomatic collocations, clumsy clause order, a stiff literal choice where
  {TGT} has an ordinary word — rewrite that part into what a {TGT} author would write.
  Keep the same meaning, the same register, and the same sentence boundaries.`)
	}
	base += r.Replace(`
Change NOTHING else: keep tr's wording, word order and style wherever it is already correct.
Do not merge or split sentences, do not alter names, numbers or quoted speech.`)
	if ctxN > 0 {
		base += r.Replace(`
Each item also carries "prev": the sentence(s) immediately before it, with their FINISHED {TGT}
translations. They are CONTEXT ONLY — already translated, already shipped. Never translate,
quote, merge or modify prev, and never return anything for it.
Use prev to settle what this sentence alone cannot: who is being spoken about and with which
gender, what a pronoun refers to, which {TGT} wording a recurring thing already got. Where prev
settles it and tr contradicts it, FIX tr. Where neither src, tr nor prev settles it, leave tr
exactly as it is — an unsupported change there is a guess, and guessing breaks the book.`)
	} else {
		base += r.Replace(`
You see ONE sentence at a time with no surrounding context: never "correct" the gender of a
person, the referent of a pronoun, or a recurring term just because it looks unexpected —
without the context that decided it, such a change is a guess, and guessing here breaks the
book.`)
	}
	if bold {
		// The default anchor ("most items need no change") caps the pass at ~2-4%
		// of sentences. Removing it raises recall; whether the extra edits are
		// improvements is exactly what the paired judging measures.
		base += r.Replace(`
Read every item as a reviewer who expects to find something: several items in a batch usually
carry a defect worth fixing. But an item that is already correct, faithful and idiomatic must
come back byte-identical — inventing a change there is itself an error.`)
	} else {
		base += ` MOST ITEMS NEED NO CHANGE — then return tr exactly as given.`
	}
	if len(glossary) > 0 {
		var sb strings.Builder
		sb.WriteString(base)
		sb.WriteString("\n\nGLOSSARY — these ")
		sb.WriteString(targetName)
		sb.WriteString(" renderings are enforced across the whole book. They are CORRECT by\n")
		sb.WriteString("decision, even where a different word looks more literal — never change them:\n")
		for _, e := range glossary {
			sb.WriteString("- ")
			sb.WriteString(e.Src)
			sb.WriteString(" → ")
			sb.WriteString(e.Tgt)
			sb.WriteString("\n")
		}
		base = strings.TrimRight(sb.String(), "\n")
	}
	return base + r.Replace(`

Reply with ONLY a single JSON object mapping each "id" (exact string) to the final {TGT}
sentence as a STRING. No code fences, no commentary. Output EVERY item.`)
}

// prevContext returns up to n sentences preceding index i, oldest first. The
// pass sees them read-only: they are already translated and aligned, and their
// only job is to settle what the sentence under repair cannot settle alone.
func prevContext(all []prevItem, i, n int) []prevItem {
	if n <= 0 || i == 0 {
		return nil
	}
	start := max(0, i-n)
	out := make([]prevItem, 0, i-start)
	out = append(out, all[start:i]...)
	return out
}

// srcGlossary loads the glossary the source cache was built with, so the
// proofreader is bound by the same enforced terminology the translator was.
// A missing or unreadable file is not fatal: the pass just runs unguarded.
func srcGlossary(dir string) []translate.GlossEntry {
	paths, err := filepath.Glob(filepath.Join(dir, "glossary-*.json"))
	if err != nil || len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	data, err := os.ReadFile(paths[0])
	if err != nil {
		return nil
	}
	var entries []translate.GlossEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

// bookSentences parses and segments the input exactly as convert does — same
// parse options, same --limit-chapters cut, same translatable set (prose +
// notes, citations unless --skip-citations) — so repair sees the sentences the
// cached run translated, no more and no fewer.
func bookSentences(cfg *config.Config) ([]*tbook.Sentence, error) {
	var (
		book *epub.Book
		err  error
	)
	if lower := strings.ToLower(cfg.Input); strings.HasSuffix(lower, ".fb2") ||
		strings.HasSuffix(lower, ".fb2.zip") {
		if book, err = fb2.Parse(cfg.Input); err != nil {
			return nil, fmt.Errorf("parse fb2: %w", err)
		}
	} else {
		book, err = epub.ParseOpts(cfg.Input, epub.Options{
			SkipMatter: !cfg.KeepMatter,
			SkipExtra:  cfg.SkipFiles,
			NoImages:   cfg.NoImages,
			NoNotes:    cfg.NoNotes,
		})
		if err != nil {
			return nil, fmt.Errorf("parse epub: %w", err)
		}
	}
	chapters := book.Chapters
	if cfg.LimitChapters > 0 && cfg.LimitChapters < len(chapters) {
		chapters = chapters[:cfg.LimitChapters]
	}
	_, sentences := segment.BuildSentenceObjects(chapters, cfg.Source)
	_, noteSents, citeSents := segment.BuildNotes(book.Notes, cfg.Source)
	fmt.Printf("Parsed %s — %q by %s, %d chapters, %d sentences + %d note sentences (%d in citations%s)\n",
		cfg.Input, book.Title, book.Author, len(chapters), len(sentences),
		len(noteSents)+len(citeSents), len(citeSents),
		map[bool]string{true: ", skipped", false: ""}[cfg.SkipCitations])

	out := append(append([]*tbook.Sentence{}, sentences...), noteSents...)
	if !cfg.SkipCitations {
		out = append(out, citeSents...)
	}
	return out, nil
}

// splitArgs peels this tool's own flags (repairFlagNames, all value-taking) out
// of the command line; the remainder goes to config.Load. Both "--flag v" and
// "--flag=v" are accepted.
func splitArgs(args []string) (mine, rest []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" { // end of flags: everything after is positional
			rest = append(rest, args[i:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' {
			rest = append(rest, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		val, hasVal := "", false
		if j := strings.IndexByte(name, '='); j >= 0 {
			name, val, hasVal = name[:j], name[j+1:], true
		}
		takesValue, mineFlag := repairFlagNames[name]
		if !mineFlag {
			rest = append(rest, a)
			continue
		}
		if hasVal {
			mine = append(mine, "-"+name+"="+val)
			continue
		}
		if !takesValue { // boolean: the next token belongs to someone else
			mine = append(mine, "-"+name)
			continue
		}
		if i+1 < len(args) {
			mine = append(mine, "-"+name, args[i+1])
			i++
		}
	}
	return mine, rest
}

// needKey turns a missing credential into an actionable error before the first
// batch, mirroring convert's checks for the two keyed backends.
func needKey(cfg *config.Config) error {
	if cfg.APIKey != "" {
		return nil
	}
	switch cfg.Provider {
	case config.ProviderGonka:
		return fmt.Errorf("GONKA_API_KEY is not set (get a key at https://proxy.gonka.gg and put it in converter/.env)")
	case config.ProviderOpenRouter:
		return fmt.Errorf("OPENROUTER_API_KEY is not set (put it in converter/.env — see .env.example)")
	}
	return nil // claude CLI / local servers need no key
}

// copyGlossaries copies the source cache's glossary entries into the
// destination. The glossary hash is part of the cache namespace, so a following
// convert run against --dst-cache must resolve the SAME glossary — with the file
// present it does so offline; without it, it would call the model, likely get a
// different hash, and miss every repaired entry. Existing files are left alone.
func copyGlossaries(srcDir, dstDir string) (int, error) {
	paths, err := filepath.Glob(filepath.Join(srcDir, "glossary-*.json"))
	if err != nil || len(paths) == 0 {
		return 0, err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return 0, err
	}
	copied := 0
	for _, p := range paths {
		dst := filepath.Join(dstDir, filepath.Base(p))
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return copied, err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return copied, err
		}
		copied++
	}
	return copied, nil
}

// glossaryNote reports what copyGlossaries would do, for --dry-run.
func glossaryNote(srcDir, dstDir string) string {
	paths, _ := filepath.Glob(filepath.Join(srcDir, "glossary-*.json"))
	n := 0
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(dstDir, filepath.Base(p))); err != nil {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(" and copy %d glossary cache file(s) so the namespace resolves offline there", n)
}

// abs resolves a cache path for the src/dst identity check; on failure the raw
// path is compared (a bad path fails later anyway).
func abs(path string) string {
	if p, err := filepath.Abs(path); err == nil {
		return p
	}
	return path
}

func pct(a, n int) float64 { return 100 * float64(a) / float64(max(1, n)) }
