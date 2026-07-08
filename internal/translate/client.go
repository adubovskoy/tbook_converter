package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dimando/reader/converter/internal/align"
)

// Options configures the OpenRouter client.
type Options struct {
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
}

// Client calls the OpenRouter chat-completions API with retry/backoff.
type Client struct {
	opts Options
	http *http.Client
}

// NewClient builds a client. Timeout applies per HTTP request.
func NewClient(o Options) *Client {
	return &Client{opts: o, http: &http.Client{Timeout: o.Timeout}}
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
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// apiError is a non-2xx response. Permanent statuses are not retried.
type apiError struct {
	status int
	msg    string
}

func (e *apiError) Error() string { return fmt.Sprintf("openrouter %d: %s", e.status, e.msg) }
func (e *apiError) permanent() bool {
	switch e.status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden:
		return true
	}
	return false
}

// chat sends one batch (system instructions + user JSON) and invokes parse on
// the model's reply. Retries transient failures (network, 5xx, 429, and parse
// errors — parse returning non-nil) with exponential backoff, honoring
// Retry-After on 429. Permanent HTTP statuses are not retried.
func (c *Client) chat(ctx context.Context, system, userJSON string, parse func(content string) error) error {
	req := chatRequest{
		Model:       c.opts.Model,
		Messages:    []message{{Role: "system", Content: system}, {Role: "user", Content: userJSON}},
		Temperature: c.opts.Temperature,
	}
	if c.opts.JSONMode {
		req.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	if c.opts.ProviderSort != "" || len(c.opts.ProviderOrder) > 0 {
		req.Provider = &providerPrefs{Sort: c.opts.ProviderSort, Order: c.opts.ProviderOrder}
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}

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
		content, retryAfter, err := c.once(ctx, payload)
		if err != nil {
			lastErr = err
			if ae, ok := err.(*apiError); ok && ae.permanent() {
				return err
			}
			wait = backoff(attempt, retryAfter)
			continue
		}
		if perr := parse(content); perr != nil {
			lastErr = fmt.Errorf("parse response: %w", perr)
			wait = backoff(attempt, 0)
			continue
		}
		return nil
	}
	return lastErr
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

func (c *Client) once(ctx context.Context, payload []byte) (string, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.opts.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.opts.APIKey)
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
		return "", ra, &apiError{status: resp.StatusCode, msg: msg}
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", 0, err
	}
	if len(cr.Choices) == 0 {
		return "", 0, fmt.Errorf("no choices in response")
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
