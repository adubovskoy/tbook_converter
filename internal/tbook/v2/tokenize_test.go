package v2

import (
	"reflect"
	"testing"
)

func TestTokenizeVectors(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		// §5.3
		{"“That’s the worst of it, Mr. Holmes, I don’t know.”",
			[]string{"That’s", "the", "worst", "of", "it", "Mr", "Holmes", "I", "don’t", "know"}},
		{"“‘Is £ 4 a week.", []string{"Is", "4", "a", "week"}},
		{"В этом-то всё и дело.", []string{"В", "этом-то", "всё", "и", "дело"}},
		{"„Er sieht nicht gerade wie eine Zierde für die Bow-Street-Zellen aus, was?“",
			[]string{"Er", "sieht", "nicht", "gerade", "wie", "eine", "Zierde", "für", "die", "Bow-Street-Zellen", "aus", "was"}},
		{". Да, Кинг-Эдвард-стрит, 17, рядом с С", []string{"Да", "Кинг-Эдвард-стрит", "17", "рядом", "с", "С"}},
		// Annex B.6
		{"Bow-Street-Zellen", []string{"Bow-Street-Zellen"}},
		{"30,000", []string{"30,000"}},
		{"5:15", []string{"5:15"}},
		{"l'avons", []string{"l'avons"}},
		{"don’t", []string{"don’t"}},
		{"этом-то", []string{"этом-то"}},
		{"£", nil},
		{"—", nil},
		{"…", nil},
		{"St. Paul’s", []string{"St", "Paul’s"}},
		{"3.14 and 1888'di, §7", []string{"3.14", "and", "1888'di", "7"}},
		{"", nil},
	}
	for _, c := range cases {
		got := TokenStrings(c.text)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Tokenize(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestTokenizeCodePoints(t *testing.T) {
	// Non-ASCII before the token: ranges are code points, not bytes.
	got := Tokenize("«Тогда я»")
	want := [][2]int{{1, 6}, {7, 8}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSentenceWordsMidWordCut(t *testing.T) {
	// S9: the boundary at 23 cuts "offices"; the token belongs to sentence 0.
	b := &Block{
		Text:  "“‘Oh, at his new offices. He did tell me the address. Yes, 17 King Edward Street, near St. Paul’s.’",
		Sents: [][2]int{{0, 23}, {23, 51}, {51, 88}, {88, 96}, {96, 99}},
	}
	words := SentenceWords(b)
	rs := []rune(b.Text)
	str := func(k int) []string {
		var out []string
		for _, w := range words[k] {
			out = append(out, string(rs[w[0]:w[1]]))
		}
		return out
	}
	if got := str(0); !reflect.DeepEqual(got, []string{"Oh", "at", "his", "new", "offices"}) {
		t.Errorf("sentence 0 words %q", got)
	}
	if got := str(1); !reflect.DeepEqual(got, []string{"He", "did", "tell", "me", "the", "address"}) {
		t.Errorf("sentence 1 words %q", got)
	}
	if got := str(2); !reflect.DeepEqual(got, []string{"Yes", "17", "King", "Edward", "Street", "near", "St"}) {
		t.Errorf("sentence 2 words %q", got)
	}
	if got := str(3); !reflect.DeepEqual(got, []string{"Paul’s"}) {
		t.Errorf("sentence 3 words %q", got)
	}
	if len(words[4]) != 0 {
		t.Errorf("sentence 4 should have no words, got %v", words[4])
	}
	if gaps := GapTokens(b); len(gaps) != 0 {
		t.Errorf("unexpected gap tokens %v", gaps)
	}
	b.Sents = [][2]int{{0, 23}, {28, 51}, {51, 88}, {88, 96}, {96, 99}} // "He" (26–28) now starts in a gap
	if gaps := GapTokens(b); len(gaps) != 1 {
		t.Errorf("want one gap token, got %v", gaps)
	}
}
