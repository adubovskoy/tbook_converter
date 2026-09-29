package v2

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

// File is an opened version-2 archive. Entries are addressed by name through
// the central directory; skeletons and overlays are decoded on demand.
type File struct {
	Path     string
	Manifest Manifest
	// RawManifest is the manifest as a generic object, for shape checks and
	// for reading formatVersion before anything else (spec §10.1).
	RawManifest map[string]json.RawMessage
	zr          *zip.ReadCloser
	byName      map[string]*zip.File
	insecure    bool // the zip carried a non-local name
}

// Open opens a version-2 file and refuses anything else with a clear message:
// a missing or different formatVersion, or a requires id this implementation
// does not know (spec §11.3).
func Open(path string) (*File, error) {
	f, err := openRaw(path)
	if err != nil {
		return nil, err
	}
	if v, err := f.formatVersion(); err != nil || v != FormatVersion {
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return nil, fmt.Errorf("%s: formatVersion %d is not %d", path, v, FormatVersion)
	}
	if err := json.Unmarshal(mustRaw(f.RawManifest), &f.Manifest); err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: manifest: %w", path, err)
	}
	if unknown := UnknownFeatures(f.Manifest.Requires); len(unknown) > 0 {
		f.Close()
		return nil, fmt.Errorf("%s: requires features this implementation lacks: %v", path, unknown)
	}
	return f, nil
}

// UnknownFeatures returns the requires ids this package does not implement.
func UnknownFeatures(requires []string) []string {
	known := map[string]bool{}
	for _, id := range Requires() {
		known[id] = true
	}
	var out []string
	for _, id := range requires {
		if !known[id] {
			out = append(out, id)
		}
	}
	return out
}

// openRaw opens the ZIP and decodes the manifest generically, without any
// version or feature check — the validator's entry point.
func openRaw(path string) (*File, error) {
	zr, err := zip.OpenReader(path)
	insecure := false
	if err != nil {
		if !errors.Is(err, zip.ErrInsecurePath) || zr == nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		insecure = true
	}
	f := &File{Path: path, zr: zr, byName: map[string]*zip.File{}, insecure: insecure}
	for _, zf := range zr.File {
		if _, dup := f.byName[zf.Name]; !dup {
			f.byName[zf.Name] = zf
		}
	}
	raw, err := f.Entry(ManifestEntry)
	if err != nil {
		f.Close()
		return nil, err
	}
	if err := json.Unmarshal(raw, &f.RawManifest); err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: manifest.json: %w", path, err)
	}
	return f, nil
}

func mustRaw(m map[string]json.RawMessage) []byte {
	b, _ := json.Marshal(m)
	return b
}

// formatVersion reads manifest.formatVersion; a missing field is an error,
// never a default (spec §10.1).
func (f *File) formatVersion() (int, error) {
	raw, ok := f.RawManifest["formatVersion"]
	if !ok {
		return 0, fmt.Errorf("manifest has no formatVersion")
	}
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, fmt.Errorf("manifest formatVersion is not an integer")
	}
	return v, nil
}

// Close releases the archive.
func (f *File) Close() error {
	if f.zr == nil {
		return nil
	}
	err := f.zr.Close()
	f.zr = nil
	return err
}

// Entries returns the zip entries in local-header order.
func (f *File) Entries() []*zip.File {
	out := append([]*zip.File(nil), f.zr.File...)
	sort.SliceStable(out, func(i, j int) bool {
		oi, _ := out[i].DataOffset()
		oj, _ := out[j].DataOffset()
		return oi < oj
	})
	return out
}

// Has reports whether an entry exists.
func (f *File) Has(name string) bool { _, ok := f.byName[name]; return ok }

// Entry returns an entry's uncompressed bytes.
func (f *File) Entry(name string) ([]byte, error) {
	zf, ok := f.byName[name]
	if !ok {
		return nil, fmt.Errorf("%s: entry %q not found", f.Path, name)
	}
	rc, err := zf.Open()
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", f.Path, name, err)
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (f *File) entryJSON(name string, v any) error {
	b, err := f.Entry(name)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %s: %w", f.Path, name, err)
	}
	return nil
}

// Skeleton decodes spine[i]'s chapter skeleton.
func (f *File) Skeleton(i int) (*Chapter, error) {
	if i < 0 || i >= len(f.Manifest.Spine) {
		return nil, fmt.Errorf("spine index %d out of range", i)
	}
	var ch Chapter
	if err := f.entryJSON(f.Manifest.Spine[i].Text, &ch); err != nil {
		return nil, err
	}
	return &ch, nil
}

// Overlay decodes spine[i]'s overlay for lang; ok is false when the chapter
// has no overlay in that language.
func (f *File) Overlay(i int, lang string) (*Overlay, bool, error) {
	if i < 0 || i >= len(f.Manifest.Spine) {
		return nil, false, fmt.Errorf("spine index %d out of range", i)
	}
	name, ok := f.Manifest.Spine[i].Gloss[lang]
	if !ok {
		return nil, false, nil
	}
	var ov Overlay
	if err := f.entryJSON(name, &ov); err != nil {
		return nil, false, err
	}
	return &ov, true, nil
}

// NotesText decodes text/notes.json; nil when the book has no footnotes.
func (f *File) NotesText() (Notes, error) {
	if f.Manifest.Notes.Text == "" {
		return nil, nil
	}
	var n Notes
	if err := f.entryJSON(f.Manifest.Notes.Text, &n); err != nil {
		return nil, err
	}
	return n, nil
}

// NotesOverlay decodes gloss/notes.<lang>.json; ok is false when absent.
func (f *File) NotesOverlay(lang string) (*NotesOverlay, bool, error) {
	name, ok := f.Manifest.Notes.Gloss[lang]
	if !ok {
		return nil, false, nil
	}
	var ov NotesOverlay
	if err := f.entryJSON(name, &ov); err != nil {
		return nil, false, err
	}
	return &ov, true, nil
}

// ReadArchive loads a whole file into an Archive, the inverse of Write, so a
// tool can edit and re-seal it.
func ReadArchive(path string) (*Archive, error) {
	f, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	a := &Archive{Manifest: f.Manifest}
	referenced := map[string]bool{MimetypeEntry: true, ManifestEntry: true, f.Manifest.Schema: true}
	for i, ref := range f.Manifest.Spine {
		skel, err := f.Skeleton(i)
		if err != nil {
			return nil, err
		}
		a.Skeletons = append(a.Skeletons, skel)
		referenced[ref.Text] = true
		ovs := map[string]*Overlay{}
		for lang, name := range ref.Gloss {
			ov, _, err := f.Overlay(i, lang)
			if err != nil {
				return nil, err
			}
			ovs[lang] = ov
			referenced[name] = true
		}
		a.Overlays = append(a.Overlays, ovs)
	}
	if a.Notes, err = f.NotesText(); err != nil {
		return nil, err
	}
	if a.Notes != nil {
		referenced[f.Manifest.Notes.Text] = true
		a.NotesOverlays = map[string]*NotesOverlay{}
		for lang, name := range f.Manifest.Notes.Gloss {
			ov, _, err := f.NotesOverlay(lang)
			if err != nil {
				return nil, err
			}
			a.NotesOverlays[lang] = ov
			referenced[name] = true
		}
	}
	if f.Manifest.Cover != nil && *f.Manifest.Cover != "" {
		if a.Cover, err = f.Entry(*f.Manifest.Cover); err != nil {
			return nil, err
		}
		referenced[*f.Manifest.Cover] = true
	}
	for _, zf := range f.zr.File {
		if referenced[zf.Name] {
			continue
		}
		data, err := f.Entry(zf.Name)
		if err != nil {
			return nil, err
		}
		if zf.Method == zip.Store {
			if a.Images == nil {
				a.Images = map[string][]byte{}
			}
			a.Images[zf.Name] = data
		} else {
			if a.Extra == nil {
				a.Extra = map[string][]byte{}
			}
			a.Extra[zf.Name] = data
		}
	}
	return a, nil
}
