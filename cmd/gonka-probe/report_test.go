package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile is a helper: the summarize mode's whole contract is "read what a
// previous run left on disk", so every test here goes through a real file.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

const oneArtifact = `{
  "tool": "gonka-probe",
  "run": "ds-b8c8",
  "started_at": "2026-09-19T14:27:22Z",
  "batch_size": 8,
  "models": [
    {"id": "MiniMaxAI/MiniMax-M2.7", "listed": true, "probed": false},
    {"id": "moonshotai/Kimi-K2.6", "listed": false, "probed": true}
  ],
  "cells": [
    {"model": "deepseek-ai/DeepSeek-V4-Flash-0731", "concurrency": 8, "requests": 12,
     "success_pct": 91.7, "p50_ms": 3647, "p90_ms": 7133,
     "sentences_asked": 96, "sentences_got": 88, "sentences_per_s": 8.14,
     "mean_output_tokens": 286, "cyrillic_majority_pct": 100}
  ],
  "variants": [
    {"model": "MiniMaxAI/MiniMax-M2.7", "variant": "enable_thinking:false", "ok": true,
     "status": 200, "completion_tokens": 764, "latency_ms": 12597,
     "think_tag": false, "reasoning_field": true, "parsed_sentences": 2, "sentences": 2}
  ]
}`

// TestSummarizeArtifact: the cell inherits the artifact's batch size (older
// artifacts have no per-cell batch_size) and the served list is rendered from
// the models block.
func TestSummarizeArtifact(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "gonka-probe-x.json", oneArtifact)

	arts, dumps, _, err := collectInputs([]string{path})
	if err != nil {
		t.Fatalf("collectInputs: %v", err)
	}
	if len(arts) != 1 || len(dumps) != 0 {
		t.Fatalf("got %d artifacts, %d dumps; want 1/0", len(arts), len(dumps))
	}

	served := renderServedTable(arts)
	if !strings.Contains(served, "`MiniMaxAI/MiniMax-M2.7`") {
		t.Error("served table does not list the listed model")
	}
	if !strings.Contains(served, "`moonshotai/Kimi-K2.6`") {
		t.Error("served table does not carry the probed-but-unlisted model")
	}

	c := arts[0].art.Cells[0]
	if c.BatchSize != 0 {
		t.Fatalf("fixture cell should have no batch_size of its own, got %d", c.BatchSize)
	}
	c.BatchSize = arts[0].art.BatchSize
	row := renderCellTable([]cellSummary{c}, []string{arts[0].art.StartedAt})
	for _, want := range []string{"| 8 |", "91.7", "8.14", "88/96", "286", "2026-09-19T14:27:22Z"} {
		if !strings.Contains(row, want) {
			t.Errorf("cell row is missing %q:\n%s", want, row)
		}
	}

	vt := renderVariantTable(arts[0].art.Variants)
	if !strings.Contains(vt, "enable_thinking:false") || !strings.Contains(vt, "2/2") {
		t.Errorf("variant table wrong:\n%s", vt)
	}
}

// TestScanDump: each defect class is counted once and quoted verbatim.
func TestScanDump(t *testing.T) {
	dir := t.TempDir()
	lines := strings.Join([]string{
		`{"src":"The morning was bright and cold.","tr":"Утро было ярким и холодным."}`,
		`{"src":"He said nothing at all.","tr":"He said nothing at all."}`,
		`{"src":"She walked away slowly.","tr":"Sie ging langsam davon."}`,
		`{"src":"A very stout gentleman sat there.","tr":"Очень immer полный джентльмен сидел там."}`,
		`{"src":"He turned to the window.","tr":"Он повернулся к окну"}`,
		`{"src":"He waited.","tr":""}`,
	}, "\n") + "\n"
	path := writeFile(t, dir, "dump.jsonl", lines)

	s, err := scanDump(path)
	if err != nil {
		t.Fatalf("scanDump: %v", err)
	}
	if s.Lines != 6 {
		t.Errorf("lines = %d, want 6", s.Lines)
	}
	if s.Identical != 1 {
		t.Errorf("identical = %d, want 1", s.Identical)
	}
	if s.Empty != 1 {
		t.Errorf("empty = %d, want 1", s.Empty)
	}
	// The untranslated English line and the German line are both non-target.
	if s.NonTarget != 2 {
		t.Errorf("non-target = %d, want 2 (untranslated + German)", s.NonTarget)
	}
	if s.Leaks != 2 {
		t.Errorf("latin leaks = %d, want 2 (German line + «immer»)", s.Leaks)
	}
	if s.NoFinal != 1 {
		t.Errorf("no-final-punctuation = %d, want 1", s.NoFinal)
	}

	var quoted bool
	for _, e := range s.Examples {
		if e.Tr == "Очень immer полный джентльмен сидел там." && e.Kind == "latin leak" {
			quoted = true
			if len(e.Leaks) != 1 || e.Leaks[0] != "immer" {
				t.Errorf("leak words = %v, want [immer]", e.Leaks)
			}
		}
	}
	if !quoted {
		t.Error("the leaking line was not kept verbatim for quoting")
	}
}

func TestEndsSentence(t *testing.T) {
	cases := map[string]bool{
		"Он повернулся к окну.":    true,
		"Он повернулся к окну":     false,
		"«Кто там?»":               true,
		"Она сказала: ":            true,
		"Он замолчал\u2026":        true,
		"":                         false,
		"Наконец он поднял голову": false,
		// A reply in the wrong language is still a finished sentence: the
		// script check is what flags it, not this one.
		"\u6211\u4e5d\u70b9\u4e00\u523b\u4ece\u5bb6\u51fa\u53d1\u3002": true,
	}
	for in, want := range cases {
		if got := endsSentence(in); got != want {
			t.Errorf("endsSentence(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestCollectInputsDir: a directory argument expands to the dumps inside it,
// which is how the smoke folder is passed.
func TestCollectInputsDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "smoke")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, sub, "a.jsonl", `{"src":"He waited.","tr":"Он ждал."}`+"\n")
	writeFile(t, sub, "b.jsonl", `{"src":"She left.","tr":"Она ушла."}`+"\n")
	writeFile(t, sub, "notes.txt", "ignored")

	_, dumps, _, err := collectInputs([]string{sub})
	if err != nil {
		t.Fatalf("collectInputs: %v", err)
	}
	if len(dumps) != 2 {
		t.Fatalf("got %d dumps, want 2", len(dumps))
	}
	if out := renderDumpTable(dumps); !strings.Contains(out, "a.jsonl") || !strings.Contains(out, "b.jsonl") {
		t.Errorf("dump table missing a file:\n%s", out)
	}
}

func TestSummarizeRejectsNonArtifact(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "other.json", `{"tool":"something-else"}`)
	if _, _, _, err := collectInputs([]string{path}); err == nil {
		t.Error("a foreign JSON file was accepted as a probe artifact")
	}
	if err := summarizeFiles(nil); err == nil {
		t.Error("summarize with no inputs should fail")
	}
}

// TestScanRaw: a reply the strict parse rejects is kept verbatim and split by
// whether the tolerant path still recovers it — a reply nothing recovers costs
// its whole batch.
func TestScanRaw(t *testing.T) {
	dir := t.TempDir()
	rows := []string{
		`{"n":1,"chars":30,"content":"{\"1\":\"Утро было ярким.\"}"}`,
		`{"n":2,"chars":60,"content":"<think>weighing register</think>\n{\"1\":\"Он ждал.\"}"}`,
		"{\"n\":3,\"chars\":70,\"content\":\"```json\\n{\\\"1\\\":\\\"Она ушла.\\\"}\\n```\"}",
		`{"n":4,"chars":48,"content":"{\"1\":\"Он молчал.\"}{\"2\":\"Она ушла.\"}"}`,
		`{"n":5,"chars":40,"content":"Here you go: {\"1\":\"Он молчал.\"}"}`,
		`{"n":6,"chars":17,"content":"I cannot do that."}`,
	}
	path := writeFile(t, dir, "ds.raw.jsonl", strings.Join(rows, "\n")+"\n")

	s, err := scanRaw(path)
	if err != nil {
		t.Fatalf("scanRaw: %v", err)
	}
	if s.Replies != 6 {
		t.Errorf("replies = %d, want 6", s.Replies)
	}
	// #4 (two objects) and #6 (prose) are unusable; #5 (prose + one object) is
	// recovered by the tolerant path.
	if s.Bad != 2 || s.Lenient != 1 {
		t.Fatalf("bad/lenient = %d/%d, want 2/1", s.Bad, s.Lenient)
	}
	if len(s.Findings) != 3 {
		t.Fatalf("%d findings, want 3", len(s.Findings))
	}
	if !strings.Contains(s.Findings[0].Reason, "after top-level value") {
		t.Errorf("first rejection reason = %q, want a trailing-data complaint", s.Findings[0].Reason)
	}
	if !strings.Contains(s.Findings[0].Content, "Он молчал.") {
		t.Errorf("rejected reply not kept verbatim: %q", s.Findings[0].Content)
	}
	if s.Findings[0].Recovered {
		t.Error("a reply with a second object appended is not recoverable")
	}
	if !s.Findings[1].Recovered {
		t.Error("prose around a single intact object is recoverable")
	}
}

// TestCollectInputsRoutesRaw: .raw.jsonl must not be read as a {src,tr} dump.
func TestCollectInputsRoutesRaw(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.raw.jsonl", `{"n":1,"chars":9,"content":"{\"1\":\"Он ждал.\"}"}`+"\n")
	writeFile(t, dir, "b.jsonl", `{"src":"He waited.","tr":"Он ждал."}`+"\n")

	_, dumps, raws, err := collectInputs([]string{dir})
	if err != nil {
		t.Fatalf("collectInputs: %v", err)
	}
	if len(dumps) != 1 || len(raws) != 1 {
		t.Fatalf("got %d dumps and %d raw captures, want 1/1", len(dumps), len(raws))
	}
	if raws[0].Bad != 0 || raws[0].Lenient != 0 {
		t.Errorf("a clean reply was flagged: %+v", raws[0].Findings)
	}
	if out := renderRawTable(raws); !strings.Contains(out, "a.raw.jsonl") {
		t.Errorf("raw table missing the capture:\n%s", out)
	}
}

func TestStripReplyWrapper(t *testing.T) {
	cases := map[string]string{
		`{"1":"Он ждал."}`:                        `{"1":"Он ждал."}`,
		"<think>hm</think>\n{\"1\":\"Он ждал.\"}": `{"1":"Он ждал."}`,
		"```json\n{\"1\":\"Он ждал.\"}\n```":      `{"1":"Он ждал."}`,
		"<think>never closed":                     "",
	}
	for in, want := range cases {
		if got := stripReplyWrapper(in); got != want {
			t.Errorf("stripReplyWrapper(%q) = %q, want %q", in, got, want)
		}
	}
}
