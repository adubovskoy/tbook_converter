package tbook

import "unicode/utf8"

// Report summarizes a structural validation pass (mirrors the spec §10.3 check).
type Report struct {
	Sentences    int
	Empty        int // translations with empty text
	OffsetErrors int // out-of-range word/align offsets or indices
}

// OK reports whether the archive is structurally sound and fully translated.
func (r Report) OK() bool { return r.OffsetErrors == 0 && r.Empty == 0 }

// Validate checks every sentence's word offsets and each translation's align
// spans/indices against the spec invariants, over the given target languages.
func Validate(chapters []Chapter, targets []string) Report {
	var rep Report
	for _, ch := range chapters {
		for _, para := range ch.Paragraphs {
			for _, s := range para {
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
				for _, code := range targets {
					tr, ok := s.Tr[code]
					if !ok || tr.Text == "" {
						rep.Empty++
						continue
					}
					textLen := utf8.RuneCountInString(tr.Text)
					for _, c := range tr.Align {
						if !(c.T[0] >= 0 && c.T[0] <= c.T[1] && c.T[1] <= textLen) {
							rep.OffsetErrors++
						}
						for _, wi := range c.W {
							if !(wi >= 0 && wi < len(s.Words)) {
								rep.OffsetErrors++
							}
						}
					}
				}
			}
		}
	}
	return rep
}
