// Command lexeval evaluates the static drift detector (internal/lexcheck)
// against a produced .tbook — offline, no API calls.
//
// Method: every sentence in the book is a presumed-good NEGATIVE (optionally
// restricted to judge-OK sentences via --judged-only, or to sentences langcheck
// considers to be in the right language via --clean-only); for each, a synthetic
// POSITIVE is derived by shifting every align mapping one source word to the
// right — the exact signature of positional drift. The detector must flag the
// shifted copies (recall) and not the originals (false positives). A threshold
// grid is swept so lexcheck's defaults can be chosen from data.
//
//	lexeval book.tbook -s es -t ru [--lexicons lexicons] [--cache-dir .tbook_cache]
//	        [--judge-model MODEL --judged-only] [--clean-only]
//	        [--covered 3,4,5 --rates 0.25,0.34 --shifts 2,3] [--examples 3] [--json out.json]
//
// --contamination answers a different question: how much of what langcheck
// calls a wrong-language cell does lexcheck flag (spoiler: unlookupable text
// yields no dictionary evidence, so nothing aggregates).
package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/dimando/reader/converter/internal/langcheck"
	"github.com/dimando/reader/converter/internal/lexcheck"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
)

type chapterPayload struct {
	Paragraphs [][]*tbook.Sentence `json:"paragraphs"`
	Tables     []tbook.Table       `json:"tables"`
}

// point is one threshold triple and what it scores on the evaluation set.
type point struct {
	MinCoveredPairs int     `json:"minCoveredPairs"`
	LowSupportRate  float64 `json:"lowSupportRate"`
	MinShiftHits    int     `json:"minShiftHits"`
	Current         bool    `json:"current,omitempty"`
	RecallFull      float64 `json:"recallFull"`
	RecallHalf      float64 `json:"recallHalf"`
	FPRate          float64 `json:"fpRate"`
	Precision       float64 `json:"precision"`
}

// report is the machine-readable form of a run (--json).
type report struct {
	Book           string  `json:"book"`
	Source         string  `json:"source"`
	Target         string  `json:"target"`
	LexiconEntries int     `json:"lexiconEntries"`
	Sentences      int     `json:"sentences"`
	Negatives      int     `json:"negatives"`
	DroppedForeign int     `json:"droppedForeign,omitempty"`
	ScoredPairs    int     `json:"scoredPairs"`
	CoveredPairs   int     `json:"coveredPairs"`
	SupportedPairs int     `json:"supportedPairs"`
	PairCoverage   float64 `json:"pairCoverage"`
	SupportRate    float64 `json:"supportRate"`
	Grid           []point `json:"grid"`
}

func main() {
	src := flag.String("s", "es", "source language")
	tgt := flag.String("t", "ru", "target language")
	lexDir := flag.String("lexicons", "lexicons", "lexicon directory")
	cacheDir := flag.String("cache-dir", ".tbook_cache", "cache dir (for --judged-only)")
	judgeModel := flag.String("judge-model", "", "judge model id (for --judged-only)")
	judgedOnly := flag.Bool("judged-only", false, "use only sentences the LLM judge marked OK as negatives")
	cleanOnly := flag.Bool("clean-only", false, "drop sentences langcheck calls wrong-language from the negatives")
	rates := flag.String("rates", "0.25,0.30,0.34,0.40,0.45", "low-support rates to sweep")
	shifts := flag.String("shifts", "2,3", "shift-hit thresholds to sweep")
	covered := flag.String("covered", "4", "min-covered-pairs thresholds to sweep")
	examples := flag.Int("examples", 0, "print this many missed positives and false positives")
	jsonOut := flag.String("json", "", "also write the numbers to this JSON file")
	contamination := flag.Bool("contamination", false, "report the flag rate split by langcheck's wrong-language verdict")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: lexeval book.tbook [flags]")
		os.Exit(2)
	}

	dict, err := lexcheck.Load(*lexDir, *src, *tgt)
	if err != nil || dict == nil {
		fmt.Fprintf(os.Stderr, "no lexicon for %s-%s in %s (err=%v)\n", *src, *tgt, *lexDir, err)
		os.Exit(1)
	}
	fmt.Printf("lexicon %s-%s: %d headwords\n", *src, *tgt, dict.Entries())

	sentences, err := readBook(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "read book:", err)
		os.Exit(1)
	}

	if *contamination {
		reportContamination(dict, sentences, *src, *tgt)
		return
	}

	rateGrid, err1 := floats(*rates)
	shiftGrid, err2 := ints(*shifts)
	coveredGrid, err3 := ints(*covered)
	for _, e := range []error{err1, err2, err3} {
		if e != nil {
			fmt.Fprintln(os.Stderr, "bad grid:", e)
			os.Exit(2)
		}
	}

	foreign := map[string]bool{}
	if *cleanOnly {
		for _, f := range langcheck.Check(sentences, *src, []string{*tgt}, langcheck.Options{}) {
			foreign[f.Src] = true
		}
	}

	var clean []*tbook.Sentence
	var dropped int
	for _, s := range sentences {
		tr, ok := s.Tr[*tgt]
		if !ok || tr.Text == "" || len(tr.Align) < 4 || len(s.Words) < 5 {
			continue
		}
		if *cleanOnly && foreign[s.Src] {
			dropped++
			continue
		}
		if *judgedOnly {
			v, ok := translate.VerdictFor(*cacheDir, *judgeModel, *src, *tgt, s)
			if !ok || !v.OK {
				continue
			}
		}
		clean = append(clean, s)
	}
	if *cleanOnly {
		fmt.Printf("dropped %d sentences langcheck calls wrong-language\n", dropped)
	}
	fmt.Printf("evaluation set: %d sentences (negatives) + %d synthetic drifted copies (positives)\n",
		len(clean), len(clean))
	if len(clean) == 0 {
		return
	}

	// Score each sentence once in its three forms; the grid then only compares
	// the stored counters against thresholds.
	type scored struct {
		s                *tbook.Sentence
		orig, full, half lexcheck.Result
	}
	set := make([]scored, len(clean))
	var scoredPairs, coveredPairs, supportedPairs int
	for i, s := range clean {
		r := dict.CheckSentence(s, *tgt)
		set[i] = scored{
			s: s, orig: r,
			full: dict.CheckSentence(shifted(s, *tgt, 0), *tgt),
			half: dict.CheckSentence(shifted(s, *tgt, 2), *tgt),
		}
		scoredPairs += r.Scored
		coveredPairs += r.Covered
		supportedPairs += r.Supported
	}
	fmt.Printf("dictionary coverage: %d scoreable pairs, %.1f%% of them covered by the lexicon; "+
		"%.1f%% of the covered ones supported on clean text\n",
		scoredPairs, 100*float64(coveredPairs)/float64(max(1, scoredPairs)),
		100*float64(supportedPairs)/float64(max(1, coveredPairs)))

	// Threshold sweep: originals = negatives; fully- and half-shifted copies =
	// positives (half-shift models a cascade that starts mid-sentence).
	rep := report{
		Book: flag.Arg(0), Source: *src, Target: *tgt,
		LexiconEntries: dict.Entries(), Sentences: len(sentences),
		Negatives: len(clean), DroppedForeign: dropped,
		ScoredPairs: scoredPairs, CoveredPairs: coveredPairs, SupportedPairs: supportedPairs,
		PairCoverage: float64(coveredPairs) / float64(max(1, scoredPairs)),
		SupportRate:  float64(supportedPairs) / float64(max(1, coveredPairs)),
	}
	fmt.Printf("\n%-30s %11s %11s %8s %9s\n", "thresholds", "recall/full", "recall/half", "FP-rate", "precision")
	for _, minCov := range coveredGrid {
		for _, rate := range rateGrid {
			for _, shiftHits := range shiftGrid {
				var fp, tpF, tpH int
				for _, sc := range set {
					if verdict(sc.orig, minCov, rate, shiftHits) {
						fp++
					}
					if verdict(sc.full, minCov, rate, shiftHits) {
						tpF++
					}
					if verdict(sc.half, minCov, rate, shiftHits) {
						tpH++
					}
				}
				n := len(set)
				p := point{
					MinCoveredPairs: minCov, LowSupportRate: rate, MinShiftHits: shiftHits,
					Current: minCov == lexcheck.MinCoveredPairs &&
						rate == lexcheck.LowSupportRate && shiftHits == lexcheck.MinShiftHits,
					RecallFull: float64(tpF) / float64(n),
					RecallHalf: float64(tpH) / float64(n),
					FPRate:     float64(fp) / float64(n),
					Precision:  float64(tpF) / float64(max(1, tpF+fp)),
				}
				rep.Grid = append(rep.Grid, p)
				mark := ""
				if p.Current {
					mark = "  ← current"
				}
				fmt.Printf("covered≥%d support≤%.2f shift≥%d %10.1f%% %10.1f%% %7.1f%% %8.1f%%%s\n",
					minCov, rate, shiftHits, 100*p.RecallFull, 100*p.RecallHalf,
					100*p.FPRate, 100*p.Precision, mark)
			}
		}
	}

	if *examples > 0 {
		fmt.Printf("\n=== missed synthetic full-cascade shifts (current thresholds) ===\n")
		n := 0
		for _, sc := range set {
			if n >= *examples || sc.full.Flagged {
				continue
			}
			n++
			showCase(dict, shifted(sc.s, *tgt, 0), sc.s, *tgt, sc.full)
		}
		fmt.Printf("\n=== false positives on unshifted text (current thresholds) ===\n")
		n = 0
		for _, sc := range set {
			if n >= *examples || !sc.orig.Flagged {
				continue
			}
			n++
			showCase(dict, sc.s, sc.s, *tgt, sc.orig)
		}
	}

	if *jsonOut != "" {
		b, err := json.MarshalIndent(rep, "", "  ")
		if err == nil {
			err = os.WriteFile(*jsonOut, append(b, '\n'), 0o644)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "write json:", err)
			os.Exit(1)
		}
	}
}

// reportContamination cross-tabulates langcheck's wrong-language verdict with
// lexcheck's flag on the same cells: the wrong-language blind spot, measured.
// Foreign-script cells are split by how much of the cell is foreign, because a
// whole leaked sentence and a single leaked name are different defects for a
// detector that aggregates per-pair evidence.
func reportContamination(dict *lexcheck.Dict, sentences []*tbook.Sentence, src, tgt string) {
	const (
		bucketWhole = "wrong-language (≥50% foreign)"
		bucketToken = "wrong-language (<50% foreign)"
		bucketUntr  = "untranslated"
		bucketRest  = "rest"
	)
	order := []string{bucketWhole, bucketToken, bucketUntr, bucketRest}
	bucketOf := map[string]string{}
	for _, f := range langcheck.Check(sentences, src, []string{tgt}, langcheck.Options{}) {
		if _, dup := bucketOf[f.Src]; dup {
			continue
		}
		switch {
		case f.Kind == langcheck.KindForeignScript && f.Share >= 0.5:
			bucketOf[f.Src] = bucketWhole
		case f.Kind == langcheck.KindForeignScript:
			bucketOf[f.Src] = bucketToken
		default:
			bucketOf[f.Src] = bucketUntr
		}
	}
	total := 0
	count := map[string]int{}
	flagged := map[string]int{}
	for _, s := range sentences {
		tr, ok := s.Tr[tgt]
		if !ok || tr.Text == "" || len(tr.Align) == 0 {
			continue
		}
		total++
		b, ok := bucketOf[s.Src]
		if !ok {
			b = bucketRest
		}
		count[b]++
		if dict.CheckSentence(s, tgt).Flagged {
			flagged[b]++
		}
	}
	pct := func(a, b int) float64 { return 100 * float64(a) / float64(max(1, b)) }
	fmt.Printf("\n%s→%s: %d translated sentences\n", src, tgt, total)
	fmt.Printf("%-30s %9s %8s %10s %9s\n", "langcheck verdict", "sentences", "share", "lex-flagged", "flag-rate")
	for _, b := range order {
		fmt.Printf("%-30s %9d %7.1f%% %10d %8.1f%%\n",
			b, count[b], pct(count[b], total), flagged[b], pct(flagged[b], count[b]))
	}
}

// showCase prints one sentence with its per-pair dictionary evidence.
// check is the sentence lexcheck scored, orig supplies the untouched reference.
func showCase(dict *lexcheck.Dict, check, orig *tbook.Sentence, tgt string, r lexcheck.Result) {
	fmt.Printf("\nSRC: %s\nTR : %s\n", orig.Src, orig.Tr[tgt].Text)
	fmt.Printf("     scored=%d covered=%d supported=%d shiftHits=%d flagged=%v %s\n",
		r.Scored, r.Covered, r.Supported, r.ShiftHits, r.Flagged, r.Reason)
	for _, line := range pairs(dict, check, tgt) {
		fmt.Println("  " + line)
	}
}

// pairs renders each align chunk with the evidence lexcheck actually used:
// [ok] supported, [??] covered but unsupported, [→N] fits the neighbor word
// instead (the shift signature), [--] scored but the dictionary knows none of
// its source words, [..] not scored at all (inserted chunk, or a fragment
// shorter than the three-letter floor). The filters mirror CheckSentence.
func pairs(dict *lexcheck.Dict, s *tbook.Sentence, tgt string) []string {
	tr := s.Tr[tgt]
	srcRunes := []rune(s.Src)
	trRunes := []rune(tr.Text)
	word := func(i int) string {
		if i < 0 || i >= len(s.Words) {
			return ""
		}
		a, b := s.Words[i][0], s.Words[i][1]
		if a < 0 || b > len(srcRunes) || a >= b {
			return ""
		}
		return string(srcRunes[a:b])
	}
	var out []string
	for _, c := range tr.Align {
		if c.T[0] < 0 || c.T[1] > len(trRunes) || c.T[0] >= c.T[1] {
			continue
		}
		frag := string(trRunes[c.T[0]:c.T[1]])
		var ws []string
		for _, wi := range c.W {
			ws = append(ws, fmt.Sprintf("%d:%s", wi, word(wi)))
		}
		mark := ".."
		if len(c.W) > 0 && letters(frag) >= 3 {
			mark = "--"
			for _, wi := range c.W {
				w := word(wi)
				if letters(w) < 3 || !dict.Covered(w) {
					continue
				}
				mark = "??"
				if dict.Supports(w, frag) {
					mark = "ok"
					break
				}
			}
			if mark == "??" {
				lo, hi := c.W[0], c.W[len(c.W)-1]
				for _, ni := range []int{lo - 1, hi + 1} {
					if w := word(ni); w != "" && dict.Covered(w) && dict.Supports(w, frag) {
						mark = "→" + strconv.Itoa(ni)
						break
					}
				}
			}
		}
		out = append(out, fmt.Sprintf("[%s] %q ← {%s}", mark, frag, strings.Join(ws, ", ")))
	}
	return out
}

// letters counts the letters in a word, the way lexcheck folds them.
func letters(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

// verdict applies a threshold triple to a lexcheck result.
func verdict(r lexcheck.Result, minCovered int, rate float64, shiftHits int) bool {
	if r.Covered >= minCovered && r.SupportRate() <= rate {
		return true
	}
	return r.ShiftHits >= shiftHits
}

// shifted returns a copy of s whose align mappings point one source word to
// the right — synthetic positional drift. divide=0 shifts every chunk (full
// cascade); divide=2 shifts only the last half (a cascade starting mid-sentence).
func shifted(s *tbook.Sentence, target string, divide int) *tbook.Sentence {
	tr := s.Tr[target]
	al := make([]tbook.AlignChunk, len(tr.Align))
	n := max(1, len(s.Words))
	from := 0
	if divide > 0 {
		from = len(tr.Align) - len(tr.Align)/divide
	}
	for i, c := range tr.Align {
		if i < from {
			al[i] = c
			continue
		}
		w := make([]int, len(c.W))
		for j, wi := range c.W {
			w[j] = (wi + 1) % n
		}
		al[i] = tbook.AlignChunk{T: c.T, W: w}
	}
	cp := *s
	cp.Tr = map[string]tbook.Translation{target: {Text: tr.Text, Align: al}}
	return &cp
}

func floats(list string) ([]float64, error) {
	var out []float64
	for _, f := range strings.Split(list, ",") {
		v, err := strconv.ParseFloat(strings.TrimSpace(f), 64)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	sort.Float64s(out)
	return out, nil
}

func ints(list string) ([]int, error) {
	var out []int
	for _, f := range strings.Split(list, ",") {
		v, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	sort.Ints(out)
	return out, nil
}

func readBook(path string) ([]*tbook.Sentence, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	byName := map[string]*zip.File{}
	for _, f := range zr.File {
		byName[f.Name] = f
	}
	readJSON := func(name string, v any) error {
		f := byName[name]
		if f == nil {
			return fmt.Errorf("missing entry %s", name)
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		return json.NewDecoder(rc).Decode(v)
	}
	var man tbook.Manifest
	if err := readJSON("manifest.json", &man); err != nil {
		return nil, err
	}
	var out []*tbook.Sentence
	for _, ch := range man.Chapters {
		var payload chapterPayload
		if err := readJSON(ch.File, &payload); err != nil {
			return nil, err
		}
		for _, para := range payload.Paragraphs {
			out = append(out, para...)
		}
		for _, t := range payload.Tables {
			for _, row := range t.Rows {
				for _, cell := range row {
					out = append(out, cell.Sentences...)
				}
			}
		}
	}
	return out, nil
}
