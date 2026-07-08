package fb2

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/charmap"

	"github.com/dimando/reader/converter/internal/tbook"
)

// pngStub is a tiny valid-base64 payload standing in for cover bytes.
var pngStub = []byte{0x89, 'P', 'N', 'G', 0}

func sampleFB2(t *testing.T) string {
	t.Helper()
	return `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
 <description>
  <title-info>
   <book-title>Атомные привычки</book-title>
   <author><first-name>James</first-name><last-name>Clear</last-name></author>
   <coverpage><image l:href="#cover.png"/></coverpage>
  </title-info>
 </description>
 <body>
  <section>
   <title><p>Глава 1</p></title>
   <epigraph><p>skipped epigraph</p></epigraph>
   <p>He was <emphasis>very</emphasis> tired.</p>
   <empty-line/>
   <subtitle>A <strong>bold</strong> subtitle</subtitle>
   <section>
    <title><p>Nested part</p></title>
    <p>Nested body text.</p>
   </section>
  </section>
  <section>
   <p>Second chapter, no title.</p>
  </section>
 </body>
 <body name="notes">
  <section><p>note body must be skipped</p></section>
 </body>
 <binary id="cover.png" content-type="image/png">` +
		base64.StdEncoding.EncodeToString(pngStub) + `</binary>
</FictionBook>`
}

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseFB2(t *testing.T) {
	book, err := Parse(writeFile(t, "book.fb2", []byte(sampleFB2(t))))
	if err != nil {
		t.Fatal(err)
	}
	if book.Title != "Атомные привычки" {
		t.Errorf("title = %q", book.Title)
	}
	if book.Author != "James Clear" {
		t.Errorf("author = %q", book.Author)
	}
	if !bytes.Equal(book.Cover, pngStub) {
		t.Errorf("cover = %v, want %v", book.Cover, pngStub)
	}
	if len(book.Chapters) != 2 {
		t.Fatalf("chapters = %d, want 2", len(book.Chapters))
	}

	ch := book.Chapters[0]
	if ch.Title != "Глава 1" {
		t.Errorf("ch0 title = %q", ch.Title)
	}
	var texts []string
	var roles []string
	for _, p := range ch.Paragraphs {
		texts = append(texts, p.Text)
		roles = append(roles, p.Role)
	}
	wantTexts := []string{"He was very tired.", "", "A bold subtitle", "Nested part", "Nested body text."}
	wantRoles := []string{tbook.RoleBody, tbook.RoleSceneBreak, tbook.RoleSubtitle, tbook.RoleHeading, tbook.RoleBody}
	for i := range wantTexts {
		if i >= len(texts) || texts[i] != wantTexts[i] || roles[i] != wantRoles[i] {
			t.Fatalf("paragraphs = %v %v, want %v %v", texts, roles, wantTexts, wantRoles)
		}
	}

	// Emphasis spans survive cleaning with correct rune offsets.
	body := ch.Paragraphs[0]
	if len(body.Spans) != 1 || body.Spans[0].K != tbook.SpanItalic {
		t.Fatalf("body spans = %+v", body.Spans)
	}
	if got := body.Text[body.Spans[0].S:body.Spans[0].E]; got != "very" {
		t.Errorf("italic span text = %q", got)
	}
	sub := ch.Paragraphs[2]
	if len(sub.Spans) != 1 || sub.Spans[0].K != tbook.SpanBold {
		t.Fatalf("subtitle spans = %+v", sub.Spans)
	}
	if got := sub.Text[sub.Spans[0].S:sub.Spans[0].E]; got != "bold" {
		t.Errorf("bold span text = %q", got)
	}

	if book.Chapters[1].Title != "" || len(book.Chapters[1].Paragraphs) != 1 {
		t.Errorf("ch1 = %+v", book.Chapters[1])
	}
}

func TestParseFB2Zip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("inner.fb2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(sampleFB2(t))); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	book, err := Parse(writeFile(t, "book.fb2.zip", buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if book.Title != "Атомные привычки" || len(book.Chapters) != 2 {
		t.Errorf("zip parse: title=%q chapters=%d", book.Title, len(book.Chapters))
	}
}

func TestParseFB2Windows1251(t *testing.T) {
	utf8XML := `<?xml version="1.0" encoding="windows-1251"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
 <description><title-info>
  <book-title>Тест</book-title>
  <author><first-name>Иван</first-name><last-name>Петров</last-name></author>
 </title-info></description>
 <body><section><title><p>Глава</p></title><p>Привет, мир.</p></section></body>
</FictionBook>`
	enc, err := charmap.Windows1251.NewEncoder().Bytes([]byte(utf8XML))
	if err != nil {
		t.Fatal(err)
	}
	book, err := Parse(writeFile(t, "cp1251.fb2", enc))
	if err != nil {
		t.Fatal(err)
	}
	if book.Title != "Тест" || book.Author != "Иван Петров" {
		t.Errorf("cp1251: title=%q author=%q", book.Title, book.Author)
	}
	if len(book.Chapters) != 1 || book.Chapters[0].Paragraphs[0].Text != "Привет, мир." {
		t.Errorf("cp1251 chapters = %+v", book.Chapters)
	}
}

func TestParseFB2NoMetadata(t *testing.T) {
	xml := `<?xml version="1.0"?><FictionBook><body><section><p>Text.</p></section></body></FictionBook>`
	book, err := Parse(writeFile(t, "bare.fb2", []byte(xml)))
	if err != nil {
		t.Fatal(err)
	}
	if book.Title != "bare" || book.Author != "Unknown" {
		t.Errorf("fallbacks: title=%q author=%q", book.Title, book.Author)
	}
}
