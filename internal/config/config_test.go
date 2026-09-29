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
		"GONKA_MODEL", "GONKA_BASE_URL", "GONKA_API_KEY",
		"OPENROUTER_API_KEY", "OPENROUTER_BASE_URL", "CONCURRENCY", "BATCH_SIZE", "MAX_RETRIES", "REQUEST_TIMEOUT_SEC",
		"JUDGE_MODEL", "ESCALATE_MODEL", "ALIGN_MODE", "REPAIR_CONTEXT",
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
	if cfg.MaxRetries != 4 {
		t.Errorf("MaxRetries = %d, want 4", cfg.MaxRetries)
	}
	if cfg.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s", cfg.Timeout)
	}
}

// The proofread pass is on for gonka (free tokens) and off elsewhere; both
// flags and both env vars override that either way.
func TestRepairDefaults(t *testing.T) {
	clearProviderEnv(t)
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"gonka on by default", []string{"book.epub", "--provider", "gonka"}, true},
		{"openrouter off by default", []string{"book.epub"}, false},
		{"gonka opts out", []string{"book.epub", "--provider", "gonka", "--no-repair"}, false},
		{"openrouter opts in", []string{"book.epub", "--repair"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := Load(c.args)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Repair != c.want {
				t.Errorf("Repair = %v, want %v", cfg.Repair, c.want)
			}
		})
	}
}

func TestRepairEnvOverridesGonkaDefault(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("NO_REPAIR", "1")
	cfg, err := Load([]string{"book.epub", "--provider", "gonka"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Repair {
		t.Error("Repair = true, want false (NO_REPAIR=1 must beat the gonka default)")
	}
}

// TestRepairContext pins the accepted values of --context: off by default, 2
// for neighbour context, and a hard refusal of 1 — measured worse than no
// context at all, so it must never reach a book by accident.
func TestRepairContext(t *testing.T) {
	clearProviderEnv(t)
	cases := []struct {
		name    string
		args    []string
		env     string
		want    int
		wantErr bool
	}{
		{name: "off by default", args: []string{"book.epub"}, want: 0},
		{name: "flag", args: []string{"book.epub", "--context", "2"}, want: 2},
		{name: "env", args: []string{"book.epub"}, env: "2", want: 2},
		{name: "flag beats env", args: []string{"book.epub", "--context", "0"}, env: "2", want: 0},
		{name: "one rejected", args: []string{"book.epub", "--context", "1"}, wantErr: true},
		{name: "negative rejected", args: []string{"book.epub", "--context", "-2"}, wantErr: true},
		{name: "env one rejected", args: []string{"book.epub"}, env: "1", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("REPAIR_CONTEXT", c.env)
			cfg, err := Load(c.args)
			if c.wantErr {
				if err == nil {
					t.Fatalf("Load: want error, got RepairContext = %d", cfg.RepairContext)
				}
				if !strings.Contains(err.Error(), "--context") {
					t.Errorf("error %q should name the flag", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.RepairContext != c.want {
				t.Errorf("RepairContext = %d, want %d", cfg.RepairContext, c.want)
			}
		})
	}
}

func TestGonkaDefaults(t *testing.T) {
	clearProviderEnv(t)
	cfg, err := Load([]string{"book.epub", "--provider", "gonka"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != defaultGonkaModel {
		t.Errorf("Model = %q, want %q", cfg.Model, defaultGonkaModel)
	}
	// Batch, retries, concurrency and timeout all follow the served-model probe
	// (bench-quality/reports/gonka-models-2026-09-19.md): the gateway's budget
	// is shared, so 8 wide is as good as 32, and past 60 s a request is parked
	// rather than slow.
	if cfg.BatchSize != defaultGonkaBatch {
		t.Errorf("BatchSize = %d, want %d", cfg.BatchSize, defaultGonkaBatch)
	}
	if cfg.MaxRetries != defaultGonkaRetries {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, defaultGonkaRetries)
	}
	if cfg.Concurrency != defaultGonkaConcurrency {
		t.Errorf("Concurrency = %d, want %d", cfg.Concurrency, defaultGonkaConcurrency)
	}
	if cfg.Timeout != defaultGonkaTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, defaultGonkaTimeout)
	}
}

// The langcheck gate is on by default and --langcheck-gate=false keeps the old
// report-only behaviour, for A/B measurement.
func TestLangcheckGateDefault(t *testing.T) {
	clearProviderEnv(t)
	cfg, err := Load([]string{"book.epub"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.LangcheckGate {
		t.Error("LangcheckGate = false, want the gate on by default")
	}
	off, err := Load([]string{"book.epub", "--langcheck-gate=false"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if off.LangcheckGate {
		t.Error("--langcheck-gate=false did not turn the gate off")
	}
}

func TestTypographyDefaultOn(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("TYPOGRAPHY", "")
	cfg, err := Load([]string{"book.epub"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Typography {
		t.Error("Typography = false, want on by default")
	}
	off, err := Load([]string{"book.epub", "--typography=false"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if off.Typography {
		t.Error("--typography=false did not turn the pass off")
	}
}

func TestGonkaExplicitOverrides(t *testing.T) {
	clearProviderEnv(t)
	cfg, err := Load([]string{"book.epub", "--provider", "gonka",
		"--batch-size", "16", "--max-retries", "2"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BatchSize != 16 {
		t.Errorf("flag --batch-size lost: got %d, want 16", cfg.BatchSize)
	}
	explicit, err := Load([]string{"book.epub", "--provider", "gonka", "--concurrency", "24"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if explicit.Concurrency != 24 {
		t.Errorf("flag --concurrency lost to the gonka default: got %d, want 24", explicit.Concurrency)
	}
	if cfg.MaxRetries != 2 {
		t.Errorf("flag --max-retries lost: got %d, want 2", cfg.MaxRetries)
	}
}

func TestUnknownProviderError(t *testing.T) {
	clearProviderEnv(t)
	_, err := Load([]string{"book.epub", "--provider", "llamafile"})
	if err == nil || !strings.Contains(err.Error(), "ollama") {
		t.Fatalf("want an error listing ollama among providers, got %v", err)
	}
}

func TestJudgeEscalateProviders(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("JUDGE_PROVIDER", "")
	t.Setenv("ESCALATE_PROVIDER", "")
	t.Setenv("GONKA_API_KEY", "gk")
	t.Setenv("OPENROUTER_API_KEY", "ork")

	cfg, err := Load([]string{"book.epub", "--provider", "gonka"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JudgeProvider != ProviderGonka || cfg.EscalateProvider != ProviderGonka {
		t.Errorf("default providers = %q/%q, want gonka", cfg.JudgeProvider, cfg.EscalateProvider)
	}

	cfg, err = Load([]string{"book.epub", "--provider", "gonka",
		"--judge-provider", "OpenRouter", "--judge-model", "google/gemini-3.7-flash",
		"--escalate-provider", "openrouter", "--escalate-model", "google/gemini-3.7-flash"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JudgeProvider != ProviderOpenRouter || cfg.EscalateProvider != ProviderOpenRouter {
		t.Errorf("providers = %q/%q, want openrouter", cfg.JudgeProvider, cfg.EscalateProvider)
	}
	if cfg.JudgeModel != "google/gemini-3.7-flash" || cfg.EscalateModel != "google/gemini-3.7-flash" {
		t.Errorf("models = %q/%q", cfg.JudgeModel, cfg.EscalateModel)
	}
	if cfg.APIKey != "gk" || !strings.Contains(cfg.BaseURL, "gonka") {
		t.Errorf("translate endpoint = %q (key %q), want gonka's", cfg.BaseURL, cfg.APIKey)
	}
	if u, k := Endpoint(cfg.JudgeProvider); k != "ork" || u != "https://openrouter.ai/api/v1" {
		t.Errorf("judge endpoint = %q (key %q), want OpenRouter's", u, k)
	}

	// A vendor-prefixed id is an OpenRouter id: kept for an OpenRouter judge
	// even though the translate provider is local.
	cfg, err = Load([]string{"book.epub", "--provider", "ollama",
		"--judge-provider", "openrouter", "--judge-model", "google/gemini-3.7-flash"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JudgeModel != "google/gemini-3.7-flash" {
		t.Errorf("JudgeModel = %q, dropped by the local-provider hygiene", cfg.JudgeModel)
	}

	for _, args := range [][]string{
		{"book.epub", "--provider", "gonka", "--judge-provider", "openrouter"},
		{"book.epub", "--provider", "gonka", "--escalate-provider", "openrouter"},
		{"book.epub", "--judge-provider", "bogus", "--judge-model", "x"},
	} {
		if _, err := Load(args); err == nil {
			t.Errorf("Load(%v) accepted", args)
		}
	}
}
