package tbook

import (
	"archive/zip"
	"fmt"
	"os"

	"github.com/dimando/reader/converter/internal/jsonx"
)

// chapterPayload is the on-disk shape of chapters/chN.json. ParagraphStyles is a
// parallel array of per-paragraph roles; omitted entirely when every paragraph
// is ordinary body text.
type chapterPayload struct {
	Paragraphs      [][]*Sentence `json:"paragraphs"`
	ParagraphStyles []string      `json:"paragraphStyles,omitempty"`
}

// Write assembles the .tbook ZIP: manifest.json + cover.jpg + chapters/chN.json.
// Chapters are numbered ch1..chN (1-based). cover may be nil.
func Write(outPath, title, author, source string, targets []string, cover []byte, chapters []Chapter) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	man := Manifest{
		FormatVersion: 1,
		Title:         title,
		Author:        author,
		SourceLang:    source,
		TargetLangs:   targets,
		Chapters:      make([]ChapterRef, 0, len(chapters)),
	}
	if cover != nil {
		c := "cover.jpg"
		man.Cover = &c
	}

	for i, ch := range chapters {
		cid := fmt.Sprintf("ch%d", i+1)
		fname := "chapters/" + cid + ".json"
		title := ch.Title
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		man.Chapters = append(man.Chapters, ChapterRef{ID: cid, Title: title, File: fname})

		payload := chapterPayload{
			Paragraphs:      normalizeParas(ch.Paragraphs),
			ParagraphStyles: trimStyles(ch.ParagraphStyles, len(ch.Paragraphs)),
		}
		b, err := jsonx.Marshal(payload)
		if err != nil {
			return err
		}
		if err := writeEntry(zw, fname, b); err != nil {
			return err
		}
	}

	if cover != nil {
		if err := writeEntry(zw, "cover.jpg", cover); err != nil {
			return err
		}
	}
	b, err := jsonx.MarshalIndent(man, "  ")
	if err != nil {
		return err
	}
	if err := writeEntry(zw, "manifest.json", b); err != nil {
		return err
	}
	return zw.Close()
}

// normalizeParas ensures every sentence serializes with non-null words/align
// (the spec requires "align": [] and a words array, never null).
func normalizeParas(paras [][]*Sentence) [][]*Sentence {
	for _, para := range paras {
		for _, s := range para {
			if s.Words == nil {
				s.Words = [][2]int{}
			}
			for code, tr := range s.Tr {
				if tr.Align == nil {
					tr.Align = []AlignChunk{}
					s.Tr[code] = tr
				}
			}
		}
	}
	return paras
}

// trimStyles returns the per-paragraph role array padded/clamped to nParas and
// with trailing body entries dropped. It returns nil when every role is body, so
// chapterPayload omits the field entirely for a chapter with no special
// formatting. A non-body role anywhere keeps the array dense up to (and
// including) the last non-body entry, preserving index alignment with
// paragraphs[].
func trimStyles(styles []string, nParas int) []string {
	last := -1
	out := make([]string, nParas)
	for i := range out {
		role := RoleBody
		if i < len(styles) && styles[i] != "" {
			role = styles[i]
		}
		out[i] = role
		if role != RoleBody {
			last = i
		}
	}
	if last < 0 {
		return nil
	}
	return out[:last+1]
}

func writeEntry(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
