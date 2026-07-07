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

// TestTwoPassPipeline mocks OpenRouter and verifies the decoupled flow: a
// translate sweep ({id,src}→text) fills the tr-cache, then an align sweep
// ({id,src,tr}→chunks) fills the final cache. Also exercises cache.Invalidate.
func TestTwoPassPipeline(t *testing.T) {
	var translateCalls, alignCalls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Pull the user message (the inputs JSON array) out of the chat request.
		var req struct {
			Messages []struct {
				Role, Content string
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		userJSON := req.Messages[len(req.Messages)-1].Content
		var items []map[string]any
		_ = json.Unmarshal([]byte(userJSON), &items)

		out := map[string]any{}
		isAlign := len(items) > 0 && items[0]["tr"] != nil
		for _, it := range items {
			id, _ := it["id"].(string)
			if isAlign {
				// one chunk per whitespace token of tr; en = that token (identity)
				tr, _ := it["tr"].(string)
				var chunks []map[string]any
				for tok := range strings.FieldsSeq(tr) {
					chunks = append(chunks, map[string]any{"tgt": tok, "en": tok})
				}
				out[id] = chunks
			} else {
				// translate phase: identity translation (tr == src)
				out[id] = it["src"]
			}
		}
		if isAlign {
			alignCalls++
		} else {
			translateCalls++
		}
		content, _ := json.Marshal(out)
		resp := map[string]any{"choices": []map[string]any{{"message": map[string]any{"content": string(content)}}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	model := "test-model"
	client := NewClient(Options{BaseURL: srv.URL, APIKey: "x", Model: model,
		Temperature: 0, MaxRetries: 2, Timeout: 5 * time.Second})
	pipe := &Pipeline{Client: client, CacheDir: cacheDir, Source: "en", BatchSize: 16, Concurrency: 2}

	sentences := []*tbook.Sentence{
		{Src: "Hello world.", Words: [][2]int{{0, 5}, {6, 11}}, Tr: map[string]tbook.Translation{}},
		{Src: "Stan went home.", Words: [][2]int{{0, 4}, {5, 9}, {10, 14}}, Tr: map[string]tbook.Translation{}},
	}

	if err := pipe.Run(context.Background(), sentences, []string{"ru"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if translateCalls == 0 || alignCalls == 0 {
		t.Fatalf("expected both phases to run, got translate=%d align=%d", translateCalls, alignCalls)
	}

	// Both phases cached → nothing pending; final entries are aligned.
	if p := CountPending(sentences, []string{"ru"}, cacheDir, "en", model, false); p != 0 {
		t.Fatalf("expected 0 pending after run, got %d", p)
	}
	found, missing := FillFromCache(sentences, []string{"ru"}, cacheDir, "en", model)
	if found != 2 || missing != 0 {
		t.Fatalf("fill: found=%d missing=%d", found, missing)
	}
	for _, s := range sentences {
		tr := s.Tr["ru"]
		if tr.Text == "" || len(tr.Align) == 0 {
			t.Fatalf("sentence %q: empty text or no align (%+v)", s.Src, tr)
		}
	}

	// Invalidate one sentence → it becomes pending again (translation+alignment cleared).
	removed := cache.Invalidate(cacheDir, []string{"Hello world."}, []string{"ru"}, "en", model)
	if removed != 2 {
		t.Fatalf("expected 2 cache files removed, got %d", removed)
	}
	if p := CountPending(sentences, []string{"ru"}, cacheDir, "en", model, false); p != 1 {
		t.Fatalf("expected 1 pending after invalidate, got %d", p)
	}
}

func TestEnsureListPrefix(t *testing.T) {
	cases := []struct{ src, text, want string }{
		{"2. Depués de hacer diez burpees.", "Сделав десять берпи.", "2. Сделав десять берпи."},
		{"2. Depués de hacer.", "2. После выполнения.", "2. После выполнения."},
		{"12) Item.", "12) Пункт.", "12) Пункт."},
		{"No marker here.", "Здесь нет маркера.", "Здесь нет маркера."},
		{"1. Marker.", "", ""}, // empty translation left alone
	}
	for _, c := range cases {
		if got := ensureListPrefix(c.src, c.text); got != c.want {
			t.Errorf("ensureListPrefix(%q, %q) = %q, want %q", c.src, c.text, got, c.want)
		}
	}
}
