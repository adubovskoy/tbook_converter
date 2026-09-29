package main

import (
	"testing"

	"github.com/dimando/reader/converter/internal/lexcheck"
	"github.com/dimando/reader/converter/internal/tbook"
)

// twoWordSentence builds a sentence with four align chunks over four words.
func fourChunkSentence() *tbook.Sentence {
	return &tbook.Sentence{
		Src:   "alpha beta gamma delta",
		Words: [][2]int{{0, 5}, {6, 10}, {11, 16}, {17, 22}},
		Tr: map[string]tbook.Translation{"ru": {Text: "a b c d", Align: []tbook.AlignChunk{
			{T: [2]int{0, 1}, W: []int{0}},
			{T: [2]int{2, 3}, W: []int{1}},
			{T: [2]int{4, 5}, W: []int{2}},
			{T: [2]int{6, 7}, W: []int{3}},
		}}},
	}
}

func words(s *tbook.Sentence) [][]int {
	var out [][]int
	for _, c := range s.Tr["ru"].Align {
		out = append(out, c.W)
	}
	return out
}

func TestFullShiftMovesEveryChunkRight(t *testing.T) {
	got := words(shifted(fourChunkSentence(), "ru", 0))
	want := [][]int{{1}, {2}, {3}, {0}} // last wraps around
	for i := range want {
		if got[i][0] != want[i][0] {
			t.Fatalf("chunk %d: got %v want %v (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestHalfShiftLeavesTheFirstHalfAlone(t *testing.T) {
	got := words(shifted(fourChunkSentence(), "ru", 2))
	want := [][]int{{0}, {1}, {3}, {0}}
	for i := range want {
		if got[i][0] != want[i][0] {
			t.Fatalf("chunk %d: got %v want %v (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestShiftDoesNotTouchTheOriginal(t *testing.T) {
	s := fourChunkSentence()
	_ = shifted(s, "ru", 0)
	if got := words(s); got[0][0] != 0 {
		t.Fatalf("original mutated: %v", got)
	}
}

func TestVerdictAppliesBothRules(t *testing.T) {
	lowSupport := lexcheck.Result{Covered: 5, Supported: 1} // rate 0.2
	if !verdict(lowSupport, 4, 0.34, 2) {
		t.Fatal("low-support result should flag at the default thresholds")
	}
	if verdict(lowSupport, 6, 0.34, 2) {
		t.Fatal("raising min-covered above the evidence must silence the rule")
	}
	shift := lexcheck.Result{Covered: 8, Supported: 8, ShiftHits: 2}
	if !verdict(shift, 4, 0.34, 2) {
		t.Fatal("shift-pattern result should flag independently of support rate")
	}
	if verdict(shift, 4, 0.34, 3) {
		t.Fatal("raising the shift threshold must silence the rule")
	}
}

func TestGridParsersSort(t *testing.T) {
	f, err := floats("0.40, 0.25,0.34")
	if err != nil || f[0] != 0.25 || f[2] != 0.40 {
		t.Fatalf("floats: %v %v", f, err)
	}
	i, err := ints("5,3,4")
	if err != nil || i[0] != 3 || i[2] != 5 {
		t.Fatalf("ints: %v %v", i, err)
	}
	if _, err := ints("4,x"); err == nil {
		t.Fatal("ints should reject garbage")
	}
}
