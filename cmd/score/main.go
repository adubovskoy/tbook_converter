// Command score grades a single produced .tbook on alignment health for the
// actual reading experience — offline, no API calls. The reader taps a SOURCE
// word and sees the matching target word(s) highlighted, so both directions
// matter and are reported separately:
//
//   - target coverage — share of translation words overlapped by ≥1 align
//     chunk (what fraction of the highlighted sentence can light up);
//   - source tap coverage — share of source words referenced by ≥1 chunk's W
//     (what fraction of taps highlight anything at all).
//
// Both are reported over all words and over content words only (stopword
// lists from internal/stopwords; function words ship unaligned harmlessly).
//
//	score book.tbook -t ru [--probe probe.json] [--dump out.json] [--min-words 4]
//
// Sentences shorter than --min-words source words are still scored (and land
// in --dump) but are excluded from the aggregates, where trivial two-word
// sentences would inflate the numbers.
//
// Probe mode checks whether known multi-word expressions (idioms, phrasal
// verbs) align as one unit: --probe takes a JSON array of
// {"src": "<exact sentence text>", "expr": "<expression as written in src>"}
// and each matched item gets a verdict — "unit" (every expression word
// referenced and the referencing chunks form one compact target group),
// "partial" (some words referenced, or the chunks are scattered), or
// "unaligned" (no expression word referenced).
//
// --dump writes the aggregates plus per-sentence and per-probe records as
// JSON for downstream analysis.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/stopwords"
	"github.com/dimando/reader/converter/internal/tbook"
)

func main() {
	tgt := flag.String("t", "ru", "target language to score")
	probePath := flag.String("probe", "", "probe JSON: [{\"src\":..., \"expr\":...}]")
	dumpPath := flag.String("dump", "", "write full JSON report to this file")
	minWords := flag.Int("min-words", 4, "exclude sentences with fewer source words from aggregates")

	// Accept the book path before the flags (score book.tbook -t ru) as well
	// as after them — stdlib flag stops at the first non-flag argument.
	args := os.Args[1:]
	book := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		book, args = args[0], args[1:]
	}
	flag.CommandLine.Parse(args)
	switch {
	case book == "" && flag.NArg() == 1:
		book = flag.Arg(0)
	case book != "" && flag.NArg() == 0:
	default:
		fmt.Fprintln(os.Stderr, "usage: score book.tbook [-t ru] [--probe probe.json] [--dump out.json] [--min-words 4]")
		os.Exit(2)
	}
	if err := run(book, *tgt, *probePath, *dumpPath, *minWords); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(path, tgt, probePath, dumpPath string, minWords int) error {
	b, err := tbook.Read(path)
	if err != nil {
		return fmt.Errorf("read book: %w", err)
	}
	found := false
	for _, t := range b.Targets {
		if t == tgt {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("target %q not in book (targets: %s)", tgt, strings.Join(b.Targets, ", "))
	}
	srcStop := stopwords.Set(b.Source)
	tgtStop := stopwords.Set(tgt)
	if srcStop == nil {
		fmt.Printf("no %s stopword list: source content metrics equal all-word metrics\n", b.Source)
	}
	if tgtStop == nil {
		fmt.Printf("no %s stopword list: target content metrics equal all-word metrics\n", tgt)
	}

	prose, noteSents, citeSents := b.Sentences()
	sentences := append(append(prose, noteSents...), citeSents...)

	d := dump{Book: path, Source: b.Source, Target: tgt, MinWords: minWords}
	var covAll, covContent, tapAll, tapContent []float64
	for _, s := range sentences {
		if len(s.Words) == 0 {
			continue
		}
		d.Totals.Sentences++
		tr := s.Tr[tgt]
		if tr.Text == "" {
			d.Totals.EmptyTranslations++
			continue
		}
		if len(tr.Align) == 0 {
			d.Totals.ZeroAlignment++
		}
		rec := scoreSentence(s, tr, srcStop, tgtStop)
		d.Sentences = append(d.Sentences, rec)
		if len(s.Words) < minWords {
			continue
		}
		d.Totals.Aggregated++
		covAll = append(covAll, rec.CovAll)
		tapAll = append(tapAll, rec.TapAll)
		if rec.CovContent >= 0 {
			covContent = append(covContent, rec.CovContent)
		}
		if rec.TapContent >= 0 {
			tapContent = append(tapContent, rec.TapContent)
		}
	}
	d.TargetCoverage = aggregate(covAll, covContent, 0.7)
	d.SourceTap = aggregate(tapAll, tapContent, 0.8)
	d.SourceTap.ContentDeciles = deciles(tapContent)

	fmt.Printf("book: %s\n", path)
	fmt.Printf("%q by %s, %s → %s\n", b.Title, b.Author, b.Source, tgt)
	fmt.Printf("sentences: %d total, %d empty translation, %d zero alignment, %d aggregated (min-words %d)\n",
		d.Totals.Sentences, d.Totals.EmptyTranslations, d.Totals.ZeroAlignment, d.Totals.Aggregated, minWords)
	fmt.Println("\ntarget coverage (translation words highlighted by ≥1 chunk):")
	printAgg(d.TargetCoverage, "content coverage")
	fmt.Println("\nsource tap coverage (source words that highlight anything on tap):")
	printAgg(d.SourceTap, "content tap coverage")
	fmt.Printf("  content tap histogram (deciles 0.0→1.0): %s\n", joinInts(d.SourceTap.ContentDeciles))

	if probePath != "" {
		recs, err := runProbes(probePath, sentences, tgt, tgtStop)
		if err != nil {
			return err
		}
		d.Probes = recs
	}

	if dumpPath != "" {
		f, err := os.Create(dumpPath)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(f)
		enc.SetIndent("", " ")
		if err := enc.Encode(d); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		fmt.Printf("\ndump written: %s (%d sentence records)\n", dumpPath, len(d.Sentences))
	}
	return nil
}

// dump is the --dump JSON layout: aggregates plus per-sentence records and,
// in probe mode, per-probe records.
type dump struct {
	Book           string           `json:"book"`
	Source         string           `json:"source"`
	Target         string           `json:"target"`
	MinWords       int              `json:"minWords"`
	Totals         totals           `json:"totals"`
	TargetCoverage agg              `json:"targetCoverage"`
	SourceTap      agg              `json:"sourceTap"`
	Sentences      []sentenceRecord `json:"sentences"`
	Probes         []probeRecord    `json:"probes,omitempty"`
}

type totals struct {
	Sentences         int `json:"sentences"`
	EmptyTranslations int `json:"emptyTranslations"`
	ZeroAlignment     int `json:"zeroAlignment"`
	Aggregated        int `json:"aggregated"`
}

// agg summarises one direction. Below counts aggregate sentences whose
// content ratio fell under the direction's threshold (0.7 target coverage,
// 0.8 source tap); ContentN is that share's denominator — sentences that have
// ≥1 content word on that side.
type agg struct {
	AllMean        float64 `json:"allMean"`
	AllMedian      float64 `json:"allMedian"`
	ContentMean    float64 `json:"contentMean"`
	ContentMedian  float64 `json:"contentMedian"`
	ContentN       int     `json:"contentN"`
	Threshold      float64 `json:"threshold"`
	Below          int     `json:"below"`
	ContentDeciles []int   `json:"contentDeciles,omitempty"`
}

// sentenceRecord is one sentence's scores. Content ratios are -1 when the
// sentence has no content words on that side (nothing to measure).
type sentenceRecord struct {
	Src                 string   `json:"src"`
	CovAll              float64  `json:"covAll"`
	CovContent          float64  `json:"covContent"`
	TapAll              float64  `json:"tapAll"`
	TapContent          float64  `json:"tapContent"`
	UncoveredSrcContent []string `json:"uncoveredSrcContent"`
	UncoveredTgtContent []string `json:"uncoveredTgtContent"`
}

// scoreSentence computes both directions for one sentence: which target words
// an align chunk overlaps, and which source words any chunk references.
func scoreSentence(s *tbook.Sentence, tr tbook.Translation, srcStop, tgtStop map[string]bool) sentenceRecord {
	rec := sentenceRecord{Src: s.Src, UncoveredSrcContent: []string{}, UncoveredTgtContent: []string{}}

	// Target side: a translation word counts as covered when ≥1 chunk's rune
	// span [t0,t1) overlaps the word's span.
	trRunes := []rune(tr.Text)
	tgtWords := segment.Tokenize(tr.Text)
	covered := make([]bool, len(tgtWords))
	for _, c := range tr.Align {
		a, b := max(c.T[0], 0), min(c.T[1], len(trRunes))
		for i, w := range tgtWords {
			if a < w[1] && w[0] < b {
				covered[i] = true
			}
		}
	}
	var nAll, nCov, nContent, nContentCov int
	for i, w := range tgtWords {
		word := string(trRunes[w[0]:w[1]])
		content := !tgtStop[strings.ToLower(word)]
		nAll++
		if content {
			nContent++
		}
		if covered[i] {
			nCov++
			if content {
				nContentCov++
			}
		} else if content {
			rec.UncoveredTgtContent = append(rec.UncoveredTgtContent, word)
		}
	}
	rec.CovAll = ratio(nCov, nAll)
	rec.CovContent = ratio(nContentCov, nContent)

	// Source side: a source word counts as tappable when ≥1 chunk lists its
	// index in W.
	srcRunes := []rune(s.Src)
	tapped := make([]bool, len(s.Words))
	for _, c := range tr.Align {
		for _, wi := range c.W {
			if wi >= 0 && wi < len(tapped) {
				tapped[wi] = true
			}
		}
	}
	nAll, nCov, nContent, nContentCov = 0, 0, 0, 0
	for i, w := range s.Words {
		word := string(srcRunes[w[0]:w[1]])
		content := !srcStop[strings.ToLower(word)]
		nAll++
		if content {
			nContent++
		}
		if tapped[i] {
			nCov++
			if content {
				nContentCov++
			}
		} else if content {
			rec.UncoveredSrcContent = append(rec.UncoveredSrcContent, word)
		}
	}
	rec.TapAll = ratio(nCov, nAll)
	rec.TapContent = ratio(nContentCov, nContent)
	return rec
}

// Probe mode.

type probeItem struct {
	Src  string `json:"src"`
	Expr string `json:"expr"`
}

// probeRecord is one probe's outcome. Verdicts: unit / partial / unaligned
// for evaluated items; src-not-found / expr-not-found when the sentence or
// the expression inside it could not be located.
type probeRecord struct {
	Src          string   `json:"src"`
	Expr         string   `json:"expr"`
	Verdict      string   `json:"verdict"`
	ExprWordIdx  []int    `json:"exprWordIdx"`
	TgtFragments []string `json:"tgtFragments"`
}

func runProbes(path string, sentences []*tbook.Sentence, tgt string, tgtStop map[string]bool) ([]probeRecord, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read probe file: %w", err)
	}
	var items []probeItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parse probe file: %w", err)
	}
	bySrc := make(map[string]*tbook.Sentence, len(sentences))
	for _, s := range sentences {
		if _, ok := bySrc[s.Src]; !ok {
			bySrc[s.Src] = s
		}
	}

	recs := make([]probeRecord, 0, len(items))
	counts := map[string]int{}
	for _, it := range items {
		rec := probeRecord{Src: it.Src, Expr: it.Expr, ExprWordIdx: []int{}, TgtFragments: []string{}}
		if s := bySrc[it.Src]; s == nil {
			rec.Verdict = "src-not-found"
		} else {
			rec = evalProbe(s, tgt, it, tgtStop)
		}
		counts[rec.Verdict]++
		recs = append(recs, rec)
	}

	matched := counts["unit"] + counts["partial"] + counts["unaligned"]
	fmt.Printf("\nprobes: %d loaded, %d matched (%d src not found, %d expr not located)\n",
		len(items), matched, counts["src-not-found"], counts["expr-not-found"])
	for _, v := range []string{"unit", "partial", "unaligned"} {
		fmt.Printf("  %-9s %d (%.1f%%)\n", v+":", counts[v], pct(counts[v], matched))
	}
	for _, r := range recs {
		fmt.Printf("  [%-13s] %q → %q\n", r.Verdict, r.Expr, strings.Join(r.TgtFragments, " | "))
	}
	return recs, nil
}

// evalProbe locates the expression's word indices in the sentence and checks
// that its target rendering is one compact group: every expression word
// referenced, the referencing chunks contiguous in the translation (gaps of
// ≤1 non-content target word allowed), and no referencing chunk more than 2
// chunks away from the rest in target order.
func evalProbe(s *tbook.Sentence, tgt string, it probeItem, tgtStop map[string]bool) probeRecord {
	rec := probeRecord{Src: it.Src, Expr: it.Expr, ExprWordIdx: []int{}, TgtFragments: []string{}}
	srcRunes := []rune(s.Src)
	srcWords := make([]string, len(s.Words))
	for i, w := range s.Words {
		srcWords[i] = strings.ToLower(string(srcRunes[w[0]:w[1]]))
	}
	exprRunes := []rune(it.Expr)
	var exprWords []string
	for _, w := range segment.Tokenize(it.Expr) {
		exprWords = append(exprWords, strings.ToLower(string(exprRunes[w[0]:w[1]])))
	}
	start := matchSeq(srcWords, exprWords)
	if start < 0 {
		rec.Verdict = "expr-not-found"
		return rec
	}
	isExpr := make(map[int]bool, len(exprWords))
	for i := range exprWords {
		rec.ExprWordIdx = append(rec.ExprWordIdx, start+i)
		isExpr[start+i] = true
	}

	// Referencing chunks, evaluated in target order over the valid chunks.
	tr := s.Tr[tgt]
	trRunes := []rune(tr.Text)
	type chunk struct {
		t0, t1 int
		refs   bool
	}
	var chunks []chunk
	referenced := map[int]bool{}
	for _, c := range tr.Align {
		a, b := max(c.T[0], 0), min(c.T[1], len(trRunes))
		if a >= b {
			continue
		}
		ck := chunk{t0: a, t1: b}
		for _, wi := range c.W {
			if isExpr[wi] {
				ck.refs = true
				referenced[wi] = true
			}
		}
		chunks = append(chunks, ck)
	}
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].t0 < chunks[j].t0 })
	var refIdx []int
	for i, c := range chunks {
		if c.refs {
			refIdx = append(refIdx, i)
			rec.TgtFragments = append(rec.TgtFragments, string(trRunes[c.t0:c.t1]))
		}
	}
	if len(refIdx) == 0 {
		rec.Verdict = "unaligned"
		return rec
	}

	complete := len(referenced) == len(exprWords)
	compact := true
	tgtWords := segment.Tokenize(tr.Text)
	for k := 1; k < len(refIdx); k++ {
		prev, next := chunks[refIdx[k-1]], chunks[refIdx[k]]
		// No referencing chunk may sit >2 chunks away from the previous one.
		if refIdx[k]-refIdx[k-1] > 3 {
			compact = false
			break
		}
		// Target words wholly inside the gap: at most one, and a non-content
		// word at that ("держал их в узде" may keep an unaligned "в").
		var gap []string
		for _, w := range tgtWords {
			if w[0] >= prev.t1 && w[1] <= next.t0 {
				gap = append(gap, strings.ToLower(string(trRunes[w[0]:w[1]])))
			}
		}
		if len(gap) > 1 || (len(gap) == 1 && !tgtStop[gap[0]]) {
			compact = false
			break
		}
	}
	switch {
	case complete && compact:
		rec.Verdict = "unit"
	default:
		rec.Verdict = "partial"
	}
	return rec
}

// matchSeq returns the first index where seq occurs contiguously in words,
// or -1.
func matchSeq(words, seq []string) int {
	if len(seq) == 0 {
		return -1
	}
	for i := 0; i+len(seq) <= len(words); i++ {
		ok := true
		for j, w := range seq {
			if words[i+j] != w {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

// Aggregation helpers.

func aggregate(all, content []float64, threshold float64) agg {
	a := agg{
		AllMean: mean(all), AllMedian: median(all),
		ContentMean: mean(content), ContentMedian: median(content),
		ContentN: len(content), Threshold: threshold,
	}
	for _, v := range content {
		if v < threshold {
			a.Below++
		}
	}
	return a
}

func printAgg(a agg, label string) {
	fmt.Printf("  all words:      mean %.3f  median %.3f\n", a.AllMean, a.AllMedian)
	fmt.Printf("  content words:  mean %.3f  median %.3f\n", a.ContentMean, a.ContentMedian)
	fmt.Printf("  sentences with %s < %.2f: %d/%d (%.1f%%)\n",
		label, a.Threshold, a.Below, a.ContentN, pct(a.Below, a.ContentN))
}

// deciles buckets values into [0,.1) … [.9,1] and returns the ten counts.
func deciles(xs []float64) []int {
	out := make([]int, 10)
	for _, x := range xs {
		out[min(max(int(x*10), 0), 9)]++
	}
	return out
}

func joinInts(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprint(x)
	}
	return strings.Join(parts, " ")
}

// ratio returns covered/total, or -1 when there is nothing to measure.
func ratio(covered, total int) float64 {
	if total == 0 {
		return -1
	}
	return float64(covered) / float64(total)
}

func pct(a, n int) float64 { return 100 * float64(a) / float64(max(1, n)) }

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	c := append([]float64(nil), xs...)
	sort.Float64s(c)
	return c[len(c)/2]
}
