package translate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The gonka gateway's own 404 body, verbatim in shape: it names what it serves,
// and that list is more current than its /v1/models (which advertised an id for
// nine minutes after the endpoint stopped accepting it).
const gonka404 = `{"error":{"message":"model not found: moonshotai/Kimi-K2.6. ` +
	`Supported models: MiniMaxAI/MiniMax-M2.7, zai-org/GLM-5.3-Flash, deepseek-ai/DeepSeek-V4-Flash-0731"}}`

func TestModelNotFoundAbortsWithoutRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(gonka404))
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL, APIKey: "x", Model: "moonshotai/Kimi-K2.6",
		Provider: ProviderGonka, MaxRetries: 8, Timeout: 5 * time.Second})
	_, err := c.TranslateText(context.Background(), "sys", `[{"id":"1","src":"x"}]`)
	var notFound *ModelNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("err = %v, want a ModelNotFoundError", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("%d requests, want exactly 1 — a 404 answers the same every time", n)
	}
	if len(notFound.Served) != 3 || notFound.Served[0] != "MiniMaxAI/MiniMax-M2.7" {
		t.Errorf("Served = %v, want the three ids from the body", notFound.Served)
	}
	msg := notFound.Error()
	for _, want := range []string{"moonshotai/Kimi-K2.6", "deepseek-ai/DeepSeek-V4-Flash-0731", "--model"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message does not mention %q: %s", want, msg)
		}
	}
	// It must abort the run, not be swallowed as an ordinary batch failure.
	if abortOnly(err) == nil {
		t.Error("abortOnly swallowed the 404; every remaining batch would fail identically")
	}
}

func TestServedFromErrorHandlesABodyWithNoList(t *testing.T) {
	if got := servedFromError("model_not_found"); got != nil {
		t.Errorf("servedFromError = %v, want nil", got)
	}
}
