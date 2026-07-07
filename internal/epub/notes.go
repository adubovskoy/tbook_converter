// Note-body resolution: after the spine walk collects in-text markers (each a
// target "file#fragment"), this pass parses each target file once, locates the
// anchor, extracts the surrounding note body as paragraphs, classifies it
// (content note vs bibliographic citation), and prunes markers whose body could
// not be found.
package epub

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
)

// noteTarget is one distinct marker destination, in first-reference order.
type noteTarget struct {
	id    string // assigned note id ("n1", "n2", …)
	file  string // resolved zip entry name
	frag  string // anchor id within the file
	label string // display label from the first marker ("1", "*", …)
}

// blockTags are elements that can host a note body.
var noteBlockTags = map[string]bool{
	"p": true, "li": true, "td": true, "dd": true, "blockquote": true,
	"aside": true, "div": true, "section": true,
}

// resolveNotes builds book.Notes from the collected targets and removes
// markers whose note body could not be located.
func (p *parser) resolveNotes() {
	if len(p.noteTargets) == 0 {
		return
	}
	docs := map[string]*goquery.Document{}
	resolved := map[string]bool{}
	for _, t := range p.noteTargets {
		doc, ok := docs[t.file]
		if !ok {
			doc = nil
			if content, err := readNamed(p.files, t.file); err == nil {
				if d, err := goquery.NewDocumentFromReader(strings.NewReader(fixSelfClosing(string(content)))); err == nil {
					doc = d
				}
			}
			docs[t.file] = doc
		}
		if doc == nil || strings.ContainsAny(t.frag, `"'\`) {
			continue
		}
		sel := doc.Find(`[id="` + t.frag + `"]`).First()
		if sel.Length() == 0 {
			continue
		}
		paras := p.noteBody(sel)
		if len(paras) == 0 {
			continue
		}
		p.book.Notes = append(p.book.Notes, segment.ParsedNote{
			ID: t.id, Label: t.label, Kind: classifyNote(paras), Paragraphs: paras,
		})
		resolved[t.id] = true
	}
	p.pruneMarks(resolved)
}

// noteBody extracts the note's paragraphs given the anchor selection. The
// anchor element itself (the backlink label) is omitted from the text.
func (p *parser) noteBody(anchor *goquery.Selection) []segment.ParsedParagraph {
	anchorNode := anchor.Nodes[0]

	// Find the hosting block: the anchor itself if it is a block, else the
	// nearest block ancestor.
	block := anchor
	if !noteBlockTags[goquery.NodeName(block)] {
		block = anchor.ParentsFiltered("p, li, td, dd, blockquote, aside, div, section").First()
		if block.Length() == 0 {
			return nil
		}
	}

	var paras []segment.ParsedParagraph
	addPara := func(sel *goquery.Selection, trimLead bool) {
		raw, spans, _ := richText(sel, richOpts{skipNode: anchorNode})
		text, cspans, _ := segment.CleanWithSpans(raw, spans, nil)
		if trimLead {
			text, cspans = trimLeadingPunct(text, cspans)
		}
		if strings.TrimSpace(text) == "" {
			return
		}
		paras = append(paras, segment.ParsedParagraph{Text: text, Role: tbook.RoleBody, Spans: cspans})
	}

	switch goquery.NodeName(block) {
	case "aside", "div", "section":
		// Container note: each child <p> is a paragraph; a container without
		// <p> children is a single paragraph itself.
		ps := block.ChildrenFiltered("p")
		if ps.Length() == 0 {
			addPara(block, true)
		} else {
			ps.Each(func(i int, s *goquery.Selection) { addPara(s, i == 0) })
		}
	default:
		addPara(block, true)
	}
	return paras
}

// trimLeadingPunct drops the separator debris left at the start of a note body
// after the backlink label is removed (e.g. ". " after "1"), shifting emphasis
// spans accordingly.
func trimLeadingPunct(text string, spans []tbook.Span) (string, []tbook.Span) {
	cut := 0
	for _, r := range text {
		if strings.ContainsRune(".,:;)]* ", r) {
			cut++
			continue
		}
		break
	}
	if cut == 0 {
		return text, spans
	}
	rs := []rune(text)
	out := string(rs[cut:])
	n := utf8.RuneCountInString(out)
	var shifted []tbook.Span
	for _, sp := range spans {
		s, e := sp.S-cut, sp.E-cut
		if s < 0 {
			s = 0
		}
		if e > n {
			e = n
		}
		if s < e {
			shifted = append(shifted, tbook.Span{S: s, E: e, K: sp.K})
		}
	}
	return out, shifted
}

var (
	urlRE  = regexp.MustCompile(`(?i)https?://|www\.|\bdoi\b`)
	yearRE = regexp.MustCompile(`\b(1[5-9]\d{2}|20\d{2})\b`)
)

// classifyNote guesses whether a note is a bibliographic citation (source
// reference — URL, or a comma-heavy line with a publication year) or a content
// note. Only used to let a producer skip translating pure citations; a wrong
// guess is cosmetic.
func classifyNote(paras []segment.ParsedParagraph) string {
	var sb strings.Builder
	for _, p := range paras {
		sb.WriteString(p.Text)
		sb.WriteString(" ")
	}
	text := sb.String()
	if urlRE.MatchString(text) {
		return tbook.NoteKindCitation
	}
	if strings.Count(text, ",") >= 3 && yearRE.MatchString(text) {
		return tbook.NoteKindCitation
	}
	return tbook.NoteKindNote
}

// pruneMarks removes note markers whose id was never resolved to a body, from
// every paragraph, figure caption, and table cell.
func (p *parser) pruneMarks(resolved map[string]bool) {
	keep := func(marks []segment.Mark) []segment.Mark {
		out := marks[:0]
		for _, m := range marks {
			if resolved[m.ID] {
				out = append(out, m)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	for ci := range p.book.Chapters {
		ch := &p.book.Chapters[ci]
		for pi := range ch.Paragraphs {
			para := &ch.Paragraphs[pi]
			para.Marks = keep(para.Marks)
			if para.Table != nil {
				for ri := range para.Table.Rows {
					for cj := range para.Table.Rows[ri] {
						cell := &para.Table.Rows[ri][cj]
						cell.Marks = keep(cell.Marks)
					}
				}
			}
		}
	}
}
