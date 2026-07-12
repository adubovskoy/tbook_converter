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
	"time"

	"github.com/dimando/reader/converter/internal/align"
)

// LLM backends. OpenRouter is the metered HTTP API; Claude shells out to the
// `claude` CLI in print mode, so batches run on the user's Claude subscription
// (OAuth) with no per-token billing; Ollama posts to a local Ollama server's
// OpenAI-compatible API (free, offline, no key).
const (
	ProviderOpenRouter = "openrouter"
	ProviderClaude     = "claude"
	ProviderOllama     = "ollama"
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
}

// Client calls an OpenAI-style chat-completions API (OpenRouter, or a local
// Ollama server) with retry/backoff.
type Client struct {
	opts Options
	http *http.Client
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
	Usage          *usageOpt       `json:"usage,omitempty"` // OpenRouter usage accounting (cost per request)
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
		}
		if c.opts.JSONMode {
			req.ResponseFormat = &responseFormat{Type: "json_object"}
		}
		// Provider routing and usage accounting are OpenRouter extensions;
		// keep them out of requests to other OpenAI-compatible servers.
		if c.opts.Provider != ProviderOllama {
			if c.opts.ProviderSort != "" || len(c.opts.ProviderOrder) > 0 {
				req.Provider = &providerPrefs{Sort: c.opts.ProviderSort, Order: c.opts.ProviderOrder}
			}
			if c.opts.Stats != nil {
				req.Usage = &usageOpt{Include: true}
			}
		}
		payload, err := json.Marshal(req)
		if err != nil {
			return err
		}
		send = func(ctx context.Context, rec *statRec) (string, time.Duration, error) {
			rec.ReqBytes = len(payload)
			return c.once(ctx, payload, rec)
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
			if ae, ok := err.(*apiError); ok && ae.permanent() {
				return err
			}
			if _, ok := errors.AsType[*UsageLimitError](err); ok {
				return err // waiting out a usage window is the caller's call
			}
			if ce, ok := errors.AsType[*cliError](err); ok && ce.perm {
				return err // bad model / logged out — retrying cannot help
			}
			wait = backoff(attempt, retryAfter)
			continue
		}
		if perr := parse(content); perr != nil {
			rec.Err = clip("parse: "+perr.Error(), 200)
			c.opts.Stats.log(rec)
			lastErr = fmt.Errorf("parse response: %w", perr)
			wait = backoff(attempt, 0)
			continue
		}
		c.opts.Stats.log(rec)
		return nil
	}
	return lastErr
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
		s := stripFences(content)
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
		return "", 0, errTruncated
	}
	return cr.Choices[0].Message.Content, 0, nil
}

// parseChunks extracts the id→chunks object from model output, tolerating code
// fences and surrounding prose.
func parseChunks(content string) (map[string][]align.Chunk, error) {
	s := stripFences(content)
	var m map[string][]align.Chunk
	if err := json.Unmarshal([]byte(s), &m); err == nil {
		return m, nil
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
// reply, tolerating code fences and surrounding prose (mirrors parseChunks).
func parseTexts(content string) (map[string]string, error) {
	s := stripFences(content)
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err == nil {
		return m, nil
	} else if obj := extractObject(s); obj != "" {
		if err2 := json.Unmarshal([]byte(obj), &m); err2 == nil {
			return m, nil
		}
		return nil, err
	} else {
		return nil, err
	}
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

func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

// CheckOllama verifies before the run that the Ollama server is reachable at
// baseURL and serves model, turning connection failures and a missing model
// into actionable errors instead of a retried batch failure minutes in.
func CheckOllama(baseURL, model string) error {
	httpc := &http.Client{Timeout: 5 * time.Second}
	url := strings.TrimRight(baseURL, "/") + "/models"
	resp, err := httpc.Get(url)
	if err != nil {
		return fmt.Errorf("can't reach the Ollama server at %s (%v) — is `ollama serve` running?", baseURL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return &apiError{status: resp.StatusCode, msg: clip(strings.TrimSpace(string(body)), 200), provider: ProviderOllama}
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return fmt.Errorf("unexpected reply from %s — not an OpenAI-style model list: %s", url, clip(string(body), 120))
	}
	names := make([]string, 0, len(list.Data))
	for _, m := range list.Data {
		// A bare model name resolves to the :latest tag on the server.
		if m.ID == model || (!strings.Contains(model, ":") && m.ID == model+":latest") {
			return nil
		}
		names = append(names, m.ID)
	}
	return fmt.Errorf("model %q not found on the Ollama server at %s (installed: %s) — run `ollama pull %s`",
		model, baseURL, strings.Join(names, ", "), model)
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
