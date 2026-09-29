package typography

import (
	"unicode"

	"github.com/dimando/reader/converter/internal/tbook"
)

// ApplyParagraphs runs the pass over every lang translation in paras, in place,
// with the previous source sentence of the paragraph as context. Alignment
// chunks are remapped to the new text. A cell whose letters or digits would
// change, or whose chunks cannot be remapped, is left untouched. Returns the
// number of changed cells.
func ApplyParagraphs(paras [][]*tbook.Sentence, lang string) int {
	if !Supported(lang) {
		return 0
	}
	changed := 0
	for _, para := range paras {
		prev := ""
		for _, s := range para {
			if tr, ok := s.Tr[lang]; ok {
				if nt, ok := applyTranslation(prev, s.Src, tr, lang); ok {
					s.Tr[lang] = nt
					changed++
				}
			}
			prev = s.Src
		}
	}
	return changed
}

func applyTranslation(prevSrc, src string, tr tbook.Translation, lang string) (tbook.Translation, bool) {
	res := Apply(Input{PrevSrc: prevSrc, Src: src, Tr: tr.Text, Lang: lang})
	if res.Text == tr.Text {
		return tr, false
	}
	spans := make([][2]int, len(tr.Align))
	for i, c := range tr.Align {
		spans[i] = c.T
	}
	mapped, ok := Remap(tr.Text, res.Text, spans)
	if !ok {
		return tr, false
	}
	out := tbook.Translation{Text: res.Text, Q: tr.Q}
	if tr.Align != nil {
		out.Align = make([]tbook.AlignChunk, 0, len(tr.Align))
	}
	for i, c := range tr.Align {
		if mapped[i][0] < 0 {
			continue // chunk held no letter or digit
		}
		out.Align = append(out.Align, tbook.AlignChunk{T: mapped[i], W: c.W})
	}
	return out, true
}

// Remap moves [start,end) rune spans of before onto after, which must carry
// the same sequence of letters and digits. Each span is narrowed to its first
// and last letter or digit; a span with none maps to {-1,-1}. ok is false when
// the letter/digit sequences differ or a span is out of range.
func Remap(before, after string, spans [][2]int) ([][2]int, bool) {
	br, ar := []rune(before), []rune(after)
	var bpos, apos []int // rune index of the k-th letter/digit
	rank := make([]int, len(br)+1)
	for i, r := range br {
		rank[i] = len(bpos)
		if isAlnum(r) {
			bpos = append(bpos, i)
		}
	}
	rank[len(br)] = len(bpos)
	for i, r := range ar {
		if isAlnum(r) {
			apos = append(apos, i)
		}
	}
	if len(bpos) != len(apos) {
		return nil, false
	}
	for k := range bpos {
		if br[bpos[k]] != ar[apos[k]] {
			return nil, false
		}
	}
	out := make([][2]int, len(spans))
	for i, sp := range spans {
		a, b := sp[0], sp[1]
		if a < 0 || b > len(br) || a > b {
			return nil, false
		}
		first, last := rank[a], rank[b]-1 // letters/digits with index in [a,b)
		if first > last {
			out[i] = [2]int{-1, -1}
			continue
		}
		out[i] = [2]int{apos[first], apos[last] + 1}
	}
	return out, true
}

func isAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
