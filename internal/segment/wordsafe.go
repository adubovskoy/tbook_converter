package segment

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sentencizer/sentencizer"
)

// wordSafeSegmenter repairs a sentencizer v0.2.0 defect: when one paragraph
// holds two different prepositive abbreviations ("Ms. … Dr.") or two different
// ’-contractions ending sentences ("Jo’s. … didn’t."), the library's
// priorIndex bookkeeping shifts its boundaries left into words:
// "Ms. Fal and Dr. Nopri." → "M" | "s. Fal and D" | "r. Nopri.".
type wordSafeSegmenter struct {
	sentencizer.Segmenter
}

// Segment returns the library's pieces with every mid-word cut moved forward
// past the rest of the word and its closing punctuation, and dropped when the
// text before it does not end a sentence on its own (an abbreviation).
func (w wordSafeSegmenter) Segment(text string) []string {
	pieces := w.Segmenter.Segment(text)
	if len(pieces) < 2 {
		return pieces
	}
	// Byte offset in text where each piece starts; bail out on any piece that
	// is not a verbatim substring — nothing reliable to repair then.
	starts := make([]int, 0, len(pieces))
	cur := 0
	for _, p := range pieces {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		i := strings.Index(text[cur:], t)
		if i < 0 {
			return pieces
		}
		starts = append(starts, cur+i)
		cur += i + len(t)
	}
	repaired := false
	cuts := make([]int, 0, len(starts))
	for _, c := range starts[1:] {
		if !midWord(text, c) {
			cuts = append(cuts, c)
			continue
		}
		repaired = true
		c = pastWordEnd(text, c)
		if c >= len(text) {
			continue
		}
		prev := 0
		if len(cuts) > 0 {
			prev = cuts[len(cuts)-1]
		}
		if c <= prev || !w.endsSentence(strings.TrimSpace(text[prev:c])) {
			continue
		}
		cuts = append(cuts, c)
	}
	if !repaired {
		return pieces
	}
	out := make([]string, 0, len(cuts)+1)
	prev := 0
	for _, c := range append(cuts, len(text)) {
		if c <= prev {
			continue
		}
		if s := strings.TrimSpace(text[prev:c]); s != "" {
			out = append(out, s)
		}
		prev = c
	}
	return out
}

// endsSentence asks the raw library whether s ends a complete sentence:
// appending a capitalized probe must add a segment. A lone "Ms." does not.
func (w wordSafeSegmenter) endsSentence(s string) bool {
	if s == "" || !quoteEndsSentenceRE.MatchString(s) {
		return false
	}
	return nonEmpty(w.Segmenter.Segment(s+" Next.")) > nonEmpty(w.Segmenter.Segment(s))
}

func nonEmpty(ss []string) int {
	n := 0
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			n++
		}
	}
	return n
}

// midWord reports whether byte offset c of text splits a word: a letter or
// digit (or a word-joining apostrophe) on both sides, no space between.
func midWord(text string, c int) bool {
	if c <= 0 || c >= len(text) {
		return false
	}
	l, _ := utf8.DecodeLastRuneInString(text[:c])
	r, _ := utf8.DecodeRuneInString(text[c:])
	word := func(x rune) bool { return unicode.IsLetter(x) || unicode.IsDigit(x) }
	return (word(l) || l == '’' || l == '\'') && word(r)
}

// pastWordEnd advances from c over the rest of the word, then over sentence
// punctuation and closing quotes, returning the offset of what follows.
func pastWordEnd(text string, c int) int {
	for c < len(text) {
		r, n := utf8.DecodeRuneInString(text[c:])
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '’' || r == '\'') {
			break
		}
		c += n
	}
	for c < len(text) {
		r, n := utf8.DecodeRuneInString(text[c:])
		if !strings.ContainsRune(`.!?…"»”’')]`, r) {
			break
		}
		c += n
	}
	return c
}
