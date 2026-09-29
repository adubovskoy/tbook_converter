package v2

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/dimando/reader/converter/internal/tbook"
)

// Addition is one target language appended to an existing file (spec §2.4).
type Addition struct {
	Lang     string
	Overlays map[int]*Overlay // spine index → overlay; chapters without one stay untranslated
	Notes    *NotesOverlay    // nil when the book has no footnotes
}

// FormatVersionOf reads manifest.formatVersion without any other check, so a
// caller can pick the version-1 or version-2 reader.
func FormatVersionOf(path string) (int, error) {
	f, err := openRaw(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return f.formatVersion()
}

var chEntryRE = regexp.MustCompile(`^text/(.+)\.json$`)

// Append adds languages to the version-2 file in and writes the result to out
// (which may equal in) exactly as spec §2.4 defines it: every local entry
// before manifest.json is kept byte for byte, the new overlays follow, then a
// rewritten manifest.json and a new central directory. The manifest keeps its
// key order and unknown fields; meta is carried over and gets run appended.
// A language the file already has is refused; see Replace.
func Append(in, out string, adds []Addition, run tbook.RunRecord) error {
	return update(in, out, adds, run, false)
}

// Replace is Append that may also replace languages the file already has
// (spec §2.5). With at least one existing language the archive is rewritten:
// every entry that is not an overlay of a replaced language is copied byte for
// byte in its local order, the new overlays take the old ones' places, and the
// manifest closes the archive. A chapter an Addition has no overlay for keeps
// its existing overlay of that language verbatim (partial replacement). With
// only new languages it is an Append.
func Replace(in, out string, adds []Addition, run tbook.RunRecord) error {
	return update(in, out, adds, run, true)
}

func update(in, out string, adds []Addition, run tbook.RunRecord, allowReplace bool) error {
	f, err := Open(in)
	if err != nil {
		return err
	}
	defer f.Close()
	m := &f.Manifest
	if len(adds) == 0 {
		return fmt.Errorf("append: no languages to add")
	}

	targets := append([]string{}, m.Languages.Targets...)
	replaced := map[string]bool{}
	seenLang := map[string]bool{}
	for _, add := range adds {
		switch {
		case add.Lang == "" || add.Lang == m.Languages.Source || seenLang[add.Lang]:
			return fmt.Errorf("append: %q is empty, the source or given twice for %s", add.Lang, in)
		case slices.Contains(targets, add.Lang):
			if !allowReplace {
				return fmt.Errorf("append: %q is an existing target of %s", add.Lang, in)
			}
			replaced[add.Lang] = true
		default:
			targets = append(targets, add.Lang)
		}
		seenLang[add.Lang] = true
	}

	// The replaced languages' overlays leave the archive; each chapter's new
	// overlay takes its old one's place (spec §2.5 step 2).
	dropped := map[string]bool{}
	oldSpine := make([]map[string]string, len(m.Spine))
	for i, ref := range m.Spine {
		for lang := range replaced {
			if name := ref.Gloss[lang]; name != "" {
				dropped[name] = true
				if oldSpine[i] == nil {
					oldSpine[i] = map[string]string{}
				}
				oldSpine[i][lang] = name
			}
		}
	}
	oldNotes := map[string]string{}
	for lang := range replaced {
		if name := m.Notes.Gloss[lang]; name != "" {
			dropped[name] = true
			oldNotes[lang] = name
		}
	}

	var entries []RawEntry
	inPlace := map[string]string{} // old entry name → new entry name
	names := map[string]bool{}
	for _, zf := range f.zr.File {
		if !dropped[zf.Name] {
			names[zf.Name] = true
		}
	}
	addEntry := func(name, old string, data []byte) error {
		if names[name] {
			return fmt.Errorf("append: entry %q already exists", name)
		}
		names[name] = true
		entries = append(entries, RawEntry{Name: name, Data: data})
		if old != "" {
			inPlace[old] = name
		}
		return nil
	}
	spineGloss := make([]map[string]string, len(m.Spine))
	notesGloss := map[string]string{}
	var notes Notes
	if m.Notes.Text != "" {
		if notes, err = f.NotesText(); err != nil {
			return err
		}
	}
	for _, add := range adds {
		for i := range m.Spine {
			ov, ok := add.Overlays[i]
			if !ok || ov == nil {
				continue
			}
			skel, err := f.Skeleton(i)
			if err != nil {
				return err
			}
			ov.Lang, ov.Chapter = add.Lang, skel.ID
			if ov.Gates == nil {
				ov.Gates = []string{}
			}
			if ov.Paragraphs == nil {
				ov.Paragraphs = []GlossParagraph{}
			}
			bts, err := sealOverlay(ov, skel)
			if err != nil {
				return fmt.Errorf("append %s chapter %d: %w", add.Lang, i+1, err)
			}
			name := OverlayEntryName(m.Spine[i].Text, i, add.Lang)
			if err := addEntry(name, oldSpine[i][add.Lang], bts); err != nil {
				return err
			}
			if spineGloss[i] == nil {
				spineGloss[i] = map[string]string{}
			}
			spineGloss[i][add.Lang] = name
		}
		if add.Notes != nil && notes != nil {
			ov := add.Notes
			ov.Lang = add.Lang
			if ov.Gates == nil {
				ov.Gates = []string{}
			}
			if ov.Notes == nil {
				ov.Notes = map[string][]GlossParagraph{}
			}
			bts, err := sealNotesOverlay(ov, notes)
			if err != nil {
				return fmt.Errorf("append %s notes: %w", add.Lang, err)
			}
			name := fmt.Sprintf("gloss/notes.%s.json", add.Lang)
			if err := addEntry(name, oldNotes[add.Lang], bts); err != nil {
				return err
			}
			notesGloss[add.Lang] = name
		}
	}
	// A chapter the run does not cover keeps its old overlay (spec §2.5 step 3).
	for i, old := range oldSpine {
		for lang, name := range old {
			if _, ok := spineGloss[i][lang]; ok {
				continue
			}
			if names[name] {
				return fmt.Errorf("append: entry %q already exists", name)
			}
			names[name] = true
			delete(dropped, name)
			if spineGloss[i] == nil {
				spineGloss[i] = map[string]string{}
			}
			spineGloss[i][lang] = name
		}
	}

	man, err := updateManifest(f, targets, spineGloss, notesGloss, replaced, entries, dropped, run)
	if err != nil {
		return err
	}
	manifest := RawEntry{Name: ManifestEntry, Data: man}

	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	var res []byte
	if len(replaced) == 0 {
		res, err = appendContainer(data, append(entries, manifest))
	} else {
		res, err = rewriteContainer(data, dropped, inPlace, entries, manifest)
	}
	if err != nil {
		return fmt.Errorf("append %s: %w", in, err)
	}
	f.Close()
	tmp, err := os.CreateTemp(filepath.Dir(out), ".tbook-append-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(res); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), out)
}

// OverlayEntryName derives an overlay's entry name from its skeleton's
// (spec §2.5): text/X.json → gloss/X.<lang>.json; chN for any other name.
func OverlayEntryName(skeleton string, i int, lang string) string {
	stem := fmt.Sprintf("ch%d", i+1)
	if mm := chEntryRE.FindStringSubmatch(skeleton); mm != nil {
		stem = mm[1]
	}
	return fmt.Sprintf("gloss/%s.%s.json", stem, lang)
}

// updateManifest rewrites the manifest for an append or a replacement:
// targets, spine[].gloss, notes.gloss, digests and meta change; every other
// byte of every other field is kept, key order included. A replaced language
// keeps its key position where spineGloss names an overlay and loses the key
// where it names none; dropped entries leave digests.
func updateManifest(f *File, targets []string, spineGloss []map[string]string, notesGloss map[string]string,
	replaced map[string]bool, added []RawEntry, dropped map[string]bool, run tbook.RunRecord) ([]byte, error) {

	raw, err := f.Entry(ManifestEntry)
	if err != nil {
		return nil, err
	}
	top, err := parseObject(raw)
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}

	langs, err := parseObject(top.get("languages"))
	if err != nil {
		return nil, fmt.Errorf("manifest.languages: %w", err)
	}
	if err := langs.setJSON("targets", targets); err != nil {
		return nil, err
	}
	top.set("languages", langs.bytes())

	var spine []json.RawMessage
	if err := json.Unmarshal(top.get("spine"), &spine); err != nil {
		return nil, fmt.Errorf("manifest.spine: %w", err)
	}
	for i := range spine {
		var set map[string]string
		if i < len(spineGloss) {
			set = spineGloss[i]
		}
		if len(set) == 0 && len(replaced) == 0 {
			continue
		}
		ref, err := parseObject(spine[i])
		if err != nil {
			return nil, fmt.Errorf("manifest.spine[%d]: %w", i, err)
		}
		gloss, err := editKeys(ref.get("gloss"), set, replaced)
		if err != nil {
			return nil, fmt.Errorf("manifest.spine[%d].gloss: %w", i, err)
		}
		ref.set("gloss", gloss)
		spine[i] = ref.bytes()
	}
	if err := top.setJSON("spine", spine); err != nil {
		return nil, err
	}

	if len(notesGloss) > 0 || len(replaced) > 0 {
		n, err := parseObject(top.get("notes"))
		if err != nil {
			return nil, fmt.Errorf("manifest.notes: %w", err)
		}
		if g := n.get("gloss"); g != nil || len(notesGloss) > 0 {
			gloss, err := editKeys(g, notesGloss, replaced)
			if err != nil {
				return nil, fmt.Errorf("manifest.notes.gloss: %w", err)
			}
			n.set("gloss", gloss)
			top.set("notes", n.bytes())
		}
	}

	digests := map[string]json.RawMessage{}
	if d := top.get("digests"); d != nil {
		if err := json.Unmarshal(d, &digests); err != nil {
			return nil, fmt.Errorf("manifest.digests: %w", err)
		}
	}
	for name := range dropped {
		delete(digests, name)
	}
	for _, e := range added {
		digests[e.Name], _ = json.Marshal(EntryDigest(e.Data))
	}
	if err := top.setJSON("digests", digests); err != nil {
		return nil, err
	}

	meta := &tbook.Meta{}
	if mr := top.get("meta"); mr != nil {
		if err := json.Unmarshal(mr, meta); err != nil {
			return nil, fmt.Errorf("manifest.meta: %w", err)
		}
	}
	addRun(meta, run)
	if err := top.setJSON("meta", meta); err != nil {
		return nil, err
	}
	return top.bytes(), nil
}

// addRun is spec §3.10 for an append or a replacement: the run is always
// recorded, updatedAt may change and runs are capped; nothing else is touched.
func addRun(m *tbook.Meta, r tbook.RunRecord) {
	if r.At != "" {
		m.UpdatedAt = r.At
	}
	m.Runs = append(m.Runs, r)
	if len(m.Runs) > tbook.MetaRunsMax {
		m.Runs = m.Runs[len(m.Runs)-tbook.MetaRunsMax:]
	}
}

// editKeys sets kv in a JSON object (null or absent = {}) and deletes the del
// keys kv does not set; existing keys keep their position.
func editKeys(obj json.RawMessage, kv map[string]string, del map[string]bool) (json.RawMessage, error) {
	o := &object{}
	if len(bytes.TrimSpace(obj)) > 0 && string(bytes.TrimSpace(obj)) != "null" {
		var err error
		if o, err = parseObject(obj); err != nil {
			return nil, err
		}
	}
	for k := range del {
		if _, ok := kv[k]; !ok {
			o.del(k)
		}
	}
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if err := o.setJSON(k, kv[k]); err != nil {
			return nil, err
		}
	}
	return o.bytes(), nil
}

// object is a JSON object that keeps its key order across an edit.
type object struct {
	keys []string
	vals map[string]json.RawMessage
}

func parseObject(b []byte) (*object, error) {
	o := &object{vals: map[string]json.RawMessage{}}
	dec := json.NewDecoder(bytes.NewReader(b))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("not a JSON object")
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		k := tok.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		if _, dup := o.vals[k]; !dup {
			o.keys = append(o.keys, k)
		}
		o.vals[k] = v
	}
	return o, nil
}

func (o *object) get(k string) json.RawMessage {
	if o.vals == nil {
		return nil
	}
	return o.vals[k]
}

func (o *object) set(k string, v json.RawMessage) {
	if o.vals == nil {
		o.vals = map[string]json.RawMessage{}
	}
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *object) del(k string) {
	if _, ok := o.vals[k]; !ok {
		return
	}
	delete(o.vals, k)
	o.keys = slices.DeleteFunc(o.keys, func(x string) bool { return x == k })
}

func (o *object) setJSON(k string, v any) error {
	b, err := marshal(v)
	if err != nil {
		return err
	}
	o.set(k, b)
	return nil
}

func (o *object) bytes() []byte {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(o.vals[k])
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// ZIP record layouts (APPNOTE 4.3.12, 4.3.16).
const (
	sigCentral = 0x02014b50
	sigEOCD    = 0x06054b50
	cdFixed    = 46
	eocdFixed  = 22
)

type cdRecord struct {
	name   string
	offset uint32 // local header offset
	raw    []byte
}

// readCentral parses the end-of-central-directory record and every central
// directory record of data. ZIP64 archives are out of scope (spec §2).
func readCentral(data []byte) (cdStart int, recs []cdRecord, err error) {
	eocd := -1
	for i := len(data) - eocdFixed; i >= 0 && i >= len(data)-eocdFixed-0xFFFF; i-- {
		if binary.LittleEndian.Uint32(data[i:]) == sigEOCD {
			eocd = i
			break
		}
	}
	if eocd < 0 {
		return 0, nil, fmt.Errorf("no end-of-central-directory record")
	}
	count := int(binary.LittleEndian.Uint16(data[eocd+10:]))
	size := binary.LittleEndian.Uint32(data[eocd+12:])
	off := binary.LittleEndian.Uint32(data[eocd+16:])
	if count == 0xFFFF || size == 0xFFFFFFFF || off == 0xFFFFFFFF {
		return 0, nil, fmt.Errorf("ZIP64 archives are not supported")
	}
	if int(off)+int(size) > eocd {
		return 0, nil, fmt.Errorf("central directory out of bounds")
	}
	p := int(off)
	for n := 0; n < count; n++ {
		if p+cdFixed > eocd || binary.LittleEndian.Uint32(data[p:]) != sigCentral {
			return 0, nil, fmt.Errorf("bad central directory record %d", n)
		}
		nl := int(binary.LittleEndian.Uint16(data[p+28:]))
		el := int(binary.LittleEndian.Uint16(data[p+30:]))
		cl := int(binary.LittleEndian.Uint16(data[p+32:]))
		end := p + cdFixed + nl + el + cl
		if end > eocd {
			return 0, nil, fmt.Errorf("central directory record %d out of bounds", n)
		}
		recs = append(recs, cdRecord{
			name:   string(data[p+cdFixed : p+cdFixed+nl]),
			offset: binary.LittleEndian.Uint32(data[p+42:]),
			raw:    data[p:end],
		})
		p = end
	}
	return int(off), recs, nil
}

// manifestLast checks the precondition of spec §2.4 and §2.5 on the central
// directory: manifest.json is listed once and is the last local entry. It
// returns the manifest's local header offset; the error names the entry that
// follows the manifest.
func manifestLast(recs []cdRecord) (uint32, error) {
	var at []uint32
	for _, r := range recs {
		if r.name == ManifestEntry {
			at = append(at, r.offset)
		}
	}
	if len(at) == 0 {
		return 0, fmt.Errorf("no %s", ManifestEntry)
	}
	trunc := slices.Min(at)
	next, found := "", false
	var nextOff uint32
	for _, r := range recs {
		if r.offset > trunc && (!found || r.offset < nextOff) {
			next, nextOff, found = r.name, r.offset, true
		}
	}
	if len(at) > 1 {
		return 0, fmt.Errorf("%s is listed %d times (%s follows the first one); the input is not a "+
			"conformant .tbook (spec §2) — rewrite it into a conformant archive first", ManifestEntry, len(at), next)
	}
	if found {
		return 0, fmt.Errorf("%s is not the last local entry (%s follows it); the input is not a "+
			"conformant .tbook (spec §2) — rewrite it into a conformant archive first", ManifestEntry, next)
	}
	return trunc, nil
}

// appendContainer performs the byte-level part of §2.4 on a whole archive.
func appendContainer(data []byte, entries []RawEntry) ([]byte, error) {
	_, recs, err := readCentral(data)
	if err != nil {
		return nil, err
	}
	trunc, err := manifestLast(recs)
	if err != nil {
		return nil, err
	}
	var kept []cdRecord
	for _, r := range recs {
		if r.name != ManifestEntry {
			kept = append(kept, r)
		}
	}

	// The new local entries and their central records come from archive/zip,
	// written as if the archive began at the truncation point.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.SetOffset(int64(trunc))
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, DeflateLevel)
	})
	for _, e := range entries {
		fh := &zip.FileHeader{Name: e.Name, Method: zip.Deflate}
		if e.Store {
			fh.Method = zip.Store
		}
		w, err := zw.CreateHeader(fh)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(e.Data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	nb := buf.Bytes()
	newStart, newRecs, err := readCentral(append(make([]byte, trunc), nb...))
	if err != nil {
		return nil, fmt.Errorf("new entries: %w", err)
	}
	locals := nb[:newStart-int(trunc)]

	out := make([]byte, 0, int(trunc)+len(nb)+len(data)-int(trunc))
	out = append(out, data[:trunc]...)
	out = append(out, locals...)
	cdOff := len(out)
	for _, r := range kept {
		out = append(out, r.raw...)
	}
	for _, r := range newRecs {
		out = append(out, r.raw...)
	}
	cdSize := len(out) - cdOff
	total := len(kept) + len(newRecs)
	if total >= 0xFFFF || len(out) >= 0xFFFFFFFF {
		return nil, fmt.Errorf("archive would need ZIP64")
	}
	var eocd [eocdFixed]byte
	binary.LittleEndian.PutUint32(eocd[0:], sigEOCD)
	binary.LittleEndian.PutUint16(eocd[8:], uint16(total))
	binary.LittleEndian.PutUint16(eocd[10:], uint16(total))
	binary.LittleEndian.PutUint32(eocd[12:], uint32(cdSize))
	binary.LittleEndian.PutUint32(eocd[16:], uint32(cdOff))
	return append(out, eocd[:]...), nil
}

// encodeEntry encodes one entry as archive/zip writes it: its local header,
// data and descriptor, and its central directory record for offset 0.
func encodeEntry(e RawEntry) (local, central []byte, err error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, DeflateLevel)
	})
	fh := &zip.FileHeader{Name: e.Name, Method: zip.Deflate}
	if e.Store {
		fh.Method = zip.Store
	}
	w, err := zw.CreateHeader(fh)
	if err != nil {
		return nil, nil, err
	}
	if _, err := w.Write(e.Data); err != nil {
		return nil, nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, nil, err
	}
	b := buf.Bytes()
	start, recs, err := readCentral(b)
	if err != nil || len(recs) != 1 {
		return nil, nil, fmt.Errorf("encode %s: %v", e.Name, err)
	}
	return b[:start], recs[0].raw, nil
}

// rewriteContainer performs the byte-level part of §2.5: every local entry
// but the manifest and the dropped ones is copied verbatim (header, data and
// descriptor) in local order; a dropped entry named in inPlace is replaced by
// that new entry at its position; the other new entries follow the last copied
// one, then the manifest. Central records are copied with the new offsets.
// The archive comment is not kept.
func rewriteContainer(data []byte, dropped map[string]bool, inPlace map[string]string,
	entries []RawEntry, manifest RawEntry) ([]byte, error) {

	cdStart, recs, err := readCentral(data)
	if err != nil {
		return nil, err
	}
	if _, err := manifestLast(recs); err != nil {
		return nil, err
	}
	byName := map[string]RawEntry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	order := slices.Clone(recs)
	slices.SortStableFunc(order, func(a, b cdRecord) int { return int(a.offset) - int(b.offset) })

	var out, cd []byte
	n := 0 // central records
	written := map[string]bool{}
	writeNew := func(e RawEntry) error {
		local, rec, err := encodeEntry(e)
		if err != nil {
			return err
		}
		rec = slices.Clone(rec)
		binary.LittleEndian.PutUint32(rec[42:], uint32(len(out)))
		out = append(out, local...)
		cd = append(cd, rec...)
		written[e.Name] = true
		n++
		return nil
	}
	for k, r := range order {
		end := uint32(cdStart)
		if k+1 < len(order) {
			end = order[k+1].offset
		}
		if int(end) > cdStart || r.offset > end || binary.LittleEndian.Uint32(data[r.offset:]) != 0x04034b50 {
			return nil, fmt.Errorf("bad local entry %s", r.name)
		}
		switch {
		case r.name == ManifestEntry:
			continue
		case dropped[r.name]:
			if name, ok := inPlace[r.name]; ok {
				if err := writeNew(byName[name]); err != nil {
					return nil, err
				}
			}
			continue
		}
		rec := slices.Clone(r.raw)
		binary.LittleEndian.PutUint32(rec[42:], uint32(len(out)))
		out = append(out, data[r.offset:end]...)
		cd = append(cd, rec...)
		n++
	}
	for _, e := range entries {
		if !written[e.Name] {
			if err := writeNew(e); err != nil {
				return nil, err
			}
		}
	}
	if err := writeNew(manifest); err != nil {
		return nil, err
	}
	cdOff := len(out)
	out = append(out, cd...)
	if n >= 0xFFFF || len(out) >= 0xFFFFFFFF {
		return nil, fmt.Errorf("archive would need ZIP64")
	}
	var eocd [eocdFixed]byte
	binary.LittleEndian.PutUint32(eocd[0:], sigEOCD)
	binary.LittleEndian.PutUint16(eocd[8:], uint16(n))
	binary.LittleEndian.PutUint16(eocd[10:], uint16(n))
	binary.LittleEndian.PutUint32(eocd[12:], uint32(len(cd)))
	binary.LittleEndian.PutUint32(eocd[16:], uint32(cdOff))
	return append(out, eocd[:]...), nil
}
