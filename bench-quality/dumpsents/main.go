// Throwaway research tool: dump a book's sentences with the production parser
// and segmenter, so an offline analysis sees exactly what convert would.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dimando/reader/converter/internal/epub"
	"github.com/dimando/reader/converter/internal/fb2"
	"github.com/dimando/reader/converter/internal/segment"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: dumpsents BOOK OUT.json SRCLANG")
		os.Exit(2)
	}
	in, out, lang := os.Args[1], os.Args[2], os.Args[3]
	var book *epub.Book
	var err error
	if l := strings.ToLower(in); strings.HasSuffix(l, ".fb2") || strings.HasSuffix(l, ".fb2.zip") {
		book, err = fb2.Parse(in)
	} else {
		book, err = epub.ParseOpts(in, epub.Options{SkipMatter: true})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse:", err)
		os.Exit(1)
	}
	_, sents := segment.BuildSentenceObjects(book.Chapters, lang)
	type row struct {
		Src   string  `json:"src"`
		Words [][]int `json:"words"`
	}
	rows := make([]row, 0, len(sents))
	for _, s := range sents {
		w := make([][]int, 0, len(s.Words))
		for _, x := range s.Words {
			w = append(w, []int{x[0], x[1]})
		}
		rows = append(rows, row{Src: s.Src, Words: w})
	}
	b, _ := json.Marshal(map[string]any{"title": book.Title, "author": book.Author, "sentences": rows})
	if err := os.WriteFile(out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s by %s — %d sentences -> %s\n", book.Title, book.Author, len(rows), out)
}
