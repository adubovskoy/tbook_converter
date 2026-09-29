package v2

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// EntryDigest returns "sha256:<hex>" of an entry's uncompressed bytes (spec §9.1).
func EntryDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// AlignDigest computes the alignment-pair digest of a chapter overlay against
// its skeleton (spec §9.2).
func AlignDigest(ov *Overlay, skel *Chapter) string {
	var sb strings.Builder
	for p, gp := range ov.Paragraphs {
		if p >= len(skel.Paragraphs) {
			break
		}
		digestParagraph(&sb, &gp, &skel.Paragraphs[p])
	}
	return EntryDigest([]byte(sb.String()))
}

// NotesAlignDigest computes the digest of a footnote overlay: notes in
// ascending code-point order of their ids, then paragraphs (spec §6.12).
func NotesAlignDigest(ov *NotesOverlay, notes Notes) string {
	ids := make([]string, 0, len(ov.Notes))
	for id := range ov.Notes {
		ids = append(ids, id)
	}
	sort.Strings(ids) // byte order of UTF-8 equals code-point order
	var sb strings.Builder
	for _, id := range ids {
		note, ok := notes[id]
		if !ok {
			continue
		}
		for p, gp := range ov.Notes[id] {
			if p >= len(note.Paragraphs) {
				break
			}
			digestParagraph(&sb, &gp, &note.Paragraphs[p])
		}
	}
	return EntryDigest([]byte(sb.String()))
}

func digestParagraph(sb *strings.Builder, gp *GlossParagraph, b *Block) {
	if gp.Rows != nil {
		if b.Table == nil {
			return
		}
		for r, row := range gp.Rows {
			if r >= len(b.Table.Rows) {
				break
			}
			for c, cell := range row {
				if c >= len(b.Table.Rows[r]) {
					break
				}
				digestSentences(sb, cell.S, &b.Table.Rows[r][c])
			}
		}
		return
	}
	digestSentences(sb, gp.S, b)
}

func digestSentences(sb *strings.Builder, s []*Translation, b *Block) {
	words := SentenceWords(b)
	src := []rune(b.Text)
	for k, tr := range s {
		if tr == nil || tr.T == "" || k >= len(words) {
			continue
		}
		word := func(i int) string {
			if i < 0 || i >= len(words[k]) {
				return ""
			}
			w := words[k][i]
			return string(src[clamp(w[0], 0, len(src)):clamp(w[1], 0, len(src))])
		}
		tgt := []rune(tr.T)
		slice := func(a, b int) string {
			a, b = clamp(a, 0, len(tgt)), clamp(b, 0, len(tgt))
			if b < a {
				b = a
			}
			return string(tgt[a:b])
		}
		toks := TargetTokens(tr)
		for j, link := range tr.A {
			if j >= len(toks) || link.IsNull() {
				continue
			}
			tok := slice(toks[j][0], toks[j][1])
			if link.Inserted {
				pair(sb, "", tok)
				continue
			}
			for _, i := range ascending(link.Words) {
				pair(sb, word(i), tok)
			}
		}
		for _, x := range tr.X {
			if len(x) < 2 {
				continue
			}
			frag := slice(x[0], x[1])
			if len(x) == 2 {
				pair(sb, "", frag)
				continue
			}
			for _, i := range ascending(x[2:]) {
				pair(sb, word(i), frag)
			}
		}
	}
}

func pair(sb *strings.Builder, word, tok string) {
	sb.WriteString(word)
	sb.WriteByte('\t')
	sb.WriteString(tok)
	sb.WriteByte('\n')
}

func ascending(idx []int) []int {
	if sort.IntsAreSorted(idx) {
		return idx
	}
	out := append([]int(nil), idx...)
	sort.Ints(out)
	return out
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
