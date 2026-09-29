// Command stopwordsdump writes internal/stopwords' closed-class lists as JSON
// so non-Go consumers can share them. The only consumer today is
// tools/embalign.py's closed-class guard (EMBALIGN_METHOD=itermax-guarded),
// which runs as a child process and cannot link the Go package.
//
// Regenerate the checked-in copy with:
//
//	go run ./cmd/stopwordsdump -o tools/stopwords.json
//
// TestExportedFileMatches in internal/stopwords fails when the file drifts.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/dimando/reader/converter/internal/stopwords"
)

func main() {
	out := flag.String("o", "", "output file (default: stdout)")
	flag.Parse()

	m := map[string][]string{}
	for _, lang := range stopwords.Langs() {
		words := make([]string, 0, len(stopwords.Set(lang)))
		for w := range stopwords.Set(lang) {
			words = append(words, w)
		}
		sort.Strings(words)
		m[lang] = words
	}
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b = append(b, '\n')
	if *out == "" {
		os.Stdout.Write(b)
		return
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "%s: %d languages\n", *out, len(m))
}
