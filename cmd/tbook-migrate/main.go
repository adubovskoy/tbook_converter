// Command tbook-migrate converts a version-1 .tbook into a version-2 .tbook
// (spec Annex A) and validates the result.
//
//	tbook-migrate in.tbook out.tbook
package main

import (
	"flag"
	"fmt"
	"os"

	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
	"github.com/dimando/reader/converter/internal/tbook/v2/migrate"
)

func main() {
	noValidate := flag.Bool("no-validate", false, "skip validating the written file")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: tbook-migrate [-no-validate] in.tbook out.tbook")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	in, out := flag.Arg(0), flag.Arg(1)
	stats, err := migrate.File(in, out, migrate.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fi, _ := os.Stat(out)
	fmt.Printf("%s → %s (%d bytes)\n", in, out, fi.Size())
	fmt.Printf("  %d sentences, %d translations, %d chunks: %d token links, %d merged, %d escapes (%d folded), %d orphaned, %d cross-sentence refs dropped, %d digit pass-through, %d unaligned, %d mid-word joins\n",
		stats.Sentences, stats.Translations, stats.Chunks, stats.TokenLinks, stats.MergedSameToken,
		stats.Escapes, stats.EscapesMerged, stats.Orphaned, stats.CrossSentenceRefs,
		stats.DigitPassthrough, stats.Unaligned, stats.JoinsWithoutSpace)
	if *noValidate {
		return
	}
	rep, err := v2.Validate(out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	for _, w := range rep.Warnings {
		fmt.Println("  warning:", w)
	}
	if !rep.OK() {
		for _, v := range rep.Violations {
			fmt.Println("  VIOLATION:", v)
		}
		os.Exit(1)
	}
	fmt.Println("  valid")
}
