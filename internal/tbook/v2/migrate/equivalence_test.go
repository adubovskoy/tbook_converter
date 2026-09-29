package migrate

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
	"golang.org/x/text/unicode/norm"
)

// v1Highlights is the version-1 §7 algorithm with clamping: the sorted,
// deduplicated ranges of tr.Text highlighted for source word i.
func v1Highlights(tr tbook.Translation, i int) [][2]int {
	n := len([]rune(tr.Text))
	set := map[[2]int]bool{}
	for _, ch := range tr.Align {
		hit := false
		for _, w := range ch.W {
			if w == i {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		a := max(0, min(ch.T[0], n))
		b := max(a, min(ch.T[1], n))
		if b > a {
			set[[2]int{a, b}] = true
		}
	}
	return sortedRanges(set)
}

func sortedRanges(set map[[2]int]bool) [][2]int {
	out := make([][2]int, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	sort.Slice(out, func(p, q int) bool {
		if out[p][0] != out[q][0] {
			return out[p][0] < out[q][0]
		}
		return out[p][1] < out[q][1]
	})
	return out
}

func rangesEqual(a, b [][2]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type equivStats struct {
	taps, equal, cross, other, notLocated int
	residue                               map[[2]int]int // (chapter, paragraph) → differing taps
	examples                              []string
}

// compareBlock walks every (sentence, word, language) tap of one v1 block and
// its migrated v2 block.
func compareBlock(st *equivStats, sents []*tbook.Sentence, blk *v2.Block, trs map[string][]*v2.Translation, langs []string, ch, para int) {
	words := v2.SentenceWords(blk)
	type tok struct{ a, b, k, i int }
	var all []tok
	for k, ws := range words {
		for i, w := range ws {
			all = append(all, tok{w[0], w[1], k, i})
		}
	}
	for k, s := range sents {
		d := blk.Sents[k][0]
		for i, w := range s.Words {
			a, b := w[0]+d, w[1]+d
			j := sort.Search(len(all), func(x int) bool { return all[x].a > a }) - 1
			for _, lang := range langs {
				st.taps++
				if j < 0 || !(all[j].a <= a && b <= all[j].b) {
					st.notLocated++
					continue
				}
				k2, i2 := all[j].k, all[j].i
				tr1 := s.Tr[lang]
				if k2 != k {
					st.cross++
					st.residue[[2]int{ch, para}]++
					continue
				}
				tr2 := trs[lang][k2]
				h1 := v1Highlights(tr1, i)
				var h2 [][2]int
				if tr2 != nil {
					h2 = v2.HighlightRanges(tr2, v2.TargetTokens(tr2), i2)
					set := map[[2]int]bool{}
					for _, r := range h2 {
						set[r] = true
					}
					h2 = sortedRanges(set)
				}
				if rangesEqual(h1, h2) {
					st.equal++
				} else {
					st.other++
					st.residue[[2]int{ch, para}]++
					if len(st.examples) < 10 {
						st.examples = append(st.examples, s.Src+" | "+lang+" | "+string([]rune(s.Src)[w[0]:w[1]]))
					}
				}
			}
		}
	}
}

func tapEquivalence(t *testing.T, path string) *equivStats {
	t.Helper()
	b, err := tbook.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	arc, _, err := Convert(b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	st := &equivStats{residue: map[[2]int]int{}}
	langs := b.Targets
	for ci, ch := range b.Chapters {
		skel := arc.Skeletons[ci]
		tables := map[int]tbook.Table{}
		for _, tb := range ch.Tables {
			tables[tb.Para] = tb
		}
		for p, para := range ch.Paragraphs {
			blk := &skel.Paragraphs[p]
			for k, s := range para {
				if norm.NFC.String(s.Src) != string([]rune(blk.Text)[blk.Sents[k][0]:blk.Sents[k][1]]) {
					t.Fatalf("ch%d p%d s%d: source text differs after migration", ci+1, p, k)
				}
			}
			if tb, ok := tables[p]; ok {
				for r, row := range tb.Rows {
					for c, cell := range row {
						trs := map[string][]*v2.Translation{}
						for _, lang := range langs {
							trs[lang] = arc.Overlays[ci][lang].Paragraphs[p].Rows[r][c].S
						}
						compareBlock(st, cell.Sentences, &blk.Table.Rows[r][c], trs, langs, ci, p)
					}
				}
				continue
			}
			trs := map[string][]*v2.Translation{}
			for _, lang := range langs {
				trs[lang] = arc.Overlays[ci][lang].Paragraphs[p].S
			}
			compareBlock(st, para, blk, trs, langs, ci, p)
		}
	}
	return st
}

func skipUnlessBook(t *testing.T, path string) {
	t.Helper()
	if testing.Short() {
		t.Skip("short mode")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("reference book not available: %v", err)
	}
}

// TestTapEquivalenceSample reproduces spec Annex A / report M8 on the
// eight-language reference sample: 836,496 of 836,600 taps identical, the
// 104 differences all on the 13 mid-word-cut paragraphs.
func TestTapEquivalenceSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "android", "app", "src", "main", "assets", "sample.tbook")
	skipUnlessBook(t, path)
	st := tapEquivalence(t, path)
	t.Logf("sample: taps=%d equal=%d cross_sentence=%d other=%d not_located=%d paragraphs_in_residue=%d",
		st.taps, st.equal, st.cross, st.other, st.notLocated, len(st.residue))
	if st.taps != 836600 || st.equal != 836496 || st.cross != 104 || st.other != 0 || st.notLocated != 0 {
		t.Fatalf("counts differ from the reference: %+v %v", *st, st.examples)
	}
	if len(st.residue) > 13 {
		t.Fatalf("residue spans %d paragraphs, want ≤ 13: %v", len(st.residue), st.residue)
	}
	if _, ok := st.residue[[2]int{2, 103}]; !ok { // sample ch3 p103, the S9 paragraph
		t.Fatalf("ch3 p103 missing from the residue: %v", st.residue)
	}
}

// TestTapEquivalenceKimi: a single-language book with no mid-word cuts is
// reproduced tap for tap (158,421 / 158,421).
func TestTapEquivalenceKimi(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "bench-quality", "full-kimi.tbook")
	skipUnlessBook(t, path)
	st := tapEquivalence(t, path)
	t.Logf("kimi: taps=%d equal=%d cross_sentence=%d other=%d", st.taps, st.equal, st.cross, st.other)
	if st.taps != 158421 || st.equal != 158421 {
		t.Fatalf("counts differ from the reference: %+v %v", *st, st.examples)
	}
}
