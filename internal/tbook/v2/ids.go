package v2

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// NFC normalizes s to Unicode NFC (spec §7).
func NFC(s string) string { return norm.NFC.String(s) }

// NormTitle is step 1 of the chapter-id recipe (spec §3.5.1): NFC, every run
// of White_Space collapsed to one U+0020, trimmed, then full case folding.
func NormTitle(title string) string {
	var sb strings.Builder
	space := false
	for _, r := range NFC(title) {
		if unicode.Is(unicode.White_Space, r) {
			space = true
			continue
		}
		if space && sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		space = false
		sb.WriteRune(r)
	}
	return cases.Fold().String(sb.String())
}

// ChapterID derives the unsuffixed chapter id from the title and the texts of
// the first up-to-three body paragraphs (spec §3.5.1, steps 1–4). Repeats are
// suffixed by the caller through an IDSet.
func ChapterID(title string, bodies []string) string {
	var sb strings.Builder
	sb.WriteString(NormTitle(title))
	for i, b := range bodies {
		if i == 3 {
			break
		}
		sb.WriteByte('\n')
		sb.WriteString(NFC(b))
	}
	return "c" + shortHash(sb.String())
}

// ChapterIDOf applies ChapterID to a skeleton: its body-role paragraphs are
// the recipe's bodies.
func ChapterIDOf(title string, ch *Chapter) string {
	var bodies []string
	for _, p := range ch.Paragraphs {
		if p.Role == "" || p.Role == RoleBody {
			bodies = append(bodies, p.Text)
			if len(bodies) == 3 {
				break
			}
		}
	}
	return ChapterID(title, bodies)
}

// ParagraphID derives the unsuffixed paragraph id from the block text (spec §4.3).
func ParagraphID(text string) string { return shortHash(NFC(text)) }

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:4])
}

// IDSet assigns the -2, -3, … suffixes to repeated ids within one scope (a
// chapter's paragraphs, or the spine).
type IDSet map[string]int

// Add returns id, or id-N for its N-th occurrence in this scope.
func (s IDSet) Add(id string) string {
	s[id]++
	if n := s[id]; n > 1 {
		return id + "-" + strconv.Itoa(n)
	}
	return id
}
