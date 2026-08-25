package translate

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testScope() GlossaryScope {
	return GlossaryScope{Source: "en", Target: "ru", Title: "The Hobbit", Author: "Tolkien", Sentences: 4211}
}

// TestGlossaryFilePathPerPair: the language pair is in the name so adding a
// language to an existing .tbook (--out defaults to the input) neither
// clobbers nor inherits the previous pair's glossary.
func TestGlossaryFilePathPerPair(t *testing.T) {
	ru := GlossaryFilePath("/books/hobbit.tbook", testScope())
	if want := "/books/hobbit.tbook.glossary.en-ru.json"; ru != want {
		t.Fatalf("path = %q, want %q", ru, want)
	}
	es := testScope()
	es.Target = "es"
	if got := GlossaryFilePath("/books/hobbit.tbook", es); got == ru {
		t.Fatalf("en-es and en-ru share the path %q", got)
	}
}

// TestGlossaryFileRoundTrip: what WriteGlossaryFile writes, LoadGlossaryFile
// reads back — and hand edits (whitespace, blanked-out terms) are normalized
// the same way BuildGlossary normalizes the model's answer.
func TestGlossaryFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "hobbit.tbook.glossary.en-ru.json")
	in := []GlossEntry{{Src: "Ring", Tgt: "Кольцо"}, {Src: "Shire", Tgt: "Шир"}}
	if err := WriteGlossaryFile(path, testScope(), in); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := LoadGlossaryFile(path, testScope())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 2 || got[0] != in[0] || got[1] != in[1] {
		t.Fatalf("round trip = %+v, want %+v", got, in)
	}
	if GlossHash(got) != GlossHash(in) {
		t.Fatal("hash changed across a round trip: the cache namespace would move for no reason")
	}
}

// The gender annotation is user-editable, so it has to survive the sidecar
// round trip untouched — and the file must stay readable by a build that has no
// idea the fields exist.
func TestGlossaryFileRoundTripsGender(t *testing.T) {
	path := filepath.Join(t.TempDir(), "g.json")
	in := []GlossEntry{
		{Src: "Ortega", Tgt: "Ортега", Gender: "f", Kind: "person"},
		{Src: "Bancroft", Tgt: "Бэнкрофт", Kind: "person"}, // deliberately unstated
		{Src: "neurachem", Tgt: "нейрохим"},
	}
	if err := WriteGlossaryFile(path, testScope(), in); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := LoadGlossaryFile(path, testScope())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != len(in) {
		t.Fatalf("got %d entries, want %d", len(got), len(in))
	}
	for i := range in {
		if got[i] != in[i] {
			t.Errorf("entry %d: got %+v, want %+v", i, got[i], in[i])
		}
	}
	if GlossHash(got) != GlossHash(in) {
		t.Error("hash changed across the round trip: the cache namespace would move for no reason")
	}
	// An entry with no gender must not gain one, and the file must not carry an
	// empty field a user could mistake for a value.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(b), `"gender"`); n != 1 {
		t.Errorf("the file should mention \"gender\" exactly once, found %d:\n%s", n, b)
	}
}

func TestLoadGlossaryFileNormalizes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "g.json")
	if err := WriteGlossaryFile(path, testScope(), []GlossEntry{
		{Src: "  Ring  ", Tgt: "\tКольцо\n"},
		{Src: "Shire", Tgt: "   "}, // blanked out by the user: stop enforcing it
		{Src: "", Tgt: "Бильбо"},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := LoadGlossaryFile(path, testScope())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0] != (GlossEntry{Src: "Ring", Tgt: "Кольцо"}) {
		t.Fatalf("got %+v, want just the trimmed Ring entry", got)
	}
}

// TestLoadGlossaryFileEmpty: deleting every term is a valid way to say "no
// glossary" — it loads as empty and hashes to "", which puts the run back in
// the plain (non-glossary) cache namespace.
func TestLoadGlossaryFileEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "g.json")
	if err := WriteGlossaryFile(path, testScope(), nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := LoadGlossaryFile(path, testScope())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want no entries", got)
	}
	if h := GlossHash(got); h != "" {
		t.Fatalf("hash = %q, want the plain namespace", h)
	}
}

// TestLoadGlossaryFileScope is the reuse guard: a glossary is only valid for
// the book and language pair it was built from. Without this an en→ru
// glossary would be enforced verbatim on an en→es run, and a glossary built
// under --limit-chapters would carry into the full book.
func TestLoadGlossaryFileScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "g.json")
	if err := WriteGlossaryFile(path, testScope(), []GlossEntry{{Src: "Ring", Tgt: "Кольцо"}}); err != nil {
		t.Fatalf("write: %v", err)
	}

	for name, mutate := range map[string]func(*GlossaryScope){
		"target":    func(s *GlossaryScope) { s.Target = "es" },
		"source":    func(s *GlossaryScope) { s.Source = "de" },
		"title":     func(s *GlossaryScope) { s.Title = "The Silmarillion" },
		"author":    func(s *GlossaryScope) { s.Author = "Someone Else" },
		"sentences": func(s *GlossaryScope) { s.Sentences = 12 }, // e.g. --limit-chapters
	} {
		t.Run(name, func(t *testing.T) {
			scope := testScope()
			mutate(&scope)
			if _, err := LoadGlossaryFile(path, scope); !errors.Is(err, ErrGlossaryScope) {
				t.Fatalf("err = %v, want ErrGlossaryScope", err)
			}
		})
	}
}

// TestLoadGlossaryFileErrors: main distinguishes "no file yet" (build one,
// quietly) from "unusable file" (say so, then build) — so a missing file must
// stay fs.ErrNotExist and a broken one must not.
func TestLoadGlossaryFileErrors(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadGlossaryFile(filepath.Join(dir, "absent.json"), testScope())
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file: err = %v, want fs.ErrNotExist", err)
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadGlossaryFile(bad, testScope())
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("malformed file: err = %v, want a parse error", err)
	}
}
