package pricing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const modelsBody = `{"data":[
  {"id":"google/gemini-3.1-flash-lite","pricing":{"prompt":"0.00000025","completion":"0.0000015"}},
  {"id":"google/gemini-3.7-flash","pricing":{"prompt":"0.000000375","completion":"0.000001875"}},
  {"id":"some/free-model","pricing":{"prompt":"","completion":""}}
]}`

// stubOpenRouter serves one /models body and counts the requests.
func stubOpenRouter(t *testing.T, body string, status int) (*Fetcher, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	f := NewFetcher()
	f.BaseURL = srv.URL
	return f, &hits
}

// One fetch serves every model, and repeat quotes hit the in-memory cache.
func TestPricesServedAndCached(t *testing.T) {
	f, hits := stubOpenRouter(t, modelsBody, http.StatusOK)
	ctx := context.Background()

	if in, out := f.Prices(ctx, "google/gemini-3.1-flash-lite"); in != 0.00000025 || out != 0.0000015 {
		t.Errorf("flash-lite = (%v, %v), want (2.5e-7, 1.5e-6)", in, out)
	}
	if in, out := f.Prices(ctx, "google/gemini-3.7-flash"); in != 0.000000375 || out != 0.000001875 {
		t.Errorf("3.7-flash = (%v, %v), want (3.75e-7, 1.875e-6)", in, out)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("%d requests to /models, want 1 — the list is cached for an hour", got)
	}
}

// A failed fetch falls back to the pinned defaults, and retries are throttled
// so every quote does not become its own HTTP attempt.
func TestPricesFallsBackOnError(t *testing.T) {
	f, hits := stubOpenRouter(t, "nope", http.StatusInternalServerError)
	ctx := context.Background()
	for range 5 {
		if in, out := f.Prices(ctx, "google/gemini-3.1-flash-lite"); in != DefaultPromptPrice || out != DefaultCompletionPrice {
			t.Fatalf("failed fetch = (%v, %v), want the pinned defaults", in, out)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("%d requests after a failure, want 1 — retries are throttled", got)
	}
}

// A model OpenRouter does not price falls back to the pinned defaults rather
// than a zero price.
func TestPricesUnknownModelFallsBackToDefaults(t *testing.T) {
	f, _ := stubOpenRouter(t, modelsBody, http.StatusOK)
	if in, out := f.Prices(context.Background(), "google/no-such-model"); in != DefaultPromptPrice || out != DefaultCompletionPrice {
		t.Errorf("unknown model = (%v, %v), want the pinned defaults", in, out)
	}
	// An entry with an unparsable price is the same case as a missing one.
	if in, out := f.Prices(context.Background(), "some/free-model"); in != DefaultPromptPrice || out != DefaultCompletionPrice {
		t.Errorf("unpriced model = (%v, %v), want the pinned defaults", in, out)
	}
}

// Once a fetch has succeeded, a later failure keeps serving the known prices.
func TestPricesKeepsLastKnownRatesOnFailure(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(modelsBody))
	}))
	defer srv.Close()

	f := NewFetcher()
	f.BaseURL = srv.URL
	ctx := context.Background()
	f.Prices(ctx, "google/gemini-3.7-flash")

	// Expire the cache and the retry throttle, then break the API.
	f.mu.Lock()
	f.fetchedAt = time.Now().Add(-2 * time.Hour)
	f.triedAt = time.Now().Add(-2 * time.Hour)
	f.mu.Unlock()
	fail.Store(true)

	if in, out := f.Prices(ctx, "google/gemini-3.7-flash"); in != 0.000000375 || out != 0.000001875 {
		t.Errorf("after a failed refresh = (%v, %v), want the last known (3.75e-7, 1.875e-6)", in, out)
	}
}
