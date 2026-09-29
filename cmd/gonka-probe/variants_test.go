package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountParsed(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int
	}{
		{"clean", `{"1":"Утро было ярким.","2":"Он молчал."}`, 2},
		{"think block first", "<think>weighing register…</think>\n{\"1\":\"Утро.\"}", 1},
		{"fenced prose around", "Here you go:\n```json\n{\"1\":\"Утро.\",\"2\":\"Ночь.\"}\n```", 2},
		{"empty value not counted", `{"1":"Утро.","2":"   "}`, 1},
		{"truncated", `{"1":"Утро.","2":"Он мол`, 0},
		{"not an object", "I cannot do that.", 0},
	}
	for _, c := range cases {
		if got := countParsed(c.content); got != c.want {
			t.Errorf("countParsed(%s) = %d, want %d", c.name, got, c.want)
		}
	}
}

// TestReasoningOff is the report's own rule: a variant counts as a win only
// when nothing reasoned AND the batch still came back usable.
func TestReasoningOff(t *testing.T) {
	cases := []struct {
		name string
		r    variantResult
		want bool
	}{
		{"silent and parsed", variantResult{OK: true, Parsed: 2}, true},
		{"think tag", variantResult{OK: true, Parsed: 2, ThinkTag: true}, false},
		{"reasoning field", variantResult{OK: true, Parsed: 2, ReasoningOut: true}, false},
		{"reasoning tokens", variantResult{OK: true, Parsed: 2, ReasoningTok: 180}, false},
		{"silent but unparseable", variantResult{OK: true, Parsed: 0}, false},
		{"failed request", variantResult{OK: false}, false},
	}
	for _, c := range cases {
		if got := c.r.ReasoningOff(); got != c.want {
			t.Errorf("%s: ReasoningOff = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestBodyVariants checks each variant adds exactly its own field to the
// production body and leaves the shared fields alone.
func TestBodyVariants(t *testing.T) {
	want := map[string]string{
		"control (no field)":                             "—",
		`thinking:{"type":"disabled"}`:                   `thinking={"type":"disabled"}`,
		"enable_thinking:false":                          "enable_thinking=false",
		`chat_template_kwargs:{"enable_thinking":false}`: `chat_template_kwargs={"enable_thinking":false}`,
		`reasoning:{"enabled":false}`:                    `reasoning={"enabled":false}`,
		`reasoning:{"effort":"none"}`:                    `reasoning={"effort":"none"}`,
		`reasoning_effort:"none"`:                        `reasoning_effort="none"`,
		"max_completion_tokens:40/sentence":              "max_completion_tokens=80",
	}
	if len(want) != len(bodyVariants) {
		t.Fatalf("%d variants, %d expectations", len(bodyVariants), len(want))
	}
	for _, v := range bodyVariants {
		body := map[string]any{
			"model": "m", "messages": nil, "temperature": 0.3,
			"response_format": nil, "user": "probe",
		}
		base := len(body)
		v.Apply(body, 2)
		got := variantFields(body, base)
		if got != want[v.Name] {
			t.Errorf("%s: fields = %q, want %q", v.Name, got, want[v.Name])
		}
		for _, k := range []string{"model", "messages", "temperature", "response_format", "user"} {
			if _, ok := body[k]; !ok {
				t.Errorf("%s: dropped base field %s", v.Name, k)
			}
		}
	}
}

func TestRejectsField(t *testing.T) {
	body := map[string]any{"model": "m", "user": "probe", "reasoning_effort": "none"}
	if !rejectsField(`unknown field "reasoning_effort"`, body, 2) {
		t.Error("a 400 naming the variant field must be reported as a rejection")
	}
	if rejectsField("service temporarily unavailable", body, 2) {
		t.Error("an unrelated error must not be read as a field rejection")
	}
}

// TestWriteDump: the smoke artifact must be in reading order even though the
// cell wrote it from concurrent batches.
func TestWriteDump(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glm.jsonl")
	rows := []dumpRow{
		{Idx: 2, Src: "c", Tr: "в"},
		{Idx: 0, Src: "a", Tr: "а"},
		{Idx: 1, Src: "b", Tr: "б"},
	}
	if err := writeDump(path, rows); err != nil {
		t.Fatalf("writeDump: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 3 {
		t.Fatalf("%d lines, want 3", len(lines))
	}
	var srcs []string
	for _, l := range lines {
		var r struct{ Src, Tr string }
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("line %q: %v", l, err)
		}
		srcs = append(srcs, r.Src)
	}
	if strings.Join(srcs, "") != "abc" {
		t.Errorf("order = %v, want [a b c]", srcs)
	}
	if strings.Contains(string(b), "\\u04") {
		t.Error("Cyrillic escaped: the smoke file must be readable as-is")
	}
}
