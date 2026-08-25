package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAcceptGlossEntryGates(t *testing.T) {
	cases := []struct {
		name   string
		entry  GlossEntry
		target string
		want   bool
	}{
		{"ordinary entry", GlossEntry{Src: "Ortega", Tgt: "Ортега"}, "ru", true},
		{"coined term", GlossEntry{Src: "neurachem", Tgt: "нейрохим"}, "ru", true},
		{"rendering equals source", GlossEntry{Src: "learnt", Tgt: "learnt"}, "ru", false},
		{"all-Latin rendering", GlossEntry{Src: "JacSol", Tgt: "JacSol"}, "ru", false},
		{"mixed script kept", GlossEntry{Src: "SISS", Tgt: "СИСС (SISS)"}, "ru", true},
		{"render mix-up", GlossEntry{Src: "safeguard", Tgt: "Институт Сильвеста"}, "ru", false},
		{"lowercase to lowercase", GlossEntry{Src: "stack", Tgt: "стек"}, "ru", true},
		{"function word", GlossEntry{Src: "into", Tgt: "внутрь"}, "ru", false},
		{"empty rendering", GlossEntry{Src: "Ortega", Tgt: "  "}, "ru", false},
		// Latin target: the script check has no opinion, so a Latin rendering of
		// a Latin term is judged only by the equal-to-source rule.
		{"latin target keeps rendering", GlossEntry{Src: "Ortega", Tgt: "Ortega-máquina"}, "es", true},
		// German capitalises every noun, so the lowercase-to-capitalised rule
		// must not fire there.
		{"german noun", GlossEntry{Src: "stack", Tgt: "Stapel"}, "de", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := acceptGlossEntry(c.entry, "en", c.target); got != c.want {
				t.Errorf("acceptGlossEntry(%q → %q, %s) = %v, want %v",
					c.entry.Src, c.entry.Tgt, c.target, got, c.want)
			}
		})
	}
}

func TestAcceptGender(t *testing.T) {
	strongF := genderEvidence{Gender: "f", Evidence: 200, Confidence: 0.53}
	weakM := genderEvidence{Gender: "m", Evidence: 4, Confidence: 1.0}
	confidentM := genderEvidence{Gender: "m", Evidence: 40, Confidence: 0.9}

	cases := []struct {
		name string
		rep  glossRenderReply
		ev   map[string]genderEvidence
		want string
	}{
		{"model tags a person", glossRenderReply{Kind: "person", Gender: "f"}, nil, "f"},
		{"not a person", glossRenderReply{Kind: "thing", Gender: "f"}, nil, ""},
		{"category of people", glossRenderReply{Kind: "org", Gender: "m"}, nil, ""},
		{"miner contradicts on strong evidence",
			glossRenderReply{Kind: "person", Gender: "m"},
			map[string]genderEvidence{"X": strongF}, ""},
		{"miner agrees despite a split",
			glossRenderReply{Kind: "person", Gender: "f"},
			map[string]genderEvidence{"X": strongF}, "f"},
		{"miner too thin to veto",
			glossRenderReply{Kind: "person", Gender: "f"},
			map[string]genderEvidence{"X": weakM}, "f"},
		{"miner alone, confident",
			glossRenderReply{Kind: "person"},
			map[string]genderEvidence{"X": confidentM}, "m"},
		{"miner alone, too thin",
			glossRenderReply{Kind: "person"},
			map[string]genderEvidence{"X": weakM}, ""},
		{"no evidence at all", glossRenderReply{Kind: "person"}, nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := acceptGender("X", c.rep, c.ev); got != c.want {
				t.Errorf("acceptGender = %q, want %q", got, c.want)
			}
		})
	}

	// A full name has no evidence of its own; the veto must reach it through
	// the surname, or "Miriam Bancroft" would be tagged from the model alone.
	ev := map[string]genderEvidence{"Bancroft": strongF}
	if got := acceptGender("Laurens Bancroft", glossRenderReply{Kind: "person", Gender: "m"}, ev); got != "" {
		t.Errorf("a shared surname must veto the full name too, got %q", got)
	}
	if got := acceptGender("Miriam Bancroft", glossRenderReply{Kind: "person", Gender: "f"}, ev); got != "f" {
		t.Errorf("agreement with the surname evidence should stand, got %q", got)
	}
}

func TestGenderMarkingTarget(t *testing.T) {
	for _, tg := range []string{"ru", "uk", "pl", "es", "fr", "it", "pt", "ar", " RU "} {
		if !GenderMarkingTarget(tg) {
			t.Errorf("%q should mark gender", tg)
		}
	}
	for _, tg := range []string{"en", "zh", "ja", "ko", "tr", "fi", "hu", "nl"} {
		if GenderMarkingTarget(tg) {
			t.Errorf("%q should not mark gender", tg)
		}
	}
}

// A glossary without gender must render byte-for-byte as it did before the
// annotation existed: the cache namespace does not change, so translations made
// under a hand-edited (gender-free) sidecar stay valid.
func TestGlossaryBlockUnchangedWithoutGender(t *testing.T) {
	g := []GlossEntry{{Src: "stack", Tgt: "стэк"}, {Src: "Ortega", Tgt: "Ортега"}}
	want := "\n\nGLOSSARY — use these Russian translations consistently wherever the term appears:\n" +
		"- stack → стэк\n- Ortega → Ортега\n"
	if got := glossaryBlock("Russian", g); got != want {
		t.Errorf("glossary block changed shape:\n got %q\nwant %q", got, want)
	}
	if strings.Contains(translateSystemPrompt("English", "Russian", g), "[male]") {
		t.Error("a gender-free glossary must not mention the tag")
	}
}

func TestGlossaryBlockWithGender(t *testing.T) {
	g := []GlossEntry{
		{Src: "Ortega", Tgt: "Ортега", Gender: "f", Kind: "person"},
		{Src: "Kovacs", Tgt: "Ковач", Gender: "m", Kind: "person"},
		{Src: "neurachem", Tgt: "нейрохим"},
	}
	got := glossaryBlock("Russian", g)
	for _, want := range []string{
		"A [male] / [female] tag gives the gender",
		"- Ortega → Ортега  [female]\n",
		"- Kovacs → Ковач  [male]\n",
		"- neurachem → нейрохим\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("block missing %q:\n%s", want, got)
		}
	}
	// The proofread prompt carries the same tags plus its own instruction.
	rep := repairSystemPrompt("English", "Russian", g, 0)
	if !strings.Contains(rep, "[female]") || !strings.Contains(rep, "FIX it") {
		t.Error("repair prompt should carry the tags and the fix-it rule")
	}
	if !strings.Contains(rep, "The one exception is a person the GLOSSARY below tags") {
		t.Error("repair prompt should carve the tag out of the no-guessing rule")
	}
}

func TestGlossHashCoversGender(t *testing.T) {
	a := []GlossEntry{{Src: "Ortega", Tgt: "Ортега"}}
	b := []GlossEntry{{Src: "Ortega", Tgt: "Ортега", Gender: "f"}}
	c := []GlossEntry{{Src: "Ortega", Tgt: "Ортега", Gender: "m"}}
	if GlossHash(a) == GlossHash(b) {
		t.Error("adding a gender must change the cache namespace")
	}
	if GlossHash(b) == GlossHash(c) {
		t.Error("flipping a gender must change the cache namespace")
	}
	// Kind never reaches the prompt, so it must NOT invalidate translations.
	d := []GlossEntry{{Src: "Ortega", Tgt: "Ортега", Kind: "person"}}
	if GlossHash(a) != GlossHash(d) {
		t.Error("kind must not change the cache namespace")
	}
}

func TestMergeGlossariesDedupesAndCaps(t *testing.T) {
	mined := []GlossEntry{
		{Src: "Ortega", Tgt: "Ортега", Gender: "f", Kind: "person"},
		{Src: "neurachem", Tgt: "нейрохим"},
	}
	sample := []GlossEntry{
		{Src: "ortega", Tgt: "Ортегова"},           // duplicate, other case
		{Src: "stack", Tgt: "стэк"},                // the sample pass's own find
		{Src: "Suntouch House", Tgt: "Дом Сантач"}, // phrase
		{Src: "learnt", Tgt: "learnt"},             // gated: equals source
	}
	o := GlossaryBuild{Source: "en", Target: "ru", Gender: true}
	got := mergeGlossaries(sample, mined, o)
	byLower := map[string]GlossEntry{}
	for _, e := range got {
		if _, dup := byLower[strings.ToLower(e.Src)]; dup {
			t.Errorf("duplicate entry for %q", e.Src)
		}
		byLower[strings.ToLower(e.Src)] = e
	}
	if e := byLower["ortega"]; e.Tgt != "Ортега" || e.Gender != "f" {
		t.Errorf("the mined entry must win the duplicate: %+v", e)
	}
	if _, ok := byLower["stack"]; !ok {
		t.Error("the sample pass's unique contribution must survive")
	}
	if _, ok := byLower["learnt"]; ok {
		t.Error("a gated entry must not survive")
	}

	// Gender off strips the annotation, so the prompt and the hash go back to
	// the pre-gender shape.
	off := mergeGlossaries(sample, mined, GlossaryBuild{Source: "en", Target: "ru"})
	for _, e := range off {
		if e.Gender != "" {
			t.Errorf("gender must be stripped when not wanted: %+v", e)
		}
	}
	// A target that does not mark gender strips it too.
	en := mergeGlossaries(nil, mined, GlossaryBuild{Source: "ru", Target: "en", Gender: true})
	for _, e := range en {
		if e.Gender != "" {
			t.Errorf("target en marks no gender: %+v", e)
		}
	}

	// The cap keeps head terms and drops the tail.
	many := make([]GlossEntry, 0, 12)
	for i := 0; i < 12; i++ {
		many = append(many, GlossEntry{Src: fmt.Sprintf("Name%02d", i), Tgt: fmt.Sprintf("Имя%02d", i)})
	}
	capped := mergeGlossaries(nil, many, GlossaryBuild{Source: "en", Target: "ru", Max: 5})
	if len(capped) != 5 {
		t.Errorf("cap not applied: got %d entries", len(capped))
	}
}

// renderCandidates must trust the id only when the reply echoes the term, the
// guard against the measured failure where a rendering was glued to the wrong
// candidate ("aside → Новый Комусо").
func TestRenderCandidatesDropsMismatchedEcho(t *testing.T) {
	reply := map[string]any{
		"0": map[string]string{"term": "Ortega", "tgt": "Ортега", "kind": "person", "gender": "f"},
		"1": map[string]string{"term": "premonitory", "tgt": "Волёва", "kind": "person"}, // wrong echo
		"2": map[string]string{"tgt": "нейрохим", "kind": "thing"},                       // no echo: id trusted
	}
	body, err := json.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		out, _ := json.Marshal(map[string]any{"choices": []any{
			map[string]any{"message": map[string]string{"content": string(body)}},
		}})
		fmt.Fprint(w, string(out))
	}))
	defer srv.Close()

	c := NewClient(Options{Provider: ProviderOpenRouter, BaseURL: srv.URL, APIKey: "k",
		Model: "test-model", JSONMode: true})
	cands := []glossCandidate{
		{Term: "Ortega", Kind: "name", Freq: 10},
		{Term: "Sajaki", Kind: "name", Freq: 9}, // id 1: reply echoes another term
		{Term: "neurachem", Kind: "coined", Freq: 8},
	}
	got, dropped, err := renderCandidates(context.Background(), c, cands,
		GlossaryBuild{Source: "en", Target: "ru", Gender: true})
	if err != nil {
		t.Fatalf("renderCandidates: %v", err)
	}
	if dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}
	if _, ok := got["Sajaki"]; ok {
		t.Error("a reply whose echo names another term must not land")
	}
	if got["Ortega"].Tgt != "Ортега" || got["Ortega"].Gender != "f" {
		t.Errorf("Ortega: %+v", got["Ortega"])
	}
	if got["neurachem"].Tgt != "нейрохим" {
		t.Errorf("an entry without an echo falls back to the id: %+v", got["neurachem"])
	}
}
