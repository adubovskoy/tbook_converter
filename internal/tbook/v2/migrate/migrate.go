// Package migrate converts a version-1 .tbook (the in-memory tbook.Book) into
// a version-2 archive following Annex A of the version-2 specification: one
// paragraph text per block with sentence ranges, one link per tbook-w2 target
// token, escape chunks for spans that are not whole tokens, digit
// pass-through, and the unaligned status.
//
// The converter's --format v2 path uses the same mapping with an Annotate hook
// that writes the statuses and verdicts the pipeline actually knows.
package migrate

import (
	"fmt"
	"regexp"
	"sort"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dimando/reader/converter/internal/buildinfo"
	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
	"golang.org/x/text/unicode/norm"
)

// Options tune the conversion. The zero value is the normative migration.
type Options struct {
	// Annotate, when set, decides s, v and lang of one translation from
	// producer state instead of the migration rule of Annex A.4. tr already
	// carries t, a and x; HasLinks says whether any link or escape exists.
	Annotate func(s *tbook.Sentence, lang string, tr *v2.Translation, hasLinks bool)
	// Skipped reports a sentence deliberately left untranslated in lang: it
	// becomes {s: skipped} instead of null. nil = never.
	Skipped func(s *tbook.Sentence, lang string) bool
	// Gates lists the quality gates that ran over every translation of lang.
	// nil = none.
	Gates func(lang string) []string
	// Source is the manifest.source block, when the caller knows it.
	Source *v2.Source
	// Run is the provenance record appended to meta.runs; nil = a migration
	// record naming this tool.
	Run *tbook.RunRecord
	// Now stamps the default run record; zero = time.Now.
	Now time.Time
}

// Stats counts what the mapping did, for reports and cross-checks.
type Stats struct {
	Sentences         int
	Translations      int // non-empty translations
	Chunks            int // v1 chunks seen
	TokenLinks        int // chunks that became a link on exactly one token
	MergedSameToken   int // chunks unioned into an already linked token
	Escapes           int // chunks that became x entries
	EscapesMerged     int // x entries folded into a linked token they overlapped
	Orphaned          int // chunks whose every source word had no v2 counterpart
	CrossSentenceRefs int // source indices dropped because their token moved sentence
	DigitPassthrough  int
	Unaligned         int // translations written with s: unaligned
	JoinsWithoutSpace int
	NotNFC            int // strings normalized on the way
}

// Convert maps a version-1 book to a version-2 archive. The archive is not
// yet written: call (*v2.Archive).Write, which seals digests.
func Convert(b *tbook.Book, opts Options) (*v2.Archive, *Stats, error) {
	c := &converter{opts: opts, stats: &Stats{}}
	arc := &v2.Archive{
		Manifest: v2.Manifest{
			Title:     b.Title,
			Authors:   authors(b.Author),
			Languages: v2.Languages{Source: b.Source, Targets: append([]string{}, b.Targets...)},
			Source:    opts.Source,
		},
		Cover:  b.Cover,
		Images: b.Images,
	}
	if arc.Manifest.Languages.Targets == nil {
		arc.Manifest.Languages.Targets = []string{}
	}
	langs := b.Targets

	spineIDs := v2.IDSet{}
	for i, ch := range b.Chapters {
		title := ch.Title
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		skel, overlays, counts := c.chapter(&ch, langs)
		skel.ID = spineIDs.Add(v2.ChapterIDOf(title, skel))
		ref := v2.ChapterRef{ID: skel.ID, Title: title, Gloss: map[string]string{},
			Paragraphs: counts[0], Sentences: counts[1], Words: counts[2]}
		arc.Manifest.Spine = append(arc.Manifest.Spine, ref)
		arc.Skeletons = append(arc.Skeletons, skel)
		arc.Overlays = append(arc.Overlays, overlays)
	}

	if len(b.Notes) > 0 {
		arc.Notes = v2.Notes{}
		arc.NotesOverlays = map[string]*v2.NotesOverlay{}
		for _, lang := range langs {
			arc.NotesOverlays[lang] = &v2.NotesOverlay{Gates: c.gates(lang), Notes: map[string][]v2.GlossParagraph{}}
		}
		for id, n := range b.Notes {
			ids := v2.IDSet{}
			note := v2.Note{Label: n.Label, Kind: n.Kind, Paragraphs: []v2.Block{}}
			if note.Kind == tbook.NoteKindNote {
				note.Kind = ""
			}
			per := map[string][]v2.GlossParagraph{}
			for _, para := range n.Paragraphs {
				blk, trs := c.block(para, langs)
				blk.block.ID = ids.Add(v2.ParagraphID(blk.block.Text))
				note.Paragraphs = append(note.Paragraphs, blk.block)
				for _, lang := range langs {
					per[lang] = append(per[lang], v2.GlossParagraph{ID: blk.block.ID, S: trs[lang]})
				}
			}
			arc.Notes[id] = note
			for _, lang := range langs {
				if per[lang] == nil {
					per[lang] = []v2.GlossParagraph{}
				}
				arc.NotesOverlays[lang].Notes[id] = per[lang]
			}
		}
	}

	meta := b.Meta
	if meta == nil {
		meta = &tbook.Meta{}
	}
	run := opts.Run
	if run == nil {
		now := opts.Now
		if now.IsZero() {
			now = time.Now()
		}
		run = &tbook.RunRecord{
			At:       now.UTC().Format(time.RFC3339),
			Producer: &tbook.Producer{Name: "tbook-migrate", Version: buildinfo.Version, Commit: buildinfo.Commit(), URL: buildinfo.URL},
			Targets:  append([]string{}, langs...),
			Options:  map[string]any{"convertedFrom": "tbook/1", "tokenizer": v2.TokenizerID},
		}
	}
	meta.AppendRun(*run)
	arc.Manifest.Meta = meta
	return arc, c.stats, nil
}

// File migrates in.tbook to out.tbook and returns the mapping statistics.
func File(in, out string, opts Options) (*Stats, error) {
	b, err := tbook.Read(in)
	if err != nil {
		return nil, err
	}
	arc, stats, err := Convert(b, opts)
	if err != nil {
		return nil, err
	}
	if err := arc.Write(out); err != nil {
		return nil, err
	}
	return stats, nil
}

func authors(a string) []string {
	if a == "" {
		return []string{}
	}
	return []string{a}
}

type converter struct {
	opts  Options
	stats *Stats
}

func (c *converter) gates(lang string) []string {
	if c.opts.Gates == nil {
		return []string{}
	}
	g := c.opts.Gates(lang)
	if g == nil {
		return []string{}
	}
	return g
}

// chapter builds one skeleton and its overlays. counts = paragraphs,
// sentences, words.
func (c *converter) chapter(ch *tbook.Chapter, langs []string) (*v2.Chapter, map[string]*v2.Overlay, [3]int) {
	skel := &v2.Chapter{Paragraphs: []v2.Block{}}
	overlays := map[string]*v2.Overlay{}
	for _, lang := range langs {
		overlays[lang] = &v2.Overlay{Gates: c.gates(lang), Paragraphs: []v2.GlossParagraph{}}
	}
	figures := map[int]tbook.Figure{}
	for _, f := range ch.Figures {
		figures[f.Para] = f
	}
	tables := map[int]tbook.Table{}
	for _, t := range ch.Tables {
		tables[t.Para] = t
	}
	ids := v2.IDSet{}
	var counts [3]int
	for p, para := range ch.Paragraphs {
		role := ""
		if p < len(ch.ParagraphStyles) && ch.ParagraphStyles[p] != "" && ch.ParagraphStyles[p] != tbook.RoleBody {
			role = ch.ParagraphStyles[p]
		}
		blk, trs := c.block(para, langs)
		block := blk.block
		block.ID = ids.Add(v2.ParagraphID(block.Text))
		block.Role = role
		counts[1] += len(block.Sents)
		for _, ws := range blk.words {
			counts[2] += len(ws)
		}
		if f, ok := figures[p]; ok {
			block.Figure = &v2.Figure{Image: f.Image, Alt: f.Alt}
		}
		if t, ok := tables[p]; ok {
			block.Table = &v2.Table{Rows: [][]v2.Block{}}
			rowsTr := map[string][][]v2.GlossCell{}
			for _, row := range t.Rows {
				cells := []v2.Block{}
				cellsTr := map[string][]v2.GlossCell{}
				for _, cell := range row {
					cb, ctr := c.block(cell.Sentences, langs)
					cb.block.Header = cell.Header
					cells = append(cells, cb.block)
					counts[1] += len(cb.block.Sents)
					for _, ws := range cb.words {
						counts[2] += len(ws)
					}
					for _, lang := range langs {
						cellsTr[lang] = append(cellsTr[lang], v2.GlossCell{S: ctr[lang]})
					}
				}
				block.Table.Rows = append(block.Table.Rows, cells)
				for _, lang := range langs {
					if cellsTr[lang] == nil {
						cellsTr[lang] = []v2.GlossCell{}
					}
					rowsTr[lang] = append(rowsTr[lang], cellsTr[lang])
				}
			}
			for _, lang := range langs {
				if rowsTr[lang] == nil {
					rowsTr[lang] = [][]v2.GlossCell{}
				}
				overlays[lang].Paragraphs = append(overlays[lang].Paragraphs, v2.GlossParagraph{ID: block.ID, Rows: rowsTr[lang]})
			}
		} else {
			for _, lang := range langs {
				overlays[lang].Paragraphs = append(overlays[lang].Paragraphs, v2.GlossParagraph{ID: block.ID, S: trs[lang]})
			}
		}
		skel.Paragraphs = append(skel.Paragraphs, block)
	}
	counts[0] = len(skel.Paragraphs)
	return skel, overlays, counts
}

// blockCtx is one text block plus what the overlays need to map links.
type blockCtx struct {
	block  v2.Block
	ranges [][2]int          // per v1 sentence: its range in block text
	words  [][][2]int        // per v2 sentence: word ranges (block coordinates)
	wmap   map[[2]int][2]int // (k, i) v1 word → (k2, i2) v2 word
	filled map[[2]int]bool   // v2 words that contain ≥ 1 v1 word
	srcMap []func(int) int   // per v1 sentence: raw → NFC offset map
}

// block joins a v1 sentence list into one block (Annex A.2) and builds every
// language's translations for it (A.3, A.4).
func (c *converter) block(sents []*tbook.Sentence, langs []string) (*blockCtx, map[string][]*v2.Translation) {
	ctx, trs, _ := c.buildBlock(sents, langs, nil)
	return ctx, trs
}

// buildBlock is block with an optional preset: an existing skeleton block the
// sentences were read from (the append path, §2.4). The preset's text, ranges
// and words are used as they are, and the sentences must match them.
func (c *converter) buildBlock(sents []*tbook.Sentence, langs []string, preset *v2.Block) (*blockCtx, map[string][]*v2.Translation, error) {
	ctx := &blockCtx{wmap: map[[2]int][2]int{}, filled: map[[2]int]bool{}}
	if preset != nil {
		if len(sents) != len(preset.Sents) {
			return nil, nil, fmt.Errorf("paragraph %s: %d sentences for %d ranges", preset.ID, len(sents), len(preset.Sents))
		}
		text := []rune(preset.Text)
		for k, s := range sents {
			src, m := c.nfc(s.Src)
			r := preset.Sents[k]
			if r[0] < 0 || r[1] > len(text) || r[0] > r[1] || string(text[r[0]:r[1]]) != src {
				return nil, nil, fmt.Errorf("paragraph %s: sentence %d differs from the skeleton", preset.ID, k)
			}
			ctx.ranges = append(ctx.ranges, r)
			ctx.srcMap = append(ctx.srcMap, m)
			c.stats.Sentences++
		}
		ctx.block = *preset
	} else {
		var text []rune
		prev := ""
		for k, s := range sents {
			src, m := c.nfc(s.Src)
			if k > 0 {
				if joinWithoutSpace(prev, src) {
					c.stats.JoinsWithoutSpace++
				} else {
					text = append(text, ' ')
				}
			}
			a := len(text)
			text = append(text, []rune(src)...)
			ctx.ranges = append(ctx.ranges, [2]int{a, len(text)})
			ctx.srcMap = append(ctx.srcMap, m)
			prev = src
			c.stats.Sentences++
		}
		ctx.block = v2.Block{Text: string(text), Sents: append([][2]int{}, ctx.ranges...)}
		if ctx.block.Sents == nil {
			ctx.block.Sents = [][2]int{}
		}
	}
	ctx.words = v2.SentenceWords(&ctx.block)

	// v1 word → containing v2 word, by offsets.
	type tok struct{ a, b, k, i int }
	var all []tok
	for k, ws := range ctx.words {
		for i, w := range ws {
			all = append(all, tok{w[0], w[1], k, i})
		}
	}
	for k, s := range sents {
		d := ctx.ranges[k][0]
		for i, w := range s.Words {
			a, b := ctx.srcMap[k](w[0])+d, ctx.srcMap[k](w[1])+d
			j := sort.Search(len(all), func(x int) bool { return all[x].a > a }) - 1
			if j < 0 || !(all[j].a <= a && b <= all[j].b) {
				continue // no v2 counterpart (never in the corpus)
			}
			ctx.wmap[[2]int{k, i}] = [2]int{all[j].k, all[j].i}
			ctx.filled[[2]int{all[j].k, all[j].i}] = true
		}
		if preset != nil {
			continue // the skeleton already carries spans and markers
		}
		for _, sp := range s.Spans {
			ctx.block.Spans = append(ctx.block.Spans, v2.Span{S: ctx.srcMap[k](sp.S) + d, E: ctx.srcMap[k](sp.E) + d, K: sp.K})
		}
		for _, nr := range s.Notes {
			ctx.block.Notes = append(ctx.block.Notes, v2.NoteRef{P: ctx.srcMap[k](nr.P) + d, ID: nr.ID, Label: nr.Label})
		}
	}

	trs := map[string][]*v2.Translation{}
	for _, lang := range langs {
		list := make([]*v2.Translation, len(sents))
		for k, s := range sents {
			list[k] = c.translation(s, k, lang, ctx)
		}
		trs[lang] = list
	}
	return ctx, trs, nil
}

var digitToken = regexp.MustCompile(`^\p{N}[\p{N}.,:]*$`)

// insideEscape reports whether token t overlaps any escape chunk; linking it
// would break §6.5.
func insideEscape(t [2]int, escapes [][]int) bool {
	for _, x := range escapes {
		if t[0] < x[1] && x[0] < t[1] {
			return true
		}
	}
	return false
}

// translation maps one v1 translation to a v2 Translation (Annex A.3, A.4).
func (c *converter) translation(s *tbook.Sentence, k int, lang string, ctx *blockCtx) *v2.Translation {
	tr, ok := s.Tr[lang]
	if !ok || tr.Text == "" {
		if c.opts.Skipped != nil && c.opts.Skipped(s, lang) {
			out := &v2.Translation{S: v2.StatusSkipped}
			if c.opts.Annotate != nil {
				c.opts.Annotate(s, lang, out, false)
			}
			return out
		}
		return nil
	}
	c.stats.Translations++
	text, tm := c.nfc(tr.Text)
	out := &v2.Translation{T: text}
	toks := v2.Tokenize(text)
	links := make([]v2.Link, len(toks))
	var escapes [][]int
	for _, ch := range tr.Align {
		c.stats.Chunks++
		var w2 []int
		for _, i := range ch.W {
			m, ok := ctx.wmap[[2]int{k, i}]
			if !ok || m[0] != k {
				c.stats.CrossSentenceRefs++
				continue
			}
			w2 = append(w2, m[1])
		}
		w2 = sortedUnique(w2)
		if len(w2) == 0 {
			c.stats.Orphaned++
			continue
		}
		a, b := tm(ch.T[0]), tm(ch.T[1])
		if j := exactToken(toks, a, b); j >= 0 {
			if links[j].IsNull() {
				links[j] = v2.Link{Words: w2}
				c.stats.TokenLinks++
			} else {
				links[j] = v2.Link{Words: sortedUnique(append(append([]int{}, links[j].Words...), w2...))}
				c.stats.MergedSameToken++
			}
			continue
		}
		if a < b && hasLetterOrDigit([]rune(text), a, b) {
			escapes = append(escapes, append([]int{a, b}, w2...))
			c.stats.Escapes++
		}
	}
	// An escape overlapping a token that received a link is illegal (§6.5):
	// fold its words into that token's link instead.
	if len(escapes) > 0 {
		kept := escapes[:0]
		for _, x := range escapes {
			merged := false
			for j, t := range toks {
				if t[0] < x[1] && x[0] < t[1] && links[j].Linked() {
					links[j] = v2.Link{Words: sortedUnique(append(append([]int{}, links[j].Words...), x[2:]...))}
					merged = true
				}
			}
			if merged {
				c.stats.EscapesMerged++
				c.stats.Escapes--
			} else {
				kept = append(kept, x)
			}
		}
		escapes = kept
	}
	// Digit pass-through (A.3 step 4).
	src := []rune(ctx.block.Text)
	tgt := []rune(text)
	for i2, w := range ctx.words[k] {
		if ctx.filled[[2]int{k, i2}] {
			continue
		}
		word := string(src[w[0]:w[1]])
		if !digitToken.MatchString(word) {
			continue
		}
		cand := -1
		for j, t := range toks {
			if string(tgt[t[0]:t[1]]) == word {
				if cand >= 0 {
					cand = -2
					break
				}
				cand = j
			}
		}
		if cand >= 0 && links[cand].IsNull() && !insideEscape(toks[cand], escapes) {
			links[cand] = v2.Single(i2)
			c.stats.DigitPassthrough++
		}
	}
	n := len(links)
	for n > 0 && links[n-1].IsNull() {
		n--
	}
	if n > 0 {
		out.A = links[:n]
	}
	if len(escapes) > 0 {
		out.X = escapes
	}
	hasLinks := len(out.A) > 0 || len(out.X) > 0
	if c.opts.Annotate != nil {
		c.opts.Annotate(s, lang, out, hasLinks)
	} else if !hasLinks && len(toks) > 0 {
		out.S = v2.StatusUnaligned
		c.stats.Unaligned++
	}
	return out
}

// nfc normalizes s and returns a map from code-point offsets of s to offsets
// of the normalized string (identity when s is already NFC).
func (c *converter) nfc(s string) (string, func(int) int) {
	if norm.NFC.IsNormalString(s) {
		return s, func(i int) int { return i }
	}
	c.stats.NotNFC++
	// Walk NFC segment boundaries: an offset inside a segment maps to the
	// segment's start in the normalized string.
	var starts, targets []int
	out := ""
	src, cp := s, 0
	for len(src) > 0 {
		n := norm.NFC.NextBoundaryInString(src, true)
		if n <= 0 {
			n = len(src)
		}
		seg := src[:n]
		starts = append(starts, cp)
		targets = append(targets, utf8.RuneCountInString(out))
		out += norm.NFC.String(seg)
		cp += utf8.RuneCountInString(seg)
		src = src[n:]
	}
	total := utf8.RuneCountInString(out)
	return out, func(i int) int {
		if i >= cp {
			return total
		}
		j := sort.Search(len(starts), func(x int) bool { return starts[x] > i }) - 1
		if j < 0 {
			return 0
		}
		return targets[j]
	}
}

// joinWithoutSpace is the Annex A.2 rule: no space when the previous sentence
// ends in a letter or digit and the next starts with a lowercase letter, or
// the previous ends in a letter + U+2019 and the next begins with a lone s.
func joinWithoutSpace(prev, next string) bool {
	if prev == "" || next == "" {
		return false
	}
	pr := []rune(prev)
	nr := []rune(next)
	a, b := pr[len(pr)-1], nr[0]
	if (unicode.IsLetter(a) || unicode.IsDigit(a)) && unicode.IsLetter(b) && unicode.IsLower(b) {
		return true
	}
	if a == '’' && len(pr) > 1 && unicode.IsLetter(pr[len(pr)-2]) && b == 's' &&
		(len(nr) == 1 || !(unicode.IsLetter(nr[1]) || unicode.IsDigit(nr[1]))) {
		return true
	}
	return false
}

func exactToken(toks [][2]int, a, b int) int {
	j := sort.Search(len(toks), func(x int) bool { return toks[x][0] >= a })
	if j < len(toks) && toks[j][0] == a && toks[j][1] == b {
		return j
	}
	return -1
}

func hasLetterOrDigit(rs []rune, a, b int) bool {
	if a < 0 {
		a = 0
	}
	if b > len(rs) {
		b = len(rs)
	}
	for _, r := range rs[a:b] {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

func sortedUnique(idx []int) []int {
	if len(idx) == 0 {
		return nil
	}
	sort.Ints(idx)
	out := idx[:1]
	for _, v := range idx[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}
