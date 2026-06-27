package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dimando/reader/converter/internal/align"
	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/epub"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
)

const sampleEPUB = "../../../robert_shekli-alien_harvest.epub"
const model = "test-model"

// TestEndToEndOffline exercises the whole non-network path — parse → segment →
// (seed cache with identity "translations") → fill → assemble → validate →
// re-read the ZIP — so the pipeline is verified without an API key.
func TestEndToEndOffline(t *testing.T) {
	if _, err := os.Stat(sampleEPUB); err != nil {
		t.Skipf("sample epub not present: %v", err)
	}
	book, err := epub.Parse(sampleEPUB)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(book.Chapters) == 0 {
		t.Fatal("no chapters parsed")
	}
	chapters := book.Chapters[:2] // keep it quick
	outChapters, sentences := segment.BuildSentenceObjects(chapters)
	if len(sentences) == 0 {
		t.Fatal("no sentences")
	}

	cacheDir := t.TempDir()
	for _, s := range sentences {
		cache.Write(cacheDir, cache.Key(s.Src, "en", "ru", model), identityTranslation(s))
	}

	if pending := translate.CountPending(sentences, []string{"ru"}, cacheDir, "en", model, false); pending != 0 {
		t.Fatalf("expected 0 pending after seeding, got %d", pending)
	}
	_, missing := translate.FillFromCache(sentences, []string{"ru"}, cacheDir, "en", model)
	if missing != 0 {
		t.Fatalf("expected 0 missing, got %d", missing)
	}

	out := filepath.Join(t.TempDir(), "out.tbook")
	if err := tbook.Write(out, book.Title, book.Author, "en", []string{"ru"}, book.Cover, outChapters); err != nil {
		t.Fatalf("write: %v", err)
	}

	rep := tbook.Validate(outChapters, []string{"ru"})
	if !rep.OK() {
		t.Fatalf("validation not OK: %+v", rep)
	}

	// Re-read the produced archive and sanity-check the manifest + a chapter.
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()
	man := readJSON[tbook.Manifest](t, zr, "manifest.json")
	if man.FormatVersion != 1 {
		t.Errorf("formatVersion = %d, want 1", man.FormatVersion)
	}
	if len(man.Chapters) != len(outChapters) {
		t.Errorf("manifest chapters = %d, want %d", len(man.Chapters), len(outChapters))
	}
	if man.Cover == nil {
		t.Errorf("expected cover in manifest")
	}
	var chap tbook.Chapter
	payload := readJSONRaw(t, zr, man.Chapters[0].File)
	if len(payload) == 0 {
		t.Fatal("empty first chapter file")
	}
	_ = chap
}

// identityTranslation builds a trivial but well-formed translation where every
// target "word" echoes its source word — enough to produce valid align spans.
func identityTranslation(s *tbook.Sentence) tbook.Translation {
	runes := []rune(s.Src)
	chunks := make([]align.Chunk, 0, len(s.Words))
	for _, w := range s.Words {
		word := string(runes[w[0]:w[1]])
		chunks = append(chunks, align.Chunk{Tgt: word, En: align.EnField{word}})
	}
	return align.BuildTextAlign(chunks, s.Src, s.Words)
}

func readJSON[T any](t *testing.T, zr *zip.ReadCloser, name string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(readJSONRaw(t, zr, name), &v); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return v
}

func readJSONRaw(t *testing.T, zr *zip.ReadCloser, name string) []byte {
	t.Helper()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer rc.Close()
			b, _ := io.ReadAll(rc)
			return b
		}
	}
	t.Fatalf("entry not found: %s", name)
	return nil
}
