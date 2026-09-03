package pricing

import "testing"

// A mid-size novel, the Altered Carbon anchor.
const (
	novelChars     = 874304
	novelSentences = 11114
)

func TestQuoteKinds(t *testing.T) {
	cases := []struct {
		provider string
		kind     string
		display  string
	}{
		{"claude", "included", "Included in your plan"},
		{"ollama", "free", "Free"},
		{"llamacpp", "free", "Free"},
	}
	for _, c := range cases {
		q := Quote(novelChars, novelSentences, "en", "ru", c.provider, pinnedIn, pinnedOut, false)
		if q.Kind != c.kind || q.Display != c.display || q.USD != 0 {
			t.Errorf("%s: got %+v, want kind %q display %q usd 0", c.provider, q, c.kind, c.display)
		}
	}
}

func TestQuoteGonka(t *testing.T) {
	q := Quote(novelChars, novelSentences, "en", "ru", "gonka", pinnedIn, pinnedOut, false)
	want := 2 * RawCost(novelChars, novelSentences, "en", "ru", GonkaTokenPrice, GonkaTokenPrice)
	if q.Kind != "usd" || !near(q.USD, want) {
		t.Errorf("gonka quote = %+v, want kind usd, USD %v", q, want)
	}
	if q.USD >= 0.01 {
		t.Fatalf("gonka novel quote %.6f is not tiny — the floor test below is meaningless", q.USD)
	}
	if q.Display != "≈ $0.01" {
		t.Errorf("gonka display = %q, want the ≈ $0.01 floor", q.Display)
	}
	// repairOn changes nothing: the ×2 already prices gonka's auto-repair.
	if r := Quote(novelChars, novelSentences, "en", "ru", "gonka", pinnedIn, pinnedOut, true); r.USD != q.USD {
		t.Errorf("gonka with repairOn = %v, want %v (unchanged)", r.USD, q.USD)
	}
}

func TestQuoteOpenRouter(t *testing.T) {
	q := Quote(novelChars, novelSentences, "en", "ru", "openrouter", pinnedIn, pinnedOut, false)
	want := RawCost(novelChars, novelSentences, "en", "ru", pinnedIn, pinnedOut)
	if q.Kind != "usd" || !near(q.USD, want) {
		t.Errorf("openrouter quote = %+v, want kind usd, USD %v", q, want)
	}

	// Repair on a metered provider adds exactly one translate-pass term.
	r := Quote(novelChars, novelSentences, "en", "ru", "openrouter", pinnedIn, pinnedOut, true)
	wantRepair := want + repairCost(novelChars, novelSentences, "en", "ru", pinnedIn, pinnedOut)
	if !near(r.USD, wantRepair) {
		t.Errorf("openrouter+repair = %v, want %v", r.USD, wantRepair)
	}
	if r.USD <= q.USD {
		t.Errorf("repair did not raise the quote: %v <= %v", r.USD, q.USD)
	}
}

func TestDisplayFormatting(t *testing.T) {
	cases := []struct {
		usd  float64
		want string
	}{
		{1.11, "≈ $1.28"},   // 1.11×1.15 = 1.2765 → ceil to cents
		{1.00, "≈ $1.15"},   // exact cent boundary survives the epsilon guard
		{0.008, "≈ $0.01"},  // 0.0092 rounds up to the floor
		{0.0001, "≈ $0.01"}, // tiny metered amount floors at one cent
		{0, "≈ $0.01"},
	}
	for _, c := range cases {
		if got := display(c.usd); got != c.want {
			t.Errorf("display(%v) = %q, want %q", c.usd, got, c.want)
		}
	}
}
