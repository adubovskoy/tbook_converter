package migrate

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
)

// TestReplaceLanguage is spec §2.5 on the fixture with a footnote.
func TestReplaceLanguage(t *testing.T) {
	replaceCheck(t, fixtureWithNote(t), 0)
}

// TestReplaceLanguagePartial replaces a language in the first chapter only of
// a two-chapter book: the second chapter keeps its old overlay (§2.5 step 3).
func TestReplaceLanguagePartial(t *testing.T) {
	b := fixtureWithNote(t)
	second := b.Chapters[0]
	second.Title = "Second"
	b.Chapters = append(b.Chapters, second)
	replaceCheck(t, b, 1)
}

// TestReplaceSample replaces the last language of the reference sample.
func TestReplaceSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "android", "app", "src", "main", "assets", "sample.tbook")
	skipUnlessBook(t, path)
	b, err := tbook.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	replaceCheck(t, b, 0)
}

// rawEntry is an entry as stored: header fields and compressed bytes.
type rawEntry struct {
	method uint16
	crc    uint32
	data   []byte
}

func rawEntries(t *testing.T, path string) (map[string]rawEntry, []string) {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	files := slices.Clone(zr.File)
	slices.SortFunc(files, func(a, b *zip.File) int {
		x, _ := a.DataOffset()
		y, _ := b.DataOffset()
		return int(x - y)
	})
	out := map[string]rawEntry{}
	var order []string
	for _, f := range files {
		r, err := f.OpenRaw()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = rawEntry{f.Method, f.CRC32, data}
		order = append(order, f.Name)
	}
	return out, order
}

// replaceCheck writes book, then replaces its last language with the text and
// links of its first one, in the first len(chapters)-drop chapters.
func replaceCheck(t *testing.T, book *tbook.Book, drop int) {
	t.Helper()
	if len(book.Targets) < 2 {
		t.Fatalf("need two targets, have %v", book.Targets)
	}
	lang, donor := book.Targets[len(book.Targets)-1], book.Targets[0]
	if book.Meta == nil {
		book.Meta = &tbook.Meta{}
	}
	book.Meta.Extra = map[string]json.RawMessage{"x-tool": json.RawMessage(`{"k":1}`)}
	arcIn, _, err := Convert(book, Options{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tbook")
	if err := arcIn.Write(in); err != nil {
		t.Fatal(err)
	}

	arc, err := v2.ReadArchive(in)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := ToBook(arc)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ToBook(arc)
	if err != nil {
		t.Fatal(err)
	}
	sentencePairs(rb, want, func(x, y *tbook.Sentence) {
		if tr, ok := x.Tr[donor]; ok {
			x.Tr[lang], y.Tr[lang] = tr, tr
		} else {
			delete(x.Tr, lang)
			delete(y.Tr, lang)
		}
	})
	kept := len(rb.Chapters) - drop
	rb.Chapters = rb.Chapters[:kept]
	adds, _, err := Overlays(rb, arc, []string{lang}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.tbook")
	run := tbook.RunRecord{At: "2026-09-28T00:00:00Z", Targets: []string{lang}}
	if err := v2.Append(in, out, adds, run); err == nil {
		t.Fatal("Append replaced an existing language")
	}
	if err := v2.Replace(in, out, adds, run); err != nil {
		t.Fatal(err)
	}

	rep, err := v2.Validate(out)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK() {
		t.Fatalf("replaced file invalid: %v", rep.Violations)
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
	ma, mb := fa.Manifest, fb.Manifest

	// Uncovered chapters' overlays are carried like any other entry.
	replacedNames := map[string]bool{}
	for _, ref := range ma.Spine[:kept] {
		replacedNames[ref.Gloss[lang]] = true
	}
	replacedNames[ma.Notes.Gloss[lang]] = true

	// Every other entry: same stored bytes, same digest, same local order.
	ra, orderA := rawEntries(t, in)
	rbE, orderB := rawEntries(t, out)
	var carriedA, carriedB []string
	for _, name := range orderA {
		if name == v2.ManifestEntry || replacedNames[name] {
			continue
		}
		carriedA = append(carriedA, name)
		x, y := ra[name], rbE[name]
		if x.method != y.method || x.crc != y.crc || !bytes.Equal(x.data, y.data) {
			t.Errorf("%s is not byte-identical", name)
		}
		if ma.Digests[name] != mb.Digests[name] {
			t.Errorf("digest of %s changed", name)
		}
	}
	for _, name := range orderB {
		if _, ok := ra[name]; ok && !replacedNames[name] && name != v2.ManifestEntry {
			carriedB = append(carriedB, name)
		}
	}
	if !slices.Equal(carriedA, carriedB) {
		t.Errorf("local order changed:\n%v\n%v", carriedA, carriedB)
	}
	if orderB[0] != v2.MimetypeEntry || rbE[v2.MimetypeEntry].method != zip.Store ||
		orderB[len(orderB)-1] != v2.ManifestEntry {
		t.Errorf("order: first %s, last %s", orderB[0], orderB[len(orderB)-1])
	}
	after, _ := os.ReadFile(out)
	n := 0
	for _, name := range centralNames(t, after) {
		if name == v2.ManifestEntry {
			n++
		}
	}
	if n != 1 {
		t.Errorf("manifest.json listed %d times", n)
	}

	// The replaced language equals a whole-book conversion with the new text.
	arcWant, _, err := Convert(want, Options{})
	if err != nil {
		t.Fatal(err)
	}
	wantEntries, err := arcWant.Entries()
	if err != nil {
		t.Fatal(err)
	}
	wantBy := map[string][]byte{}
	for _, e := range wantEntries {
		wantBy[e.Name] = e.Data
	}
	for i, ref := range mb.Spine {
		name := ref.Gloss[lang]
		if i >= kept {
			if old := ma.Spine[i].Gloss[lang]; old == "" || name != old {
				t.Errorf("spine[%d]: %s overlay %q, want the old %q", i, lang, name, old)
			}
			continue
		}
		if name != ma.Spine[i].Gloss[lang] {
			t.Errorf("spine[%d]: overlay renamed %s → %s", i, ma.Spine[i].Gloss[lang], name)
		}
		got, err := fb.Entry(name)
		if err != nil {
			t.Fatal(err)
		}
		old, _ := fa.Entry(name)
		if bytes.Equal(got, old) {
			t.Errorf("%s was not replaced", name)
		}
		if !bytes.Equal(got, wantBy[name]) {
			t.Errorf("%s differs from a whole-book conversion", name)
		}
		if mb.Digests[name] != v2.EntryDigest(got) {
			t.Errorf("%s: digest not updated", name)
		}
	}
	if name := mb.Notes.Gloss[lang]; ma.Notes.Text != "" {
		got, _ := fb.Entry(name)
		if name == "" || !bytes.Equal(got, wantBy[name]) {
			t.Errorf("notes overlay %q differs from a whole-book conversion", name)
		}
	}
	if !slices.Equal(mb.Languages.Targets, ma.Languages.Targets) {
		t.Errorf("targets %v, want %v", mb.Languages.Targets, ma.Languages.Targets)
	}
	runs := mb.Meta.Runs
	if len(runs) != min(len(ma.Meta.Runs)+1, tbook.MetaRunsMax) || !slices.Equal(runs[len(runs)-1].Targets, []string{lang}) {
		t.Errorf("meta runs: %+v", runs)
	}
	if mb.Meta.CreatedAt != ma.Meta.CreatedAt || mb.Meta.UpdatedAt != run.At {
		t.Errorf("meta times: %q %q", mb.Meta.CreatedAt, mb.Meta.UpdatedAt)
	}
	if string(mb.Meta.Extra["x-tool"]) != `{"k":1}` {
		t.Errorf("unknown meta key not carried over: %v", mb.Meta.Extra)
	}

	// The same replacement again records a second run (no collapsing).
	again := filepath.Join(dir, "again.tbook")
	if err := v2.Replace(out, again, adds, run); err != nil {
		t.Fatal(err)
	}
	fc, err := v2.Open(again)
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Close()
	if len(fc.Manifest.Meta.Runs) != min(len(runs)+1, tbook.MetaRunsMax) {
		t.Errorf("second replacement: %d runs after %d", len(fc.Manifest.Meta.Runs), len(runs))
	}
}

// TestUpdateRefusesNonConformant: an entry after manifest.json makes both an
// append and a replacement refuse, naming that entry (spec §2.4, G10).
func TestUpdateRefusesNonConformant(t *testing.T) {
	book := fixtureWithNote(t)
	arc, _, err := Convert(book, Options{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "good.tbook")
	if err := arc.Write(good); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(good)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		if err := zw.Copy(f); err != nil {
			t.Fatal(err)
		}
	}
	zr.Close()
	w, _ := zw.Create("images/late.png")
	w.Write([]byte("x"))
	zw.Close()
	bad := filepath.Join(dir, "bad.tbook")
	if err := os.WriteFile(bad, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	lang := book.Targets[len(book.Targets)-1]
	rb, _ := ToBook(arc)
	adds, _, err := Overlays(rb, arc, []string{lang}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	run := tbook.RunRecord{At: "2026-09-28T00:00:00Z"}
	if err := v2.Replace(bad, filepath.Join(dir, "out.tbook"), adds, run); err == nil ||
		!strings.Contains(err.Error(), "images/late.png follows it") {
		t.Errorf("replace on a non-conformant input: %v", err)
	}
	adds[0].Lang = "xx"
	if err := v2.Append(bad, filepath.Join(dir, "out.tbook"), adds, run); err == nil ||
		!strings.Contains(err.Error(), "images/late.png follows it") {
		t.Errorf("append on a non-conformant input: %v", err)
	}
}

// TestNotesBindByPosition: a note paragraph id that differs from the notes
// skeleton is a warning, not a binding failure (spec §6.12, G6).
func TestNotesBindByPosition(t *testing.T) {
	book := fixtureWithNote(t)
	arc, _, err := Convert(book, Options{})
	if err != nil {
		t.Fatal(err)
	}
	lang := book.Targets[0]
	arc.NotesOverlays[lang].Notes["n1"][0].ID = "00000000"
	path := filepath.Join(t.TempDir(), "n.tbook")
	if err := arc.Write(path); err != nil {
		t.Fatal(err)
	}
	rep, err := v2.Validate(path)
	if err != nil {
		t.Fatal(err)
	}
	warned := false
	for _, w := range rep.Warnings {
		warned = warned || w.Rule == "notes.binding"
	}
	if !rep.OK() || !warned {
		t.Fatalf("violations %v, warnings %v", rep.Violations, rep.Warnings)
	}
	b, err := ToBook(arc)
	if err != nil {
		t.Fatal(err)
	}
	if tr := b.Notes["n1"].Paragraphs[0][0].Tr[lang]; tr.Text == "" {
		t.Error("note translation dropped")
	}
}
