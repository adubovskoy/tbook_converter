package keycheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// execRunner abstracts binary lookup and process execution so the claude
// probes are testable without a real CLI.
type execRunner interface {
	lookPath(file string) (string, error)
	run(ctx context.Context, timeout time.Duration, env []string, bin string, args ...string) (stdout, stderr string, err error)
}

type sysExec struct{}

func (sysExec) lookPath(file string) (string, error) { return exec.LookPath(file) }

func (sysExec) run(ctx context.Context, timeout time.Duration, env []string, bin string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env // nil = inherit
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// oldCLIRE matches an `auth status` failure that means the subcommand does
// not exist (older CLI), as opposed to a logged-out state.
var oldCLIRE = regexp.MustCompile(`(?i)unknown (command|option|argument)|unexpected argument|usage:`)

// CheckClaudeCLI verifies the claude CLI is installed and logged in.
// bin "" means "claude". GUI apps get a minimal PATH, so a bare name missing
// from PATH is also probed in the usual install dirs.
func CheckClaudeCLI(ctx context.Context, bin string) Result {
	return checkClaudeCLI(ctx, bin, sysExec{})
}

func checkClaudeCLI(ctx context.Context, bin string, ex execRunner) Result {
	if bin == "" {
		bin = "claude"
	}
	resolved, err := ex.lookPath(bin)
	if err != nil && filepath.Base(bin) == bin {
		for _, dir := range probeDirs() {
			if p, e := ex.lookPath(filepath.Join(dir, bin)); e == nil {
				resolved, err = p, nil
				break
			}
		}
	}
	if err != nil {
		return Result{Status: StatusNotInstalled,
			Detail: "claude CLI not found — install Claude Code from claude.com/claude-code, open it once and sign in"}
	}

	out, _, err := ex.run(ctx, 5*time.Second, nil, resolved, "--version")
	if err != nil {
		return Result{Status: StatusNotInstalled, ResolvedBin: resolved,
			Detail: fmt.Sprintf("%s is present but broken: `--version` failed (%v)", resolved, err)}
	}
	version := firstLine(out)

	out, errOut, runErr := ex.run(ctx, 10*time.Second, claudeProbeEnv(), resolved, "auth", "status")
	var st struct {
		LoggedIn         bool   `json:"loggedIn"`
		SubscriptionType string `json:"subscriptionType"`
		Email            string `json:"email"`
	}
	if obj := extractJSONObject(out); obj != "" && json.Unmarshal([]byte(obj), &st) == nil {
		if st.LoggedIn {
			return Result{Status: StatusOK, ResolvedBin: resolved, ClaudeVersion: version, Subscription: st.SubscriptionType,
				Detail: fmt.Sprintf("logged in as %s (%s)", st.Email, st.SubscriptionType)}
		}
		return Result{Status: StatusNotLoggedIn, ResolvedBin: resolved, ClaudeVersion: version,
			Detail: "claude CLI is installed but not logged in — run `claude` in a terminal and log in, then test again"}
	}
	if oldCLIRE.MatchString(out+"\n"+errOut) || runErr == nil {
		return Result{Status: StatusOK, ResolvedBin: resolved, ClaudeVersion: version,
			Detail:  fmt.Sprintf("claude CLI installed (%s)", version),
			Warning: "login state unknown — this CLI version has no `auth status`; conversions will fail if you are not logged in"}
	}
	return Result{Status: StatusNotLoggedIn, ResolvedBin: resolved, ClaudeVersion: version,
		Detail: "claude CLI reports you are not logged in — run `claude` in a terminal and log in, then test again"}
}

// probeDirs are install locations commonly missing from a GUI app's PATH.
func probeDirs() []string {
	dirs := []string{"/usr/local/bin", "/opt/homebrew/bin"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append([]string{filepath.Join(home, ".local", "bin")}, dirs...)
	}
	return dirs
}

// claudeProbeEnv strips API-key credentials so `auth status` reports the
// subscription (OAuth) auth path — the one conversions actually use. Mirrors
// claudeEnv in internal/translate.
func claudeProbeEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") || strings.HasPrefix(kv, "ANTHROPIC_AUTH_TOKEN=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return ""
}
