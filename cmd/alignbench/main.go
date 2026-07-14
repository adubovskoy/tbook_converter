// Command alignbench re-aligns every sentence of a produced .tbook with the
// local embedding aligner and reports alignment-quality aggregates — offline,
// no API calls. Point --script at two aligner versions and diff the numbers
// to validate an aligner change (the same methodology that picked
// LaBSE+argmax over the LLM align pass):
//
//	alignbench book.tbook -s ru -t es [--script tools/embalign.py]
//	           [--lexicons lexicons] [--limit N]
//
// Reported: all-word and content-word coverage (mean + share of sentences a
// hybrid gate at EmbQMin would send to the LLM pass), and lexcheck evidence
// over the produced pairs (scored pairs, support rate, flagged sentences).
package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dimando/reader/converter/internal/embalign"
	"github.com/dimando/reader/converter/internal/lexcheck"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/stopwords"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
)

const batchSize = 64

func main() {
	src := flag.String("s", "ru", "source language")
	tgt := flag.String("t", "es", "target language")
	script := flag.String("script", "tools/embalign.py", "aligner script to benchmark")
	lexDir := flag.String("lexicons", "lexicons", "lexicon directory")
	limit := flag.Int("limit", 0, "evaluate only the first N sentences (0 = all)")
	qmin := flag.Float64("qmin", translate.DefaultEmbQMin, "gate threshold to report against")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: alignbench book.tbook [flags]")
		os.Exit(2)
	}

	dict, err := lexcheck.Load(*lexDir, *src, *tgt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lexicon:", err)
		os.Exit(1)
	}
	if dict == nil {
		fmt.Printf("no %s-%s lexicon: lexcheck columns will be empty\n", *src, *tgt)
	}
	stop := stopwords.Set(*tgt)
	if stop == nil {
		fmt.Printf("no %s stopword list: content coverage equals all-word coverage\n", *tgt)
	}

	sentences, err := readBook(flag.Arg(0), *tgt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read book:", err)
		os.Exit(1)
	}
	if *limit > 0 && len(sentences) > *limit {
		sentences = sentences[:*limit]
	}
	fmt.Printf("%d sentences, script %s\n", len(sentences), *script)

	aligner, err := embalign.Start(embalign.Options{Script: *script})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer aligner.Close()

	var (
		qAll, qContent       []float64
		gatedQ, flagged      int
		covered, supported   int
		cCovered, cSupported int
		pairsTotal           int
	)
	for start := 0; start < len(sentences); start += batchSize {
		end := min(start+batchSize, len(sentences))
		batch := sentences[start:end]
		srcs := make([][]string, len(batch))
		tgts := make([][]string, len(batch))
		trWords := make([][][2]int, len(batch))
		for i, s := range batch {
			text := s.Tr[*tgt].Text
			trWords[i] = segment.Tokenize(text)
			srcs[i] = embalign.WordStrings(s.Src, s.Words)
			tgts[i] = embalign.WordStrings(text, trWords[i])
		}
		results, err := aligner.AlignBatch(srcs, tgts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "align:", err)
			os.Exit(1)
		}
		for i, s := range batch {
			if results[i] == nil {
				continue
			}
			text := s.Tr[*tgt].Text
			chunks, q := embalign.Chunks(results[i], trWords[i])
			cq := embalign.ContentQ(chunks, trWords[i], text, stop)
			qAll = append(qAll, q)
			qContent = append(qContent, cq)
			pairsTotal += len(chunks)
			if cq < *qmin {
				gatedQ++
			}
			if dict != nil {
				tmp := &tbook.Sentence{Src: s.Src, Words: s.Words,
					Tr: map[string]tbook.Translation{*tgt: {Text: text, Align: chunks}}}
				r := dict.CheckSentence(tmp, *tgt)
				covered += r.Covered
				supported += r.Supported
				if r.Flagged {
					flagged++
				}
				if stop != nil {
					// Content pairs only: dictionaries can't support a
					// function-word attachment ("la"←Зоной), so those pairs
					// dilute the all-pair rate without meaning misalignment.
					tmp.Tr[*tgt] = tbook.Translation{Text: text, Align: contentChunks(chunks, text, stop)}
					rc := dict.CheckSentence(tmp, *tgt)
					cCovered += rc.Covered
					cSupported += rc.Supported
				}
			}
		}
		fmt.Fprintf(os.Stderr, "\r%d/%d", end, len(sentences))
	}
	fmt.Fprintln(os.Stderr)

	n := len(qAll)
	fmt.Printf("aligned sentences:      %d\n", n)
	fmt.Printf("coverage all-word:      mean %.3f  median %.3f\n", mean(qAll), median(qAll))
	fmt.Printf("coverage content-word:  mean %.3f  median %.3f\n", mean(qContent), median(qContent))
	fmt.Printf("gate (contentQ < %.2f): %d (%.1f%%)\n", *qmin, gatedQ, pct(gatedQ, n))
	fmt.Printf("chunks per sentence:    %.1f\n", float64(pairsTotal)/float64(max(1, n)))
	if dict != nil {
		fmt.Printf("lexcheck: %d scored pairs, support rate %.3f, flagged %d (%.1f%%)\n",
			covered, float64(supported)/float64(max(1, covered)), flagged, pct(flagged, n))
		if stop != nil {
			fmt.Printf("lexcheck content pairs: %d scored, support rate %.3f\n",
				cCovered, float64(cSupported)/float64(max(1, cCovered)))
		}
	}
}

// contentChunks drops chunks whose target span is a stopword — what remains
// is the pairs a dictionary has a chance to support.
func contentChunks(chunks []tbook.AlignChunk, text string, stop map[string]bool) []tbook.AlignChunk {
	runes := []rune(text)
	out := make([]tbook.AlignChunk, 0, len(chunks))
	for _, c := range chunks {
		a, b := c.T[0], c.T[1]
		if a < 0 || b > len(runes) || a >= b {
			continue
		}
		if !stop[strings.ToLower(string(runes[a:b]))] {
			out = append(out, c)
		}
	}
	return out
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

type chapterPayload struct {
	Paragraphs [][]*tbook.Sentence `json:"paragraphs"`
	Tables     []tbook.Table       `json:"tables"`
}

// readBook returns the book's sentences that carry a non-empty target
// translation and tappable source words.
func readBook(path, tgt string) ([]*tbook.Sentence, error) {
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
	keep := func(s *tbook.Sentence) {
		if len(s.Words) > 0 && s.Tr[tgt].Text != "" {
			out = append(out, s)
		}
	}
	for _, ch := range man.Chapters {
		var payload chapterPayload
		if err := readJSON(ch.File, &payload); err != nil {
			return nil, err
		}
		for _, para := range payload.Paragraphs {
			for _, s := range para {
				keep(s)
			}
		}
		for _, t := range payload.Tables {
			for _, row := range t.Rows {
				for _, cell := range row {
					for _, s := range cell.Sentences {
						keep(s)
					}
				}
			}
		}
	}
	return out, nil
}
