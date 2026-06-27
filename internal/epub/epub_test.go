package epub

import (
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
	cur := parseDoc([]byte(doc), nil, &out, false)
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
