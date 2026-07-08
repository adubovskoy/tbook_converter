// Package fb2 parses FictionBook 2 files (.fb2, .fb2.zip) into the same
// parsed-book structure the EPUB parser produces, so the rest of the pipeline
// (segment → translate → assemble) is format-agnostic.
//
// Scope mirrors the reference converter's FB2 support: metadata
// (title/author), the cover image, and body sections as chapters with
// paragraph roles (heading, subtitle, scene break) and inline emphasis spans.
// Note bodies (<body name="notes">), body images, tables, poems and epigraphs
// are skipped.
package fb2

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html/charset"

	"github.com/dimando/reader/converter/internal/epub"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
)

// elem is a minimal DOM node. Kids interleaves child *elems with text strings
// in document order — FB2 rich text ("text <emphasis>styled</emphasis> tail")
// needs that interleaving to compute emphasis-span offsets; encoding/xml
// struct unmarshaling collapses it.
type elem struct {
	name string            // local (namespace-stripped) element name
	attr map[string]string // local attr name → value
	kids []any             // *elem | string
}

// child returns the first direct child element with the given local name.
func (e *elem) child(name string) *elem {
	for _, k := range e.kids {
		if el, ok := k.(*elem); ok && el.name == name {
			return el
		}
	}
	return nil
}

// text returns all text content, recursively, in document order.
func (e *elem) text() string {
	var sb strings.Builder
	var walk func(*elem)
	walk = func(n *elem) {
		for _, k := range n.kids {
			switch v := k.(type) {
			case string:
				sb.WriteString(v)
			case *elem:
				walk(v)
			}
		}
	}
	walk(e)
	return sb.String()
}

// Parse reads an FB2 book (.fb2, or .fb2.zip — the first .fb2 entry of the
// archive) into the shared parsed-book structure.
func Parse(path string) (*epub.Book, error) {
	data, err := readFB2Bytes(path)
	if err != nil {
		return nil, err
	}
	root, err := buildDOM(data)
	if err != nil {
		return nil, fmt.Errorf("fb2: %w", err)
	}
	if root.name != "FictionBook" {
		return nil, fmt.Errorf("fb2: root element is <%s>, want <FictionBook>", root.name)
	}

	book := &epub.Book{}
	book.Title, book.Author, book.Cover = metadata(root)
	if book.Title == "" {
		book.Title = fb2ExtRE.ReplaceAllString(filepath.Base(path), "")
	}
	if book.Author == "" {
		book.Author = "Unknown"
	}

	for _, k := range root.kids {
		body, ok := k.(*elem)
		if !ok || body.name != "body" {
			continue
		}
		if n := strings.ToLower(body.attr["name"]); n == "notes" || n == "comments" {
			continue
		}
		hasSection := false
		for _, bk := range body.kids {
			if sec, ok := bk.(*elem); ok && sec.name == "section" {
				hasSection = true
				book.Chapters = append(book.Chapters, sectionToChapter(sec))
			}
		}
		// A body with direct paragraphs and no sections is a single chapter.
		if !hasSection && body.child("p") != nil {
			book.Chapters = append(book.Chapters, sectionToChapter(body))
		}
	}
	return book, nil
}

var fb2ExtRE = regexp.MustCompile(`(?i)\.fb2(\.zip)?$`)

// readFB2Bytes returns the raw XML: the file itself, or the first .fb2 entry
// (else the first entry) when the input is a ZIP archive.
func readFB2Bytes(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return data, nil // not a zip — plain .fb2
	}
	var pick *zip.File
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".fb2") {
			pick = f
			break
		}
	}
	if pick == nil {
		if len(zr.File) == 0 {
			return nil, fmt.Errorf("fb2: empty zip archive")
		}
		pick = zr.File[0]
	}
	rc, err := pick.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// buildDOM token-parses the XML into the interleaved-kids tree. Non-UTF-8
// encodings declared in the XML prolog (windows-1251 is common in FB2) are
// transcoded via the charset reader.
func buildDOM(data []byte) (*elem, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charset.NewReaderLabel
	root := &elem{name: "#doc"}
	stack := []*elem{root}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		top := stack[len(stack)-1]
		switch t := tok.(type) {
		case xml.StartElement:
			el := &elem{name: t.Name.Local, attr: map[string]string{}}
			for _, a := range t.Attr {
				el.attr[a.Name.Local] = a.Value
			}
			top.kids = append(top.kids, el)
			stack = append(stack, el)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if s := string(t); s != "" {
				top.kids = append(top.kids, s)
			}
		}
	}
	for _, k := range root.kids {
		if el, ok := k.(*elem); ok {
			return el, nil
		}
	}
	return nil, fmt.Errorf("no root element")
}

// metadata extracts title, author and cover bytes from description/title-info
// and the referenced <binary>.
func metadata(root *elem) (title, author string, cover []byte) {
	desc := root.child("description")
	if desc == nil {
		return "", "", nil
	}
	ti := desc.child("title-info")
	if ti == nil {
		return "", "", nil
	}
	coverHref := ""
	for _, k := range ti.kids {
		el, ok := k.(*elem)
		if !ok {
			continue
		}
		switch el.name {
		case "book-title":
			if title == "" {
				title = segment.CleanText(el.text())
			}
		case "author":
			if author == "" {
				first, last := "", ""
				for _, p := range el.kids {
					pe, ok := p.(*elem)
					if !ok {
						continue
					}
					switch pe.name {
					case "first-name":
						first = strings.TrimSpace(pe.text())
					case "last-name":
						last = strings.TrimSpace(pe.text())
					case "nickname":
						if first == "" && last == "" {
							first = strings.TrimSpace(pe.text())
						}
					}
				}
				author = segment.CleanText(strings.TrimSpace(first + " " + last))
			}
		case "coverpage":
			if coverHref == "" {
				if img := findImage(el); img != nil {
					coverHref = strings.TrimPrefix(imageHref(img), "#")
				}
			}
		}
	}
	return title, author, binaryBytes(root, coverHref)
}

// findImage returns the first <image> descendant.
func findImage(e *elem) *elem {
	for _, k := range e.kids {
		el, ok := k.(*elem)
		if !ok {
			continue
		}
		if el.name == "image" {
			return el
		}
		if img := findImage(el); img != nil {
			return img
		}
	}
	return nil
}

// imageHref returns the image's href attribute regardless of its namespace
// prefix (xlink:href arrives with the local name "href").
func imageHref(img *elem) string {
	return img.attr["href"]
}

// binaryBytes base64-decodes the <binary id=…> element's content.
func binaryBytes(root *elem, id string) []byte {
	if id == "" {
		return nil
	}
	for _, k := range root.kids {
		el, ok := k.(*elem)
		if !ok || el.name != "binary" || el.attr["id"] != id {
			continue
		}
		raw := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				return -1
			}
			return r
		}, el.text())
		b, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil
		}
		return b
	}
	return nil
}

// sectionToChapter converts one <section> (or a section-less <body>) into a
// chapter: its direct <title> becomes the chapter title; content paragraphs
// keep their roles and emphasis spans; nested sections flatten in.
func sectionToChapter(sec *elem) segment.ParsedChapter {
	ch := segment.ParsedChapter{}
	if t := sec.child("title"); t != nil {
		ch.Title = segment.CleanText(t.text())
	}
	var walk func(node *elem, isTop bool)
	walk = func(node *elem, isTop bool) {
		for _, k := range node.kids {
			el, ok := k.(*elem)
			if !ok {
				continue
			}
			switch el.name {
			case "title":
				if isTop {
					continue // already the chapter title
				}
				if text := segment.CleanText(el.text()); text != "" {
					ch.Paragraphs = append(ch.Paragraphs,
						segment.ParsedParagraph{Text: text, Role: tbook.RoleHeading})
				}
			case "subtitle":
				appendPara(&ch, el, tbook.RoleSubtitle)
			case "empty-line":
				ch.Paragraphs = append(ch.Paragraphs,
					segment.ParsedParagraph{Role: tbook.RoleSceneBreak})
			case "p":
				appendPara(&ch, el, tbook.RoleBody)
			case "section":
				walk(el, false)
				// epigraph, annotation, cite, poem, image, table: skipped
			}
		}
	}
	walk(sec, true)
	return ch
}

// appendPara renders one paragraph element with emphasis spans and appends it
// (empty paragraphs are dropped).
func appendPara(ch *segment.ParsedChapter, el *elem, role string) {
	raw, spans := richText(el)
	text, spans, _ := segment.CleanWithSpans(raw, spans, nil)
	if text == "" {
		return
	}
	ch.Paragraphs = append(ch.Paragraphs,
		segment.ParsedParagraph{Text: text, Role: role, Spans: spans})
}

// richText flattens an element's mixed content into text plus emphasis spans
// (<emphasis>/<i>/<em> → italic, <strong>/<b> → bold), offsets in RUNES of the
// raw (uncleaned) text, per the .tbook spec.
func richText(el *elem) (string, []tbook.Span) {
	var sb strings.Builder
	var spans []tbook.Span
	pos := 0 // rune offset
	emit := func(s string, italic, bold bool) {
		if s == "" {
			return
		}
		start := pos
		sb.WriteString(s)
		pos += utf8.RuneCountInString(s)
		if italic {
			spans = append(spans, tbook.Span{S: start, E: pos, K: tbook.SpanItalic})
		}
		if bold {
			spans = append(spans, tbook.Span{S: start, E: pos, K: tbook.SpanBold})
		}
	}
	var walk func(n *elem, italic, bold bool)
	walk = func(n *elem, italic, bold bool) {
		for _, k := range n.kids {
			switch v := k.(type) {
			case string:
				emit(v, italic, bold)
			case *elem:
				it, bd := italic, bold
				switch v.name {
				case "emphasis", "i", "em":
					it = true
				case "strong", "b":
					bd = true
				}
				walk(v, it, bd)
			}
		}
	}
	walk(el, false, false)
	return sb.String(), spans
}
