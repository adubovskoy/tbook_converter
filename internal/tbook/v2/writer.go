package v2

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sort"
	"strings"
)

// DeflateLevel is the compression level of DEFLATE entries. A .tbook is
// written once and read many times, so the slowest level is the default.
var DeflateLevel = flate.BestCompression

// Archive is a version-2 book in memory: the manifest, one skeleton per spine
// entry, the overlays of each chapter keyed by language, footnotes, and the
// binary entries. Entries() serializes it; Write stores it as a ZIP.
type Archive struct {
	Manifest      Manifest
	Skeletons     []*Chapter            // parallel to Manifest.Spine
	Overlays      []map[string]*Overlay // per spine index: lang → overlay
	Notes         Notes                 // nil = no footnotes
	NotesOverlays map[string]*NotesOverlay
	Cover         []byte            // stored at *Manifest.Cover ("cover.jpg" when unset)
	Images        map[string][]byte // entry name → bytes, STORED
	Extra         map[string][]byte // producer-defined entries, DEFLATE
}

// RawEntry is one archive entry about to be written.
type RawEntry struct {
	Name  string
	Data  []byte
	Store bool // STORED instead of DEFLATE
}

// Entries serializes the archive: mimetype first, manifest.json last, the
// schema shipped, digests complete, every overlay's alignDigest computed from
// the in-memory links and verified against the serialized bytes (spec §11.2).
// Entry names missing from the manifest get the conventional chN names.
func (a *Archive) Entries() ([]RawEntry, error) {
	m := &a.Manifest
	m.FormatVersion = FormatVersion
	m.Requires = Requires()
	m.Text = DefaultTextDecl()
	m.Schema = SchemaEntry
	if m.Authors == nil {
		m.Authors = []string{}
	}
	if m.Languages.Targets == nil {
		m.Languages.Targets = []string{}
	}
	if len(a.Skeletons) != len(m.Spine) {
		return nil, fmt.Errorf("%d skeletons for %d spine entries", len(a.Skeletons), len(m.Spine))
	}

	var body []RawEntry
	add := func(name string, data []byte, store bool) {
		body = append(body, RawEntry{Name: name, Data: data, Store: store})
	}
	add(SchemaEntry, SchemaJSON, false)

	for i, skel := range a.Skeletons {
		ref := &m.Spine[i]
		if ref.Text == "" {
			ref.Text = fmt.Sprintf("text/ch%d.json", i+1)
		}
		if ref.Gloss == nil {
			ref.Gloss = map[string]string{}
		}
		if skel.ID == "" {
			skel.ID = ref.ID
		}
		if skel.ID != ref.ID {
			return nil, fmt.Errorf("spine[%d]: skeleton id %q ≠ %q", i, skel.ID, ref.ID)
		}
		normalizeBlocks(skel.Paragraphs)
		if skel.Paragraphs == nil {
			skel.Paragraphs = []Block{}
		}
		bts, err := marshal(skel)
		if err != nil {
			return nil, err
		}
		add(ref.Text, bts, false)

		var langs []string
		if i < len(a.Overlays) {
			for lang := range a.Overlays[i] {
				langs = append(langs, lang)
			}
		}
		sort.Strings(langs)
		for _, lang := range langs {
			ov := a.Overlays[i][lang]
			name := ref.Gloss[lang]
			if name == "" {
				name = fmt.Sprintf("gloss/ch%d.%s.json", i+1, lang)
				ref.Gloss[lang] = name
			}
			ov.Lang, ov.Chapter = lang, skel.ID
			if ov.Gates == nil {
				ov.Gates = []string{}
			}
			if ov.Paragraphs == nil {
				ov.Paragraphs = []GlossParagraph{}
			}
			bts, err := sealOverlay(ov, skel)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			add(name, bts, false)
		}
	}

	if len(a.Notes) > 0 {
		for id, n := range a.Notes {
			normalizeBlocks(n.Paragraphs)
			if n.Paragraphs == nil {
				n.Paragraphs = []Block{}
				a.Notes[id] = n
			}
		}
		if m.Notes.Text == "" {
			m.Notes.Text = "text/notes.json"
		}
		if m.Notes.Gloss == nil {
			m.Notes.Gloss = map[string]string{}
		}
		bts, err := marshal(a.Notes)
		if err != nil {
			return nil, err
		}
		add(m.Notes.Text, bts, false)
		var langs []string
		for lang := range a.NotesOverlays {
			langs = append(langs, lang)
		}
		sort.Strings(langs)
		for _, lang := range langs {
			ov := a.NotesOverlays[lang]
			name := m.Notes.Gloss[lang]
			if name == "" {
				name = fmt.Sprintf("gloss/notes.%s.json", lang)
				m.Notes.Gloss[lang] = name
			}
			ov.Lang = lang
			if ov.Gates == nil {
				ov.Gates = []string{}
			}
			if ov.Notes == nil {
				ov.Notes = map[string][]GlossParagraph{}
			}
			bts, err := sealNotesOverlay(ov, a.Notes)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			add(name, bts, false)
		}
	} else {
		m.Notes = NotesRef{}
	}

	if a.Cover != nil {
		if m.Cover == nil || *m.Cover == "" {
			c := "cover.jpg"
			m.Cover = &c
		}
		add(*m.Cover, a.Cover, true)
	} else {
		m.Cover = nil
	}
	for _, name := range sortedKeys(a.Images) {
		add(name, a.Images[name], true)
	}
	for _, name := range sortedKeys(a.Extra) {
		add(name, a.Extra[name], false)
	}

	m.Digests = make(map[string]string, len(body))
	for _, e := range body {
		if _, dup := m.Digests[e.Name]; dup {
			return nil, fmt.Errorf("duplicate entry name %q", e.Name)
		}
		m.Digests[e.Name] = EntryDigest(e.Data)
	}
	manBytes, err := marshal(m)
	if err != nil {
		return nil, err
	}
	out := make([]RawEntry, 0, len(body)+2)
	out = append(out, RawEntry{Name: MimetypeEntry, Data: []byte(Mimetype), Store: true})
	out = append(out, body...)
	out = append(out, RawEntry{Name: ManifestEntry, Data: manBytes})
	return out, nil
}

// sealOverlay computes the overlay's alignDigest from the in-memory links (or
// verifies a caller-supplied one), serializes it, and re-derives the digest
// from the serialized bytes: an index drift between links and tokens would
// show up as a mismatch here, before anything is written.
func sealOverlay(ov *Overlay, skel *Chapter) ([]byte, error) {
	want := AlignDigest(ov, skel)
	if ov.AlignDigest == "" {
		ov.AlignDigest = want
	} else if ov.AlignDigest != want {
		return nil, fmt.Errorf("alignDigest %s does not match the links (%s)", ov.AlignDigest, want)
	}
	bts, err := marshal(ov)
	if err != nil {
		return nil, err
	}
	var back Overlay
	if err := unmarshal(bts, &back); err != nil {
		return nil, err
	}
	if got := AlignDigest(&back, skel); got != want {
		return nil, fmt.Errorf("serialized overlay digests to %s, links to %s", got, want)
	}
	return bts, nil
}

func sealNotesOverlay(ov *NotesOverlay, notes Notes) ([]byte, error) {
	want := NotesAlignDigest(ov, notes)
	if ov.AlignDigest == "" {
		ov.AlignDigest = want
	} else if ov.AlignDigest != want {
		return nil, fmt.Errorf("alignDigest %s does not match the links (%s)", ov.AlignDigest, want)
	}
	bts, err := marshal(ov)
	if err != nil {
		return nil, err
	}
	var back NotesOverlay
	if err := unmarshal(bts, &back); err != nil {
		return nil, err
	}
	if got := NotesAlignDigest(&back, notes); got != want {
		return nil, fmt.Errorf("serialized overlay digests to %s, links to %s", got, want)
	}
	return bts, nil
}

// normalizeBlocks makes the required arrays serialize as [] rather than null.
func normalizeBlocks(blocks []Block) {
	for i := range blocks {
		if blocks[i].Sents == nil {
			blocks[i].Sents = [][2]int{}
		}
		if blocks[i].Table != nil {
			for _, row := range blocks[i].Table.Rows {
				normalizeBlocks(row)
			}
		}
	}
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Write serializes the archive to path.
func (a *Archive) Write(path string) error {
	entries, err := a.Entries()
	if err != nil {
		return err
	}
	return WriteRaw(path, entries)
}

// WriteRaw writes entries in the given order as a version-2 container: the
// first entry must be mimetype and is STORED with no extra field so its bytes
// start at offset 38; JSON entries are DEFLATEd unless Store is set. The
// caller is responsible for putting manifest.json last and for its digests.
func WriteRaw(path string, entries []RawEntry) error {
	var buf bytes.Buffer
	if err := writeZip(&buf, entries); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func writeZip(w *bytes.Buffer, entries []RawEntry) error {
	if len(entries) == 0 || entries[0].Name != MimetypeEntry {
		return fmt.Errorf("first entry must be %s", MimetypeEntry)
	}
	zw := zip.NewWriter(w)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, DeflateLevel)
	})
	for i, e := range entries {
		fh := &zip.FileHeader{Name: e.Name, Method: zip.Deflate}
		if e.Store || i == 0 {
			fh.Method = zip.Store
		}
		if i == 0 {
			// CreateRaw writes sizes and CRC in the local header itself: no
			// data descriptor, no extra field, the string at offset 38.
			fh.CRC32 = crc32.ChecksumIEEE(e.Data)
			fh.CompressedSize64 = uint64(len(e.Data))
			fh.UncompressedSize64 = uint64(len(e.Data))
			fw, err := zw.CreateRaw(fh)
			if err != nil {
				return err
			}
			if _, err := fw.Write(e.Data); err != nil {
				return err
			}
			continue
		}
		fw, err := zw.CreateHeader(fh)
		if err != nil {
			return err
		}
		if _, err := fw.Write(e.Data); err != nil {
			return err
		}
	}
	return zw.Close()
}

// RecomputeDigests returns entries with manifest.json's digests rebuilt from
// the other entries' bytes, everything else in the manifest untouched. It lets
// a tool that edited one entry re-seal the container.
func RecomputeDigests(entries []RawEntry) ([]RawEntry, error) {
	out := make([]RawEntry, len(entries))
	copy(out, entries)
	mi := -1
	digests := map[string]string{}
	for i, e := range out {
		switch e.Name {
		case ManifestEntry:
			mi = i
		case MimetypeEntry:
		default:
			digests[e.Name] = EntryDigest(e.Data)
		}
	}
	if mi < 0 {
		return nil, fmt.Errorf("no %s entry", ManifestEntry)
	}
	var man map[string]json.RawMessage
	if err := json.Unmarshal(out[mi].Data, &man); err != nil {
		return nil, err
	}
	d, err := json.Marshal(digests)
	if err != nil {
		return nil, err
	}
	man["digests"] = d
	// Keep the original key order where possible: re-serialize by walking the
	// original object's keys, then any new ones.
	var keys []string
	dec := json.NewDecoder(bytes.NewReader(out[mi].Data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("manifest is not an object")
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		sb.Write(kb)
		sb.WriteByte(':')
		sb.Write(man[k])
	}
	sb.WriteByte('}')
	out[mi] = RawEntry{Name: ManifestEntry, Data: []byte(sb.String())}
	return out, nil
}
