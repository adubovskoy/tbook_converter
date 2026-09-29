package migrate

import (
	"fmt"

	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
)

// Overlays builds the overlays of langs against the existing skeletons of arc
// (the append path, spec §2.4). b is the model ToBook read from arc, with the
// new languages translated into it; it may hold fewer chapters than arc (a
// --limit-chapters run), and those chapters simply get no overlay. Skeletons
// are never re-derived: every link is mapped onto the blocks as they are.
func Overlays(b *tbook.Book, arc *v2.Archive, langs []string, opts Options) ([]v2.Addition, *Stats, error) {
	c := &converter{opts: opts, stats: &Stats{}}
	if len(b.Chapters) > len(arc.Skeletons) {
		return nil, nil, fmt.Errorf("%d chapters for %d skeletons", len(b.Chapters), len(arc.Skeletons))
	}
	adds := make([]v2.Addition, len(langs))
	for i, lang := range langs {
		adds[i] = v2.Addition{Lang: lang, Overlays: map[int]*v2.Overlay{}}
	}
	for ci := range b.Chapters {
		ch := &b.Chapters[ci]
		skel := arc.Skeletons[ci]
		if len(ch.Paragraphs) != len(skel.Paragraphs) {
			return nil, nil, fmt.Errorf("chapter %d: %d paragraphs for %d blocks", ci+1, len(ch.Paragraphs), len(skel.Paragraphs))
		}
		tables := map[int]tbook.Table{}
		for _, t := range ch.Tables {
			tables[t.Para] = t
		}
		ovs := make([]*v2.Overlay, len(langs))
		for i, lang := range langs {
			ovs[i] = &v2.Overlay{Gates: c.gates(lang), Paragraphs: []v2.GlossParagraph{}}
		}
		for p := range skel.Paragraphs {
			blk := &skel.Paragraphs[p]
			if blk.Table == nil {
				_, trs, err := c.buildBlock(ch.Paragraphs[p], langs, blk)
				if err != nil {
					return nil, nil, fmt.Errorf("chapter %d: %w", ci+1, err)
				}
				for i, lang := range langs {
					ovs[i].Paragraphs = append(ovs[i].Paragraphs, v2.GlossParagraph{ID: blk.ID, S: trs[lang]})
				}
				continue
			}
			t, ok := tables[p]
			if !ok || len(t.Rows) != len(blk.Table.Rows) {
				return nil, nil, fmt.Errorf("chapter %d: table %s does not match the skeleton", ci+1, blk.ID)
			}
			rows := make([][][]v2.GlossCell, len(langs))
			for r, row := range blk.Table.Rows {
				if len(t.Rows[r]) != len(row) {
					return nil, nil, fmt.Errorf("chapter %d: table %s row %d does not match the skeleton", ci+1, blk.ID, r)
				}
				cells := make([][]v2.GlossCell, len(langs))
				for i := range cells {
					cells[i] = []v2.GlossCell{}
				}
				for cc := range row {
					_, trs, err := c.buildBlock(t.Rows[r][cc].Sentences, langs, &row[cc])
					if err != nil {
						return nil, nil, fmt.Errorf("chapter %d: table %s: %w", ci+1, blk.ID, err)
					}
					for i, lang := range langs {
						cells[i] = append(cells[i], v2.GlossCell{S: trs[lang]})
					}
				}
				for i := range langs {
					rows[i] = append(rows[i], cells[i])
				}
			}
			for i := range langs {
				if rows[i] == nil {
					rows[i] = [][]v2.GlossCell{}
				}
				ovs[i].Paragraphs = append(ovs[i].Paragraphs, v2.GlossParagraph{ID: blk.ID, Rows: rows[i]})
			}
		}
		for i := range langs {
			adds[i].Overlays[ci] = ovs[i]
		}
	}

	if len(arc.Notes) > 0 {
		for i, lang := range langs {
			adds[i].Notes = &v2.NotesOverlay{Gates: c.gates(lang), Notes: map[string][]v2.GlossParagraph{}}
		}
		for id, note := range arc.Notes {
			n := b.Notes[id]
			if n == nil || len(n.Paragraphs) != len(note.Paragraphs) {
				return nil, nil, fmt.Errorf("note %s does not match the skeleton", id)
			}
			per := make([][]v2.GlossParagraph, len(langs))
			for i := range per {
				per[i] = []v2.GlossParagraph{}
			}
			for p := range note.Paragraphs {
				blk := &note.Paragraphs[p]
				_, trs, err := c.buildBlock(n.Paragraphs[p], langs, blk)
				if err != nil {
					return nil, nil, fmt.Errorf("note %s: %w", id, err)
				}
				for i, lang := range langs {
					per[i] = append(per[i], v2.GlossParagraph{ID: blk.ID, S: trs[lang]})
				}
			}
			for i := range langs {
				adds[i].Notes.Notes[id] = per[i]
			}
		}
	}
	return adds, c.stats, nil
}
