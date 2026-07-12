package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseChunks(t *testing.T) {
	cases := []struct {
		name, in string
		ids      int
	}{
		{"plain", `{"a":[{"tgt":"x","en":"X"}],"b":[]}`, 2},
		{"fenced", "```json\n{\"a\":[{\"tgt\":\"x\",\"en\":\"X\"}]}\n```", 1},
		{"prose-wrapped", "Here you go:\n{\"a\":[{\"tgt\":\"x\",\"en\":\"X\"}]}\nDone.", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := parseChunks(c.in)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if len(m) != c.ids {
				t.Fatalf("got %d ids, want %d", len(m), c.ids)
			}
		})
	}
	if _, err := parseChunks("not json at all"); err == nil {
		t.Errorf("expected error on garbage")
	}
}

func TestParseTexts(t *testing.T) {
	cases := []struct {
		name, in string
		ids      int
	}{
		{"plain", `{"a":"Привет","b":"Мир"}`, 2},
		{"fenced", "```json\n{\"a\":\"Привет\"}\n```", 1},
		{"prose-wrapped", "Here:\n{\"a\":\"Привет\"}\nDone.", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := parseTexts(c.in)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if len(m) != c.ids {
				t.Fatalf("got %d ids, want %d", len(m), c.ids)
			}
		})
	}
	if _, err := parseTexts("not json at all"); err == nil {
		t.Errorf("expected error on garbage")
	}
}

func TestBackoffHonorsRetryAfter(t *testing.T) {
	if got := backoff(0, 7_000_000_000); got != 7_000_000_000 {
		t.Errorf("retry-after not honored: %v", got)
	}
}

// The ollama provider must speak plain OpenAI chat-completions: no
// Authorization header without a key, no OpenRouter routing/usage extensions.
func TestOllamaRequestShape(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"1\":\"Привет\"}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	}))
	defer srv.Close()

	c := NewClient(Options{Provider: ProviderOllama, BaseURL: srv.URL,
		Model: "translategemma:12b", JSONMode: true, ProviderSort: "throughput"})
	out, err := c.TranslateText(context.Background(), "sys", `[{"id":"1","src":"Hi"}]`)
	if err != nil {
		t.Fatalf("TranslateText: %v", err)
	}
	if out["1"] != "Привет" {
		t.Errorf("out = %v", out)
	}
	if gotAuth != "" {
		t.Errorf("Authorization sent without a key: %q", gotAuth)
	}
	for _, k := range []string{"provider", "usage"} {
		if _, ok := gotBody[k]; ok {
			t.Errorf("OpenRouter-only field %q sent to ollama", k)
		}
	}
	if rf, ok := gotBody["response_format"].(map[string]any); !ok || rf["type"] != "json_object" {
		t.Errorf("response_format missing/wrong: %v", gotBody["response_format"])
	}
}

func TestCheckOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"object":"list","data":[{"id":"translategemma:12b"},{"id":"qwen3:latest"}]}`)
	}))
	defer srv.Close()

	if err := CheckOllama(srv.URL, "translategemma:12b"); err != nil {
		t.Errorf("exact tag: %v", err)
	}
	if err := CheckOllama(srv.URL, "qwen3"); err != nil {
		t.Errorf("bare name should match :latest: %v", err)
	}
	err := CheckOllama(srv.URL, "mistral:7b")
	if err == nil || !strings.Contains(err.Error(), "ollama pull mistral:7b") {
		t.Errorf("missing model should suggest a pull, got: %v", err)
	}
	srv.Close()
	if err := CheckOllama(srv.URL, "translategemma:12b"); err == nil ||
		!strings.Contains(err.Error(), "ollama serve") {
		t.Errorf("dead server should point at ollama serve, got: %v", err)
	}
}
