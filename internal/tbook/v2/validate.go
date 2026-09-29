package v2

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Violation is one failed conformance rule (spec §11.1). Rule is a stable id
// such as "overlay.alignDigest"; Entry names the archive entry concerned.
type Violation struct {
	Rule  string
	Entry string
	Msg   string
}

func (v Violation) String() string {
	if v.Entry == "" {
		return v.Rule + ": " + v.Msg
	}
	return v.Rule + " " + v.Entry + ": " + v.Msg
}

// Report is the outcome of Validate: MUST-level violations and SHOULD-level
// warnings.
type Report struct {
	Violations []Violation
	Warnings   []Violation
}

// OK reports whether the file is conformant (warnings allowed).
func (r *Report) OK() bool { return len(r.Violations) == 0 }

// Rules returns the distinct violation rule ids, sorted.
func (r *Report) Rules() []string {
	set := map[string]bool{}
	for _, v := range r.Violations {
		set[v.Rule] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (r *Report) fail(rule, entry, format string, args ...any) {
	r.Violations = append(r.Violations, Violation{rule, entry, fmt.Sprintf(format, args...)})
}

func (r *Report) warn(rule, entry, format string, args ...any) {
	r.Warnings = append(r.Warnings, Violation{rule, entry, fmt.Sprintf(format, args...)})
}

// Validate checks a file against every rule of spec §11.1: container,
// manifest, shipped schema and entry shapes, skeleton and overlay invariants,
// digests and alignDigest. An error means the file could not be read at all.
func Validate(path string) (*Report, error) {
	rep := &Report{}
	if err := checkHeader(path, rep); err != nil {
		return nil, err
	}
	f, err := openRaw(path)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			rep.fail("manifest.missing", ManifestEntry, "%v", err)
			return rep, nil
		}
		return nil, err
	}
	defer f.Close()
	v := &validator{f: f, rep: rep}
	v.container()
	if !v.manifest() {
		return rep, nil
	}
	v.entries()
	v.chapters()
	v.notes()
	return rep, nil
}

// checkHeader sniffs the leading bytes: mimetype first, STORED, no extra
// field, the 25-byte string at offset 38 (spec §2.2).
func checkHeader(path string, rep *Report) error {
	fh, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	buf := make([]byte, 38+len(Mimetype))
	n, _ := fh.Read(buf)
	buf = buf[:n]
	if len(buf) < 30 || binary.LittleEndian.Uint32(buf) != 0x04034b50 {
		rep.fail("container.mimetype", MimetypeEntry, "file does not start with a local file header")
		return nil
	}
	method := binary.LittleEndian.Uint16(buf[8:])
	nameLen := binary.LittleEndian.Uint16(buf[26:])
	extraLen := binary.LittleEndian.Uint16(buf[28:])
	switch {
	case nameLen != 8 || len(buf) < 38 || string(buf[30:38]) != MimetypeEntry:
		rep.fail("container.mimetype", MimetypeEntry, "first entry is not mimetype")
	case method != 0:
		rep.fail("container.mimetype", MimetypeEntry, "mimetype must be STORED (method %d)", method)
	case extraLen != 0:
		rep.fail("container.mimetype", MimetypeEntry, "mimetype local header carries an extra field (%d bytes)", extraLen)
	case string(buf[38:]) != Mimetype:
		rep.fail("container.mimetype", MimetypeEntry, "bytes at offset 38 are not %q", Mimetype)
	}
	return nil
}

type validator struct {
	f   *File
	rep *Report
	man Manifest
	// jsonEntries are the entries validated for encoding/shape, by def name.
	skeletons []*Chapter
	notesText Notes
}

func (v *validator) container() {
	rep, f := v.rep, v.f
	if f.insecure {
		rep.fail("container.names", "", "an entry name is absolute or contains a .. segment")
	}
	seen := map[string]int{}
	for _, zf := range f.zr.File {
		seen[zf.Name]++
		if zf.Method != zip.Store && zf.Method != zip.Deflate {
			rep.fail("container.method", zf.Name, "compression method %d is not STORED or DEFLATE", zf.Method)
		}
		if zf.Flags&1 != 0 {
			rep.fail("container.method", zf.Name, "entry is encrypted")
		}
		if !utf8.ValidString(zf.Name) || strings.HasPrefix(zf.Name, "/") || hasDotDot(zf.Name) {
			rep.fail("container.names", zf.Name, "invalid entry name")
		}
	}
	for name, n := range seen {
		if n > 1 {
			rep.fail("container.names", name, "entry name occurs %d times", n)
		}
	}
	order := f.Entries()
	if len(order) == 0 || order[0].Name != MimetypeEntry {
		rep.fail("container.mimetype", MimetypeEntry, "mimetype is not the first local entry")
	} else if data, err := f.Entry(MimetypeEntry); err != nil || string(data) != Mimetype {
		rep.fail("container.mimetype", MimetypeEntry, "content is not exactly %q", Mimetype)
	}
	if len(order) == 0 || order[len(order)-1].Name != ManifestEntry {
		rep.fail("container.manifestLast", ManifestEntry, "manifest.json is not the last local entry")
	}
}

func hasDotDot(name string) bool {
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// manifest validates manifest.json and returns false when nothing else can be
// checked (wrong version or unusable shape).
func (v *validator) manifest() bool {
	rep, f := v.rep, v.f
	ver, err := f.formatVersion()
	if err != nil || ver != FormatVersion {
		if err != nil {
			rep.fail("manifest.formatVersion", ManifestEntry, "%v", err)
		} else {
			rep.fail("manifest.formatVersion", ManifestEntry, "formatVersion %d, want %d", ver, FormatVersion)
		}
		return false
	}
	raw, _ := f.Entry(ManifestEntry)
	v.encoding(ManifestEntry, raw)
	shape, err := decodeShape(raw)
	if err != nil {
		rep.fail("entry.shape", ManifestEntry, "%v", err)
		return false
	}
	for _, msg := range ShapeErrors("manifest", shape) {
		rep.fail("entry.shape", ManifestEntry, "%s", msg)
	}
	if err := json.Unmarshal(raw, &v.man); err != nil {
		rep.fail("entry.shape", ManifestEntry, "%v", err)
		return false
	}
	m := &v.man

	have := map[string]bool{}
	for _, id := range m.Requires {
		have[id] = true
	}
	for _, id := range Requires() {
		if !have[id] {
			rep.fail("manifest.requires", ManifestEntry, "requires lacks %q", id)
		}
	}
	for _, id := range UnknownFeatures(m.Requires) {
		rep.warn("manifest.requires", ManifestEntry, "requires %q, a feature this validator does not know", id)
	}
	if m.Text != DefaultTextDecl() {
		rep.fail("manifest.text", ManifestEntry, "text declaration %+v is not the version-2 constant", m.Text)
	}

	targets := map[string]bool{}
	for _, t := range m.Languages.Targets {
		if t == m.Languages.Source {
			rep.fail("manifest.languages", ManifestEntry, "target %q equals the source language", t)
		}
		if targets[t] {
			rep.fail("manifest.languages", ManifestEntry, "duplicate target %q", t)
		}
		targets[t] = true
	}
	if len(m.Languages.Targets) == 0 {
		rep.warn("manifest.languages", ManifestEntry, "no target languages")
	}

	if len(m.Spine) == 0 {
		rep.fail("manifest.spine", ManifestEntry, "spine is empty")
		return false
	}
	ids := map[string]bool{}
	for i, ref := range m.Spine {
		if ids[ref.ID] {
			rep.fail("manifest.spine", ManifestEntry, "spine[%d] repeats id %q", i, ref.ID)
		}
		ids[ref.ID] = true
		if !f.Has(ref.Text) {
			rep.fail("manifest.entries", ref.Text, "spine[%d].text names a missing entry", i)
		}
		for lang, name := range ref.Gloss {
			if !f.Has(name) {
				rep.fail("manifest.entries", name, "spine[%d].gloss[%s] names a missing entry", i, lang)
			}
			if !targets[lang] {
				rep.warn("manifest.languages", ManifestEntry, "spine[%d] has an overlay for %q, which is not a target", i, lang)
			}
		}
		for _, lang := range m.Languages.Targets {
			if _, ok := ref.Gloss[lang]; !ok {
				rep.warn("manifest.spine", ManifestEntry, "spine[%d] (%s) has no %s overlay", i, ref.ID, lang)
			}
		}
	}
	if m.Notes.Text != "" {
		if !f.Has(m.Notes.Text) {
			rep.fail("manifest.entries", m.Notes.Text, "notes.text names a missing entry")
		}
		for lang, name := range m.Notes.Gloss {
			if !f.Has(name) {
				rep.fail("manifest.entries", name, "notes.gloss[%s] names a missing entry", lang)
			}
			if !targets[lang] {
				rep.fail("manifest.notes", ManifestEntry, "notes.gloss key %q is not a target", lang)
			}
		}
	} else if len(m.Notes.Gloss) > 0 {
		rep.fail("manifest.notes", ManifestEntry, "notes.gloss without notes.text")
	}
	if m.Cover != nil && *m.Cover != "" && !f.Has(*m.Cover) {
		rep.fail("manifest.entries", *m.Cover, "cover names a missing entry")
	}

	if m.Schema == "" || !f.Has(m.Schema) {
		rep.fail("manifest.schema", m.Schema, "schema entry is missing")
	} else if data, _ := f.Entry(m.Schema); !bytes.Equal(data, SchemaJSON) {
		rep.fail("manifest.schema", m.Schema, "shipped schema differs from the published one")
	}

	for _, zf := range f.zr.File {
		if zf.Name == MimetypeEntry || zf.Name == ManifestEntry {
			continue
		}
		want, ok := m.Digests[zf.Name]
		if !ok {
			rep.fail("manifest.digests", zf.Name, "entry has no digest")
			continue
		}
		data, err := f.Entry(zf.Name)
		if err != nil {
			rep.fail("manifest.digests", zf.Name, "%v", err)
			continue
		}
		if got := EntryDigest(data); got != want {
			rep.fail("manifest.digests", zf.Name, "digest %s, manifest says %s", got, want)
		}
	}
	for name := range m.Digests {
		if !f.Has(name) {
			rep.fail("manifest.digests", name, "digest for a missing entry")
		}
	}
	if m.Meta != nil {
		for i, run := range m.Meta.Runs {
			if tok, ok := run.Options["tokenizer"]; ok && tok != m.Text.Tokenizer {
				rep.fail("manifest.meta", ManifestEntry, "runs[%d].options.tokenizer %v ≠ text.tokenizer", i, tok)
			}
		}
	}
	return true
}

// encoding checks UTF-8 without BOM and NFC for a JSON entry (spec §2, §7).
func (v *validator) encoding(name string, data []byte) {
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		v.rep.fail("entry.utf8", name, "byte-order mark")
	}
	if !utf8.Valid(data) {
		v.rep.fail("entry.utf8", name, "invalid UTF-8")
		return
	}
	if !norm.NFC.IsNormal(data) {
		v.rep.fail("entry.nfc", name, "entry is not NFC")
	}
}

// entry loads a JSON entry, checks encoding and shape, and decodes it into v.
func (v *validator) entry(name, def string, out any) bool {
	data, err := v.f.Entry(name)
	if err != nil {
		v.rep.fail("manifest.entries", name, "%v", err)
		return false
	}
	v.encoding(name, data)
	shape, err := decodeShape(data)
	if err != nil {
		v.rep.fail("entry.shape", name, "%v", err)
		return false
	}
	msgs := ShapeErrors(def, shape)
	for _, msg := range msgs {
		v.rep.fail("entry.shape", name, "%s", msg)
	}
	if len(msgs) > 0 {
		return false
	}
	if err := json.Unmarshal(data, out); err != nil {
		v.rep.fail("entry.shape", name, "%v", err)
		return false
	}
	return true
}

// entries validates the encoding of every JSON entry that no other check
// decodes (the schema itself).
func (v *validator) entries() {
	if v.man.Schema != "" && v.f.Has(v.man.Schema) {
		data, _ := v.f.Entry(v.man.Schema)
		v.encoding(v.man.Schema, data)
	}
	if v.man.Notes.Text != "" && v.f.Has(v.man.Notes.Text) {
		var n Notes
		if v.entry(v.man.Notes.Text, "textNotes", &n) {
			v.notesText = n
		}
	}
}

func (v *validator) chapters() {
	rep := v.rep
	spineIDs := IDSet{}
	for i, ref := range v.man.Spine {
		if !v.f.Has(ref.Text) {
			v.skeletons = append(v.skeletons, nil)
			continue
		}
		var skel Chapter
		if !v.entry(ref.Text, "textChapter", &skel) {
			v.skeletons = append(v.skeletons, nil)
			continue
		}
		v.skeletons = append(v.skeletons, &skel)
		if skel.ID != ref.ID {
			rep.fail("skeleton.id", ref.Text, "id %q ≠ spine[%d].id %q", skel.ID, i, ref.ID)
		}
		if want := spineIDs.Add(ChapterIDOf(ref.Title, &skel)); want != ref.ID {
			rep.warn("skeleton.id", ref.Text, "spine[%d].id %q ≠ recipe id %q", i, ref.ID, want)
		}
		v.blocks(ref.Text, skel.Paragraphs, true)
		for lang, name := range ref.Gloss {
			var ov Overlay
			if !v.entry(name, "glossChapter", &ov) {
				continue
			}
			if ov.Lang != lang {
				rep.fail("overlay.lang", name, "lang %q ≠ spine gloss key %q", ov.Lang, lang)
			}
			if ov.Chapter != skel.ID {
				rep.fail("overlay.binding", name, "chapter %q ≠ skeleton id %q", ov.Chapter, skel.ID)
			}
			if !v.bind(name, ov.Paragraphs, skel.Paragraphs, ov.Gates, false) {
				continue
			}
			if got := AlignDigest(&ov, &skel); got != ov.AlignDigest {
				rep.fail("overlay.alignDigest", name, "recomputed %s, file says %s", got, ov.AlignDigest)
			}
		}
	}
}

func (v *validator) notes() {
	rep := v.rep
	if v.man.Notes.Text == "" || v.notesText == nil {
		return
	}
	for id, n := range v.notesText {
		v.blocks(v.man.Notes.Text+"#"+id, n.Paragraphs, false)
	}
	for lang, name := range v.man.Notes.Gloss {
		var ov NotesOverlay
		if !v.entry(name, "glossNotes", &ov) {
			continue
		}
		if ov.Lang != lang {
			rep.fail("overlay.lang", name, "lang %q ≠ notes gloss key %q", ov.Lang, lang)
		}
		ok := true
		for id, paras := range ov.Notes {
			note, exists := v.notesText[id]
			if !exists {
				rep.fail("notes.binding", name, "note %q is not in %s", id, v.man.Notes.Text)
				ok = false
				continue
			}
			if !v.bind(name+"#"+id, paras, note.Paragraphs, ov.Gates, true) {
				ok = false
			}
		}
		if !ok {
			continue
		}
		if got := NotesAlignDigest(&ov, v.notesText); got != ov.AlignDigest {
			rep.fail("overlay.alignDigest", name, "recomputed %s, file says %s", got, ov.AlignDigest)
		}
	}
}

// blocks validates a paragraph list (spec §4); withIDs enforces the paragraph
// id recipe, which binds chapter paragraphs but is optional for cells and
// footnote paragraphs.
func (v *validator) blocks(entry string, blocks []Block, withIDs bool) {
	rep := v.rep
	ids := IDSet{}
	for p := range blocks {
		b := &blocks[p]
		at := fmt.Sprintf("paragraphs[%d]", p)
		if withIDs {
			if b.ID == "" {
				rep.fail("skeleton.paragraphId", entry, "%s has no id", at)
			} else if want := ids.Add(ParagraphID(b.Text)); want != b.ID {
				rep.fail("skeleton.paragraphId", entry, "%s id %q ≠ recipe id %q", at, b.ID, want)
			}
		}
		v.block(entry, at, b)
	}
}

func (v *validator) block(entry, at string, b *Block) {
	rep := v.rep
	n := utf8.RuneCountInString(b.Text)
	if !norm.NFC.IsNormalString(b.Text) {
		rep.fail("entry.nfc", entry, "%s text is not NFC", at)
	}
	prevEnd := -1
	for k, s := range b.Sents {
		if !(0 <= s[0] && s[0] < s[1] && s[1] <= n) {
			rep.fail("skeleton.sents", entry, "%s sents[%d] = %v out of range (text has %d code points)", at, k, s, n)
		} else if s[0] < prevEnd {
			rep.fail("skeleton.sents", entry, "%s sents[%d] overlaps or precedes the previous range", at, k)
		}
		prevEnd = s[1]
	}
	switch b.Role {
	case "", RoleBody, RoleSubtitle, RoleHeading, RoleFigure:
	case RoleSceneBreak, RoleTable:
		if b.Text != "" || len(b.Sents) != 0 {
			rep.fail("skeleton.role", entry, "%s: a %s block must have empty text and no sentences", at, b.Role)
		}
	default:
		rep.warn("skeleton.role", entry, "%s has unknown role %q (rendered as body)", at, b.Role)
	}
	if b.Figure != nil {
		if b.Role != RoleFigure {
			rep.fail("skeleton.figure", entry, "%s carries figure but has role %q", at, b.Role)
		}
		if b.Figure.Image == "" || !v.f.Has(b.Figure.Image) {
			rep.fail("skeleton.figure", entry, "%s figure.image %q is not an entry", at, b.Figure.Image)
		}
	} else if b.Role == RoleFigure {
		rep.warn("skeleton.figure", entry, "%s has role figure but no figure object", at)
	}
	if b.Table != nil {
		if b.Role != RoleTable {
			rep.fail("skeleton.table", entry, "%s carries table but has role %q", at, b.Role)
		}
		if len(b.Table.Rows) == 0 {
			rep.fail("skeleton.table", entry, "%s table has no rows", at)
		}
		for r, row := range b.Table.Rows {
			for c := range row {
				v.block(entry, fmt.Sprintf("%s.rows[%d][%d]", at, r, c), &row[c])
			}
		}
	} else if b.Role == RoleTable {
		rep.fail("skeleton.table", entry, "%s has role table but no table object", at)
	}
	if b.Words != nil {
		if len(b.Words) != len(b.Sents) {
			rep.fail("skeleton.words", entry, "%s words has %d lists for %d sentences", at, len(b.Words), len(b.Sents))
		}
		for k, ws := range b.Words {
			if k >= len(b.Sents) {
				break
			}
			v.ranges("skeleton.words", entry, at+fmt.Sprintf(".words[%d]", k), ws, b.Sents[k][0], b.Sents[k][1], b.Text)
		}
	} else {
		for _, g := range GapTokens(b) {
			rep.fail("skeleton.gap", entry, "%s token %v starts outside every sentence", at, g)
		}
	}
	for i, sp := range b.Spans {
		if !(0 <= sp.S && sp.S < sp.E && sp.E <= n) {
			rep.fail("skeleton.spans", entry, "%s spans[%d] out of range", at, i)
		}
		if sp.K != "i" && sp.K != "b" {
			rep.warn("skeleton.spans", entry, "%s spans[%d] has unknown kind %q", at, i, sp.K)
		}
	}
	for i, nr := range b.Notes {
		if nr.P < 0 || nr.P > n {
			rep.fail("skeleton.notes", entry, "%s notes[%d].p out of range", at, i)
		}
		if _, ok := v.notesText[nr.ID]; !ok {
			rep.warn("skeleton.notes", entry, "%s notes[%d] references unknown note %q", at, i, nr.ID)
		}
	}
	if len(b.Units) > 0 {
		words := SentenceWords(b)
		for i, u := range b.Units {
			if u.S < 0 || u.S >= len(b.Sents) {
				rep.fail("skeleton.units", entry, "%s units[%d].s out of range", at, i)
				continue
			}
			if len(u.W) < 2 || !ascendingUnique(u.W) {
				rep.fail("skeleton.units", entry, "%s units[%d].w must list ≥ 2 ascending indices", at, i)
			}
			for _, w := range u.W {
				if w < 0 || w >= len(words[u.S]) {
					rep.fail("skeleton.units", entry, "%s units[%d] word %d out of range", at, i, w)
				}
			}
		}
	}
}

// ranges checks an explicit word/token inventory: sorted, non-overlapping,
// a < b, inside [lo, hi), no tab or newline inside a range.
func (v *validator) ranges(rule, entry, at string, rs [][2]int, lo, hi int, text string) {
	runes := []rune(text)
	prev := lo
	for i, r := range rs {
		if !(lo <= r[0] && r[0] < r[1] && r[1] <= hi) {
			v.rep.fail(rule, entry, "%s[%d] = %v out of range [%d, %d)", at, i, r, lo, hi)
			continue
		}
		if r[0] < prev {
			v.rep.fail(rule, entry, "%s[%d] overlaps or precedes the previous range", at, i)
		}
		prev = r[1]
		for _, c := range runes[r[0]:r[1]] {
			if c == '\t' || c == '\n' {
				v.rep.fail(rule, entry, "%s[%d] contains a tab or newline", at, i)
				break
			}
		}
	}
}

// bind checks the binding rules of §6.1 and every translation of an overlay
// paragraph list against its skeleton blocks. Returns false on a binding
// failure (the overlay must be rejected; its digest is not checked). With
// positional (note paragraphs, §6.12) an id mismatch is only a warning.
func (v *validator) bind(entry string, gp []GlossParagraph, blocks []Block, gates []string, positional bool) bool {
	rep := v.rep
	if len(gp) != len(blocks) {
		rep.fail("overlay.binding", entry, "%d paragraphs, skeleton has %d", len(gp), len(blocks))
		return false
	}
	ok := true
	for p := range gp {
		at := fmt.Sprintf("paragraphs[%d]", p)
		b := &blocks[p]
		if gp[p].ID != b.ID {
			// Note paragraphs bind by position: §6.12 leaves out rule 3.
			if !positional {
				rep.fail("overlay.binding", entry, "%s id %q ≠ skeleton id %q", at, gp[p].ID, b.ID)
				ok = false
				continue
			}
			rep.warn("notes.binding", entry, "%s id %q ≠ note paragraph id %q (bound by position)", at, gp[p].ID, b.ID)
		}
		if b.Table != nil {
			if gp[p].Rows == nil || len(gp[p].Rows) != len(b.Table.Rows) {
				rep.fail("overlay.binding", entry, "%s: table row count differs", at)
				ok = false
				continue
			}
			for r, row := range gp[p].Rows {
				if len(row) != len(b.Table.Rows[r]) {
					rep.fail("overlay.binding", entry, "%s row %d: cell count differs", at, r)
					ok = false
					continue
				}
				for c := range row {
					cat := fmt.Sprintf("%s.rows[%d][%d]", at, r, c)
					if !v.sentences(entry, cat, row[c].S, &b.Table.Rows[r][c], gates) {
						ok = false
					}
				}
			}
			continue
		}
		if gp[p].Rows != nil {
			rep.fail("overlay.binding", entry, "%s has rows but the skeleton block is not a table", at)
			ok = false
			continue
		}
		if !v.sentences(entry, at, gp[p].S, b, gates) {
			ok = false
		}
	}
	return ok
}

func (v *validator) sentences(entry, at string, s []*Translation, b *Block, gates []string) bool {
	if len(s) != len(b.Sents) {
		v.rep.fail("overlay.binding", entry, "%s has %d translations for %d sentences", at, len(s), len(b.Sents))
		return false
	}
	words := SentenceWords(b)
	for k, tr := range s {
		if tr != nil {
			v.translation(entry, fmt.Sprintf("%s.s[%d]", at, k), tr, len(words[k]), gates)
		}
	}
	return true
}

var knownStatus = map[string]bool{StatusRaw: true, StatusUnaligned: true, StatusSkipped: true,
	StatusRejected: true, StatusUnverified: true, StatusWrongLang: true, StatusFree: true}

func (v *validator) translation(entry, at string, tr *Translation, wordCount int, gates []string) {
	rep := v.rep
	if tr.T == "" {
		if tr.A != nil || tr.X != nil || tr.Words != nil || tr.Groups != nil || tr.Parts != nil {
			rep.fail("overlay.translation", entry, "%s carries alignment fields without t", at)
		}
		switch tr.S {
		case StatusRaw, StatusUnaligned, StatusUnverified, StatusWrongLang, StatusFree:
			rep.fail("overlay.status", entry, "%s status %q requires t", at, tr.S)
		}
	}
	if tr.S != "" && !knownStatus[tr.S] {
		rep.warn("overlay.status", entry, "%s has unknown status %q", at, tr.S)
	}
	if tr.Lang != "" && tr.S != StatusWrongLang {
		rep.fail("overlay.status", entry, "%s has lang without s: wrongLang", at)
	}
	if tr.V != nil {
		listed := false
		for _, g := range gates {
			if g == tr.V.By {
				listed = true
				break
			}
		}
		if !listed {
			rep.fail("overlay.verdict", entry, "%s v.by %q is not in gates %v", at, tr.V.By, gates)
		}
	}
	if tr.T == "" {
		return
	}
	if !norm.NFC.IsNormalString(tr.T) {
		rep.fail("entry.nfc", entry, "%s t is not NFC", at)
	}
	n := utf8.RuneCountInString(tr.T)
	if tr.Words != nil {
		v.ranges("overlay.words", entry, at+".words", tr.Words, 0, n, tr.T)
	}
	toks := TargetTokens(tr)
	if len(tr.A) > len(toks) {
		rep.fail("overlay.links", entry, "%s has %d links for %d tokens", at, len(tr.A), len(toks))
	}
	for j, link := range tr.A {
		if link.IsNull() || link.Inserted {
			continue
		}
		if !ascendingUnique(link.Words) {
			rep.fail("overlay.links", entry, "%s a[%d] = %v is not ascending without duplicates", at, j, link.Words)
		}
		for _, i := range link.Words {
			if i < 0 || i >= wordCount {
				rep.fail("overlay.links", entry, "%s a[%d] names word %d of %d", at, j, i, wordCount)
			}
		}
	}
	runes := []rune(tr.T)
	for c, x := range tr.X {
		if len(x) < 2 {
			rep.fail("overlay.escapes", entry, "%s x[%d] is too short", at, c)
			continue
		}
		a, b := x[0], x[1]
		if !(0 <= a && a < b && b <= n) {
			rep.fail("overlay.escapes", entry, "%s x[%d] range [%d, %d) out of range", at, c, a, b)
			continue
		}
		if !hasLetterOrDigit(runes[a:b]) {
			rep.fail("overlay.escapes", entry, "%s x[%d] contains no letter or digit", at, c)
		}
		if !ascendingUnique(x[2:]) {
			rep.fail("overlay.escapes", entry, "%s x[%d] indices are not ascending without duplicates", at, c)
		}
		for _, i := range x[2:] {
			if i < 0 || i >= wordCount {
				rep.fail("overlay.escapes", entry, "%s x[%d] names word %d of %d", at, c, i, wordCount)
			}
		}
		for j, t := range toks {
			if t[0] == a && t[1] == b {
				rep.fail("overlay.escapes", entry, "%s x[%d] coincides with token %d (that is a link)", at, c, j)
			} else if t[0] < b && a < t[1] && j < len(tr.A) && tr.A[j].Linked() {
				rep.fail("overlay.escapes", entry, "%s x[%d] overlaps linked token %d", at, c, j)
			}
		}
	}
	type span struct{ a, b int }
	perToken := map[int][]span{}
	for i, p := range tr.Parts {
		if p.Token < 0 || p.Token >= len(toks) || p.Token >= len(tr.A) || !tr.A[p.Token].Linked() {
			rep.fail("overlay.parts", entry, "%s parts[%d] refines token %d, which has no link", at, i, p.Token)
			continue
		}
		tokLen := toks[p.Token][1] - toks[p.Token][0]
		if !(0 <= p.A && p.A < p.B && p.B <= tokLen) {
			rep.fail("overlay.parts", entry, "%s parts[%d] span [%d, %d) exceeds the token (%d)", at, i, p.A, p.B, tokLen)
		}
		for _, o := range perToken[p.Token] {
			if o.a < p.B && p.A < o.b {
				rep.fail("overlay.parts", entry, "%s parts[%d] overlaps another part of token %d", at, i, p.Token)
			}
		}
		perToken[p.Token] = append(perToken[p.Token], span{p.A, p.B})
		for _, w := range p.Link.Words {
			if !tr.A[p.Token].Contains(w) {
				rep.fail("overlay.parts", entry, "%s parts[%d] links word %d, not in the token's link", at, i, w)
			}
		}
	}
	for i, g := range tr.Groups {
		for _, j := range g.Tokens {
			if j < 0 || j >= len(toks) {
				rep.fail("overlay.groups", entry, "%s groups[%d] names token %d of %d", at, i, j, len(toks))
				continue
			}
			hit := false
			if j < len(tr.A) {
				for _, w := range g.Words {
					if tr.A[j].Contains(w) {
						hit = true
						break
					}
				}
			}
			if !hit {
				rep.fail("overlay.groups", entry, "%s groups[%d]: token %d links none of the listed words", at, i, j)
			}
		}
	}
}

func ascendingUnique(idx []int) bool {
	for k := 1; k < len(idx); k++ {
		if idx[k] <= idx[k-1] {
			return false
		}
	}
	return true
}

func hasLetterOrDigit(rs []rune) bool {
	for _, r := range rs {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}
