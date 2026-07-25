// Command tbookdiff compares two .tbook files produced from the same book —
// same sentence set, possibly different translations/alignments — and reports
// per-sentence and aggregate differences for one target language: changed
// translation texts, all-word and content-word target coverage, source-side
// tap coverage, and the worst coverage regressions. Offline, no API calls —
// an A/B instrument for comparing converter/model/aligner runs:
//
//	tbookdiff [flags] A.tbook B.tbook
//	          -t ru [--limit N] [--worst N] [--dump out.json]
//
// Sentences are matched by exact source text (by occurrence index when the
// same text repeats). Coverage is recomputed from the alignments themselves —
// never taken from the stored q — so both sides are measured identically;
// the stored q is reported once as a sanity line. --dump writes the full
// per-sentence records as a JSON array for a downstream LLM-judging harness.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/stopwords"
	"github.com/dimando/reader/converter/internal/tbook"
)

// regressDelta is the content-coverage change that counts a sentence as
// regressed/improved in the aggregate report.
const regressDelta = 0.1

func main() {
	tgt := flag.String("t", "ru", "target language")
	limit := flag.Int("limit", 0, "evaluate only the first N matched pairs (0 = all)")
	worst := flag.Int("worst", 10, "print the top N sentences by coverage regression")
	dump := flag.String("dump", "", "write per-sentence records to this JSON file")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: tbookdiff [flags] A.tbook B.tbook")
		os.Exit(2)
	}
	if err := run(flag.Arg(0), flag.Arg(1), *tgt, *limit, *worst, *dump); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(pathA, pathB, tgt string, limit, worst int, dump string) error {
	bookA, err := tbook.Read(pathA)
	if err != nil {
		return fmt.Errorf("read A: %w", err)
	}
	bookB, err := tbook.Read(pathB)
	if err != nil {
		return fmt.Errorf("read B: %w", err)
	}
	for name, b := range map[string]*tbook.Book{"A": bookA, "B": bookB} {
		if !slices.Contains(b.Targets, tgt) {
			fmt.Printf("warning: %s is not in %s's targetLangs %v\n", tgt, name, b.Targets)
		}
	}
	if bookA.Source != bookB.Source {
		fmt.Printf("warning: source languages differ (A %s, B %s); using A's for tap classification\n",
			bookA.Source, bookB.Source)
	}
	tgtStop := stopwords.Set(tgt)
	if tgtStop == nil {
		fmt.Printf("no %s stopword list: content coverage equals all-word coverage\n", tgt)
	}
	srcStop := stopwords.Set(bookA.Source)
	if srcStop == nil {
		fmt.Printf("no %s stopword list: content tap coverage equals all-word tap coverage\n", bookA.Source)
	}

	sentsA, sentsB := collect(bookA), collect(bookB)
	pairs := match(sentsA, sentsB, tgt, tgtStop, srcStop)
	fmt.Printf("sentences: A %d, B %d, matched %d (A-only %d, B-only %d)\n",
		len(sentsA), len(sentsB), len(pairs), len(sentsA)-len(pairs), len(sentsB)-len(pairs))
	if limit > 0 && len(pairs) > limit {
		pairs = pairs[:limit]
		fmt.Printf("evaluating first %d matched pairs (--limit)\n", limit)
	}

	report(pairs, tgt, worst)

	if dump != "" {
		if err := writeDump(dump, pairs); err != nil {
			return fmt.Errorf("dump: %w", err)
		}
		fmt.Printf("wrote %d records to %s\n", len(pairs), dump)
	}
	return nil
}

// collect returns every sentence of the book in a deterministic order:
// chapters in manifest order (paragraphs, then table cells), then note bodies
// sorted by note id. Book.Sentences is not used because it walks the notes
// map in random order, which would break occurrence-index matching whenever
// a source text repeats across notes.
func collect(b *tbook.Book) []*tbook.Sentence {
	var out []*tbook.Sentence
	for _, ch := range b.Chapters {
		for _, para := range ch.Paragraphs {
			out = append(out, para...)
		}
		for _, t := range ch.Tables {
			for _, row := range t.Rows {
				for _, cell := range row {
					out = append(out, cell.Sentences...)
				}
			}
		}
	}
	ids := make([]string, 0, len(b.Notes))
	for id := range b.Notes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		for _, para := range b.Notes[id].Paragraphs {
			out = append(out, para...)
		}
	}
	return out
}

// side holds one file's measurements for one matched sentence. Coverage is
// recomputed from the alignment (same rule as embalign: a target word counts
// as covered when overlapped by ≥1 chunk that claims ≥1 source word), so A
// and B are always measured identically regardless of who wrote their q.
type side struct {
	text       string
	translated bool     // non-empty target text
	chunks     int      // number of align chunks
	q          float64  // stored Translation.Q, informative only
	covAll     float64  // target words covered
	covContent float64  // target content words covered (falls back to covAll)
	tapAll     float64  // source words referenced by ≥1 chunk's w[]
	tapContent float64  // source content words referenced (falls back to tapAll)
	hasWords   bool     // sentence has tappable source words
	uncovered  []string // uncovered target content words, in text order
}

// pair is one matched sentence with both sides measured.
type pair struct {
	src  string
	a, b side
}

// match pairs A's and B's sentences by exact source text, disambiguating
// repeats by occurrence index (the k-th "Yes." in B matches the k-th in A),
// and measures both sides of every pair.
func match(sentsA, sentsB []*tbook.Sentence, tgt string, tgtStop, srcStop map[string]bool) []pair {
	aBySrc := map[string][]*tbook.Sentence{}
	for _, s := range sentsA {
		aBySrc[s.Src] = append(aBySrc[s.Src], s)
	}
	var pairs []pair
	seen := map[string]int{}
	for _, sb := range sentsB {
		k := seen[sb.Src]
		seen[sb.Src]++
		if k >= len(aBySrc[sb.Src]) {
			continue // B-only sentence
		}
		sa := aBySrc[sb.Src][k]
		pairs = append(pairs, pair{
			src: sb.Src,
			a:   measure(sa, tgt, tgtStop, srcStop),
			b:   measure(sb, tgt, tgtStop, srcStop),
		})
	}
	return pairs
}

// measure computes one side's metrics. A sentence with a missing or empty
// target translation returns translated=false with zeroed coverage — the
// caller counts it instead of crashing on it.
func measure(s *tbook.Sentence, tgt string, tgtStop, srcStop map[string]bool) side {
	m := side{uncovered: []string{}}
	tr := s.Tr[tgt]
	m.text = tr.Text
	if tr.Text == "" {
		return m
	}
	m.translated = true
	m.chunks = len(tr.Align)
	m.q = tr.Q

	// Target-side coverage over the tokenized translation.
	trWords := segment.Tokenize(tr.Text)
	trRunes := []rune(tr.Text)
	allN, allCov, contentN, contentCov := len(trWords), 0, 0, 0
	for _, w := range trWords {
		covered := false
		for _, c := range tr.Align {
			if len(c.W) > 0 && c.T[0] < w[1] && w[0] < c.T[1] {
				covered = true
				break
			}
		}
		word := string(trRunes[w[0]:w[1]])
		content := tgtStop == nil || !tgtStop[strings.ToLower(word)]
		if covered {
			allCov++
			if content {
				contentCov++
			}
		} else if content {
			m.uncovered = append(m.uncovered, word)
		}
		if content {
			contentN++
		}
	}
	if allN > 0 {
		m.covAll = float64(allCov) / float64(allN)
	}
	if contentN > 0 {
		m.covContent = float64(contentCov) / float64(contentN)
	} else {
		m.covContent = m.covAll // no content words: same fallback as embalign.ContentQ
	}

	// Source-side tap coverage: which source words any chunk references.
	if n := len(s.Words); n > 0 {
		m.hasWords = true
		hit := make([]bool, n)
		for _, c := range tr.Align {
			for _, wi := range c.W {
				if wi >= 0 && wi < n {
					hit[wi] = true
				}
			}
		}
		srcRunes := []rune(s.Src)
		tapAll, tapContent, srcContentN := 0, 0, 0
		for i, w := range s.Words {
			var word string
			if w[0] >= 0 && w[0] < w[1] && w[1] <= len(srcRunes) {
				word = string(srcRunes[w[0]:w[1]])
			}
			content := srcStop == nil || !srcStop[strings.ToLower(word)]
			if hit[i] {
				tapAll++
				if content {
					tapContent++
				}
			}
			if content {
				srcContentN++
			}
		}
		m.tapAll = float64(tapAll) / float64(n)
		if srcContentN > 0 {
			m.tapContent = float64(tapContent) / float64(srcContentN)
		} else {
			m.tapContent = m.tapAll
		}
	}
	return m
}

// report prints the aggregate comparison and the worst regressions. Coverage
// aggregates run over pairs translated on BOTH sides (so the means compare
// like with like); tap aggregates additionally need tappable source words.
func report(pairs []pair, tgt string, worst int) {
	var (
		missA, missB, changed int
		both                  []pair // translated on both sides
	)
	for _, p := range pairs {
		if !p.a.translated {
			missA++
		}
		if !p.b.translated {
			missB++
		}
		if p.a.translated && p.b.translated {
			both = append(both, p)
			if p.a.text != p.b.text {
				changed++
			}
		}
	}
	fmt.Printf("missing/empty %s translation: A %d, B %d (%d pairs excluded from coverage stats)\n",
		tgt, missA, missB, len(pairs)-len(both))
	fmt.Printf("translation text changed: %d / %d (%.1f%%)\n\n", changed, len(both), pct(changed, len(both)))

	col := func(xs []float64) string { return fmt.Sprintf("%.3f / %.3f", mean(xs), median(xs)) }
	row := func(label string, as, bs []float64) {
		fmt.Printf("  %-22s %-15s %-15s %+.3f\n", label, col(as), col(bs), mean(bs)-mean(as))
	}
	pick := func(f func(pair) float64, keep func(pair) bool) []float64 {
		var xs []float64
		for _, p := range both {
			if keep == nil || keep(p) {
				xs = append(xs, f(p))
			}
		}
		return xs
	}
	tappable := func(p pair) bool { return p.a.hasWords && p.b.hasWords }

	fmt.Printf("  %-22s %-15s %-15s %s\n", "", "A mean/median", "B mean/median", "Δmean")
	fmt.Printf("target coverage (%s), recomputed:\n", tgt)
	row("all-word", pick(func(p pair) float64 { return p.a.covAll }, nil),
		pick(func(p pair) float64 { return p.b.covAll }, nil))
	row("content-word", pick(func(p pair) float64 { return p.a.covContent }, nil),
		pick(func(p pair) float64 { return p.b.covContent }, nil))
	fmt.Println("source tap coverage:")
	row("all-word", pick(func(p pair) float64 { return p.a.tapAll }, tappable),
		pick(func(p pair) float64 { return p.b.tapAll }, tappable))
	row("content-word", pick(func(p pair) float64 { return p.a.tapContent }, tappable),
		pick(func(p pair) float64 { return p.b.tapContent }, tappable))
	row("stored q", pick(func(p pair) float64 { return p.a.q }, nil),
		pick(func(p pair) float64 { return p.b.q }, nil))
	fmt.Printf("  %-22s %-15.1f %-15.1f\n", "chunks per sentence",
		mean(pick(func(p pair) float64 { return float64(p.a.chunks) }, nil)),
		mean(pick(func(p pair) float64 { return float64(p.b.chunks) }, nil)))

	regressed, improved := 0, 0
	for _, p := range both {
		switch d := p.b.covContent - p.a.covContent; {
		case d < -regressDelta:
			regressed++
		case d > regressDelta:
			improved++
		}
	}
	fmt.Printf("\ncontent coverage A→B: regressed >%.1f: %d (%.1f%%), improved >%.1f: %d (%.1f%%)\n",
		regressDelta, regressed, pct(regressed, len(both)),
		regressDelta, improved, pct(improved, len(both)))

	// Worst regressions: largest content-coverage drop first.
	ranked := append([]pair(nil), both...)
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].b.covContent-ranked[i].a.covContent < ranked[j].b.covContent-ranked[j].a.covContent
	})
	fmt.Printf("\nworst %d by content-coverage regression:\n", worst)
	shown := 0
	for _, p := range ranked {
		d := p.b.covContent - p.a.covContent
		if d >= 0 || shown >= worst {
			break
		}
		shown++
		fmt.Printf("%3d. Δ%+.3f\n     SRC: %s\n", shown, d, p.src)
		fmt.Printf("     A:   %s\n          uncovered: %v\n", p.a.text, p.a.uncovered)
		fmt.Printf("     B:   %s\n          uncovered: %v\n", p.b.text, p.b.uncovered)
	}
	if shown == 0 {
		fmt.Println("     (none)")
	}
}

// record is one --dump entry; the field names are the downstream harness's
// contract.
type record struct {
	Src         string   `json:"src"`
	AText       string   `json:"aText"`
	BText       string   `json:"bText"`
	TextChanged bool     `json:"textChanged"`
	ACovAll     float64  `json:"aCovAll"`
	ACovContent float64  `json:"aCovContent"`
	BCovAll     float64  `json:"bCovAll"`
	BCovContent float64  `json:"bCovContent"`
	ATapContent float64  `json:"aTapContent"`
	BTapContent float64  `json:"bTapContent"`
	AUncovered  []string `json:"aUncovered"`
	BUncovered  []string `json:"bUncovered"`
}

// writeDump writes every matched pair (including untranslated ones, with
// zeroed coverage — the harness filters) as a JSON array.
func writeDump(path string, pairs []pair) error {
	recs := make([]record, len(pairs))
	for i, p := range pairs {
		recs[i] = record{
			Src: p.src, AText: p.a.text, BText: p.b.text,
			TextChanged: p.a.text != p.b.text,
			ACovAll:     p.a.covAll, ACovContent: p.a.covContent,
			BCovAll: p.b.covAll, BCovContent: p.b.covContent,
			ATapContent: p.a.tapContent, BTapContent: p.b.tapContent,
			AUncovered: p.a.uncovered, BUncovered: p.b.uncovered,
		}
	}
	data, err := json.MarshalIndent(recs, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
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
