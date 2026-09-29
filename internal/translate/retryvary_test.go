package translate

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// The gonka gateway answers an identical request body from its cache, so a
// retry that changes nothing is answered with the identical malformed reply —
// measured, the same 16 French sentences came back truncated on six
// consecutive attempts across two runs and shipped untranslated. A retry after
// a parse failure must therefore change the body.
func TestParseFailureRetryVariesTheBody(t *testing.T) {
	var (
		mu     sync.Mutex
		bodies []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		n := len(bodies)
		mu.Unlock()

		content := `{"1":"«Брак вам к лицу», — заметил он."}`
		if n < 3 { // the first two attempts come back unparseable
			content = `{"1":"…` // never closes
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": content}}}})
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL, APIKey: "x", Model: "test-model",
		MaxRetries: 3, Timeout: 5 * time.Second})
	out, err := c.TranslateText(context.Background(), "sys", `[{"id":"1","src":"x"}]`)
	if err != nil {
		t.Fatalf("TranslateText: %v", err)
	}
	if out["1"] == "" {
		t.Fatalf("no translation: %v", out)
	}
	if len(bodies) != 3 {
		t.Fatalf("%d requests, want 3 (two parse failures then a success)", len(bodies))
	}
	for i := 1; i < len(bodies); i++ {
		if bodies[i] == bodies[i-1] {
			t.Fatalf("attempt %d re-sent an identical body; the gateway cache would "+
				"return the same malformed reply forever", i)
		}
	}
	// The varying part is the OpenAI-standard "user" field, and the prompt and
	// the sentences themselves are untouched.
	if !strings.Contains(bodies[1], `"user":"retry-1"`) ||
		!strings.Contains(bodies[2], `"user":"retry-2"`) {
		t.Errorf("retries do not carry a per-attempt user tag:\n%s\n%s", bodies[1], bodies[2])
	}
	// (every body contains the "user" ROLE; the tag is the "user" FIELD)
	if strings.Contains(bodies[0], `"user":"retry-`) {
		t.Error("the first attempt carried a retry tag; production sends none")
	}
	for _, b := range bodies {
		if !strings.Contains(b, `[{\"id\":\"1\",\"src\":\"x\"}]`) {
			t.Errorf("a retry changed the batch itself: %s", b)
		}
	}
}

func TestRetryTagKeepsTheCallersOwnTag(t *testing.T) {
	if got := retryTag("", 2); got != "retry-2" {
		t.Errorf("retryTag(\"\", 2) = %q", got)
	}
	if got := retryTag("bench-arm-a", 1); got != "bench-arm-a/retry-1" {
		t.Errorf("retryTag with a caller tag = %q", got)
	}
}
