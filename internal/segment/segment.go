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

// ParsedParagraph is one parsed paragraph: cleaned text, a role (one of the
// tbook.Role* constants), and inline-emphasis spans in Text's rune coordinates.
type ParsedParagraph struct {
	Text  string
	Role  string
	Spans []tbook.Span
}

// ParsedChapter is the raw output of EPUB parsing: a title and paragraphs.
type ParsedChapter struct {
	Title      string
	Paragraphs []ParsedParagraph
}

// CleanText collapses all (Unicode) whitespace runs — including non-breaking
// spaces — to single spaces and trims.
func CleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
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

// CleanWithSpans cleans raw text and remaps its emphasis spans (in raw rune
// coordinates) into the cleaned text's coordinates. Used by the EPUB parser.
func CleanWithSpans(raw string, spans []tbook.Span) (string, []tbook.Span) {
	clean, m := CleanTextWithMap(raw)
	return clean, remapSpans(spans, m, utf8.RuneCountInString(clean))
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
	for _, piece := range seg.Segment(norm) {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
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
	if len(sents) == 0 {
		return []string{norm}, [][2]int{{0, b2r[len(norm)]}}
	}
	return sents, ranges
}

// sentResult is one segmented sentence plus its emphasis spans, rebased to the
// sentence's own rune coordinates.
type sentResult struct {
	Src   string
	Spans []tbook.Span
}

// segmentParagraph segments a paragraph and distributes its emphasis spans (in
// text's rune coordinates) onto the resulting sentences. Spans follow the same
// whitespace + sentence-fix normalization as the text, then are clipped to each
// sentence and rebased to sentence-local offsets.
func segmentParagraph(text string, spans []tbook.Span, seg sentencizer.Segmenter) []sentResult {
	clean, cm := CleanTextWithMap(text)
	cspans := remapSpans(spans, cm, utf8.RuneCountInString(clean))

	norm := normalizePara(clean) // sentFix only — clean is already collapsed/trimmed
	nm := mapByInsertion(clean, norm)
	nspans := remapSpans(cspans, nm, utf8.RuneCountInString(norm))

	sents, ranges := segmentNorm(norm, seg)
	out := make([]sentResult, 0, len(sents))
	for i, src := range sents {
		rg := ranges[i]
		var sps []tbook.Span
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
		}
		out = append(out, sentResult{Src: src, Spans: sps})
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

// BuildSentenceObjects converts parsed chapters into the nested sentence-object
// structure. Returns the chapters (whose sentences are pointers) and a flat
// slice of every sentence, so translation can fill them in place. Per-paragraph
// roles are carried into Chapter.ParagraphStyles (index-aligned with the kept
// paragraphs); scene breaks are kept as empty paragraphs that carry their role.
func BuildSentenceObjects(chapters []ParsedChapter) ([]tbook.Chapter, []*tbook.Sentence) {
	seg := sentencizer.NewSegmenter("en")
	out := make([]tbook.Chapter, 0, len(chapters))
	var all []*tbook.Sentence

	for _, ch := range chapters {
		var paras [][]*tbook.Sentence
		var styles []string
		for _, para := range ch.Paragraphs {
			if para.Role == tbook.RoleSceneBreak {
				paras = append(paras, []*tbook.Sentence{})
				styles = append(styles, tbook.RoleSceneBreak)
				continue
			}
			var sentObjs []*tbook.Sentence
			for _, r := range segmentParagraph(para.Text, para.Spans, seg) {
				obj := &tbook.Sentence{
					Src:   r.Src,
					Words: Tokenize(r.Src),
					Tr:    map[string]tbook.Translation{},
					Spans: r.Spans,
				}
				sentObjs = append(sentObjs, obj)
				all = append(all, obj)
			}
			if len(sentObjs) > 0 {
				paras = append(paras, sentObjs)
				styles = append(styles, para.Role)
			}
		}
		out = append(out, tbook.Chapter{Title: ch.Title, Paragraphs: paras, ParagraphStyles: styles})
	}
	return out, all
}
