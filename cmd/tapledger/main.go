// Command tapledger scores what a learner gets from one tap, offline and with
// no API calls. Every (source word, target language) tap of a .tbook is put in
// one of six classes, exactly as the reference consumers render it (spec §7 for
// version 1, §8.2/§8.5 for version 2):
//
//	clean         one chunk lights, it maps this word only, and it is one target token
//	clean-multi   ≥ 2 chunks light, each exclusive to this word (fr «ne … pas»)
//	shared        a lit chunk also maps other source words — the same token answers
//	              a different question too (tr «bekleyebilirim» ← I + can + wait)
//	smear         ≥ 3 target tokens light: a phrase-level highlight
//	none          no chunk contains the word: a silent tap, with no reason shown
//	dead-sentence the translation is there but its alignment is empty: every tap
//	              in that sentence is silent
//
// Priority when several apply: dead-sentence > none > smear > shared >
// clean-multi > clean. Taps in a sentence with no translation in that language
// are not counted (they appear in the coarse collapse below as
// no-translation).
//
// "content-word silent" is the share of content-word taps that light nothing
// (class none or dead-sentence). The content-word heuristic is the one the
// panel used: the tapped word has at least four letters and is not in an
// 86-word English stoplist (internal/learner/words.go). --content-only
// restricts the whole ledger to those taps.
//
// The coarse collapse printed after the table is the three-class version
// (clean / shared / dead) of the same taps: a tap is shared as soon as a token
// it lights is claimed by another source word, even when two separate chunks
// claim it, and its denominator includes taps in untranslated sentences.
//
//	tapledger book.tbook [--lang ru,de] [--content-only] [--json out.json]
//
// Both format versions are accepted; version 2 is scored through its own tap
// algorithm. The panel also cross-tabbed every tap with the consumers' .tdict
// lookup; that part is not ported — this tool needs no dictionary.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dimando/reader/converter/internal/learner"
)

func main() {
	langs := flag.String("lang", "", "comma-separated target languages (default: every target of the manifest)")
	contentOnly := flag.Bool("content-only", false, "score content-word taps only (≥ 4 letters, not in the stoplist)")
	jsonOut := flag.String("json", "", "write the full report as JSON to this file")
	flag.Usage = usage

	// The book path may come before the flags as well as after them.
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
	if err := run(book, *langs, *contentOnly, *jsonOut); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: tapledger book.tbook [--lang ru,de] [--content-only] [--json out.json]

Classifies every (source word, target language) tap of a .tbook (format version
1 or 2) into the six classes a reader gets:

  clean          one chunk lights, it maps this word only, and it is one token
  clean-multi    >= 2 chunks light, each exclusive to this word (fr "ne ... pas")
  shared         a lit chunk also maps other source words: the same token
                 answers a different question too
  smear          >= 3 target tokens light: a phrase-level highlight
  none           no chunk contains the word: a silent tap, no reason shown
  dead-sentence  the translation is there but its alignment is empty

Priority: dead-sentence > none > smear > shared > clean-multi > clean. Taps in a
sentence with no translation in that language are not counted in the ledger;
they appear in the coarse collapse as no-translation.

content-word heuristic (as used by the panel): the tapped word has at least four
letters and is not in an 86-word English stoplist. "content-word silent" is the
share of those taps in class none or dead-sentence; --content-only restricts the
whole ledger to them.

The panel cross-tabbed every tap with the consumers' .tdict lookup; that part is
not ported, so there is no --dict: this tool needs no dictionary.

flags:
`)
	flag.PrintDefaults()
}

func run(path, langs string, contentOnly bool, jsonOut string) error {
	b, err := learner.Load(path)
	if err != nil {
		return err
	}
	opts := learner.LedgerOpts{ContentOnly: contentOnly}
	if langs != "" {
		for _, l := range strings.Split(langs, ",") {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if !contains(b.Targets, l) {
				return fmt.Errorf("%s has no target %q (targets: %s)", path, l, strings.Join(b.Targets, ", "))
			}
			opts.Langs = append(opts.Langs, l)
		}
	}
	rep := b.TapLedger(opts)
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

func printReport(rep *learner.Ledger) {
	scope := "all taps"
	if rep.ContentOnly {
		scope = "content-word taps only"
	}
	fmt.Printf("tap ledger — %s (formatVersion %d, %d sentences, %s)\n\n",
		rep.Book, rep.FormatVersion, rep.Sentences, scope)
	fmt.Printf("%-6s %9s", "lang", "taps")
	for _, n := range learner.ClassNames {
		fmt.Printf(" %13s", n)
	}
	fmt.Printf(" %8s %9s\n", "silent", "cw-silent")
	for _, row := range append(append([]*learner.LangLedger{}, rep.Languages...), rep.Total) {
		fmt.Printf("%-6s %9d", row.Lang, row.Taps)
		for _, n := range learner.ClassNames {
			fmt.Printf(" %12.2f%%", row.Pct[n])
		}
		fmt.Printf(" %7.2f%% %8.2f%%\n", row.SilentPct, row.ContentSilentPct)
	}
	fmt.Println("\ncontent-word silent = share of content-word taps (≥ 4 letters, not a stopword) lighting nothing")
	for _, row := range rep.Languages {
		fmt.Printf("  %-4s %6d of %6d content taps silent; %d taps in a script-clean translation (%.2f%%)\n",
			row.Lang, row.ContentSilent, row.ContentTaps, row.ScriptCleanTaps, pctOf(row.ScriptCleanTaps, row.Taps))
	}
	fmt.Println("\ncoarse collapse (clean / shared / dead, untranslated taps included in the denominator)")
	for _, row := range append(append([]*learner.LangLedger{}, rep.Languages...), rep.Total) {
		c := row.Coarse
		fmt.Printf("  %-5s taps %8d  clean %6.2f%%  shared %6.2f%%  dead %5.2f%%  no-translation %5.2f%%\n",
			row.Lang, c.Taps, c.Pct["clean"], c.Pct["shared"], c.Pct["dead"], c.Pct["noTranslation"])
	}
}

func pctOf(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return 100 * float64(n) / float64(d)
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
