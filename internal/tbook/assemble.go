package tbook

import (
	"archive/zip"
	"fmt"
	"math"
	"os"
	"sort"
	"unicode"

	"github.com/dimando/reader/converter/internal/jsonx"
)

// chapterPayload is the on-disk shape of chapters/chN.json. ParagraphStyles is a
// parallel array of per-paragraph roles; omitted entirely when every paragraph
// is ordinary body text. Figures/Tables attach images and tables to paragraphs
// by index; omitted when the chapter has none.
type chapterPayload struct {
	Paragraphs      [][]*Sentence `json:"paragraphs"`
	ParagraphStyles []string      `json:"paragraphStyles,omitempty"`
	Figures         []Figure      `json:"figures,omitempty"`
	Tables          []Table       `json:"tables,omitempty"`
}

// Write assembles the .tbook ZIP: manifest.json + cover.jpg + chapters/chN.json
// (+ notes.json and images/* when present). Chapters are numbered ch1..chN.
func Write(outPath string, b *Book) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	man := Manifest{
		FormatVersion: 1,
		Title:         b.Title,
		Author:        b.Author,
		SourceLang:    b.Source,
		TargetLangs:   b.Targets,
		Chapters:      make([]ChapterRef, 0, len(b.Chapters)),
	}
	if b.Cover != nil {
		c := "cover.jpg"
		man.Cover = &c
	}
	if len(b.Notes) > 0 {
		n := "notes.json"
		man.Notes = &n
	}

	for i, ch := range b.Chapters {
		cid := fmt.Sprintf("ch%d", i+1)
		fname := "chapters/" + cid + ".json"
		title := ch.Title
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		man.Chapters = append(man.Chapters, ChapterRef{ID: cid, Title: title, File: fname})

		for _, t := range ch.Tables {
			for _, row := range t.Rows {
				for _, cell := range row {
					normalizeSentences(cell.Sentences)
				}
			}
		}
		payload := chapterPayload{
			Paragraphs:      normalizeParas(ch.Paragraphs),
			ParagraphStyles: trimStyles(ch.ParagraphStyles, len(ch.Paragraphs)),
			Figures:         ch.Figures,
			Tables:          ch.Tables,
		}
		bts, err := jsonx.Marshal(payload)
		if err != nil {
			return err
		}
		if err := writeEntry(zw, fname, bts); err != nil {
			return err
		}
	}

	if len(b.Notes) > 0 {
		for _, n := range b.Notes {
			_ = normalizeParas(n.Paragraphs)
		}
		bts, err := jsonx.Marshal(b.Notes)
		if err != nil {
			return err
		}
		if err := writeEntry(zw, "notes.json", bts); err != nil {
			return err
		}
	}
	// Deterministic image order.
	names := make([]string, 0, len(b.Images))
	for name := range b.Images {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := writeEntry(zw, name, b.Images[name]); err != nil {
			return err
		}
	}

	if b.Cover != nil {
		if err := writeEntry(zw, "cover.jpg", b.Cover); err != nil {
			return err
		}
	}
	bts, err := jsonx.MarshalIndent(man, "  ")
	if err != nil {
		return err
	}
	if err := writeEntry(zw, "manifest.json", bts); err != nil {
		return err
	}
	return zw.Close()
}

// normalizeParas ensures every sentence serializes with non-null words/align
// (the spec requires "align": [] and a words array, never null) and stamps each
// non-empty translation's coverage score q.
func normalizeParas(paras [][]*Sentence) [][]*Sentence {
	for _, para := range paras {
		normalizeSentences(para)
	}
	return paras
}

func normalizeSentences(sents []*Sentence) {
	for _, s := range sents {
		if s.Words == nil {
			s.Words = [][2]int{}
		}
		for code, tr := range s.Tr {
			if tr.Align == nil {
				tr.Align = []AlignChunk{}
			}
			tr.Q = coverageQ(tr)
			s.Tr[code] = tr
		}
	}
}

// coverageQ is the per-sentence alignment-coverage score: the fraction of
// rendered translation words (chunks containing a letter/digit) whose chunk
// highlights ≥1 source word. 0 when nothing is rendered or aligned.
func coverageQ(tr Translation) float64 {
	if tr.Text == "" || len(tr.Align) == 0 {
		return 0
	}
	runes := []rune(tr.Text)
	words, aligned := 0, 0
	for _, c := range tr.Align {
		a, b := clampRange(c.T[0], c.T[1], len(runes))
		if !hasLetterOrDigit(runes[a:b]) {
			continue
		}
		words++
		if len(c.W) > 0 {
			aligned++
		}
	}
	if words == 0 {
		return 0
	}
	return math.Round(100*float64(aligned)/float64(words)) / 100
}

func clampRange(a, b, n int) (int, int) {
	if a < 0 {
		a = 0
	}
	if b > n {
		b = n
	}
	if a > b {
		a = b
	}
	return a, b
}

func hasLetterOrDigit(rs []rune) bool {
	for _, r := range rs {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
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
