// Package epub parses an EPUB into chapters of raw paragraph text, in spine
// (reading) order, plus book-level figures, tables, and footnotes.
//
// Chapter boundaries come from the EPUB navigation (NCX, all levels flattened)
// when present, else from <div class="title1">/<div class="chapter"> heading
// divs (the style Project Gutenberg's ebookmaker emits). Front/back matter
// (cover, synopsis, credits, index, the notes file) is skipped by default.
// Inline footnote markers (<sup><a href="…#id">N</a></sup> and friends) are
// stripped from prose and recorded as positional marks; their bodies are
// resolved from the target documents afterwards. Body images become figure
// paragraphs (caption = adjacent caption paragraph), tables become table
// paragraphs, <hr> becomes a scene break. The cover image bytes are extracted
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
	Language string // dc:language metadata as declared (e.g. "en", "en-US"); "" if absent
	Cover    []byte // nil if none
	Chapters []segment.ParsedChapter
	Images   map[string][]byte    // output entry name ("images/imgN.ext") → bytes
	Notes    []segment.ParsedNote // in first-reference order
}

// Options tunes parsing.
type Options struct {
	SkipMatter bool     // skip front/back matter (cover, synopsis, credits, index, notes file)
	SkipExtra  []string // extra case-insensitive patterns to skip (matched on chapter title and filename)
	NoImages   bool     // drop body images (no figure paragraphs)
	NoNotes    bool     // drop footnotes entirely (markers are still stripped from prose)
}

type container struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type opf struct {
	Metadata struct {
		Title    []string `xml:"title"`    // dc:title
		Creator  []string `xml:"creator"`  // dc:creator
		Language []string `xml:"language"` // dc:language
		Metas    []struct {
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

// ncx is the EPUB2 navigation document. NavPoints nest; ALL levels are
// flattened into chapter boundaries in document order, so a book organized as
// parts → chapters yields one chapter per NCX entry (parts become their own
// short chapters), not one giant chapter per part.
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
	Children []ncxNavPoint `xml:"navPoint"`
}

type tocEntry struct {
	Title string
	File  string // resolved zip entry name (fragment stripped)
}

// parser carries the walk state shared across spine documents.
type parser struct {
	files map[string]*zip.File
	opts  Options
	book  *Book
	skipRE []*regexp.Regexp // compiled Options.SkipExtra

	// Footnote markers: distinct targets in first-reference order.
	noteIDByKey map[string]string // "file#frag" → note id
	noteTargets []noteTarget
	noteFiles   map[string]bool // files hosting note bodies (skipped as chapters)

	// Body images: source entry → output entry (a failed read caches "").
	imgNameBySrc map[string]string
	imgCount     int
}

// Parse reads the EPUB at path with default options (matter skipping on).
func Parse(epubPath string) (*Book, error) {
	return ParseOpts(epubPath, Options{SkipMatter: true})
}

// ParseOpts reads the EPUB at path and returns its title, author, cover,
// chapters, images, and notes.
func ParseOpts(epubPath string, opts Options) (*Book, error) {
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
		Title:    firstNonEmpty(pkg.Metadata.Title),
		Author:   firstNonEmpty(pkg.Metadata.Creator),
		Language: firstNonEmpty(pkg.Metadata.Language),
		Cover:    extractCover(files, pkg, opfDir, hrefByID),
	}
	if book.Title == "" {
		book.Title = strings.TrimSuffix(path.Base(epubPath), path.Ext(epubPath))
	}
	if book.Author == "" {
		book.Author = "Unknown"
	}

	p := &parser{
		files:        files,
		opts:         opts,
		book:         book,
		noteIDByKey:  map[string]string{},
		noteFiles:    map[string]bool{},
		imgNameBySrc: map[string]string{},
	}
	for _, pat := range opts.SkipExtra {
		if re, err := regexp.Compile("(?i)" + pat); err == nil {
			p.skipRE = append(p.skipRE, re)
		}
	}

	// Prefer the EPUB's own navigation (NCX) for chapter boundaries: each nav
	// entry (any nesting level) is one chapter. Far more reliable than guessing
	// from heading classes, which collapses or over-splits anthologies. Falls
	// back to heading-based splitting (inside parseDoc) when no usable NCX.
	chapterStarts := map[string]string{}
	for _, e := range tocEntries(files, pkg, opfDir) {
		if _, ok := chapterStarts[e.File]; !ok {
			chapterStarts[e.File] = e.Title
		}
	}
	tocMode := len(chapterStarts) >= 2

	// Walk spine docs in order, accumulating chapters. `current` persists across
	// files (a chapter may continue into the next spine document). `skipping`
	// suppresses everything from a skipped chapter start until the next start.
	var current *segment.ParsedChapter
	skipping := false
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
		if tocMode {
			if title, ok := chapterStarts[name]; ok {
				if current != nil {
					book.Chapters = append(book.Chapters, *current)
					current = nil
				}
				if p.skipDoc(name, title) {
					skipping = true
					continue
				}
				skipping = false
				current = &segment.ParsedChapter{Title: title}
			} else if skipping || p.skipDoc(name, "") {
				continue
			}
		} else if p.skipDoc(name, "") {
			continue
		}
		content, err := readAll(f)
		if err != nil {
			continue
		}
		current = p.parseDoc(name, content, current, &book.Chapters, tocMode)
	}
	if current != nil {
		book.Chapters = append(book.Chapters, *current)
	}

	// Resolve footnote bodies from their target documents; drop markers whose
	// body cannot be found. With NoNotes, markers are already suppressed.
	if !opts.NoNotes {
		p.resolveNotes()
	}

	for i := range book.Chapters {
		if tocMode {
			dedupTitleHeadings(&book.Chapters[i])
		}
		cleanupSceneBreaks(&book.Chapters[i])
	}
	// Drop bare title-page chapters (no prose) that would render blank.
	book.Chapters = dropEmptyChapters(book.Chapters)
	return book, nil
}

// skipDoc reports whether a spine document (with optional NCX title) is
// front/back matter or a notes file and should not become chapter prose.
func (p *parser) skipDoc(name, title string) bool {
	if p.noteFiles[name] {
		// A file already known to host note bodies referenced from earlier
		// chapters (endnotes files come after their references in the spine).
		return true
	}
	base := strings.TrimSuffix(path.Base(name), path.Ext(path.Base(name)))
	for _, re := range p.skipRE {
		if re.MatchString(title) || re.MatchString(base) {
			return true
		}
	}
	if !p.opts.SkipMatter {
		return false
	}
	return isMatter(title) || isMatter(strings.NewReplacer("_", " ", "-", " ").Replace(base))
}

// Matter patterns are matched on diacritic-folded, lowercased text. Exact
// patterns must match the whole title/filename; loose ones anywhere.
var (
	matterExactRE = regexp.MustCompile(`^\s*(indice|index|toc|contents?|table of contents|tabla de contenidos?|contenidos?|notas|notes|endnotes|footnotes)\s*$`)
	matterLooseRE = regexp.MustCompile(`\b(portada|cover|cubierta|sinopsis|synopsis|creditos|copyright|colophon|titlepage|title page|half.?title|ncl.?indice)\b`)
)

func isMatter(s string) bool {
	f := foldLatin(s)
	if f == "" {
		return false
	}
	return matterExactRE.MatchString(f) || matterLooseRE.MatchString(f)
}

// foldLatin lowercases and strips common Latin diacritics — enough for
// matter-pattern and title matching without a Unicode-normalization dependency.
var latinFold = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ä': 'a', 'ã': 'a', 'å': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'ö': 'o', 'õ': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ñ': 'n', 'ç': 'c', 'ý': 'y',
}

func foldLatin(s string) string {
	s = strings.ToLower(s)
	return strings.Map(func(r rune) rune {
		if f, ok := latinFold[r]; ok {
			return f
		}
		return r
	}, s)
}

// dedupTitleHeadings drops a chapter's leading heading paragraphs when they
// merely repeat (parts of) the NCX title — e.g. "CAPÍTULO" / "1" / "El
// sorprendente poder…" under an NCX title "Capítulo 1. El sorprendente
// poder…" — so the title is not rendered twice.
func dedupTitleHeadings(ch *segment.ParsedChapter) {
	ft := foldLatin(ch.Title)
	if ft == "" {
		return
	}
	for len(ch.Paragraphs) > 0 {
		p0 := ch.Paragraphs[0]
		if p0.Role != tbook.RoleHeading {
			break
		}
		t := strings.Trim(foldLatin(p0.Text), " .:;,")
		if t == "" || !strings.Contains(ft, t) {
			break
		}
		ch.Paragraphs = ch.Paragraphs[1:]
	}
}

// cleanupSceneBreaks drops scene breaks before the first content paragraph and
// after the last, and collapses runs into one.
func cleanupSceneBreaks(ch *segment.ParsedChapter) {
	var out []segment.ParsedParagraph
	pending := false
	seen := false
	for _, para := range ch.Paragraphs {
		if para.Role == tbook.RoleSceneBreak {
			if seen {
				pending = true
			}
			continue
		}
		isContent := para.Text != "" || para.Role == tbook.RoleFigure || para.Role == tbook.RoleTable
		if isContent && pending {
			out = append(out, segment.ParsedParagraph{Role: tbook.RoleSceneBreak})
		}
		if isContent {
			pending = false
			seen = true
		}
		out = append(out, para)
	}
	ch.Paragraphs = out
}

// parseDoc walks one XHTML document's block elements in document order, updating
// the current chapter and appending completed chapters. Returns the (possibly
// new) current chapter. Block prose carries its role (heading/subtitle/scene
// break/body) plus inline italic/bold spans and footnote marks; body images
// become figure paragraphs and tables become table paragraphs.
func (p *parser) parseDoc(docName string, content []byte, current *segment.ParsedChapter, out *[]segment.ParsedChapter, tocMode bool) *segment.ParsedChapter {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(fixSelfClosing(string(content))))
	if err != nil {
		return current
	}
	consumed := map[*html.Node]bool{}
	// titlePending is true from a <div class="chapter"> boundary until the
	// chapter's first body paragraph: leading headings inside the wrapper form
	// the title and are not repeated as body (mirrors the div.title1 path).
	titlePending := false
	doc.Find("body").Find("div, p, h1, h2, h3, h4, h5, h6, img, table, hr").Each(func(_ int, s *goquery.Selection) {
		node := s.Nodes[0]
		if consumed[node] {
			return
		}
		// Anything inside a table is handled by the table extraction (and a
		// nested table belongs to its outer cell's text).
		if s.ParentsFiltered("table").Length() > 0 {
			return
		}
		switch goquery.NodeName(s) {
		case "div":
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
		case "img":
			if p.opts.NoImages || current == nil || inSpecialDiv(s) || inBoilerplate(s) {
				return
			}
			if fig := p.handleImg(s, docName, consumed); fig != nil {
				titlePending = false
				current.Paragraphs = append(current.Paragraphs, *fig)
			}
			return
		case "table":
			if current == nil || inSpecialDiv(s) || inBoilerplate(s) {
				return
			}
			if tbl := p.handleTable(s, docName); tbl != nil {
				titlePending = false
				current.Paragraphs = append(current.Paragraphs, *tbl)
			}
			return
		case "hr":
			// A mid-chapter divider becomes a scene break; decorative rules in
			// the chapter-title block (before any body prose) are dropped.
			// cleanupSceneBreaks later collapses runs and trims edges.
			if current != nil && chapterHasBody(*current) {
				current.Paragraphs = append(current.Paragraphs, segment.ParsedParagraph{Role: tbook.RoleSceneBreak})
			}
			return
		}
		// <p>/<hN> prose.
		if inSpecialDiv(s) || inBoilerplate(s) || current == nil {
			return
		}
		if isFootnotePara(s) {
			return // note bodies are extracted separately via their anchors
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
		text, spans, marks := p.blockText(s, docName)
		if text == "" && role != tbook.RoleSceneBreak {
			return
		}
		current.Paragraphs = append(current.Paragraphs, segment.ParsedParagraph{
			Text:  text,
			Role:  role,
			Spans: spans,
			Marks: marks,
		})
	})
	return current
}

// blockText extracts a block element's cleaned text, emphasis spans, and
// footnote marks (marker anchors are stripped from the text and their targets
// registered for body resolution).
func (p *parser) blockText(s *goquery.Selection, docName string) (string, []tbook.Span, []segment.Mark) {
	raw, spans, refs := richText(s, richOpts{noterefs: true})
	var marks []segment.Mark
	if !p.opts.NoNotes {
		for _, r := range refs {
			id := p.registerNoteRef(r, docName)
			if id == "" {
				continue
			}
			marks = append(marks, segment.Mark{Pos: r.Pos, ID: id, Label: r.Label})
		}
	}
	return segment.CleanWithSpans(raw, spans, marks)
}

// registerNoteRef resolves a raw marker's href against the current document and
// assigns (or reuses) its note id. Returns "" for an unusable target.
func (p *parser) registerNoteRef(r rawMark, docName string) string {
	href := strings.TrimSpace(r.Href)
	i := strings.IndexByte(href, '#')
	if i < 0 || i == len(href)-1 {
		return ""
	}
	frag := href[i+1:]
	file := docName
	if fp := href[:i]; fp != "" {
		if u, err := url.PathUnescape(fp); err == nil {
			fp = u
		}
		file = path.Clean(path.Dir(docName) + "/" + fp)
	}
	key := file + "#" + frag
	if id, ok := p.noteIDByKey[key]; ok {
		return id
	}
	id := "n" + itoa(len(p.noteTargets)+1)
	p.noteIDByKey[key] = id
	p.noteTargets = append(p.noteTargets, noteTarget{id: id, file: file, frag: frag, label: r.Label})
	p.noteFiles[file] = true
	return id
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// isFootnotePara reports whether a paragraph is a footnote body by class
// convention (e.g. <p class="footnote"> in a shared notes document).
func isFootnotePara(s *goquery.Selection) bool {
	return s.HasClass("footnote") || s.HasClass("footnotes") ||
		s.HasClass("endnote") || s.HasClass("endnotes")
}

// captionClassRE marks caption paragraphs attached to images.
var captionClassRE = regexp.MustCompile(`(?i)(imagefoot|caption|figcaption|leyenda|pie)`)

func isCaptionPara(s *goquery.Selection) bool {
	cls, _ := s.Attr("class")
	return captionClassRE.MatchString(cls)
}

// handleImg turns a body <img> into a figure paragraph whose text is the
// figure's caption (from <figcaption>, a caption-classed sibling paragraph, or
// empty). The caption element is consumed so the main walk skips it.
func (p *parser) handleImg(s *goquery.Selection, docName string, consumed map[*html.Node]bool) *segment.ParsedParagraph {
	src, _ := s.Attr("src")
	src = strings.TrimSpace(src)
	if src == "" || strings.HasPrefix(src, "data:") {
		return nil
	}
	if u, err := url.PathUnescape(src); err == nil {
		src = u
	}
	entry := path.Clean(path.Dir(docName) + "/" + src)
	outName := p.registerImage(entry)
	if outName == "" {
		return nil
	}

	// Hosting block: nearest p/div/figure ancestor (or the img itself when bare).
	block := s.ParentsFiltered("p, div, figure").First()

	var caption *goquery.Selection
	if block.Length() > 0 && goquery.NodeName(block) == "figure" {
		if fc := block.Find("figcaption").First(); fc.Length() > 0 {
			caption = fc
		}
	}
	if caption == nil && block.Length() > 0 {
		// A caption-classed paragraph inside the same wrapper (e.g.
		// <div class="image"><img/><p class="imagefoot">…</p></div>) …
		block.Find("p").EachWithBreak(func(_ int, c *goquery.Selection) bool {
			if isCaptionPara(c) {
				caption = c
				return false
			}
			return true
		})
		// … or immediately after it.
		if caption == nil {
			if next := block.Next(); next.Length() > 0 && next.Is("p") && isCaptionPara(next) {
				caption = next
			}
		}
	}

	var text string
	var spans []tbook.Span
	var marks []segment.Mark
	if caption != nil {
		consumed[caption.Nodes[0]] = true
		text, spans, marks = p.blockText(caption, docName)
	}
	alt := strings.TrimSpace(s.AttrOr("alt", ""))
	return &segment.ParsedParagraph{
		Text:   text,
		Role:   tbook.RoleFigure,
		Spans:  spans,
		Marks:  marks,
		Figure: &segment.ParsedFigure{Image: outName, Alt: alt},
	}
}

// headerCellRE marks header cells by class convention (th is always a header).
var headerCellRE = regexp.MustCompile(`(?i)(_cab|head|th\b)`)

// handleTable extracts a <table> into a table paragraph: rows of cells, each
// cell a mini-paragraph with emphasis and footnote marks.
func (p *parser) handleTable(s *goquery.Selection, docName string) *segment.ParsedParagraph {
	var rows [][]segment.ParsedCell
	s.Find("tr").Each(func(_ int, tr *goquery.Selection) {
		var row []segment.ParsedCell
		nonEmpty := false
		tr.Find("td, th").Each(func(_ int, td *goquery.Selection) {
			text, spans, marks := p.blockText(td, docName)
			cls, _ := td.Attr("class")
			header := goquery.NodeName(td) == "th" || headerCellRE.MatchString(cls)
			if text != "" {
				nonEmpty = true
			}
			row = append(row, segment.ParsedCell{Text: text, Spans: spans, Marks: marks, Header: header})
		})
		if nonEmpty {
			rows = append(rows, row)
		}
	})
	if len(rows) == 0 {
		return nil
	}
	return &segment.ParsedParagraph{
		Role:  tbook.RoleTable,
		Table: &segment.ParsedTable{Rows: rows},
	}
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

// tocEntries returns the ordered table-of-contents entries from the NCX
// (EPUB2), flattening ALL nesting levels in document order. Returns nil when no
// usable NCX is found (caller then falls back to heading-based splitting).
func tocEntries(files map[string]*zip.File, pkg *opf, opfDir string) []tocEntry {
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
	var walk func(nps []ncxNavPoint)
	walk = func(nps []ncxNavPoint) {
		for _, np := range nps {
			src := np.Content.Src
			if src != "" {
				if i := strings.IndexByte(src, '#'); i >= 0 {
					src = src[:i]
				}
				out = append(out, tocEntry{
					Title: segment.CleanText(np.Text),
					File:  resolve(ncxDir, src),
				})
			}
			walk(np.Children)
		}
	}
	walk(n.NavMap.NavPoints)
	return out
}

// dropEmptyChapters removes chapters with no content (e.g. a bare title page),
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
		if strings.TrimSpace(p.Text) != "" ||
			p.Role == tbook.RoleFigure || p.Role == tbook.RoleTable {
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

// rawMark is a footnote marker found during rich-text extraction: an insertion
// point in the raw text, the anchor's href (unresolved), and its visible label.
type rawMark struct {
	Pos   int
	Href  string
	Label string
}

// richOpts tunes richText extraction.
type richOpts struct {
	noterefs bool       // detect noteref anchors: strip their text, emit rawMarks
	skipNode *html.Node // a node to omit entirely (a note body's backlink anchor)
}

// richText returns an element's text (identical to goquery's .Text() except for
// stripped noteref markers, so sentence segmentation and the translation cache
// are unaffected by emphasis handling) plus the inline italic/bold spans found
// in it, in raw-text rune coordinates, plus any footnote markers. Nested
// emphasis emits overlapping spans (e.g. bold inside italic → both "i" and
// "b"), which the consumer unions.
func richText(s *goquery.Selection, o richOpts) (string, []tbook.Span, []rawMark) {
	if len(s.Nodes) == 0 {
		return "", nil, nil
	}
	var sb strings.Builder
	var spans []tbook.Span
	var marks []rawMark
	pos := 0
	var walk func(n *html.Node, italic, bold, sup bool)
	walk = func(n *html.Node, italic, bold, sup bool) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c == o.skipNode {
				continue
			}
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
				tag := strings.ToLower(c.Data)
				if o.noterefs && tag == "a" {
					if href, label, ok := noterefInfo(c, sup); ok {
						marks = append(marks, rawMark{Pos: pos, Href: href, Label: label})
						continue // the marker's label is not body text
					}
				}
				it, bd, sp := italic, bold, sup
				switch tag {
				case "em", "i":
					it = true
				case "b", "strong":
					bd = true
				case "sup":
					sp = true
				}
				walk(c, it, bd, sp)
			}
		}
	}
	walk(s.Nodes[0], false, false, false)
	return sb.String(), spans, marks
}

// noterefClassRE marks anchors whose class names them footnote markers.
var noterefClassRE = regexp.MustCompile(`(?i)\b(noteref|footnote)\b`)

// noterefInfo decides whether an <a> is a footnote marker and returns its href
// and visible label. A marker links to a fragment and is superscripted (an
// ancestor or child <sup>) or explicitly typed (epub:type="noteref" /
// noteref-ish class), with a short label ("1", "*", "†", …).
func noterefInfo(a *html.Node, inSup bool) (href, label string, ok bool) {
	href = attrVal(a, "href")
	if href == "" || !strings.Contains(href, "#") {
		return "", "", false
	}
	isRef := inSup || hasChildTag(a, "sup") ||
		strings.Contains(attrVal(a, "epub:type"), "noteref") ||
		noterefClassRE.MatchString(attrVal(a, "class"))
	if !isRef {
		return "", "", false
	}
	label = strings.Join(strings.Fields(textContent(a)), " ")
	if label == "" || utf8.RuneCountInString(label) > 8 {
		return "", "", false
	}
	return href, label, true
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasChildTag(n *html.Node, tag string) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.ToLower(c.Data) == tag {
			return true
		}
		if hasChildTag(c, tag) {
			return true
		}
	}
	return false
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(m *html.Node) {
		for c := m.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				sb.WriteString(c.Data)
			} else {
				walk(c)
			}
		}
	}
	walk(n)
	return sb.String()
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
