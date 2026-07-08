package tbook

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Read loads an existing .tbook archive back into a *Book, the inverse of
// Write. It reconstructs the chapters (with sentence pointers), book-level
// notes, image bytes, and cover, so a producer can add a target language to an
// already-assembled book without the source EPUB: read → translate the new
// target over the sentence pointers → Write.
//
// The manifest's sourceLang and targetLangs are carried on Book.Source and
// Book.Targets; the caller decides which targets to (re)translate and what the
// final Targets list should be.
func Read(path string) (*Book, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[f.Name] = f
	}
	readBytes := func(name string) ([]byte, error) {
		f, ok := files[name]
		if !ok {
			return nil, fmt.Errorf("entry %q not found", name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	readJSON := func(name string, v any) error {
		b, err := readBytes(name)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, v)
	}

	var man Manifest
	if err := readJSON("manifest.json", &man); err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	b := &Book{
		Title:   man.Title,
		Author:  man.Author,
		Source:  man.SourceLang,
		Targets: man.TargetLangs,
	}

	if man.Cover != nil {
		if b.Cover, err = readBytes(*man.Cover); err != nil {
			return nil, fmt.Errorf("read cover: %w", err)
		}
	}
	if man.Notes != nil {
		if err := readJSON(*man.Notes, &b.Notes); err != nil {
			return nil, fmt.Errorf("read notes: %w", err)
		}
	}

	// Copy any images/* entries verbatim so Write re-emits them unchanged.
	for name := range files {
		if strings.HasPrefix(name, "images/") {
			bts, err := readBytes(name)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", name, err)
			}
			if b.Images == nil {
				b.Images = map[string][]byte{}
			}
			b.Images[name] = bts
		}
	}

	for _, cr := range man.Chapters {
		var p chapterPayload
		if err := readJSON(cr.File, &p); err != nil {
			return nil, fmt.Errorf("read chapter %s: %w", cr.File, err)
		}
		b.Chapters = append(b.Chapters, Chapter{
			Title:           cr.Title,
			Paragraphs:      p.Paragraphs,
			ParagraphStyles: p.ParagraphStyles,
			Figures:         p.Figures,
			Tables:          p.Tables,
		})
	}
	return b, nil
}

// Sentences returns pointers to every sentence in the book, grouped exactly as
// segment.BuildSentenceObjects + BuildNotes group them for a fresh conversion:
//
//   - prose      — chapter body + figure captions + table-cell sentences
//   - noteSents  — content-note (kind != citation) body sentences
//   - citeSents  — citation-note body sentences
//
// The returned pointers share storage with the Book, so translating into a
// sentence's Tr map mutates the Book in place. Callers reproduce the producer's
// translatable/all sets from these three groups (citations join unless skipped).
func (b *Book) Sentences() (prose, noteSents, citeSents []*Sentence) {
	for _, ch := range b.Chapters {
		for _, para := range ch.Paragraphs {
			prose = append(prose, para...)
		}
		for _, t := range ch.Tables {
			for _, row := range t.Rows {
				for _, cell := range row {
					prose = append(prose, cell.Sentences...)
				}
			}
		}
	}
	for _, n := range b.Notes {
		for _, para := range n.Paragraphs {
			if n.Kind == NoteKindCitation {
				citeSents = append(citeSents, para...)
			} else {
				noteSents = append(noteSents, para...)
			}
		}
	}
	return prose, noteSents, citeSents
}
