//go:build unix

package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The runner builds the child environment from scratch, so the classic
// GO_WANT_HELPER_PROCESS argv trick can't work; instead tests inject a marker
// through Runner.extraEnv and TestMain dispatches on it before running tests.
func TestMain(m *testing.M) {
	if os.Getenv("TBOOK_RUNNER_TEST_HELPER") == "1" {
		os.Exit(helperMain())
	}
	os.Exit(m.Run())
}

func helperMain() int {
	args := os.Args[1:]
	if len(args) == 0 || args[0] != "convert" {
		fmt.Fprintln(os.Stderr, "helper: expected convert sentinel, got", args)
		return 2
	}
	if slices.Contains(args, "--estimate") {
		fmt.Println(`{"title":"Test Book","author":"A. Author","detectedLanguage":"en",` +
			`"chapters":3,"sentences":42,"noteSentences":2,"words":500,"chars":2500,"warnings":[]}`)
		return 0
	}
	progressPath := argValue(args, "--progress-file")
	statsPath := argValue(args, "--stats")
	switch os.Getenv("HELPER_MODE") {
	case "ok":
		writeLines(progressPath,
			`{"ts":"t","phase":"translate","target":"ru","done":5,"total":10}`,
			`{"ts":"t","phase":"translate","target":"ru","done":10,"total":10}`,
			`{"ts":"t","phase":"assemble","done":1,"total":1}`,
			`{"ts":"t","phase":"done","ok":true}`,
		)
		writeLines(statsPath, `{"cost":0.5}`, `not json`, `{"cost":0.25}`)
		fmt.Println("converted fine")
		fmt.Fprintln(os.Stderr, "some warning")
		return 0
	case "fail":
		writeLines(progressPath,
			`{"ts":"t","phase":"translate","target":"ru","done":3,"total":10}`,
			`{"ts":"t","phase":"done","ok":false,"error":"boom"}`,
		)
		fmt.Fprintln(os.Stderr, "stderr-tail-marker")
		return 1
	case "sleep":
		writeLines(progressPath,
			`{"ts":"t","phase":"translate","target":"ru","done":1,"total":10}`)
		_ = os.WriteFile(os.Getenv("HELPER_PIDFILE"),
			[]byte(strconv.Itoa(os.Getpid())), 0o644)
		time.Sleep(60 * time.Second)
		return 0
	}
	fmt.Fprintln(os.Stderr, "helper: unknown HELPER_MODE")
	return 2
}

func argValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func writeLines(path string, lines ...string) {
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func newTestRunner(t *testing.T, mode string, extra ...string) *Runner {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	dir := t.TempDir()
	env := append([]string{"TBOOK_RUNNER_TEST_HELPER=1"}, extra...)
	if mode != "" {
		env = append(env, "HELPER_MODE="+mode)
	}
	return &Runner{
		Exe:      exe,
		WorkDir:  dir,
		CacheDir: filepath.Join(dir, "cache"),
		RunsDir:  filepath.Join(dir, "runs"),
		extraEnv: env,
	}
}

func baseSpec() ConvertSpec {
	return ConvertSpec{
		JobID: "job1", Attempt: "1",
		InputPath: "/in/book.epub", OutputPath: "/out/book.tbook",
		Source: "en", Targets: []string{"ru"},
		Provider: "openrouter", APIKey: "sk-or-test",
	}
}

func TestEstimate(t *testing.T) {
	r := newTestRunner(t, "")
	est, err := r.Estimate(context.Background(), "/in/book.epub")
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if est.Title != "Test Book" || est.Author != "A. Author" {
		t.Errorf("title/author = %q/%q", est.Title, est.Author)
	}
	if est.DetectedLanguage == nil || *est.DetectedLanguage != "en" {
		t.Errorf("detectedLanguage = %v", est.DetectedLanguage)
	}
	if est.Chapters != 3 || est.Sentences != 42 || est.NoteSentences != 2 ||
		est.Words != 500 || est.Chars != 2500 {
		t.Errorf("counts = %+v", est)
	}
	if est.Warnings == nil || len(est.Warnings) != 0 {
		t.Errorf("warnings = %#v", est.Warnings)
	}
}

func TestConvertHappyPath(t *testing.T) {
	r := newTestRunner(t, "ok")
	var mu sync.Mutex
	var events []ProgressEvent
	var costs []float64
	cost, err := r.Convert(context.Background(), baseSpec(),
		func(ev ProgressEvent) { mu.Lock(); events = append(events, ev); mu.Unlock() },
		func(c float64) { mu.Lock(); costs = append(costs, c); mu.Unlock() },
	)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4: %+v", len(events), events)
	}
	if events[0].Phase != "translate" || events[0].Done != 5 || events[0].Total != 10 ||
		events[0].Target != "ru" {
		t.Errorf("event 0 = %+v", events[0])
	}
	if events[1].Done != 10 || events[2].Phase != "assemble" {
		t.Errorf("events 1,2 = %+v %+v", events[1], events[2])
	}
	last := events[3]
	if last.Phase != "done" || last.OK == nil || !*last.OK {
		t.Errorf("done event = %+v", last)
	}
	if cost < 0.7499 || cost > 0.7501 {
		t.Errorf("cost = %v, want 0.75", cost)
	}
	if len(costs) == 0 || costs[len(costs)-1] != cost {
		t.Errorf("onCost calls = %v, want final %v", costs, cost)
	}
	log, err := os.ReadFile(r.LogPath("job1", "1"))
	if err != nil {
		t.Fatalf("log file: %v", err)
	}
	if !strings.Contains(string(log), "converted fine") ||
		!strings.Contains(string(log), "some warning") {
		t.Errorf("log misses teed output: %q", log)
	}
}

func TestConvertFailure(t *testing.T) {
	r := newTestRunner(t, "fail")
	_, err := r.Convert(context.Background(), baseSpec(), nil, nil)
	if err == nil {
		t.Fatal("Convert: want error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error misses done-event message: %v", err)
	}
	if !strings.Contains(err.Error(), "stderr-tail-marker") {
		t.Errorf("error misses stderr tail: %v", err)
	}
}

func TestConvertCancel(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	r := newTestRunner(t, "sleep", "HELPER_PIDFILE="+pidFile)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := r.Convert(ctx, baseSpec(), nil, nil)
		errCh <- err
	}()

	pid := waitForPidFile(t, pidFile)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Convert error = %v, want context.Canceled", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Convert did not return after cancel")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return // child gone
		}
		if time.Now().After(deadline) {
			t.Fatalf("child %d still alive after cancel", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func waitForPidFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				t.Fatalf("bad pid file: %q", b)
			}
			return pid
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("helper never wrote its pid file")
	return 0
}

func boolPtr(b bool) *bool { return &b }

func hasFlag(args []string, flag string) bool { return slices.Contains(args, flag) }

func TestBuildArgs(t *testing.T) {
	r := &Runner{
		CacheDir: "/cache", RunsDir: "/runs", LexiconsDir: "/lex",
		EmbalignPython: "/venv/bin/python", EmbalignScript: "/tools/embalign.py",
	}
	tests := []struct {
		name string
		spec ConvertSpec
		want []string // space-separated flag(+value) sequences that must appear
		ban  []string // flags that must not appear
	}{
		{
			name: "openrouter defaults",
			spec: baseSpec(),
			want: []string{"--provider openrouter", "--align-mode llm", "--lexicons /lex"},
			ban:  []string{"--max-retries", "--repair", "--no-repair", "--context", "--judge", "--force", "--embalign-python", "--limit-chapters"},
		},
		{
			name: "gonka gets retries and default-on repair context",
			spec: func() ConvertSpec {
				s := baseSpec()
				s.Provider = "gonka"
				s.RepairContext = 2
				return s
			}(),
			want: []string{"--context 2", "--max-retries 12"},
			ban:  []string{"--repair", "--no-repair"},
		},
		{
			name: "repair nil emits neither flag, context dropped when repair off",
			spec: func() ConvertSpec {
				s := baseSpec()
				s.RepairContext = 2 // openrouter: repair resolves off
				return s
			}(),
			ban: []string{"--repair", "--no-repair", "--context"},
		},
		{
			name: "context 1 never emitted",
			spec: func() ConvertSpec {
				s := baseSpec()
				s.Repair = boolPtr(true)
				s.RepairContext = 1
				return s
			}(),
			want: []string{"--repair"},
			ban:  []string{"--context"},
		},
		{
			name: "explicit no-repair beats gonka default, kills context",
			spec: func() ConvertSpec {
				s := baseSpec()
				s.Provider = "gonka"
				s.Repair = boolPtr(false)
				s.RepairContext = 2
				return s
			}(),
			want: []string{"--no-repair", "--max-retries 12"},
			ban:  []string{"--repair", "--context"},
		},
		{
			name: "aligner installed: embalign paths, no align-mode",
			spec: func() ConvertSpec {
				s := baseSpec()
				s.AlignerInstalled = true
				return s
			}(),
			want: []string{"--embalign-python /venv/bin/python", "--embalign-script /tools/embalign.py"},
			ban:  []string{"--align-mode"},
		},
		{
			name: "limit, judge, force",
			spec: func() ConvertSpec {
				s := baseSpec()
				s.LimitChapters = 3
				s.Judge = true
				s.Force = true
				return s
			}(),
			want: []string{"--limit-chapters 3", "--judge", "--force"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := r.buildArgs(tt.spec, "/runs/j-1.stats.jsonl", "/runs/j-1.progress.ndjson")
			if args[0] != "convert" {
				t.Fatalf("args[0] = %q, want convert sentinel", args[0])
			}
			joined := " " + strings.Join(args, " ") + " "
			for _, w := range tt.want {
				if !strings.Contains(joined, " "+w+" ") {
					t.Errorf("args missing %q:\n%v", w, args)
				}
			}
			for _, b := range tt.ban {
				if hasFlag(args, b) {
					t.Errorf("args must not contain %s:\n%v", b, args)
				}
			}
			// Always-on plumbing.
			for _, want := range []string{"-o", "-s", "-t", "--cache-dir", "--stats", "--progress-file"} {
				if !hasFlag(args, want) {
					t.Errorf("args missing %s:\n%v", want, args)
				}
			}
		})
	}
}

func TestBuildArgsNoLexicons(t *testing.T) {
	r := &Runner{CacheDir: "/cache", RunsDir: "/runs"}
	args := r.buildArgs(baseSpec(), "/s", "/p")
	if hasFlag(args, "--lexicons") {
		t.Errorf("unconfigured LexiconsDir must not emit --lexicons: %v", args)
	}
}

func TestBuildArgsMultiTarget(t *testing.T) {
	r := &Runner{CacheDir: "/cache", RunsDir: "/runs"}
	s := baseSpec()
	s.Targets = []string{"ru", "de"}
	args := r.buildArgs(s, "/s", "/p")
	if v := argValue(args, "-t"); v != "ru,de" {
		t.Errorf("-t = %q, want ru,de", v)
	}
}

// forbidden are variables the runner must never set: they would override the
// converter's provider-adaptive defaults or leak tuning into the child.
var forbidden = map[string]bool{
	"CONCURRENCY": true, "BATCH_SIZE": true, "MAX_RETRIES": true,
	"REPAIR": true, "NO_REPAIR": true, "PROVIDER": true,
	"SOURCE_LANG": true, "TARGET_LANG": true, "ALIGN_MODE": true,
}

func envKeys(env []string) map[string]string {
	m := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}
	return m
}

func TestBuildEnv(t *testing.T) {
	r := &Runner{}
	providerVars := []string{
		"OPENROUTER_API_KEY", "MODEL", "GONKA_API_KEY", "GONKA_MODEL",
		"CLAUDE_BIN", "OLLAMA_BASE_URL", "OLLAMA_MODEL", "OLLAMA_API_KEY",
		"LLAMACPP_BASE_URL", "LLAMACPP_MODEL", "LLAMACPP_API_KEY",
	}
	tests := []struct {
		name string
		spec ConvertSpec
		want map[string]string
	}{
		{
			name: "openrouter with model",
			spec: ConvertSpec{Provider: "openrouter", APIKey: "k1", Model: "m1"},
			want: map[string]string{"OPENROUTER_API_KEY": "k1", "MODEL": "m1"},
		},
		{
			name: "openrouter without model",
			spec: ConvertSpec{Provider: "openrouter", APIKey: "k1"},
			want: map[string]string{"OPENROUTER_API_KEY": "k1"},
		},
		{
			name: "gonka",
			spec: ConvertSpec{Provider: "gonka", APIKey: "k2", Model: "m2"},
			want: map[string]string{"GONKA_API_KEY": "k2", "GONKA_MODEL": "m2"},
		},
		{
			name: "claude with bin",
			spec: ConvertSpec{Provider: "claude", ClaudeBin: "/usr/local/bin/claude"},
			want: map[string]string{"CLAUDE_BIN": "/usr/local/bin/claude"},
		},
		{
			name: "claude without bin sets nothing",
			spec: ConvertSpec{Provider: "claude"},
			want: map[string]string{},
		},
		{
			name: "ollama",
			spec: ConvertSpec{Provider: "ollama", BaseURL: "http://x:11434/v1", Model: "m3"},
			want: map[string]string{"OLLAMA_BASE_URL": "http://x:11434/v1", "OLLAMA_MODEL": "m3"},
		},
		{
			name: "llamacpp",
			spec: ConvertSpec{Provider: "llamacpp", BaseURL: "http://x:8080/v1", APIKey: "k4"},
			want: map[string]string{"LLAMACPP_BASE_URL": "http://x:8080/v1", "LLAMACPP_API_KEY": "k4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envKeys(r.buildEnv(tt.spec))
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("%s = %q, want %q", k, got[k], v)
				}
			}
			// Exactly one provider block: no other provider's vars leak.
			for _, k := range providerVars {
				if _, expected := tt.want[k]; expected {
					continue
				}
				if v, ok := got[k]; ok {
					t.Errorf("unexpected %s=%q for provider %s", k, v, tt.spec.Provider)
				}
			}
			for k := range got {
				if forbidden[k] || strings.HasPrefix(k, "ANTHROPIC_") {
					t.Errorf("forbidden env var set: %s", k)
				}
			}
		})
	}
}

func TestBaseEnvExtraPathDirs(t *testing.T) {
	r := &Runner{ExtraPathDirs: []string{"/opt/tbook/bin"}}
	got := envKeys(r.baseEnv())
	if !strings.HasSuffix(got["PATH"], string(os.PathListSeparator)+"/opt/tbook/bin") {
		t.Errorf("PATH = %q, want ExtraPathDirs appended", got["PATH"])
	}
}

func TestPaths(t *testing.T) {
	r := &Runner{RunsDir: "/runs"}
	if got := r.ProgressPath("j", "2"); got != "/runs/j-2.progress.ndjson" {
		t.Errorf("ProgressPath = %q", got)
	}
	if got := r.StatsPath("j", "2"); got != "/runs/j-2.stats.jsonl" {
		t.Errorf("StatsPath = %q", got)
	}
	if got := r.LogPath("j", "2"); got != "/runs/j-2.log" {
		t.Errorf("LogPath = %q", got)
	}
}
