// Package typography is a deterministic, idempotent punctuation pass for
// translated sentences. It changes only quotation marks, dashes, punctuation
// placement and whitespace; letters, digits and word order are never touched.
//
// The central rule is source mirroring: a sentence cut out of a longer
// quotation carries an opening mark iff its source cell starts one and a
// closing mark iff its source cell ends one.
package typography

import (
	"regexp"
	"strings"
	"unicode"
)

// Rule names reported in Result.Rules.
const (
	RuleASCIIQuote  = "ascii-quote"
	RuleDash        = "dash"
	RuleNestInner   = "nest-inner"
	RuleDoubleClose = "double-close"
	RuleAddOpen     = "add-open"
	RuleAddClose    = "add-close"
	RuleStrayOpen   = "stray-open"
	RuleStrayClose  = "stray-close"
	RuleUnwrap      = "unwrap"
	RuleRuPeriod    = "ru-period-close"
	RuleRuComma     = "ru-comma-close"
	RuleFrComma     = "fr-comma-close"
	RuleFrNNBSP     = "fr-nnbsp"
	RuleSpace       = "space"
	RuleRuTagDash   = "ru-tag-dash"
	RuleQuoteDash   = "quote-dash"
	RuleForeign     = "foreign-quote"
)

const nnbsp = ' '

type spec struct {
	open, close   rune // outer quotation marks
	iopen, iclose rune // inner quotation marks
	dash          rune // sentence dash, always spaced
	dialogue      bool // a leading dash may stand for an opening mark
	fr            bool // French spacing
}

var specs = map[string]spec{
	"ru": {'«', '»', '„', '“', '—', true, false},
	"de": {'„', '“', '‚', '‘', '–', false, false},
	"fr": {'«', '»', '“', '”', '—', true, true},
}

// Supported reports whether the pass has rules for the target language.
func Supported(lang string) bool {
	_, ok := specs[base(lang)]
	return ok
}

// Input is one translation cell.
type Input struct {
	PrevSrc string // previous source sentence of the same paragraph ("" if none)
	Src     string // source sentence (English)
	Tr      string // translation
	Lang    string // target language code (ru, de, fr; region suffixes allowed)
}

// Result is the fixed text and the rules that changed it, in firing order.
type Result struct {
	Text  string
	Rules []string
}

// Fix is Apply without paragraph context.
func Fix(src, tr, lang string) string {
	return Apply(Input{Src: src, Tr: tr, Lang: lang}).Text
}

// Apply runs the pass. Unsupported languages are returned unchanged. The rule
// sequence is repeated until the text is stable (at most three rounds), so
// Apply is idempotent even where one rule enables another.
func Apply(in Input) Result {
	sp, ok := specs[base(in.Lang)]
	if !ok || strings.TrimSpace(in.Tr) == "" {
		return Result{Text: in.Tr}
	}
	var fired []string
	t := in.Tr
	for round := 0; round < 3; round++ {
		n := once(in, t, sp, &fired)
		if n == t {
			break
		}
		t = n
	}
	return Result{Text: t, Rules: dedup(fired)}
}

func once(in Input, t string, sp spec, fired *[]string) string {
	step := func(name string, f func(string) string) {
		if n := f(t); n != t {
			t = n
			*fired = append(*fired, name)
		}
	}
	src := analyzeSource(in.Src)
	step(RuleSpace, collapseSpaces)
	step(RuleASCIIQuote, func(s string) string { return asciiQuotes(s, sp) })
	step(RuleDash, func(s string) string { return dashes(s, sp) })
	step(RuleNestInner, func(s string) string { return nestInner(s, sp, src.single) })
	step(RuleDoubleClose, func(s string) string { return doubleClose(s, sp) })
	step(RuleForeign, func(s string) string { return foreignQuotes(s, sp) })
	if sp.dialogue && src.startsOpen {
		step(RuleQuoteDash, func(s string) string { return quoteDash(s, sp) })
	}
	mirror(&t, sp, src, fired)
	if base(in.Lang) == "ru" {
		step(RuleRuComma, ruCommaClose)
		step(RuleRuPeriod, ruPeriodClose)
		step(RuleRuTagDash, func(s string) string { return ruTagDash(s, in.PrevSrc, in.Src) })
	}
	if sp.fr {
		step(RuleFrComma, frCommaClose)
	}
	step(RuleSpace, func(s string) string { return spacing(s, sp) })
	if sp.fr {
		step(RuleFrNNBSP, frSpacing)
	}
	return t
}

func base(lang string) string {
	l := strings.ToLower(lang)
	if i := strings.IndexAny(l, "-_"); i > 0 {
		l = l[:i]
	}
	return l
}

func dedup(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range s {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// ---- source analysis ----

type srcInfo struct {
	startsOpen bool // cell begins with an opening quotation mark
	endsClose  bool // cell ends with a closing quotation mark
	endsSingle bool // ... and that mark is a single one (’)
	endsAny    bool // cell ends with any closing mark, even one of an inner quote
	single     bool // cell has an inner (single) opening quote: nesting evidence
	interior   bool // a double quotation mark occurs away from the cell's ends
	anyQuote   bool // any quotation mark at all
}

func analyzeSource(s string) srcInfo {
	r := []rune(strings.TrimSpace(s))
	var si srcInfo
	if len(r) == 0 {
		return si
	}
	switch r[0] {
	case '“', '‘', '"':
		si.startsOpen = true
	}
	// the final mark closes the cell-level quotation only if it does not
	// close a quote opened inside the cell (… a shriek of “Fire!”)
	last := r[len(r)-1]
	si.endsAny = last == '”' || (last == '"' && !(len(r) > 1 && unicode.IsDigit(r[len(r)-2]))) ||
		(last == '’' && len(r) > 1 && strings.ContainsRune(".,!?…—", r[len(r)-2]))
	switch {
	case last == '”' || (last == '"' && !(len(r) > 1 && unicode.IsDigit(r[len(r)-2]))):
		m := srcPartner(r, '“', '”')
		si.endsClose = m < 0 || m == 0
	case last == '’' && len(r) > 1 && strings.ContainsRune(".,!?…—", r[len(r)-2]):
		m := srcPartner(r, '‘', '’')
		si.endsClose = m < 0 || m == 0
		si.endsSingle = si.endsClose
	}
	si.single = strings.ContainsRune(s, '‘')
	si.anyQuote = strings.ContainsAny(s, "“”‘\"")
	lo, hi := 0, len(r)
	for lo < hi && strings.ContainsRune("“‘\"", r[lo]) {
		lo++
	}
	for hi > lo && strings.ContainsRune("”’\".,!?…—", r[hi-1]) {
		hi--
	}
	si.interior = strings.ContainsAny(string(r[lo:hi]), "“”\"")
	return si
}

// srcPartner returns the index of the opening mark matched by the cell's final
// closing mark, or -1. For single quotes, ’ between letters is an apostrophe.
func srcPartner(r []rune, open, close rune) int {
	var st []int
	res := -1
	for i, c := range r {
		switch {
		case c == open || (open == '“' && c == '"' && (i == 0 || unicode.IsSpace(r[i-1]))):
			st = append(st, i)
		case c == close || (close == '”' && c == '"' && i > 0 && !unicode.IsSpace(r[i-1])):
			if close == '’' && i > 0 && i+1 < len(r) && unicode.IsLetter(r[i-1]) && unicode.IsLetter(r[i+1]) {
				continue
			}
			if len(st) > 0 {
				j := st[len(st)-1]
				st = st[:len(st)-1]
				if i == len(r)-1 {
					res = j
				}
			} else if i == len(r)-1 {
				res = -1
			}
		}
	}
	return res
}

// ---- helpers ----

func isQuoteRune(r rune) bool {
	return strings.ContainsRune("«»„“”‚‘’\"‹›", r)
}

func isDashRune(r rune) bool { return r == '—' || r == '–' }

var multiSpace = regexp.MustCompile(`[ \t]{2,}`)

func collapseSpaces(s string) string {
	return strings.TrimSpace(multiSpace.ReplaceAllString(s, " "))
}

// ---- ASCII quotes ----

func asciiQuotes(s string, sp spec) string {
	if !strings.Contains(s, `"`) || strings.Contains(s, "`") {
		return s
	}
	r := []rune(s)
	for i, c := range r {
		if c != '"' {
			continue
		}
		var prev, next rune = ' ', ' '
		if i > 0 {
			prev = r[i-1]
		}
		if i+1 < len(r) {
			next = r[i+1]
		}
		if unicode.IsDigit(prev) {
			continue // inch mark
		}
		prevOpenCtx := i == 0 || unicode.IsSpace(prev) || strings.ContainsRune("([{—–-«„“‚‘/", prev)
		nextSpace := i+1 >= len(r) || unicode.IsSpace(next)
		switch {
		case prevOpenCtx && !nextSpace:
			r[i] = sp.open
		case !prevOpenCtx:
			r[i] = sp.close
		}
	}
	return string(r)
}

// ---- dashes ----

var (
	doubleHyphen  = regexp.MustCompile(`\s*--+\s*`)
	spacedHyphen  = regexp.MustCompile(` - `)
	leadHyphen    = regexp.MustCompile(`^-\s+`)
	leadDashTight = regexp.MustCompile(`^([—–])([\p{L}«„])`)
	spacedEn      = regexp.MustCompile(` – `)
	// dash between two tokens, with any spacing
	midDash = regexp.MustCompile(`([\p{L}\p{N}.,;!?…»“”’)])[ \x{00A0}]*—[ \x{00A0}]*([\p{L}\p{N}«„“(])`)
	// dash glued to a word before a closing mark or the end
	tailDash = regexp.MustCompile(`([\p{L}\p{N}])—([»“”’]|$)`)
)

func dashes(s string, sp spec) string {
	d := string(sp.dash)
	s = doubleHyphen.ReplaceAllString(s, " — ")
	s = spacedHyphen.ReplaceAllString(s, " — ")
	if sp.dialogue {
		s = leadHyphen.ReplaceAllString(s, "— ")
		if strings.HasPrefix(s, "– ") {
			s = "— " + s[len("– "):]
		}
		s = leadDashTight.ReplaceAllString(s, "— $2")
		s = spacedEn.ReplaceAllString(s, " — ")
	}
	for k := 0; k < 3; k++ {
		n := midDash.ReplaceAllStringFunc(s, func(m string) string {
			sub := midDash.FindStringSubmatch(m)
			a, b := []rune(sub[1]), []rune(sub[2])
			if unicode.IsDigit(a[len(a)-1]) && unicode.IsDigit(b[0]) {
				return m // numeric range
			}
			return sub[1] + " " + d + " " + sub[2]
		})
		if n == s {
			break
		}
		s = n
	}
	s = tailDash.ReplaceAllString(s, "$1 "+d+"$2")
	if sp.dash != '—' {
		// the leading em dash of a line (dialogue) is kept only where dialogue dashes exist
		s = strings.ReplaceAll(s, " — ", " "+d+" ")
	}
	return s
}

// quoteDash drops a dialogue dash right after the opening mark of a cell
// whose source opens a quotation: «— Текст → «Текст. The mark already opens
// the speech; the pair is a double marking.
var quoteDashRe = regexp.MustCompile(`^([«])[ \x{00A0}\x{202F}]*(?:[—–]|-[ \x{00A0}\x{202F}])[ \x{00A0}\x{202F}]*`)

func quoteDash(s string, sp spec) string {
	if !strings.HasPrefix(s, string(sp.open)) {
		return s
	}
	return quoteDashRe.ReplaceAllString(s, "$1")
}

// foreignQuotes turns English “ ” used as the cell's outer marks into the
// target's outer marks: a “ opening the cell, and a ” (ru: also “) ending it
// that closes no inner quotation of the target.
func foreignQuotes(s string, sp spec) string {
	if strings.Contains(s, "`") {
		return s
	}
	r := []rune(s)
	if len(r) < 2 {
		return s
	}
	if r[0] == '“' && (unicode.IsLetter(r[1]) || unicode.IsDigit(r[1]) || r[1] == '‘' || r[1] == '…') {
		r[0] = sp.open
	}
	last := len(r) - 1
	c := r[last]
	if isSpaceRune(r[last-1]) || !(c == '”' || c == '“' && sp.iclose == '“') || c == sp.close {
		return string(r)
	}
	prefix := r[:last]
	switch {
	case sp.fr && strings.ContainsRune(string(prefix), '“'):
		return string(r) // closes an inner “…”
	case sp.iclose == c && strings.ContainsRune(string(prefix), sp.iopen):
		return string(r) // ru „…“
	}
	r[last] = sp.close
	return string(r)
}

// ---- nesting ----

// depthScan returns, for every outer mark position, the depth after walking
// the marks, starting from d0 = number of leading unmatched closings.
func depthScan(r []rune, sp spec) (d0, maxd, final int) {
	d, minD := 0, 0
	for _, c := range r {
		switch c {
		case sp.open:
			d++
		case sp.close:
			d--
			if d < minD {
				minD = d
			}
		}
	}
	d0 = -minD
	d = d0
	maxd = d
	for _, c := range r {
		switch c {
		case sp.open:
			d++
			if d > maxd {
				maxd = d
			}
		case sp.close:
			d--
		}
	}
	return d0, maxd, d
}

func nestInner(s string, sp spec, srcSingle bool) string {
	r := []rune(s)
	d0, maxd, final := depthScan(r, sp)
	if maxd != 2 || !(srcSingle || (d0 == 0 && final == 0)) {
		return s
	}
	d := d0
	for i, c := range r {
		switch c {
		case sp.open:
			d++
			if d == 2 {
				r[i] = sp.iopen
			}
		case sp.close:
			if d == 2 {
				r[i] = sp.iclose
			}
			d--
		}
	}
	return string(r)
}

func doubleClose(s string, sp spec) string {
	dd := string([]rune{sp.close, sp.close})
	for strings.Contains(s, dd) {
		s = strings.ReplaceAll(s, dd, string(sp.close))
	}
	return s
}

// ---- source mirroring ----

// matches pairs outer marks with a stack; unmatched marks map to -1.
func matches(r []rune, sp spec) []int {
	m := make([]int, len(r))
	var st []int
	for i, c := range r {
		m[i] = -1
		switch c {
		case sp.open:
			st = append(st, i)
		case sp.close:
			if len(st) > 0 {
				j := st[len(st)-1]
				st = st[:len(st)-1]
				m[i], m[j] = j, i
			}
		}
	}
	return m
}

const endPunct = ".,;:!?…"

// finalClose is the index of the closing mark that ends the cell (optionally
// followed by final punctuation, as in ru `».`), or -1.
func finalClose(r []rune, sp spec) int {
	i := len(r) - 1
	for i >= 0 && isSpaceRune(r[i]) {
		i--
	}
	if i >= 0 && r[i] == sp.close {
		return i
	}
	j := i
	for j >= 0 && strings.ContainsRune(endPunct, r[j]) {
		j--
	}
	if j < i {
		for j >= 0 && isSpaceRune(r[j]) {
			j--
		}
		if j >= 0 && r[j] == sp.close {
			return j
		}
	}
	return -1
}

func isSpaceRune(r rune) bool { return unicode.IsSpace(r) || r == nnbsp }

func hasLetter(r []rune) bool {
	for _, c := range r {
		if unicode.IsLetter(c) || unicode.IsDigit(c) {
			return true
		}
	}
	return false
}

// openDepth is the number of opening marks left unmatched at the end.
func openDepth(r []rune, sp spec) int {
	d := 0
	for _, c := range r {
		switch c {
		case sp.open:
			d++
		case sp.close:
			if d > 0 {
				d--
			}
		}
	}
	return d
}

func mirror(t *string, sp spec, src srcInfo, fired *[]string) {
	r := []rune(*t)
	if !hasLetter(r) {
		return
	}
	note := func(n string) { *fired = append(*fired, n) }
	startsOpen := r[0] == sp.open
	dashStart := sp.dialogue && isDashRune(r[0])

	// opening side
	if src.startsOpen && !startsOpen && !dashStart && !isQuoteRune(r[0]) {
		r = append([]rune{sp.open}, r...)
		note(RuleAddOpen)
	} else if !src.startsOpen && startsOpen {
		m := matches(r, sp)
		fc := finalClose(r, sp)
		switch {
		case m[0] == -1:
			r = removeAt(r, 0)
			note(RuleStrayOpen)
		case m[0] == fc && !src.anyQuote:
			r = removeAt(r, fc)
			r = removeAt(r, 0)
			note(RuleUnwrap)
			*t = string(r)
			return
		case m[0] == fc && src.endsClose && !src.interior:
			r = removeAt(r, 0)
			note(RuleStrayOpen)
		}
	}

	// closing side
	fc := finalClose(r, sp)
	dashStart = sp.dialogue && isDashRune(r[0])
	if src.endsClose && fc < 0 && !dashStart {
		last := r[len(r)-1]
		// a self-contained source quote needs a close only for an open mark
		mirrored := src.endsSingle && last == sp.iclose
		if !mirrored && (!isQuoteRune(last) || last == sp.iclose) && (!src.startsOpen || openDepth(r, sp) > 0) {
			r = addClose(r, sp)
			note(RuleAddClose)
		}
	} else if !src.endsAny && fc >= 0 {
		m := matches(r, sp)
		if m[fc] == -1 || (m[fc] == 0 && src.startsOpen && !src.interior) {
			r = removeAt(r, fc)
			note(RuleStrayClose)
		}
	}
	*t = string(r)
}

func removeAt(r []rune, i int) []rune {
	out := append([]rune{}, r[:i]...)
	return append(out, r[i+1:]...)
}

// addClose puts the closing mark after the final punctuation; in Russian a
// single final period goes outside the guillemet (`».`).
func addClose(r []rune, sp spec) []rune {
	j := len(r)
	for j > 0 && strings.ContainsRune(endPunct, r[j-1]) {
		j--
	}
	tail := string(r[j:])
	if sp.open == '«' && !sp.fr && tail == "." {
		out := append(append([]rune{}, r[:j]...), sp.close, '.')
		return out
	}
	return append(append([]rune{}, r...), sp.close)
}

// ---- Russian placement ----

var ruComma = regexp.MustCompile(`,(\s*)»`)

func ruCommaClose(s string) string { return ruComma.ReplaceAllString(s, "»,") }

// ruPeriodClose moves a period out of the closing guillemet where the quote
// ends the cell or a sentence inside it. Ellipses and one-letter
// abbreviations (т. д.) stay inside.
func ruPeriodClose(s string) string {
	r := []rune(s)
	for i := 1; i+1 < len(r); i++ {
		if r[i] != '.' || r[i+1] != '»' {
			continue
		}
		if r[i-1] == '.' || r[i-1] == '…' {
			continue
		}
		// word before the period
		k := i - 1
		for k >= 0 && unicode.IsLetter(r[k]) {
			k--
		}
		if i-1-k == 1 && (k < 0 || r[k] == ' ') {
			continue // single-letter abbreviation
		}
		rest := r[i+2:]
		atEnd := strings.TrimSpace(string(rest)) == ""
		nextSentence := len(rest) >= 2 && rest[0] == ' ' && (unicode.IsUpper(rest[1]) || rest[1] == '«')
		if atEnd || nextSentence {
			r[i], r[i+1] = '»', '.'
		}
	}
	return string(r)
}

// ruTagDash adds the dialogue dash to a speech-tag fragment split off after a
// quoted cell: “Seven!” | I answered. -> «Семь!» | — ответил я.
func ruTagDash(s, prevSrc, src string) string {
	p := []rune(strings.TrimSpace(prevSrc))
	if len(p) < 2 || p[len(p)-1] != '”' || !strings.ContainsRune("?!…—.,", p[len(p)-2]) {
		return s
	}
	if ss := strings.TrimSpace(src); ss == "" || isQuoteRune([]rune(ss)[0]) {
		return s
	}
	r := []rune(s)
	if len(r) == 0 || !unicode.Is(unicode.Cyrillic, r[0]) || !unicode.IsLower(r[0]) {
		return s
	}
	return "— " + s
}

// ---- French ----

var frComma = regexp.MustCompile(`[ \x{00A0}\x{202F}]*,[ \x{00A0}\x{202F}]*»`)

func frCommaClose(s string) string { return frComma.ReplaceAllString(s, " »,") }

func isFrSpace(c rune) bool { return c == ' ' || c == '\u00A0' || c == nnbsp }

var (
	frInnerOpen  = regexp.MustCompile(`“[ \x{00A0}\x{202F}]+`)
	frInnerClose = regexp.MustCompile(`[ \x{00A0}\x{202F}]+”`)
)

// frSpacing puts U+202F inside guillemets and before ; : ! ? and keeps the
// inner “ ” tight.
func frSpacing(s string) string {
	s = frInnerOpen.ReplaceAllString(s, "“")
	s = frInnerClose.ReplaceAllString(s, "”")
	r := []rune(s)
	out := make([]rune, 0, len(r)+8)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '«':
			out = append(out, c)
			j := i + 1
			for j < len(r) && isFrSpace(r[j]) {
				j++
			}
			if j < len(r) {
				out = append(out, nnbsp)
			}
			i = j - 1
		case c == '»' || strings.ContainsRune(";:!?", c):
			k := len(out)
			for k > 0 && isFrSpace(out[k-1]) {
				k--
			}
			if k == 0 {
				out = append(out, c)
				continue
			}
			prev := out[k-1]
			need := true
			if c != '»' && (strings.ContainsRune(";:!?", prev) || prev == '«') {
				need = false
			}
			if c == ':' && unicode.IsDigit(prev) && i+1 < len(r) && unicode.IsDigit(r[i+1]) {
				need = false // time, 10:30
			}
			if c == ':' && i+2 < len(r) && r[i+1] == '/' && r[i+2] == '/' {
				need = false // URL
			}
			if !need && k == len(out) {
				out = append(out, c)
				continue
			}
			out = out[:k]
			if need {
				out = append(out, nnbsp)
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

// ---- generic spacing ----

var (
	spaceBeforeComma = regexp.MustCompile(` +,`)
	spaceBeforeDot   = regexp.MustCompile(` +\.([^.]|$)`)
	spaceBeforeMark  = regexp.MustCompile(` +([;:!?])`)
	closeRes         = map[rune]*regexp.Regexp{}
	openRes          = map[rune]*regexp.Regexp{}
)

func init() {
	for _, sp := range specs {
		if sp.fr {
			continue
		}
		closeRes[sp.close] = regexp.MustCompile(` +` + regexp.QuoteMeta(string(sp.close)) + `([^\p{L}\p{N}]|$)`)
		openRes[sp.open] = regexp.MustCompile(regexp.QuoteMeta(string(sp.open)) + ` +`)
	}
}

func spacing(s string, sp spec) string {
	s = collapseSpaces(s)
	s = spaceBeforeComma.ReplaceAllString(s, ",")
	s = spaceBeforeDot.ReplaceAllString(s, ".$1")
	if !sp.fr {
		s = spaceBeforeMark.ReplaceAllString(s, "$1")
		s = openRes[sp.open].ReplaceAllString(s, string(sp.open))
		s = closeRes[sp.close].ReplaceAllString(s, string(sp.close)+"$1")
	}
	return s
}
