package v2

import (
	"reflect"
	"testing"
)

func s1Overlay(links []Link) (*Overlay, *Chapter) {
	skel := &Chapter{ID: "cbedb517c", Paragraphs: []Block{{ID: "04537723", Text: fixS1, Sents: [][2]int{{0, 35}}}}}
	ov := &Overlay{Lang: "ru", Chapter: "cbedb517c", Gates: []string{}, Paragraphs: []GlossParagraph{
		{ID: "04537723", S: []*Translation{{T: "«Тогда я могу подождать в соседней комнате.»", A: links}}},
	}}
	return ov, skel
}

func TestAlignDigestS1(t *testing.T) {
	ov, skel := s1Overlay([]Link{Single(0), Single(1), Single(2), Single(3), Single(4), Multi(5, 6), Single(7)})
	const want = "sha256:4bfa91a5b38848ba41ed5bf471da900413b0acdb6c68928dfaa74c0b6e832918"
	if got := AlignDigest(ov, skel); got != want {
		t.Fatalf("AlignDigest = %s, want %s", got, want)
	}
	// Links of «Тогда» and «комнате» swapped.
	ov, skel = s1Overlay([]Link{Single(7), Single(1), Single(2), Single(3), Single(4), Multi(5, 6), Single(0)})
	const swapped = "sha256:f7552db092fe0c64ff64ab2d69079e20a36e4fb3f9efd8c51dd6ab99e5e24d7b"
	if got := AlignDigest(ov, skel); got != swapped {
		t.Fatalf("swapped AlignDigest = %s, want %s", got, swapped)
	}
	// No pairs → digest of the empty string.
	ov, skel = s1Overlay(nil)
	if got := AlignDigest(ov, skel); got != EntryDigest(nil) {
		t.Fatalf("empty overlay digest = %s", got)
	}
}

func TestAlignDigestInsertedAndEscapes(t *testing.T) {
	ov, skel := s1Overlay([]Link{InsertedLink(), NullLink(), Single(2)})
	ov.Paragraphs[0].S[0].X = [][]int{{1, 6, 0}, {7, 8}}
	want := EntryDigest([]byte("\tТогда\ncan\tмогу\nThen\tТогда\n\tя\n"))
	if got := AlignDigest(ov, skel); got != want {
		t.Fatalf("digest with [] and x = %s, want %s", got, want)
	}
}

func TestHighlightRanges(t *testing.T) {
	tr := &Translation{T: "“O zaman bitişik odada bekleyebilirim.”",
		A:     []Link{Single(0), Single(0), Multi(5, 6), Multi(4, 7), Multi(1, 2, 3)},
		Parts: []Part{{4, 0, 5, Single(3)}, {4, 5, 10, Single(2)}, {4, 10, 12, InsertedLink()}, {4, 12, 14, Single(1)}}}
	toks := TargetTokens(tr)
	rs := []rune(tr.T)
	text := func(rr [][2]int) []string {
		var out []string
		for _, r := range rr {
			out = append(out, string(rs[r[0]:r[1]]))
		}
		return out
	}
	if got := text(HighlightRanges(tr, toks, 0)); !reflect.DeepEqual(got, []string{"O", "zaman"}) {
		t.Errorf("Then → %q", got)
	}
	if got := text(HighlightRanges(tr, toks, 3)); !reflect.DeepEqual(got, []string{"bekle"}) {
		t.Errorf("wait → %q (parts: smallest containing wins)", got)
	}
	if got := text(HighlightRanges(tr, toks, 6)); !reflect.DeepEqual(got, []string{"bitişik"}) {
		t.Errorf("next → %q", got)
	}
	if Label([]int{1, 2}) != "run" || Label([]int{1, 10}) != "split" || Label([]int{3}) != "" {
		t.Error("Label")
	}
}

func TestLinkJSON(t *testing.T) {
	tr := Translation{T: "x", A: []Link{NullLink(), InsertedLink(), Single(3), Multi(1, 2)}}
	b, err := marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"t":"x","a":[null,[],3,[1,2]]}` {
		t.Fatalf("marshal: %s", b)
	}
	var back Translation
	if err := unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.A, tr.A) {
		t.Fatalf("round trip: %+v", back.A)
	}
	var one Link
	if err := unmarshal([]byte("[5]"), &one); err != nil || !reflect.DeepEqual(one.Words, []int{5}) {
		t.Fatalf("[5] → %+v %v", one, err)
	}
}
