package pricing

import (
	"math"
	"testing"
)

// Pinned prices used by converter_web's calibration tests (SPEC §5):
// $0.30/M prompt, $2.50/M completion.
const (
	pinnedIn  = 0.0000003
	pinnedOut = 0.0000025
)

func near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}

// TestRawCostHandComputed pins RawCost against values derived by hand from
// the formula, so a silent porting drift from converter_web fails loudly.
func TestRawCostHandComputed(t *testing.T) {
	cases := []struct {
		name      string
		chars     int64
		sentences int
		src, tgt  string
		pIn, pOut float64
		want      float64
	}{
		// en→ru: srcTok 400, trTok 525 (pair override 1.05, cyrillic 3.2);
		// translate 982/556.5, align +842.75/+294, judge +487/+4.8,
		// glossary +5000/+500 → in 7311.75, out 1355.3.
		{"en-ru", 1600, 10, "en", "ru", 1e-6, 2e-6, 0.01002235},
		// de→en: srcTok 1000, trTok 950 (de→latin 0.95); expectation produced by
		// running converter_web's own RawCost on this tuple.
		{"de-en", 4000, 32, "de", "en", 3e-7, 2.5e-6, 0.00774905},
		// en→zh: srcTok 250, trTok 250 (expand 0.45, cpt 1.8); translate 820/265,
		// align +802.5/+140, judge +445/+3.84, glossary → in 7067.5, out 908.84.
		{"en-zh", 1000, 8, "en", "zh", 1e-6, 1e-6, 0.00797634},
	}
	for _, c := range cases {
		got := RawCost(c.chars, c.sentences, c.src, c.tgt, c.pIn, c.pOut)
		if !near(got, c.want) {
			t.Errorf("%s: RawCost = %.10f, want %.10f", c.name, got, c.want)
		}
	}
}

func TestRawCostZeroInput(t *testing.T) {
	if got := RawCost(0, 100, "en", "ru", pinnedIn, pinnedOut); got != 0 {
		t.Errorf("RawCost with 0 chars = %v, want 0", got)
	}
	if got := RawCost(100, 0, "en", "ru", pinnedIn, pinnedOut); got != 0 {
		t.Errorf("RawCost with 0 sentences = %v, want 0", got)
	}
}

// Ported calibration anchors from converter_web pricing_test.go: real all-in
// costs bound the raw estimate.
func TestCalibrationAlteredCarbon(t *testing.T) {
	raw := RawCost(874304, 11114, "en", "ru", pinnedIn, pinnedOut)
	if raw < 1.00 || raw > 2.20 {
		t.Fatalf("Altered Carbon en→ru rawCost = %.4f, want in [1.00, 2.20]", raw)
	}
}

func TestCalibrationRose(t *testing.T) {
	raw := RawCost(1100000, 11034, "de", "ru", pinnedIn, pinnedOut)
	if raw < 1.20 || raw > 2.40 {
		t.Fatalf("Rose de→ru rawCost = %.4f, want in [1.20, 2.40]", raw)
	}
}

// TestRepairCostIsTranslateShare: the repair approximation equals exactly one
// more translate pass — RawCost minus its align/judge/glossary terms.
func TestRepairCostIsTranslateShare(t *testing.T) {
	const (
		chars     = int64(874304)
		sentences = 11114
	)
	got := repairCost(chars, sentences, "en", "ru", pinnedIn, pinnedOut)

	c, s := float64(chars), float64(sentences)
	inTok := math.Ceil(s/16)*550 + (c/4.0)*1.08
	outTok := (c * 1.05 / 3.2) * 1.06
	want := inTok*pinnedIn + outTok*pinnedOut
	if !near(got, want) {
		t.Errorf("repairCost = %.10f, want translate share %.10f", got, want)
	}
}
