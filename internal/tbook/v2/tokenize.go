package v2

import (
	"regexp"
	"sort"
	"unicode/utf8"
)

// tokenRE is the normative tokenizer tbook-w2 (spec §5.1): a letter or number
// starts a token; letters, marks and numbers continue it; an apostrophe or a
// hyphen joins two runs; '.', ',' and ':' join two runs only when the
// right-hand run starts with a digit.
var tokenRE = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{M}\p{N}]*(?:['’\-][\p{L}\p{N}][\p{L}\p{M}\p{N}]*|[.,:]\p{N}[\p{L}\p{M}\p{N}]*)*`)

// Tokenize returns the [start, end) code-point ranges of every tbook-w2 token
// of text, in text order. text is expected to be NFC.
func Tokenize(text string) [][2]int {
	locs := tokenRE.FindAllStringIndex(text, -1)
	out := make([][2]int, 0, len(locs))
	if len(locs) == 0 {
		return out
	}
	// Byte → code-point offsets; matches always fall on rune boundaries.
	cp, bi := 0, 0
	for _, loc := range locs {
		for bi < loc[0] {
			_, n := utf8.DecodeRuneInString(text[bi:])
			bi += n
			cp++
		}
		a := cp
		for bi < loc[1] {
			_, n := utf8.DecodeRuneInString(text[bi:])
			bi += n
			cp++
		}
		out = append(out, [2]int{a, cp})
	}
	return out
}

// TokenStrings returns the token texts of text.
func TokenStrings(text string) []string {
	rs := []rune(text)
	toks := Tokenize(text)
	out := make([]string, len(toks))
	for i, t := range toks {
		out[i] = string(rs[t[0]:t[1]])
	}
	return out
}

// TargetTokens returns a translation's token inventory: the words override
// when present, else the tokenizer output (spec §6.6).
func TargetTokens(tr *Translation) [][2]int {
	if tr.Words != nil {
		return tr.Words
	}
	return Tokenize(tr.T)
}

// SentenceWords returns the word inventory of every sentence of a block: the
// words override when present, else the tokens of text grouped by the sentence
// range containing each token's first code point (spec §4.5). A token that
// starts in no range (a gap, reported by the validator) is grouped with the
// preceding sentence so that indices stay stable.
func SentenceWords(b *Block) [][][2]int {
	if b.Words != nil {
		return b.Words
	}
	out := make([][][2]int, len(b.Sents))
	for k := range out {
		out[k] = [][2]int{}
	}
	if len(b.Sents) == 0 {
		return out
	}
	starts := make([]int, len(b.Sents))
	for k, s := range b.Sents {
		starts[k] = s[0]
	}
	for _, t := range Tokenize(b.Text) {
		k := sort.Search(len(starts), func(i int) bool { return starts[i] > t[0] }) - 1
		if k < 0 {
			k = 0
		}
		out[k] = append(out[k], t)
	}
	return out
}

// GapTokens returns the tokens of text whose first code point lies in no
// sentence range — the words no tap can reach (spec §4.5, last rule).
func GapTokens(b *Block) [][2]int {
	if b.Words != nil {
		return nil
	}
	var gaps [][2]int
	for _, t := range Tokenize(b.Text) {
		inside := false
		for _, s := range b.Sents {
			if s[0] <= t[0] && t[0] < s[1] {
				inside = true
				break
			}
		}
		if !inside {
			gaps = append(gaps, t)
		}
	}
	return gaps
}
