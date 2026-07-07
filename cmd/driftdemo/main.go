// Command driftdemo is a microscope for the drift-catching flow: it runs the
// real translate→align pipeline on a small plain-text passage (one paragraph
// per line) with its own cache dir, then shows every verification stage —
// the align pairs, per-pair lexcheck evidence, the aggregate lexcheck verdict,
// and the LLM judge verdict — on the live output AND on a synthetically
// drifted copy (every mapping shifted one source word right), so the detectors
// can be watched catching a known-bad alignment.
//
//	driftdemo passage.txt -t ru --model google/gemini-2.5-flash-lite \
//	          --judge-model google/gemini-2.5-flash --cache-dir /tmp/demo_cache
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dimando/reader/converter/internal/lexcheck"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/translate"
	"github.com/joho/godotenv"
)

func main() {
	src := flag.String("s", "es", "source language")
	tgt := flag.String("t", "ru", "target language")
	model := flag.String("model", "google/gemini-2.5-flash-lite", "translate/align model")
	judgeModel := flag.String("judge-model", "", "judge model (empty = skip judge)")
	cacheDir := flag.String("cache-dir", ".driftdemo_cache", "cache dir (fresh dir = fresh model output)")
	lexDir := flag.String("lexicons", "lexicons", "lexicon directory")
	batch := flag.Int("batch-size", 8, "translate batch size")
	alignBatch := flag.Int("align-batch", 4, "align batch size")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: driftdemo [flags] passage.txt")
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *src, *tgt, *model, *judgeModel, *cacheDir, *lexDir, *batch, *alignBatch); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(path, src, tgt, model, judgeModel, cacheDir, lexDir string, batch, alignBatch int) error {
	_ = godotenv.Load()
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	ch := segment.ParsedChapter{Title: "demo"}
	for line := range strings.SplitSeq(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			ch.Paragraphs = append(ch.Paragraphs, segment.ParsedParagraph{Text: line, Role: tbook.RoleBody})
		}
	}
	_, sentences := segment.BuildSentenceObjects([]segment.ParsedChapter{ch}, src)
	fmt.Printf("passage: %d paragraphs → %d sentences\n", len(ch.Paragraphs), len(sentences))

	client := translate.NewClient(translate.Options{
		BaseURL: "https://openrouter.ai/api/v1", APIKey: os.Getenv("OPENROUTER_API_KEY"),
		Model: model, Temperature: 0.3, JSONMode: true, MaxRetries: 4,
	})
	pipe := &translate.Pipeline{
		Client: client, CacheDir: cacheDir, Source: src,
		BatchSize: batch, AlignBatch: alignBatch, Concurrency: 4,
	}
	ctx := context.Background()
	if err := pipe.Run(ctx, sentences, []string{tgt}); err != nil {
		return err
	}
	found, missing := translate.FillFromCache(sentences, []string{tgt}, cacheDir, src, model)
	fmt.Printf("filled %d from cache (%d missing)\n", found, missing)

	dict, err := lexcheck.Load(lexDir, src, tgt)
	if err != nil {
		return err
	}
	if dict == nil {
		fmt.Printf("no %s-%s lexicon in %s — lexcheck skipped\n", src, tgt, lexDir)
	}

	// Drifted twins: same translation, every mapping shifted one source word
	// right — the exact positional-cascade signature the detectors exist for.
	drifted := make([]*tbook.Sentence, len(sentences))
	for i, s := range sentences {
		drifted[i] = shifted(s, tgt)
	}

	if judgeModel != "" {
		jc := translate.NewClient(translate.Options{
			BaseURL: "https://openrouter.ai/api/v1", APIKey: os.Getenv("OPENROUTER_API_KEY"),
			Model: judgeModel, Temperature: 0, JSONMode: true, MaxRetries: 4,
		})
		for name, set := range map[string][]*tbook.Sentence{"live": sentences, "drifted": drifted} {
			rep, err := translate.Judge(ctx, jc, cacheDir, src, []string{tgt}, set, 4, 4)
			if err != nil {
				return err
			}
			fmt.Printf("judge[%s]: %d checked, %d flagged, %d unjudged\n", name, rep.Checked, rep.Flagged, rep.Unjudged)
		}
	}

	for i, s := range sentences {
		fmt.Printf("\n=== sentence %d ===\nSRC: %s\n", i+1, s.Src)
		tr := s.Tr[tgt]
		fmt.Printf("TR:  %s\n", tr.Text)
		fmt.Println("PAIRS (live):")
		printPairs(s, tgt, dict)
		if dict != nil {
			report("lexcheck live   ", dict.CheckSentence(s, tgt))
			report("lexcheck drifted", dict.CheckSentence(drifted[i], tgt))
		}
		if judgeModel != "" {
			printVerdict("judge live   ", cacheDir, judgeModel, src, tgt, s)
			printVerdict("judge drifted", cacheDir, judgeModel, src, tgt, drifted[i])
		}
	}
	return nil
}

func report(label string, r lexcheck.Result) {
	fmt.Printf("%s: covered=%d supported=%d shiftHits=%d flagged=%v %s\n",
		label, r.Covered, r.Supported, r.ShiftHits, r.Flagged, r.Reason)
}

func printVerdict(label, cacheDir, judgeModel, src, tgt string, s *tbook.Sentence) {
	if v, ok := translate.VerdictFor(cacheDir, judgeModel, src, tgt, s); ok {
		fmt.Printf("%s: ok=%v %s\n", label, v.OK, v.Why)
	} else {
		fmt.Printf("%s: (no verdict)\n", label)
	}
}

// printPairs lists each align chunk with its per-pair lexcheck evidence:
// [ok] dictionary-supported, [??] covered but unsupported, [--] no evidence
// (uncovered / too short / inserted), and →N when the fragment fits the
// neighbor word instead (the shift signature).
func printPairs(s *tbook.Sentence, tgt string, dict *lexcheck.Dict) {
	tr := s.Tr[tgt]
	srcRunes := []rune(s.Src)
	trRunes := []rune(tr.Text)
	word := func(i int) string {
		if i < 0 || i >= len(s.Words) {
			return ""
		}
		return string(srcRunes[s.Words[i][0]:s.Words[i][1]])
	}
	for _, c := range tr.Align {
		frag := string(trRunes[c.T[0]:c.T[1]])
		var ws []string
		for _, wi := range c.W {
			ws = append(ws, fmt.Sprintf("%d:%s", wi, word(wi)))
		}
		mark := "--"
		if dict != nil && len(c.W) > 0 {
			for _, wi := range c.W {
				if !dict.Covered(word(wi)) {
					continue
				}
				mark = "??"
				if dict.Supports(word(wi), frag) {
					mark = "ok"
					break
				}
			}
			if mark == "??" {
				lo, hi := c.W[0], c.W[len(c.W)-1]
				for _, ni := range []int{lo - 1, hi + 1} {
					if w := word(ni); w != "" && dict.Covered(w) && dict.Supports(w, frag) {
						mark = "→" + fmt.Sprint(ni)
						break
					}
				}
			}
		}
		fmt.Printf("  [%s] %q ← {%s}\n", mark, frag, strings.Join(ws, ", "))
	}
}

// shifted returns a copy of s whose align mappings point one source word to the
// right — synthetic positional drift (same construction as cmd/lexeval).
func shifted(s *tbook.Sentence, target string) *tbook.Sentence {
	tr := s.Tr[target]
	al := make([]tbook.AlignChunk, len(tr.Align))
	n := max(1, len(s.Words))
	for i, c := range tr.Align {
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
