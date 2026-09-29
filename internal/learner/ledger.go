package learner

import "sort"

// Class is one tap outcome of the ledger (panel phase2/critic-learner-ux.md T0).
type Class int

// The six classes, in report order. Priority when several apply is
// dead-sentence > none > smear > shared > clean-multi > clean.
const (
	Clean      Class = iota // exactly one chunk lights, it maps this word only, one target token
	CleanMulti              // ≥ 2 chunks light, each exclusive to this word (ne … pas)
	Shared                  // a lit chunk also maps other source words: the token answers two questions
	Smear                   // ≥ 3 target tokens light: a phrase-level highlight
	None                    // no chunk contains the word: a silent tap with no reason shown
	Dead                    // translation present but align is empty: every tap in the sentence is silent
	numClass
)

// ClassNames are the names the panel and the reports use.
var ClassNames = [numClass]string{"clean", "clean-multi", "shared", "smear", "none", "dead-sentence"}

func (c Class) String() string {
	if c < 0 || c >= numClass {
		return "?"
	}
	return ClassNames[c]
}

// Classify returns the outcome of tapping source word i of this cell.
func (c *Cell) Classify(i int) Class {
	if len(c.Chunks) == 0 {
		return Dead
	}
	lit, toks, shared := 0, 0, false
	for _, ch := range c.Chunks {
		if !ch.Lit(i) {
			continue
		}
		lit++
		toks += ch.toks
		if len(ch.Words) >= 2 {
			shared = true
		}
	}
	switch {
	case lit == 0:
		return None
	case toks >= 3:
		return Smear
	case shared:
		return Shared
	case lit >= 2:
		return CleanMulti
	}
	return Clean
}

// LedgerOpts configures a ledger run.
type LedgerOpts struct {
	Langs       []string // nil or empty: every target of the manifest
	ContentOnly bool     // score content-word taps only (IsContentWord)
}

// Coarse is the three-class collapse of the same taps used by the panel's
// phase-3 table: a tap is shared as soon as a target token it lights is also
// claimed by another source word, even when two separate chunks claim it. Its
// denominator includes taps in sentences with no translation at all, which the
// six-class ledger skips.
type Coarse struct {
	Taps          int                `json:"taps"`
	Clean         int                `json:"clean"`
	Shared        int                `json:"shared"`
	Dead          int                `json:"dead"`
	NoTranslation int                `json:"noTranslation"`
	Pct           map[string]float64 `json:"pct"`
}

// LangLedger is one language's row.
type LangLedger struct {
	Lang             string             `json:"lang"`
	Taps             int                `json:"taps"`
	Classes          map[string]int     `json:"classes"`
	Pct              map[string]float64 `json:"pct"`
	Silent           int                `json:"silent"`
	SilentPct        float64            `json:"silentPct"`
	ContentTaps      int                `json:"contentTaps"`
	ContentSilent    int                `json:"contentSilent"`
	ContentSilentPct float64            `json:"contentSilentPct"`
	ScriptCleanTaps  int                `json:"scriptCleanTaps"`
	Coarse           Coarse             `json:"coarse"`

	counts  [numClass]int
	coarse  Coarse
	content [2]int // content taps, content silent taps
	sclean  int
}

// Ledger is the whole report.
type Ledger struct {
	Book          string        `json:"book"`
	FormatVersion int           `json:"formatVersion"`
	ContentOnly   bool          `json:"contentOnly"`
	Sentences     int           `json:"sentences"`
	Languages     []*LangLedger `json:"languages"`
	Total         *LangLedger   `json:"total"`
}

// TapLedger classifies every (source word, language) tap of the book.
func (b *Book) TapLedger(opts LedgerOpts) *Ledger {
	langs := opts.Langs
	if len(langs) == 0 {
		langs = b.Targets
	}
	rep := &Ledger{Book: b.Path, FormatVersion: b.Version, ContentOnly: opts.ContentOnly, Sentences: len(b.Sents)}
	for _, lang := range langs {
		row := &LangLedger{Lang: lang}
		for _, s := range b.Sents {
			row.add(s, lang, opts.ContentOnly)
		}
		rep.Languages = append(rep.Languages, row)
	}
	total := &LangLedger{Lang: "total"}
	for _, row := range rep.Languages {
		for c := range row.counts {
			total.counts[c] += row.counts[c]
		}
		total.content[0] += row.content[0]
		total.content[1] += row.content[1]
		total.sclean += row.sclean
		total.coarse.Taps += row.coarse.Taps
		total.coarse.Clean += row.coarse.Clean
		total.coarse.Shared += row.coarse.Shared
		total.coarse.Dead += row.coarse.Dead
		total.coarse.NoTranslation += row.coarse.NoTranslation
	}
	for _, row := range rep.Languages {
		row.finish()
	}
	total.finish()
	rep.Total = total
	return rep
}

func (r *LangLedger) add(s *Sentence, lang string, contentOnly bool) {
	cell := s.Tr[lang]
	if cell == nil {
		for _, w := range s.Words {
			if contentOnly && !IsContentWord(w) {
				continue
			}
			r.coarse.Taps++
			r.coarse.NoTranslation++
		}
		return
	}
	clean := scriptClean(lang, cell.Text)
	lit, shared := cell.coarse(len(s.Words))
	for i, w := range s.Words {
		if contentOnly && !IsContentWord(w) {
			continue
		}
		k := cell.Classify(i)
		r.counts[k]++
		if clean {
			r.sclean++
		}
		if IsContentWord(w) {
			r.content[0]++
			if k == None || k == Dead {
				r.content[1]++
			}
		}
		r.coarse.Taps++
		switch {
		case !lit[i]:
			r.coarse.Dead++
		case shared[i]:
			r.coarse.Shared++
		default:
			r.coarse.Clean++
		}
	}
}

// coarse reports, per source word, whether any target token lights and whether
// a lit token is claimed by more than one source word. Chunks over the same
// target span count as one token — that merge is what separates the coarse
// collapse from the six-class ledger, where two exclusive chunks are
// clean-multi.
func (c *Cell) coarse(n int) (lit, shared []bool) {
	lit, shared = make([]bool, n), make([]bool, n)
	type group struct {
		a, b  int
		words []int
	}
	groups := make([]group, 0, len(c.Chunks))
chunks:
	for _, ch := range c.Chunks {
		for g := range groups {
			if groups[g].a != ch.A || groups[g].b != ch.B {
				continue
			}
			for _, w := range ch.Words {
				if !hasInt(groups[g].words, w) {
					groups[g].words = append(groups[g].words, w)
				}
			}
			continue chunks
		}
		groups = append(groups, group{ch.A, ch.B, append([]int(nil), ch.Words...)})
	}
	for _, g := range groups {
		for _, w := range g.words {
			if w < 0 || w >= n {
				continue
			}
			lit[w] = true
			if len(g.words) > 1 {
				shared[w] = true
			}
		}
	}
	return lit, shared
}

func hasInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func (r *LangLedger) finish() {
	for c := range r.counts {
		r.Taps += r.counts[c]
	}
	r.Classes = map[string]int{}
	r.Pct = map[string]float64{}
	for c := range r.counts {
		r.Classes[ClassNames[c]] = r.counts[c]
		r.Pct[ClassNames[c]] = pct(r.counts[c], r.Taps)
	}
	r.Silent = r.counts[None] + r.counts[Dead]
	r.SilentPct = pct(r.Silent, r.Taps)
	r.ContentTaps, r.ContentSilent = r.content[0], r.content[1]
	r.ContentSilentPct = pct(r.ContentSilent, r.ContentTaps)
	r.ScriptCleanTaps = r.sclean
	r.coarse.Pct = map[string]float64{
		"clean":         pct(r.coarse.Clean, r.coarse.Taps),
		"shared":        pct(r.coarse.Shared, r.coarse.Taps),
		"dead":          pct(r.coarse.Dead, r.coarse.Taps),
		"noTranslation": pct(r.coarse.NoTranslation, r.coarse.Taps),
	}
	r.Coarse = r.coarse
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return 100 * float64(n) / float64(d)
}

// Surface is one source word type with a count, for the "top offenders" lists.
type Surface struct {
	Word string `json:"word"`
	N    int    `json:"n"`
}

func topSurfaces(c map[string]int, n int) []Surface {
	out := make([]Surface, 0, len(c))
	for w, v := range c {
		out = append(out, Surface{w, v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Word < out[j].Word
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}
