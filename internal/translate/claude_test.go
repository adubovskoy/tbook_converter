package translate

import (
	"strings"
	"testing"
	"time"
)

func TestDetectUsageLimit(t *testing.T) {
	e := detectUsageLimit("Claude AI usage limit reached|1712345678")
	if e == nil {
		t.Fatal("epoch form not detected")
	}
	if want := time.Unix(1712345678, 0); !e.ResetAt.Equal(want) {
		t.Errorf("ResetAt = %v, want %v", e.ResetAt, want)
	}

	e = detectUsageLimit("You've hit your session limit · resets 11pm")
	if e == nil {
		t.Fatal("session-limit wording not detected")
	}
	if !e.ResetAt.IsZero() {
		t.Errorf("ResetAt should be zero without an epoch, got %v", e.ResetAt)
	}
	if !strings.Contains(e.Error(), "resumable") {
		t.Errorf("error text should mention resumability: %q", e.Error())
	}

	if detectUsageLimit(`{"1":"обычный перевод"}`) != nil {
		t.Error("plain JSON content misdetected as usage limit")
	}
}

func TestCLIPermanentDetection(t *testing.T) {
	msg := "There's an issue with the selected model (deepseek/x). It may not exist or you may not have access to it."
	if !cliPermanentRE.MatchString(msg) {
		t.Error("bad-model CLI message not detected as permanent")
	}
	if cliPermanentRE.MatchString("Стэн прошёл в гостиную") {
		t.Error("ordinary content misdetected as permanent CLI error")
	}
}

func TestClaudeEnvStripsCredentials(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "tok")
	t.Setenv("HARMLESS_VAR", "keep")
	kept := false
	for _, kv := range claudeEnv() {
		if strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") || strings.HasPrefix(kv, "ANTHROPIC_AUTH_TOKEN=") {
			t.Errorf("credential leaked into claude CLI env: %s", kv)
		}
		if kv == "HARMLESS_VAR=keep" {
			kept = true
		}
	}
	if !kept {
		t.Error("unrelated env vars must pass through")
	}
}
