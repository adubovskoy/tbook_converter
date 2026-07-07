package tbook

import (
	"unicode"
	"unicode/utf8"
)

// CoverageWarn is the target-word coverage below which a producer should warn:
// a structurally-valid file can still be uselessly aligned (positional drift,
// empty "en"). Informative only — alignment is LM-produced and never blocks.
const CoverageWarn = 0.55

// Report summarizes a validation pass. Beyond structure, it tracks alignment
// COVERAGE — the fraction of words actually mapped — which catches a gross
// failure (empty/absent alignment) that offset checks miss. NOTE: coverage does
// NOT detect a partial positional drift or a wrong-word mapping (those keep
// coverage ~100%); only semantic verification (an LLM judge) catches those. See
// spec §10.4.
type Report struct {
	Sentences    int
	Empty        int // translations with empty text
	OffsetErrors int // out-of-range word/align offsets or indices
	StructErrors int // figure/table/note structure violations

	TgtWords   int // words (letter/digit runs) in the translation texts of aligned sentences
	TgtAligned int // of those, how many lie inside a chunk that highlights ≥1 source word
	SrcTotal   int // source words of aligned sentences
	SrcCovered int // of those, source words highlighted by some target word
	Unaligned  int // translated sentences with no alignment (raw-translation fallback)
}

// OK reports whether the archive is structurally sound and fully translated.
func (r Report) OK() bool { return r.OffsetErrors == 0 && r.StructErrors == 0 && r.Empty == 0 }

// TgtCoverage is the fraction of rendered translation words that highlight a
// source word; SrcCoverage the fraction of source words highlighted.
func (r Report) TgtCoverage() float64 {
	if r.TgtWords == 0 {
		return 1
	}
	return float64(r.TgtAligned) / float64(r.TgtWords)
}

func (r Report) SrcCoverage() float64 {
	if r.SrcTotal == 0 {
		return 1
	}
	return float64(r.SrcCovered) / float64(r.SrcTotal)
}

// wordRuns returns the [start,end) rune ranges of letter/digit runs in rs.
func wordRuns(rs []rune) []struct{ a, b int } {
	var out []struct{ a, b int }
	start := -1
	for i, r := range rs {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			out = append(out, struct{ a, b int }{start, i})
			start = -1
		}
	}
	if start >= 0 {
		out = append(out, struct{ a, b int }{start, len(rs)})
	}
	return out
}


// Validate checks every sentence's word offsets and each translation's align
// spans/indices against the spec invariants, over the given target languages —
// covering chapter prose, figure captions, table cells, and note bodies — plus
// the figure/table/note structural rules.
func Validate(b *Book, targets []string) Report {
	var rep Report
	for _, ch := range b.Chapters {
		for _, para := range ch.Paragraphs {
			for _, s := range para {
				rep.checkSentence(s, targets, b.Notes)
			}
		}
		roleAt := func(i int) string {
			if i < len(ch.ParagraphStyles) && ch.ParagraphStyles[i] != "" {
				return ch.ParagraphStyles[i]
			}
			return RoleBody
		}
		// sceneBreak/table paragraphs carry no sentences.
		for i, para := range ch.Paragraphs {
			if r := roleAt(i); (r == RoleSceneBreak || r == RoleTable) && len(para) > 0 {
				rep.StructErrors++
			}
		}
		for _, fig := range ch.Figures {
			if fig.Para < 0 || fig.Para >= len(ch.Paragraphs) || roleAt(fig.Para) != RoleFigure {
				rep.StructErrors++
			}
			if fig.Image == "" {
				rep.StructErrors++
			} else if _, ok := b.Images[fig.Image]; !ok {
				rep.StructErrors++
			}
		}
		for _, t := range ch.Tables {
			if t.Para < 0 || t.Para >= len(ch.Paragraphs) || roleAt(t.Para) != RoleTable {
				rep.StructErrors++
			}
			if len(t.Rows) == 0 {
				rep.StructErrors++
			}
			for _, row := range t.Rows {
				for _, cell := range row {
					for _, s := range cell.Sentences {
						rep.checkSentence(s, targets, b.Notes)
					}
				}
			}
		}
	}
	for _, n := range b.Notes {
		if n == nil {
			rep.StructErrors++
			continue
		}
		for _, para := range n.Paragraphs {
			for _, s := range para {
				rep.checkSentence(s, targets, b.Notes)
			}
		}
	}
	return rep
}

func (rep *Report) checkSentence(s *Sentence, targets []string, notes map[string]*Note) {
	rep.Sentences++
	srcLen := utf8.RuneCountInString(s.Src)
	for _, w := range s.Words {
		if !(w[0] >= 0 && w[0] < w[1] && w[1] <= srcLen) {
			rep.OffsetErrors++
		}
	}
	// Inline-emphasis spans: half-open rune range into Src, kind i/b.
	for _, sp := range s.Spans {
		if !(sp.S >= 0 && sp.S < sp.E && sp.E <= srcLen) ||
			(sp.K != SpanItalic && sp.K != SpanBold) {
			rep.OffsetErrors++
		}
	}
	// Note markers: insertion point in [0, len(src)], id must resolve.
	for _, nr := range s.Notes {
		if nr.P < 0 || nr.P > srcLen {
			rep.OffsetErrors++
		}
		if _, ok := notes[nr.ID]; !ok {
			rep.StructErrors++
		}
	}
	for _, code := range targets {
		tr, ok := s.Tr[code]
		if !ok || tr.Text == "" {
			rep.Empty++
			continue
		}
		textRunes := []rune(tr.Text)
		textLen := len(textRunes)
		covered := map[int]bool{}
		type span struct{ a, b int }
		var mapped []span
		for _, c := range tr.Align {
			if !(c.T[0] >= 0 && c.T[0] <= c.T[1] && c.T[1] <= textLen) {
				rep.OffsetErrors++
			}
			for _, wi := range c.W {
				if !(wi >= 0 && wi < len(s.Words)) {
					rep.OffsetErrors++
				} else {
					covered[wi] = true
				}
			}
			if len(c.W) > 0 {
				a, b := clampRange(c.T[0], c.T[1], textLen)
				mapped = append(mapped, span{a, b})
			}
		}
		// Sentences with text but no alignment at all (raw-translation
		// fallback — legal, e.g. skipped citations) are counted separately so
		// they don't drown the coverage of actually-aligned sentences.
		if len(mapped) == 0 {
			rep.Unaligned++
			continue
		}
		// Target coverage: which words of the translation TEXT does a highlight
		// reach? (Since v7 the text is the raw translation and chunks only
		// locate spans in it, so counting chunks would be vacuous — every
		// emitted chunk maps a source word by construction.)
		for _, w := range wordRuns(textRunes) {
			rep.TgtWords++
			for _, m := range mapped {
				if m.a < w.b && w.a < m.b {
					rep.TgtAligned++
					break
				}
			}
		}
		rep.SrcTotal += len(s.Words)
		rep.SrcCovered += len(covered)
	}
}
