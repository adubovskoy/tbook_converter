package tbook

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// mustJSON renders meta compactly, normalizing the whitespace of preserved raw
// keys so two metas can be compared by value.
func mustJSON(t *testing.T, m *Meta) string {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	return string(b)
}

func TestAppendRunStampsTimestamps(t *testing.T) {
	m := &Meta{}
	m.AppendRun(RunRecord{At: "2026-07-19T08:02:55Z", Targets: []string{"ru"}})
	if m.CreatedAt != "2026-07-19T08:02:55Z" || m.UpdatedAt != "" {
		t.Fatalf("first run: createdAt=%q updatedAt=%q, want createdAt set and updatedAt empty",
			m.CreatedAt, m.UpdatedAt)
	}
	m.AppendRun(RunRecord{At: "2026-07-28T09:11:04Z", Targets: []string{"de"}})
	if m.CreatedAt != "2026-07-19T08:02:55Z" || m.UpdatedAt != "2026-07-28T09:11:04Z" {
		t.Fatalf("second run: createdAt=%q updatedAt=%q", m.CreatedAt, m.UpdatedAt)
	}
	if len(m.Runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(m.Runs))
	}
	if m.Runs[0].Targets[0] != "ru" || m.Runs[1].Targets[0] != "de" {
		t.Fatalf("runs not oldest-first: %+v", m.Runs)
	}
}

// A resumed conversion writes the file on every run; identical settings must
// collapse into one record (§3.4) while still moving updatedAt.
func TestAppendRunCollapsesIdenticalRecord(t *testing.T) {
	rec := RunRecord{
		At: "2026-07-28T09:00:00Z", Provider: "claude",
		Models:  map[string]string{"translate": "claude-sonnet-5"},
		Options: map[string]any{"alignMode": "hybrid"},
	}
	m := &Meta{}
	m.AppendRun(rec)
	again := rec
	again.At = "2026-07-28T10:30:00Z"
	m.AppendRun(again)
	if len(m.Runs) != 1 {
		t.Fatalf("runs = %d, want 1 (identical record collapsed)", len(m.Runs))
	}
	if m.Runs[0].At != "2026-07-28T09:00:00Z" {
		t.Fatalf("collapsed record kept at=%q, want the first timestamp", m.Runs[0].At)
	}
	if m.UpdatedAt != "2026-07-28T10:30:00Z" {
		t.Fatalf("updatedAt = %q, want the rewrite's timestamp", m.UpdatedAt)
	}
	// A changed setting is a different run and must be recorded.
	changed := rec
	changed.At = "2026-07-28T11:00:00Z"
	changed.Options = map[string]any{"alignMode": "llm"}
	m.AppendRun(changed)
	if len(m.Runs) != 2 {
		t.Fatalf("runs = %d, want 2 after a settings change", len(m.Runs))
	}
}

func TestAppendRunCapsHistory(t *testing.T) {
	m := &Meta{}
	for i := range MetaRunsMax + 5 {
		m.AppendRun(RunRecord{At: fmt.Sprintf("2026-07-28T09:%02d:00Z", i), Provider: fmt.Sprintf("p%d", i)})
	}
	if len(m.Runs) != MetaRunsMax {
		t.Fatalf("runs = %d, want the cap %d", len(m.Runs), MetaRunsMax)
	}
	if got := m.Runs[len(m.Runs)-1].Provider; got != fmt.Sprintf("p%d", MetaRunsMax+4) {
		t.Fatalf("newest record dropped: last provider = %q", got)
	}
	if m.Runs[0].Provider == "p0" {
		t.Fatalf("oldest record should have been dropped")
	}
}

// Unknown top-level keys are legal by construction (§3.4): a rewrite must carry
// another tool's bookkeeping through untouched, and registered keys must win
// over a stale carried-over copy.
func TestMetaJSONPreservesUnknownKeys(t *testing.T) {
	const in = `{"createdAt":"2026-07-19T08:02:55Z",` +
		`"runs":[{"at":"2026-07-19T08:02:55Z","provider":"openrouter","prompts":{"align":"v8"}}],` +
		`"x-mytool":{"anything":"here"},"dev.tbook.stats":[1,2,3]}`
	var m Meta
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.CreatedAt != "2026-07-19T08:02:55Z" || len(m.Runs) != 1 || m.Runs[0].Prompts["align"] != "v8" {
		t.Fatalf("registered keys not decoded: %+v", m)
	}
	if len(m.Extra) != 2 {
		t.Fatalf("Extra = %v, want the two unknown keys", m.Extra)
	}
	m.AppendRun(RunRecord{At: "2026-07-28T09:11:04Z", Provider: "claude"})

	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Meta
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if !reflect.DeepEqual(m, back) {
		t.Fatalf("round-trip changed meta:\n got %+v\nwant %+v", back, m)
	}
	if !strings.Contains(string(out), `"x-mytool":{"anything":"here"}`) {
		t.Fatalf("unknown key not preserved verbatim: %s", out)
	}
	if n := strings.Count(string(out), `"createdAt"`); n != 1 {
		t.Fatalf("createdAt appears %d times, want 1: %s", n, out)
	}
}

// meta must survive the archive: Write puts it in manifest.json and Read hands
// it back, so `convert book.tbook -t de` appends instead of starting over.
func TestWriteReadRoundTripsMeta(t *testing.T) {
	meta := &Meta{Extra: map[string]json.RawMessage{"x-mytool": json.RawMessage(`{"k":1}`)}}
	meta.AppendRun(RunRecord{
		At:       "2026-07-19T08:02:55Z",
		Producer: &Producer{Name: "tbook-converter", Version: "1.4.0"},
		Targets:  []string{"ru"},
		Provider: "openrouter",
		Models:   map[string]string{"translate": "google/gemini-2.5-flash"},
		Prompts:  map[string]string{"translate": "v5", "align": "v8"},
		Options:  map[string]any{"alignMode": "hybrid", "glossary": true},
	})
	b := &Book{
		Title: "Demo", Author: "A. Author", Source: "en", Targets: []string{"ru"},
		Meta: meta,
		Chapters: []Chapter{{Title: "Chapter 1", Paragraphs: [][]*Sentence{{{
			Src:   "A quiet evening.",
			Words: [][2]int{{0, 1}, {2, 7}, {8, 15}},
			Tr:    map[string]Translation{"ru": {Text: "Тихий вечер.", Align: []AlignChunk{}}},
		}}}}},
	}
	path := filepath.Join(t.TempDir(), "demo.tbook")
	if err := Write(path, b); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Meta == nil {
		t.Fatal("meta lost on round-trip")
	}
	// Compare as JSON: the manifest is written indented, so a preserved unknown
	// key comes back re-indented — equal data, different bytes.
	if a, b := mustJSON(t, got.Meta), mustJSON(t, meta); a != b {
		t.Fatalf("meta changed:\n got %s\nwant %s", a, b)
	}

	// A second run appends and leaves the first record intact.
	got.Meta.AppendRun(RunRecord{At: "2026-07-28T09:11:04Z", Targets: []string{"de"}, Provider: "claude"})
	got.Targets = []string{"ru", "de"}
	if err := Write(path, got); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	again, err := Read(path)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if len(again.Meta.Runs) != 2 || again.Meta.Runs[0].Provider != "openrouter" ||
		again.Meta.UpdatedAt != "2026-07-28T09:11:04Z" {
		t.Fatalf("append lost history: %+v", again.Meta)
	}
	if _, ok := again.Meta.Extra["x-mytool"]; !ok {
		t.Fatalf("unknown key dropped by the rewrite: %v", again.Meta.Extra)
	}
}

// A book with no provenance stays exactly as before: no "meta" key at all.
func TestWriteOmitsAbsentMeta(t *testing.T) {
	b := &Book{
		Title: "Demo", Source: "en", Targets: []string{"ru"},
		Chapters: []Chapter{{Title: "Chapter 1", Paragraphs: [][]*Sentence{{{
			Src: "Hi.", Words: [][2]int{{0, 2}},
			Tr: map[string]Translation{"ru": {Text: "Привет.", Align: []AlignChunk{}}},
		}}}}},
	}
	path := filepath.Join(t.TempDir(), "demo.tbook")
	if err := Write(path, b); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Meta != nil {
		t.Fatalf("meta = %+v, want nil for a book with no provenance", got.Meta)
	}
}
