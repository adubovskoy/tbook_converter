package translate

import (
	"context"
	"fmt"

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
	Concurrency int
}

type item struct {
	key string
	s   *tbook.Sentence
}

// batchInput is one sentence as sent to the model.
type batchInput struct {
	ID    string   `json:"id"`
	Src   string   `json:"src"`
	Words [][2]int `json:"words"`
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

func (p *Pipeline) runTarget(ctx context.Context, sentences []*tbook.Sentence, target string) error {
	model := p.Client.opts.Model
	seen := map[string]bool{}
	var todo []item
	cached := 0
	for _, s := range sentences {
		key := cache.Key(s.Src, p.Source, target, model)
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, ok := cache.Read(p.CacheDir, key); ok {
			cached++
			continue
		}
		todo = append(todo, item{key: key, s: s})
	}
	fmt.Printf("[%s] %d unique sentences — %d cached, %d to translate\n",
		target, len(seen), cached, len(todo))
	if len(todo) == 0 {
		return nil
	}

	system := systemPrompt(LangName(p.Source), LangName(target))
	remaining := todo
	for round := 0; round < maxRounds && len(remaining) > 0; round++ {
		if round > 0 {
			fmt.Printf("[%s] retrying %d sentences (round %d/%d)\n",
				target, len(remaining), round+1, maxRounds)
		}
		p.translateBatches(ctx, system, remaining)
		if err := ctx.Err(); err != nil {
			return err
		}
		// Recompute what's still missing (model may have dropped sentences).
		var still []item
		for _, it := range remaining {
			if _, ok := cache.Read(p.CacheDir, it.key); !ok {
				still = append(still, it)
			}
		}
		remaining = still
	}
	if len(remaining) > 0 {
		fmt.Printf("[%s] WARNING: %d sentences left untranslated (assembled empty)\n",
			target, len(remaining))
	}
	return nil
}

// translateBatches splits items into batches and runs them concurrently.
func (p *Pipeline) translateBatches(ctx context.Context, system string, items []item) {
	var batches [][]item
	for i := 0; i < len(items); i += p.BatchSize {
		end := min(i+p.BatchSize, len(items))
		batches = append(batches, items[i:end])
	}

	bar := progressbar.Default(int64(len(batches)), "translating")
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(max(1, p.Concurrency))
	for _, batch := range batches {
		g.Go(func() error {
			p.doBatch(ctx, system, batch) // failures left for the next round
			_ = bar.Add(1)
			return nil
		})
	}
	_ = g.Wait()
	_ = bar.Finish()
}

func (p *Pipeline) doBatch(ctx context.Context, system string, batch []item) {
	inputs := make([]batchInput, len(batch))
	byKey := make(map[string]*tbook.Sentence, len(batch))
	for i, it := range batch {
		inputs[i] = batchInput{ID: it.key, Src: it.s.Src, Words: it.s.Words}
		byKey[it.key] = it.s
	}
	userJSON, err := jsonx.Marshal(inputs)
	if err != nil {
		return
	}
	out, err := p.Client.Translate(ctx, system, string(userJSON))
	if err != nil {
		return
	}
	for id, chunks := range out {
		s, ok := byKey[id]
		if !ok {
			continue
		}
		tr := align.BuildTextAlign(chunks, s.Src, s.Words)
		if tr.Text == "" {
			continue
		}
		_ = cache.Write(p.CacheDir, id, tr)
	}
}

// CountPending returns how many unique (sentence,target) translations are not
// yet cached — i.e. how much work needs the API. Zero means a fully offline
// assemble/resume is possible (no key required).
func CountPending(sentences []*tbook.Sentence, targets []string, cacheDir, source, model string) int {
	seen := map[string]bool{}
	pending := 0
	for _, target := range targets {
		for _, s := range sentences {
			key := cache.Key(s.Src, source, target, model)
			if seen[key] {
				continue
			}
			seen[key] = true
			if _, ok := cache.Read(cacheDir, key); !ok {
				pending++
			}
		}
	}
	return pending
}

// FillFromCache populates each sentence's tr[target] from the cache. Sentences
// not present are given an empty translation (no highlight). Returns the count
// of (sentence,target) pairs found and missing.
func FillFromCache(sentences []*tbook.Sentence, targets []string, cacheDir, source, model string) (found, missing int) {
	for _, s := range sentences {
		for _, target := range targets {
			key := cache.Key(s.Src, source, target, model)
			if tr, ok := cache.Read(cacheDir, key); ok {
				s.Tr[target] = tr
				found++
			} else {
				s.Tr[target] = tbook.Translation{Text: "", Align: []tbook.AlignChunk{}}
				missing++
			}
		}
	}
	return found, missing
}
