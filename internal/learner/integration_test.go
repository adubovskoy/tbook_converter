package learner

import (
	"math"
	"os"
	"sync"
	"testing"

	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/dimando/reader/converter/internal/tbook/v2/migrate"
)

// The two books the 2026-09 panel measured. Both tools are held to the numbers
// it published, so a change in the loaders, the tokenizer or the class rules
// shows up here: panel/phase2/critic-learner-ux.md T0 (the six classes),
// panel/phase3/judge-learner-first.md §1 (the coarse collapse) and
// panel/phase2/critic-producer-economics.md PE-10 (consensus).
const (
	samplePath = "../../../android/app/src/main/assets/sample.tbook"
	kimiPath   = "../../bench-quality/full-kimi.tbook"
	// TBOOK_V2_SAMPLE points at a version-2 copy of sample.tbook; without it
	// the test migrates one itself.
	v2SampleEnv = "TBOOK_V2_SAMPLE"
)

// tolerance of the published shares, in percentage points. The panel scored
// chapter paragraphs only; these tools also score the two table cells of
// sample.tbook, which moves a share by at most 0.02 pp.
const tolPP = 0.2

// Decoding sample.tbook takes seconds, so every test here shares one copy.
var (
	sampleRaw = sync.OnceValues(func() (*tbook.Book, error) { return tbook.Read(samplePath) })
	sampleV1  = sync.OnceValues(func() (*Book, error) {
		raw, err := sampleRaw()
		if err != nil {
			return nil, err
		}
		return FromV1Book(samplePath, raw), nil
	})
)

func sampleOrSkip(t *testing.T) *Book {
	t.Helper()
	skipMissing(t, samplePath)
	b, err := sampleV1()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func loadOrSkip(t *testing.T, path string) *Book {
	t.Helper()
	skipMissing(t, path)
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func skipMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("%s is not in this checkout", path)
	}
}

func near(t *testing.T, what string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.2f, want %.2f (±%.2f)", what, got, want, tol)
	}
}

// panelRow is one row of T0: the six class shares plus content-word silent.
type panelRow struct {
	clean, cleanMulti, shared, smear, none, dead, contentSilent float64
}

var sampleT0 = map[string]panelRow{
	"ru": {51.35, 3.27, 40.57, 0.66, 4.15, 0.00, 2.83},
	"uk": {52.02, 2.80, 41.21, 0.46, 3.51, 0.00, 2.91},
	"tr": {33.23, 3.74, 56.63, 0.57, 5.83, 0.00, 3.57},
	"de": {72.89, 4.33, 18.94, 0.60, 3.10, 0.14, 3.07},
	"fr": {64.93, 6.69, 23.55, 1.64, 3.18, 0.00, 2.73},
	"es": {62.90, 6.23, 25.53, 1.30, 3.00, 1.04, 3.56},
	"it": {58.60, 5.86, 31.32, 1.01, 3.19, 0.01, 2.83},
	"pt": {63.20, 5.46, 27.89, 0.94, 2.50, 0.00, 2.18},
}

func checkRow(t *testing.T, row *LangLedger, want panelRow) {
	t.Helper()
	near(t, row.Lang+" clean", row.Pct["clean"], want.clean, tolPP)
	near(t, row.Lang+" clean-multi", row.Pct["clean-multi"], want.cleanMulti, tolPP)
	near(t, row.Lang+" shared", row.Pct["shared"], want.shared, tolPP)
	near(t, row.Lang+" smear", row.Pct["smear"], want.smear, tolPP)
	near(t, row.Lang+" none", row.Pct["none"], want.none, tolPP)
	near(t, row.Lang+" dead-sentence", row.Pct["dead-sentence"], want.dead, tolPP)
	near(t, row.Lang+" content-word silent", row.ContentSilentPct, want.contentSilent, tolPP)
}

func TestSampleTapLedger(t *testing.T) {
	rep := sampleOrSkip(t).TapLedger(LedgerOpts{})
	if len(rep.Languages) != len(sampleT0) {
		t.Fatalf("languages = %d, want %d", len(rep.Languages), len(sampleT0))
	}
	for _, row := range rep.Languages {
		want, ok := sampleT0[row.Lang]
		if !ok {
			t.Fatalf("unexpected target %q", row.Lang)
		}
		checkRow(t, row, want)
	}
	// The coarse collapse over all 8 languages (judge-learner-first §1).
	c := rep.Total.Coarse
	near(t, "coarse clean", c.Pct["clean"], 62.58, tolPP)
	near(t, "coarse shared", c.Pct["shared"], 33.37, tolPP)
	near(t, "coarse dead", c.Pct["dead"], 3.70, tolPP)
	near(t, "coarse no-translation", c.Pct["noTranslation"], 0.36, tolPP)
}

func TestKimiTapLedger(t *testing.T) {
	rep := loadOrSkip(t, kimiPath).TapLedger(LedgerOpts{})
	if len(rep.Languages) != 1 {
		t.Fatalf("languages = %d, want 1 (ru)", len(rep.Languages))
	}
	checkRow(t, rep.Languages[0], panelRow{52.21, 4.21, 39.34, 0.88, 3.35, 0.00, 2.27})
	// full-kimi's published triple is the coarse collapse, not the six-class
	// ledger: judge-learner-first §1 counts a tap shared as soon as a token it
	// lights is claimed by another source word, even by a second chunk.
	c := rep.Languages[0].Coarse
	near(t, "kimi coarse clean", c.Pct["clean"], 56.95, tolPP)
	near(t, "kimi coarse shared", c.Pct["shared"], 39.69, tolPP)
	near(t, "kimi coarse dead", c.Pct["dead"], 3.35, tolPP)
}

// The migration to version 2 must preserve the ledger: the same book scored
// through the version-2 tap algorithm lands within a rounding step of the
// version-1 numbers.
func TestV2LedgerMatchesV1(t *testing.T) {
	v1 := sampleOrSkip(t)
	var v2b *Book
	if path := os.Getenv(v2SampleEnv); path != "" {
		var err error
		if v2b, err = Load(path); err != nil {
			t.Fatal(err)
		}
		if v2b.Version != 2 {
			t.Fatalf("%s is formatVersion %d, want 2", path, v2b.Version)
		}
	} else {
		raw, err := sampleRaw()
		if err != nil {
			t.Fatal(err)
		}
		arch, _, err := migrate.Convert(raw, migrate.Options{})
		if err != nil {
			t.Fatalf("migrate: %v", err)
		}
		v2b = FromV2Archive(samplePath+" (migrated)", arch)
	}
	a, b := v1.TapLedger(LedgerOpts{}), v2b.TapLedger(LedgerOpts{})
	rows := map[string]*LangLedger{}
	for _, row := range b.Languages {
		rows[row.Lang] = row
	}
	// Both tokenizers disagree only about digit runs, which version 2 makes
	// tappable: a handful of taps per language, hence the loose 0.06 pp.
	const tol = 0.06
	for _, row := range a.Languages {
		other, ok := rows[row.Lang]
		if !ok {
			t.Errorf("version 2 has no target %q", row.Lang)
			continue
		}
		for _, name := range ClassNames {
			near(t, "v2 "+row.Lang+" "+name, other.Pct[name], row.Pct[name], tol)
		}
		near(t, "v2 "+row.Lang+" content-word silent", other.ContentSilentPct, row.ContentSilentPct, tol)
	}
}

func TestSampleConsensus(t *testing.T) {
	b := sampleOrSkip(t)
	// PE-10 was measured with 6 of the 8 languages required per sentence.
	r, err := b.Consensus(ConsensusOpts{MinLangs: 6})
	if err != nil {
		t.Fatal(err)
	}
	near(t, "unanimous", r.UnanimousPct, 82.59, tolPP)
	near(t, "missed", r.MissedPct, 17.41, tolPP)
	countNear(t, "solo misses", r.Solo, 11844, 40)
	countNear(t, "unanimous misses", r.UnanimousMiss, 36, 5)
	for lang, want := range map[string]int{
		"tr": 3660, "ru": 1520, "de": 1331, "fr": 1274,
		"it": 1241, "uk": 1144, "es": 979, "pt": 695,
	} {
		countNear(t, "solo misses "+lang, r.SoloByLang[lang], want, 20)
	}

	rc, err := b.Consensus(ConsensusOpts{MinLangs: 6, ContentOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	countNear(t, "content slots", rc.Slots, 51635, 100)
	countNear(t, "content solo misses", rc.Solo, 4610, 40)
	countNear(t, "content unanimous misses", rc.UnanimousMiss, 17, 5)
}

func countNear(t *testing.T, what string, got, want, tol int) {
	t.Helper()
	if d := got - want; d > tol || d < -tol {
		t.Errorf("%s = %d, want %d (±%d)", what, got, want, tol)
	}
}
