package learner

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// countWords counts the word tokens of rs[a:b], a highlighted fragment. The
// token is the panel's (Python `\w+(?:['’\-]\w+)*` under re.UNICODE), not the
// normative tbook-w2 token: the ledger's smear threshold is defined in terms
// of it and the published class shares only reproduce with it. Hand-scanned
// rather than a regexp because it runs once per align chunk of the book.
func countWords(rs []rune, a, b int) int {
	n := 0
	for i := a; i < b; {
		if !wordRune(rs[i]) {
			i++
			continue
		}
		n++
		for i < b {
			for i < b && wordRune(rs[i]) {
				i++
			}
			if i+1 < b && joinRune(rs[i]) && wordRune(rs[i+1]) {
				i++
				continue
			}
			break
		}
	}
	return n
}

func wordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

func joinRune(r rune) bool { return r == '\'' || r == '’' || r == '-' }

// contentStop is the tap ledger's stoplist: with the ≥ 4-letter rule below it
// defines the "content word" of the content-word silent share. Kept verbatim
// from the panel script rather than taken from internal/stopwords, whose lists
// are tuned for the alignment gate and would not reproduce the published
// numbers.
var contentStop = set(`
	the a an of to in on at by for with from as into upon and or but if that which who whom whose
	this these those it its he she they them his her their we you i me my our your be is are was were been am
	have has had do does did not no so than then there here very will would shall should can could may might
	must up out down over under about after before between through during without within`)

// IsContentWord is the ledger's content-word heuristic: at least four letters
// and not in the stoplist above.
func IsContentWord(w string) bool {
	lw := strings.ToLower(w)
	return utf8.RuneCountInString(lw) >= 4 && !contentStop[lw]
}

// functionWords is the consensus metric's own, shorter stoplist (the panel's
// consensus dump used it): solo misses are dominated by function words, and
// this list is what --content-only removes. It is deliberately not the ledger's
// list — the two metrics were calibrated separately.
var functionWords = set(`
	i you your the a an but and or of to in it he she his her that this had have has is was were be been
	as at for with on not no should would could might may will shall do did there then so if which who what
	we they them my me him us from by all any one up out`)

// IsFunctionWord reports whether a source word is closed-class for the
// consensus metric.
func IsFunctionWord(w string) bool { return functionWords[strings.ToLower(w)] }

// cyrillicTarget lists the target languages normally written in Cyrillic; for
// every other language Cyrillic letters in the translation mean a glossary or
// prompt leak, and the ledger reports how many taps land in a clean cell.
var cyrillicTarget = set("ru uk be bg mk sr kk ky tg mn")

func scriptClean(lang, text string) bool {
	if cyrillicTarget[lang] {
		return true
	}
	for _, r := range text {
		if unicode.Is(unicode.Cyrillic, r) {
			return false
		}
	}
	return true
}

func set(words string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}
