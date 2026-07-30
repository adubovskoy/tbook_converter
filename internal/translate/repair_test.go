package translate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/tbook"
)

// repairServer mocks the three passes and records every proofread item it was
// sent. Translate returns "TR:<src>", repair returns "RP:<tr>", align echoes
// one chunk per token — enough to tell the phases apart in the cache.
func repairServer(t *testing.T, seen *[]repairInput) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		userJSON := req.Messages[len(req.Messages)-1].Content

		var items []repairInput
		_ = json.Unmarshal([]byte(userJSON), &items)
		// The align pass sends {id,src,words,tr}; the repair pass {id,src,tr}.
		isAlign := strings.Contains(userJSON, `"words"`)
		out := map[string]any{}
		for _, it := range items {
			switch {
			case isAlign:
				var chunks []map[string]any
				for tok := range strings.FieldsSeq(it.Tr) {
					chunks = append(chunks, map[string]any{"tgt": tok, "en": tok})
				}
				out[it.ID] = chunks
			case it.Tr != "": // repair
				*seen = append(*seen, it)
				out[it.ID] = "RP:" + it.Tr
			default: // translate
				out[it.ID] = "TR:" + it.Src
			}
		}
		content, _ := json.Marshal(out)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": string(content)}}}})
	}))
}

func repairPipeline(t *testing.T, srv *httptest.Server, cacheDir, model string, ctxN int) *Pipeline {
	t.Helper()
	client := NewClient(Options{BaseURL: srv.URL, APIKey: "x", Model: model,
		Temperature: 0, MaxRetries: 2, Timeout: 5 * time.Second})
	return &Pipeline{Client: client, CacheDir: cacheDir, Source: "en",
		BatchSize: 16, Concurrency: 1, Repair: true, RepairContext: ctxN}
}

func repairSentences() []*tbook.Sentence {
	return []*tbook.Sentence{
		{Src: "One.", Words: [][2]int{{0, 3}}, Tr: map[string]tbook.Translation{}},
		{Src: "Two.", Words: [][2]int{{0, 3}}, Tr: map[string]tbook.Translation{}},
		{Src: "Three.", Words: [][2]int{{0, 5}}, Tr: map[string]tbook.Translation{}},
		{Src: "Four.", Words: [][2]int{{0, 4}}, Tr: map[string]tbook.Translation{}},
	}
}

// TestRepairContextSendsNeighbours checks the --context payload: every item
// carries up to N preceding sentences in book order with their translations,
// the first sentence carries none, and the neighbours are context only (they
// are not themselves re-sent as items to fix).
func TestRepairContextSendsNeighbours(t *testing.T) {
	var seen []repairInput
	srv := repairServer(t, &seen)
	defer srv.Close()

	pipe := repairPipeline(t, srv, t.TempDir(), "test-model", 2)
	if err := pipe.Run(context.Background(), repairSentences(), []string{"ru"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(seen) != 4 {
		t.Fatalf("proofread %d items, want 4", len(seen))
	}
	byID := map[string]repairInput{}
	for _, it := range seen {
		byID[it.Src] = it
	}
	for _, c := range []struct {
		src  string
		prev []string
	}{
		{"One.", nil},
		{"Two.", []string{"One."}},
		{"Three.", []string{"One.", "Two."}},
		{"Four.", []string{"Two.", "Three."}},
	} {
		got := byID[c.src]
		if len(got.Prev) != len(c.prev) {
			t.Fatalf("%s: %d prev items, want %d (%+v)", c.src, len(got.Prev), len(c.prev), got.Prev)
		}
		for i, want := range c.prev {
			if got.Prev[i].Src != want {
				t.Errorf("%s: prev[%d].src = %q, want %q", c.src, i, got.Prev[i].Src, want)
			}
			if got.Prev[i].Tr != "TR:"+want {
				t.Errorf("%s: prev[%d].tr = %q, want the neighbour's translation", c.src, i, got.Prev[i].Tr)
			}
		}
	}
}

// TestRepairContextIsOffByDefault: without --context the pass stays blind, so
// no neighbour text can leak into the prompt.
func TestRepairContextIsOffByDefault(t *testing.T) {
	var seen []repairInput
	srv := repairServer(t, &seen)
	defer srv.Close()

	pipe := repairPipeline(t, srv, t.TempDir(), "test-model", 0)
	if err := pipe.Run(context.Background(), repairSentences(), []string{"ru"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, it := range seen {
		if len(it.Prev) != 0 {
			t.Errorf("%s: got %d prev items with --context 0", it.Src, len(it.Prev))
		}
	}
}

// TestRepairContextOwnsItsCacheNamespace is the resumability contract: a
// context run must not adopt (or overwrite) the text a blind run proofread —
// they are different products — while the RAW translations both consume stay
// shared, so switching costs one proofread pass and never a re-translation.
func TestRepairContextCacheNamespace(t *testing.T) {
	var seen []repairInput
	srv := repairServer(t, &seen)
	defer srv.Close()

	cacheDir, model := t.TempDir(), "test-model"
	sentences := repairSentences()

	blind := repairPipeline(t, srv, cacheDir, model, 0)
	if err := blind.Run(context.Background(), sentences, []string{"ru"}); err != nil {
		t.Fatalf("blind run: %v", err)
	}
	translated := len(seen)

	seen = nil
	ctx2 := repairPipeline(t, srv, cacheDir, model, 2)
	if err := ctx2.Run(context.Background(), sentences, []string{"ru"}); err != nil {
		t.Fatalf("context run: %v", err)
	}
	if len(seen) != translated {
		t.Fatalf("context run proofread %d sentences, want all %d re-done in its own namespace",
			len(seen), translated)
	}
	for _, it := range seen {
		if strings.HasPrefix(it.Tr, "RP:") {
			t.Fatalf("%s: proofreading its own output %q — the raw namespace is not shared", it.Src, it.Tr)
		}
	}

	// Both texts survive side by side, each reachable under its own namespace.
	for _, c := range []struct{ model, want string }{
		{RepairTextCacheModel(model, 0), "RP:TR:One."},
		{RepairTextCacheModel(model, 2), "RP:TR:One."},
	} {
		tr, ok := cache.Read(cacheDir, cache.RepairKey("One.", "en", "ru", c.model))
		if !ok || tr.Text != c.want {
			t.Errorf("namespace %q: got %q (ok=%v), want %q", c.model, tr.Text, ok, c.want)
		}
	}
	if RepairCacheModel(model, 2) == RepairCacheModel(model, 0) {
		t.Error("final aligned entries share a namespace across context modes")
	}
}

// TestRepairPromptContextRules pins the rule swap: blind forbids touching a
// gender or referent, context tells the model to settle it from prev.
func TestRepairPromptContextRules(t *testing.T) {
	blind := repairSystemPrompt("English", "Russian", nil, 0)
	if !strings.Contains(blind, "ONE sentence at a time") {
		t.Error("blind prompt lost the no-context rule")
	}
	if strings.Contains(blind, `"prev"`) {
		t.Error("blind prompt mentions prev, which it never receives")
	}
	ctx2 := repairSystemPrompt("English", "Russian", nil, 2)
	if !strings.Contains(ctx2, `"prev"`) {
		t.Error("context prompt does not explain the prev field")
	}
	if strings.Contains(ctx2, "ONE sentence at a time") {
		t.Error("context prompt kept the contradictory no-context rule")
	}
	for _, p := range []string{blind, ctx2} {
		if !strings.Contains(p, "MOST ITEMS NEED NO CHANGE") {
			t.Error("prompt lost the no-change anchor")
		}
	}
}
