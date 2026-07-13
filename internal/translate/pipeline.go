package translate

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dimando/reader/converter/internal/align"
	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/embalign"
	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/lexcheck"
	"github.com/dimando/reader/converter/internal/progress"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"
)

// maxRounds is the total number of translation passes per language: the first
// pass plus retries for sentences the model dropped (returned no chunks for).
// Transient HTTP failures are already retried inside Client.Translate.
const maxRounds = 3

// Alignment-pass modes. AlignEmb and AlignHybrid use the local embedding
// aligner (internal/embalign) instead of — or, for hybrid, in front of — the
// LLM align pass.
const (
	AlignLLM    = "llm"    // LLM align pass only (production default)
	AlignEmb    = "emb"    // embedding aligner only; no LLM fallback
	AlignHybrid = "hybrid" // embedding aligner, LLM pass for gated sentences
)

// DefaultEmbQMin is the hybrid-gate coverage threshold: an embedding
// alignment covering less than this fraction of translation words goes to the
// LLM align pass instead. Measured on an en→ru novel, 0.7 routes ~7% of
// sentences to the LLM (together with the lexcheck part of the gate).
const DefaultEmbQMin = 0.7

// WordAligner is the embedding aligner interface (satisfied by
// *embalign.Aligner): word strings in, [srcWordIdx, tgtWordIdx] pairs out.
type WordAligner interface {
	Align(srcWords, tgtWords []string) ([][2]int, error)
}

// BatchWordAligner is the optional batched fast path (satisfied by
// *embalign.Aligner): many sentence pairs per request, encoded by the child in
// one padded forward pass per side — ~2x faster than per-sentence on CPU. A
// nil per-item result marks that item's failure (gated like a low-Q alignment).
type BatchWordAligner interface {
	AlignBatch(srcs, tgts [][]string) ([][][2]int, error)
}

// embBatchSize is the embedding-aligner request size. The child encodes each
// request in length-sorted padded sub-batches of 32; a large request gives the
// sort enough spread to group similar lengths (measured ~1.9x over unsorted on
// real book sentences).
const embBatchSize = 256

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

	// Embedding alignment (see AlignLLM/AlignEmb/AlignHybrid). EmbAligner must
	// be non-nil for the emb/hybrid modes. LexDicts, when non-nil, supplies the
	// per-target lexcheck dictionary for the hybrid gate (nil dict = Q-only
	// gate). EmbQMin is the gate's coverage threshold (0 = DefaultEmbQMin).
	AlignMode  string
	EmbAligner WordAligner
	LexDicts   func(target string) *lexcheck.Dict
	EmbQMin    float64

	// Progress, when non-nil, receives machine-readable NDJSON progress
	// events (--progress-file) alongside the human bars.
	Progress *progress.Sink

	mu          sync.Mutex
	lastErr     error // most recent batch failure, for the leftovers warning
	totalSents  int
	cachedSents int
	okSents     int
	leftSents   int
	errSents    int
	progTarget  string // current target language, for progress events
	progTotal   int    // current phase's item count, for progress events
}

func (p *Pipeline) updateBarDescription(bar *progressbar.ProgressBar, name string) {
	if bar == nil {
		return
	}
	p.mu.Lock()
	width := len(strconv.Itoa(p.totalSents))
	if width < 1 {
		width = 1
	}
	format := fmt.Sprintf("%%-9s [ok:%%%dd/%%d | left:%%%dd | err:%%%dd]", width, width, width)
	desc := fmt.Sprintf(format, name, p.okSents, p.totalSents, p.leftSents, p.errSents)
	p.mu.Unlock()
	bar.Describe(desc)
}

func (p *Pipeline) addBatchResult(bar *progressbar.ProgressBar, name string, failed, succeeded int) {
	p.mu.Lock()
	p.okSents += succeeded
	p.leftSents -= (failed + succeeded)
	p.errSents += failed
	target, done, total := p.progTarget, p.progTotal-p.leftSents, p.progTotal
	p.mu.Unlock()
	p.updateBarDescription(bar, name)
	p.Progress.Update(name, target, done, total)
}

// noteErr records a batch failure so the "sentences left" warning can say why.
func (p *Pipeline) noteErr(err error) {
	p.mu.Lock()
	p.lastErr = err
	p.mu.Unlock()
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

	p.mu.Lock()
	p.totalSents = nUnique
	p.cachedSents = cached
	p.mu.Unlock()

	if len(needTr) > 0 {
		sys := translateSystemPrompt(LangName(p.Source), LangName(target), p.Glossary)
		t0 := time.Now()
		if err := p.runPhase(ctx, sys, needTr, target, phaseTranslate); err != nil {
			return err
		}
		el := time.Since(t0)
		fmt.Printf("[%s] translate phase: %d sentences in %s (%.1f sent/s)\n",
			target, len(needTr), el.Round(time.Second), float64(len(needTr))/el.Seconds())
	}
	// Sentences just translated are now alignable. Force must NOT propagate
	// here: force means "redo, don't trust the cache" — but the raw translation
	// was just rewritten, and keeping force on would put every sentence back in
	// the translate set and skip the align phase entirely (escalated sentences
	// then ship with no alignment at all).
	if _, needAl, _, _ = p.pendingTwoPhase(sentences, target, false); len(needAl) > 0 {
		if p.EmbAligner != nil && (p.AlignMode == AlignEmb || p.AlignMode == AlignHybrid) {
			needAl = p.embedAlign(ctx, needAl, target)
		}
		if len(needAl) > 0 {
			sys := alignSystemPrompt(LangName(p.Source), LangName(target))
			t0 := time.Now()
			if err := p.runPhase(ctx, sys, needAl, target, phaseAlign); err != nil {
				return err
			}
			el := time.Since(t0)
			fmt.Printf("[%s] LLM align phase: %d sentences in %s (%.1f sent/s)\n",
				target, len(needAl), el.Round(time.Second), float64(len(needAl))/el.Seconds())
		}
	}
	return nil
}

// embedAlign aligns pending sentences with the local embedding aligner,
// writing final cache entries indistinguishable from LLM-aligned ones (same
// key, same shape — FillFromCache, judge and escalation are agnostic). In
// hybrid mode, sentences the gate rejects (lexcheck flag or coverage below
// EmbQMin) are returned for the LLM align pass; in emb mode nothing is
// returned and gate-rejected sentences ship as aligned-by-embedding anyway.
func (p *Pipeline) embedAlign(ctx context.Context, items []item, target string) (leftover []item) {
	model := p.cacheModel()
	var dict *lexcheck.Dict
	if p.LexDicts != nil {
		dict = p.LexDicts(target)
	}
	bar := progressbar.Default(int64(len(items)), "embalign")
	aligned, gated, done := 0, 0, 0
	step := func() { // one item processed: human bar + machine progress
		_ = bar.Add(1)
		done++
		p.Progress.Update("embalign", target, done, len(items))
	}
	t0 := time.Now()

	type prep struct {
		it      item
		text    string
		trWords [][2]int
	}
	batcher, canBatch := p.EmbAligner.(BatchWordAligner)
	dead := false
	next := 0 // first item not yet processed (for the dead-aligner tail)

	for start := 0; start < len(items) && !dead && ctx.Err() == nil; start += embBatchSize {
		end := min(start+embBatchSize, len(items))
		next = end
		preps := make([]prep, 0, end-start)
		for _, it := range items[start:end] {
			raw, ok := cache.Read(p.CacheDir, cache.TrKey(it.s.Src, p.Source, target, model))
			if !ok || strings.TrimSpace(raw.Text) == "" {
				step()
				continue // no raw translation to align; ships via the raw fallback
			}
			text := ensureListPrefix(it.s.Src, raw.Text)
			preps = append(preps, prep{it: it, text: text, trWords: segment.Tokenize(text)})
		}
		if len(preps) == 0 {
			continue
		}
		srcs := make([][]string, len(preps))
		tgts := make([][]string, len(preps))
		for i, pr := range preps {
			srcs[i] = embalign.WordStrings(pr.it.s.Src, pr.it.s.Words)
			tgts[i] = embalign.WordStrings(pr.text, pr.trWords)
		}

		// Batched fast path; per-sentence fallback covers non-batch aligners and
		// a child that rejects the batch protocol (old script). A nil per-item
		// result — failure or death mid-chunk — gates that sentence.
		var results [][][2]int
		if canBatch {
			res, err := batcher.AlignBatch(srcs, tgts)
			switch {
			case err == nil:
				results = res
			case errors.Is(err, embalign.ErrDead):
				dead = true
			default:
				canBatch = false // protocol failure: stop batching, go per-sentence
			}
		}
		if results == nil && !dead {
			results = make([][][2]int, len(preps))
			for i := range preps {
				pairs, err := p.EmbAligner.Align(srcs[i], tgts[i])
				if err != nil {
					if errors.Is(err, embalign.ErrDead) {
						dead = true
						break
					}
					continue // per-sentence failure: results[i] stays nil → gated
				}
				if pairs == nil {
					pairs = [][2]int{}
				}
				results[i] = pairs
			}
		}

		for i, pr := range preps {
			var pairs [][2]int
			if results != nil {
				pairs = results[i]
			}
			if pairs == nil { // failed or unprocessed: gate like a low-Q alignment
				if p.AlignMode == AlignHybrid {
					leftover = append(leftover, pr.it)
					gated++
				}
				step()
				continue
			}
			chunks, q := embalign.Chunks(pairs, pr.trWords)
			tr := tbook.Translation{Text: pr.text, Align: chunks, Q: q}
			if p.AlignMode == AlignHybrid && p.rejectEmbedded(dict, pr.it.s, tr, target) {
				leftover = append(leftover, pr.it)
				gated++
				step()
				continue
			}
			_ = cache.Write(p.CacheDir, pr.it.key, tr)
			aligned++
			step()
		}
	}
	if dead && next < len(items) {
		fmt.Printf("[%s] embedding aligner died — %d sentences fall back to the LLM align pass\n",
			target, len(items)-next)
		if p.AlignMode == AlignHybrid {
			leftover = append(leftover, items[next:]...)
		}
	}
	_ = bar.Finish()
	p.Progress.Final("embalign", target, done, len(items))
	el := time.Since(t0)
	fmt.Printf("[%s] embedding-aligned %d sentences locally; %d gated to the LLM align pass (%s, %.1f sent/s)\n",
		target, aligned, gated, el.Round(time.Second), float64(len(items))/el.Seconds())
	return leftover
}

// rejectEmbedded is the hybrid gate: too-low alignment coverage, or a
// lexcheck flag (drift/wrong-word evidence), sends the sentence to the LLM
// align pass instead.
func (p *Pipeline) rejectEmbedded(dict *lexcheck.Dict, s *tbook.Sentence, tr tbook.Translation, target string) bool {
	qmin := p.EmbQMin
	if qmin == 0 {
		qmin = DefaultEmbQMin
	}
	if tr.Q < qmin {
		return true
	}
	if dict != nil {
		tmp := &tbook.Sentence{Src: s.Src, Words: s.Words, Tr: map[string]tbook.Translation{target: tr}}
		if dict.CheckSentence(tmp, target).Flagged {
			return true
		}
	}
	return false
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
	p.mu.Lock()
	p.progTarget, p.progTotal = target, len(items)
	p.mu.Unlock()
	// The phase's closing progress line is guaranteed even when retries leave
	// sentences behind (done < total then; the leftovers warning explains why).
	defer func() { p.Progress.Final(ph.String(), target, len(items)-len(remaining), len(items)) }()
	for round := 0; round < maxRounds && len(remaining) > 0; round++ {
		p.mu.Lock()
		p.leftSents = len(remaining)
		p.errSents = 0
		p.okSents = p.totalSents - len(remaining)
		p.mu.Unlock()

		if round > 0 {
			fmt.Printf("[%s] retrying %d sentences (%s, round %d/%d)\n",
				target, len(remaining), ph, round+1, maxRounds)
		}
		if err := p.phaseBatches(ctx, system, remaining, target, ph); err != nil {
			return err
		}
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
		why := ""
		p.mu.Lock()
		if p.lastErr != nil {
			why = fmt.Sprintf(" (last error: %v)", p.lastErr)
		}
		p.mu.Unlock()
		fmt.Printf("[%s] WARNING: %d sentences left at phase %s%s\n", target, len(remaining), ph, why)
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

// phaseBatches splits items into batches and runs them concurrently. Batch
// failures are left for the next round — except a subscription usage limit,
// which cancels the remaining batches and aborts the run (waiting out a
// multi-hour window inside the process helps nobody; the cache resumes).
func (p *Pipeline) phaseBatches(ctx context.Context, system string, items []item, target string, ph phase) error {
	size := p.batchSizeFor(ph)
	var batches [][]item
	for i := 0; i < len(items); i += size {
		end := min(i+size, len(items))
		batches = append(batches, items[i:end])
	}

	bar := progressbar.Default(int64(len(batches)), ph.String())
	p.updateBarDescription(bar, ph.String())
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(max(1, p.Concurrency))
	for _, batch := range batches {
		batch := batch
		g.Go(func() error {
			err := p.doBatch(ctx, system, batch, target, ph, bar)
			_ = bar.Add(1)
			return err
		})
	}
	err := g.Wait()
	_ = bar.Finish()
	return err
}

// doBatch runs one batch. Its returned error is non-nil ONLY for failures that
// must abort the run (usage limit) — ordinary batch failures return nil and
// are retried by the round loop.
//
// Items are sent under short per-batch ids ("1".."n"), never the 64-hex cache
// keys: a cheap model reliably echoes short ids where long ones get mangled or
// dropped (the judge learned this first), and ids are echoed in BOTH request
// and reply — at batch 16 the hex keys alone were ~30% of translate tokens.
func (p *Pipeline) doBatch(ctx context.Context, system string, batch []item, target string, ph phase, bar *progressbar.ProgressBar) error {
	model := p.cacheModel()
	ctx = WithPhase(ctx, ph.String())
	// Progress-bar statistics: a sentence counts as done when its phase's cache
	// entry exists (the same check the round loop uses).
	defer func() {
		successCount := 0
		for _, it := range batch {
			doneKey := it.key
			if ph == phaseTranslate {
				doneKey = cache.TrKey(it.s.Src, p.Source, target, model)
			}
			if _, ok := cache.Read(p.CacheDir, doneKey); ok {
				successCount++
			}
		}
		failCount := len(batch) - successCount
		p.addBatchResult(bar, ph.String(), failCount, successCount)
	}()

	// byID resolves an echoed short id back to the batch item.
	byID := func(id string) (item, bool) {
		n, err := strconv.Atoi(strings.TrimSpace(id))
		if err != nil || n < 1 || n > len(batch) {
			return item{}, false
		}
		return batch[n-1], true
	}

	if ph == phaseTranslate {
		inputs := make([]translateInput, len(batch))
		for i, it := range batch {
			inputs[i] = translateInput{ID: strconv.Itoa(i + 1), Src: it.s.Src}
		}
		userJSON, err := jsonx.Marshal(inputs)
		if err != nil {
			return nil
		}
		out, err := p.Client.TranslateText(ctx, system, string(userJSON))
		if err != nil {
			p.noteErr(err)
			return abortOnly(err)
		}
		for id, text := range out {
			it, ok := byID(id)
			if !ok {
				continue
			}
			if text = strings.TrimSpace(text); text == "" || suspiciousTranslation(text) {
				continue // dropped/artifact output — retried next round
			}
			_ = cache.Write(p.CacheDir, cache.TrKey(it.s.Src, p.Source, target, model),
				tbook.Translation{Text: text})
			// The final aligned entry is DERIVED from the raw translation just
			// replaced — drop it or the align phase would skip the sentence and
			// FillFromCache would keep serving the stale text (this is how
			// escalated sentences used to ship with the old model's content).
			cache.Remove(p.CacheDir, it.key)
		}
		return nil
	}

	// align phase
	inputs := make([]alignInput, len(batch))
	for i, it := range batch {
		tr, _ := cache.Read(p.CacheDir, cache.TrKey(it.s.Src, p.Source, target, model))
		inputs[i] = alignInput{ID: strconv.Itoa(i + 1), Src: it.s.Src, Words: numberedWords(it.s),
			Tr: ensureListPrefix(it.s.Src, tr.Text)}
	}
	userJSON, err := jsonx.Marshal(inputs)
	if err != nil {
		return nil
	}
	out, err := p.Client.Translate(ctx, system, string(userJSON))
	if err != nil {
		p.noteErr(err)
		return abortOnly(err)
	}
	for id, chunks := range out {
		it, ok := byID(id)
		if !ok {
			continue
		}
		// The raw translation is the canonical text; the align output only
		// locates fragments inside it. An echo that diverged from the raw text
		// (unlocatable mapped fragment) returns the zero Translation — retried
		// next round; a sentence that never aligns still ships via the
		// raw-translation fallback in FillFromCache.
		n, _ := strconv.Atoi(strings.TrimSpace(id))
		tr := align.BuildTextAlign(chunks, it.s.Src, it.s.Words, inputs[n-1].Tr)
		if tr.Text == "" {
			continue
		}
		_ = cache.Write(p.CacheDir, it.key, tr)
	}
	return nil
}

// abortOnly keeps only the errors that must abort the whole run: a
// subscription usage limit, or a permanent CLI failure (bad model, logged
// out) that would fail every remaining batch identically. Anything else is an
// ordinary batch failure the round loop retries, reported as nil.
func abortOnly(err error) error {
	if _, ok := errors.AsType[*UsageLimitError](err); ok {
		return err
	}
	if ce, ok := errors.AsType[*cliError](err); ok && ce.perm {
		return err
	}
	return nil
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

// CountPendingTranslate returns how many unique (sentence,target) pairs have
// no cached RAW translation — the work that strictly needs the LLM. With
// align-mode emb the align phase is local, so a run whose translate phase is
// clean (e.g. re-aligning after a PromptVersion bump) needs no API at all.
func CountPendingTranslate(sentences []*tbook.Sentence, targets []string, cacheDir, source, model string, force bool) int {
	seen := map[string]bool{}
	pending := 0
	for _, target := range targets {
		for _, s := range sentences {
			key := cache.TrKey(s.Src, source, target, model)
			if seen[key] {
				continue
			}
			seen[key] = true
			if force {
				pending++
				continue
			}
			if tr, ok := cache.Read(cacheDir, key); !ok || suspiciousTranslation(tr.Text) {
				pending++
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
