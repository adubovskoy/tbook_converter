package learner

import (
	"fmt"
	"strings"
)

// MinTargets is the smallest number of target languages the consensus metric
// is defined for: with fewer there is nothing to disagree.
const MinTargets = 3

// ConsensusOpts configures a consensus run.
type ConsensusOpts struct {
	// MinLangs is how many aligned languages a sentence needs before it votes.
	// Below MinTargets it is raised to MinTargets. The panel's published
	// numbers for the 8-language sample were taken at 6.
	MinLangs    int
	ContentOnly bool // count content-class slots only (IsFunctionWord removed)
	Top         int  // how many surfaces to list per table (0 = 20)
}

// Consensus is the cross-target agreement report: per source-word slot, which
// languages highlight it.
type Consensus struct {
	Book              string         `json:"book"`
	FormatVersion     int            `json:"formatVersion"`
	Targets           []string       `json:"targets"`
	MinLangs          int            `json:"minLangs"`
	ContentOnly       bool           `json:"contentOnly"`
	Sentences         int            `json:"sentences"`
	Cells             int            `json:"cells"`
	Slots             int            `json:"slots"`
	Unanimous         int            `json:"unanimous"`
	UnanimousPct      float64        `json:"unanimousPct"`
	Missed            int            `json:"missed"`
	MissedPct         float64        `json:"missedPct"`
	Hist              map[int]int    `json:"hist"`
	Solo              int            `json:"solo"`
	SoloPct           float64        `json:"soloPct"`
	SoloOfMissedPct   float64        `json:"soloOfMissedPct"`
	SoloByLang        map[string]int `json:"soloByLang"`
	SoloSentences     int            `json:"soloSentences"`
	UnanimousMiss     int            `json:"unanimousMiss"`
	UnanimousDistinct int            `json:"unanimousMissDistinct"`
	TopUnanimousMiss  []Surface      `json:"topUnanimousMiss"`
	TopSolo           []Surface      `json:"topSolo"`
}

// Consensus computes the report. It refuses a book with fewer than MinTargets
// targets: the metric is the disagreement between independent alignments of
// the same sentence, and two opinions cannot single anyone out.
func (b *Book) Consensus(opts ConsensusOpts) (*Consensus, error) {
	if len(b.Targets) < MinTargets {
		return nil, fmt.Errorf("%s has %d target language(s) %v: cross-target consensus needs at least %d "+
			"(it scores which languages disagree about a source word)", b.Path, len(b.Targets), b.Targets, MinTargets)
	}
	minLangs := opts.MinLangs
	if minLangs < MinTargets {
		minLangs = MinTargets
	}
	top := opts.Top
	if top <= 0 {
		top = 20
	}
	r := &Consensus{
		Book: b.Path, FormatVersion: b.Version, Targets: b.Targets,
		MinLangs: minLangs, ContentOnly: opts.ContentOnly,
		Hist: map[int]int{}, SoloByLang: map[string]int{},
	}
	soloWords, unanimousWords := map[string]int{}, map[string]int{}
	hit := map[string]map[int]bool{}
	for _, s := range b.Sents {
		if len(s.Words) == 0 {
			continue
		}
		var langs []string
		for _, lang := range b.Targets {
			if c := s.Tr[lang]; c != nil && len(c.Chunks) > 0 {
				langs = append(langs, lang)
			}
		}
		if len(langs) < minLangs {
			continue
		}
		r.Sentences++
		r.Cells += len(langs)
		for _, lang := range langs {
			h := map[int]bool{}
			for _, ch := range s.Tr[lang].Chunks {
				for _, w := range ch.Words {
					h[w] = true
				}
			}
			hit[lang] = h
		}
		anySolo := false
		for i, w := range s.Words {
			if opts.ContentOnly && IsFunctionWord(w) {
				continue
			}
			r.Slots++
			k, solo := 0, ""
			for _, lang := range langs {
				if !hit[lang][i] {
					k++
					solo = lang
				}
			}
			r.Hist[k]++
			switch {
			case k == 0:
				r.Unanimous++
			case k == len(langs):
				r.UnanimousMiss++
				unanimousWords[strings.ToLower(w)]++
			case k == 1:
				r.Solo++
				r.SoloByLang[solo]++
				soloWords[strings.ToLower(w)]++
				anySolo = true
			}
		}
		if anySolo {
			r.SoloSentences++
		}
	}
	r.Missed = r.Slots - r.Unanimous
	r.UnanimousPct = pct(r.Unanimous, r.Slots)
	r.MissedPct = pct(r.Missed, r.Slots)
	r.SoloPct = pct(r.Solo, r.Slots)
	r.SoloOfMissedPct = pct(r.Solo, r.Missed)
	r.UnanimousDistinct = len(unanimousWords)
	r.TopUnanimousMiss = topSurfaces(unanimousWords, top)
	r.TopSolo = topSurfaces(soloWords, top)
	return r, nil
}
