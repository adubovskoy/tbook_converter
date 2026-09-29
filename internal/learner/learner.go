// Package learner scores what a reader gets from one tap, offline and with no
// API calls. It holds two metrics ported from the 2026-09 format-review panel:
//
//   - the tap ledger — every (source word, language) tap of a book classified
//     into six outcome classes (cmd/tapledger);
//   - cross-target consensus — for a book with several targets, which
//     languages highlight each source-word slot (cmd/consensus).
//
// Both read .tbook version 1 and version 2 through one version-independent
// view: a sentence is a list of tappable source words plus, per language, a
// translation text and the chunks that light up when a word is tapped. For
// version 2 the chunks are derived with the package's own tap algorithm
// (v2.HighlightRanges, spec §8.2/§8.5), so the ledger scores what a version-2
// reader shows.
package learner

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
)

// Chunk is one highlightable fragment of a translation: A and B are the raw
// code-point range as the file stores it, Words the source word indices that
// light it. toks is the number of word tokens inside the clamped fragment and
// is 0 when the fragment is empty — such a chunk lights nothing.
type Chunk struct {
	A, B  int
	Words []int
	toks  int
}

// Lit reports whether tapping source word i lights this chunk.
func (c Chunk) Lit(i int) bool {
	if c.toks == 0 {
		return false
	}
	for _, w := range c.Words {
		if w == i {
			return true
		}
	}
	return false
}

// Cell is one language's rendering of one sentence. A sentence with no
// rendering in a language has no Cell at all.
type Cell struct {
	Text   string
	Chunks []Chunk
}

// Sentence is one source sentence: the surface text of every tappable word
// plus the target cells. Loc is a human-readable position for reports.
type Sentence struct {
	Loc   string
	Words []string
	Tr    map[string]*Cell
}

// Book is the version-independent view both metrics read.
type Book struct {
	Path    string
	Version int
	Targets []string
	Sents   []*Sentence
}

// Load reads a .tbook of either format version into the common view. Version
// dispatch is on manifest.formatVersion, never on the file name.
func Load(path string) (*Book, error) {
	v, err := formatVersion(path)
	if err != nil {
		return nil, err
	}
	switch v {
	case 1:
		return loadV1(path)
	case v2.FormatVersion:
		return loadV2(path)
	}
	return nil, fmt.Errorf("%s: formatVersion %d is neither 1 nor %d", path, v, v2.FormatVersion)
}

func formatVersion(path string) (int, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "manifest.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return 0, fmt.Errorf("%s: manifest.json: %w", path, err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			return 0, fmt.Errorf("%s: manifest.json: %w", path, err)
		}
		var probe struct {
			FormatVersion int `json:"formatVersion"`
		}
		if err := json.Unmarshal(b, &probe); err != nil {
			return 0, fmt.Errorf("%s: manifest.json: %w", path, err)
		}
		if probe.FormatVersion == 0 {
			return 0, fmt.Errorf("%s: manifest.json has no formatVersion", path)
		}
		return probe.FormatVersion, nil
	}
	return 0, fmt.Errorf("%s: no manifest.json (not a .tbook archive)", path)
}

// newChunk clamps a raw span against the translation's rune slice and counts
// the word tokens it covers.
func newChunk(rs []rune, a, b int, words []int) Chunk {
	ca, cb := clamp(a, 0, len(rs)), clamp(b, 0, len(rs))
	if cb < ca {
		cb = ca
	}
	n := 0
	if cb > ca {
		n = countWords(rs, ca, cb)
		if n == 0 {
			n = 1 // a fragment of pure punctuation still lights one span
		}
	}
	return Chunk{A: a, B: b, Words: words, toks: n}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---- version 1 ----

func loadV1(path string) (*Book, error) {
	b, err := tbook.Read(path)
	if err != nil {
		return nil, err
	}
	return FromV1Book(path, b), nil
}

// FromV1Book builds the common view from an already-decoded version-1 book,
// for a caller that has one in hand (and to save re-reading the archive).
func FromV1Book(path string, b *tbook.Book) *Book {
	out := &Book{Path: path, Version: 1, Targets: b.Targets}
	for ci, ch := range b.Chapters {
		for pi, para := range ch.Paragraphs {
			for si, s := range para {
				out.Sents = append(out.Sents, sentV1(s, fmt.Sprintf("ch%d p%d s%d", ci, pi, si)))
			}
		}
		for _, t := range ch.Tables {
			for ri, row := range t.Rows {
				for cj, cell := range row {
					for si, s := range cell.Sentences {
						loc := fmt.Sprintf("ch%d p%d r%dc%d s%d", ci, t.Para, ri, cj, si)
						out.Sents = append(out.Sents, sentV1(s, loc))
					}
				}
			}
		}
	}
	ids := make([]string, 0, len(b.Notes))
	for id := range b.Notes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		for pi, para := range b.Notes[id].Paragraphs {
			for si, s := range para {
				out.Sents = append(out.Sents, sentV1(s, fmt.Sprintf("note %s p%d s%d", id, pi, si)))
			}
		}
	}
	return out
}

func sentV1(s *tbook.Sentence, loc string) *Sentence {
	rs := []rune(s.Src)
	out := &Sentence{Loc: loc, Words: make([]string, len(s.Words)), Tr: map[string]*Cell{}}
	for i, w := range s.Words {
		a, b := clamp(w[0], 0, len(rs)), clamp(w[1], 0, len(rs))
		if b < a {
			b = a
		}
		out.Words[i] = string(rs[a:b])
	}
	for lang, tr := range s.Tr {
		if tr.Text == "" {
			continue
		}
		trs := []rune(tr.Text)
		c := &Cell{Text: tr.Text, Chunks: make([]Chunk, 0, len(tr.Align))}
		for _, ch := range tr.Align {
			c.Chunks = append(c.Chunks, newChunk(trs, ch.T[0], ch.T[1], ch.W))
		}
		out.Tr[lang] = c
	}
	return out
}

// ---- version 2 ----

func loadV2(path string) (*Book, error) {
	a, err := v2.ReadArchive(path)
	if err != nil {
		return nil, err
	}
	return FromV2Archive(path, a), nil
}

// FromV2Archive builds the common view from an in-memory version-2 archive, so
// a caller can score a migration without writing and re-reading it.
func FromV2Archive(path string, a *v2.Archive) *Book {
	langs := a.Manifest.Languages.Targets
	out := &Book{Path: path, Version: v2.FormatVersion, Targets: langs}
	for i, ref := range a.Manifest.Spine {
		if i >= len(a.Skeletons) {
			break
		}
		gloss := map[string]map[string]*v2.GlossParagraph{}
		if i < len(a.Overlays) {
			for lang, ov := range a.Overlays[i] {
				if ov != nil {
					gloss[lang] = byBlockID(ov.Paragraphs)
				}
			}
		}
		for _, blk := range a.Skeletons[i].Paragraphs {
			out.Sents = append(out.Sents, sentsV2(blk, ref.ID, langs, gloss)...)
		}
	}
	if len(a.Notes) > 0 {
		gloss := map[string]map[string]*v2.GlossParagraph{}
		for lang, ov := range a.NotesOverlays {
			if ov == nil {
				continue
			}
			m := map[string]*v2.GlossParagraph{}
			for _, ps := range ov.Notes {
				for k := range ps {
					m[ps[k].ID] = &ps[k]
				}
			}
			gloss[lang] = m
		}
		ids := make([]string, 0, len(a.Notes))
		for id := range a.Notes {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			for _, blk := range a.Notes[id].Paragraphs {
				out.Sents = append(out.Sents, sentsV2(blk, "note "+id, langs, gloss)...)
			}
		}
	}
	return out
}

func byBlockID(ps []v2.GlossParagraph) map[string]*v2.GlossParagraph {
	m := make(map[string]*v2.GlossParagraph, len(ps))
	for i := range ps {
		m[ps[i].ID] = &ps[i]
	}
	return m
}

// sentsV2 turns one skeleton block (a paragraph, or a table's cells) into
// common-view sentences, pairing each with its overlay translations by block id.
func sentsV2(blk v2.Block, chapter string, langs []string, gloss map[string]map[string]*v2.GlossParagraph) []*Sentence {
	if blk.Table != nil {
		var out []*Sentence
		for ri, row := range blk.Table.Rows {
			for cj := range row {
				cell := row[cj]
				cellGloss := map[string][]*v2.Translation{}
				for lang, m := range gloss {
					p := m[blk.ID]
					if p == nil || ri >= len(p.Rows) || cj >= len(p.Rows[ri]) {
						continue
					}
					cellGloss[lang] = p.Rows[ri][cj].S
				}
				out = append(out, blockSents(cell, fmt.Sprintf("%s %s r%dc%d", chapter, blk.ID, ri, cj), langs, cellGloss)...)
			}
		}
		return out
	}
	cellGloss := map[string][]*v2.Translation{}
	for lang, m := range gloss {
		if p := m[blk.ID]; p != nil {
			cellGloss[lang] = p.S
		}
	}
	return blockSents(blk, chapter+" "+blk.ID, langs, cellGloss)
}

func blockSents(blk v2.Block, loc string, langs []string, gloss map[string][]*v2.Translation) []*Sentence {
	rs := []rune(blk.Text)
	words := v2.SentenceWords(&blk)
	out := make([]*Sentence, 0, len(words))
	for k, ws := range words {
		s := &Sentence{Loc: fmt.Sprintf("%s s%d", loc, k), Words: make([]string, len(ws)), Tr: map[string]*Cell{}}
		for i, w := range ws {
			a, b := clamp(w[0], 0, len(rs)), clamp(w[1], 0, len(rs))
			if b < a {
				b = a
			}
			s.Words[i] = string(rs[a:b])
		}
		for _, lang := range langs {
			list := gloss[lang]
			if k >= len(list) || list[k] == nil || list[k].T == "" {
				continue
			}
			s.Tr[lang] = cellV2(list[k])
		}
		out = append(out, s)
	}
	return out
}

// cellV2 turns a version-2 translation into the common chunk view: one chunk
// per linked token (split into its parts where parts refine it) plus one per
// escape span. This is v2.HighlightRanges decomposed so that each highlighted
// fragment keeps the source words it belongs to.
func cellV2(tr *v2.Translation) *Cell {
	toks := v2.TargetTokens(tr)
	rs := []rune(tr.T)
	c := &Cell{Text: tr.T}
	for j, link := range tr.A {
		if j >= len(toks) || !link.Linked() {
			continue
		}
		ta, tb := toks[j][0], toks[j][1]
		covered := map[int]bool{}
		for _, p := range tr.Parts {
			if p.Token != j {
				continue
			}
			var w []int
			for _, i := range p.Link.Words {
				if link.Contains(i) {
					w = append(w, i)
					covered[i] = true
				}
			}
			if len(w) > 0 {
				c.Chunks = append(c.Chunks, newChunk(rs, ta+p.A, ta+p.B, w))
			}
		}
		var whole []int
		for _, i := range link.Words {
			if !covered[i] {
				whole = append(whole, i)
			}
		}
		if len(whole) > 0 {
			c.Chunks = append(c.Chunks, newChunk(rs, ta, tb, whole))
		}
	}
	for _, x := range tr.X {
		if len(x) < 3 {
			continue
		}
		c.Chunks = append(c.Chunks, newChunk(rs, x[0], x[1], append([]int(nil), x[2:]...)))
	}
	return c
}
