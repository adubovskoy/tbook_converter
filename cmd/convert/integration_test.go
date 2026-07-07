package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	outChapters, sentences := segment.BuildSentenceObjects(chapters, "en")
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
	outBook := &tbook.Book{Title: book.Title, Author: book.Author, Source: "en", Targets: []string{"ru"},
		Cover: book.Cover, Images: book.Images, Chapters: outChapters}
	if err := tbook.Write(out, outBook); err != nil {
		t.Fatalf("write: %v", err)
	}

	rep := tbook.Validate(outBook, []string{"ru"})
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
	texts := make([]string, 0, len(s.Words))
	for _, w := range s.Words {
		word := string(runes[w[0]:w[1]])
		chunks = append(chunks, align.Chunk{Tgt: word, En: align.EnField{word}})
		texts = append(texts, word)
	}
	return align.BuildTextAlign(chunks, s.Src, s.Words, strings.Join(texts, " "))
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

const habitosEPUB = "../../dokumen_pub_habitos_atomicos_edicion_espaola_cambios_pequeos_resultados.epub"

// TestHabitosStructural runs the full offline path on a real, formatting-heavy
// EPUB (Atomic Habits, Spanish): footnote markers must be stripped from prose
// and resolved to note bodies, figures and tables extracted, front/back matter
// skipped, and the assembled archive must validate with notes.json and images.
func TestHabitosStructural(t *testing.T) {
	if _, err := os.Stat(habitosEPUB); err != nil {
		t.Skipf("sample epub not present: %v", err)
	}
	book, err := epub.ParseOpts(habitosEPUB, epub.Options{SkipMatter: true})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Chapter structure: NCX flattened (parts AND chapters), matter skipped.
	if len(book.Chapters) < 28 {
		t.Fatalf("chapters = %d, want ≥28 (parts + 20 capítulos + apéndice…)", len(book.Chapters))
	}
	for _, ch := range book.Chapters {
		for _, bad := range []string{"Portada", "Sinopsis", "Notas", "Créditos"} {
			if ch.Title == bad {
				t.Errorf("matter chapter %q not skipped", bad)
			}
		}
	}

	if len(book.Images) != 20 {
		t.Errorf("images = %d, want 20", len(book.Images))
	}
	if len(book.Notes) != 286 {
		t.Errorf("notes = %d, want 286", len(book.Notes))
	}
	kinds := map[string]int{}
	for _, n := range book.Notes {
		kinds[n.Kind]++
		if len(n.Paragraphs) == 0 || n.Paragraphs[0].Text == "" {
			t.Errorf("note %s has empty body", n.ID)
		}
		if n.Label == "" {
			t.Errorf("note %s has empty label", n.ID)
		}
	}
	if kinds["citation"] == 0 || kinds["note"] == 0 {
		t.Errorf("note kinds = %v, want both citation and note", kinds)
	}

	outChapters, sentences := segment.BuildSentenceObjects(book.Chapters, "es")
	notes, noteSents, citeSents := segment.BuildNotes(book.Notes, "es")

	// No stray marker digits/asterisks left in prose (the old defect).
	leakRE := regexp.MustCompile(`\p{Ll}[.!?»)]?\d{1,3}( |$)`)
	leaks := 0
	for _, s := range sentences {
		if strings.Contains(s.Src, "*") {
			leaks++
			t.Logf("asterisk leak: %q", s.Src)
		} else if m := leakRE.FindString(s.Src); m != "" && !yearLike.MatchString(m) {
			// digits glued to a word — years etc. appear after spaces, so this
			// catches only glued footnote numbers
			leaks++
			if leaks < 6 {
				t.Logf("digit leak: %q (%q)", s.Src, m)
			}
		}
	}
	if leaks > 0 {
		t.Errorf("%d sentences still carry leaked markers", leaks)
	}

	figures, tables, markers := 0, 0, 0
	for _, c := range outChapters {
		figures += len(c.Figures)
		tables += len(c.Tables)
		for _, para := range c.Paragraphs {
			for _, s := range para {
				markers += len(s.Notes)
			}
		}
		for _, tb := range c.Tables {
			for _, row := range tb.Rows {
				for _, cell := range row {
					for _, s := range cell.Sentences {
						markers += len(s.Notes)
					}
				}
			}
		}
	}
	if figures != 20 {
		t.Errorf("figures = %d, want 20", figures)
	}
	if tables < 25 {
		t.Errorf("tables = %d, want ≥25", tables)
	}
	if markers != 286 {
		t.Errorf("note markers = %d, want 286 (one per distinct note)", markers)
	}

	// Assemble with empty translations (legal) and re-read the archive.
	out := filepath.Join(t.TempDir(), "habitos.tbook")
	outBook := &tbook.Book{Title: book.Title, Author: book.Author, Source: "es", Targets: []string{"ru"},
		Cover: book.Cover, Images: book.Images, Notes: notes, Chapters: outChapters}
	for _, s := range append(append(append([]*tbook.Sentence{}, sentences...), noteSents...), citeSents...) {
		s.Tr["ru"] = tbook.Translation{Text: "", Align: []tbook.AlignChunk{}}
	}
	if err := tbook.Write(out, outBook); err != nil {
		t.Fatalf("write: %v", err)
	}
	rep := tbook.Validate(outBook, []string{"ru"})
	if rep.OffsetErrors != 0 || rep.StructErrors != 0 {
		t.Fatalf("validation errors: %+v", rep)
	}

	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()
	man := readJSON[tbook.Manifest](t, zr, "manifest.json")
	if man.Notes == nil || *man.Notes != "notes.json" {
		t.Errorf("manifest.notes = %v, want notes.json", man.Notes)
	}
	nts := readJSON[map[string]*tbook.Note](t, zr, "notes.json")
	if len(nts) != 286 {
		t.Errorf("notes.json entries = %d, want 286", len(nts))
	}
	imgEntries := 0
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "images/") {
			imgEntries++
		}
	}
	if imgEntries != 20 {
		t.Errorf("archive image entries = %d, want 20", imgEntries)
	}
}

var yearLike = regexp.MustCompile(`^\d{4}`)
