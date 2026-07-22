// Package segment splits parsed paragraphs into sentences and tokenizes source
// words, producing the .tbook sentence objects. Faithful port of tbook.py's
// clean_text / split_sentences / tokenize / build_sentence_objects.
//
// Sentence boundaries come from the pure-Go sentencizer library (pysbd-inspired;
// minor divergence from Python's pysbd is expected). Word offsets are emitted as
// CODE-POINT (rune) indices.
package segment

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/sentencizer/sentencizer"
)

var (
	// A word = a letter run, allowing intra-word apostrophes/hyphens.
	wordRE = regexp.MustCompile(`\p{L}[\p{L}\p{M}]*(?:['’\-]\p{L}[\p{L}\p{M}]*)*`)
	// Reinsert a space when sentence-final punctuation (+ optional closer) is
	// glued directly to the capital letter starting the next sentence.
	sentFixRE = regexp.MustCompile(`([.!?…]{1,3}["»”’')\]]?)(\p{Lu})`)
)

// Mark is an inline footnote marker at rune offset Pos of the paragraph text
// (an insertion point — the marker's label is NOT part of the text). ID is the
// book-level note id it references; Label its display text ("1", "*", …).
type Mark struct {
	Pos   int
	ID    string
	Label string
}

// ParsedFigure is an image attached to a figure paragraph (whose Text is the
// caption). Image is the OUTPUT archive entry name (e.g. "images/img3.jpg").
type ParsedFigure struct {
	Image string
	Alt   string
}

// ParsedCell is one table cell: a mini-paragraph of text with emphasis spans
// and footnote marks, optionally a header cell.
type ParsedCell struct {
	Text   string
	Spans  []tbook.Span
	Marks  []Mark
	Header bool
}

// ParsedTable is a table attached to a table paragraph (which carries no text).
type ParsedTable struct {
	Rows [][]ParsedCell
}

// ParsedParagraph is one parsed paragraph: cleaned text, a role (one of the
// tbook.Role* constants), inline-emphasis spans and footnote marks in Text's
// rune coordinates, plus the figure/table payload for those roles.
type ParsedParagraph struct {
	Text   string
	Role   string
	Spans  []tbook.Span
	Marks  []Mark
	Figure *ParsedFigure // role RoleFigure only
	Table  *ParsedTable  // role RoleTable only
}

// ParsedChapter is the raw output of EPUB parsing: a title and paragraphs.
type ParsedChapter struct {
	Title      string
	Paragraphs []ParsedParagraph
}

// ParsedNote is one footnote/endnote body parsed from the EPUB, before
// segmentation. Kind is tbook.NoteKindNote or tbook.NoteKindCitation.
type ParsedNote struct {
	ID         string
	Label      string
	Kind       string
	Paragraphs []ParsedParagraph
}

// CleanText collapses all (Unicode) whitespace runs — including non-breaking
// spaces — to single spaces and trims.
func CleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// InsertTitleHeading prepends the chapter's title as a heading-role paragraph
// so it is translated, aligned, and tappable like any prose. Consumers can
// render this heading directly instead of synthesizing an untranslated one
// from ChapterRef.title (which remains the TOC/navigation title). Skipped for
// blank titles and when an equal heading already leads the chapter.
func InsertTitleHeading(ch *ParsedChapter) {
	title := CleanText(ch.Title)
	if title == "" {
		return
	}
	if len(ch.Paragraphs) > 0 {
		p0 := ch.Paragraphs[0]
		if p0.Role == tbook.RoleHeading && strings.EqualFold(CleanText(p0.Text), title) {
			return
		}
	}
	ch.Paragraphs = append(
		[]ParsedParagraph{{Text: title, Role: tbook.RoleHeading}},
		ch.Paragraphs...,
	)
}

// CleanTextWithMap is CleanText plus a rune-index map: ret[i] is the rune index
// in the cleaned string for rune i of s (len(ret) == runeCount(s)+1, with the
// final entry == len(clean)). Whitespace runes map to the next kept rune. It lets
// inline-span offsets follow the same whitespace normalization the text does.
func CleanTextWithMap(s string) (string, []int) {
	rs := []rune(s)
	m := make([]int, len(rs)+1)
	clean := make([]rune, 0, len(rs))
	pendingSpace := false
	for i, r := range rs {
		if unicode.IsSpace(r) {
			m[i] = len(clean)
			if len(clean) > 0 {
				pendingSpace = true
			}
			continue
		}
		if pendingSpace {
			clean = append(clean, ' ')
			pendingSpace = false
		}
		m[i] = len(clean)
		clean = append(clean, r)
	}
	m[len(rs)] = len(clean)
	return string(clean), m
}

// CleanWithSpans cleans raw text and remaps its emphasis spans and footnote
// marks (in raw rune coordinates) into the cleaned text's coordinates. Used by
// the EPUB parser.
func CleanWithSpans(raw string, spans []tbook.Span, marks []Mark) (string, []tbook.Span, []Mark) {
	clean, m := CleanTextWithMap(raw)
	n := utf8.RuneCountInString(clean)
	return clean, remapSpans(spans, m, n), remapMarks(marks, m, n)
}

// remapMarks translates each mark's insertion point through rune-index map m,
// clamped to [0, dstLen].
func remapMarks(marks []Mark, m []int, dstLen int) []Mark {
	if len(marks) == 0 {
		return nil
	}
	out := make([]Mark, 0, len(marks))
	for _, mk := range marks {
		p := mapIdx(m, mk.Pos)
		if p > dstLen {
			p = dstLen
		}
		out = append(out, Mark{Pos: p, ID: mk.ID, Label: mk.Label})
	}
	return out
}

// SplitSentences segments one paragraph into sentence strings.
func SplitSentences(paragraph string, seg sentencizer.Segmenter) []string {
	sents, _ := segmentNorm(normalizePara(paragraph), seg)
	return sents
}

// normalizePara applies the producer's sentence-boundary normalization:
// whitespace collapse, a space reinserted where sentence-final punctuation is
// glued to the next capital, then a final collapse.
func normalizePara(p string) string {
	return CleanText(sentFixRE.ReplaceAllString(CleanText(p), "${1} ${2}"))
}

// segmentNorm splits an already-normalized paragraph into sentences, returning
// each sentence's text and its [start,end) rune range within norm. A range of
// {-1,-1} marks a piece that wasn't a verbatim substring (no offsets available).
func segmentNorm(norm string, seg sentencizer.Segmenter) (sents []string, ranges [][2]int) {
	if norm == "" {
		return nil, nil
	}
	b2r := byteToRune(norm)
	cur := 0
	for _, raw := range seg.Segment(norm) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		for _, piece := range splitQuoted(raw, seg) {
			idx := strings.Index(norm[cur:], piece)
			if idx >= 0 {
				idx += cur
			} else if idx = strings.Index(norm, piece); idx < 0 {
				sents = append(sents, piece)
				ranges = append(ranges, [2]int{-1, -1})
				continue
			}
			end := idx + len(piece)
			cur = end
			sents = append(sents, norm[idx:end])
			ranges = append(ranges, [2]int{b2r[idx], b2r[end]})
		}
	}
	if len(sents) == 0 {
		return []string{norm}, [][2]int{{0, b2r[len(norm)]}}
	}
	return sents, ranges
}

// quotedRegionREs are the quote pairs whose interiors the sentencizer library
// masks before boundary detection (its between-punctuation pass), so a
// multi-sentence quotation comes back as ONE piece. Straight single quotes are
// left out: they double as apostrophes, and the library's own masking of them
// is guarded/rare, so there is usually no join to undo.
var quotedRegionREs = []*regexp.Regexp{
	regexp.MustCompile(`“[^”]*”`),
	regexp.MustCompile(`«[^»]*»`),
	regexp.MustCompile(`"[^"]*"`),
	regexp.MustCompile(`‘(?:[^’]|’\p{L})*’`), // ’ before a letter = apostrophe, not a closer
}

var (
	// A quote whose interior ends in sentence-final punctuation terminates its
	// sentence at the closing quote…
	quoteEndsSentenceRE = regexp.MustCompile(`[.!?…]["»”’')\]]?$`)
	// …when followed by the start of a new sentence (mirrors how the
	// segmenter splits after unmasked quotes, e.g. "Wonderful!" | I ejaculated.).
	nextSentStartRE = regexp.MustCompile(`^(\s+)["“«‘']?\p{Lu}`)
)

const quoteChars = `“«"‘`

// splitQuoted re-splits one segmenter piece at the sentence boundaries the
// segmenter suppressed because of quote pairs. The suppression is twofold:
// by design, the between-punctuation masking joins a multi-sentence quotation
// into one piece; and a port bug in the library's sentenceBoundaryPunctuation
// (priorIndex) makes a late quote also swallow boundaries in the plain text
// before it. So both the quote interiors and the fragments outside the quotes
// are re-segmented standalone (where neither effect applies) and the piece is
// cut at every boundary found. The opening quote stays with the quote's first
// sentence, the closing quote (plus any speech tag) with its last. Returned
// pieces are trimmed verbatim substrings of piece, in order; quoteless pieces
// come back as-is.
func splitQuoted(piece string, seg sentencizer.Segmenter) []string {
	cuts := quoteCuts(piece, seg, 3)
	if len(cuts) == 0 {
		return []string{piece}
	}
	sort.Ints(cuts)
	out := make([]string, 0, len(cuts)+1)
	prev := 0
	for _, c := range cuts {
		if c <= prev || c >= len(piece) {
			continue
		}
		if s := strings.TrimSpace(piece[prev:c]); s != "" {
			out = append(out, s)
		}
		prev = c
	}
	if s := strings.TrimSpace(piece[prev:]); s != "" {
		out = append(out, s)
	}
	if len(out) == 0 {
		return []string{piece}
	}
	return out
}

// quoteCuts returns the byte offsets within piece where it must be cut. depth
// bounds recursion into nested quotes.
func quoteCuts(piece string, seg sentencizer.Segmenter, depth int) []int {
	if depth == 0 || !strings.ContainsAny(piece, quoteChars) {
		return nil
	}
	regions := quoteRegions(piece)
	if len(regions) == 0 {
		return nil
	}
	var cuts []int
	pos := 0
	for _, r := range regions {
		outside := piece[pos:r[0]]
		cuts = append(cuts, fragmentCuts(outside, pos, seg)...)
		// A completed sentence before the quote: the quote starts a new one.
		if endsSentence(strings.TrimSpace(outside), seg) {
			cuts = append(cuts, r[0])
		}
		_, osz := utf8.DecodeRuneInString(piece[r[0]:])
		_, csz := utf8.DecodeLastRuneInString(piece[:r[1]])
		is, ie := r[0]+osz, r[1]-csz
		inner := piece[is:ie]
		cuts = append(cuts, fragmentCuts(inner, is, seg)...)
		for _, c := range quoteCuts(inner, seg, depth-1) {
			cuts = append(cuts, is+c)
		}
		// The boundary after the closing quote: cut when the quote ends its
		// sentence and a new one follows.
		if endsSentence(inner, seg) {
			if m := nextSentStartRE.FindStringSubmatchIndex(piece[r[1]:]); m != nil {
				cuts = append(cuts, r[1]+m[3]) // after the whitespace run
			}
		}
		pos = r[1]
	}
	cuts = append(cuts, fragmentCuts(piece[pos:], pos, seg)...)
	return cuts
}

// quoteRegions returns the non-overlapping quote-pair regions of piece as
// [start,end) byte ranges, in order. On overlap the earlier/longer region
// wins; regions nested inside it are found by quoteCuts' recursion instead.
func quoteRegions(piece string) [][2]int {
	var all [][2]int
	for _, re := range quotedRegionREs {
		for _, loc := range re.FindAllStringIndex(piece, -1) {
			all = append(all, [2]int{loc[0], loc[1]})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i][0] != all[j][0] {
			return all[i][0] < all[j][0]
		}
		return all[i][1] > all[j][1]
	})
	out := all[:0]
	end := 0
	for _, r := range all {
		if r[0] >= end {
			out = append(out, r)
			end = r[1]
		}
	}
	return out
}

// fragmentCuts re-segments one quote-free fragment of a piece standalone and
// returns the absolute (base-offset) byte positions where each sentence after
// the first starts. Non-verbatim segmenter output yields no reliable offset
// and is skipped (conservative: fewer cuts).
func fragmentCuts(fragment string, base int, seg sentencizer.Segmenter) []int {
	if strings.TrimSpace(fragment) == "" {
		return nil
	}
	var cuts []int
	cur := 0
	first := true
	for _, s := range seg.Segment(fragment) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		idx := strings.Index(fragment[cur:], s)
		if idx < 0 {
			continue
		}
		idx += cur
		cur = idx + len(s)
		if !first {
			cuts = append(cuts, base+idx)
		}
		first = false
	}
	return cuts
}

// endsSentence reports whether text ends a complete sentence, per the
// segmenter itself (so abbreviations like "Mr." don't count): appending a
// capitalized probe word must yield one more segment than text alone.
func endsSentence(text string, seg sentencizer.Segmenter) bool {
	if text == "" || !quoteEndsSentenceRE.MatchString(text) {
		return false
	}
	return countSegments(text+" Next.", seg) > countSegments(text, seg)
}

func countSegments(text string, seg sentencizer.Segmenter) int {
	n := 0
	for _, s := range seg.Segment(text) {
		if strings.TrimSpace(s) != "" {
			n++
		}
	}
	return n
}

// sentResult is one segmented sentence plus its emphasis spans and footnote
// marks, rebased to the sentence's own rune coordinates.
type sentResult struct {
	Src   string
	Spans []tbook.Span
	Marks []Mark
}

// segmentParagraph segments a paragraph and distributes its emphasis spans and
// footnote marks (in text's rune coordinates) onto the resulting sentences.
// Both follow the same whitespace + sentence-fix normalization as the text,
// then are clipped to each sentence and rebased to sentence-local offsets.
func segmentParagraph(text string, spans []tbook.Span, marks []Mark, seg sentencizer.Segmenter) []sentResult {
	clean, cm := CleanTextWithMap(text)
	cleanLen := utf8.RuneCountInString(clean)
	cspans := remapSpans(spans, cm, cleanLen)
	cmarks := remapMarks(marks, cm, cleanLen)

	norm := normalizePara(clean) // sentFix only — clean is already collapsed/trimmed
	nm := mapByInsertion(clean, norm)
	normLen := utf8.RuneCountInString(norm)
	nspans := remapSpans(cspans, nm, normLen)
	nmarks := remapMarks(cmarks, nm, normLen)

	sents, ranges := segmentNorm(norm, seg)
	out := make([]sentResult, 0, len(sents))
	claimed := make([]bool, len(nmarks))
	for i, src := range sents {
		rg := ranges[i]
		var sps []tbook.Span
		var mks []Mark
		if rg[0] >= 0 {
			for _, sp := range nspans {
				a, b := sp.S, sp.E
				if a < rg[0] {
					a = rg[0]
				}
				if b > rg[1] {
					b = rg[1]
				}
				if a < b {
					sps = append(sps, tbook.Span{S: a - rg[0], E: b - rg[0], K: sp.K})
				}
			}
			// A mark belongs to the first sentence whose end reaches it (markers
			// typically sit right after a sentence's final punctuation, at rg[1]).
			// The last sentence takes any leftovers.
			srcLen := rg[1] - rg[0]
			for j, mk := range nmarks {
				if claimed[j] {
					continue
				}
				if mk.Pos <= rg[1] || i == len(sents)-1 {
					claimed[j] = true
					p := mk.Pos - rg[0]
					if p < 0 {
						p = 0
					}
					if p > srcLen {
						p = srcLen
					}
					mks = append(mks, Mark{Pos: p, ID: mk.ID, Label: mk.Label})
				}
			}
		}
		out = append(out, sentResult{Src: src, Spans: sps, Marks: mks})
	}
	return out
}

// mapByInsertion aligns two strings that differ only by runes inserted into b,
// returning ret where ret[i] is the rune index in b of rune i of a
// (len == runeCount(a)+1, final entry == runeCount(b)).
func mapByInsertion(a, b string) []int {
	ar, br := []rune(a), []rune(b)
	m := make([]int, len(ar)+1)
	j := 0
	for i := range ar {
		for j < len(br) && br[j] != ar[i] {
			j++
		}
		m[i] = j
		if j < len(br) {
			j++
		}
	}
	m[len(ar)] = len(br)
	return m
}

// remapSpans translates each span's [S,E) through rune-index map m, clamps to
// dstLen, and drops anything that collapses to empty.
func remapSpans(spans []tbook.Span, m []int, dstLen int) []tbook.Span {
	if len(spans) == 0 {
		return nil
	}
	out := make([]tbook.Span, 0, len(spans))
	for _, sp := range spans {
		s, e := mapIdx(m, sp.S), mapIdx(m, sp.E)
		if e > dstLen {
			e = dstLen
		}
		if s < e {
			out = append(out, tbook.Span{S: s, E: e, K: sp.K})
		}
	}
	return out
}

func mapIdx(m []int, i int) int {
	if i < 0 {
		return 0
	}
	if i >= len(m) {
		return m[len(m)-1]
	}
	return m[i]
}

// Tokenize returns the [start,end) rune offsets of each tappable word in src.
func Tokenize(src string) [][2]int {
	locs := wordRE.FindAllStringIndex(src, -1)
	out := make([][2]int, 0, len(locs))
	if len(locs) == 0 {
		return out
	}
	b2r := byteToRune(src)
	for _, loc := range locs {
		out = append(out, [2]int{b2r[loc[0]], b2r[loc[1]]})
	}
	return out
}

// byteToRune maps every byte offset in s (0..len) to its rune index. Word match
// offsets always fall on rune boundaries, so the mapping is exact there.
func byteToRune(s string) []int {
	m := make([]int, len(s)+1)
	ri, bi := 0, 0
	for bi < len(s) {
		_, size := utf8.DecodeRuneInString(s[bi:])
		for k := range size {
			m[bi+k] = ri
		}
		bi += size
		ri++
	}
	m[len(s)] = ri
	return m
}

// segmenterLangs are the language codes the sentencizer library implements;
// anything else falls back to English rules (sentence-final punctuation is
// near-universal across European languages, so this stays correct for e.g.
// Spanish — only language-specific abbreviation lists are missed).
var segmenterLangs = map[string]bool{
	"en": true, "ru": true, "ja": true, "zh": true, "he": true, "lt": true,
}

// NewSegmenter returns a sentence segmenter for the language, falling back to
// English rules for languages the library does not implement.
func NewSegmenter(lang string) sentencizer.Segmenter {
	if !segmenterLangs[lang] {
		lang = "en"
	}
	return sentencizer.NewSegmenter(lang)
}

// buildSentences segments one paragraph's text into final sentence objects,
// appending each to *all for in-place translation.
func buildSentences(text string, spans []tbook.Span, marks []Mark, seg sentencizer.Segmenter, all *[]*tbook.Sentence) []*tbook.Sentence {
	var sentObjs []*tbook.Sentence
	for _, r := range segmentParagraph(text, spans, marks, seg) {
		obj := &tbook.Sentence{
			Src:   r.Src,
			Words: Tokenize(r.Src),
			Tr:    map[string]tbook.Translation{},
			Spans: r.Spans,
			Notes: marksToRefs(r.Marks),
		}
		sentObjs = append(sentObjs, obj)
		*all = append(*all, obj)
	}
	return sentObjs
}

func marksToRefs(marks []Mark) []tbook.NoteRef {
	if len(marks) == 0 {
		return nil
	}
	out := make([]tbook.NoteRef, 0, len(marks))
	for _, m := range marks {
		out = append(out, tbook.NoteRef{P: m.Pos, ID: m.ID, Label: m.Label})
	}
	return out
}

// BuildSentenceObjects converts parsed chapters into the nested sentence-object
// structure. Returns the chapters (whose sentences are pointers) and a flat
// slice of every sentence — body prose, figure captions, and table cells — so
// translation can fill them in place. Per-paragraph roles are carried into
// Chapter.ParagraphStyles (index-aligned with the kept paragraphs); scene
// breaks, figures, and tables are kept even when they carry no sentences.
// sourceLang selects the sentence-splitting rules.
func BuildSentenceObjects(chapters []ParsedChapter, sourceLang string) ([]tbook.Chapter, []*tbook.Sentence) {
	seg := NewSegmenter(sourceLang)
	out := make([]tbook.Chapter, 0, len(chapters))
	var all []*tbook.Sentence

	for _, ch := range chapters {
		var paras [][]*tbook.Sentence
		var styles []string
		var figures []tbook.Figure
		var tables []tbook.Table
		for _, para := range ch.Paragraphs {
			switch para.Role {
			case tbook.RoleSceneBreak:
				paras = append(paras, []*tbook.Sentence{})
				styles = append(styles, tbook.RoleSceneBreak)
			case tbook.RoleFigure:
				if para.Figure == nil {
					continue
				}
				caption := buildSentences(para.Text, para.Spans, para.Marks, seg, &all)
				if caption == nil {
					caption = []*tbook.Sentence{}
				}
				paras = append(paras, caption)
				styles = append(styles, tbook.RoleFigure)
				figures = append(figures, tbook.Figure{
					Para: len(paras) - 1, Image: para.Figure.Image, Alt: para.Figure.Alt,
				})
			case tbook.RoleTable:
				if para.Table == nil || len(para.Table.Rows) == 0 {
					continue
				}
				rows := make([][]tbook.TableCell, 0, len(para.Table.Rows))
				for _, prow := range para.Table.Rows {
					row := make([]tbook.TableCell, 0, len(prow))
					for _, pc := range prow {
						cellSents := buildSentences(pc.Text, pc.Spans, pc.Marks, seg, &all)
						if cellSents == nil {
							cellSents = []*tbook.Sentence{}
						}
						row = append(row, tbook.TableCell{Sentences: cellSents, Header: pc.Header})
					}
					rows = append(rows, row)
				}
				paras = append(paras, []*tbook.Sentence{})
				styles = append(styles, tbook.RoleTable)
				tables = append(tables, tbook.Table{Para: len(paras) - 1, Rows: rows})
			default:
				sentObjs := buildSentences(para.Text, para.Spans, para.Marks, seg, &all)
				if len(sentObjs) > 0 {
					paras = append(paras, sentObjs)
					styles = append(styles, para.Role)
				}
			}
		}
		out = append(out, tbook.Chapter{
			Title: ch.Title, Paragraphs: paras, ParagraphStyles: styles,
			Figures: figures, Tables: tables,
		})
	}
	return out, all
}

// BuildNotes segments parsed note bodies into final note objects. Returns the
// id → note map plus two flat sentence slices: content-note sentences and
// citation sentences (kept separate so a producer can skip translating
// citations).
func BuildNotes(notes []ParsedNote, sourceLang string) (map[string]*tbook.Note, []*tbook.Sentence, []*tbook.Sentence) {
	if len(notes) == 0 {
		return nil, nil, nil
	}
	seg := NewSegmenter(sourceLang)
	out := make(map[string]*tbook.Note, len(notes))
	var noteSents, citeSents []*tbook.Sentence
	for _, pn := range notes {
		var all []*tbook.Sentence
		var paras [][]*tbook.Sentence
		for _, para := range pn.Paragraphs {
			sentObjs := buildSentences(para.Text, para.Spans, para.Marks, seg, &all)
			if len(sentObjs) > 0 {
				paras = append(paras, sentObjs)
			}
		}
		if paras == nil {
			paras = [][]*tbook.Sentence{}
		}
		out[pn.ID] = &tbook.Note{Label: pn.Label, Kind: pn.Kind, Paragraphs: paras}
		if pn.Kind == tbook.NoteKindCitation {
			citeSents = append(citeSents, all...)
		} else {
			noteSents = append(noteSents, all...)
		}
	}
	return out, noteSents, citeSents
}
