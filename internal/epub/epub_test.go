package epub

import (
	"archive/zip"
	"strings"
	"testing"

	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
)

// parseChapters runs one XHTML body through parseDoc and flushes the last chapter.
func parseChapters(t *testing.T, body string) []segment.ParsedChapter {
	t.Helper()
	doc := `<html><body>` + body + `</body></html>`
	var out []segment.ParsedChapter
	p := newTestParser()
	cur := p.parseDoc("OPS/doc.xhtml", []byte(doc), nil, &out, false)
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

func TestParseDocRolesAndEmphasis(t *testing.T) {
	body := `
    <div class="title1"><p class="p">THIRTY-ONE</p></div>
    <p class="subtitle"><em>Cerberus, Delta Pavonis</em></p>
    <p class="empty-line"/>
    <p class="p1">He realised the suits <em>were</em> spacecraft.</p>`
	chs := parseChapters(t, body)
	if len(chs) != 1 {
		t.Fatalf("chapters = %d, want 1", len(chs))
	}
	ch := chs[0]
	if ch.Title != "THIRTY-ONE" {
		t.Fatalf("title = %q", ch.Title)
	}
	if len(ch.Paragraphs) != 3 {
		t.Fatalf("paragraphs = %d, want 3 (subtitle, scene break, body)", len(ch.Paragraphs))
	}

	sub := ch.Paragraphs[0]
	if sub.Role != tbook.RoleSubtitle {
		t.Errorf("paragraph 0 role = %q, want subtitle", sub.Role)
	}
	if len(sub.Spans) != 1 || emphText(sub.Text, sub.Spans[0]) != "Cerberus, Delta Pavonis" {
		t.Errorf("subtitle spans = %+v over %q", sub.Spans, sub.Text)
	}

	sb := ch.Paragraphs[1]
	if sb.Role != tbook.RoleSceneBreak || sb.Text != "" {
		t.Errorf("paragraph 1 = %+v, want empty sceneBreak", sb)
	}

	bodyP := ch.Paragraphs[2]
	if bodyP.Role != tbook.RoleBody {
		t.Errorf("paragraph 2 role = %q, want body", bodyP.Role)
	}
	if bodyP.Text != "He realised the suits were spacecraft." {
		t.Errorf("body text = %q", bodyP.Text)
	}
	if len(bodyP.Spans) != 1 || bodyP.Spans[0].K != tbook.SpanItalic ||
		emphText(bodyP.Text, bodyP.Spans[0]) != "were" {
		t.Errorf("body spans = %+v over %q", bodyP.Spans, bodyP.Text)
	}
}

func TestRichTextNestedEmphasisUnions(t *testing.T) {
	chs := parseChapters(t,
		`<div class="title1">T</div><p class="p1">a <b><em>x</em></b> b</p>`)
	p := chs[0].Paragraphs[0]
	// bold-italic "x" emits both an "i" and a "b" span over the same range.
	var kinds []string
	for _, sp := range p.Spans {
		if emphText(p.Text, sp) == "x" {
			kinds = append(kinds, sp.K)
		}
	}
	if !(contains(kinds, tbook.SpanItalic) && contains(kinds, tbook.SpanBold)) {
		t.Fatalf("nested emphasis kinds over \"x\" = %v, want both i and b", kinds)
	}
}

// TestParseGutenbergChapterDiv covers Project Gutenberg's ebookmaker layout:
// each chapter is wrapped in <div class="chapter"> with the title in an inner
// heading, and a trailing pg-boilerplate footer carries the license. The title
// comes from the first heading (the redundant chapter-number heading is dropped),
// body prose is kept, and the boilerplate heading must not leak into the chapter.
func TestParseGutenbergChapterDiv(t *testing.T) {
	body := `
    <div class="chapter">
      <h2><a id="chap01"/>I.<br/>
A SCANDAL IN BOHEMIA</h2>
      <h3>I.</h3>
      <p class="pfirst">To Sherlock Holmes she is always the woman.</p>
      <p>I had seen little of Holmes lately.</p>
    </div>
    <footer class="pg-boilerplate pgheader" id="pg-footer">
      <h2>THE FULL PROJECT GUTENBERG LICENSE</h2>
    </footer>`
	chs := parseChapters(t, body)
	if len(chs) != 1 {
		t.Fatalf("chapters = %d, want 1", len(chs))
	}
	ch := chs[0]
	if ch.Title != "I. A SCANDAL IN BOHEMIA" {
		t.Fatalf("title = %q, want %q", ch.Title, "I. A SCANDAL IN BOHEMIA")
	}
	if len(ch.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2 (leading headings + boilerplate excluded)", len(ch.Paragraphs))
	}
	for _, p := range ch.Paragraphs {
		if p.Role != tbook.RoleBody {
			t.Errorf("paragraph role = %q, want body", p.Role)
		}
		if strings.Contains(p.Text, "LICENSE") {
			t.Errorf("Gutenberg boilerplate leaked into chapter: %q", p.Text)
		}
	}
	if ch.Paragraphs[0].Text != "To Sherlock Holmes she is always the woman." {
		t.Errorf("first body paragraph = %q", ch.Paragraphs[0].Text)
	}
}

func emphText(s string, sp tbook.Span) string {
	r := []rune(s)
	if sp.S < 0 || sp.E > len(r) || sp.S >= sp.E {
		return ""
	}
	return string(r[sp.S:sp.E])
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// newTestParser builds a parser with empty state for direct parseDoc tests.
func newTestParser() *parser {
	return &parser{
		files:        map[string]*zip.File{},
		book:         &Book{},
		noteIDByKey:  map[string]string{},
		noteFiles:    map[string]bool{},
		imgNameBySrc: map[string]string{},
	}
}

func TestNoterefStrippedAndMarked(t *testing.T) {
	body := `
    <div class="title1">C</div>
    <p>El año 2003.<sup><a href="notas.htm#destino_1" id="origen_1">1</a></sup> y todo cambió.<a href="notas.htm#destino_2" id="origen_2"><sup>*</sup></a></p>`
	doc := `<html><body>` + body + `</body></html>`
	var out []segment.ParsedChapter
	p := newTestParser()
	cur := p.parseDoc("OPS/doc.xhtml", []byte(doc), nil, &out, false)
	if cur == nil {
		t.Fatal("no chapter")
	}
	para := cur.Paragraphs[0]
	if strings.Contains(para.Text, "1") || strings.Contains(para.Text, "*") {
		t.Fatalf("marker text leaked into paragraph: %q", para.Text)
	}
	if para.Text != "El año 2003. y todo cambió." {
		t.Fatalf("text = %q", para.Text)
	}
	if len(para.Marks) != 2 {
		t.Fatalf("marks = %+v, want 2", para.Marks)
	}
	// First marker sits right after "2003." (rune offset 12), second at the end.
	if para.Marks[0].Pos != 12 || para.Marks[0].Label != "1" {
		t.Errorf("mark 0 = %+v, want pos 12 label 1", para.Marks[0])
	}
	if para.Marks[1].Pos != len([]rune(para.Text)) || para.Marks[1].Label != "*" {
		t.Errorf("mark 1 = %+v, want pos at end, label *", para.Marks[1])
	}
	// Targets registered against the resolved file.
	if len(p.noteTargets) != 2 || p.noteTargets[0].file != "OPS/notas.htm" || p.noteTargets[0].frag != "destino_1" {
		t.Errorf("noteTargets = %+v", p.noteTargets)
	}
	// A plain hyperlink must NOT be treated as a marker.
	var out2 []segment.ParsedChapter
	cur = p.parseDoc("OPS/doc.xhtml", []byte(`<html><body><div class="title1">C</div><p>Visit <a href="http://x.com/#frag">this site</a> now.</p></body></html>`), nil, &out2, false)
	if cur.Paragraphs[0].Text != "Visit this site now." {
		t.Fatalf("plain link text lost: %q", cur.Paragraphs[0].Text)
	}
}

func TestTableExtraction(t *testing.T) {
	body := `
    <div class="title1">C</div>
    <p>Intro.</p>
    <table class="tabla">
      <tr><td class="td50_cab"><strong>Buenos</strong></td><td class="td50_cab">Malos</td></tr>
      <tr><td class="td50">Leer<sup><a href="notas.htm#d1" id="o1">7</a></sup> cada día</td><td class="td50">Fumar</td></tr>
    </table>`
	chs := parseChapters(t, body)
	ch := chs[0]
	if len(ch.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2 (intro + table)", len(ch.Paragraphs))
	}
	tp := ch.Paragraphs[1]
	if tp.Role != tbook.RoleTable || tp.Table == nil {
		t.Fatalf("paragraph 1 = %+v, want table", tp)
	}
	if len(tp.Table.Rows) != 2 || len(tp.Table.Rows[0]) != 2 {
		t.Fatalf("rows = %+v", tp.Table.Rows)
	}
	if !tp.Table.Rows[0][0].Header || !tp.Table.Rows[0][1].Header {
		t.Errorf("first row should be header cells: %+v", tp.Table.Rows[0])
	}
	cell := tp.Table.Rows[1][0]
	if cell.Text != "Leer cada día" {
		t.Errorf("cell text = %q (marker should be stripped)", cell.Text)
	}
	if len(cell.Marks) != 1 || cell.Marks[0].Label != "7" {
		t.Errorf("cell marks = %+v", cell.Marks)
	}
	if tp.Table.Rows[1][1].Header {
		t.Errorf("body cell marked as header")
	}
}

func TestHrSceneBreakOnlyMidChapter(t *testing.T) {
	body := `
    <div class="title1">C</div>
    <hr/>
    <p>First part.</p>
    <hr/>
    <p>Second part.</p>
    <hr/>`
	chs := parseChapters(t, body)
	ch := chs[0]
	cleanupSceneBreaks(&ch) // normally applied by ParseOpts after the spine walk
	var roles []string
	for _, p := range ch.Paragraphs {
		roles = append(roles, p.Role)
	}
	want := []string{tbook.RoleBody, tbook.RoleSceneBreak, tbook.RoleBody}
	if len(roles) != 3 || roles[0] != want[0] || roles[1] != want[1] || roles[2] != want[2] {
		t.Fatalf("roles = %v, want %v (leading/trailing hr dropped)", roles, want)
	}
}

func TestInsertTitleHeading(t *testing.T) {
	ch := segment.ParsedChapter{
		Title: "I. A SCANDAL IN BOHEMIA",
		Paragraphs: []segment.ParsedParagraph{
			{Text: "To Sherlock Holmes she is always the woman.", Role: tbook.RoleBody},
		},
	}
	insertTitleHeading(&ch)
	if len(ch.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2 (title heading + body)", len(ch.Paragraphs))
	}
	if h := ch.Paragraphs[0]; h.Role != tbook.RoleHeading || h.Text != ch.Title {
		t.Fatalf("paragraph 0 = %+v, want the title as heading", h)
	}

	// Idempotent: an identical leading heading is not duplicated.
	insertTitleHeading(&ch)
	if len(ch.Paragraphs) != 2 {
		t.Fatalf("paragraphs after second insert = %d, want 2", len(ch.Paragraphs))
	}

	// A different leading heading (a section title) stays below the inserted one.
	ch2 := segment.ParsedChapter{
		Title: "Chapter One",
		Paragraphs: []segment.ParsedParagraph{
			{Text: "PART ONE", Role: tbook.RoleHeading},
		},
	}
	insertTitleHeading(&ch2)
	if len(ch2.Paragraphs) != 2 || ch2.Paragraphs[0].Text != "Chapter One" {
		t.Fatalf("paragraphs = %+v, want inserted title above section heading", ch2.Paragraphs)
	}

	// Blank titles insert nothing.
	empty := segment.ParsedChapter{
		Paragraphs: []segment.ParsedParagraph{{Text: "x", Role: tbook.RoleBody}},
	}
	insertTitleHeading(&empty)
	if len(empty.Paragraphs) != 1 {
		t.Fatalf("blank title inserted a heading: %+v", empty.Paragraphs)
	}
}

func TestMatterSkipPatterns(t *testing.T) {
	for _, s := range []string{"Portada", "Sinopsis", "Créditos", "Índice", "Notas", "notas", "Table of Contents"} {
		if !isMatter(s) {
			t.Errorf("isMatter(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"Portadilla", "Capítulo 1. El poder", "Introducción. Mi historia", "El contenido de la felicidad", "Notas sobre el amor"} {
		if isMatter(s) {
			t.Errorf("isMatter(%q) = true, want false", s)
		}
	}
}

func TestDedupTitleHeadings(t *testing.T) {
	ch := segment.ParsedChapter{
		Title: "Capítulo 1. El sorprendente poder de los hábitos atómicos",
		Paragraphs: []segment.ParsedParagraph{
			{Text: "CAPÍTULO", Role: tbook.RoleHeading},
			{Text: "1", Role: tbook.RoleHeading},
			{Text: "El sorprendente poder", Role: tbook.RoleHeading},
			{Text: "de los hábitos atómicos", Role: tbook.RoleHeading},
			{Text: "PRIMERA SECCIÓN", Role: tbook.RoleHeading},
			{Text: "Cuerpo.", Role: tbook.RoleBody},
		},
	}
	dedupTitleHeadings(&ch)
	if len(ch.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2 (title headings dropped, section heading kept)", len(ch.Paragraphs))
	}
	if ch.Paragraphs[0].Text != "PRIMERA SECCIÓN" {
		t.Fatalf("first remaining = %q", ch.Paragraphs[0].Text)
	}
}
