package migrate

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
)

// fixtureWithNote is the Annex B.1 fixture plus one footnote whose body is a
// copy of the first sentence, so the notes overlay takes part in the append.
func fixtureWithNote(t *testing.T) *tbook.Book {
	b := fixtureBook(t, true)
	s := *b.Chapters[0].Paragraphs[0][0]
	b.Notes = map[string]*tbook.Note{"n1": {Label: "1", Paragraphs: [][]*tbook.Sentence{{&s}}}}
	return b
}

// sentencePairs walks two structurally identical books in parallel.
func sentencePairs(a, b *tbook.Book, fn func(x, y *tbook.Sentence)) {
	for ci := range a.Chapters {
		for p, para := range a.Chapters[ci].Paragraphs {
			for k, s := range para {
				fn(s, b.Chapters[ci].Paragraphs[p][k])
			}
		}
		for ti, tb := range a.Chapters[ci].Tables {
			for r, row := range tb.Rows {
				for c, cell := range row {
					for k, s := range cell.Sentences {
						fn(s, b.Chapters[ci].Tables[ti].Rows[r][c].Sentences[k])
					}
				}
			}
		}
	}
	for id, n := range a.Notes {
		for p, para := range n.Paragraphs {
			for k, s := range para {
				fn(s, b.Notes[id].Paragraphs[p][k])
			}
		}
	}
}

func centralNames(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

// TestAppendLanguage is spec §2.4 on the fixture: the book is written without
// one language, which is then appended. Every pre-existing entry keeps its
// bytes and digest, the file validates, mimetype stays first and the manifest
// last and unique, meta gains one record, and the appended overlays are
// byte-identical to the ones a whole-book conversion writes.
func TestAppendLanguage(t *testing.T) {
	appendCheck(t, fixtureWithNote(t), fixtureWithNote(t))
}

// TestAppendSample appends the eighth language of the reference sample to a
// seven-language migration of it.
func TestAppendSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "android", "app", "src", "main", "assets", "sample.tbook")
	skipUnlessBook(t, path)
	full, err := tbook.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	base, err := tbook.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	appendCheck(t, full, base)
}

// appendCheck writes base without full's last language, appends that
// language, and checks the result. base and full are two copies of one book.
func appendCheck(t *testing.T, full, base *tbook.Book) {
	t.Helper()
	add := full.Targets[len(full.Targets)-1]
	base.Targets = base.Targets[:len(base.Targets)-1]
	if base.Meta == nil {
		base.Meta = &tbook.Meta{}
	}
	base.Meta.Extra = map[string]json.RawMessage{"x-tool": json.RawMessage(`{"k":1}`)}

	arcBase, _, err := Convert(base, Options{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "base.tbook")
	if err := arcBase.Write(in); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(in)
	if err != nil {
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
	// The new language as the pipeline would produce it: aligned to the
	// book's tbook-w2 words, which is what a whole-book migration yields too.
	arcFull, _, err := Convert(full, Options{})
	if err != nil {
		t.Fatal(err)
	}
	fullV2, err := ToBook(arcFull)
	if err != nil {
		t.Fatal(err)
	}
	sentencePairs(fullV2, rb, func(x, y *tbook.Sentence) {
		if tr, ok := x.Tr[add]; ok {
			y.Tr[add] = tr
		}
	})
	adds, _, err := Overlays(rb, arc, []string{add}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "appended.tbook")
	run := tbook.RunRecord{At: "2026-09-28T00:00:00Z", Targets: []string{add}}
	if err := v2.Append(in, out, adds, run); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	// Skeletons and existing overlays are untouched: the whole prefix up to
	// the old manifest's local header is byte-identical.
	cut := bytes.LastIndex(before, []byte("PK\x03\x04")) // manifest.json is the last local header
	if !bytes.Equal(before[:cut], after[:cut]) {
		t.Fatal("bytes before the old manifest changed")
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
	for name, d := range fa.Manifest.Digests {
		if fb.Manifest.Digests[name] != d {
			t.Errorf("digest of %s changed", name)
		}
		x, _ := fa.Entry(name)
		y, err := fb.Entry(name)
		if err != nil || !bytes.Equal(x, y) {
			t.Errorf("entry %s changed", name)
		}
	}

	rep, err := v2.Validate(out)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK() {
		t.Fatalf("appended file invalid: %v", rep.Violations)
	}

	entries := fb.Entries()
	if entries[0].Name != v2.MimetypeEntry || entries[len(entries)-1].Name != v2.ManifestEntry {
		t.Errorf("order: first %s, last %s", entries[0].Name, entries[len(entries)-1].Name)
	}
	n := 0
	for _, name := range centralNames(t, after) {
		if name == v2.ManifestEntry {
			n++
		}
	}
	if n != 1 || bytes.Count(after, []byte(v2.ManifestEntry)) != 2 { // one local header, one central record
		t.Errorf("manifest.json occurs %d times in the central directory, %d times in the file",
			n, bytes.Count(after, []byte(v2.ManifestEntry)))
	}

	m := fb.Manifest
	if !slices.Equal(m.Languages.Targets, full.Targets) {
		t.Errorf("targets %v, want %v", m.Languages.Targets, full.Targets)
	}
	for i, ref := range m.Spine {
		if ref.Gloss[add] == "" {
			t.Errorf("spine[%d] lacks %s", i, add)
		}
	}
	if m.Notes.Text != "" && m.Notes.Gloss[add] == "" {
		t.Errorf("notes lack %s", add)
	}
	if got := len(m.Meta.Runs); got != len(fa.Manifest.Meta.Runs)+1 ||
		!slices.Equal(m.Meta.Runs[got-1].Targets, []string{add}) {
		t.Errorf("meta runs: %+v", m.Meta.Runs)
	}
	if string(m.Meta.Extra["x-tool"]) != `{"k":1}` {
		t.Errorf("unknown meta key not carried over: %v", m.Meta.Extra)
	}

	// The appended overlays equal a whole-book conversion's.
	fullEntries, err := arcFull.Entries()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range fullEntries {
		name := e.Name
		if !fb.Has(name) || name == v2.ManifestEntry || fa.Has(name) {
			continue
		}
		got, _ := fb.Entry(name)
		if !bytes.Equal(got, e.Data) {
			t.Errorf("%s differs from a whole-book conversion", name)
		}
		checked++
	}
	want := len(m.Spine)
	if m.Notes.Text != "" {
		want++
	}
	if checked != want {
		t.Errorf("compared %d appended entries, want %d", checked, want)
	}

	// Appending an existing language is refused.
	if err := v2.Append(out, filepath.Join(dir, "again.tbook"), adds, run); err == nil {
		t.Error("appending an existing language succeeded")
	}
}

// roundTrip checks that reading a version-2 file into the version-1 model and
// writing it again reproduces every skeleton and overlay byte for byte — hence
// every tap.
func roundTrip(t *testing.T, arc *v2.Archive) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a.tbook")
	if err := arc.Write(path); err != nil {
		t.Fatal(err)
	}
	b, ver, err := ReadBook(path)
	if err != nil || ver != 2 {
		t.Fatalf("ReadBook: version %d, %v", ver, err)
	}
	arc2, _, err := Convert(b, Options{Source: arc.Manifest.Source})
	if err != nil {
		t.Fatal(err)
	}
	e1, err := arc.Entries()
	if err != nil {
		t.Fatal(err)
	}
	e2, err := arc2.Entries()
	if err != nil {
		t.Fatal(err)
	}
	second := map[string][]byte{}
	for _, e := range e2 {
		second[e.Name] = e.Data
	}
	diff := 0
	for _, e := range e1 {
		if e.Name == v2.ManifestEntry {
			continue
		}
		if !bytes.Equal(second[e.Name], e.Data) {
			if diff < 5 {
				t.Errorf("%s differs after the round trip", e.Name)
			}
			diff++
		}
	}
	if len(e1) != len(e2) || diff > 0 {
		t.Fatalf("%d of %d entries differ (%d vs %d entries)", diff, len(e1), len(e1), len(e2))
	}
}

func TestRoundTripFixture(t *testing.T) {
	arc, _, err := Convert(fixtureWithNote(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, arc)
}

// TestRoundTripSample: the migrated eight-language reference sample survives
// read → model → write unchanged.
func TestRoundTripSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "android", "app", "src", "main", "assets", "sample.tbook")
	skipUnlessBook(t, path)
	b, _, err := ReadBook(path)
	if err != nil {
		t.Fatal(err)
	}
	arc, _, err := Convert(b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, arc)
}

func TestRoundTripKimi(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "bench-quality", "full-kimi.tbook")
	skipUnlessBook(t, path)
	b, _, err := ReadBook(path)
	if err != nil {
		t.Fatal(err)
	}
	arc, _, err := Convert(b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, arc)
}
