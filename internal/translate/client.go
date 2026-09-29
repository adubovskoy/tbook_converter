package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dimando/reader/converter/internal/align"
)

// LLM backends. OpenRouter is the metered HTTP API; Gonka is the
// proxy.gonka.gg gateway to the Gonka decentralized compute network (also
// OpenAI-compatible, keyed, near-free per token); Claude shells out to the
// `claude` CLI in print mode, so batches run on the user's Claude subscription
// (OAuth) with no per-token billing; Ollama and LlamaCpp post to a local
// server's OpenAI-compatible API (free, offline, no key).
const (
	ProviderOpenRouter = "openrouter"
	ProviderGonka      = "gonka"
	ProviderClaude     = "claude"
	ProviderOllama     = "ollama"
	ProviderLlamaCpp   = "llamacpp"
)

// Options configures the LLM client.
type Options struct {
	Provider  string // ProviderOpenRouter (default) or ProviderClaude
	ClaudeBin string // claude CLI path for ProviderClaude; "" = "claude" from $PATH

	BaseURL     string
	APIKey      string
	Model       string
	Referer     string
	Title       string
	Temperature float64
	JSONMode    bool
	MaxRetries  int
	Timeout     time.Duration

	// ProviderSort biases OpenRouter provider routing: "throughput" (fastest
	// tokens/sec), "latency" (lowest time-to-first-token), or "price". Empty
	// leaves OpenRouter's default routing. ProviderOrder pins specific provider
	// slugs (e.g. "alibaba") in priority order. See
	// https://openrouter.ai/docs/features/provider-routing.
	ProviderSort  string
	ProviderOrder []string

	// Stats, when non-nil, receives one JSONL record per request attempt
	// (latency, status, tokens, cost, serving provider). Also switches on
	// OpenRouter usage accounting in responses.
	Stats *Stats

	// UserTag fills the OpenAI-standard "user" field. The gonka gateway caches
	// identical request bodies — a repeated batch comes back byte-identical in
	// ~0.2 s — so a benchmark that sends one sentence set twice must vary
	// something outside the prompt. This does, without touching sampling.
	// Empty (production) omits the field.
	UserTag string

	// NoThinking omits gonka's "thinking":{"type":"disabled"} switch, to
	// measure what reasoning costs. Benchmarks only.
	NoThinking bool

	// OnContent, when set, receives each raw reply before parsing, so a probe
	// can see a <think> block that stripThink would remove. Called from every
	// request goroutine: it must be safe for concurrent use.
	OnContent func(content string)
}

// Client calls an OpenAI-style chat-completions API (OpenRouter, or a local
// Ollama server) with retry/backoff.
type Client struct {
	opts Options
	http *http.Client
	// mandatesReasoning latches once the endpoint has rejected the
	// reasoning-off switch, so the remaining batches of a run (hundreds, all
	// concurrent) start with the effort tier instead of each paying its own
	// rejected request first.
	mandatesReasoning atomic.Bool
}

// NewClient builds a client. Timeout applies per HTTP request. The transport
// keeps a generous idle-connection pool so high --concurrency doesn't churn
// TCP/TLS between request waves (requests multiplex over HTTP/2 anyway; this
// is insurance for HTTP/1.1 fallback).
func NewClient(o Options) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = 64
	return &Client{opts: o, http: &http.Client{Timeout: o.Timeout, Transport: tr}}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Provider       *providerPrefs  `json:"provider,omitempty"`
	Usage          *usageOpt       `json:"usage,omitempty"`     // OpenRouter usage accounting (cost per request)
	Thinking       *thinkingOpt    `json:"thinking,omitempty"`  // Gonka: reasoning off for models that honor it
	Reasoning      *reasoningOpt   `json:"reasoning,omitempty"` // OpenRouter: unified reasoning switch
	User           string          `json:"user,omitempty"`      // cache-busting tag for benchmarks (Options.UserTag)
}

// reasoningOpt is OpenRouter's unified reasoning control. {"enabled":false}
// turns reasoning off on hybrid models and is a no-op on the rest; endpoints
// that mandate reasoning reject it (see reasoningMandatory) and take an effort
// tier instead.
type reasoningOpt struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Effort  string `json:"effort,omitempty"`
}

// minimalEffort is the cheapest reasoning tier, for endpoints that refuse to
// turn reasoning off. Measured on a 16-sentence en→ru batch against
// google/gemini-3.7-flash: 0 reasoning tokens and 259 completion tokens, where
// omitting the field (or asking only to hide the reasoning with
// {"exclude":true}) reasons anyway — 441 reasoning tokens, 2.4× the cost, none
// of which the pipeline reads. See converter/bench-quality/reports/
// four-model-bench-2026-08.md §5.1.
const minimalEffort = "minimal"

// reasoningOff and reasoningMinimal are the two shapes above.
func reasoningOff() *reasoningOpt {
	off := false
	return &reasoningOpt{Enabled: &off}
}

func reasoningMinimal() *reasoningOpt { return &reasoningOpt{Effort: minimalEffort} }

// reasoningMandatory reports whether a 400 rejected the reasoning-off switch.
// OpenRouter's wording: "Reasoning is mandatory for this endpoint and cannot
// be disabled." — every Gemini 3.x but 3.1 Flash Lite answers this.
func reasoningMandatory(err error) bool {
	ae, ok := err.(*apiError)
	return ok && ae.status == http.StatusBadRequest &&
		strings.Contains(strings.ToLower(ae.msg), "reasoning is mandatory")
}

// thinkingOpt is the Anthropic-style reasoning switch the Gonka gateway
// forwards to its models. {"type":"disabled"} turns Kimi-K2.6's reasoning off
// (measured: 975→30 output tokens, 16s→1s on a one-sentence batch);
// MiniMax-M2.7 always thinks and ignores it — its inline <think> block is
// stripped at parse time instead.
type thinkingOpt struct {
	Type string `json:"type"`
}

type usageOpt struct {
	Include bool `json:"include"`
}

type responseFormat struct {
	Type string `json:"type"`
}

// providerPrefs is OpenRouter's provider-routing preference object. Sort orders
// candidate providers ("throughput"|"latency"|"price"); Order pins providers by
// slug. Both omitted ⇒ field absent ⇒ default routing.
type providerPrefs struct {
	Sort  string   `json:"sort,omitempty"`
	Order []string `json:"order,omitempty"`
}

type chatResponse struct {
	Provider string `json:"provider"` // serving provider (OpenRouter)
	Choices  []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// apiError is a non-2xx response. Permanent statuses are not retried.
type apiError struct {
	status   int
	msg      string
	provider string // backend label for the error message
}

func (e *apiError) Error() string { return fmt.Sprintf("%s %d: %s", e.provider, e.status, e.msg) }
func (e *apiError) permanent() bool {
	switch e.status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusPaymentRequired,
		http.StatusForbidden, http.StatusNotFound: // 404: model not pulled / bad endpoint
		return true
	}
	return false
}

// providerLabel names the HTTP backend in errors and stats.
func (c *Client) providerLabel() string {
	if c.opts.Provider == "" {
		return ProviderOpenRouter
	}
	return c.opts.Provider
}

// chat sends one batch (system instructions + user JSON) and invokes parse on
// the model's reply. Retries transient failures (network, 5xx, 429, and parse
// errors — parse returning non-nil) with exponential backoff, honoring
// Retry-After on 429. Permanent HTTP statuses and a subscription usage limit
// (*UsageLimitError) are not retried.
func (c *Client) chat(ctx context.Context, system, userJSON string, parse func(content string) error) error {
	var send func(ctx context.Context, rec *statRec) (string, time.Duration, error)
	// adapt gets one chance to rewrite the request after an otherwise
	// permanent failure, and reports whether it did. nil = nothing to adapt.
	var adapt func(error) bool
	// revary rewrites the request body before a retry so a cached malformed
	// reply cannot be served again. nil for the claude CLI, which has no body.
	var revary func(attempt int)
	if c.opts.Provider == ProviderClaude {
		send = func(ctx context.Context, rec *statRec) (string, time.Duration, error) {
			rec.Provider = "claude-cli"
			return c.claudeOnce(ctx, system, userJSON)
		}
	} else {
		req := chatRequest{
			Model:       c.opts.Model,
			Messages:    []message{{Role: "system", Content: system}, {Role: "user", Content: userJSON}},
			Temperature: c.opts.Temperature,
			User:        c.opts.UserTag,
		}
		if c.opts.JSONMode {
			req.ResponseFormat = &responseFormat{Type: "json_object"}
		}
		// Provider routing and usage accounting are OpenRouter extensions;
		// keep them out of requests to other OpenAI-compatible servers.
		if c.opts.Provider == "" || c.opts.Provider == ProviderOpenRouter {
			if c.opts.ProviderSort != "" || len(c.opts.ProviderOrder) > 0 {
				req.Provider = &providerPrefs{Sort: c.opts.ProviderSort, Order: c.opts.ProviderOrder}
			}
			if c.opts.Stats != nil {
				req.Usage = &usageOpt{Include: true}
			}
			// Hybrid-reasoning models (glm-5.2: 229 reasoning tokens and 8×
			// the cost on a one-sentence probe) reason by default; the
			// pipeline never reads reasoning, so switch it off. No-op for
			// non-reasoning models like the gemini default.
			req.Reasoning = reasoningOff()
			if c.mandatesReasoning.Load() {
				req.Reasoning = reasoningMinimal()
			}
		}
		// Gonka serves reasoning models; a translation batch never wants the
		// reasoning (it inflates output tokens and latency, not quality).
		if c.opts.Provider == ProviderGonka && !c.opts.NoThinking {
			req.Thinking = &thinkingOpt{Type: "disabled"}
		}
		payload, err := json.Marshal(req)
		if err != nil {
			return err
		}
		send = func(ctx context.Context, rec *statRec) (string, time.Duration, error) {
			rec.ReqBytes = len(payload)
			return c.once(ctx, payload, rec)
		}
		// The gonka gateway answers an IDENTICAL request body from its cache —
		// a repeated batch comes back byte-for-byte in ~90 ms. A malformed
		// reply is therefore permanent for that body: measured, 16 French
		// sentences returned the same truncated JSON on six consecutive
		// attempts across two runs and shipped untranslated. So a retry after a
		// parse failure changes the body, using the OpenAI-standard "user"
		// field that already exists for exactly this purpose. It changes
		// nothing about sampling and is ignored by servers that do not read it.
		revary = func(attempt int) {
			req.User = retryTag(c.opts.UserTag, attempt)
			if next, mErr := json.Marshal(req); mErr == nil {
				payload = next
			}
		}
		// An endpoint that mandates reasoning 400s on the switch above, which
		// would fail the whole run on its first batch (the glossary call).
		// Fall back to the cheapest reasoning tier, once, then carry on.
		adapt = func(err error) bool {
			if !reasoningMandatory(err) || req.Reasoning == nil || req.Reasoning.Effort != "" {
				return false
			}
			c.mandatesReasoning.Store(true)
			req.Reasoning = reasoningMinimal()
			next, mErr := json.Marshal(req)
			if mErr != nil {
				return false
			}
			payload = next
			// No log line: this package prints nothing. The rejected attempt
			// is already in the stats JSONL with its 400.
			return true
		}
	}

	phase := phaseOf(ctx)
	var lastErr error
	var wait time.Duration
	for attempt := 0; attempt <= c.opts.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		rec := &statRec{TS: nowTS(), Model: c.opts.Model, Phase: phase, Attempt: attempt}
		start := time.Now()
		content, retryAfter, err := send(ctx, rec)
		rec.LatencyMS = time.Since(start).Milliseconds()
		if err != nil {
			rec.Err = clip(err.Error(), 200)
			c.opts.Stats.log(rec)
			lastErr = err
			// A truncated reply carries its partial content; show it to a
			// caller that asked to see raw replies, then fail as usual.
			if content != "" && c.opts.OnContent != nil {
				c.opts.OnContent(content)
			}
			if ae, ok := err.(*apiError); ok && ae.permanent() {
				if adapt != nil && adapt(err) {
					wait = 0
					continue
				}
				return err
			}
			if _, ok := errors.AsType[*UsageLimitError](err); ok {
				return err // waiting out a usage window is the caller's call
			}
			if _, ok := errors.AsType[*ModelNotFoundError](err); ok {
				return err // the id is not served; every retry answers the same 404
			}
			if ce, ok := errors.AsType[*cliError](err); ok && ce.perm {
				return err // bad model / logged out — retrying cannot help
			}
			wait = backoff(attempt, retryAfter)
			continue
		}
		if c.opts.OnContent != nil {
			c.opts.OnContent(content)
		}
		if perr := parse(content); perr != nil {
			rec.Err = clip("parse: "+perr.Error(), 200)
			c.opts.Stats.log(rec)
			lastErr = fmt.Errorf("parse response: %w", perr)
			if revary != nil {
				revary(attempt + 1)
			}
			wait = backoff(attempt, 0)
			continue
		}
		c.opts.Stats.log(rec)
		return nil
	}
	return lastErr
}

// ModelNotFoundError is a 404 from the chat endpoint: the model id is not
// served. It aborts the run instead of failing every batch identically — on the
// gonka gateway the served list changes within hours, and the shipped default
// has answered 404 for days at a time, so the message names what IS served.
type ModelNotFoundError struct {
	Model    string
	Provider string
	BaseURL  string
	Served   []string // parsed from the 404 body, when it names one
	Message  string
}

func (e *ModelNotFoundError) Error() string {
	s := fmt.Sprintf("%s does not serve model %q (404)", e.Provider, e.Model)
	if len(e.Served) > 0 {
		return s + " — it serves: " + strings.Join(e.Served, ", ") + "\nRe-run with --model <one of those>."
	}
	if e.Message != "" {
		s += ": " + e.Message
	}
	return s + "\nAsk the gateway what it serves, then re-run with --model."
}

// servedModelsRE pulls the id list out of a 404 body that names one
// ("Supported models: a, b, c"). The gonka gateway writes exactly this, and it
// is more current than its own /v1/models, which has advertised an id for nine
// minutes after the endpoint stopped accepting it.
var servedModelsRE = regexp.MustCompile(`(?i)(?:supported|available) models:?\s*([^\n"}]+)`)

func servedFromError(msg string) []string {
	m := servedModelsRE.FindStringSubmatch(msg)
	if m == nil {
		return nil
	}
	var out []string
	for _, id := range strings.Split(m[1], ",") {
		if id = strings.Trim(strings.TrimSpace(id), "\"'`.;"); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// UsageLimitError signals the Claude subscription hit its usage-window cap.
// The run should stop promptly — the per-sentence cache makes re-running the
// same command after ResetAt (when known) resume exactly where it left off.
type UsageLimitError struct {
	ResetAt time.Time // zero when the CLI output carried no reset timestamp
	Message string    // the CLI's own wording, for the human
}

func (e *UsageLimitError) Error() string {
	when := ""
	if !e.ResetAt.IsZero() {
		when = fmt.Sprintf(" (resets %s)", e.ResetAt.Local().Format("2006-01-02 15:04"))
	}
	return fmt.Sprintf("claude subscription usage limit reached%s: %s — the cache is resumable; re-run the same command after the reset",
		when, e.Message)
}

// usageLimitRE matches the claude CLI's usage/session-limit wording; epochRE
// extracts the reset timestamp from the "…limit reached|<unix-epoch>" form.
var (
	usageLimitRE = regexp.MustCompile(`(?i)(usage limit|session limit|hit your .{0,20}limit|limit reached)`)
	epochRE      = regexp.MustCompile(`\|(\d{10})\b`)
)

// detectUsageLimit reports whether CLI output announces an exhausted usage
// window, extracting the reset time when present.
func detectUsageLimit(s string) *UsageLimitError {
	if !usageLimitRE.MatchString(s) {
		return nil
	}
	e := &UsageLimitError{Message: strings.TrimSpace(s)}
	if len(e.Message) > 200 {
		e.Message = e.Message[:200]
	}
	if m := epochRE.FindStringSubmatch(s); m != nil {
		if sec, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			e.ResetAt = time.Unix(sec, 0)
		}
	}
	return e
}

// cliError is a claude CLI failure. Permanent ones (unknown model, logged
// out) abort instead of retrying — every batch would fail identically.
type cliError struct {
	msg  string
	perm bool
}

func (e *cliError) Error() string { return "claude CLI: " + e.msg }

// cliPermanentRE matches CLI failures no retry can fix. The CLI prints these
// to STDOUT with exit code 0, so they must be sniffed from the output text.
var cliPermanentRE = regexp.MustCompile(`(?i)issue with the selected model|may not exist or you may not have access|not logged in|please run /login|invalid api key`)

// claudeOnce sends one batch through `claude -p` (headless Claude Code). The
// system prompt replaces the CLI's default one, tools/settings/skills are
// disabled, sessions are not persisted, and the working directory is neutral —
// each call is a plain stateless LLM request billed to the subscription.
func (c *Client) claudeOnce(ctx context.Context, system, userJSON string) (string, time.Duration, error) {
	if c.opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opts.Timeout)
		defer cancel()
	}
	bin := c.opts.ClaudeBin
	if bin == "" {
		bin = "claude"
	}
	cmd := exec.CommandContext(ctx, bin,
		"-p",
		"--model", c.opts.Model,
		"--system-prompt", system,
		"--tools", "",
		"--no-session-persistence",
		"--disable-slash-commands",
		"--setting-sources", "",
	)
	cmd.Stdin = strings.NewReader(userJSON)
	cmd.Dir = os.TempDir() // neutral cwd: no project CLAUDE.md / settings pickup
	cmd.Env = claudeEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()

	out := strings.TrimSpace(stdout.String())
	// Batch replies are JSON objects; a reply that isn't one is CLI prose
	// (errors land on stdout with exit 0). Never sniff error phrases out of
	// real JSON content — a book about APIs may well contain them.
	if !strings.HasPrefix(out, "{") {
		if ule := detectUsageLimit(out + "\n" + stderr.String()); ule != nil {
			return "", 0, ule
		}
		if cliPermanentRE.MatchString(out) {
			return "", 0, &cliError{msg: clip(out, 300), perm: true}
		}
	}
	if runErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = out
		}
		return "", 0, &cliError{msg: fmt.Sprintf("%v: %s", runErr, clip(msg, 300))}
	}
	return out, 0, nil
}

// clip truncates s to at most n bytes for error messages.
func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// claudeEnv is the process environment minus API-key credentials, so the CLI
// can only authenticate via the logged-in subscription (OAuth) — a stray
// ANTHROPIC_API_KEY would silently switch every call to per-token billing.
func claudeEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") || strings.HasPrefix(kv, "ANTHROPIC_AUTH_TOKEN=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// Translate runs the ALIGN pass: returns the parsed id→chunks map.
func (c *Client) Translate(ctx context.Context, system, userJSON string) (map[string][]align.Chunk, error) {
	var out map[string][]align.Chunk
	err := c.chat(ctx, system, userJSON, func(content string) error {
		m, perr := parseChunks(content)
		if perr != nil {
			return perr
		}
		out = m
		return nil
	})
	return out, err
}

// ChatJSON sends one batch and unmarshals the model's JSON-object reply into
// out (tolerating code fences and surrounding prose). Used by the glossary and
// judge passes.
func (c *Client) ChatJSON(ctx context.Context, system, userJSON string, out any) error {
	return c.chat(ctx, system, userJSON, func(content string) error {
		s := stripFences(stripThink(content))
		if err := json.Unmarshal([]byte(s), out); err != nil {
			if obj := extractObject(s); obj != "" {
				return json.Unmarshal([]byte(obj), out)
			}
			return err
		}
		return nil
	})
}

// Model returns the model id this client sends requests with.
func (c *Client) Model() string { return c.opts.Model }

// MaxRetries is the per-request retry budget this client was built with.
func (c *Client) MaxRetries() int { return c.opts.MaxRetries }

// TranslateText runs the TRANSLATE pass: returns the parsed id→translation-text
// map (each value a plain string, not chunks).
func (c *Client) TranslateText(ctx context.Context, system, userJSON string) (map[string]string, error) {
	var out map[string]string
	err := c.chat(ctx, system, userJSON, func(content string) error {
		m, perr := parseTexts(content)
		if perr != nil {
			return perr
		}
		out = m
		return nil
	})
	return out, err
}

// errTruncated marks a completion the provider cut off (finish_reason
// "length"): the batch likely exceeds the provider's output cap. Retryable —
// OpenRouter may route the retry to a provider with a higher cap — but a batch
// that keeps truncating burns its rounds; watch finish_reason in --stats.
var errTruncated = errors.New("completion truncated (finish_reason=length) — batch may exceed the provider's output cap")

func (c *Client) once(ctx context.Context, payload []byte, rec *statRec) (string, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.opts.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	if c.opts.APIKey != "" { // Ollama runs keyless; don't send an empty Bearer
		req.Header.Set("Authorization", "Bearer "+c.opts.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.opts.Referer != "" {
		req.Header.Set("HTTP-Referer", c.opts.Referer)
	}
	if c.opts.Title != "" {
		req.Header.Set("X-Title", c.opts.Title)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, err // network/timeout — retryable
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	rec.Status = resp.StatusCode
	rec.RespBytes = len(body)

	if resp.StatusCode/100 != 2 {
		ra := parseRetryAfter(resp.Header.Get("Retry-After"))
		msg := strings.TrimSpace(string(body))
		var cr chatResponse
		if json.Unmarshal(body, &cr) == nil && cr.Error != nil && cr.Error.Message != "" {
			msg = cr.Error.Message
		}
		if resp.StatusCode == http.StatusNotFound {
			return "", 0, &ModelNotFoundError{Model: c.opts.Model, Provider: c.providerLabel(),
				BaseURL: c.opts.BaseURL, Served: servedFromError(msg), Message: clip(msg, 300)}
		}
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return "", ra, &apiError{status: resp.StatusCode, msg: msg, provider: c.providerLabel()}
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", 0, err
	}
	rec.Provider = cr.Provider
	if rec.Provider == "" { // only OpenRouter reports a serving provider
		rec.Provider = c.providerLabel()
	}
	if cr.Usage != nil {
		rec.PromptTok = cr.Usage.PromptTokens
		rec.OutputTok = cr.Usage.CompletionTokens
		rec.Cost = cr.Usage.Cost
	}
	if len(cr.Choices) == 0 {
		return "", 0, fmt.Errorf("no choices in response")
	}
	rec.FinishReason = cr.Choices[0].FinishReason
	if cr.Choices[0].FinishReason == "length" {
		// The partial rides along with the error so a caller with OnContent can
		// SEE it. Until B1c a truncated reply was discarded here and the report
		// could only say "it cannot be quoted"; the glossary pass truncating on
		// deepseek-ai/DeepSeek-V4-Flash-0731 was undiagnosable for that reason.
		// It is still an error: nothing downstream may treat it as an answer.
		return cr.Choices[0].Message.Content, 0, errTruncated
	}
	return cr.Choices[0].Message.Content, 0, nil
}

// parseChunks extracts the id→chunks object from model output, tolerating code
// fences, surrounding prose, and a leading <think> block.
func parseChunks(content string) (map[string][]align.Chunk, error) {
	s := stripFences(stripThink(content))
	var m map[string][]align.Chunk
	if err := json.Unmarshal([]byte(s), &m); err == nil {
		return m, nil
	} else if nd, ok := mergeJSONStream[[]align.Chunk](s); ok {
		return nd, nil
	} else if obj := extractObject(s); obj != "" {
		if err2 := json.Unmarshal([]byte(obj), &m); err2 == nil {
			return m, nil
		}
		return nil, err
	} else {
		return nil, err
	}
}

// parseTexts extracts the id→translation-string object from a translate-pass
// reply, tolerating code fences, surrounding prose (mirrors parseChunks) and a
// newline-delimited sequence of objects (mergeNDJSON).
func parseTexts(content string) (map[string]string, error) {
	s := stripFences(stripThink(content))
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err == nil {
		return m, nil
	} else if nd, ok := mergeNDJSON(s); ok {
		return nd, nil
	} else if obj := extractObject(s); obj != "" {
		if err2 := json.Unmarshal([]byte(obj), &m); err2 == nil {
			return m, nil
		}
		return nil, err
	} else {
		return nil, err
	}
}

// mergeNDJSON reads the reply as a STREAM of JSON values instead of one, which
// rescues the two malformed shapes DeepSeek-V4-Flash-0731 produces on the gonka
// gateway. Both are well-formed JSON with something the strict parse will not
// accept next to it, and `extractObject` cannot rescue either, because it spans
// from the first "{" to the last "}" and so swallows whatever follows:
//
//   - one object per LINE instead of one object — ~8% of requests in the
//     served-model probe, translations correct and in the target language;
//   - one object followed by trailing commentary — 15 of 100 translate requests
//     in the B1 measurement run, reported as
//     `parse: invalid character '#' after top-level value`. Three rounds of
//     retries returned the same shape and 8 sentences shipped untranslated.
//
// Reading as a stream takes whatever complete objects are there and stops at
// the first thing that is not one: a truncated reply still fails (its first
// value never closes), and the ids the model did not return are simply absent,
// which the round loop already retries. The merge stays narrow otherwise: every
// value must be a string and no id may appear twice, because a repeated id
// means the objects are alternatives rather than parts of one answer, and
// guessing which was meant is worse than a retry.
func mergeNDJSON(s string) (map[string]string, bool) { return mergeJSONStream[string](s) }

// mergeJSONStream is the shape-agnostic half, so the align reply (id → chunks)
// gets exactly the same tolerance as the translate reply (id → text): both come
// from the same model over the same gateway and fail the same way.
func mergeJSONStream[T any](s string) (map[string]T, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	out := map[string]T{}
	objects := 0
	for {
		var m map[string]T
		err := dec.Decode(&m)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if objects == 0 {
				return nil, false // nothing usable: not this shape
			}
			break // trailing junk after a complete answer
		}
		objects++
		for k, v := range m {
			if _, dup := out[k]; dup {
				return nil, false
			}
			out[k] = v
		}
	}
	if objects == 0 || len(out) == 0 {
		return nil, false
	}
	return out, true
}

// stripThink drops the leading <think>…</think> block some reasoning models
// (MiniMax-M2, local qwen/deepseek builds) emit inside message content — it
// routinely mentions JSON braces, which would derail extractObject. An
// unterminated block means the answer never started: return "" so the parse
// fails and the attempt is retried.
func stripThink(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "<think>") {
		return s
	}
	if _, after, ok := strings.Cut(s, "</think>"); ok {
		return strings.TrimSpace(after)
	}
	return ""
}

func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	if i := strings.IndexByte(s, '\n'); i >= 0 { // drop an optional language tag line
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func extractObject(s string) string {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return ""
}

// retryTag is the per-attempt value of the request's "user" field. It keeps
// whatever the caller set (the benchmarks use it to bust the same cache) and
// appends the attempt, so successive retries of one batch differ from each
// other and from the first try.
func retryTag(base string, attempt int) string {
	tag := "retry-" + strconv.Itoa(attempt)
	if base == "" {
		return tag
	}
	return base + "/" + tag
}

func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

// ServedModels lists the model ids an OpenAI-compatible server reports at
// GET {baseURL}/models (Ollama and llama-server both support it). apiKey may
// be empty.
func ServedModels(baseURL, apiKey string) ([]string, error) {
	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	httpc := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, &apiError{status: resp.StatusCode, msg: clip(strings.TrimSpace(string(body)), 200), provider: "server"}
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("unexpected reply from %s — not an OpenAI-style model list: %s", url, clip(string(body), 120))
	}
	ids := make([]string, 0, len(list.Data))
	for _, m := range list.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// CheckOllama verifies before the run that the Ollama server is reachable at
// baseURL and serves model, turning connection failures and a missing model
// into actionable errors instead of a retried batch failure minutes in.
func CheckOllama(baseURL, model string) error {
	ids, err := ServedModels(baseURL, "")
	if err != nil {
		return fmt.Errorf("can't reach the Ollama server at %s (%v) — is `ollama serve` running?", baseURL, err)
	}
	for _, id := range ids {
		// A bare model name resolves to the :latest tag on the server.
		if id == model || (!strings.Contains(model, ":") && id == model+":latest") {
			return nil
		}
	}
	return fmt.Errorf("model %q not found on the Ollama server at %s (installed: %s) — run `ollama pull %s`",
		model, baseURL, strings.Join(ids, ", "), model)
}

// AdoptServedModel returns a clean model id from what a llama.cpp server
// reports serving — llama-server loads ONE model, ignores the requested id,
// and lists it as a GGUF path or alias; the cleaned basename keys the cache
// stably across machines.
func AdoptServedModel(baseURL, apiKey string) (string, error) {
	ids, err := ServedModels(baseURL, apiKey)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", errors.New("the server reports no loaded model")
	}
	name := ids[0]
	if i := strings.LastIndexAny(name, "/\\"); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimSuffix(name, ".gguf"), nil
}

// ServedBy reports whether model matches one of the served ids, exactly or by
// AdoptServedModel's cleaned form (basename, no .gguf).
func ServedBy(ids []string, model string) bool {
	for _, id := range ids {
		if id == model {
			return true
		}
		name := id
		if i := strings.LastIndexAny(name, "/\\"); i >= 0 {
			name = name[i+1:]
		}
		if strings.TrimSuffix(name, ".gguf") == model {
			return true
		}
	}
	return false
}

// backoff returns the wait before the next attempt: Retry-After if the server
// gave one, else exponential (1,2,4,…s capped at 30s) with jitter.
func backoff(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	d := min(time.Duration(1<<attempt)*time.Second, 30*time.Second)
	jitter := time.Duration(rand.Int63n(int64(500 * time.Millisecond)))
	return d + jitter
}
