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
		// MiniMax-M2 (Gonka) emits its reasoning inline; the think text often
		// quotes JSON braces, which would derail the extractObject fallback.
		{"think-block", "<think>I should return {\"a\": ...} here.</think>\n{\"a\":\"Привет\"}", 1},
		{"think-then-fenced", "<think>plan</think>\n```json\n{\"a\":\"Привет\"}\n```", 1},
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
	// A truncated think block never reached the answer — must fail (and be
	// retried), not parse leftovers out of the reasoning text.
	if _, err := parseTexts(`<think>so {"a":"draft"} maybe`); err == nil {
		t.Errorf("expected error on an unterminated think block")
	}
}

func TestBackoffHonorsRetryAfter(t *testing.T) {
	if got := backoff(0, 7_000_000_000); got != 7_000_000_000 {
		t.Errorf("retry-after not honored: %v", got)
	}
}

// The local providers must speak plain OpenAI chat-completions: no
// Authorization header without a key, no OpenRouter routing/usage extensions.
func TestLocalProviderRequestShape(t *testing.T) {
	for _, provider := range []string{ProviderOllama, ProviderLlamaCpp} {
		t.Run(provider, func(t *testing.T) {
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

			c := NewClient(Options{Provider: provider, BaseURL: srv.URL,
				Model: "test-model", JSONMode: true, ProviderSort: "throughput"})
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
			for _, k := range []string{"provider", "usage", "reasoning", "thinking"} {
				if _, ok := gotBody[k]; ok {
					t.Errorf("extension field %q sent to %s", k, provider)
				}
			}
			if rf, ok := gotBody["response_format"].(map[string]any); !ok || rf["type"] != "json_object" {
				t.Errorf("response_format missing/wrong: %v", gotBody["response_format"])
			}
		})
	}
}

// Gonka speaks plain OpenAI chat-completions plus the thinking switch: no
// OpenRouter routing/usage extensions, thinking disabled, key sent as Bearer.
func TestGonkaRequestShape(t *testing.T) {
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

	stats, err := OpenStats(t.TempDir() + "/stats.jsonl")
	if err != nil {
		t.Fatalf("OpenStats: %v", err)
	}
	defer stats.Close()
	c := NewClient(Options{Provider: ProviderGonka, BaseURL: srv.URL, APIKey: "sekrit",
		Model: "moonshotai/Kimi-K2.6", JSONMode: true, ProviderSort: "throughput", Stats: stats})
	out, err := c.TranslateText(context.Background(), "sys", `[{"id":"1","src":"Hi"}]`)
	if err != nil {
		t.Fatalf("TranslateText: %v", err)
	}
	if out["1"] != "Привет" {
		t.Errorf("out = %v", out)
	}
	if gotAuth != "Bearer sekrit" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	for _, k := range []string{"provider", "usage", "reasoning"} {
		if _, ok := gotBody[k]; ok {
			t.Errorf("OpenRouter-only field %q sent to gonka", k)
		}
	}
	th, ok := gotBody["thinking"].(map[string]any)
	if !ok || th["type"] != "disabled" {
		t.Errorf("thinking not disabled: %v", gotBody["thinking"])
	}
}

// OpenRouter requests must carry the reasoning-off switch (hybrid-reasoning
// models reason by default, inflating cost ~8×) alongside routing/usage.
func TestOpenRouterReasoningDisabled(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"1\":\"Привет\"}"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	c := NewClient(Options{Provider: ProviderOpenRouter, BaseURL: srv.URL, APIKey: "k", Model: "z-ai/glm-5.2"})
	if _, err := c.TranslateText(context.Background(), "sys", `[{"id":"1","src":"Hi"}]`); err != nil {
		t.Fatalf("TranslateText: %v", err)
	}
	rs, ok := gotBody["reasoning"].(map[string]any)
	if !ok || rs["enabled"] != false {
		t.Errorf("reasoning not disabled: %v", gotBody["reasoning"])
	}
	if _, ok := gotBody["thinking"]; ok {
		t.Errorf("gonka-only thinking field sent to openrouter")
	}
}

func TestAdoptServedModel(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"object":"list","data":[{"id":"/home/u/.cache/llama.cpp/gemma-3-4b-it-Q4_K_M.gguf"}]}`)
	}))
	defer srv.Close()

	m, err := AdoptServedModel(srv.URL, "sekrit")
	if err != nil {
		t.Fatalf("AdoptServedModel: %v", err)
	}
	if m != "gemma-3-4b-it-Q4_K_M" {
		t.Errorf("adopted %q, want cleaned basename", m)
	}
	if gotAuth != "Bearer sekrit" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if !ServedBy([]string{"/home/u/.cache/llama.cpp/gemma-3-4b-it-Q4_K_M.gguf"}, "gemma-3-4b-it-Q4_K_M") {
		t.Errorf("ServedBy should match the cleaned form")
	}
	if ServedBy([]string{"other.gguf"}, "gemma-3-4b-it-Q4_K_M") {
		t.Errorf("ServedBy matched a different model")
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
