// Command lexeval evaluates the static drift detector (internal/lexcheck)
// against a produced .tbook — offline, no API calls.
//
// Method: every sentence in the book is a presumed-good NEGATIVE (optionally
// restricted to judge-OK sentences via --judged-only); for each, a synthetic
// POSITIVE is derived by shifting every align mapping one source word to the
// right — the exact signature of positional drift. The detector must flag the
// shifted copies (recall) and not the originals (false positives). A threshold
// grid is swept so lexcheck's defaults can be chosen from data.
//
//	lexeval book.tbook -s es -t ru [--lexicons lexicons] [--cache-dir .tbook_cache]
//	        [--judge-model MODEL --judged-only]
package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/dimando/reader/converter/internal/lexcheck"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
)

type chapterPayload struct {
	Paragraphs [][]*tbook.Sentence `json:"paragraphs"`
	Tables     []tbook.Table       `json:"tables"`
}

func main() {
	src := flag.String("s", "es", "source language")
	tgt := flag.String("t", "ru", "target language")
	lexDir := flag.String("lexicons", "lexicons", "lexicon directory")
	cacheDir := flag.String("cache-dir", ".tbook_cache", "cache dir (for --judged-only)")
	judgeModel := flag.String("judge-model", "", "judge model id (for --judged-only)")
	judgedOnly := flag.Bool("judged-only", false, "use only sentences the LLM judge marked OK as negatives")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: lexeval book.tbook [flags]")
		os.Exit(2)
	}

	dict, err := lexcheck.Load(*lexDir, *src, *tgt)
	if err != nil || dict == nil {
		fmt.Fprintf(os.Stderr, "no lexicon for %s-%s in %s (err=%v)\n", *src, *tgt, *lexDir, err)
		os.Exit(1)
	}
	fmt.Printf("lexicon %s-%s: %d headwords\n", *src, *tgt, dict.Entries())

	sentences, err := readBook(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "read book:", err)
		os.Exit(1)
	}

	var clean []*tbook.Sentence
	for _, s := range sentences {
		tr, ok := s.Tr[*tgt]
		if !ok || tr.Text == "" || len(tr.Align) < 4 || len(s.Words) < 5 {
			continue
		}
		if *judgedOnly {
			v, ok := translate.VerdictFor(*cacheDir, *judgeModel, *src, *tgt, s)
			if !ok || !v.OK {
				continue
			}
		}
		clean = append(clean, s)
	}
	fmt.Printf("evaluation set: %d sentences (negatives) + %d synthetic drifted copies (positives)\n",
		len(clean), len(clean))

	// Coverage: how much of the book's alignment can the dictionary see at all?
	var covered, supported int
	for _, s := range clean {
		r := dict.CheckSentence(s, *tgt)
		covered += r.Covered
		supported += r.Supported
	}
	fmt.Printf("dictionary coverage: %d scored pairs, %.0f%% of them supported on clean text\n",
		covered, 100*float64(supported)/float64(max(1, covered)))

	// Threshold sweep: originals = negatives; fully- and half-shifted copies =
	// positives (half-shift models a cascade that starts mid-sentence).
	fmt.Printf("\n%-22s %10s %10s %8s %9s\n", "thresholds", "recall/full", "recall/half", "FP-rate", "precision")
	for _, rate := range []float64{0.25, 0.30, 0.34, 0.40, 0.45} {
		for _, shiftHits := range []int{2, 3} {
			var fp, tpF, tpH, tn int
			for _, s := range clean {
				if verdict(dict.CheckSentence(s, *tgt), rate, shiftHits) {
					fp++
				} else {
					tn++
				}
				if verdict(dict.CheckSentence(shifted(s, *tgt, 0), *tgt), rate, shiftHits) {
					tpF++
				}
				if verdict(dict.CheckSentence(shifted(s, *tgt, 2), *tgt), rate, shiftHits) {
					tpH++
				}
			}
			n := len(clean)
			prec := float64(tpF) / float64(max(1, tpF+fp))
			fmt.Printf("support≤%.2f shift≥%d   %9.1f%% %9.1f%% %7.1f%% %8.1f%%\n",
				rate, shiftHits, 100*float64(tpF)/float64(n), 100*float64(tpH)/float64(n),
				100*float64(fp)/float64(n), 100*prec)
		}
	}
}

// verdict applies a threshold pair to a lexcheck result.
func verdict(r lexcheck.Result, rate float64, shiftHits int) bool {
	if r.Covered >= lexcheck.MinCoveredPairs && r.SupportRate() <= rate {
		return true
	}
	return r.ShiftHits >= shiftHits
}

// shifted returns a copy of s whose align mappings point one source word to
// the right — synthetic positional drift. divide=0 shifts every chunk (full
// cascade); divide=2 shifts only the last half (a cascade starting mid-sentence).
func shifted(s *tbook.Sentence, target string, divide int) *tbook.Sentence {
	tr := s.Tr[target]
	al := make([]tbook.AlignChunk, len(tr.Align))
	n := len(s.Words)
	from := 0
	if divide > 0 {
		from = len(tr.Align) - len(tr.Align)/divide
	}
	for i, c := range tr.Align {
		if i < from {
			al[i] = c
			continue
		}
		w := make([]int, len(c.W))
		for j, wi := range c.W {
			w[j] = (wi + 1) % n
		}
		al[i] = tbook.AlignChunk{T: c.T, W: w}
	}
	cp := *s
	cp.Tr = map[string]tbook.Translation{target: {Text: tr.Text, Align: al}}
	return &cp
}

func readBook(path string) ([]*tbook.Sentence, error) {
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
	for _, ch := range man.Chapters {
		var payload chapterPayload
		if err := readJSON(ch.File, &payload); err != nil {
			return nil, err
		}
		for _, para := range payload.Paragraphs {
			out = append(out, para...)
		}
		for _, t := range payload.Tables {
			for _, row := range t.Rows {
				for _, cell := range row {
					out = append(out, cell.Sentences...)
				}
			}
		}
	}
	return out, nil
}
