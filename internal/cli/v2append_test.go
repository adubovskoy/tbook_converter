package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
	"github.com/dimando/reader/converter/internal/tbook/v2/migrate"
)

// fakeTranslation renders every source word as word<i><suffix>, aligned 1:1,
// so two languages never produce the same text.
func fakeTranslation(s *tbook.Sentence, stem string) tbook.Translation {
	var parts []string
	var chunks []tbook.AlignChunk
	pos := 0
	for i := range s.Words {
		w := fmt.Sprintf("%s%d", stem, i)
		if i > 0 {
			pos++
		}
		chunks = append(chunks, tbook.AlignChunk{T: [2]int{pos, pos + len([]rune(w))}, W: []int{i}})
		parts = append(parts, w)
		pos += len([]rune(w))
	}
	return tbook.Translation{Text: strings.Join(parts, " "), Align: chunks}
}

// v2Fixture writes a version-2 book with Russian and returns its sentences.
func v2Fixture(t *testing.T, path string) []*tbook.Sentence {
	t.Helper()
	chapters := []segment.ParsedChapter{{
		Title: "One",
		Paragraphs: []segment.ParsedParagraph{
			{Text: "Sylveste walked into Revelation Space. He did not look back."},
			{Text: "Revelation Space was cold and very quiet."},
		},
	}}
	outChapters, sentences := segment.BuildSentenceObjects(chapters, "en")
	for _, s := range sentences {
		s.Tr["ru"] = fakeTranslation(s, "слово")
	}
	book := &tbook.Book{Title: bookTitle, Author: bookAuthor, Source: "en",
		Targets: []string{"ru"}, Chapters: outChapters}
	arc, _, err := migrate.Convert(book, migrate.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := arc.Write(path); err != nil {
		t.Fatal(err)
	}
	return sentences
}

// TestAddLanguageToV2Book drives the CLI: a fully cached new language is
// appended to a version-2 input without touching any existing entry.
func TestAddLanguageToV2Book(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tbook")
	out := filepath.Join(dir, "out.tbook")
	cacheDir := filepath.Join(dir, "cache")
	for _, s := range v2Fixture(t, in) {
		if err := cache.Write(cacheDir, cache.Key(s.Src, "en", "de", model), fakeTranslation(s, "wort")); err != nil {
			t.Fatal(err)
		}
	}
	if err := runConvert(t, "", in, "-o", out, "-t", "de", "--no-glossary", "--no-lexcheck",
		"--cache-dir", cacheDir); err != nil {
		t.Fatalf("run: %v", err)
	}
	rep, err := v2.Validate(out)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK() {
		t.Fatalf("invalid: %v", rep.Violations)
	}
	fa, err := v2.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer fa.Close()
	fb, err := v2.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()
	if !slices.Equal(fb.Manifest.Languages.Targets, []string{"ru", "de"}) {
		t.Errorf("targets %v", fb.Manifest.Languages.Targets)
	}
	for name, d := range fa.Manifest.Digests {
		if fb.Manifest.Digests[name] != d {
			t.Errorf("%s changed", name)
		}
	}
	ov, ok, err := fb.Overlay(0, "de")
	if err != nil || !ok {
		t.Fatalf("no de overlay: %v", err)
	}
	if tr := ov.Paragraphs[0].S[0]; tr == nil || !strings.HasPrefix(tr.T, "wort0") || len(tr.A) == 0 {
		t.Errorf("de translation: %+v", tr)
	}
	runs := fb.Manifest.Meta.Runs
	if len(runs) != len(fa.Manifest.Meta.Runs)+1 || !slices.Equal(runs[len(runs)-1].Targets, []string{"de"}) {
		t.Errorf("runs: %+v", runs)
	}

	// In place (the default -o for a .tbook input) works too.
	for _, s := range v2Fixture(t, in) {
		if err := cache.Write(cacheDir, cache.Key(s.Src, "en", "fr", model), fakeTranslation(s, "mot")); err != nil {
			t.Fatal(err)
		}
	}
	if err := runConvert(t, "", in, "-t", "fr", "--no-glossary", "--no-lexcheck", "--cache-dir", cacheDir); err != nil {
		t.Fatalf("in-place run: %v", err)
	}
	if rep, err := v2.Validate(in); err != nil || !rep.OK() {
		t.Fatalf("in-place result invalid: %v %v", err, rep)
	}
}

func TestV2InputRefusals(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tbook")
	v2Fixture(t, in)
	cacheDir := filepath.Join(dir, "cache")

	err := runConvert(t, "", in, "-o", filepath.Join(dir, "v1.tbook"), "-t", "de", "--format", "v1",
		"--no-glossary", "--cache-dir", cacheDir)
	if err == nil || !strings.Contains(err.Error(), "formatVersion 2") || !strings.Contains(err.Error(), "--format v1") {
		t.Errorf("--format v1 on a v2 input: %v", err)
	}
	err = runConvert(t, "", in, "-o", filepath.Join(dir, "x.tbook"), "-t", "en",
		"--no-glossary", "--cache-dir", cacheDir)
	if err == nil || !strings.Contains(err.Error(), "source language") {
		t.Errorf("source language as a target: %v", err)
	}
}

// TestReplaceLanguageInV2Book drives the CLI: -t ru on a book that has ru
// re-translates it from the cache and replaces its overlays (spec §2.5).
func TestReplaceLanguageInV2Book(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tbook")
	out := filepath.Join(dir, "out.tbook")
	cacheDir := filepath.Join(dir, "cache")
	for _, s := range v2Fixture(t, in) {
		if err := cache.Write(cacheDir, cache.Key(s.Src, "en", "ru", model), fakeTranslation(s, "другое")); err != nil {
			t.Fatal(err)
		}
	}
	if err := runConvert(t, "", in, "-o", out, "-t", "ru", "--no-glossary", "--no-lexcheck",
		"--langcheck-gate=false", "--cache-dir", cacheDir); err != nil {
		t.Fatalf("run: %v", err)
	}
	rep, err := v2.Validate(out)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK() {
		t.Fatalf("invalid: %v", rep.Violations)
	}
	fa, err := v2.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer fa.Close()
	fb, err := v2.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()
	if !slices.Equal(fb.Manifest.Languages.Targets, []string{"ru"}) {
		t.Errorf("targets %v", fb.Manifest.Languages.Targets)
	}
	ruName := fa.Manifest.Spine[0].Gloss["ru"]
	for name, d := range fa.Manifest.Digests {
		if (fb.Manifest.Digests[name] == d) == (name == ruName) {
			t.Errorf("%s: digest %s → %s", name, d, fb.Manifest.Digests[name])
		}
	}
	ov, ok, err := fb.Overlay(0, "ru")
	if err != nil || !ok {
		t.Fatalf("no ru overlay: %v", err)
	}
	if tr := ov.Paragraphs[0].S[0]; tr == nil || !strings.HasPrefix(tr.T, "другое0") || len(tr.A) == 0 {
		t.Errorf("ru translation: %+v", tr)
	}
	runs := fb.Manifest.Meta.Runs
	if len(runs) != len(fa.Manifest.Meta.Runs)+1 || !slices.Equal(runs[len(runs)-1].Targets, []string{"ru"}) {
		t.Errorf("runs: %+v", runs)
	}
}

// TestAddLanguageToV2Sample appends the eighth language of the reference
// sample through the CLI, from a cache holding that language's version-1
// alignments (the word inventory an EPUB run caches), and compares every cell
// with a whole-book migration of the eight-language original.
func TestAddLanguageToV2Sample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "android", "app", "src", "main", "assets", "sample.tbook")
	if testing.Short() {
		t.Skip("short mode")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("reference book not available: %v", err)
	}
	full, err := tbook.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	add := full.Targets[len(full.Targets)-1]
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	prose, notes, cites := full.Sentences()
	for _, s := range append(append(prose, notes...), cites...) {
		if tr, ok := s.Tr[add]; ok {
			if err := cache.Write(cacheDir, cache.Key(s.Src, full.Source, add, model), tr); err != nil {
				t.Fatal(err)
			}
		}
	}
	want, _, err := migrate.Convert(full, migrate.Options{})
	if err != nil {
		t.Fatal(err)
	}

	base, err := tbook.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	base.Targets = base.Targets[:len(base.Targets)-1]
	arc, _, err := migrate.Convert(base, migrate.Options{})
	if err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(dir, "in.tbook")
	if err := arc.Write(in); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.tbook")
	if err := runConvert(t, "", in, "-o", out, "-t", add, "--no-glossary", "--no-lexcheck",
		"--langcheck-gate=false", "--cache-dir", cacheDir); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := v2.ReadArchive(out)
	if err != nil {
		t.Fatal(err)
	}
	cells, diff := 0, 0
	for i := range want.Skeletons {
		w, g := want.Overlays[i][add], got.Overlays[i][add]
		for p := range w.Paragraphs {
			ws, gs := w.Paragraphs[p].S, g.Paragraphs[p].S
			for k := range ws {
				cells++
				a, b := ws[k], gs[k]
				if (a == nil) != (b == nil) || a != nil && (a.T != b.T || linksKey(a) != linksKey(b)) {
					diff++
				}
			}
		}
	}
	t.Logf("%s: %d cells, %d differ", add, cells, diff)
	if cells == 0 || diff > 0 {
		t.Fatalf("%d of %d cells differ from a whole-book migration", diff, cells)
	}
}

func linksKey(tr *v2.Translation) string {
	return fmt.Sprint(tr.A, tr.X)
}

// TestReplaceLanguageInV2Sample replaces the last language of the migrated
// reference sample through the CLI from a cache of its version-1 alignments:
// every other entry keeps its digest and the replaced cells equal a whole-book
// migration.
func TestReplaceLanguageInV2Sample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "android", "app", "src", "main", "assets", "sample.tbook")
	if testing.Short() {
		t.Skip("short mode")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("reference book not available: %v", err)
	}
	full, err := tbook.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	lang := full.Targets[len(full.Targets)-1]
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	prose, notes, cites := full.Sentences()
	for _, s := range append(append(prose, notes...), cites...) {
		if tr, ok := s.Tr[lang]; ok {
			if err := cache.Write(cacheDir, cache.Key(s.Src, full.Source, lang, model), tr); err != nil {
				t.Fatal(err)
			}
		}
	}
	want, _, err := migrate.Convert(full, migrate.Options{})
	if err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(dir, "in.tbook")
	if err := want.Write(in); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.tbook")
	if err := runConvert(t, "", in, "-o", out, "-t", lang, "--no-glossary", "--no-lexcheck",
		"--langcheck-gate=false", "--cache-dir", cacheDir); err != nil {
		t.Fatalf("run: %v", err)
	}
	if rep, err := v2.Validate(out); err != nil || !rep.OK() {
		t.Fatalf("invalid: %v %v", err, rep)
	}
	fa, err := v2.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer fa.Close()
	fb, err := v2.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()
	replaced := map[string]bool{fa.Manifest.Notes.Gloss[lang]: true}
	for _, ref := range fa.Manifest.Spine {
		replaced[ref.Gloss[lang]] = true
	}
	for name, d := range fa.Manifest.Digests {
		if !replaced[name] && fb.Manifest.Digests[name] != d {
			t.Errorf("%s changed", name)
		}
	}
	got, err := v2.ReadArchive(out)
	if err != nil {
		t.Fatal(err)
	}
	cells, diff := 0, 0
	for i := range want.Skeletons {
		w, g := want.Overlays[i][lang], got.Overlays[i][lang]
		for p := range w.Paragraphs {
			ws, gs := w.Paragraphs[p].S, g.Paragraphs[p].S
			for k := range ws {
				cells++
				a, b := ws[k], gs[k]
				if (a == nil) != (b == nil) || a != nil && (a.T != b.T || linksKey(a) != linksKey(b)) {
					diff++
				}
			}
		}
	}
	t.Logf("%s: %d cells, %d differ", lang, cells, diff)
	if cells == 0 || diff > 0 {
		t.Fatalf("%d of %d cells differ from a whole-book migration", diff, cells)
	}
	if n := len(fb.Manifest.Meta.Runs); n != min(len(fa.Manifest.Meta.Runs)+1, tbook.MetaRunsMax) {
		t.Errorf("runs: %d after %d", n, len(fa.Manifest.Meta.Runs))
	}
}
