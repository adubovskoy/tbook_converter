package translate

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dimando/reader/converter/internal/align"
	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"
)

// maxRounds is the total number of translation passes per language: the first
// pass plus retries for sentences the model dropped (returned no chunks for).
// Transient HTTP failures are already retried inside Client.Translate.
const maxRounds = 3

// Pipeline translates sentences into the cache via OpenRouter.
type Pipeline struct {
	Client      *Client
	CacheDir    string
	Source      string
	BatchSize   int
	AlignBatch  int // align-pass batch size; 0 = max(1, BatchSize/4). Smaller batches measurably reduce positional drift.
	Concurrency int
	Force       bool // re-translate even cached sentences (overwrites cache)

	// Glossary, when non-empty, is appended to every translate prompt as
	// enforced terminology. CacheModel then carries a glossary-hash suffix so
	// glossary translations never mix with plain ones in the cache.
	Glossary   []GlossEntry
	CacheModel string // cache-key model component; empty = Client model
}

// cacheModel is the model string used in cache keys. CacheKeyModel builds the
// glossary-suffixed form shared by the pipeline and the offline cache fill.
func (p *Pipeline) cacheModel() string {
	if p.CacheModel != "" {
		return p.CacheModel
	}
	return p.Client.opts.Model
}

// CacheKeyModel returns the cache-key model component for a model and an
// optional glossary hash (empty hash = plain model).
func CacheKeyModel(model, glossaryHash string) string {
	if glossaryHash == "" {
		return model
	}
	return model + "+g:" + glossaryHash
}

type item struct {
	key string
	s   *tbook.Sentence
}

// translateInput / alignInput are one sentence as sent to the model, per phase.
// (No character offsets: the model echoes numbered source-word text; offsets
// are computed locally by align.BuildTextAlign.)
type translateInput struct {
	ID  string `json:"id"`
	Src string `json:"src"`
}

type alignInput struct {
	ID    string `json:"id"`
	Src   string `json:"src"`
	Words string `json:"words"` // numbered source words: "0:Los 1:hábitos …"
	Tr    string `json:"tr"`
}

// numberedWords renders a sentence's tokenized words as the numbered list the
// v5 align prompt expects.
func numberedWords(s *tbook.Sentence) string {
	runes := []rune(s.Src)
	var sb strings.Builder
	for i, w := range s.Words {
		a, b := w[0], w[1]
		if a < 0 || b > len(runes) || a >= b {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(strconv.Itoa(i))
		sb.WriteByte(':')
		sb.WriteString(string(runes[a:b]))
	}
	return sb.String()
}

// suspiciousTranslation reports model-output artifacts that must never reach
// the book: leaked special tokens (<bos>, <eos>, <|…|>, …) or the Unicode
// replacement character. Such a translation is dropped and retried.
var artifactRE = regexp.MustCompile(`(?i)<(bos|eos|pad|unk|s|/s|im_start|im_end)>|<\|[a-z_]+\|>|\x{FFFD}`)

func suspiciousTranslation(text string) bool {
	return artifactRE.MatchString(text)
}

// phase is one of the two production passes.
type phase int

const (
	phaseTranslate phase = iota
	phaseAlign
)

func (ph phase) String() string {
	if ph == phaseTranslate {
		return "translate"
	}
	return "align"
}

// Run translates every target language (hub-and-spoke: each target is
// translated from the source independently). Already-cached sentences are
// skipped, so the run is resumable and adding a language only does the new one.
func (p *Pipeline) Run(ctx context.Context, sentences []*tbook.Sentence, targets []string) error {
	for _, target := range targets {
		if err := p.runTarget(ctx, sentences, target); err != nil {
			return err
		}
	}
	return nil
}

// runTarget produces one target in TWO passes: translate everything pending,
// then align everything translated-but-unaligned. Decoupling avoids the
// positional drift a single translate+align pass falls into at batch scale.
func (p *Pipeline) runTarget(ctx context.Context, sentences []*tbook.Sentence, target string) error {
	needTr, needAl, nUnique, cached := p.pendingTwoPhase(sentences, target, p.Force)
	fmt.Printf("[%s] %d unique sentences — %d cached, %d to translate, %d to align\n",
		target, nUnique, cached, len(needTr), len(needAl))

	if len(needTr) > 0 {
		sys := translateSystemPrompt(LangName(p.Source), LangName(target), p.Glossary)
		if err := p.runPhase(ctx, sys, needTr, target, phaseTranslate); err != nil {
			return err
		}
	}
	// Sentences just translated are now alignable. Force must NOT propagate
	// here: force means "redo, don't trust the cache" — but the raw translation
	// was just rewritten, and keeping force on would put every sentence back in
	// the translate set and skip the align phase entirely (escalated sentences
	// then ship with no alignment at all).
	if _, needAl, _, _ = p.pendingTwoPhase(sentences, target, false); len(needAl) > 0 {
		sys := alignSystemPrompt(LangName(p.Source), LangName(target))
		if err := p.runPhase(ctx, sys, needAl, target, phaseAlign); err != nil {
			return err
		}
	}
	return nil
}

// pendingTwoPhase splits not-yet-final sentences (de-duped) into the translate
// set (no raw translation cached) and the align set (translated, not aligned).
// With force, every sentence goes to the translate set regardless of cache.
func (p *Pipeline) pendingTwoPhase(sentences []*tbook.Sentence, target string, force bool) (needTr, needAl []item, nUnique, cached int) {
	model := p.cacheModel()
	seen := map[string]bool{}
	for _, s := range sentences {
		key := cache.Key(s.Src, p.Source, target, model)
		if seen[key] {
			continue
		}
		seen[key] = true
		if !force {
			if fin, ok := cache.Read(p.CacheDir, key); ok {
				if suspiciousTranslation(fin.Text) {
					cache.Remove(p.CacheDir, key) // heal: artifact slipped into an earlier align
				} else {
					cached++
					continue
				}
			}
			trKey := cache.TrKey(s.Src, p.Source, target, model)
			if tr, ok := cache.Read(p.CacheDir, trKey); ok {
				if suspiciousTranslation(tr.Text) {
					// A leaked-token artifact slipped into an earlier run's raw
					// translation — drop it and re-translate.
					cache.Remove(p.CacheDir, trKey)
				} else {
					needAl = append(needAl, item{key: key, s: s})
					continue
				}
			}
		}
		needTr = append(needTr, item{key: key, s: s})
	}
	return needTr, needAl, len(seen), cached
}

// runPhase runs one phase's batches with the dropped-sentence retry loop. A
// sentence is "done" for the translate phase when its raw translation is cached,
// for the align phase when its final alignment is cached.
func (p *Pipeline) runPhase(ctx context.Context, system string, items []item, target string, ph phase) error {
	model := p.cacheModel()
	remaining := items
	for round := 0; round < maxRounds && len(remaining) > 0; round++ {
		if round > 0 {
			fmt.Printf("[%s] retrying %d sentences (%s, round %d/%d)\n",
				target, len(remaining), ph, round+1, maxRounds)
		}
		p.phaseBatches(ctx, system, remaining, target, ph)
		if err := ctx.Err(); err != nil {
			return err
		}
		var still []item
		for _, it := range remaining {
			doneKey := it.key
			if ph == phaseTranslate {
				doneKey = cache.TrKey(it.s.Src, p.Source, target, model)
			}
			if _, ok := cache.Read(p.CacheDir, doneKey); !ok {
				still = append(still, it)
			}
		}
		remaining = still
	}
	if len(remaining) > 0 {
		fmt.Printf("[%s] WARNING: %d sentences left at phase %s\n", target, len(remaining), ph)
	}
	return nil
}

// batchSizeFor returns the batch size for a phase: alignment runs in smaller
// batches (attention over many long alignment items is where cheap models
// drift), translation in full ones.
func (p *Pipeline) batchSizeFor(ph phase) int {
	if ph == phaseAlign {
		if p.AlignBatch > 0 {
			return p.AlignBatch
		}
		return max(1, p.BatchSize/4)
	}
	return p.BatchSize
}

// phaseBatches splits items into batches and runs them concurrently.
func (p *Pipeline) phaseBatches(ctx context.Context, system string, items []item, target string, ph phase) {
	size := p.batchSizeFor(ph)
	var batches [][]item
	for i := 0; i < len(items); i += size {
		end := min(i+size, len(items))
		batches = append(batches, items[i:end])
	}

	bar := progressbar.Default(int64(len(batches)), ph.String())
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(max(1, p.Concurrency))
	for _, batch := range batches {
		g.Go(func() error {
			p.doBatch(ctx, system, batch, target, ph) // failures left for the next round
			_ = bar.Add(1)
			return nil
		})
	}
	_ = g.Wait()
	_ = bar.Finish()
}

func (p *Pipeline) doBatch(ctx context.Context, system string, batch []item, target string, ph phase) {
	model := p.cacheModel()
	byKey := make(map[string]*tbook.Sentence, len(batch))
	for _, it := range batch {
		byKey[it.key] = it.s
	}

	if ph == phaseTranslate {
		inputs := make([]translateInput, len(batch))
		for i, it := range batch {
			inputs[i] = translateInput{ID: it.key, Src: it.s.Src}
		}
		userJSON, err := jsonx.Marshal(inputs)
		if err != nil {
			return
		}
		out, err := p.Client.TranslateText(ctx, system, string(userJSON))
		if err != nil {
			return
		}
		for id, text := range out {
			s, ok := byKey[id]
			if !ok {
				continue
			}
			if text = strings.TrimSpace(text); text == "" || suspiciousTranslation(text) {
				continue // dropped/artifact output — retried next round
			}
			_ = cache.Write(p.CacheDir, cache.TrKey(s.Src, p.Source, target, model),
				tbook.Translation{Text: text})
			// The final aligned entry is DERIVED from the raw translation just
			// replaced — drop it or the align phase would skip the sentence and
			// FillFromCache would keep serving the stale text (this is how
			// escalated sentences used to ship with the old model's content).
			cache.Remove(p.CacheDir, id)
		}
		return
	}

	// align phase
	inputs := make([]alignInput, len(batch))
	for i, it := range batch {
		tr, _ := cache.Read(p.CacheDir, cache.TrKey(it.s.Src, p.Source, target, model))
		inputs[i] = alignInput{ID: it.key, Src: it.s.Src, Words: numberedWords(it.s),
			Tr: ensureListPrefix(it.s.Src, tr.Text)}
	}
	userJSON, err := jsonx.Marshal(inputs)
	if err != nil {
		return
	}
	out, err := p.Client.Translate(ctx, system, string(userJSON))
	if err != nil {
		return
	}
	rawByKey := make(map[string]string, len(batch))
	for i, it := range batch {
		rawByKey[it.key] = inputs[i].Tr
	}
	for id, chunks := range out {
		s, ok := byKey[id]
		if !ok {
			continue
		}
		// The raw translation is the canonical text; the align output only
		// locates fragments inside it. An echo that diverged from the raw text
		// (unlocatable mapped fragment) returns the zero Translation — retried
		// next round; a sentence that never aligns still ships via the
		// raw-translation fallback in FillFromCache.
		tr := align.BuildTextAlign(chunks, s.Src, s.Words, rawByKey[id])
		if tr.Text == "" {
			continue
		}
		_ = cache.Write(p.CacheDir, id, tr)
	}
}

// listPrefixRE matches a leading list marker ("1. ", "12) ") in a source
// sentence.
var listPrefixRE = regexp.MustCompile(`^(\d{1,3}[.)])\s+`)

// ensureListPrefix restores a source sentence's leading list marker when the
// model dropped it from the translation — deterministic, so it is applied at
// raw-translation read time rather than cached.
func ensureListPrefix(src, text string) string {
	m := listPrefixRE.FindStringSubmatch(src)
	if m == nil || text == "" {
		return text
	}
	digits := strings.TrimRight(m[1], ".)")
	if strings.HasPrefix(text, digits) {
		return text
	}
	return m[1] + " " + text
}

// CountPending returns how many unique (sentence,target) translations are not
// yet cached — i.e. how much work needs the API. Zero means a fully offline
// assemble/resume is possible (no key required). When force is set every unique
// pair counts as pending, since cached entries will be re-translated.
func CountPending(sentences []*tbook.Sentence, targets []string, cacheDir, source, model string, force bool) int {
	seen := map[string]bool{}
	pending := 0
	for _, target := range targets {
		for _, s := range sentences {
			key := cache.Key(s.Src, source, target, model)
			if seen[key] {
				continue
			}
			seen[key] = true
			if force {
				pending++
				continue
			}
			if fin, ok := cache.Read(cacheDir, key); !ok || suspiciousTranslation(fin.Text) {
				pending++ // absent, or carries an artifact that healing will redo
			}
		}
	}
	return pending
}

// FillFromCache populates each sentence's tr[target] from the cache. A sentence
// with no final aligned entry falls back to its raw pass-1 translation with an
// empty alignment (translated, no highlights) — better than shipping it
// untranslated; sentences with neither get an empty translation. Returns the
// count of (sentence,target) pairs found and missing.
func FillFromCache(sentences []*tbook.Sentence, targets []string, cacheDir, source, model string) (found, missing int) {
	for _, s := range sentences {
		for _, target := range targets {
			key := cache.Key(s.Src, source, target, model)
			if tr, ok := cache.Read(cacheDir, key); ok && !suspiciousTranslation(tr.Text) {
				s.Tr[target] = tr
				found++
				continue
			}
			if raw, ok := cache.Read(cacheDir, cache.TrKey(s.Src, source, target, model)); ok &&
				raw.Text != "" && !suspiciousTranslation(raw.Text) {
				s.Tr[target] = tbook.Translation{Text: ensureListPrefix(s.Src, raw.Text), Align: []tbook.AlignChunk{}}
				found++
				continue
			}
			s.Tr[target] = tbook.Translation{Text: "", Align: []tbook.AlignChunk{}}
			missing++
		}
	}
	return found, missing
}
