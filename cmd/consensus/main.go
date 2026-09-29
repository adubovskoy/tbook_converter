// Command consensus measures cross-target agreement in a multi-target .tbook,
// offline and with no API calls. The pivot architecture gives the file N
// independent alignments of the same source sentence, so for every source-word
// slot it already records which languages highlight it and which do not:
//
//	unanimous       every aligned language highlights the slot
//	solo miss       exactly one language fails where all the others succeed —
//	                a per-language aligner regression signal
//	unanimous miss  no language highlights it — usually a hard sense every
//	                language did translate and none could place ("a deal table")
//
//	consensus book.tbook [--content-only] [--min-langs 6] [--json out.json]
//
// A book with fewer than three targets is refused: two opinions cannot single
// anyone out. --min-langs is how many aligned languages a sentence needs before
// it votes (default 3; the panel's published numbers for the 8-language
// sample.tbook were taken at 6). --content-only drops closed-class source words,
// which otherwise dominate solo misses — an article legitimately absorbed into
// its noun's chunk is not a defect.
//
// The metric says nothing about a fluent mistranslation; it finds the class the
// judge calls "wrong-mapping" and coverage cannot see. Both format versions are
// accepted.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dimando/reader/converter/internal/learner"
)

func main() {
	contentOnly := flag.Bool("content-only", false, "count content-class slots only (function words removed)")
	minLangs := flag.Int("min-langs", learner.MinTargets, "aligned languages a sentence needs to vote")
	jsonOut := flag.String("json", "", "write the full report as JSON to this file")
	flag.Usage = usage

	args := os.Args[1:]
	book := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		book, args = args[0], args[1:]
	}
	flag.CommandLine.Parse(args)
	if book == "" && flag.NArg() == 1 {
		book = flag.Arg(0)
	}
	if book == "" {
		usage()
		os.Exit(2)
	}
	if err := run(book, *contentOnly, *minLangs, *jsonOut); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: consensus book.tbook [--content-only] [--min-langs 6] [--json out.json]

For a .tbook with at least three targets (format version 1 or 2), reports per
source-word slot which languages highlight it:

  unanimous       every aligned language highlights the slot
  solo miss       exactly one language fails where all others succeed — the
                  per-language aligner regression signal
  unanimous miss  no language highlights it, though every one translated the
                  word: a hard sense none could place

--content-only drops closed-class source words (a 64-word English function-word
list); solo misses are otherwise dominated by articles and prepositions that are
legitimately absorbed into a neighbouring chunk. --min-langs is how many aligned
languages a sentence needs before it votes.

flags:
`)
	flag.PrintDefaults()
}

func run(path string, contentOnly bool, minLangs int, jsonOut string) error {
	b, err := learner.Load(path)
	if err != nil {
		return err
	}
	rep, err := b.Consensus(learner.ConsensusOpts{MinLangs: minLangs, ContentOnly: contentOnly})
	if err != nil {
		return err
	}
	printReport(rep)
	if jsonOut != "" {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(jsonOut, append(data, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Printf("\nJSON report: %s\n", jsonOut)
	}
	return nil
}

func printReport(r *learner.Consensus) {
	scope := "all source-word slots"
	if r.ContentOnly {
		scope = "content-class slots only"
	}
	fmt.Printf("cross-target consensus — %s (formatVersion %d, targets %s)\n",
		r.Book, r.FormatVersion, strings.Join(r.Targets, " "))
	fmt.Printf("%d sentences with ≥ %d aligned languages, %d (sentence, language) cells, %d slots examined (%s)\n\n",
		r.Sentences, r.MinLangs, r.Cells, r.Slots, scope)
	if r.Slots == 0 {
		fmt.Printf("no sentence reaches %d aligned languages — lower --min-langs\n", r.MinLangs)
		return
	}
	fmt.Printf("unanimous (every language highlights the slot): %d (%.2f%%)\n", r.Unanimous, r.UnanimousPct)
	fmt.Printf("missed by at least one language:               %d (%.2f%%)\n\n", r.Missed, r.MissedPct)

	fmt.Println("languages leaving a slot unhighlighted:")
	ks := make([]int, 0, len(r.Hist))
	for k := range r.Hist {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	for _, k := range ks {
		fmt.Printf("  k=%-2d %8d  (%6.2f%% of slots)\n", k, r.Hist[k], 100*float64(r.Hist[k])/float64(r.Slots))
	}
	fmt.Printf("\nsolo misses (one language fails where all others succeed): %d = %.2f%% of slots, %.1f%% of misses\n",
		r.Solo, r.SoloPct, r.SoloOfMissedPct)
	fmt.Printf("sentences carrying ≥ 1 solo miss: %d/%d\n", r.SoloSentences, r.Sentences)
	fmt.Print("solo misses per language: ")
	type kv struct {
		lang string
		n    int
	}
	var by []kv
	for l, n := range r.SoloByLang {
		by = append(by, kv{l, n})
	}
	sort.Slice(by, func(i, j int) bool {
		if by[i].n != by[j].n {
			return by[i].n > by[j].n
		}
		return by[i].lang < by[j].lang
	})
	parts := make([]string, 0, len(by))
	for _, e := range by {
		parts = append(parts, fmt.Sprintf("%s %d", e.lang, e.n))
	}
	fmt.Println(strings.Join(parts, ", "))
	fmt.Printf("\nunanimous misses (no language highlights the slot): %d slots, %d distinct surfaces\n",
		r.UnanimousMiss, r.UnanimousDistinct)
	fmt.Printf("  top surfaces: %s\n", surfaces(r.TopUnanimousMiss))
	fmt.Printf("  top solo surfaces: %s\n", surfaces(r.TopSolo))
}

func surfaces(s []learner.Surface) string {
	if len(s) == 0 {
		return "(none)"
	}
	parts := make([]string, len(s))
	for i, e := range s {
		parts[i] = fmt.Sprintf("%s %d", e.Word, e.N)
	}
	return strings.Join(parts, ", ")
}
