// Package runner executes the converter CLI as a child process (the GUI
// binary re-runs itself via the "convert" sentinel argv), with a hand-built
// environment so no .env or inherited variable leaks into a run. Progress is
// tailed from the --progress-file NDJSON, cost is summed from --stats JSONL.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Runner executes conversions against one set of app-owned directories.
type Runner struct {
	Exe     string // the GUI binary itself; children run "<Exe> convert <args…>"
	WorkDir string // child CWD — app-owned, guaranteed-.env-free

	CacheDir string
	RunsDir  string

	LexiconsDir    string // "" = not configured
	EmbalignPython string
	EmbalignScript string

	ExtraPathDirs []string // appended to the child PATH

	extraEnv []string // test hook: appended to every child environment
}

// Estimate mirrors the CLI's --estimate JSON contract (a machine contract —
// see internal/cli estimateOut; deliberately not imported).
type Estimate struct {
	Title            string   `json:"title"`
	Author           string   `json:"author"`
	DetectedLanguage *string  `json:"detectedLanguage"`
	Chapters         int      `json:"chapters"`
	Sentences        int      `json:"sentences"`
	NoteSentences    int      `json:"noteSentences"`
	Words            int      `json:"words"`
	Chars            int      `json:"chars"`
	Warnings         []string `json:"warnings"`
}

// ProgressEvent is one NDJSON line of the converter's --progress-file.
type ProgressEvent struct {
	TS     string `json:"ts"`
	Phase  string `json:"phase"`
	Target string `json:"target"`
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	OK     *bool  `json:"ok"`
	Error  string `json:"error"`
}

// ConvertSpec is everything one conversion attempt needs.
type ConvertSpec struct {
	JobID   string
	Attempt string

	InputPath  string
	OutputPath string
	Source     string
	Targets    []string

	Provider  string
	Model     string
	BaseURL   string
	APIKey    string
	ClaudeBin string

	LimitChapters int
	Repair        *bool // nil = provider default
	RepairContext int   // 0|2 only; --context 1 is a hard CLI error
	Judge         bool
	Force         bool

	// AlignerInstalled false → --align-mode llm; true → the embalign paths
	// and no --align-mode (implicit mode degrades instead of failing).
	AlignerInstalled bool
}

func (r *Runner) ProgressPath(jobID, attempt string) string {
	return filepath.Join(r.RunsDir, jobID+"-"+attempt+".progress.ndjson")
}

func (r *Runner) StatsPath(jobID, attempt string) string {
	return filepath.Join(r.RunsDir, jobID+"-"+attempt+".stats.jsonl")
}

func (r *Runner) LogPath(jobID, attempt string) string {
	return filepath.Join(r.RunsDir, jobID+"-"+attempt+".log")
}

// baseEnv passes through only what the child genuinely needs — never the
// full parent environment.
func (r *Runner) baseEnv() []string {
	var env []string
	path := os.Getenv("PATH")
	if len(r.ExtraPathDirs) > 0 {
		extra := strings.Join(r.ExtraPathDirs, string(os.PathListSeparator))
		if path != "" {
			path = path + string(os.PathListSeparator) + extra
		} else {
			path = extra
		}
	}
	if path != "" {
		env = append(env, "PATH="+path)
	}
	keys := []string{"HOME", "LANG", "LC_ALL", "TMPDIR"}
	if runtime.GOOS == "windows" {
		keys = append(keys, "SystemRoot", "ComSpec", "PATHEXT",
			"USERPROFILE", "APPDATA", "LOCALAPPDATA", "TEMP", "TMP")
	}
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return append(env, r.extraEnv...)
}

// tailBuffer keeps the last max bytes written to it (stderr excerpts).
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.buf))
}

// Estimate runs `<Exe> convert <path> --estimate` (no API key needed) and
// parses the single JSON object from stdout.
func (r *Runner) Estimate(ctx context.Context, inputPath string) (*Estimate, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.Exe, "convert", inputPath, "--estimate")
	cmd.Dir = r.WorkDir
	cmd.Env = r.baseEnv()

	var stdout bytes.Buffer
	stderr := &tailBuffer{max: 4096}
	cmd.Stdout = &stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("converter --estimate: %w: %s", err, stderr.String())
	}

	var est Estimate
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &est); err != nil {
		return nil, fmt.Errorf("converter --estimate: bad JSON on stdout: %w", err)
	}
	return &est, nil
}

// repairOn reports whether the proofread pass will actually run: an explicit
// choice wins, otherwise the provider default (on for gonka only).
func (s ConvertSpec) repairOn() bool {
	if s.Repair != nil {
		return *s.Repair
	}
	return s.Provider == "gonka"
}

func (r *Runner) buildArgs(spec ConvertSpec, statsPath, progressPath string) []string {
	args := []string{
		"convert",
		spec.InputPath,
		"-o", spec.OutputPath,
		"-s", spec.Source,
		"-t", strings.Join(spec.Targets, ","),
		"--provider", spec.Provider,
		"--cache-dir", r.CacheDir,
		"--stats", statsPath,
		"--progress-file", progressPath,
	}
	if r.LexiconsDir != "" {
		args = append(args, "--lexicons", r.LexiconsDir)
	}
	if spec.AlignerInstalled {
		args = append(args,
			"--embalign-python", r.EmbalignPython,
			"--embalign-script", r.EmbalignScript)
	} else {
		args = append(args, "--align-mode", "llm")
	}
	if spec.LimitChapters > 0 {
		args = append(args, "--limit-chapters", strconv.Itoa(spec.LimitChapters))
	}
	if spec.Repair != nil {
		if *spec.Repair {
			args = append(args, "--repair")
		} else {
			args = append(args, "--no-repair")
		}
	}
	if spec.RepairContext == 2 && spec.repairOn() {
		args = append(args, "--context", "2")
	}
	if spec.Judge {
		args = append(args, "--judge")
	}
	if spec.Force {
		args = append(args, "--force")
	}
	// Gonka's network 429s transiently under load; the converter's default
	// retry budget dies on the first LLM call against a saturated gateway.
	if spec.Provider == "gonka" {
		args = append(args, "--max-retries", "12")
	}
	return args
}

// buildEnv returns the child environment: base plus exactly one provider
// block, only non-empty values (empty keeps the converter's own defaults).
func (r *Runner) buildEnv(spec ConvertSpec) []string {
	env := r.baseEnv()
	add := func(k, v string) {
		if v != "" {
			env = append(env, k+"="+v)
		}
	}
	switch spec.Provider {
	case "gonka":
		add("GONKA_API_KEY", spec.APIKey)
		add("GONKA_MODEL", spec.Model)
	case "claude":
		add("CLAUDE_BIN", spec.ClaudeBin)
	case "ollama":
		add("OLLAMA_BASE_URL", spec.BaseURL)
		add("OLLAMA_MODEL", spec.Model)
		add("OLLAMA_API_KEY", spec.APIKey)
	case "llamacpp":
		add("LLAMACPP_BASE_URL", spec.BaseURL)
		add("LLAMACPP_MODEL", spec.Model)
		add("LLAMACPP_API_KEY", spec.APIKey)
	default: // openrouter
		add("OPENROUTER_API_KEY", spec.APIKey)
		add("MODEL", spec.Model)
	}
	return env
}

// Convert runs one conversion attempt, invoking onProgress for every progress
// event and onCost (every 5s and once after exit) with the cost summed so far
// from the stats JSONL. Cancelling ctx kills the whole child process group.
// Success is exit 0; the done event is informational.
func (r *Runner) Convert(ctx context.Context, spec ConvertSpec, onProgress func(ProgressEvent), onCost func(float64)) (float64, error) {
	progressPath := r.ProgressPath(spec.JobID, spec.Attempt)
	statsPath := r.StatsPath(spec.JobID, spec.Attempt)
	logPath := r.LogPath(spec.JobID, spec.Attempt)

	if err := os.MkdirAll(r.RunsDir, 0o755); err != nil {
		return 0, fmt.Errorf("create runs dir: %w", err)
	}
	// The progress sink opens O_APPEND — a stale file would replay old events.
	_ = os.Remove(progressPath)

	logFile, err := os.Create(logPath)
	if err != nil {
		return 0, fmt.Errorf("create log: %w", err)
	}
	defer logFile.Close()

	cmd := exec.CommandContext(ctx, r.Exe, r.buildArgs(spec, statsPath, progressPath)...)
	cmd.Dir = r.WorkDir
	cmd.Env = r.buildEnv(spec)
	cleanup := setupCmd(cmd)
	defer cleanup()

	stderrTail := &tailBuffer{max: 8192}
	cmd.Stdout = logFile
	cmd.Stderr = io.MultiWriter(logFile, stderrTail)

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start converter: %w", err)
	}

	tailer := newProgressTailer(progressPath, onProgress)
	stopTailer := tailer.start()

	costDone := make(chan struct{})
	var costWG sync.WaitGroup
	if onCost != nil {
		costWG.Add(1)
		go func() {
			defer costWG.Done()
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-costDone:
					return
				case <-ticker.C:
					onCost(SumStatsCost(statsPath))
				}
			}
		}()
	}

	runErr := cmd.Wait()
	stopTailer() // final drain of the progress file
	close(costDone)
	costWG.Wait()

	cost := SumStatsCost(statsPath)
	if onCost != nil {
		onCost(cost)
	}

	if runErr != nil {
		if ctx.Err() != nil {
			return cost, ctx.Err()
		}
		msg := stderrTail.String()
		if ev := tailer.finalError(); ev != "" {
			msg = ev + "; " + msg
		}
		return cost, fmt.Errorf("converter failed: %w: %s", runErr, msg)
	}
	return cost, nil
}
