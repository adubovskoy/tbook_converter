package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
)

// fixtureBook builds the Annex B.1 book from the panel's canonical sentences:
// one chapter "Canonical sentences" with eleven paragraphs (S8 a heading).
// withSecond adds a second chapter holding the S9 paragraph again.
func fixtureBook(t *testing.T, withSecond bool) *tbook.Book {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "canonical-sentences.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fix map[string]struct {
		Sentence  *tbook.Sentence   `json:"sentence"`
		Paragraph []*tbook.Sentence `json:"paragraph"`
		Sentences []*tbook.Sentence `json:"sentences"`
	}
	if err := json.Unmarshal(raw, &fix); err != nil {
		t.Fatal(err)
	}
	order := []string{"S1_morphology_reorder", "S2_many_to_many", "S3_contraction_idiom_honorific", "S4_number_currency",
		"S5_separable_verb", "S6_idiom_split_romance", "S7_free_translation", "S8_wrong_language_q1",
		"S9_midword_segmentation", "S10_polysemy_case"}
	var paras [][]*tbook.Sentence
	langSet := map[string]bool{}
	for _, key := range order {
		v, ok := fix[key]
		if !ok {
			t.Fatalf("fixture lacks %s", key)
		}
		switch {
		case v.Paragraph != nil:
			paras = append(paras, v.Paragraph)
		case v.Sentences != nil:
			for _, s := range v.Sentences {
				paras = append(paras, []*tbook.Sentence{s})
			}
		default:
			paras = append(paras, []*tbook.Sentence{v.Sentence})
		}
	}
	for _, p := range paras {
		for _, s := range p {
			for lang := range s.Tr {
				langSet[lang] = true
			}
		}
	}
	var langs []string
	for l := range langSet {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	styles := make([]string, len(paras))
	styles[7] = tbook.RoleHeading
	b := &tbook.Book{Title: "Canonical sentences", Author: "Arthur Conan Doyle", Source: "en", Targets: langs,
		Chapters: []tbook.Chapter{{Title: "Canonical sentences", Paragraphs: paras, ParagraphStyles: styles}}}
	if withSecond {
		s9 := fix["S9_midword_segmentation"].Paragraph
		b.Chapters = append(b.Chapters, tbook.Chapter{Title: "Second", Paragraphs: [][]*tbook.Sentence{s9}})
	}
	return b
}

func linksJSON(t *testing.T, tr *v2.Translation) string {
	t.Helper()
	if tr == nil || tr.A == nil {
		return ""
	}
	b, err := json.Marshal(tr.A)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// tap returns the highlighted target strings for a tap on the source word
// spelled word (first occurrence unless nth > 0) of sentence k of paragraph p.
func tap(t *testing.T, arc *v2.Archive, p, k int, lang, word string, nth int) []string {
	t.Helper()
	blk := &arc.Skeletons[0].Paragraphs[p]
	words := v2.SentenceWords(blk)[k]
	src := []rune(blk.Text)
	idx := -1
	for i, w := range words {
		if string(src[w[0]:w[1]]) == word {
			if nth == 0 {
				idx = i
				break
			}
			nth--
		}
	}
	if idx < 0 {
		t.Fatalf("p%d s%d has no word %q", p, k, word)
	}
	tr := arc.Overlays[0][lang].Paragraphs[p].S[k]
	if tr == nil {
		return nil
	}
	rs := []rune(tr.T)
	out := []string{}
	for _, r := range v2.HighlightRanges(tr, v2.TargetTokens(tr), idx) {
		out = append(out, string(rs[r[0]:r[1]]))
	}
	return out
}

func TestFixtureIDsAndLinks(t *testing.T) {
	arc, stats, err := Convert(fixtureBook(t, false), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if arc.Manifest.Spine[0].ID != "cbedb517c" || arc.Skeletons[0].ID != "cbedb517c" {
		t.Fatalf("chapter id %s", arc.Manifest.Spine[0].ID)
	}
	wantIDs := []string{"04537723", "2b606e75", "1bfb4bea", "8d89345a", "0e2c5231", "ff1de7b8", "ea90ea77", "0f4552b1", "781304bb", "0982e735", "b51a0a09"}
	for i, want := range wantIDs {
		if got := arc.Skeletons[0].Paragraphs[i].ID; got != want {
			t.Errorf("paragraph %d id %s, want %s", i, got, want)
		}
	}
	if arc.Skeletons[0].Paragraphs[7].Role != v2.RoleHeading {
		t.Error("S8 should be a heading")
	}
	s9 := arc.Skeletons[0].Paragraphs[8]
	if s9.Text != "“‘Oh, at his new offices. He did tell me the address. Yes, 17 King Edward Street, near St. Paul’s.’" {
		t.Errorf("S9 text %q", s9.Text)
	}
	if !reflect.DeepEqual(s9.Sents, [][2]int{{0, 23}, {23, 51}, {51, 88}, {88, 96}, {96, 99}}) {
		t.Errorf("S9 sents %v", s9.Sents)
	}
	if stats.JoinsWithoutSpace != 4 {
		t.Errorf("S9 needs 4 mid-word joins, got %d", stats.JoinsWithoutSpace)
	}

	ov := func(lang string) *v2.Overlay { return arc.Overlays[0][lang] }
	cases := []struct {
		p, k int
		lang string
		want string
	}{
		{0, 0, "ru", "[0,1,2,3,4,[5,6],7]"},
		{0, 0, "tr", "[0,0,[5,6],[4,7],[1,2,3]]"},
		{0, 0, "de", "[0,2,1,[4,5],[6,7],3]"},
		{1, 0, "ru", "[1,[0,1,2],2,1,[3,4]]"},
		{2, 0, "tr", "[1,[2,3],3,4,5,6,[7,8,9]]"},
		{2, 0, "es", "[0,0,[1,3],2,5,6,8,9,[7,9]]"},
		{3, 0, "tr", "[[0,2,3],1,2]"},
		{3, 0, "de", "[0,0,1,2,2,3]"},
		{3, 0, "ru", "[0,[0,2],2,3]"},
		{4, 0, "de", "[0,2,1,null,null,3,4,5,6,[7,8,9],2,[10,11]]"},
		{4, 0, "ru", "[1,2,null,0,4,9,5,[6,7,8],[10,11]]"},
		{5, 0, "ru", "[0,0,1,3,2,4,4,4,5,[6,7],10,[8,9],[10,12],[10,11],13,14,15,16]"},
		{6, 0, "ru", "[1,[0,2],2]"},
		{7, 0, "ru", "[1,[0,2],3]"},
		{8, 0, "ru", "[0,1,2,3,4]"},
		{8, 1, "ru", "[0,1,[1,2],3,[4,5]]"},
		{8, 2, "ru", "[0,[2,3,4],1,5,5,6]"},
		{8, 3, "ru", "[0]"},
		{8, 4, "ru", ""},
	}
	for _, c := range cases {
		if got := linksJSON(t, ov(c.lang).Paragraphs[c.p].S[c.k]); got != c.want {
			t.Errorf("p%d s%d %s: a = %s, want %s", c.p, c.k, c.lang, got, c.want)
		}
	}
	if tr := ov("ru").Paragraphs[8].S[4]; tr.S != "" || tr.T != ".»" {
		t.Errorf("S9 s4 ru (no tokens) must carry no status: %+v", tr)
	}
	if tr := ov("tr").Paragraphs[1].S[0]; tr == nil || tr.T == "" {
		t.Error("S2 tr present")
	}
	if tr := ov("es").Paragraphs[1].S[0]; tr != nil {
		t.Error("S2 has no es translation → null")
	}
	if len(ov("ru").Gates) != 0 || ov("ru").Paragraphs[0].S[0].V != nil {
		t.Error("migration writes no gates and no verdicts")
	}
	for _, ovl := range arc.Overlays[0] {
		for _, gp := range ovl.Paragraphs {
			for _, tr := range gp.S {
				if tr != nil && tr.S != "" && tr.S != v2.StatusUnaligned {
					t.Errorf("migration wrote status %q", tr.S)
				}
				for _, l := range tr2links(tr) {
					if l.Inserted {
						t.Error("migration wrote an inserted link")
					}
				}
			}
		}
	}
}

func tr2links(tr *v2.Translation) []v2.Link {
	if tr == nil {
		return nil
	}
	return tr.A
}

func TestFixtureTaps(t *testing.T) {
	arc, _, err := Convert(fixtureBook(t, false), Options{})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		p, k  int
		lang  string
		word  string
		nth   int
		want  []string
		label string
	}{
		{0, 0, "ru", "wait", 0, []string{"подождать"}, ""},
		{0, 0, "tr", "wait", 0, []string{"bekleyebilirim"}, ""},
		{0, 0, "de", "wait", 0, []string{"warten"}, ""},
		{0, 0, "tr", "I", 0, []string{"bekleyebilirim"}, ""},
		{0, 0, "tr", "can", 0, []string{"bekleyebilirim"}, ""},
		{1, 0, "ru", "just", 0, []string{"этом-то", "всё"}, "run"},
		{1, 0, "ru", "is", 0, []string{"В", "этом-то", "и"}, "split"},
		{2, 0, "tr", "of", 0, []string{"kötüsü", "de"}, "run"},
		{2, 0, "es", "of", 0, []string{"lo"}, ""},
		{2, 0, "tr", "don’t", 0, []string{"bilmiyorum"}, ""},
		{3, 0, "tr", "4", 0, []string{"4"}, ""},
		{3, 0, "de", "4", 0, []string{"4"}, ""},
		{3, 0, "ru", "4", 0, []string{}, ""},
		{3, 0, "ru", "Is", 0, []string{"Четыре", "фунта"}, "run"},
		{4, 0, "de", "look", 0, []string{"sieht", "aus"}, "split"},
		{4, 0, "ru", "look", 0, []string{"похоже"}, ""},
		{5, 0, "ru", "of", 0, []string{"конечно"}, ""},
		{5, 0, "ru", "course", 0, []string{"конечно"}, ""},
		{6, 0, "ru", "brought", 0, []string{}, ""},
		{6, 0, "ru", "rear", 0, []string{}, ""},
		{6, 0, "ru", "attendants", 0, []string{"сопровождающих", "замыкали"}, "run"},
		{8, 0, "ru", "offices", 0, []string{"офисе"}, ""},
		{8, 1, "ru", "He", 0, []string{"Он"}, ""},
		{8, 1, "ru", "address", 0, []string{"адрес"}, ""},
		{8, 2, "ru", "17", 0, []string{"17"}, ""},
		{8, 2, "ru", "St", 0, []string{"С"}, ""},
		{8, 3, "ru", "Paul’s", 0, []string{"Павлом"}, ""},
		{9, 0, "ru", "case", 0, []string{"дело"}, ""},
		{10, 0, "ru", "case", 0, []string{"футляр"}, ""},
		{10, 0, "ru", "case", 1, []string{}, ""},
	}
	for _, c := range cases {
		got := tap(t, arc, c.p, c.k, c.lang, c.word, c.nth)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("p%d s%d %s tap %q#%d → %q, want %q", c.p, c.k, c.lang, c.word, c.nth, got, c.want)
		}
		if c.label != "" {
			tr := arc.Overlays[0][c.lang].Paragraphs[c.p].S[c.k]
			blk := &arc.Skeletons[0].Paragraphs[c.p]
			src := []rune(blk.Text)
			var idx []int
			words := v2.SentenceWords(blk)[c.k]
			wi := -1
			n := c.nth
			for i, w := range words {
				if string(src[w[0]:w[1]]) == c.word {
					if n == 0 {
						wi = i
						break
					}
					n--
				}
			}
			for j, l := range tr.A {
				if l.Contains(wi) {
					idx = append(idx, j)
				}
			}
			if got := v2.Label(idx); got != c.label {
				t.Errorf("p%d %s %q label %q, want %q", c.p, c.lang, c.word, got, c.label)
			}
		}
	}
	// The S9 sentence 4 has no words; S8's word list is tappable in every language.
	if len(v2.SentenceWords(&arc.Skeletons[0].Paragraphs[8])[4]) != 0 {
		t.Error("S9 sentence 4 must have no words")
	}
}

func writeArchive(t *testing.T, arc *v2.Archive, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := arc.Write(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMigratedFixtureValidates(t *testing.T) {
	arc, _, err := Convert(fixtureBook(t, true), Options{})
	if err != nil {
		t.Fatal(err)
	}
	path := writeArchive(t, arc, "fixture.tbook")
	rep, err := v2.Validate(path)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK() {
		t.Fatalf("migrated fixture invalid: %v", rep.Violations)
	}
	for _, w := range rep.Warnings {
		if !strings.Contains(w.Msg, "overlay") && !strings.Contains(w.Rule, "spine") {
			t.Logf("warning: %s", w)
		}
	}
	f, err := v2.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if len(f.Manifest.Meta.Runs) != 1 || f.Manifest.Meta.Runs[0].Options["tokenizer"] != v2.TokenizerID ||
		f.Manifest.Meta.Runs[0].Options["convertedFrom"] != "tbook/1" {
		t.Errorf("migration run record: %+v", f.Manifest.Meta.Runs)
	}
	if f.Manifest.Spine[0].Paragraphs != 11 || f.Manifest.Spine[0].Sentences != 15 {
		t.Errorf("counts: %+v", f.Manifest.Spine[0])
	}
}

// Annex B.3: permuting the links of one sentence of a migrated file passes
// every shape and range check and fails on alignDigest only.
func TestPermutedMigratedFile(t *testing.T) {
	arc, _, _ := Convert(fixtureBook(t, false), Options{})
	entries, err := arc.Entries()
	if err != nil {
		t.Fatal(err)
	}
	ru := arc.Overlays[0]["ru"]
	ru.Paragraphs[0].S[0].A = []v2.Link{v2.Single(7), v2.Single(1), v2.Single(2), v2.Single(3), v2.Single(4), v2.Multi(5, 6), v2.Single(0)}
	permuted, _ := json.Marshal(ru) // alignDigest still the sealed one
	replaced := false
	for i := range entries {
		if entries[i].Name == arc.Manifest.Spine[0].Gloss["ru"] {
			entries[i].Data = permuted
			replaced = true
		}
	}
	if !replaced {
		t.Fatal("ru overlay entry not found")
	}
	entries, _ = v2.RecomputeDigests(entries)
	path := filepath.Join(t.TempDir(), "permuted.tbook")
	if err := v2.WriteRaw(path, entries); err != nil {
		t.Fatal(err)
	}
	rep, _ := v2.Validate(path)
	if rules := rep.Rules(); !reflect.DeepEqual(rules, []string{"overlay.alignDigest"}) {
		t.Fatalf("want alignDigest only, got %v", rep.Violations)
	}
	ru.AlignDigest = ""
	if rep, _ := v2.Validate(writeArchive(t, arc, "resealed.tbook")); !rep.OK() {
		t.Fatalf("resealed permutation must pass: %v", rep.Violations)
	}
}

// Annex B.4 tamper tests on the migrated two-chapter fixture.
func TestTamperMigratedFile(t *testing.T) {
	fresh := func() (*v2.Archive, []v2.RawEntry) {
		arc, _, _ := Convert(fixtureBook(t, true), Options{})
		e, err := arc.Entries()
		if err != nil {
			t.Fatal(err)
		}
		return arc, e
	}
	idx := func(e []v2.RawEntry, name string) int {
		for i := range e {
			if e[i].Name == name {
				return i
			}
		}
		t.Fatalf("no %s", name)
		return -1
	}
	validate := func(e []v2.RawEntry, name string) *v2.Report {
		path := filepath.Join(t.TempDir(), name)
		if err := v2.WriteRaw(path, e); err != nil {
			t.Fatal(err)
		}
		rep, err := v2.Validate(path)
		if err != nil {
			t.Fatal(err)
		}
		return rep
	}
	rules := func(rep *v2.Report) string { return strings.Join(rep.Rules(), ",") }

	arc, e := fresh()
	e[idx(e, arc.Manifest.Spine[0].Gloss["ru"])].Data = e[idx(e, arc.Manifest.Spine[0].Gloss["tr"])].Data
	if rep := validate(e, "replace.tbook"); rep.OK() || !strings.Contains(rules(rep), "manifest.digests") || !strings.Contains(rules(rep), "overlay.lang") {
		t.Errorf("replace: %v", rep.Violations)
	}

	arc, e = fresh()
	i, j := idx(e, arc.Manifest.Spine[0].Text), idx(e, arc.Manifest.Spine[1].Text)
	e[i].Data, e[j].Data = e[j].Data, e[i].Data
	if rep := validate(e, "swap.tbook"); rep.OK() || !strings.Contains(rules(rep), "manifest.digests") || !strings.Contains(rules(rep), "skeleton.id") {
		t.Errorf("swap skeletons: %v", rep.Violations)
	}
	arc, e = fresh()
	i, j = idx(e, arc.Manifest.Spine[0].Gloss["ru"]), idx(e, arc.Manifest.Spine[1].Gloss["ru"])
	e[i].Data, e[j].Data = e[j].Data, e[i].Data
	e, _ = v2.RecomputeDigests(e)
	if rep := validate(e, "swap2.tbook"); rep.OK() || !strings.Contains(rules(rep), "overlay.binding") {
		t.Errorf("swap overlays (digests repaired): %v", rep.Violations)
	}

	arc, e = fresh()
	ru := arc.Overlays[0]["ru"]
	ru.Paragraphs = append(ru.Paragraphs[1:], ru.Paragraphs[len(ru.Paragraphs)-1])
	shifted, _ := json.Marshal(ru)
	e[idx(e, arc.Manifest.Spine[0].Gloss["ru"])].Data = shifted
	e, _ = v2.RecomputeDigests(e)
	if rep := validate(e, "shift.tbook"); rep.OK() || !strings.Contains(rules(rep), "overlay.binding") {
		t.Errorf("shift: %v", rep.Violations)
	}

	_, e = fresh()
	e = append(e, v2.RawEntry{Name: v2.ManifestEntry, Data: e[len(e)-1].Data})
	if rep := validate(e, "dup.tbook"); rep.OK() || !strings.Contains(rules(rep), "container.names") {
		t.Errorf("duplicate manifest: %v", rep.Violations)
	}
}

func TestJoinWithoutSpace(t *testing.T) {
	cases := []struct {
		prev, next string
		want       bool
	}{
		{"“‘Oh, at his new office", "s. He did tell me the addres", true},
		{"s. Yes, 17 King Edward Street, near S", "t. Paul’", true},
		{"t. Paul’", "s.’", true},
		{"He said.", "Then he left.", false},
		{"He said", "Then he left.", false}, // uppercase start
		{"It was 1888", "and later.", true},
		{"Paul’", "she said", false}, // s followed by a letter
		{"", "x", false},
	}
	for _, c := range cases {
		if got := joinWithoutSpace(c.prev, c.next); got != c.want {
			t.Errorf("joinWithoutSpace(%q, %q) = %v", c.prev, c.next, got)
		}
	}
}

func TestAnnotateAndSkipped(t *testing.T) {
	b := fixtureBook(t, false)
	arc, stats, err := Convert(b, Options{
		Gates: func(lang string) []string { return []string{v2.GateLangcheck} },
		Annotate: func(s *tbook.Sentence, lang string, tr *v2.Translation, hasLinks bool) {
			if lang == "tr" && s.Src == "A SCANDAL IN BOHEMIA" {
				tr.S, tr.Lang = v2.StatusWrongLang, "ru"
				tr.V = &v2.Verdict{OK: false, By: v2.GateLangcheck, Why: "foreign-script"}
			} else if !hasLinks && tr.T != "" {
				tr.S = v2.StatusRaw
			}
		},
		Skipped: func(s *tbook.Sentence, lang string) bool { return lang == "es" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Unaligned != 0 {
		t.Error("Annotate replaces the unaligned rule")
	}
	s8 := arc.Overlays[0]["tr"].Paragraphs[7].S[0]
	if s8.S != v2.StatusWrongLang || s8.Lang != "ru" || s8.V == nil || s8.V.By != v2.GateLangcheck {
		t.Errorf("S8 tr: %+v", s8)
	}
	if got := arc.Overlays[0]["es"].Paragraphs[1].S[0]; got == nil || got.S != v2.StatusSkipped || got.T != "" {
		t.Errorf("skipped es cell: %+v", got)
	}
	if rep, _ := v2.Validate(writeArchive(t, arc, "annotated.tbook")); !rep.OK() {
		t.Fatalf("annotated file invalid: %v", rep.Violations)
	}
}

func TestDigitPassthroughSkipsEscapedToken(t *testing.T) {
	s := &tbook.Sentence{
		Src:   "design 10",
		Words: [][2]int{{0, 6}},
		Tr: map[string]tbook.Translation{"ru": {
			Text:  "проектирования».10",
			Align: []tbook.AlignChunk{{T: [2]int{0, 18}, W: []int{0}}},
		}},
	}
	b := &tbook.Book{Title: "T", Source: "en", Targets: []string{"ru"},
		Chapters: []tbook.Chapter{{Title: "C", Paragraphs: [][]*tbook.Sentence{{s}}}}}
	arc, stats, err := Convert(b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.DigitPassthrough != 0 {
		t.Errorf("digit inside an escape was linked: %d", stats.DigitPassthrough)
	}
	if rep, _ := v2.Validate(writeArchive(t, arc, "escaped-digit.tbook")); !rep.OK() {
		t.Fatalf("invalid: %v", rep.Violations)
	}
}
