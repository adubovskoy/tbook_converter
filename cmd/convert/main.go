// Command convert turns an EPUB or FB2 into a .tbook language-learning
// archive, translating every sentence with word-level alignment via OpenRouter
// (metered) or the claude CLI (the user's Claude subscription).
//
//	convert book.epub -o book.tbook            # English → Russian (default)
//	convert book.epub --provider claude        # translate on the Claude subscription
//	convert book.fb2 -t ru,de -o book.tbook    # FB2 input, multiple targets
//	convert book.epub --dry-run                # parse + segment only, no API
//	convert book.epub --glossary --judge       # quality passes (see README)
//
// The implementation lives in internal/cli so the GUI binary can self-exec it.
package main

import (
	"os"

	"github.com/dimando/reader/converter/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
