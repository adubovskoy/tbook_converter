// Command tbook-validate checks version-2 .tbook files against every rule of
// spec §11.1 and exits non-zero listing the violations.
//
//	tbook-validate book.tbook [more.tbook ...]
package main

import (
	"flag"
	"fmt"
	"os"

	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
)

func main() {
	quiet := flag.Bool("q", false, "do not print warnings")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: tbook-validate [-q] book.tbook [more.tbook ...]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	failed := false
	for _, path := range flag.Args() {
		rep, err := v2.Validate(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: error: %v\n", path, err)
			failed = true
			continue
		}
		if !*quiet {
			for _, w := range rep.Warnings {
				fmt.Printf("%s: warning: %s\n", path, w)
			}
		}
		for _, v := range rep.Violations {
			fmt.Printf("%s: %s\n", path, v)
		}
		if rep.OK() {
			fmt.Printf("%s: valid (%d warnings)\n", path, len(rep.Warnings))
		} else {
			fmt.Printf("%s: INVALID — %d violations\n", path, len(rep.Violations))
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}
