package v2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnnexCSkeletonBytes(t *testing.T) {
	skel := &Chapter{ID: "cbedb517c", Paragraphs: []Block{{ID: "04537723", Text: fixS1, Sents: [][2]int{{0, 35}}}}}
	b, err := marshal(skel)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 117 {
		t.Fatalf("skeleton is %d bytes, want 117: %s", len(b), b)
	}
	sum := sha256.Sum256(b)
	if got := hex.EncodeToString(sum[:]); got != "6d345441cb4fa35e2e1c71ccf0b4a12874c2dfd3679cbcd52cfec965dc21e141" {
		t.Fatalf("sha256 %s", got)
	}
}

// twoChapterBook builds a small valid book: chapter 1 is the Annex C
// paragraph, chapter 2 holds S9 and a scene break, in ru and de.
func twoChapterBook() *Archive {
	ch1 := &Chapter{Paragraphs: []Block{{ID: "04537723", Text: fixS1, Sents: [][2]int{{0, 35}}}}}
	ch1.ID = ChapterIDOf("One", ch1)
	ch2 := &Chapter{Paragraphs: []Block{
		{ID: ParagraphID(fixS9), Text: fixS9, Sents: [][2]int{{0, 23}, {23, 51}, {51, 88}, {88, 96}, {96, 99}}},
		{ID: ParagraphID(""), Role: RoleSceneBreak, Text: "", Sents: [][2]int{}},
	}}
	ch2.ID = ChapterIDOf("Two", ch2)
	ru1 := &Overlay{Paragraphs: []GlossParagraph{{ID: "04537723", S: []*Translation{
		{T: "«Тогда я могу подождать в соседней комнате.»", A: []Link{Single(0), Single(1), Single(2), Single(3), Single(4), Multi(5, 6), Single(7)}},
	}}}}
	de1 := &Overlay{Paragraphs: []GlossParagraph{{ID: "04537723", S: []*Translation{
		{T: "„Dann kann ich im Nebenzimmer warten.“", A: []Link{Single(0), Single(2), Single(1), Multi(4, 5), Multi(6, 7), Single(3)}},
	}}}}
	ru2 := &Overlay{Paragraphs: []GlossParagraph{
		{ID: ch2.Paragraphs[0].ID, S: []*Translation{
			{T: "«О, в его новом офисе", A: []Link{Single(0), Single(1), Single(2), Single(3), Single(4)}},
			{T: ". Он действительно сказал мне адрес", A: []Link{Single(0), Single(1), Multi(1, 2), Single(3), Multi(4, 5)}},
			{T: ". Да, Кинг-Эдвард-стрит, 17, рядом с С", A: []Link{Single(0), Multi(2, 3, 4), Single(1), Single(5), Single(5), Single(6)}},
			{T: ". Павлом»", A: []Link{Single(0)}},
			{T: ".»"},
		}},
		{ID: ch2.Paragraphs[1].ID, S: []*Translation{}},
	}}
	de2 := &Overlay{Paragraphs: []GlossParagraph{
		{ID: ch2.Paragraphs[0].ID, S: []*Translation{nil, nil, nil, nil, {S: StatusSkipped}}},
		{ID: ch2.Paragraphs[1].ID, S: []*Translation{}},
	}}
	return &Archive{
		Manifest: Manifest{Title: "Two chapters", Authors: []string{"Arthur Conan Doyle"},
			Languages: Languages{Source: "en", Targets: []string{"ru", "de"}},
			Spine:     []ChapterRef{{ID: ch1.ID, Title: "One"}, {ID: ch2.ID, Title: "Two"}}},
		Skeletons: []*Chapter{ch1, ch2},
		Overlays:  []map[string]*Overlay{{"ru": ru1, "de": de1}, {"ru": ru2, "de": de2}},
		Cover:     []byte{0xFF, 0xD8, 0xFF, 0xD9},
	}
}

func writeTemp(t *testing.T, entries []RawEntry, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := WriteRaw(path, entries); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustValidate(t *testing.T, path string) *Report {
	t.Helper()
	rep, err := Validate(path)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func TestWriteValidRoundTrip(t *testing.T) {
	arc := twoChapterBook()
	entries, err := arc.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Name != MimetypeEntry || entries[len(entries)-1].Name != ManifestEntry {
		t.Fatalf("entry order: %s … %s", entries[0].Name, entries[len(entries)-1].Name)
	}
	path := writeTemp(t, entries, "two.tbook")

	raw, _ := os.ReadFile(path)
	if string(raw[38:38+len(Mimetype)]) != Mimetype {
		t.Fatalf("mimetype not at offset 38: %q", raw[30:63])
	}
	if binary.LittleEndian.Uint16(raw[8:]) != 0 || binary.LittleEndian.Uint16(raw[28:]) != 0 {
		t.Fatal("mimetype must be STORED without an extra field")
	}
	if binary.LittleEndian.Uint16(raw[6:])&0x8 != 0 {
		t.Fatal("mimetype header must carry its sizes, not a data descriptor")
	}

	rep := mustValidate(t, path)
	if !rep.OK() {
		t.Fatalf("valid book rejected: %v", rep.Violations)
	}
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if f.Manifest.Digests[SchemaEntry] != EntryDigest(SchemaJSON) || len(f.Manifest.Digests) != 8 {
		t.Fatalf("digests: %v", f.Manifest.Digests)
	}
	if got := f.Manifest.Notes; got.Text != "" {
		t.Fatalf("notes should be {}: %+v", got)
	}
	var generic map[string]json.RawMessage
	_ = json.Unmarshal(mustRaw(f.RawManifest), &generic)
	if string(generic["notes"]) != "{}" || string(generic["cover"]) != `"cover.jpg"` {
		t.Fatalf("manifest shape: notes=%s cover=%s", generic["notes"], generic["cover"])
	}
	ov, ok, err := f.Overlay(0, "ru")
	if err != nil || !ok {
		t.Fatal(err, ok)
	}
	if ov.AlignDigest != "sha256:4bfa91a5b38848ba41ed5bf471da900413b0acdb6c68928dfaa74c0b6e832918" {
		t.Fatalf("alignDigest %s", ov.AlignDigest)
	}
	back, err := ReadArchive(path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := back.Entries()
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		if entries[i].Name != again[i].Name || string(entries[i].Data) != string(again[i].Data) {
			t.Fatalf("round trip differs at %s", entries[i].Name)
		}
	}
	// A skeleton opens with the S9 sentence 4 having no words; the de overlay
	// marks it skipped without text: legal.
	if rep := mustValidate(t, path); len(rep.Warnings) != 0 {
		t.Logf("warnings: %v", rep.Warnings)
	}
}

func TestPermutedAlignmentFailsOnDigestOnly(t *testing.T) {
	arc := twoChapterBook()
	entries, _ := arc.Entries()
	// Keep the sealed alignDigest but permute the links of S1 (Annex B.3).
	ov := arc.Overlays[0]["ru"]
	ov.Paragraphs[0].S[0].A = []Link{Single(7), Single(1), Single(2), Single(3), Single(4), Multi(5, 6), Single(0)}
	permuted, _ := marshal(ov)
	for i := range entries {
		if entries[i].Name == "gloss/ch1.ru.json" {
			entries[i].Data = permuted
		}
	}
	entries, err := RecomputeDigests(entries)
	if err != nil {
		t.Fatal(err)
	}
	rep := mustValidate(t, writeTemp(t, entries, "permuted.tbook"))
	if rules := rep.Rules(); len(rules) != 1 || rules[0] != "overlay.alignDigest" {
		t.Fatalf("want exactly overlay.alignDigest, got %v", rep.Violations)
	}
	// Re-sealing the permuted overlay makes it pass again.
	ov.AlignDigest = ""
	fixed, _ := arc.Entries()
	if rep := mustValidate(t, writeTemp(t, fixed, "resealed.tbook")); !rep.OK() {
		t.Fatalf("resealed: %v", rep.Violations)
	}
}

func TestTamperTests(t *testing.T) {
	base := func() []RawEntry {
		e, err := twoChapterBook().Entries()
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	find := func(entries []RawEntry, name string) int {
		for i := range entries {
			if entries[i].Name == name {
				return i
			}
		}
		t.Fatalf("no entry %s", name)
		return -1
	}
	has := func(rep *Report, rule string) bool {
		for _, r := range rep.Rules() {
			if r == rule {
				return true
			}
		}
		return false
	}

	t.Run("replace", func(t *testing.T) {
		e := base()
		e[find(e, "gloss/ch1.ru.json")].Data = e[find(e, "gloss/ch1.de.json")].Data
		rep := mustValidate(t, writeTemp(t, e, "replace.tbook"))
		if rep.OK() || !has(rep, "manifest.digests") || !has(rep, "overlay.lang") {
			t.Fatalf("want digests + lang violations, got %v", rep.Violations)
		}
	})
	t.Run("swap", func(t *testing.T) {
		e := base()
		i, j := find(e, "gloss/ch1.ru.json"), find(e, "gloss/ch2.ru.json")
		e[i].Data, e[j].Data = e[j].Data, e[i].Data
		rep := mustValidate(t, writeTemp(t, e, "swap.tbook"))
		if rep.OK() || !has(rep, "manifest.digests") || !has(rep, "overlay.binding") {
			t.Fatalf("want digests + binding violations, got %v", rep.Violations)
		}
		// Even with digests repaired, the chapter binding refuses it.
		e, _ = RecomputeDigests(e)
		rep = mustValidate(t, writeTemp(t, e, "swap2.tbook"))
		if rep.OK() || has(rep, "manifest.digests") || !has(rep, "overlay.binding") {
			t.Fatalf("want binding violation only, got %v", rep.Violations)
		}
	})
	t.Run("shift", func(t *testing.T) {
		arc := twoChapterBook()
		e, _ := arc.Entries()
		ov := arc.Overlays[1]["ru"]
		ov.Paragraphs = append(ov.Paragraphs[1:], ov.Paragraphs[len(ov.Paragraphs)-1])
		shifted, _ := marshal(ov)
		e[find(e, "gloss/ch2.ru.json")].Data = shifted
		e, _ = RecomputeDigests(e)
		rep := mustValidate(t, writeTemp(t, e, "shift.tbook"))
		if rep.OK() || !has(rep, "overlay.binding") {
			t.Fatalf("want binding violation, got %v", rep.Violations)
		}
	})
	t.Run("duplicate manifest", func(t *testing.T) {
		e := base()
		e = append(e, RawEntry{Name: ManifestEntry, Data: e[len(e)-1].Data})
		rep := mustValidate(t, writeTemp(t, e, "dup.tbook"))
		if rep.OK() || !has(rep, "container.names") {
			t.Fatalf("want container.names, got %v", rep.Violations)
		}
	})
	t.Run("misbinding", func(t *testing.T) {
		arc := twoChapterBook()
		e, _ := arc.Entries()
		ov := arc.Overlays[0]["ru"]
		ov.Chapter = arc.Skeletons[1].ID
		wrong, _ := marshal(ov)
		e[find(e, "gloss/ch1.ru.json")].Data = wrong
		e, _ = RecomputeDigests(e)
		rep := mustValidate(t, writeTemp(t, e, "misbind.tbook"))
		if rep.OK() || !has(rep, "overlay.binding") {
			t.Fatalf("want binding violation, got %v", rep.Violations)
		}
	})
	t.Run("manifest not last", func(t *testing.T) {
		e := base()
		n := len(e)
		e[n-1], e[n-2] = e[n-2], e[n-1]
		rep := mustValidate(t, writeTemp(t, e, "order.tbook"))
		if !has(rep, "container.manifestLast") {
			t.Fatalf("want container.manifestLast, got %v", rep.Violations)
		}
	})
}

func TestOverlayInvariants(t *testing.T) {
	check := func(t *testing.T, mutate func(*Archive), wantRule string) {
		t.Helper()
		arc := twoChapterBook()
		mutate(arc)
		e, err := arc.Entries()
		if err != nil {
			t.Fatalf("Entries: %v", err)
		}
		rep := mustValidate(t, writeTemp(t, e, "inv.tbook"))
		for _, r := range rep.Rules() {
			if r == wantRule {
				return
			}
		}
		t.Fatalf("want %s, got %v (warnings %v)", wantRule, rep.Violations, rep.Warnings)
	}
	s1 := func(a *Archive) *Translation { return a.Overlays[0]["ru"].Paragraphs[0].S[0] }
	check(t, func(a *Archive) { s1(a).A[0] = Single(8) }, "overlay.links")
	check(t, func(a *Archive) { s1(a).A[0] = Multi(2, 1) }, "overlay.links")
	check(t, func(a *Archive) { s1(a).A = append(s1(a).A, Single(0)) }, "overlay.links")
	check(t, func(a *Archive) { s1(a).X = [][]int{{1, 6, 0}} }, "overlay.escapes") // whole token
	check(t, func(a *Archive) { s1(a).X = [][]int{{1, 3, 0}} }, "overlay.escapes") // overlaps linked token
	check(t, func(a *Archive) { s1(a).X = [][]int{{0, 1, 0}} }, "overlay.escapes") // no letter
	check(t, func(a *Archive) { s1(a).V = &Verdict{OK: false, By: "judge"} }, "overlay.verdict")
	check(t, func(a *Archive) { s1(a).Lang = "uk" }, "overlay.status")
	check(t, func(a *Archive) { s1(a).Parts = []Part{{0, 0, 9, Single(0)}} }, "overlay.parts")
	check(t, func(a *Archive) { s1(a).Parts = []Part{{0, 0, 2, Single(3)}} }, "overlay.parts")
	check(t, func(a *Archive) { s1(a).Groups = []Group{{"lexeme", []int{0, 1}, []int{5}}} }, "overlay.groups")
	check(t, func(a *Archive) { a.Overlays[0]["de"].Paragraphs[0].S[0] = &Translation{A: []Link{Single(0)}} }, "overlay.translation")
	check(t, func(a *Archive) { a.Skeletons[0].Paragraphs[0].Sents = [][2]int{{0, 40}} }, "skeleton.sents")
	check(t, func(a *Archive) { a.Skeletons[0].Paragraphs[0].Sents = [][2]int{{0, 5}} }, "skeleton.gap")
	check(t, func(a *Archive) { a.Skeletons[0].Paragraphs[0].ID = "deadbeef" }, "skeleton.paragraphId")
	check(t, func(a *Archive) { a.Skeletons[1].Paragraphs[1].Text = "x" }, "skeleton.role")
	check(t, func(a *Archive) { a.Manifest.Languages.Targets = []string{"ru", "de", "en"} }, "manifest.languages")
	check(t, func(a *Archive) {
		a.Manifest.Meta = &tbookMeta{}
		a.Manifest.Meta.Runs = append(a.Manifest.Meta.Runs, tbookRun{Options: map[string]any{"tokenizer": "other"}})
	}, "manifest.meta")
	// A verdict whose gate ran is fine; so is a units entry that fits.
	arc := twoChapterBook()
	arc.Overlays[0]["ru"].Gates = []string{GateJudge}
	s1(arc).V = &Verdict{OK: false, By: GateJudge, Why: "drift"}
	arc.Skeletons[0].Paragraphs[0].Units = []Unit{{S: 0, W: []int{5, 6}, K: "fixed"}}
	e, err := arc.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if rep := mustValidate(t, writeTemp(t, e, "ok.tbook")); !rep.OK() {
		t.Fatalf("unexpected violations: %v", rep.Violations)
	}
}

func TestOpenRefusesOtherVersions(t *testing.T) {
	arc := twoChapterBook()
	e, _ := arc.Entries()
	var man map[string]json.RawMessage
	_ = json.Unmarshal(e[len(e)-1].Data, &man)
	man["formatVersion"] = json.RawMessage("1")
	e[len(e)-1].Data, _ = json.Marshal(man)
	path := writeTemp(t, e, "v1ish.tbook")
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "formatVersion 1") {
		t.Fatalf("Open should refuse formatVersion 1, got %v", err)
	}
	rep := mustValidate(t, path)
	if rep.OK() || rep.Rules()[0] != "manifest.formatVersion" {
		t.Fatalf("validator: %v", rep.Violations)
	}
	delete(man, "formatVersion")
	e[len(e)-1].Data, _ = json.Marshal(man)
	if _, err := Open(writeTemp(t, e, "nover.tbook")); err == nil || !strings.Contains(err.Error(), "formatVersion") {
		t.Fatalf("missing formatVersion must not default: %v", err)
	}
	man["formatVersion"] = json.RawMessage("2")
	man["requires"] = json.RawMessage(`["text/paragraph","align/token","tokenizer/tbook-w2","x-future"]`)
	e[len(e)-1].Data, _ = json.Marshal(man)
	if _, err := Open(writeTemp(t, e, "future.tbook")); err == nil || !strings.Contains(err.Error(), "x-future") {
		t.Fatalf("unknown requires id must be refused by name: %v", err)
	}
}

func TestShapeErrors(t *testing.T) {
	bad := `{"lang":"ru","chapter":"c1","gates":[],"alignDigest":"sha256:00","paragraphs":[{"id":"p","s":[{"t":"x","a":[1.5,"y"]}]},{"id":"q"}]}`
	v, err := decodeShape([]byte(bad))
	if err != nil {
		t.Fatal(err)
	}
	msgs := ShapeErrors("glossChapter", v)
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{"alignDigest", "a[0]", "a[1]", "paragraphs[1]"} {
		if !strings.Contains(joined, want) {
			t.Errorf("shape errors lack %q:\n%s", want, joined)
		}
	}
	good := `{"formatVersion":2,"requires":["text/paragraph"],"title":"","authors":[],"languages":{"source":"en","targets":[]},"text":{"tokenizer":"tbook-w2","offsets":"codepoint","normalization":"NFC"},"cover":null,"notes":{},"spine":[{"id":"c1","title":"t","level":0,"text":"text/ch1.json","gloss":{}}],"schema":"schema/tbook-2.schema.json","digests":{}}`
	v, _ = decodeShape([]byte(good))
	if msgs := ShapeErrors("manifest", v); len(msgs) != 0 {
		t.Fatalf("valid manifest rejected: %v", msgs)
	}
	v, _ = decodeShape([]byte(`{"text":"x","gloss":{}}`))
	if msgs := ShapeErrors("manifest", map[string]any{"notes": v}); len(msgs) == 0 {
		t.Fatal("manifest missing required fields accepted")
	}
}
