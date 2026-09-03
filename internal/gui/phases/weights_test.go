package phases

import (
	"math"
	"testing"
)

func near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}

func TestNewPlanRenormalizes(t *testing.T) {
	full := NewPlan(true, true, true)
	wantFull := []phaseWeight{
		{"translate", 0.16}, {"repair", 0.47}, {"embalign", 0.26},
		{"align", 0.07}, {"judge", 0.02}, {"assemble", 0.02},
	}
	if len(full.phases) != len(wantFull) {
		t.Fatalf("full plan has %d phases, want %d", len(full.phases), len(wantFull))
	}
	for i, w := range wantFull {
		got := full.phases[i]
		if got.name != w.name || !near(got.weight, w.weight) {
			t.Errorf("full plan[%d] = %v, want %v", i, got, w)
		}
	}

	// Without repair/judge/embalign the rest rescales: .16/.25, .07/.25, .02/.25.
	lean := NewPlan(false, false, false)
	wantLean := []phaseWeight{{"translate", 0.64}, {"align", 0.28}, {"assemble", 0.08}}
	if len(lean.phases) != len(wantLean) {
		t.Fatalf("lean plan has %d phases, want %d", len(lean.phases), len(wantLean))
	}
	sum := 0.0
	for i, w := range wantLean {
		got := lean.phases[i]
		if got.name != w.name || !near(got.weight, w.weight) {
			t.Errorf("lean plan[%d] = %v, want %v", i, got, w)
		}
		sum += got.weight
	}
	if !near(sum, 1) {
		t.Errorf("lean plan weights sum to %v, want 1", sum)
	}
}

// checkAt asserts the percent after one update, within float tolerance.
func checkAt(t *testing.T, tr *Tracker, phase, target string, done, total int, want float64) {
	t.Helper()
	tr.Update(phase, target, done, total)
	if got := tr.Percent(); !near(got, want) {
		t.Errorf("after %s|%s %d/%d: percent = %v, want %v", phase, target, done, total, got, want)
	}
}

func TestSingleTargetFullRun(t *testing.T) {
	tr := NewTracker(NewPlan(true, true, true), []string{"ru"})
	checkAt(t, tr, "translate", "ru", 0, 100, 0)
	checkAt(t, tr, "translate", "ru", 50, 100, 8)
	checkAt(t, tr, "translate", "ru", 100, 100, 16)
	checkAt(t, tr, "repair", "ru", 100, 200, 39.5)
	checkAt(t, tr, "repair", "ru", 200, 200, 63)
	checkAt(t, tr, "embalign", "ru", 500, 1000, 76)
	checkAt(t, tr, "embalign", "ru", 1000, 1000, 89)
	checkAt(t, tr, "align", "ru", 5, 10, 92.5)
	checkAt(t, tr, "judge", "ru", 2, 4, 97) // judge totals are batches; the ratio is unitless
	checkAt(t, tr, "assemble", "", 0, 1, 98)
	checkAt(t, tr, "assemble", "", 1, 1, 100)
	checkAt(t, tr, "done", "", 0, 0, 100)
}

func TestRenormalizedRun(t *testing.T) {
	tr := NewTracker(NewPlan(false, false, false), []string{"ru"})
	checkAt(t, tr, "translate", "ru", 100, 100, 64)
	// Events for dropped phases are ignored, not counted.
	checkAt(t, tr, "repair", "ru", 10, 20, 64)
	checkAt(t, tr, "embalign", "ru", 1, 2, 64)
	checkAt(t, tr, "align", "ru", 1, 2, 78)
	checkAt(t, tr, "align", "ru", 2, 2, 92)
	checkAt(t, tr, "assemble", "", 1, 1, 100)
}

func TestMultiTargetInterleave(t *testing.T) {
	tr := NewTracker(NewPlan(false, false, false), []string{"ru", "de"})
	checkAt(t, tr, "translate", "ru", 100, 100, 32) // each target carries weight/2
	checkAt(t, tr, "translate", "de", 50, 100, 48)
	checkAt(t, tr, "align", "ru", 5, 10, 55)
	checkAt(t, tr, "align", "de", 0, 10, 71) // de's translate now counts complete despite 50/100
	checkAt(t, tr, "assemble", "", 1, 1, 100)
}

// A phase that ends short (done<total) counts as full weight once the next
// phase for that target starts reporting.
func TestShortPhaseCompletesOnNextPhase(t *testing.T) {
	tr := NewTracker(NewPlan(false, false, true), []string{"ru"})
	wTranslate := 100 * 0.16 / 0.51
	tr.Update("translate", "ru", 80, 100)
	if got := tr.Percent(); !near(got, wTranslate*0.8) {
		t.Fatalf("translate 80/100 = %v, want %v", got, wTranslate*0.8)
	}
	tr.Update("embalign", "ru", 0, 50)
	if got := tr.Percent(); !near(got, wTranslate) {
		t.Errorf("after embalign starts: percent = %v, want translate's full %v", got, wTranslate)
	}
}

func TestMonotonicAndUnknownPhase(t *testing.T) {
	tr := NewTracker(NewPlan(true, true, true), []string{"ru"})
	tr.Update("translate", "ru", 80, 100)
	p1 := tr.Percent()
	if !near(p1, 12.8) {
		t.Fatalf("translate 80/100 = %v, want 12.8", p1)
	}
	// A regressing report never moves the bar backwards.
	tr.Update("translate", "ru", 60, 100)
	if got := tr.Percent(); got != p1 {
		t.Errorf("after regressing report: percent = %v, want clamped %v", got, p1)
	}
	// An unknown phase is ignored.
	tr.Update("frobnicate", "ru", 1, 2)
	if got := tr.Percent(); got != p1 {
		t.Errorf("after unknown phase: percent = %v, want %v", got, p1)
	}
	// Zero total contributes nothing (and must not divide by zero).
	tr.Update("repair", "ru", 0, 0)
	if got := tr.Percent(); !near(got, 16) {
		t.Errorf("repair 0/0 = %v, want 16 (translate complete, repair at 0)", got)
	}
}

func TestLabel(t *testing.T) {
	cases := map[string]string{
		"translate": "Translating sentences",
		"repair":    "Proofreading the translation",
		"embalign":  "Aligning words (fast pass)",
		"align":     "Aligning words",
		"judge":     "Reviewing translation quality",
		"assemble":  "Assembling the book",
		"done":      "Finishing up",
		"nope":      "",
	}
	for phase, want := range cases {
		if got := Label(phase); got != want {
			t.Errorf("Label(%q) = %q, want %q", phase, got, want)
		}
	}
}
