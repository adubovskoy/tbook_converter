package migrate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
)

// ReadBook loads a .tbook of either format version into the version-1 model
// the pipeline and the bench tools work on. It returns the file's version.
func ReadBook(path string) (*tbook.Book, int, error) {
	ver, err := v2.FormatVersionOf(path)
	if err != nil {
		return nil, 0, err
	}
	if ver != v2.FormatVersion {
		b, err := tbook.Read(path)
		return b, ver, err
	}
	arc, err := v2.ReadArchive(path)
	if err != nil {
		return nil, ver, err
	}
	b, err := ToBook(arc)
	return b, ver, err
}

// ToBook is the inverse of Convert: it rebuilds the version-1 model from a
// version-2 archive. Each block becomes one paragraph whose sentences are the
// block's sentence ranges; source words are the block's word inventory; every
// linked target token and every escape becomes one alignment chunk.
//
// What version 1 cannot hold is dropped: statuses, verdicts, gates, groups,
// parts, units and inserted-token links. A cell without t (skipped, rejected)
// becomes an absent translation.
func ToBook(arc *v2.Archive) (*tbook.Book, error) {
	m := &arc.Manifest
	b := &tbook.Book{
		Title:   m.Title,
		Author:  strings.Join(m.Authors, ", "),
		Source:  m.Languages.Source,
		Targets: append([]string{}, m.Languages.Targets...),
		Cover:   arc.Cover,
		Meta:    m.Meta,
	}
	for name, data := range arc.Images {
		if b.Images == nil {
			b.Images = map[string][]byte{}
		}
		b.Images[name] = data
	}
	for name, data := range arc.Extra {
		if strings.HasPrefix(name, "images/") {
			if b.Images == nil {
				b.Images = map[string][]byte{}
			}
			b.Images[name] = data
		}
	}
	langs := b.Targets

	for i, skel := range arc.Skeletons {
		ref := m.Spine[i]
		ovs := arc.Overlays[i]
		ch := tbook.Chapter{Title: ref.Title}
		for p := range skel.Paragraphs {
			blk := &skel.Paragraphs[p]
			glossFor := func(lang string) *v2.GlossParagraph {
				ov := ovs[lang]
				if ov == nil || p >= len(ov.Paragraphs) || ov.Paragraphs[p].ID != blk.ID {
					return nil
				}
				return &ov.Paragraphs[p]
			}
			per := map[string][]*v2.Translation{}
			for _, lang := range langs {
				if gp := glossFor(lang); gp != nil {
					per[lang] = gp.S
				}
			}
			sents, err := blockSentences(blk, per)
			if err != nil {
				return nil, fmt.Errorf("spine[%d] paragraph %s: %w", i, blk.ID, err)
			}
			ch.Paragraphs = append(ch.Paragraphs, sents)
			ch.ParagraphStyles = append(ch.ParagraphStyles, blk.Role)
			if blk.Figure != nil {
				ch.Figures = append(ch.Figures, tbook.Figure{Para: p, Image: blk.Figure.Image, Alt: blk.Figure.Alt})
			}
			if blk.Table != nil {
				tb := tbook.Table{Para: p, Rows: [][]tbook.TableCell{}}
				for r, row := range blk.Table.Rows {
					cells := []tbook.TableCell{}
					for c := range row {
						cper := map[string][]*v2.Translation{}
						for _, lang := range langs {
							if gp := glossFor(lang); gp != nil && r < len(gp.Rows) && c < len(gp.Rows[r]) {
								cper[lang] = gp.Rows[r][c].S
							}
						}
						cs, err := blockSentences(&row[c], cper)
						if err != nil {
							return nil, fmt.Errorf("spine[%d] table %s: %w", i, blk.ID, err)
						}
						cells = append(cells, tbook.TableCell{Sentences: cs, Header: row[c].Header})
					}
					tb.Rows = append(tb.Rows, cells)
				}
				ch.Tables = append(ch.Tables, tb)
			}
		}
		b.Chapters = append(b.Chapters, ch)
	}

	if len(arc.Notes) > 0 {
		b.Notes = map[string]*tbook.Note{}
		for id, n := range arc.Notes {
			note := &tbook.Note{Label: n.Label, Kind: n.Kind}
			for p := range n.Paragraphs {
				blk := &n.Paragraphs[p]
				per := map[string][]*v2.Translation{}
				for _, lang := range langs {
					ov := arc.NotesOverlays[lang]
					if ov == nil {
						continue
					}
					if gps := ov.Notes[id]; len(gps) == len(n.Paragraphs) { // by position (§6.12)
						per[lang] = gps[p].S
					}
				}
				sents, err := blockSentences(blk, per)
				if err != nil {
					return nil, fmt.Errorf("note %s: %w", id, err)
				}
				note.Paragraphs = append(note.Paragraphs, sents)
			}
			if note.Paragraphs == nil {
				note.Paragraphs = [][]*tbook.Sentence{}
			}
			b.Notes[id] = note
		}
	}
	return b, nil
}

// blockSentences splits one block into version-1 sentences with their
// translations (per: lang → the overlay's translations of this block).
func blockSentences(blk *v2.Block, per map[string][]*v2.Translation) ([]*tbook.Sentence, error) {
	text := []rune(blk.Text)
	words := v2.SentenceWords(blk)
	out := make([]*tbook.Sentence, 0, len(blk.Sents))
	for k, r := range blk.Sents {
		if r[0] < 0 || r[1] > len(text) || r[0] > r[1] {
			return nil, fmt.Errorf("sentence %d range %v outside the text", k, r)
		}
		s := &tbook.Sentence{Src: string(text[r[0]:r[1]]), Words: [][2]int{}, Tr: map[string]tbook.Translation{}}
		for _, w := range words[k] {
			s.Words = append(s.Words, [2]int{w[0] - r[0], w[1] - r[0]})
		}
		for lang, trs := range per {
			if k < len(trs) && trs[k] != nil && trs[k].T != "" {
				s.Tr[lang] = toV1Translation(trs[k])
			}
		}
		out = append(out, s)
	}
	// Spans are clipped to the sentences they cover; a note marker belongs to
	// the first sentence whose range holds its point (a marker in a gap goes to
	// the end of the sentence before it).
	for _, sp := range blk.Spans {
		for k, r := range blk.Sents {
			a, e := max(sp.S, r[0]), min(sp.E, r[1])
			if a < e {
				out[k].Spans = append(out[k].Spans, tbook.Span{S: a - r[0], E: e - r[0], K: sp.K})
			}
		}
	}
	for _, nr := range blk.Notes {
		if len(out) == 0 {
			break
		}
		k := sort.Search(len(blk.Sents), func(i int) bool { return blk.Sents[i][1] >= nr.P })
		if k == len(blk.Sents) {
			k = len(blk.Sents) - 1
		}
		r := blk.Sents[k]
		if nr.P < r[0] && k > 0 {
			k--
			r = blk.Sents[k]
		}
		p := min(max(nr.P, r[0]), r[1]) - r[0]
		out[k].Notes = append(out[k].Notes, tbook.NoteRef{P: p, ID: nr.ID, Label: nr.Label})
	}
	return out, nil
}

// toV1Translation turns token links and escapes back into rune-range chunks.
func toV1Translation(tr *v2.Translation) tbook.Translation {
	out := tbook.Translation{Text: tr.T, Align: []tbook.AlignChunk{}}
	toks := v2.TargetTokens(tr)
	for j, l := range tr.A {
		if j >= len(toks) || !l.Linked() {
			continue
		}
		out.Align = append(out.Align, tbook.AlignChunk{T: toks[j], W: append([]int{}, l.Words...)})
	}
	for _, x := range tr.X {
		if len(x) < 3 {
			continue
		}
		out.Align = append(out.Align, tbook.AlignChunk{T: [2]int{x[0], x[1]}, W: append([]int{}, x[2:]...)})
	}
	sort.SliceStable(out.Align, func(p, q int) bool { return out.Align[p].T[0] < out.Align[q].T[0] })
	return out
}
