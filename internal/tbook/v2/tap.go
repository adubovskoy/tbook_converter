package v2

import "sort"

// HighlightRanges returns the code-point ranges of tr.T to highlight for a tap
// on source word i — the word rung of the tap algorithm (spec §8.2, §8.5):
// every token whose link names i (its part spans when a part links i), plus
// every escape chunk whose indices name i. toks is the translation's token
// inventory (TargetTokens). Ranges are clamped, degenerate ones dropped, and
// the result is sorted.
func HighlightRanges(tr *Translation, toks [][2]int, i int) [][2]int {
	if tr == nil || tr.T == "" {
		return nil
	}
	n := len([]rune(tr.T))
	var out [][2]int
	for j, link := range tr.A {
		if j >= len(toks) || !link.Contains(i) {
			continue
		}
		ta, tb := toks[j][0], toks[j][1]
		whole := true
		for _, p := range tr.Parts {
			if p.Token == j && p.Link.Contains(i) {
				out = append(out, [2]int{ta + p.A, ta + p.B})
				whole = false
			}
		}
		if whole {
			out = append(out, [2]int{ta, tb})
		}
	}
	for _, x := range tr.X {
		if len(x) < 3 {
			continue
		}
		for _, w := range x[2:] {
			if w == i {
				out = append(out, [2]int{x[0], x[1]})
				break
			}
		}
	}
	res := out[:0]
	for _, r := range out {
		a, b := clamp(r[0], 0, n), clamp(r[1], 0, n)
		if b > a {
			res = append(res, [2]int{a, b})
		}
	}
	sort.Slice(res, func(p, q int) bool {
		if res[p][0] != res[q][0] {
			return res[p][0] < res[q][0]
		}
		return res[p][1] < res[q][1]
	})
	return res
}

// Label classifies a multi-token highlight (spec §8.3): "run" when the token
// indices are consecutive, "split" otherwise, "" for fewer than two tokens.
func Label(tokenIdx []int) string {
	if len(tokenIdx) < 2 {
		return ""
	}
	s := append([]int(nil), tokenIdx...)
	sort.Ints(s)
	for k := 1; k < len(s); k++ {
		if s[k] != s[k-1]+1 {
			return "split"
		}
	}
	return "run"
}
