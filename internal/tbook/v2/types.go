// Package v2 implements the .tbook format version 2 (doc/specs/tbook-format-v2.md):
// the data model of manifest.json, the language-free chapter skeletons and the
// per-(chapter, language) overlays, the normative tokenizer tbook-w2, the id
// and digest recipes, a writer, a reader and a validator.
//
// Every offset in this package is a Unicode code-point offset into an NFC
// string, never a byte offset. Struct field order below is the key order the
// spec shows; encoding/json preserves it.
package v2

import (
	"encoding/json"

	"github.com/dimando/reader/converter/internal/tbook"
)

// Container constants (spec §2, §3.2, §3.3, §3.9).
const (
	FormatVersion = 2
	Mimetype      = "application/vnd.tbook+zip"
	TokenizerID   = "tbook-w2"
	SchemaEntry   = "schema/tbook-2.schema.json"
	SchemaID      = "https://tbook.dev/schema/tbook-2.schema.json"
	ManifestEntry = "manifest.json"
	MimetypeEntry = "mimetype"
)

// Feature ids every version-2 file lists in manifest.requires.
const (
	FeatureParagraph  = "text/paragraph"
	FeatureAlignToken = "align/token"
	FeatureTokenizer  = "tokenizer/" + TokenizerID
)

// Requires returns the mandatory feature ids in registration order.
func Requires() []string {
	return []string{FeatureParagraph, FeatureAlignToken, FeatureTokenizer}
}

// Translation statuses (spec §6.7).
const (
	StatusRaw        = "raw"
	StatusUnaligned  = "unaligned"
	StatusSkipped    = "skipped"
	StatusRejected   = "rejected"
	StatusUnverified = "unverified"
	StatusWrongLang  = "wrongLang"
	StatusFree       = "free"
)

// Registered gate ids (spec §6.8).
const (
	GateLangcheck = "langcheck"
	GateLexcheck  = "lexcheck"
	GateJudge     = "judge"
)

// Block roles mirror version 1 (spec §4.4).
const (
	RoleBody       = tbook.RoleBody
	RoleSubtitle   = tbook.RoleSubtitle
	RoleHeading    = tbook.RoleHeading
	RoleSceneBreak = tbook.RoleSceneBreak
	RoleFigure     = tbook.RoleFigure
	RoleTable      = tbook.RoleTable
)

// Manifest is manifest.json (spec §3).
type Manifest struct {
	FormatVersion int               `json:"formatVersion"`
	Requires      []string          `json:"requires"`
	Title         string            `json:"title"`
	Authors       []string          `json:"authors"`
	Languages     Languages         `json:"languages"`
	Text          TextDecl          `json:"text"`
	Cover         *string           `json:"cover"`
	Notes         NotesRef          `json:"notes"`
	Spine         []ChapterRef      `json:"spine"`
	Source        *Source           `json:"source,omitempty"`
	Schema        string            `json:"schema"`
	Digests       map[string]string `json:"digests"`
	Meta          *tbook.Meta       `json:"meta,omitempty"`
}

// Languages is manifest.languages (spec §3.4).
type Languages struct {
	Source  string   `json:"source"`
	Targets []string `json:"targets"`
}

// TextDecl is manifest.text (spec §3.3); every field is a constant under v2.
type TextDecl struct {
	Tokenizer     string `json:"tokenizer"`
	Offsets       string `json:"offsets"`
	Normalization string `json:"normalization"`
}

// DefaultTextDecl returns the only declaration version 2 registers.
func DefaultTextDecl() TextDecl {
	return TextDecl{Tokenizer: TokenizerID, Offsets: "codepoint", Normalization: "NFC"}
}

// NotesRef is manifest.notes (spec §3.7): {} without footnotes, else the
// skeleton entry and one overlay entry per language.
type NotesRef struct {
	Text  string
	Gloss map[string]string
}

// MarshalJSON emits {} for a book without footnotes and always an object for
// gloss otherwise, so the shape is stable for version-1 consumers (§10.4).
func (n NotesRef) MarshalJSON() ([]byte, error) {
	if n.Text == "" {
		return []byte("{}"), nil
	}
	gloss := n.Gloss
	if gloss == nil {
		gloss = map[string]string{}
	}
	return json.Marshal(struct {
		Text  string            `json:"text"`
		Gloss map[string]string `json:"gloss"`
	}{n.Text, gloss})
}

// UnmarshalJSON accepts both forms.
func (n *NotesRef) UnmarshalJSON(b []byte) error {
	var raw struct {
		Text  string            `json:"text"`
		Gloss map[string]string `json:"gloss"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*n = NotesRef{Text: raw.Text, Gloss: raw.Gloss}
	return nil
}

// ChapterRef is one manifest.spine entry (spec §3.5).
type ChapterRef struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Level      int               `json:"level"`
	Text       string            `json:"text"`
	Gloss      map[string]string `json:"gloss"`
	Paragraphs int               `json:"paragraphs,omitempty"`
	Sentences  int               `json:"sentences,omitempty"`
	Words      int               `json:"words,omitempty"`
}

// Source is manifest.source, the identity of the converted document (spec §3.6).
type Source struct {
	Kind       string      `json:"kind,omitempty"`
	SHA256     string      `json:"sha256,omitempty"`
	Identifier string      `json:"identifier,omitempty"`
	Title      string      `json:"title,omitempty"`
	Language   string      `json:"language,omitempty"`
	TextSHA256 string      `json:"textSha256,omitempty"`
	Docs       []SourceDoc `json:"docs,omitempty"`
}

// SourceDoc is one source document record of Source.Docs.
type SourceDoc struct {
	Href   string `json:"href"`
	Chars  int    `json:"chars"`
	SHA256 string `json:"sha256"`
	Title  string `json:"title,omitempty"`
}

// Chapter is a skeleton entry text/chN.json (spec §4).
type Chapter struct {
	ID         string  `json:"id"`
	Paragraphs []Block `json:"paragraphs"`
}

// Span and NoteRef keep the version-1 shape (spec §4.7, §4.8).
type (
	Span    = tbook.Span
	NoteRef = tbook.NoteRef
)

// Block is a paragraph, a table cell or a footnote paragraph (spec §4.2).
type Block struct {
	ID     string     `json:"id,omitempty"`
	Role   string     `json:"role,omitempty"`
	Text   string     `json:"text"`
	Sents  [][2]int   `json:"sents"`
	Words  [][][2]int `json:"words,omitempty"`
	Spans  []Span     `json:"spans,omitempty"`
	Notes  []NoteRef  `json:"notes,omitempty"`
	Units  []Unit     `json:"units,omitempty"`
	Figure *Figure    `json:"figure,omitempty"`
	Table  *Table     `json:"table,omitempty"`
	Header bool       `json:"header,omitempty"`
}

// Unit is a source-side multi-word expression (spec §4.9).
type Unit struct {
	S int    `json:"s"`
	W []int  `json:"w"`
	K string `json:"k,omitempty"`
}

// Figure attaches an image to a figure-role block (spec §4.10).
type Figure struct {
	Image string `json:"image"`
	Alt   string `json:"alt,omitempty"`
}

// Table holds the cells of a table-role block (spec §4.11).
type Table struct {
	Rows [][]Block `json:"rows"`
}

// Notes is text/notes.json: note id → body (spec §4.12).
type Notes map[string]Note

// Note is one footnote body.
type Note struct {
	Label      string  `json:"label"`
	Kind       string  `json:"kind,omitempty"`
	Paragraphs []Block `json:"paragraphs"`
}

// Overlay is gloss/chN.<lang>.json (spec §6).
type Overlay struct {
	Lang        string           `json:"lang"`
	Chapter     string           `json:"chapter"`
	Gates       []string         `json:"gates"`
	AlignDigest string           `json:"alignDigest"`
	Paragraphs  []GlossParagraph `json:"paragraphs"`
}

// GlossParagraph is one overlay paragraph: {id, s} for a text block or
// {id, rows} for a table block (spec §6.1). Rows == nil means a text block.
type GlossParagraph struct {
	ID   string
	S    []*Translation
	Rows [][]GlossCell
}

// GlossCell is one translated table cell.
type GlossCell struct {
	S []*Translation `json:"s"`
}

func (p GlossParagraph) MarshalJSON() ([]byte, error) {
	if p.Rows != nil {
		return json.Marshal(struct {
			ID   string        `json:"id"`
			Rows [][]GlossCell `json:"rows"`
		}{p.ID, p.Rows})
	}
	s := p.S
	if s == nil {
		s = []*Translation{}
	}
	return json.Marshal(struct {
		ID string         `json:"id"`
		S  []*Translation `json:"s"`
	}{p.ID, s})
}

func (p *GlossParagraph) UnmarshalJSON(b []byte) error {
	var raw struct {
		ID   string         `json:"id"`
		S    []*Translation `json:"s"`
		Rows [][]GlossCell  `json:"rows"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*p = GlossParagraph{ID: raw.ID, S: raw.S, Rows: raw.Rows}
	return nil
}

// Translation is one sentence's rendering in one language (spec §6.2).
type Translation struct {
	T      string   `json:"t,omitempty"`
	A      []Link   `json:"a,omitempty"`
	X      [][]int  `json:"x,omitempty"`
	Words  [][2]int `json:"words,omitempty"`
	S      string   `json:"s,omitempty"`
	V      *Verdict `json:"v,omitempty"`
	Lang   string   `json:"lang,omitempty"`
	Groups []Group  `json:"groups,omitempty"`
	Parts  []Part   `json:"parts,omitempty"`
}

// Verdict is the result of a quality gate that did not pass a cell (spec §6.8).
type Verdict struct {
	OK  bool   `json:"ok"`
	By  string `json:"by"`
	Why string `json:"why,omitempty"`
}

// Group is [kind, targetTokens[], sourceWords[]] (spec §6.10).
type Group struct {
	Kind   string
	Tokens []int
	Words  []int
}

func (g Group) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{g.Kind, nonNil(g.Tokens), nonNil(g.Words)})
}

func (g *Group) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if len(raw) != 3 {
		return errShape("group must be [kind, tokens, words]")
	}
	var out Group
	if err := json.Unmarshal(raw[0], &out.Kind); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &out.Tokens); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[2], &out.Words); err != nil {
		return err
	}
	*g = out
	return nil
}

// Part is [token, a, b, link]: a sub-token span with its own link (spec §6.11).
type Part struct {
	Token int
	A, B  int
	Link  Link
}

func (p Part) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{p.Token, p.A, p.B, p.Link})
}

func (p *Part) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if len(raw) != 4 {
		return errShape("part must be [token, a, b, link]")
	}
	var out Part
	for i, dst := range []*int{&out.Token, &out.A, &out.B} {
		if err := json.Unmarshal(raw[i], dst); err != nil {
			return err
		}
	}
	if err := json.Unmarshal(raw[3], &out.Link); err != nil {
		return err
	}
	*p = out
	return nil
}

// NotesOverlay is gloss/notes.<lang>.json (spec §6.12).
type NotesOverlay struct {
	Lang        string                      `json:"lang"`
	Gates       []string                    `json:"gates"`
	AlignDigest string                      `json:"alignDigest"`
	Notes       map[string][]GlossParagraph `json:"notes"`
}

type errShape string

func (e errShape) Error() string { return string(e) }

func nonNil(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}
