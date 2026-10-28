package segment

import (
	"reflect"
	"testing"
)

func TestSplitSentencesNeverCutsMidWord(t *testing.T) {
	seg := NewSegmenter("en")
	cases := []struct {
		in   string
		want []string
	}{
		{"Ms. Fal and Dr. Nopri.", []string{"Ms. Fal and Dr. Nopri."}},
		{"It hit Jo’s. It didn’t.", []string{"It hit Jo’s.", "It didn’t."}},
		{
			"Oh, she thought. She hoped the shell she’d found didn’t blow up when her ship hit Hino’s. It didn’t. Phew, she thought.",
			[]string{"Oh, she thought.", "She hoped the shell she’d found didn’t blow up when her ship hit Hino’s.", "It didn’t.", "Phew, she thought."},
		},
		{
			"She thought of the elegantly elderly Ms. Fal Dvelner and the terribly earnest, multiply-reincarnated Mr. Nopri.",
			[]string{"She thought of the elegantly elderly Ms. Fal Dvelner and the terribly earnest, multiply-reincarnated Mr. Nopri."},
		},
		{
			"“We’ll come to that shortly. The salient point is that Ms. Y’breq may be on her way back, intending violence to Mr. Veppers, in revenge for her earlier murder.”",
			[]string{"“We’ll come to that shortly.", "The salient point is that Ms. Y’breq may be on her way back, intending violence to Mr. Veppers, in revenge for her earlier murder.”"},
		},
	}
	for _, c := range cases {
		if got := SplitSentences(c.in, seg); !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitSentences(%q)\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}

func TestWordSafeSegmenterLeavesCleanOutputAlone(t *testing.T) {
	seg := NewSegmenter("en")
	in := "Mr. Fal and Mr. Nopri. They left."
	if got, want := seg.Segment(in), []string{"Mr. Fal and Mr. Nopri.", "They left."}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q want %q", got, want)
	}
}
