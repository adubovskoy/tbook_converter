// Package keycheck implements the GUI's "Test connection" probes: one cheap,
// classified check per provider, returning a Result the setup dialog can
// render. Detail strings never contain the API key.
package keycheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dimando/reader/converter/internal/translate"
)

// Status classifies a probe outcome.
type Status string

const (
	StatusOK           Status = "ok"
	StatusInvalidKey   Status = "invalid_key"
	StatusNoCredits    Status = "no_credits"
	StatusUnreachable  Status = "unreachable"
	StatusModelMissing Status = "model_missing"
	StatusNotInstalled Status = "not_installed"
	StatusNotLoggedIn  Status = "not_logged_in"
)

// Result is one provider probe's outcome.
type Result struct {
	Status         Status   `json:"status"`
	Detail         string   `json:"detail"`            // human-readable, never contains the key
	Warning        string   `json:"warning,omitempty"` // non-fatal caveat
	Models         []string `json:"models,omitempty"`  // served models (ollama/llamacpp)
	UsageUSD       float64  `json:"usageUSD,omitempty"`
	LimitRemaining *float64 `json:"limitRemaining,omitempty"`
	IsFreeTier     bool     `json:"isFreeTier,omitempty"`
	ClaudeVersion  string   `json:"claudeVersion,omitempty"`
	Subscription   string   `json:"subscription,omitempty"`
	ResolvedBin    string   `json:"resolvedBin,omitempty"`
}

const (
	openRouterBase = "https://openrouter.ai/api/v1"
	gonkaBase      = "https://proxy.gonka.gg/v1"
	gonkaModel     = "moonshotai/Kimi-K2.6"
	ollamaBase     = "http://localhost:11434/v1"
	ollamaModel    = "translategemma:12b"
	llamaCppBase   = "http://localhost:8080/v1"
)

var httpClient = &http.Client{}

// CheckOpenRouter validates an OpenRouter key against GET {base}/key, which
// also reports usage and remaining credit limit.
func CheckOpenRouter(ctx context.Context, baseURL, key string) Result {
	base := openRouterBase
	if baseURL != "" {
		base = strings.TrimRight(baseURL, "/")
	}
	status, body, err := doJSON(ctx, http.MethodGet, base+"/key", key, nil, 10*time.Second)
	if err != nil {
		return Result{Status: StatusUnreachable, Detail: "can't reach OpenRouter: " + scrub(clip(err.Error(), 200), key)}
	}
	switch status {
	case http.StatusUnauthorized:
		return Result{Status: StatusInvalidKey,
			Detail: "OpenRouter rejected the key — check you pasted the whole key (it starts with sk-or-) from openrouter.ai/settings/keys"}
	case http.StatusPaymentRequired:
		return Result{Status: StatusNoCredits,
			Detail: "no credits on this key — add credits at openrouter.ai/settings/credits"}
	case http.StatusTooManyRequests:
		return Result{Status: StatusOK, Detail: "Key accepted",
			Warning: "OpenRouter is rate limited right now — try again in a moment"}
	}
	if status/100 != 2 {
		return Result{Status: StatusUnreachable,
			Detail: fmt.Sprintf("OpenRouter answered HTTP %d: %s", status, scrub(clipBody(body), key))}
	}

	// The documented shape is {"data":{...}}; accept a bare object too.
	var d struct {
		Label          string   `json:"label"`
		Usage          float64  `json:"usage"`
		Limit          *float64 `json:"limit"`
		LimitRemaining *float64 `json:"limit_remaining"`
		IsFreeTier     bool     `json:"is_free_tier"`
	}
	var wrapped struct {
		Data json.RawMessage `json:"data"`
	}
	raw := body
	if json.Unmarshal(body, &wrapped) == nil && len(wrapped.Data) > 0 && string(wrapped.Data) != "null" {
		raw = wrapped.Data
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return Result{Status: StatusUnreachable,
			Detail: "unexpected reply from OpenRouter: " + scrub(clipBody(body), key)}
	}
	res := Result{Status: StatusOK, UsageUSD: d.Usage, LimitRemaining: d.LimitRemaining, IsFreeTier: d.IsFreeTier,
		Detail: fmt.Sprintf("Key valid — $%.2f used", d.Usage)}
	if d.Limit != nil && d.LimitRemaining != nil && *d.LimitRemaining <= 0 {
		res.Status = StatusNoCredits
		res.Detail = "the key's credit limit is exhausted — raise it or add credits at openrouter.ai/settings/credits"
		return res
	}
	if d.IsFreeTier {
		res.Warning = "free-tier key: no purchased credits — paid models will fail with 402; add credits at openrouter.ai/settings/credits"
	}
	return res
}

// CheckGonka validates a Gonka gateway key in two stages: list served models
// (auth check), then a one-token completion (the definitive check — the
// gateway's auth behavior on /models is undocumented).
func CheckGonka(ctx context.Context, baseURL, key, model string) Result {
	base := gonkaBase
	if baseURL != "" {
		base = strings.TrimRight(baseURL, "/")
	}
	if model == "" {
		model = gonkaModel
	}
	ids, err := translate.ServedModels(base, key)
	if err != nil {
		msg := scrub(clip(err.Error(), 200), key)
		if strings.Contains(msg, " 401: ") || strings.Contains(msg, " 403: ") {
			return Result{Status: StatusInvalidKey,
				Detail: "the Gonka gateway rejected the key — check it at proxy.gonka.gg/dashboard"}
		}
		return Result{Status: StatusUnreachable, Detail: "can't reach the Gonka gateway: " + msg}
	}
	res := gonkaPing(ctx, base, key, model)
	if res.Status == StatusModelMissing {
		if len(ids) > 0 && ids[0] != model {
			if retry := gonkaPing(ctx, base, key, ids[0]); retry.Status == StatusOK {
				retry.Warning = fmt.Sprintf("model %q isn't served right now; %q answered instead", model, ids[0])
				return retry
			}
		}
		res.Models = ids
	}
	return res
}

// gonkaPing sends a one-token chat completion (near-free on gonka) and
// classifies the reply.
func gonkaPing(ctx context.Context, base, key, model string) Result {
	payload, _ := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens": 1,
		"stream":     false,
	})
	status, body, err := doJSON(ctx, http.MethodPost, base+"/chat/completions", key, payload, 30*time.Second)
	if err != nil {
		return Result{Status: StatusUnreachable, Detail: "can't reach the Gonka gateway: " + scrub(clip(err.Error(), 200), key)}
	}
	excerpt := scrub(clipBody(body), key)
	switch {
	case status/100 == 2:
		return Result{Status: StatusOK, Detail: "Key valid — test request completed"}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return Result{Status: StatusInvalidKey,
			Detail: "the Gonka gateway rejected the key — check it at proxy.gonka.gg/dashboard"}
	case status == http.StatusPaymentRequired:
		return Result{Status: StatusNoCredits, Detail: "the gateway reports no credits: " + excerpt}
	case status == http.StatusTooManyRequests:
		return Result{Status: StatusOK, Detail: "Key valid",
			Warning: "the Gonka network is saturated right now — conversions may start slowly"}
	case status == http.StatusNotFound || modelMissingBody(body):
		return Result{Status: StatusModelMissing, Detail: fmt.Sprintf("model %q not available: %s", model, excerpt)}
	default:
		return Result{Status: StatusUnreachable, Detail: fmt.Sprintf("gateway answered HTTP %d: %s", status, excerpt)}
	}
}

func modelMissingBody(body []byte) bool {
	s := strings.ToLower(string(body))
	return strings.Contains(s, "model") &&
		(strings.Contains(s, "not found") || strings.Contains(s, "not exist") || strings.Contains(s, "unknown model"))
}

// CheckOllamaServer verifies an Ollama server is reachable and serves model.
// A bare model name matches the server's name:latest tag.
func CheckOllamaServer(ctx context.Context, baseURL, model string) Result {
	_ = ctx
	base := ollamaBase
	if baseURL != "" {
		base = strings.TrimRight(baseURL, "/")
	}
	if model == "" {
		model = ollamaModel
	}
	ids, err := translate.ServedModels(base, "")
	if err != nil {
		return Result{Status: StatusUnreachable,
			Detail: fmt.Sprintf("can't reach the Ollama server at %s (%s) — is `ollama serve` running?", base, clip(err.Error(), 200))}
	}
	served := translate.ServedBy(ids, model) ||
		(!strings.Contains(model, ":") && translate.ServedBy(ids, model+":latest"))
	if served {
		return Result{Status: StatusOK, Models: ids,
			Detail: fmt.Sprintf("Ollama is running and %q is installed", model)}
	}
	return Result{Status: StatusModelMissing, Models: ids,
		Detail: fmt.Sprintf("model %q not found on the Ollama server — run `ollama pull %s`", model, model)}
}

// CheckLlamaCpp verifies a llama.cpp server is reachable and reports what it
// serves. llama-server loads one model and ignores the requested id, so a
// mismatch is a warning, not a failure.
func CheckLlamaCpp(ctx context.Context, baseURL, apiKey, model string) Result {
	_ = ctx
	base := llamaCppBase
	if baseURL != "" {
		base = strings.TrimRight(baseURL, "/")
	}
	ids, err := translate.ServedModels(base, apiKey)
	if err != nil {
		return Result{Status: StatusUnreachable,
			Detail: scrub(fmt.Sprintf("can't reach the server at %s (%s) — is llama-server running?", base, clip(err.Error(), 200)), apiKey)}
	}
	if len(ids) == 0 {
		return Result{Status: StatusModelMissing, Detail: "the server reports no loaded model"}
	}
	res := Result{Status: StatusOK, Models: ids,
		Detail: fmt.Sprintf("llama-server is running, serving %q", ids[0])}
	if model != "" && !translate.ServedBy(ids, model) {
		res.Warning = fmt.Sprintf("server serves %q; it will be used instead of %q", ids[0], model)
	}
	return res
}

// doJSON performs one HTTP request with a per-call timeout layered on ctx and
// returns the status and body (capped at 1 MB).
func doJSON(ctx context.Context, method, url, key string, payload []byte, timeout time.Duration) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var rd io.Reader
	if payload != nil {
		rd = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return 0, nil, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, body, nil
}

// scrub removes the API key from text destined for Detail/Warning.
func scrub(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "[key]")
}

func clipBody(body []byte) string { return clip(strings.TrimSpace(string(body)), 200) }

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
