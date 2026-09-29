// Command gonka-probe measures what the gonka gateway serves today and how
// reliably it serves it — over the converter's own translate path, so the
// numbers describe production and not a bespoke HTTP client.
//
// It lists the served models (/v1/models), then for each model and each
// concurrency level sends real translate batches (prompt v5, batch 8) built
// from a fixed sentence set, recording latency, status, finish_reason, output
// tokens, error class and a script-sanity check on the returned target text.
// It also measures what reasoning costs, by sending one batch with gonka's
// "thinking":{"type":"disabled"} switch and one without it.
//
//	gonka-probe [--models a,b] [--concurrency 4,8,16,32] [--sentences 96]
//	            [--batch 8] [-t ru] [--book B.tbook --chapter chapters/ch3.json]
//	            [--max-retries 0] [--timeout 120s] [--out FILE]
//	            [--dump-dir DIR] [--raw-dir DIR] [--variants [--variant-sentences 2]]
//	gonka-probe --summarize FILE... | DIR...
//
// --dump-dir writes each cell's returned translations as {src,tr} JSONL, for a
// script-sanity read of the output. --variants replaces the cell sweep with the
// reasoning-off body-variant matrix (variants.go): one batch per (model, body
// shape), reading the reasoning fields the production client discards.
// --summarize sends nothing at all: it re-renders the report tables from
// artifacts and dumps already on disk (report.go), so every table in the
// report can be regenerated without paying for another sweep.
//
// Probe defaults differ from production on purpose: --max-retries 0 (failures
// must be visible, not retried away) and a 120 s request timeout. Every
// request carries a per-cell "user" tag, because the gateway returns a cached
// byte-identical reply to an identical body and would otherwise report a
// second cell's latency as ~0.2 s.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
)

// repoAssumed are the gonka model ids the repo and its reports assume exist;
// they are probed even when /v1/models does not list them.
var repoAssumed = []string{
	"moonshotai/Kimi-K2.6",
	"deepseek-ai/DeepSeek-V4-Flash-0731",
	"MiniMaxAI/MiniMax-M2.7",
}

// translateInput mirrors the pass-1 wire item ({id, src}) so the probe sends
// byte-identical user payloads.
type translateInput struct {
	ID  string `json:"id"`
	Src string `json:"src"`
}

type modelInfo struct {
	ID      string `json:"id"`
	CtxLen  int    `json:"context_length,omitempty"`
	MaxToks int    `json:"max_tokens,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
	Listed  bool   `json:"listed"`
	Probed  bool   `json:"probed"`
}

type shot struct {
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latency_ms"`
	OutputTok int    `json:"completion_tokens,omitempty"`
	Think     bool   `json:"think,omitempty"`
	Err       string `json:"err,omitempty"`
}

type reasoningProbe struct {
	Model     string `json:"model"`
	Sentences int    `json:"sentences"`
	Disabled  shot   `json:"thinking_disabled"`
	Omitted   shot   `json:"thinking_omitted"`
}

type example struct {
	Model       string   `json:"model"`
	Concurrency int      `json:"concurrency"`
	Kind        string   `json:"kind"`
	Src         string   `json:"src"`
	Out         string   `json:"out"`
	Leaks       []string `json:"leaks,omitempty"`
}

type artifact struct {
	Tool          string           `json:"tool"`
	Run           string           `json:"run"`
	StartedAt     string           `json:"started_at"`
	FinishedAt    string           `json:"finished_at"`
	Provider      string           `json:"provider"`
	BaseURL       string           `json:"base_url"`
	Source        string           `json:"source"`
	Target        string           `json:"target"`
	Book          string           `json:"book"`
	Chapter       string           `json:"chapter"`
	SentenceCount int              `json:"sentence_count"`
	BatchSize     int              `json:"batch_size"`
	Temperature   float64          `json:"temperature"`
	MaxRetries    int              `json:"max_retries"`
	TimeoutS      float64          `json:"timeout_s"`
	PromptVersion string           `json:"translate_prompt_version"`
	ServedErr     string           `json:"served_err,omitempty"`
	Models        []modelInfo      `json:"models"`
	Reasoning     []reasoningProbe `json:"reasoning"`
	Variants      []variantResult  `json:"variants,omitempty"`
	Cells         []cellSummary    `json:"cells"`
	Requests      []reqRow         `json:"requests"`
	Batches       []batchRow       `json:"batches"`
	Examples      []example        `json:"examples"`
}

type prober struct {
	baseURL, apiKey string
	source, target  string
	sentences       []string
	batchSize       int
	temperature     float64
	maxRetries      int
	timeout         time.Duration
	failStreak      int
	run             string
	tmp             string
	dumpDir         string
	rawDir          string
	cyrTarget       bool

	mu       sync.Mutex
	requests []reqRow
	batches  []batchRow
	examples []example
}

func main() {
	modelsFlag := flag.String("models", "", "comma-separated model ids (default: every served model plus the ids the repo assumes)")
	concFlag := flag.String("concurrency", "4,8,16,32", "comma-separated concurrency levels")
	nSent := flag.Int("sentences", 96, "sentences per (model, concurrency) cell")
	batchSize := flag.Int("batch", 8, "sentences per request (gonka production default)")
	source := flag.String("s", "en", "source language")
	target := flag.String("t", "ru", "target language")
	book := flag.String("book", "../android/app/src/main/assets/sample.tbook", "source .tbook for the sentence set")
	chapter := flag.String("chapter", "chapters/ch3.json", "chapter entry to draw sentences from")
	minWords := flag.Int("min-words", 6, "skip sentences shorter than this")
	outPath := flag.String("out", "", "artifact path (default bench-quality/.artifacts/gonka-probe-<date>.json)")
	timeout := flag.Duration("timeout", 120*time.Second, "hard per-request timeout")
	maxRetries := flag.Int("max-retries", 0, "retries per request (0: failures stay visible)")
	failStreak := flag.Int("fail-streak", 6, "consecutive failures that abort a cell as unservable")
	runLabel := flag.String("run", "", "run label; also the cache-busting request tag (default: hhmmss)")
	temperature := flag.Float64("temperature", 0.3, "sampling temperature (production default)")
	noReasoning := flag.Bool("no-reasoning-probe", false, "skip the reasoning-disable probe")
	variantsOnly := flag.Bool("variants", false, "run the reasoning-off body-variant matrix instead of the cell sweep")
	variantSents := flag.Int("variant-sentences", 2, "sentences per body-variant request")
	variantMatch := flag.String("variant-match", "", "with --variants: only run variants whose name contains this")
	dumpDir := flag.String("dump-dir", "", "write each cell's returned translations as {src,tr} JSONL into this directory")
	rawDir := flag.String("raw-dir", "", "write every raw reply as JSONL into this directory, so an unparseable one can be quoted")
	summarize := flag.Bool("summarize", false, "re-render the report tables from saved artifacts/dumps named as arguments; sends nothing")
	flag.Parse()

	// --summarize is offline: no credential, no gateway, no traffic.
	if *summarize {
		if err := summarizeFiles(flag.Args()); err != nil {
			fatal("summarize: %v", err)
		}
		return
	}

	_ = godotenv.Load()
	baseURL := strings.TrimRight(envOr("GONKA_BASE_URL", "https://proxy.gonka.gg/v1"), "/")
	apiKey := os.Getenv("GONKA_API_KEY")
	if apiKey == "" {
		fatal("GONKA_API_KEY is not set (put it in .env next to the binary's working directory)")
	}
	if *runLabel == "" {
		*runLabel = time.Now().Format("150405")
	}

	sents, err := loadSentences(*book, *chapter, *nSent, *minWords)
	if err != nil {
		fatal("sentence set: %v", err)
	}

	tmp, err := os.MkdirTemp("", "gonka-probe-")
	if err != nil {
		fatal("temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	p := &prober{
		baseURL: baseURL, apiKey: apiKey, source: *source, target: *target,
		sentences: sents, batchSize: *batchSize, temperature: *temperature,
		maxRetries: *maxRetries, timeout: *timeout, failStreak: *failStreak,
		run: *runLabel, tmp: tmp, dumpDir: *dumpDir, rawDir: *rawDir, cyrTarget: cyrillicTarget(*target),
	}

	art := artifact{
		Tool: "gonka-probe", Run: *runLabel, StartedAt: nowTS(),
		Provider: translate.ProviderGonka, BaseURL: baseURL,
		Source: *source, Target: *target, Book: *book, Chapter: *chapter,
		SentenceCount: len(sents), BatchSize: *batchSize, Temperature: *temperature,
		MaxRetries: *maxRetries, TimeoutS: timeout.Seconds(),
		PromptVersion: cache.TrPromptVersion,
	}

	// 1. What the gateway says it serves. ServedModels is the production call;
	// the raw list adds the context/max-token columns it does not carry.
	served, err := translate.ServedModels(baseURL, apiKey)
	if err != nil {
		art.ServedErr = err.Error()
		fmt.Printf("/v1/models FAILED: %v\n", err)
	}
	details := modelDetails(baseURL, apiKey)
	models := chooseModels(*modelsFlag, served)
	for _, id := range served {
		d := details[id]
		d.ID, d.Listed, d.Probed = id, true, contains(models, id)
		art.Models = append(art.Models, d)
	}
	for _, id := range models {
		if !contains(served, id) {
			art.Models = append(art.Models, modelInfo{ID: id, Probed: true})
		}
	}
	fmt.Printf("gateway %s — %d model(s) listed:\n", baseURL, len(served))
	for _, m := range art.Models {
		mark := " "
		if !m.Listed {
			mark = "!"
		}
		fmt.Printf("  %s %-40s ctx=%-8s max_tokens=%-6s listed=%v probed=%v\n",
			mark, m.ID, num(m.CtxLen), num(m.MaxToks), m.Listed, m.Probed)
	}
	fmt.Printf("sentence set: %d sentences from %s (%s), batch %d, prompt %s, target %s\n\n",
		len(sents), *chapter, filepath.Base(*book), *batchSize, cache.TrPromptVersion, *target)

	ctx := context.Background()
	levels := parseInts(*concFlag)

	if *variantsOnly {
		fmt.Printf("reasoning-off body variants, %d sentences per request:\n", *variantSents)
		for _, model := range models {
			fmt.Printf("%s\n", model)
			art.Variants = append(art.Variants, p.variantSweep(ctx, []string{model}, *variantSents, *variantMatch)...)
		}
		art.FinishedAt = nowTS()
		if err := writeArtifact(artifactPath(*outPath), art); err != nil {
			fatal("write artifact: %v", err)
		}
		fmt.Println(renderVariantTable(art.Variants))
		fmt.Printf("\nartifact: %s (%d variant requests)\n", artifactPath(*outPath), len(art.Variants))
		return
	}

	for _, model := range models {
		if !*noReasoning {
			rp := p.reasoningProbe(ctx, model)
			art.Reasoning = append(art.Reasoning, rp)
			fmt.Printf("%s reasoning probe: disabled=%s omitted=%s\n", model, fmtShot(rp.Disabled), fmtShot(rp.Omitted))
		}
		for i, conc := range levels {
			c := p.runCell(ctx, model, conc)
			art.Cells = append(art.Cells, c)
			fmt.Printf("%s c=%-2d %s\n", model, conc, fmtCell(c))
			if c.Unservable && i == 0 {
				fmt.Printf("%s unservable at the lowest concurrency — skipping higher levels\n", model)
				break
			}
		}
		fmt.Println()
	}

	art.FinishedAt = nowTS()
	p.mu.Lock()
	art.Requests, art.Batches, art.Examples = p.requests, p.batches, p.examples
	p.mu.Unlock()

	out := artifactPath(*outPath)
	if err := writeArtifact(out, art); err != nil {
		fatal("write artifact: %v", err)
	}
	fmt.Println(renderTable(art.Cells))
	fmt.Printf("\nartifact: %s (%d requests, %d batches)\n", out, len(art.Requests), len(art.Batches))
}

// runCell sends the whole sentence set at one concurrency level, aborting
// early once failStreak requests have failed in a row.
func (p *prober) runCell(ctx context.Context, model string, conc int) cellSummary {
	tag := fmt.Sprintf("probe-%s-%s-b%d-c%d", p.run, shortID(model), p.batchSize, conc)
	statsPath := filepath.Join(p.tmp, tag+".jsonl")
	st, err := translate.OpenStats(statsPath)
	if err != nil {
		fatal("stats sink: %v", err)
	}

	var mu sync.Mutex
	var replies, think, streak int
	var raw []rawReply
	client := translate.NewClient(translate.Options{
		Provider: translate.ProviderGonka,
		BaseURL:  p.baseURL, APIKey: p.apiKey, Model: model,
		Temperature: p.temperature, JSONMode: true,
		MaxRetries: p.maxRetries, Timeout: p.timeout,
		Stats: st, UserTag: tag,
		OnContent: func(c string) {
			mu.Lock()
			replies++
			if hasThink(c) {
				think++
			}
			if p.rawDir != "" {
				raw = append(raw, rawReply{Model: model, Conc: conc, Run: p.run, N: replies, Chars: len(c), Content: c})
			}
			mu.Unlock()
		},
	})
	sys := translate.TranslateSystemPrompt(translate.LangName(p.source), translate.LangName(p.target), nil)

	cellCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	var rows []batchRow
	var dumped []dumpRow
	batches := chunk(p.sentences, p.batchSize)
	start := time.Now()
	for i, b := range batches {
		if cellCtx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, b []string) {
			defer wg.Done()
			defer func() { <-sem }()
			row, out := p.runBatch(cellCtx, client, sys, model, conc, i, b)
			mu.Lock()
			rows = append(rows, row)
			dumped = append(dumped, out...)
			switch {
			case row.OK:
				streak = 0
			case row.ErrClass != "canceled":
				streak++
			}
			stop := streak >= p.failStreak
			mu.Unlock()
			if stop {
				cancel()
			}
		}(i, b)
	}
	wg.Wait()
	wall := time.Since(start)
	_ = st.Close()

	reqs := readStats(statsPath, model, conc, p.run)
	unservable := streak >= p.failStreak
	sum := summarize(cellInput{
		Model: model, Conc: conc, Batch: p.batchSize, Run: p.run, Reqs: reqs, Batches: rows,
		Wall: wall, Unservable: unservable, Replies: replies, ThinkReplies: think,
	})
	if p.dumpDir != "" {
		path := filepath.Join(p.dumpDir, fmt.Sprintf("%s-b%d-c%d.jsonl", shortID(model), p.batchSize, conc))
		if err := writeDump(path, dumped); err != nil {
			fmt.Printf("dump %s: %v\n", path, err)
		}
	}
	if p.rawDir != "" {
		path := filepath.Join(p.rawDir, fmt.Sprintf("%s-b%d-c%d.raw.jsonl", shortID(model), p.batchSize, conc))
		if err := writeRaw(path, raw); err != nil {
			fmt.Printf("raw %s: %v\n", path, err)
		}
	}
	p.mu.Lock()
	p.requests = append(p.requests, reqs...)
	p.batches = append(p.batches, rows...)
	p.mu.Unlock()
	return sum
}

// runBatch sends one production translate batch and checks the target text it
// gets back.
func (p *prober) runBatch(ctx context.Context, client *translate.Client, sys, model string, conc, idx int, sents []string) (batchRow, []dumpRow) {
	inputs := make([]translateInput, len(sents))
	for i, s := range sents {
		inputs[i] = translateInput{ID: strconv.Itoa(i + 1), Src: s}
	}
	userJSON, err := jsonx.Marshal(inputs)
	if err != nil {
		fatal("marshal batch: %v", err)
	}
	row := batchRow{Model: model, Concurrency: conc, Run: p.run, Batch: idx, Sent: len(sents)}
	start := time.Now()
	out, err := client.TranslateText(ctx, sys, string(userJSON))
	row.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		row.ErrClass = classifyErr(err)
		row.Err = clip(err.Error(), 200)
		return row, nil
	}
	row.OK = true
	var dump []dumpRow
	for id, text := range out {
		n, e := strconv.Atoi(strings.TrimSpace(id))
		if e != nil || n < 1 || n > len(sents) {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		row.Got++
		src := sents[n-1]
		dump = append(dump, dumpRow{Idx: idx*p.batchSize + n - 1, Src: src, Tr: text})
		switch {
		case !p.cyrTarget || cyrillicMajority(text):
			row.CyrMajority++
		default:
			p.addExample(example{Model: model, Concurrency: conc, Kind: "not_majority_target_script", Src: src, Out: text})
		}
		if leaks := latinLeaks(src, text); len(leaks) > 0 {
			row.LatinLeak++
			row.Leaks = append(row.Leaks, leaks...)
			p.addExample(example{Model: model, Concurrency: conc, Kind: "latin_leak", Src: src, Out: text, Leaks: leaks})
		}
	}
	return row, dump
}

// dumpRow is one returned translation, for the quality smoke artifact. Idx is
// the sentence's position in the probe set, so a concurrent run still writes
// the file in reading order.
type dumpRow struct {
	Idx int    `json:"-"`
	Src string `json:"src"`
	Tr  string `json:"tr"`
}

// writeDump writes the cell's translations as {src,tr} JSONL in sentence order.
func writeDump(path string, rows []dumpRow) error {
	if len(rows) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Idx < rows[j].Idx })
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// rawReply is one reply exactly as the model sent it, before the client's
// stripThink/stripFences and before parsing. A reply the parser rejects is
// discarded by the production path and leaves no text behind, so this is the
// only way to quote a malformed answer as evidence.
type rawReply struct {
	Model   string `json:"model"`
	Conc    int    `json:"concurrency"`
	Run     string `json:"run"`
	N       int    `json:"n"`
	Chars   int    `json:"chars"`
	Content string `json:"content"`
}

func writeRaw(path string, rows []rawReply) error {
	if len(rows) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// reasoningProbe sends the same short batch twice — once with gonka's
// thinking-off switch, once without it — to see whether the model honors it.
func (p *prober) reasoningProbe(ctx context.Context, model string) reasoningProbe {
	n := min(4, len(p.sentences))
	rp := reasoningProbe{Model: model, Sentences: n}
	rp.Disabled = p.oneShot(ctx, model, p.sentences[:n], false)
	rp.Omitted = p.oneShot(ctx, model, p.sentences[:n], true)
	return rp
}

func (p *prober) oneShot(ctx context.Context, model string, sents []string, noThinking bool) shot {
	kind := "off"
	if noThinking {
		kind = "on"
	}
	tag := fmt.Sprintf("probe-%s-%s-think%s", p.run, shortID(model), kind)
	statsPath := filepath.Join(p.tmp, tag+".jsonl")
	st, err := translate.OpenStats(statsPath)
	if err != nil {
		fatal("stats sink: %v", err)
	}
	var think bool
	client := translate.NewClient(translate.Options{
		Provider: translate.ProviderGonka,
		BaseURL:  p.baseURL, APIKey: p.apiKey, Model: model,
		Temperature: p.temperature, JSONMode: true,
		MaxRetries: p.maxRetries, Timeout: p.timeout,
		Stats: st, UserTag: tag, NoThinking: noThinking,
		OnContent: func(c string) { think = hasThink(c) },
	})
	sys := translate.TranslateSystemPrompt(translate.LangName(p.source), translate.LangName(p.target), nil)
	inputs := make([]translateInput, len(sents))
	for i, s := range sents {
		inputs[i] = translateInput{ID: strconv.Itoa(i + 1), Src: s}
	}
	userJSON, _ := jsonx.Marshal(inputs)
	start := time.Now()
	_, err = client.TranslateText(ctx, sys, string(userJSON))
	sh := shot{OK: err == nil, LatencyMS: time.Since(start).Milliseconds(), Think: think}
	if err != nil {
		sh.Err = clip(err.Error(), 200)
	}
	_ = st.Close()
	for _, r := range readStats(statsPath, model, 1, p.run) {
		if r.OutputTok > 0 {
			sh.OutputTok = r.OutputTok
		}
	}
	return sh
}

func (p *prober) addExample(e example) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.examples) >= 60 {
		return
	}
	e.Src, e.Out = clip(e.Src, 300), clip(e.Out, 300)
	p.examples = append(p.examples, e)
}

// readStats turns the client's own JSONL records into per-request rows.
func readStats(path, model string, conc int, run string) []reqRow {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var rows []reqRow
	dec := json.NewDecoder(f)
	for {
		var rec struct {
			TS           string `json:"ts"`
			LatencyMS    int64  `json:"latency_ms"`
			Status       int    `json:"status"`
			PromptTok    int    `json:"prompt_tokens"`
			OutputTok    int    `json:"completion_tokens"`
			FinishReason string `json:"finish_reason"`
			Err          string `json:"err"`
		}
		if err := dec.Decode(&rec); err != nil {
			break
		}
		rows = append(rows, reqRow{
			Model: model, Concurrency: conc, Run: run,
			TS: rec.TS, LatencyMS: rec.LatencyMS, Status: rec.Status,
			FinishReason: rec.FinishReason, PromptTok: rec.PromptTok, OutputTok: rec.OutputTok,
			Err: rec.Err, ErrClass: classifyRec(rec.Status, rec.FinishReason, rec.Err),
		})
	}
	return rows
}

// loadSentences draws n sentences at an even stride from one chapter of a
// .tbook, skipping anything shorter than minWords.
func loadSentences(book, chapter string, n, minWords int) ([]string, error) {
	zr, err := zip.OpenReader(book)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var payload struct {
		Paragraphs [][]*tbook.Sentence `json:"paragraphs"`
	}
	found := false
	for _, f := range zr.File {
		if f.Name != chapter {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		err = json.NewDecoder(rc).Decode(&payload)
		rc.Close()
		if err != nil {
			return nil, err
		}
		found = true
		break
	}
	if !found {
		return nil, fmt.Errorf("no entry %s in %s", chapter, book)
	}
	var cands []string
	for _, para := range payload.Paragraphs {
		for _, s := range para {
			if src := strings.TrimSpace(s.Src); len(strings.Fields(src)) >= minWords {
				cands = append(cands, src)
			}
		}
	}
	if len(cands) < n {
		return nil, fmt.Errorf("only %d sentences of ≥%d words in %s, need %d", len(cands), minWords, chapter, n)
	}
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, cands[i*len(cands)/n])
	}
	return out, nil
}

// modelDetails reads the context/max-token columns the /v1/models payload
// carries beyond the ids ServedModels returns.
func modelDetails(baseURL, apiKey string) map[string]modelInfo {
	out := map[string]modelInfo{}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return out
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var list struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
			CtxLen  int    `json:"context_length"`
			MaxToks int    `json:"max_tokens"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &list) != nil {
		return out
	}
	for _, m := range list.Data {
		out[m.ID] = modelInfo{ID: m.ID, CtxLen: m.CtxLen, MaxToks: m.MaxToks, OwnedBy: m.OwnedBy}
	}
	return out
}

// chooseModels resolves --models, defaulting to every served id plus the ids
// the repo assumes are there (so a missing default shows up as a failing cell
// rather than as silence).
func chooseModels(flagVal string, served []string) []string {
	if strings.TrimSpace(flagVal) != "" {
		return splitList(flagVal)
	}
	out := append([]string(nil), served...)
	for _, id := range repoAssumed {
		if !contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

func renderTable(cells []cellSummary) string {
	var sb strings.Builder
	sb.WriteString("\n| model | batch | conc | req | ok % | 429 % | 502 % | trunc % | p50 s | p90 s | sent/s | sent | out tok | think % | cyr % | latin % | verdict |\n")
	sb.WriteString("|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|---|\n")
	for _, c := range cells {
		verdict := "ok"
		if c.Unservable {
			verdict = "unservable"
		}
		fmt.Fprintf(&sb, "| `%s` | %d | %d | %d | %.1f | %.1f | %.1f | %.1f | %.1f | %.1f | %.2f | %d/%d | %d | %.0f | %.1f | %.1f | %s |\n",
			c.Model, c.BatchSize, c.Concurrency, c.Requests, c.SuccessPct, c.Pct429, c.Pct502, c.PctTrunc,
			float64(c.P50MS)/1000, float64(c.P90MS)/1000, c.SentPerSec, c.SentGot, c.SentAsked,
			c.MeanOutTok, c.ThinkPct, c.CyrPct, c.LatinPct, verdict)
	}
	return sb.String()
}

// artifactPath resolves --out, defaulting to today's file under .artifacts.
func artifactPath(out string) string {
	if out != "" {
		return out
	}
	return filepath.Join("bench-quality", ".artifacts",
		fmt.Sprintf("gonka-probe-%s.json", time.Now().Format("2006-01-02")))
}

func writeArtifact(path string, art artifact) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := jsonx.MarshalIndent(art, "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func chunk(s []string, n int) [][]string {
	var out [][]string
	for i := 0; i < len(s); i += n {
		out = append(out, s[i:min(i+n, len(s))])
	}
	return out
}

func cyrillicTarget(t string) bool {
	switch t {
	case "ru", "uk", "bg", "sr", "mk", "be":
		return true
	}
	return false
}

func fmtShot(s shot) string {
	if !s.OK {
		return "FAIL(" + clip(s.Err, 60) + ")"
	}
	return fmt.Sprintf("%d tok / %.1fs%s", s.OutputTok, float64(s.LatencyMS)/1000, map[bool]string{true: " <think>"}[s.Think])
}

func fmtCell(c cellSummary) string {
	v := ""
	if c.Unservable {
		v = " UNSERVABLE"
	}
	return fmt.Sprintf("ok %.0f%% 429 %.0f%% 502 %.0f%% trunc %.0f%% p50 %.1fs p90 %.1fs %.2f sent/s (%d/%d)%s",
		c.SuccessPct, c.Pct429, c.Pct502, c.PctTrunc, float64(c.P50MS)/1000, float64(c.P90MS)/1000,
		c.SentPerSec, c.SentGot, c.SentAsked, v)
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseInts(s string) []int {
	var out []int
	for _, p := range splitList(s) {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func shortID(model string) string {
	if i := strings.LastIndexByte(model, '/'); i >= 0 {
		model = model[i+1:]
	}
	return strings.Map(func(r rune) rune {
		if r == '.' || r == ':' {
			return '-'
		}
		return r
	}, model)
}

func num(n int) string {
	if n == 0 {
		return "?"
	}
	return strconv.Itoa(n)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func nowTS() string { return time.Now().UTC().Format(time.RFC3339) }

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
