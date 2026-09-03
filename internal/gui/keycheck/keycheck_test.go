package keycheck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testKey = "sk-or-v1-secret-test-key"

// deadServerURL returns a URL nothing listens on.
func deadServerURL(t *testing.T) string {
	t.Helper()
	ts := httptest.NewServer(http.NotFoundHandler())
	url := ts.URL
	ts.Close()
	return url
}

func TestCheckOpenRouter(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		ctxTimeout time.Duration
		dead       bool
		want       Status
		detailHas  string
		warnHas    string
		check      func(t *testing.T, r Result)
	}{
		{
			name: "ok wrapped",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer "+testKey {
					t.Error("missing bearer auth")
				}
				w.Write([]byte(`{"data":{"label":"k","usage":1.234,"limit":10,"limit_remaining":8.77,"is_free_tier":false}}`))
			},
			want:      StatusOK,
			detailHas: "$1.23 used",
			check: func(t *testing.T, r Result) {
				if r.UsageUSD != 1.234 || r.LimitRemaining == nil || *r.LimitRemaining != 8.77 {
					t.Errorf("usage/limit not carried over: %+v", r)
				}
			},
		},
		{
			name: "ok bare object free tier",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"usage":0.5,"is_free_tier":true}`))
			},
			want:    StatusOK,
			warnHas: "402",
			check: func(t *testing.T, r Result) {
				if !r.IsFreeTier {
					t.Error("IsFreeTier not set")
				}
			},
		},
		{
			name: "limit exhausted",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"data":{"usage":5,"limit":5,"limit_remaining":0}}`))
			},
			want:      StatusNoCredits,
			detailHas: "credit limit is exhausted",
		},
		{
			name:      "401",
			handler:   func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) },
			want:      StatusInvalidKey,
			detailHas: "sk-or-",
		},
		{
			name:      "402",
			handler:   func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(402) },
			want:      StatusNoCredits,
			detailHas: "credits",
		},
		{
			name:    "429",
			handler: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) },
			want:    StatusOK,
			warnHas: "rate limited",
		},
		{
			name: "timeout",
			handler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(500 * time.Millisecond)
			},
			ctxTimeout: 50 * time.Millisecond,
			want:       StatusUnreachable,
		},
		{
			name:      "garbage body",
			handler:   func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>oops")) },
			want:      StatusUnreachable,
			detailHas: "unexpected reply",
		},
		{
			name: "key echoed by server is scrubbed",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
				w.Write([]byte("boom " + r.Header.Get("Authorization")))
			},
			want: StatusUnreachable,
		},
		{
			name: "unreachable",
			dead: true,
			want: StatusUnreachable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var base string
			if tc.dead {
				base = deadServerURL(t)
			} else {
				ts := httptest.NewServer(tc.handler)
				defer ts.Close()
				base = ts.URL
			}
			ctx := context.Background()
			if tc.ctxTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.ctxTimeout)
				defer cancel()
			}
			r := CheckOpenRouter(ctx, base, testKey)
			if r.Status != tc.want {
				t.Fatalf("status = %q, want %q (detail: %s)", r.Status, tc.want, r.Detail)
			}
			if tc.detailHas != "" && !strings.Contains(r.Detail, tc.detailHas) {
				t.Errorf("detail %q missing %q", r.Detail, tc.detailHas)
			}
			if tc.warnHas != "" && !strings.Contains(r.Warning, tc.warnHas) {
				t.Errorf("warning %q missing %q", r.Warning, tc.warnHas)
			}
			if strings.Contains(r.Detail+r.Warning, testKey) {
				t.Errorf("key leaked into result: %+v", r)
			}
			if tc.check != nil {
				tc.check(t, r)
			}
		})
	}
}

func TestCheckOpenRouterDefaultsBaseURL(t *testing.T) {
	// "" base must target the real endpoint; just check it doesn't panic and
	// classifies a canceled context as unreachable without a network call.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r := CheckOpenRouter(ctx, "", testKey); r.Status != StatusUnreachable {
		t.Fatalf("status = %q, want unreachable", r.Status)
	}
}

// gonkaServer builds a gateway fake: /models returns ids, /chat/completions
// is classified per requested model.
func gonkaServer(t *testing.T, ids []string, modelsStatus int, completion func(model string, w http.ResponseWriter)) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		if modelsStatus != 200 {
			w.WriteHeader(modelsStatus)
			return
		}
		type m struct {
			ID string `json:"id"`
		}
		list := struct {
			Data []m `json:"data"`
		}{}
		for _, id := range ids {
			list.Data = append(list.Data, m{ID: id})
		}
		json.NewEncoder(w).Encode(list)
	})
	mux.HandleFunc("/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("bad completion payload: %v", err)
		}
		if req.MaxTokens != 1 {
			t.Errorf("max_tokens = %d, want 1", req.MaxTokens)
		}
		completion(req.Model, w)
	})
	return httptest.NewServer(mux)
}

func TestCheckGonka(t *testing.T) {
	ok := func(model string, w http.ResponseWriter) {
		w.Write([]byte(`{"choices":[{"message":{"content":"p"}}]}`))
	}
	status := func(code int, body string) func(string, http.ResponseWriter) {
		return func(model string, w http.ResponseWriter) {
			w.WriteHeader(code)
			w.Write([]byte(body))
		}
	}

	t.Run("ok", func(t *testing.T) {
		ts := gonkaServer(t, []string{"moonshotai/Kimi-K2.6"}, 200, ok)
		defer ts.Close()
		r := CheckGonka(context.Background(), ts.URL, testKey, "")
		if r.Status != StatusOK {
			t.Fatalf("status = %q (%s)", r.Status, r.Detail)
		}
	})
	t.Run("models 401", func(t *testing.T) {
		ts := gonkaServer(t, nil, 401, ok)
		defer ts.Close()
		r := CheckGonka(context.Background(), ts.URL, testKey, "")
		if r.Status != StatusInvalidKey {
			t.Fatalf("status = %q (%s)", r.Status, r.Detail)
		}
	})
	t.Run("unreachable", func(t *testing.T) {
		r := CheckGonka(context.Background(), deadServerURL(t), testKey, "")
		if r.Status != StatusUnreachable {
			t.Fatalf("status = %q (%s)", r.Status, r.Detail)
		}
	})
	t.Run("models ok completion 401", func(t *testing.T) {
		ts := gonkaServer(t, []string{"a"}, 200, status(401, `{"error":{"message":"bad key"}}`))
		defer ts.Close()
		r := CheckGonka(context.Background(), ts.URL, testKey, "a")
		if r.Status != StatusInvalidKey {
			t.Fatalf("status = %q (%s)", r.Status, r.Detail)
		}
	})
	t.Run("completion 402", func(t *testing.T) {
		ts := gonkaServer(t, []string{"a"}, 200, status(402, "no credits"))
		defer ts.Close()
		r := CheckGonka(context.Background(), ts.URL, testKey, "a")
		if r.Status != StatusNoCredits {
			t.Fatalf("status = %q (%s)", r.Status, r.Detail)
		}
	})
	t.Run("completion 429", func(t *testing.T) {
		ts := gonkaServer(t, []string{"a"}, 200, status(429, "busy"))
		defer ts.Close()
		r := CheckGonka(context.Background(), ts.URL, testKey, "a")
		if r.Status != StatusOK || !strings.Contains(r.Warning, "saturated") {
			t.Fatalf("status = %q warning = %q", r.Status, r.Warning)
		}
	})
	t.Run("model 404 retry with served id", func(t *testing.T) {
		ts := gonkaServer(t, []string{"served-1"}, 200, func(model string, w http.ResponseWriter) {
			if model == "served-1" {
				ok(model, w)
				return
			}
			status(404, `{"error":{"message":"model not found"}}`)(model, w)
		})
		defer ts.Close()
		r := CheckGonka(context.Background(), ts.URL, testKey, "gone-model")
		if r.Status != StatusOK || !strings.Contains(r.Warning, "served-1") {
			t.Fatalf("status = %q warning = %q (%s)", r.Status, r.Warning, r.Detail)
		}
	})
	t.Run("model missing everywhere", func(t *testing.T) {
		ts := gonkaServer(t, []string{"served-1"}, 200, status(404, `{"error":{"message":"model not found"}}`))
		defer ts.Close()
		r := CheckGonka(context.Background(), ts.URL, testKey, "gone-model")
		if r.Status != StatusModelMissing {
			t.Fatalf("status = %q (%s)", r.Status, r.Detail)
		}
		if len(r.Models) != 1 || r.Models[0] != "served-1" {
			t.Errorf("Models = %v", r.Models)
		}
	})
}

// modelsServer serves the OpenAI-style GET /models list ServedModels expects.
func modelsServer(t *testing.T, ids ...string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		type m struct {
			ID string `json:"id"`
		}
		list := struct {
			Data []m `json:"data"`
		}{Data: []m{}}
		for _, id := range ids {
			list.Data = append(list.Data, m{ID: id})
		}
		json.NewEncoder(w).Encode(list)
	}))
}

func TestCheckOllamaServer(t *testing.T) {
	t.Run("exact tag served", func(t *testing.T) {
		ts := modelsServer(t, "translategemma:12b", "llama3:latest")
		defer ts.Close()
		r := CheckOllamaServer(context.Background(), ts.URL, "translategemma:12b")
		if r.Status != StatusOK {
			t.Fatalf("status = %q (%s)", r.Status, r.Detail)
		}
		if len(r.Models) != 2 {
			t.Errorf("Models = %v", r.Models)
		}
	})
	t.Run("bare name matches latest", func(t *testing.T) {
		ts := modelsServer(t, "llama3:latest")
		defer ts.Close()
		r := CheckOllamaServer(context.Background(), ts.URL, "llama3")
		if r.Status != StatusOK {
			t.Fatalf("status = %q (%s)", r.Status, r.Detail)
		}
	})
	t.Run("model missing", func(t *testing.T) {
		ts := modelsServer(t, "llama3:latest")
		defer ts.Close()
		r := CheckOllamaServer(context.Background(), ts.URL, "translategemma:12b")
		if r.Status != StatusModelMissing {
			t.Fatalf("status = %q (%s)", r.Status, r.Detail)
		}
		if !strings.Contains(r.Detail, "ollama pull translategemma:12b") {
			t.Errorf("detail %q missing pull hint", r.Detail)
		}
		if len(r.Models) != 1 {
			t.Errorf("Models = %v", r.Models)
		}
	})
	t.Run("server down", func(t *testing.T) {
		r := CheckOllamaServer(context.Background(), deadServerURL(t), "")
		if r.Status != StatusUnreachable || !strings.Contains(r.Detail, "ollama serve") {
			t.Fatalf("status = %q detail = %q", r.Status, r.Detail)
		}
	})
}

func TestCheckLlamaCpp(t *testing.T) {
	t.Run("cleaned gguf name matches", func(t *testing.T) {
		ts := modelsServer(t, "/models/gemma-3-4b-it.gguf")
		defer ts.Close()
		r := CheckLlamaCpp(context.Background(), ts.URL, "", "gemma-3-4b-it")
		if r.Status != StatusOK || r.Warning != "" {
			t.Fatalf("status = %q warning = %q", r.Status, r.Warning)
		}
	})
	t.Run("mismatch is a warning", func(t *testing.T) {
		ts := modelsServer(t, "foo.gguf")
		defer ts.Close()
		r := CheckLlamaCpp(context.Background(), ts.URL, "", "bar")
		if r.Status != StatusOK || !strings.Contains(r.Warning, `"foo.gguf"`) {
			t.Fatalf("status = %q warning = %q", r.Status, r.Warning)
		}
	})
	t.Run("no loaded model", func(t *testing.T) {
		ts := modelsServer(t)
		defer ts.Close()
		r := CheckLlamaCpp(context.Background(), ts.URL, "", "")
		if r.Status != StatusModelMissing || !strings.Contains(r.Detail, "no loaded model") {
			t.Fatalf("status = %q detail = %q", r.Status, r.Detail)
		}
	})
	t.Run("server down", func(t *testing.T) {
		r := CheckLlamaCpp(context.Background(), deadServerURL(t), testKey, "")
		if r.Status != StatusUnreachable || !strings.Contains(r.Detail, "llama-server") {
			t.Fatalf("status = %q detail = %q", r.Status, r.Detail)
		}
		if strings.Contains(r.Detail, testKey) {
			t.Error("key leaked into detail")
		}
	})
}

type fakeExec struct {
	lookFn func(file string) (string, error)
	runFn  func(bin string, args ...string) (string, string, error)
	envs   [][]string
}

func (f *fakeExec) lookPath(file string) (string, error) { return f.lookFn(file) }

func (f *fakeExec) run(_ context.Context, _ time.Duration, env []string, bin string, args ...string) (string, string, error) {
	f.envs = append(f.envs, env)
	return f.runFn(bin, args...)
}

func lookOnPath(path string) func(string) (string, error) {
	return func(file string) (string, error) {
		if file == "claude" {
			return path, nil
		}
		return "", errors.New("not found")
	}
}

func TestCheckClaudeCLI(t *testing.T) {
	version := "2.1.232 (Claude Code)"
	dispatch := func(authOut, authErrOut string, authErr error) func(string, ...string) (string, string, error) {
		return func(bin string, args ...string) (string, string, error) {
			if args[0] == "--version" {
				return version + "\n", "", nil
			}
			return authOut, authErrOut, authErr
		}
	}

	t.Run("logged in", func(t *testing.T) {
		ex := &fakeExec{lookFn: lookOnPath("/usr/bin/claude"),
			runFn: dispatch(`{"loggedIn":true,"subscriptionType":"max","email":"u@example.com"}`, "", nil)}
		r := checkClaudeCLI(context.Background(), "", ex)
		if r.Status != StatusOK {
			t.Fatalf("status = %q (%s)", r.Status, r.Detail)
		}
		if r.ResolvedBin != "/usr/bin/claude" || r.ClaudeVersion != version || r.Subscription != "max" {
			t.Errorf("fields: %+v", r)
		}
		if !strings.Contains(r.Detail, "u@example.com") {
			t.Errorf("detail %q missing email", r.Detail)
		}
	})
	t.Run("logged out", func(t *testing.T) {
		ex := &fakeExec{lookFn: lookOnPath("/usr/bin/claude"),
			runFn: dispatch(`{"loggedIn":false}`, "", errors.New("exit status 1"))}
		r := checkClaudeCLI(context.Background(), "", ex)
		if r.Status != StatusNotLoggedIn || !strings.Contains(r.Detail, "log in") {
			t.Fatalf("status = %q detail = %q", r.Status, r.Detail)
		}
	})
	t.Run("not installed", func(t *testing.T) {
		ex := &fakeExec{lookFn: func(string) (string, error) { return "", errors.New("not found") },
			runFn: dispatch("", "", nil)}
		r := checkClaudeCLI(context.Background(), "", ex)
		if r.Status != StatusNotInstalled {
			t.Fatalf("status = %q", r.Status)
		}
	})
	t.Run("found in probe dir", func(t *testing.T) {
		ex := &fakeExec{
			lookFn: func(file string) (string, error) {
				if file == "claude" {
					return "", errors.New("not in PATH")
				}
				if strings.HasSuffix(file, "/claude") {
					return file, nil
				}
				return "", errors.New("not found")
			},
			runFn: dispatch(`{"loggedIn":true,"email":"u@example.com"}`, "", nil),
		}
		r := checkClaudeCLI(context.Background(), "", ex)
		if r.Status != StatusOK || r.ResolvedBin == "" || r.ResolvedBin == "claude" {
			t.Fatalf("status = %q resolvedBin = %q", r.Status, r.ResolvedBin)
		}
	})
	t.Run("broken binary", func(t *testing.T) {
		ex := &fakeExec{lookFn: lookOnPath("/usr/bin/claude"),
			runFn: func(bin string, args ...string) (string, string, error) {
				return "", "", errors.New("exec format error")
			}}
		r := checkClaudeCLI(context.Background(), "", ex)
		if r.Status != StatusNotInstalled || !strings.Contains(r.Detail, "broken") {
			t.Fatalf("status = %q detail = %q", r.Status, r.Detail)
		}
	})
	t.Run("old CLI without auth status", func(t *testing.T) {
		ex := &fakeExec{lookFn: lookOnPath("/usr/bin/claude"),
			runFn: dispatch("", "Unknown command: auth", errors.New("exit status 1"))}
		r := checkClaudeCLI(context.Background(), "", ex)
		if r.Status != StatusOK || !strings.Contains(r.Warning, "login state unknown") {
			t.Fatalf("status = %q warning = %q", r.Status, r.Warning)
		}
		if r.ClaudeVersion != version {
			t.Errorf("version = %q", r.ClaudeVersion)
		}
	})
}

func TestClaudeProbeEnvStripsAnthropicKeys(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-secret")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "tok")
	t.Setenv("ANTHROPIC_MODEL", "keepme")
	for _, kv := range claudeProbeEnv() {
		if strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") || strings.HasPrefix(kv, "ANTHROPIC_AUTH_TOKEN=") {
			t.Fatalf("credential not stripped: %s", kv)
		}
	}
	found := false
	for _, kv := range claudeProbeEnv() {
		if kv == "ANTHROPIC_MODEL=keepme" {
			found = true
		}
	}
	if !found {
		t.Error("unrelated ANTHROPIC_ var was stripped")
	}
}
