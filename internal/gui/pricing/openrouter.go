package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const defaultOpenRouterBase = "https://openrouter.ai/api/v1"

const (
	// freshFor is how long a successful fetch is reused.
	freshFor = time.Hour
	// retryAfter throttles re-fetching after a failure: quotes are recomputed
	// on every estimate, and an unreachable OpenRouter must not turn each one
	// into its own HTTP attempt.
	retryAfter = time.Minute
)

// Rate is one model's prompt/completion price in USD per token.
type Rate struct {
	Prompt     float64
	Completion float64
}

// Fetcher looks up OpenRouter token prices, caching the whole model list in
// memory for an hour. One fetch serves every model, so quoting another model
// costs no extra request.
type Fetcher struct {
	BaseURL string // "" means the public OpenRouter API; override in tests
	HTTP    *http.Client

	mu        sync.Mutex
	rates     map[string]Rate
	fetchedAt time.Time // last successful fetch
	triedAt   time.Time // last attempt, successful or not
	warned    map[string]bool
}

// NewFetcher builds a Fetcher against the public OpenRouter API.
func NewFetcher() *Fetcher {
	return &Fetcher{
		BaseURL: defaultOpenRouterBase,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Prices returns (promptPrice, completionPrice) in USD per token for one
// OpenRouter model id. It never fails: an unreachable API falls back to the
// last known rates, then to the pinned defaults (with a logged warning).
func (f *Fetcher) Prices(ctx context.Context, model string) (float64, float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	stale := f.rates == nil || time.Since(f.fetchedAt) >= freshFor
	if stale && time.Since(f.triedAt) >= retryAfter {
		f.triedAt = time.Now()
		rates, err := f.fetch(ctx)
		switch {
		case err != nil && f.rates != nil:
			log.Printf("pricing: OpenRouter fetch failed (%v); using last known prices", err)
		case err != nil:
			log.Printf("pricing: OpenRouter fetch failed (%v); using pinned default prices", err)
		default:
			f.rates, f.fetchedAt, f.warned = rates, time.Now(), nil
		}
	}

	if r, ok := f.rates[model]; ok {
		return r.Prompt, r.Completion
	}
	// Unknown model: log once per fetch cycle, not once per quote.
	if !f.warned[model] {
		if f.warned == nil {
			f.warned = make(map[string]bool)
		}
		f.warned[model] = true
		log.Printf("pricing: model %s has no OpenRouter price; using pinned defaults", model)
	}
	return DefaultPromptPrice, DefaultCompletionPrice
}

// fetch reads the full model list and returns every usable price in it.
func (f *Fetcher) fetch(ctx context.Context) (map[string]Rate, error) {
	base := f.BaseURL
	if base == "" {
		base = defaultOpenRouterBase
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	client := f.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter /models: HTTP %d", resp.StatusCode)
	}

	var body struct {
		Data []struct {
			ID      string `json:"id"`
			Pricing struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode /models: %w", err)
	}

	rates := make(map[string]Rate, len(body.Data))
	for _, m := range body.Data {
		p, err := strconv.ParseFloat(m.Pricing.Prompt, 64)
		if err != nil {
			continue // free/unpriced or malformed entry: not one we can quote
		}
		c, err := strconv.ParseFloat(m.Pricing.Completion, 64)
		if err != nil {
			continue
		}
		rates[m.ID] = Rate{Prompt: p, Completion: c}
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("openrouter /models: no priced models in the response")
	}
	return rates, nil
}
