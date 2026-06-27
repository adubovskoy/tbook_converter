// Package epub parses an EPUB into chapters of raw paragraph text, in spine
// (reading) order. Chapters split on <div class="title1"> (title is the div's
// text) or <div class="chapter"> (title is its first inner heading, the style
// Project Gutenberg's ebookmaker emits); <div class="title"|"epigraph"> and
// Gutenberg's <*.pg-boilerplate> header/footer are skipped; only <p> prose
// after the first chapter heading is kept. The cover image bytes are extracted
// for the archive.
package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	"golang.org/x/net/html"
)

// XHTML often self-closes RAWTEXT/RCDATA elements (e.g. <title/>). The HTML5
// parser ignores the slash, enters raw-text mode, and swallows the rest of the
// document. Rewrite those to explicit open/close before parsing. (Void elements
// like <link/>, <meta/>, <br/> are fine self-closed and are left alone.)
var selfClosingRawtext = regexp.MustCompile(`(?is)<(title|textarea|style|script|iframe|noembed|noframes|xmp)\b([^>]*?)/>`)

func fixSelfClosing(s string) string {
	return selfClosingRawtext.ReplaceAllString(s, "<$1$2></$1>")
}

// Book is the parsed result.
type Book struct {
	Title    string
	Author   string
	Cover    []byte // nil if none
	Chapters []segment.ParsedChapter
}

type container struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type opf struct {
	Metadata struct {
		Title   []string `xml:"title"`   // dc:title
		Creator []string `xml:"creator"` // dc:creator
		Metas   []struct {
			Name    string `xml:"name,attr"`
			Content string `xml:"content,attr"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest struct {
		Items []struct {
			ID         string `xml:"id,attr"`
			Href       string `xml:"href,attr"`
			MediaType  string `xml:"media-type,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Itemrefs []struct {
			IDref string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

// ncx is the EPUB2 navigation document. Only the TOP-LEVEL navPoints (direct
// children of navMap) are unmarshaled; nested navPoints (a story's numbered
// parts) are intentionally not modeled, so each story becomes one chapter.
type ncx struct {
	NavMap struct {
		NavPoints []ncxNavPoint `xml:"navPoint"`
	} `xml:"navMap"`
}

type ncxNavPoint struct {
	Text    string `xml:"navLabel>text"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
}

type tocEntry struct {
	Title string
	File  string // resolved zip entry name (fragment stripped)
}

// Parse reads the EPUB at path and returns its title, author, cover, and
// chapters.
func Parse(epubPath string) (*Book, error) {
	zr, err := zip.OpenReader(epubPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[path.Clean(f.Name)] = f
	}

	opfPath, err := rootfilePath(files)
	if err != nil {
		return nil, err
	}
	pkg, err := readOPF(files, opfPath)
	if err != nil {
		return nil, err
	}
	opfDir := path.Dir(opfPath)

	// Manifest lookup by id → resolved zip path.
	hrefByID := map[string]string{}
	for _, it := range pkg.Manifest.Items {
		hrefByID[it.ID] = resolve(opfDir, it.Href)
	}

	book := &Book{
		Title:  firstNonEmpty(pkg.Metadata.Title),
		Author: firstNonEmpty(pkg.Metadata.Creator),
		Cover:  extractCover(files, pkg, opfDir, hrefByID),
	}
	if book.Title == "" {
		book.Title = strings.TrimSuffix(path.Base(epubPath), path.Ext(epubPath))
	}
	if book.Author == "" {
		book.Author = "Unknown"
	}

	// Prefer the EPUB's own navigation (NCX) for chapter boundaries: each
	// TOP-LEVEL nav entry is one chapter. This groups multi-file stories and
	// folds nested sub-sections (numbered parts) into their parent — far more
	// reliable than guessing from heading classes, which collapses or
	// over-splits anthologies. Falls back to heading-based splitting (inside
	// parseDoc) when no usable NCX is present.
	chapterStarts := map[string]string{}
	for _, e := range tocTopLevel(files, pkg, opfDir) {
		if _, ok := chapterStarts[e.File]; !ok {
			chapterStarts[e.File] = e.Title
		}
	}
	tocMode := len(chapterStarts) >= 2

	// Walk spine docs in order, accumulating chapters. `current` persists across
	// files (a chapter may continue into the next spine document).
	var current *segment.ParsedChapter
	for _, ref := range pkg.Spine.Itemrefs {
		name := hrefByID[ref.IDref]
		if name == "" {
			continue
		}
		if strings.HasSuffix(strings.ToLower(name), "cover.xhtml") {
			continue
		}
		f := files[name]
		if f == nil {
			continue
		}
		content, err := readAll(f)
		if err != nil {
			continue
		}
		if tocMode {
			if title, ok := chapterStarts[name]; ok {
				if current != nil {
					book.Chapters = append(book.Chapters, *current)
				}
				current = &segment.ParsedChapter{Title: title}
			}
		}
		current = parseDoc(content, current, &book.Chapters, tocMode)
	}
	if current != nil {
		book.Chapters = append(book.Chapters, *current)
	}

	// Drop bare title-page chapters (no prose) that would render blank.
	book.Chapters = dropEmptyChapters(book.Chapters)
	return book, nil
}

// parseDoc walks one XHTML document's block elements in document order, updating
// the current chapter and appending completed chapters. Returns the (possibly
// new) current chapter. Block prose carries its role (heading/subtitle/scene
// break/body) and inline italic/bold spans for formatting fidelity.
func parseDoc(content []byte, current *segment.ParsedChapter, out *[]segment.ParsedChapter, tocMode bool) *segment.ParsedChapter {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(fixSelfClosing(string(content))))
	if err != nil {
		return current
	}
	// titlePending is true from a <div class="chapter"> boundary until the
	// chapter's first body paragraph: leading headings inside the wrapper form
	// the title and are not repeated as body (mirrors the div.title1 path).
	titlePending := false
	doc.Find("body").Find("div, p, h1, h2, h3, h4, h5, h6").Each(func(_ int, s *goquery.Selection) {
		if goquery.NodeName(s) == "div" {
			// When the EPUB navigation (NCX) drives chapter boundaries, heading
			// divs do NOT split chapters here — that would over-split anthologies
			// whose stories use mixed heading classes and whose multi-part
			// stories nest numbered sub-sections.
			if !tocMode {
				switch {
				case hasTitleNClass(s): // boundary; any "titleN" div's text is the title
					if current != nil {
						*out = append(*out, *current)
					}
					current = &segment.ParsedChapter{Title: segment.CleanText(s.Text())}
					titlePending = false
				case s.HasClass("chapter"): // boundary; title comes from its first heading
					if current != nil {
						*out = append(*out, *current)
					}
					current = &segment.ParsedChapter{}
					titlePending = true
				}
			}
			// heading / "title" / "epigraph" divs carry no body prose themselves.
			return
		}
		// <p>/<hN> prose.
		if inSpecialDiv(s) || inBoilerplate(s) || current == nil {
			return
		}
		role := paragraphRole(s)
		if titlePending && role == tbook.RoleHeading {
			// Leading heading(s) of a div.chapter: the first fills the chapter
			// title; any others (e.g. a repeated chapter number) are dropped.
			if current.Title == "" {
				current.Title = segment.CleanText(s.Text())
			}
			return
		}
		titlePending = false
		raw, rawSpans := richText(s)
		text, spans := segment.CleanWithSpans(raw, rawSpans)
		if text == "" && role != tbook.RoleSceneBreak {
			return
		}
		current.Paragraphs = append(current.Paragraphs, segment.ParsedParagraph{
			Text:  text,
			Role:  role,
			Spans: spans,
		})
	})
	return current
}

// titleNClass matches heading-div classes title1, title2, … (story/section
// headings) but not the plain "title" book-title wrapper.
var titleNClass = regexp.MustCompile(`^title\d+$`)

func hasTitleNClass(s *goquery.Selection) bool {
	cls, _ := s.Attr("class")
	for _, c := range strings.Fields(cls) {
		if titleNClass.MatchString(c) {
			return true
		}
	}
	return false
}

// tocTopLevel returns the ordered TOP-LEVEL table-of-contents entries from the
// NCX (EPUB2). Nested sub-entries (e.g. a story's numbered parts) are excluded
// so each story/section becomes exactly one chapter. Returns nil when no usable
// NCX is found (caller then falls back to heading-based splitting).
func tocTopLevel(files map[string]*zip.File, pkg *opf, opfDir string) []tocEntry {
	var ncxPath string
	for _, it := range pkg.Manifest.Items {
		if it.MediaType == "application/x-dtbncx+xml" ||
			strings.HasSuffix(strings.ToLower(it.Href), ".ncx") {
			ncxPath = resolve(opfDir, it.Href)
			break
		}
	}
	if ncxPath == "" {
		return nil
	}
	b, err := readNamed(files, ncxPath)
	if err != nil {
		return nil
	}
	var n ncx
	if err := xml.Unmarshal(b, &n); err != nil {
		return nil
	}
	ncxDir := path.Dir(ncxPath)
	var out []tocEntry
	for _, np := range n.NavMap.NavPoints {
		src := np.Content.Src
		if src == "" {
			continue
		}
		if i := strings.IndexByte(src, '#'); i >= 0 {
			src = src[:i]
		}
		out = append(out, tocEntry{
			Title: segment.CleanText(np.Text),
			File:  resolve(ncxDir, src),
		})
	}
	return out
}

// dropEmptyChapters removes chapters with no prose (e.g. a bare title page),
// which render as a blank "1/1" page. Keeps the input if every chapter is empty.
func dropEmptyChapters(chs []segment.ParsedChapter) []segment.ParsedChapter {
	kept := make([]segment.ParsedChapter, 0, len(chs))
	for _, c := range chs {
		if chapterHasBody(c) {
			kept = append(kept, c)
		}
	}
	if len(kept) == 0 {
		return chs
	}
	return kept
}

func chapterHasBody(c segment.ParsedChapter) bool {
	for _, p := range c.Paragraphs {
		if strings.TrimSpace(p.Text) != "" {
			return true
		}
	}
	return false
}

// paragraphRole maps a block element to its .tbook role. Headings (h1–h6) and the
// EPUB conventions seen in real books — class "subtitle" for chapter subtitles
// and class "empty-line" for scene breaks — drive the role; everything else is
// ordinary body text.
func paragraphRole(s *goquery.Selection) string {
	switch goquery.NodeName(s) {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return tbook.RoleHeading
	}
	switch {
	case s.HasClass("subtitle"):
		return tbook.RoleSubtitle
	case s.HasClass("empty-line"):
		return tbook.RoleSceneBreak
	default:
		return tbook.RoleBody
	}
}

// richText returns an element's text (identical to goquery's .Text(), so
// sentence segmentation and the translation cache are unaffected) plus the
// inline italic/bold spans found in it, in raw-text rune coordinates. Nested
// emphasis emits overlapping spans (e.g. bold inside italic → both "i" and "b"),
// which the consumer unions.
func richText(s *goquery.Selection) (string, []tbook.Span) {
	if len(s.Nodes) == 0 {
		return "", nil
	}
	var sb strings.Builder
	var spans []tbook.Span
	pos := 0
	var walk func(n *html.Node, italic, bold bool)
	walk = func(n *html.Node, italic, bold bool) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch c.Type {
			case html.TextNode:
				start := pos
				sb.WriteString(c.Data)
				pos += utf8.RuneCountInString(c.Data)
				if pos > start {
					if italic {
						spans = append(spans, tbook.Span{S: start, E: pos, K: tbook.SpanItalic})
					}
					if bold {
						spans = append(spans, tbook.Span{S: start, E: pos, K: tbook.SpanBold})
					}
				}
			case html.ElementNode:
				it, bd := italic, bold
				switch strings.ToLower(c.Data) {
				case "em", "i":
					it = true
				case "b", "strong":
					bd = true
				}
				walk(c, it, bd)
			}
		}
	}
	walk(s.Nodes[0], false, false)
	return sb.String(), spans
}

// inSpecialDiv reports whether the element is inside a title/title1/epigraph div.
func inSpecialDiv(s *goquery.Selection) bool {
	return s.ParentsFiltered("div.title, div.title1, div.epigraph").Length() > 0
}

// inBoilerplate reports whether the element is inside Project Gutenberg's
// pg-boilerplate header/footer (ebook banner, license text) — never book prose.
func inBoilerplate(s *goquery.Selection) bool {
	return s.Closest(".pg-boilerplate").Length() > 0
}

func rootfilePath(files map[string]*zip.File) (string, error) {
	f := files["META-INF/container.xml"]
	if f == nil {
		return "", errMissing("META-INF/container.xml")
	}
	b, err := readAll(f)
	if err != nil {
		return "", err
	}
	var c container
	if err := xml.Unmarshal(b, &c); err != nil {
		return "", err
	}
	if len(c.Rootfiles) == 0 || c.Rootfiles[0].FullPath == "" {
		return "", errMissing("rootfile in container.xml")
	}
	return path.Clean(c.Rootfiles[0].FullPath), nil
}

func readOPF(files map[string]*zip.File, opfPath string) (*opf, error) {
	f := files[opfPath]
	if f == nil {
		return nil, errMissing(opfPath)
	}
	b, err := readAll(f)
	if err != nil {
		return nil, err
	}
	var pkg opf
	if err := xml.Unmarshal(b, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

// extractCover mirrors ebooklib's cover heuristics: EPUB3 manifest
// properties="cover-image"; EPUB2 <meta name="cover" content="ID">; else an
// image whose name contains "cover"; else the first image.
func extractCover(files map[string]*zip.File, pkg *opf, opfDir string, hrefByID map[string]string) []byte {
	// EPUB3 cover-image property.
	for _, it := range pkg.Manifest.Items {
		if strings.Contains(it.Properties, "cover-image") {
			if b, err := readNamed(files, resolve(opfDir, it.Href)); err == nil {
				return b
			}
		}
	}
	// EPUB2 meta name=cover → manifest id.
	for _, m := range pkg.Metadata.Metas {
		if strings.EqualFold(m.Name, "cover") && m.Content != "" {
			if name := hrefByID[m.Content]; name != "" {
				if b, err := readNamed(files, name); err == nil {
					return b
				}
			}
		}
	}
	// Image whose manifest name contains "cover", else first image.
	var firstImage string
	for _, it := range pkg.Manifest.Items {
		if !strings.HasPrefix(it.MediaType, "image/") {
			continue
		}
		name := resolve(opfDir, it.Href)
		if firstImage == "" {
			firstImage = name
		}
		if strings.Contains(strings.ToLower(it.Href), "cover") {
			if b, err := readNamed(files, name); err == nil {
				return b
			}
		}
	}
	if firstImage != "" {
		if b, err := readNamed(files, firstImage); err == nil {
			return b
		}
	}
	return nil
}

// resolve turns a manifest href (relative to the OPF dir, possibly
// percent-encoded) into a cleaned zip entry name.
func resolve(opfDir, href string) string {
	if h, err := url.PathUnescape(href); err == nil {
		href = h
	}
	if opfDir == "." || opfDir == "" {
		return path.Clean(href)
	}
	return path.Clean(opfDir + "/" + href)
}

func readNamed(files map[string]*zip.File, name string) ([]byte, error) {
	f := files[name]
	if f == nil {
		return nil, errMissing(name)
	}
	return readAll(f)
}

func readAll(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func firstNonEmpty(ss []string) string {
	for _, s := range ss {
		if c := segment.CleanText(s); c != "" {
			return c
		}
	}
	return ""
}

type errMissing string

func (e errMissing) Error() string { return "epub: missing entry: " + string(e) }
