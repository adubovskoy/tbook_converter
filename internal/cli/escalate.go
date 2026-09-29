package cli

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/embalign"
	"github.com/dimando/reader/converter/internal/lexcheck"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
)

// escalator redoes gate-flagged cells with --escalate-model and re-verifies
// them. A failed attempt never costs the book a cell: whatever escalation
// cannot deliver verified and aligned — a request error, no usable reply, text
// that will not align, output the gates still flag — goes back to the
// translation and alignment the cell shipped with before, recorded as
// unverified (flagged, not fixed).
type escalator struct {
	cfg        *config.Config
	glossaries []*glossaryNS
	esc        *escalationOutcome
	orig       map[string]map[string]cellSnapshot // target → src → state before escalation
	// emb supplies the local embedding aligner for escalated text the LLM
	// align pass left unaligned; nil, or a nil result, means unavailable.
	emb func() translate.WordAligner
}

// cellSnapshot is one cell as it stood before escalation: every cache entry
// assembly or a later run reads for it (nil = absent), and the cell itself.
type cellSnapshot struct {
	entries map[string]*tbook.Translation
	shipped tbook.Translation
	verdict *translate.Verdict // its failing judge verdict, if any
}

func newEscalator(cfg *config.Config, glossaries []*glossaryNS, subset []*tbook.Sentence,
	esc *escalationOutcome, emb func() translate.WordAligner) *escalator {

	e := &escalator{cfg: cfg, glossaries: glossaries, esc: esc, emb: emb,
		orig: map[string]map[string]cellSnapshot{}}
	for _, g := range glossaries {
		per := map[string]cellSnapshot{}
		for _, s := range subset {
			if _, dup := per[s.Src]; dup {
				continue
			}
			snap := cellSnapshot{entries: map[string]*tbook.Translation{}, shipped: s.Tr[g.target]}
			for _, k := range cellKeys(cfg, g, s.Src) {
				if tr, ok := cache.Read(cfg.CacheDir, k); ok {
					snap.entries[k] = &tr
				} else {
					snap.entries[k] = nil
				}
			}
			if cfg.Judge {
				if v, ok := translate.VerdictFor(cfg.CacheDir, cfg.JudgeModel, cfg.Source, g.target, s); ok && !v.OK {
					snap.verdict = &v
				}
			}
			per[s.Src] = snap
		}
		e.orig[g.target] = per
	}
	return e
}

// cellKeys lists the cache entries that make up one cell: raw text, proofread
// text (under --repair), and the aligned entry in both final namespaces.
func cellKeys(cfg *config.Config, g *glossaryNS, src string) []string {
	keys := []string{
		cache.TrKey(src, cfg.Source, g.target, g.cacheModel),
		cache.Key(src, cfg.Source, g.target, g.cacheModel),
	}
	if fm := finalModel(cfg, g.cacheModel); fm != g.cacheModel {
		keys = append(keys, cache.Key(src, cfg.Source, g.target, fm))
	}
	if rp := repairModel(cfg, g.cacheModel); rp != "" {
		keys = append(keys, cache.RepairKey(src, cfg.Source, g.target, rp))
	}
	return keys
}

// restore puts a cell back to its pre-escalation state, in the cache and in
// every sentence that shares its source. It reports false (and changes
// nothing) when the cell had no translation to go back to.
func (e *escalator) restore(target, src string, sentences []*tbook.Sentence) bool {
	snap, ok := e.orig[target][src]
	if !ok || snap.shipped.Text == "" {
		return false
	}
	for k, tr := range snap.entries {
		if tr == nil {
			cache.Remove(e.cfg.CacheDir, k)
		} else {
			_ = cache.Write(e.cfg.CacheDir, k, *tr)
		}
	}
	for _, s := range sentences {
		if s.Src == src {
			s.Tr[target] = snap.shipped
		}
	}
	return true
}

// giveUp restores a cell if it can and records it as flagged but not fixed,
// with the failing verdict of whatever now ships. Reports whether it restored.
func (e *escalator) giveUp(target, src string, sentences []*tbook.Sentence) bool {
	restored := e.restore(target, src, sentences)
	var v *translate.Verdict
	if restored {
		v = e.orig[target][src].verdict
	} else if e.cfg.Judge {
		for _, s := range sentences {
			if s.Src == src {
				if got, ok := translate.VerdictFor(e.cfg.CacheDir, e.cfg.JudgeModel, e.cfg.Source, target, s); ok && !got.OK {
					v = &got
				}
				break
			}
		}
	}
	e.esc.markUnverified(target, src, v)
	return restored
}

// run redoes a subset (translate + align) with the stronger model, overwriting
// its entries in the PRIMARY model's cache namespace, then refreshes it from
// cache. Escalated text that ends up with no alignment is aligned locally, and
// failing that goes back to its original. A request failure that stops the
// pipeline restores the whole subset and reports aborted; only a cancelled
// context is returned as an error.
func (e *escalator) run(ctx context.Context, subset []*tbook.Sentence) (aborted bool, err error) {
	cfg := e.cfg
	ec := translate.NewClient(clientOptionsVia(cfg, cfg.EscalateProvider, cfg.EscalateModel, cfg.Temperature))
	pipe := &translate.Pipeline{
		Client: ec, CacheDir: cfg.CacheDir, Source: cfg.Source,
		BatchSize: cfg.BatchSize, AlignBatch: cfg.AlignBatch,
		Concurrency: cfg.Concurrency,
		Force:       true, // redo even though (bad) entries are cached
	}
	found, missing, reverted := 0, 0, 0
	for _, g := range e.glossaries {
		pipe.Glossary, pipe.CacheModel = g.entries, g.cacheModel
		before := rawTexts(cfg, g, subset)
		if err := pipe.Run(ctx, subset, []string{g.target}); err != nil {
			if ctx.Err() != nil {
				return false, err
			}
			srcs := uniqueSrcs(subset)
			for _, g2 := range e.glossaries {
				for _, src := range srcs {
					e.giveUp(g2.target, src, subset)
				}
			}
			fmt.Printf("Escalation failed (%v) — %d sentences keep their original translation, "+
				"flagged as unverified.\n", err, len(srcs))
			return true, nil
		}
		changed := changedSrcs(cfg, g, subset, before)
		e.alignLocally(ctx, g, subsetBySrc(subset, changed))
		promoteEscalated(cfg, g, subset, before)
		f, m := translate.FillFromCache(subset, []string{g.target}, cfg.CacheDir, cfg.Source,
			finalModel(cfg, g.cacheModel), g.cacheModel, repairModel(cfg, g.cacheModel))
		found, missing = found+f, missing+m
		// Escalated text with no alignment would ship without highlights:
		// the original, aligned, is the better cell. Verification then sees
		// the original again and either retries or records it unverified.
		for _, s := range subsetBySrc(subset, changed) {
			if tr := s.Tr[g.target]; tr.Text != "" && len(tr.Align) > 0 {
				continue
			}
			if e.restore(g.target, s.Src, subset) {
				reverted++
			}
		}
	}
	fmt.Printf("Escalation done: %d refreshed (%d missing).\n", found, missing)
	if reverted > 0 {
		fmt.Printf("  %d escalated translations could not be aligned — kept the original translation.\n", reverted)
	}
	return false, nil
}

// alignLocally aligns escalated text the LLM align pass did not, with the
// local embedding aligner.
func (e *escalator) alignLocally(ctx context.Context, g *glossaryNS, set []*tbook.Sentence) {
	var need []*tbook.Sentence
	for _, s := range set {
		if _, ok := cache.Read(e.cfg.CacheDir, cache.Key(s.Src, e.cfg.Source, g.target, g.cacheModel)); !ok {
			need = append(need, s)
		}
	}
	if len(need) == 0 || e.emb == nil {
		return
	}
	a := e.emb()
	if a == nil {
		return
	}
	fmt.Printf("[%s] %d escalated sentences came back unaligned — aligning them locally ...\n", g.target, len(need))
	p := &translate.Pipeline{
		Client:   translate.NewClient(clientOptionsVia(e.cfg, e.cfg.EscalateProvider, e.cfg.EscalateModel, e.cfg.Temperature)),
		CacheDir: e.cfg.CacheDir, Source: e.cfg.Source, CacheModel: g.cacheModel,
		BatchSize: e.cfg.BatchSize, Concurrency: e.cfg.Concurrency,
		AlignMode: translate.AlignEmb, EmbAligner: a,
	}
	if err := p.Run(ctx, need, []string{g.target}); err != nil {
		fmt.Printf("[%s] local alignment failed: %v\n", g.target, err)
	}
}

// rawTexts snapshots the raw translations of a subset, to tell afterwards
// which ones the escalation actually replaced.
func rawTexts(cfg *config.Config, g *glossaryNS, subset []*tbook.Sentence) map[string]string {
	out := make(map[string]string, len(subset))
	for _, s := range subset {
		if tr, ok := cache.Read(cfg.CacheDir, cache.TrKey(s.Src, cfg.Source, g.target, g.cacheModel)); ok {
			out[s.Src] = tr.Text
		}
	}
	return out
}

// changedSrcs lists the sources whose raw translation the escalation replaced.
func changedSrcs(cfg *config.Config, g *glossaryNS, subset []*tbook.Sentence, before map[string]string) []string {
	var out []string
	for src, text := range rawTexts(cfg, g, subset) {
		if text != "" && text != before[src] {
			out = append(out, src)
		}
	}
	sort.Strings(out)
	return out
}

func uniqueSrcs(sentences []*tbook.Sentence) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range sentences {
		if !seen[s.Src] {
			seen[s.Src] = true
			out = append(out, s.Src)
		}
	}
	return out
}

// promoteEscalated makes an escalated translation the text that ships under
// --repair. The escalation pipeline writes the raw namespace, while assembly
// reads the proofread one first, so without this the old model's proofread
// text would keep shipping. The escalated text is final: it is not proofread
// again. Sentences whose raw text did not change (escalation failed) keep
// their proofread entries.
func promoteEscalated(cfg *config.Config, g *glossaryNS, subset []*tbook.Sentence, before map[string]string) {
	if !cfg.Repair {
		return
	}
	final, rpModel := finalModel(cfg, g.cacheModel), repairModel(cfg, g.cacheModel)
	for _, s := range subset {
		raw, ok := cache.Read(cfg.CacheDir, cache.TrKey(s.Src, cfg.Source, g.target, g.cacheModel))
		if !ok || raw.Text == "" || raw.Text == before[s.Src] {
			continue
		}
		_ = cache.Write(cfg.CacheDir, cache.RepairKey(s.Src, cfg.Source, g.target, rpModel), raw)
		finalKey := cache.Key(s.Src, cfg.Source, g.target, final)
		if tr, ok := cache.Read(cfg.CacheDir, cache.Key(s.Src, cfg.Source, g.target, g.cacheModel)); ok && tr.Text != "" {
			_ = cache.Write(cfg.CacheDir, finalKey, tr)
		} else {
			cache.Remove(cfg.CacheDir, finalKey) // unaligned: the caller reverts it
		}
	}
}

// escalate redoes the flagged sentences, re-verifies them and settles what
// still fails. See escalator.
func escalate(ctx context.Context, cfg *config.Config, glossaries []*glossaryNS,
	sentences []*tbook.Sentence, flagged []string, esc *escalationOutcome,
	emb func() translate.WordAligner) error {

	subset := subsetBySrc(sentences, flagged)
	if len(subset) == 0 {
		return nil
	}
	e := newEscalator(cfg, glossaries, subset, esc, emb)
	fmt.Printf("Escalating %d flagged sentences to %s ...\n", len(subset), cfg.EscalateModel)
	aborted, err := e.run(ctx, subset)
	if err != nil || aborted {
		return err
	}
	// A stronger model is not a verified model: re-run the same gates over
	// the escalated output, give it one redo, and fall back for whatever
	// still fails — never ship an unvetted rewrite.
	return e.reverify(ctx, subset)
}

// verify runs the gates (lexcheck + judge) over a set and returns the failing
// cells per target, plus their sources.
func (e *escalator) verify(ctx context.Context, set []*tbook.Sentence) (map[string]map[string]bool, []string, error) {
	cfg := e.cfg
	bad := map[string]map[string]bool{}
	flag := func(target, src string) {
		if bad[target] == nil {
			bad[target] = map[string]bool{}
		}
		bad[target][src] = true
	}
	if cfg.Lexcheck {
		for _, g := range e.glossaries {
			if dict, err := lexcheck.Load(cfg.LexiconDir, cfg.Source, g.target); err == nil && dict != nil {
				for _, s := range set {
					if dict.CheckSentence(s, g.target).Flagged {
						flag(g.target, s.Src)
					}
				}
			}
		}
	}
	if cfg.Judge {
		jc := translate.NewClient(clientOptionsVia(cfg, cfg.JudgeProvider, cfg.JudgeModel, 0))
		// No progress sink: re-verification covers a small escalated subset
		// and would only emit confusing extra "judge" phases.
		if _, err := translate.Judge(ctx, jc, cfg.CacheDir, cfg.Source, cfg.Targets,
			set, max(1, cfg.BatchSize/2), cfg.Concurrency, nil); err != nil {
			return nil, nil, err
		}
		for _, g := range e.glossaries {
			for _, s := range set {
				if v, ok := translate.VerdictFor(cfg.CacheDir, cfg.JudgeModel, cfg.Source, g.target, s); ok && !v.OK {
					flag(g.target, s.Src)
				}
			}
		}
	}
	seen := map[string]bool{}
	var srcs []string
	for _, per := range bad {
		for src := range per {
			if !seen[src] {
				seen[src] = true
				srcs = append(srcs, src)
			}
		}
	}
	sort.Strings(srcs)
	return bad, srcs, nil
}

// reverify closes the loop: the gates re-check the escalated output, anything
// still flagged gets ONE more escalation attempt, and whatever fails after that
// goes back to its original translation and alignment, recorded unverified.
func (e *escalator) reverify(ctx context.Context, subset []*tbook.Sentence) error {
	cfg := e.cfg
	if len(subset) == 0 || (!cfg.Lexcheck && !cfg.Judge) {
		return nil
	}
	fmt.Printf("Re-verifying %d escalated sentences ...\n", len(subset))
	bad, still, err := e.verify(ctx, subset)
	if err != nil {
		return err
	}
	if len(still) > 0 { // one redo, then re-check
		fmt.Printf("Re-escalating %d still-flagged sentences ...\n", len(still))
		aborted, err := e.run(ctx, subsetBySrc(subset, still))
		if err != nil || aborted {
			return err
		}
		if bad, still, err = e.verify(ctx, subsetBySrc(subset, still)); err != nil {
			return err
		}
	}
	if len(still) == 0 {
		fmt.Println("Escalation verified clean.")
		return nil
	}

	kept, escalated := 0, 0
	for _, g := range e.glossaries {
		srcs := make([]string, 0, len(bad[g.target]))
		for src := range bad[g.target] {
			srcs = append(srcs, src)
		}
		sort.Strings(srcs)
		for _, src := range srcs {
			if e.giveUp(g.target, src, subset) {
				kept++
			} else {
				escalated++ // nothing to go back to: the escalated text ships
			}
		}
	}
	path := cfg.Out + ".unverified.json"
	_ = translate.WriteFlagged(path, still)
	fmt.Printf("Escalation left %d sentences unverified (%d cells keep their original translation "+
		"and alignment, %d keep the escalated text) — listed in %s; re-run to retry them.\n",
		len(still), kept, escalated, path)
	return nil
}

// lazyAligner hands escalation the run's embedding aligner, or starts one on
// first use when the run had none (an offline re-assembly never starts it).
type lazyAligner struct {
	cfg     *config.Config
	pipe    *translate.Pipeline
	once    sync.Once
	started *embalign.Aligner
	a       translate.WordAligner
}

func (l *lazyAligner) get() translate.WordAligner {
	l.once.Do(func() {
		if l.pipe != nil && l.pipe.EmbAligner != nil {
			l.a = l.pipe.EmbAligner
			return
		}
		fmt.Printf("Starting embedding aligner for escalated sentences (%s, %s) ...\n", l.cfg.EmbModel, l.cfg.EmbMethod)
		a, err := embalign.Start(embalign.Options{
			Python: l.cfg.EmbPython, Script: l.cfg.EmbScript,
			Model: l.cfg.EmbModel, Layer: l.cfg.EmbLayer, Method: l.cfg.EmbMethod,
			SrcLang: l.cfg.Source,
		})
		if err != nil {
			fmt.Printf("Embedding aligner unavailable (%v) — unaligned escalations keep their original.\n", err)
			return
		}
		l.started, l.a = a, a
	})
	return l.a
}

func (l *lazyAligner) close() {
	if l.started != nil {
		_ = l.started.Close()
	}
}
