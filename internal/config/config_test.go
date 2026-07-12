package config

import (
	"strings"
	"testing"
	"time"
)

// clearProviderEnv blanks every env var that feeds provider resolution so
// tests see the built-in defaults regardless of the developer's shell/.env.
func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"PROVIDER", "MODEL", "CLAUDE_MODEL", "OLLAMA_MODEL", "OLLAMA_BASE_URL", "OLLAMA_API_KEY",
		"LLAMACPP_MODEL", "LLAMACPP_BASE_URL", "LLAMACPP_API_KEY",
		"OPENROUTER_API_KEY", "OPENROUTER_BASE_URL", "CONCURRENCY", "BATCH_SIZE", "REQUEST_TIMEOUT_SEC",
		"JUDGE_MODEL", "ESCALATE_MODEL", "ALIGN_MODE",
	} {
		t.Setenv(k, "")
	}
}

func TestOllamaDefaults(t *testing.T) {
	clearProviderEnv(t)
	cfg, err := Load([]string{"book.epub", "--provider", "ollama"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != defaultOllamaModel {
		t.Errorf("Model = %q, want %q", cfg.Model, defaultOllamaModel)
	}
	if cfg.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty (Ollama is keyless)", cfg.APIKey)
	}
	if cfg.Concurrency != defaultLocalConcurrency {
		t.Errorf("Concurrency = %d, want %d", cfg.Concurrency, defaultLocalConcurrency)
	}
	if cfg.BatchSize != defaultLocalBatch {
		t.Errorf("BatchSize = %d, want %d", cfg.BatchSize, defaultLocalBatch)
	}
	if cfg.Timeout != 300*time.Second {
		t.Errorf("Timeout = %v, want 300s", cfg.Timeout)
	}
}

func TestOllamaExplicitOverrides(t *testing.T) {
	clearProviderEnv(t)
	cfg, err := Load([]string{"book.epub", "--provider", "ollama", "--concurrency", "8", "--model", "qwen3:8b"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Concurrency != 8 {
		t.Errorf("flag --concurrency lost: got %d, want 8", cfg.Concurrency)
	}
	if cfg.Model != "qwen3:8b" {
		t.Errorf("flag --model lost: got %q", cfg.Model)
	}

	t.Setenv("CONCURRENCY", "6")
	t.Setenv("BATCH_SIZE", "12")
	t.Setenv("OLLAMA_BASE_URL", "http://gpu-box:11434/v1/")
	cfg, err = Load([]string{"book.epub", "--provider", "ollama"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Concurrency != 6 {
		t.Errorf("CONCURRENCY env lost: got %d, want 6", cfg.Concurrency)
	}
	if cfg.BatchSize != 12 {
		t.Errorf("BATCH_SIZE env lost: got %d, want 12", cfg.BatchSize)
	}
	if cfg.BaseURL != "http://gpu-box:11434/v1" {
		t.Errorf("OLLAMA_BASE_URL not applied/trimmed: %q", cfg.BaseURL)
	}
}

// OpenRouter model ids left in .env (vendor/model, no tag) must not leak into
// the judge/escalate passes under the ollama provider; registry-style Ollama
// ids ("hf.co/org/model:Q4") must survive.
func TestOllamaDropsOpenRouterModelLeftovers(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("JUDGE_MODEL", "google/gemini-2.5-flash")
	t.Setenv("ESCALATE_MODEL", "hf.co/org/model:Q4")
	cfg, err := Load([]string{"book.epub", "--provider", "ollama"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JudgeModel != cfg.Model {
		t.Errorf("JudgeModel = %q, want fallback to %q", cfg.JudgeModel, cfg.Model)
	}
	if cfg.EscalateModel != "hf.co/org/model:Q4" {
		t.Errorf("EscalateModel = %q, want the hf.co id kept", cfg.EscalateModel)
	}
}

func TestLlamaCppDefaults(t *testing.T) {
	clearProviderEnv(t)
	cfg, err := Load([]string{"book.epub", "--provider", "llama.cpp"}) // alias spelling
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Provider != ProviderLlamaCpp {
		t.Errorf("alias not normalized: Provider = %q", cfg.Provider)
	}
	if cfg.Model != "" {
		t.Errorf("Model = %q, want empty (adopted from the server at run time)", cfg.Model)
	}
	if cfg.BaseURL != "http://localhost:8080/v1" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Concurrency != defaultLocalConcurrency || cfg.BatchSize != defaultLocalBatch {
		t.Errorf("Concurrency/BatchSize = %d/%d, want %d/%d",
			cfg.Concurrency, cfg.BatchSize, defaultLocalConcurrency, defaultLocalBatch)
	}
	if cfg.Timeout != 300*time.Second {
		t.Errorf("Timeout = %v, want 300s", cfg.Timeout)
	}

	t.Setenv("LLAMACPP_MODEL", "gemma-3-4b-it-Q4_K_M")
	t.Setenv("LLAMACPP_API_KEY", "sekrit")
	cfg, err = Load([]string{"book.epub", "--provider", "llamacpp"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "gemma-3-4b-it-Q4_K_M" {
		t.Errorf("LLAMACPP_MODEL lost: %q", cfg.Model)
	}
	if cfg.APIKey != "sekrit" {
		t.Errorf("LLAMACPP_API_KEY lost: %q", cfg.APIKey)
	}
}

func TestOpenRouterDefaultsUnchanged(t *testing.T) {
	clearProviderEnv(t)
	cfg, err := Load([]string{"book.epub"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Provider != ProviderOpenRouter {
		t.Errorf("Provider = %q", cfg.Provider)
	}
	if cfg.Model != defaultOpenRouterModel {
		t.Errorf("Model = %q", cfg.Model)
	}
	if cfg.BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Concurrency != 32 {
		t.Errorf("Concurrency = %d, want 32", cfg.Concurrency)
	}
	if cfg.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s", cfg.Timeout)
	}
}

func TestUnknownProviderError(t *testing.T) {
	clearProviderEnv(t)
	_, err := Load([]string{"book.epub", "--provider", "llamafile"})
	if err == nil || !strings.Contains(err.Error(), "ollama") {
		t.Fatalf("want an error listing ollama among providers, got %v", err)
	}
}
