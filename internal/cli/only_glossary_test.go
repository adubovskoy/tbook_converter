package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
)

// --only-glossary is the interactive pre-pass: build the glossary, write it to
// a sidecar the user edits, exit before spending translation calls. These
// tests drive the real CLI (run() over os.Args) against a mock LLM, because
// the things that can go wrong here — reusing another language's glossary,
// demanding an API key for a pass that needs none, telling a supervisor the
// conversion succeeded — only show up at that level.

// bookTitle/bookAuthor are the fixture's metadata; they are part of the
// glossary sidecar's scope.
const (
	bookTitle  = "A Test Book"
	bookAuthor = "A. Tester"
)

// fixtureTbook writes a tiny valid .tbook to use as the converter's input.
// A .tbook input (the add-a-language flow) keeps the test self-contained —
// no EPUB fixture to skip on.
func fixtureTbook(t *testing.T, path string) {
	t.Helper()
	chapters := []segment.ParsedChapter{{
		Title: "One",
		Paragraphs: []segment.ParsedParagraph{
			{Text: "Sylveste walked into Revelation Space. He did not look back."},
			{Text: "Revelation Space was cold and very quiet."},
		},
	}}
	outChapters, sentences := segment.BuildSentenceObjects(chapters, "en")
	if len(sentences) == 0 {
		t.Fatal("fixture produced no sentences")
	}
	for _, s := range sentences {
		s.Tr["ru"] = tbook.Translation{Text: "", Align: []tbook.AlignChunk{}}
	}
	book := &tbook.Book{Title: bookTitle, Author: bookAuthor, Source: "en",
		Targets: []string{"ru"}, Chapters: outChapters}
	if err := tbook.Write(path, book); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

// mockLLM stands in for the provider: any chat call returns the same glossary.
// Returns the base URL and a counter of calls received.
func mockLLM(t *testing.T, terms ...translate.GlossEntry) (string, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	body, err := json.Marshal(map[string]any{"glossary": terms})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": string(body)}}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &calls
}

// runConvert invokes the CLI exactly as main() would. baseURL == "" means no
// provider is reachable and no API key is set: the run must then succeed only
// if it genuinely needs no LLM.
func runConvert(t *testing.T, baseURL string, args ...string) error {
	t.Helper()
	statsLog, progressLog = nil, nil // package-level sinks; don't leak between tests
	opened = nil
	openGlossary = func(path string) error { opened = append(opened, path); return nil }
	t.Cleanup(func() { openGlossary = openInDefaultApp })

	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("MODEL", model)
	t.Setenv("OPENROUTER_BASE_URL", baseURL)
	if baseURL != "" {
		t.Setenv("OPENROUTER_API_KEY", "test-key")
	}

	return Run(args)
}

// opened records the sidecar paths --only-glossary handed to the editor.
var opened []string

// readSidecar decodes a glossary sidecar as the user would see it on disk.
func readSidecar(t *testing.T, path string) (translate.GlossaryScope, []translate.GlossEntry) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	var f struct {
		translate.GlossaryScope
		Terms []translate.GlossEntry `json:"terms"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("decode sidecar %s: %v", path, err)
	}
	return f.GlossaryScope, f.Terms
}

// TestOnlyGlossaryWritesAndStops: one LLM call, a sidecar scoped to this book
// and pair, the editor invoked — and crucially no .tbook, because the whole
// point is to stop before paying for the translation.
func TestOnlyGlossaryWritesAndStops(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tbook")
	out := filepath.Join(dir, "out.tbook")
	fixtureTbook(t, in)

	base, calls := mockLLM(t, translate.GlossEntry{Src: "Revelation Space", Tgt: "Пространство Откровения"})
	err := runConvert(t, base, in, "-o", out, "-t", "es", "--cache-dir", filepath.Join(dir, "cache"), "--only-glossary")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("LLM calls = %d, want exactly 1 (the glossary)", calls.Load())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("%s was written; --only-glossary must not translate or assemble", out)
	}

	path := out + ".glossary.en-es.json"
	scope, terms := readSidecar(t, path)
	want := translate.GlossaryScope{Source: "en", Target: "es", Title: bookTitle, Author: bookAuthor, Sentences: scope.Sentences}
	if scope != want || scope.Sentences == 0 {
		t.Errorf("scope = %+v, want %+v with a non-zero sentence count", scope, want)
	}
	if len(terms) != 1 || terms[0].Tgt != "Пространство Откровения" {
		t.Errorf("terms = %+v, want the model's single entry", terms)
	}
	if len(opened) != 1 || opened[0] != path {
		t.Errorf("opened = %v, want [%s]", opened, path)
	}
}

// TestOnlyGlossaryReusesSidecarOffline is the regression test for the flag's
// core promise: hand edits survive, and re-opening the file to tweak it again
// costs neither an LLM call nor an API key.
func TestOnlyGlossaryReusesSidecarOffline(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tbook")
	out := filepath.Join(dir, "out.tbook")
	cacheDir := filepath.Join(dir, "cache")
	fixtureTbook(t, in)

	base, calls := mockLLM(t, translate.GlossEntry{Src: "Sylveste", Tgt: "Sylveste-machine"})
	if err := runConvert(t, base, in, "-o", out, "-t", "es", "--cache-dir", cacheDir, "--only-glossary"); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// The user rewrites a term the model got wrong.
	path := out + ".glossary.en-es.json"
	scope, _ := readSidecar(t, path)
	if err := translate.WriteGlossaryFile(path, scope,
		[]translate.GlossEntry{{Src: "Sylveste", Tgt: "Silveste-humano"}}); err != nil {
		t.Fatalf("edit sidecar: %v", err)
	}

	// Second run with no provider and no key: it must load the edit as-is.
	before := calls.Load()
	if err := runConvert(t, "", in, "-o", out, "-t", "es", "--cache-dir", cacheDir, "--only-glossary"); err != nil {
		t.Fatalf("second run needed an LLM it should not have: %v", err)
	}
	if calls.Load() != before {
		t.Errorf("LLM calls = %d, want no new call", calls.Load()-before)
	}
	_, terms := readSidecar(t, path)
	if len(terms) != 1 || terms[0].Tgt != "Silveste-humano" {
		t.Errorf("terms = %+v, want the user's edit preserved", terms)
	}
}

// TestOnlyGlossaryForceRebuilds: --force is the documented way back to the
// model's glossary after edits go wrong.
func TestOnlyGlossaryForceRebuilds(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tbook")
	out := filepath.Join(dir, "out.tbook")
	fixtureTbook(t, in)
	path := out + ".glossary.en-es.json"

	base, _ := mockLLM(t, translate.GlossEntry{Src: "Sylveste", Tgt: "Силвест"})
	args := []string{in, "-o", out, "-t", "es", "--cache-dir", filepath.Join(dir, "cache"), "--only-glossary"}
	if err := runConvert(t, base, args...); err != nil {
		t.Fatalf("first run: %v", err)
	}
	scope, _ := readSidecar(t, path)
	if err := translate.WriteGlossaryFile(path, scope,
		[]translate.GlossEntry{{Src: "Sylveste", Tgt: "oops"}}); err != nil {
		t.Fatalf("edit sidecar: %v", err)
	}

	if err := runConvert(t, base, append(args, "--force")...); err != nil {
		t.Fatalf("force run: %v", err)
	}
	if _, terms := readSidecar(t, path); len(terms) != 1 || terms[0].Tgt != "Силвест" {
		t.Errorf("terms = %+v, want the model's glossary restored", terms)
	}
}

// TestOnlyGlossaryPerLanguagePair: adding a language must not inherit or
// clobber the glossary of the language already in the book. --out defaults to
// the input path here, which is exactly how the add-a-language flow is run.
func TestOnlyGlossaryPerLanguagePair(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "book.tbook")
	cacheDir := filepath.Join(dir, "cache")
	fixtureTbook(t, in)

	esBase, _ := mockLLM(t, translate.GlossEntry{Src: "Sylveste", Tgt: "Silveste"})
	if err := runConvert(t, esBase, in, "-t", "es", "--cache-dir", cacheDir, "--only-glossary"); err != nil {
		t.Fatalf("es run: %v", err)
	}
	frBase, frCalls := mockLLM(t, translate.GlossEntry{Src: "Sylveste", Tgt: "Sylvestre"})
	if err := runConvert(t, frBase, in, "-t", "fr", "--cache-dir", cacheDir, "--only-glossary"); err != nil {
		t.Fatalf("fr run: %v", err)
	}
	if frCalls.Load() != 1 {
		t.Fatalf("fr LLM calls = %d, want 1: the es sidecar was reused for fr", frCalls.Load())
	}

	if _, terms := readSidecar(t, in+".glossary.en-es.json"); terms[0].Tgt != "Silveste" {
		t.Errorf("es terms = %+v, want the Spanish glossary intact", terms)
	}
	if _, terms := readSidecar(t, in+".glossary.en-fr.json"); terms[0].Tgt != "Sylvestre" {
		t.Errorf("fr terms = %+v, want the French glossary", terms)
	}
}

// TestOnlyGlossaryRejectsForeignSidecar: a sidecar whose scope doesn't match
// the run (here: fewer sentences, i.e. built under --limit-chapters) is
// rebuilt rather than enforced on a book it was never built for.
func TestOnlyGlossaryRejectsForeignSidecar(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tbook")
	out := filepath.Join(dir, "out.tbook")
	fixtureTbook(t, in)

	path := out + ".glossary.en-es.json"
	stale := translate.GlossaryScope{Source: "en", Target: "es", Title: bookTitle, Author: bookAuthor, Sentences: 1}
	if err := translate.WriteGlossaryFile(path, stale,
		[]translate.GlossEntry{{Src: "Sylveste", Tgt: "from-a-partial-run"}}); err != nil {
		t.Fatalf("seed stale sidecar: %v", err)
	}

	base, calls := mockLLM(t, translate.GlossEntry{Src: "Sylveste", Tgt: "Silveste"})
	if err := runConvert(t, base, in, "-o", out, "-t", "es",
		"--cache-dir", filepath.Join(dir, "cache"), "--only-glossary"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("LLM calls = %d, want 1: the stale sidecar was reused", calls.Load())
	}
	scope, terms := readSidecar(t, path)
	if scope.Sentences == 1 || len(terms) != 1 || terms[0].Tgt != "Silveste" {
		t.Errorf("sidecar = %+v %+v, want it rebuilt for this run", scope, terms)
	}
}

// TestOnlyGlossaryWritesNoProgressFile: --progress-file is a machine contract.
// A run that produces no .tbook must not leave a "done" event a supervisor
// (converter_web) would read as a finished conversion.
func TestOnlyGlossaryWritesNoProgressFile(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tbook")
	out := filepath.Join(dir, "out.tbook")
	progressPath := filepath.Join(dir, "progress.ndjson")
	fixtureTbook(t, in)

	base, _ := mockLLM(t, translate.GlossEntry{Src: "Sylveste", Tgt: "Silveste"})
	if err := runConvert(t, base, in, "-o", out, "-t", "es", "--cache-dir", filepath.Join(dir, "cache"),
		"--progress-file", progressPath, "--only-glossary"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if b, err := os.ReadFile(progressPath); err == nil {
		t.Errorf("progress file written with %q; --only-glossary produces no conversion to report",
			strings.TrimSpace(string(b)))
	}
}
