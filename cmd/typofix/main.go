// Command typofix applies the typography pass to one language of a .tbook
// offline and writes the changed cells as JSONL. It never rewrites the book.
//
//	typofix -lang ru -o changed.jsonl [-all all.jsonl] book.tbook
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/dimando/reader/converter/internal/tbook/v2/migrate"
	"github.com/dimando/reader/converter/internal/typography"
)

type row struct {
	ID     string   `json:"id"`
	Src    string   `json:"src"`
	Before string   `json:"before"`
	After  string   `json:"after"`
	Rules  []string `json:"rules,omitempty"`
}

func main() {
	lang := flag.String("lang", "", "target language")
	out := flag.String("o", "", "JSONL of changed cells")
	all := flag.String("all", "", "optional JSONL of every cell (changed or not)")
	flag.Parse()
	if flag.NArg() != 1 || *lang == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	b, _, err := migrate.ReadBook(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fo, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer fo.Close()
	eo := json.NewEncoder(fo)
	eo.SetEscapeHTML(false)
	var ea *json.Encoder
	if *all != "" {
		fa, err := os.Create(*all)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer fa.Close()
		ea = json.NewEncoder(fa)
		ea.SetEscapeHTML(false)
	}
	cells, changed, nonIdem, letterDiff := 0, 0, 0, 0
	byRule := map[string]int{}
	for ci, ch := range b.Chapters {
		for pi, para := range ch.Paragraphs {
			prev := ""
			for si, s := range para {
				tr, ok := s.Tr[*lang]
				if !ok {
					prev = s.Src
					continue
				}
				cells++
				res := typography.Apply(typography.Input{PrevSrc: prev, Src: s.Src, Tr: tr.Text, Lang: *lang})
				again := typography.Apply(typography.Input{PrevSrc: prev, Src: s.Src, Tr: res.Text, Lang: *lang}).Text
				if again != res.Text {
					nonIdem++
					fmt.Fprintf(os.Stderr, "not idempotent: %q -> %q\n", res.Text, again)
				}
				if letters(res.Text) != letters(tr.Text) {
					letterDiff++
				}
				r := row{ID: fmt.Sprintf("ch%d:%d:%d", ci+1, pi, si), Src: s.Src, Before: tr.Text, After: res.Text, Rules: res.Rules}
				if res.Text != tr.Text {
					changed++
					for _, n := range res.Rules {
						byRule[n]++
					}
					eo.Encode(r)
				}
				if ea != nil {
					ea.Encode(r)
				}
				prev = s.Src
			}
		}
	}
	fmt.Printf("cells %d changed %d (non-idempotent %d, letters changed %d)\n", cells, changed, nonIdem, letterDiff)
	names := make([]string, 0, len(byRule))
	for n := range byRule {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return byRule[names[i]] > byRule[names[j]] })
	for _, n := range names {
		fmt.Printf("  %-16s %d\n", n, byRule[n])
	}
}

func letters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
