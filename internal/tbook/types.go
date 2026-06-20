// Package tbook holds the .tbook data model plus archive assembly and
// validation. It is the leaf package every other internal package builds on.
//
// The on-disk format (version 1) is specified in doc/specs/tbook-format.md. All
// character offsets in `Words`, `AlignChunk.T`, and `Span` are CODE-POINT (rune)
// indices, matching the spec and the Android consumer — never byte offsets.
package tbook

// AlignChunk maps a [start,end) rune span of a translation's text to the source
// word indices (into Sentence.Words) it translates.
type AlignChunk struct {
	T [2]int `json:"t"`
	W []int  `json:"w"`
}

// Translation is one target-language rendering of a sentence.
type Translation struct {
	Text  string       `json:"text"`
	Align []AlignChunk `json:"align"`
}

// Span marks an inline-emphasis run inside a sentence's Src: the half-open rune
// range [S,E) is styled K, where K is "i" (italic) or "b" (bold). Spans are
// optional — absent/empty means no emphasis.
type Span struct {
	S int    `json:"s"`
	E int    `json:"e"`
	K string `json:"k"`
}

// Span kinds.
const (
	SpanItalic = "i"
	SpanBold   = "b"
)

// Sentence is the atomic translatable unit. Words are [start,end) rune offsets
// of each tappable source word in Src. Tr maps a language code to its rendering.
// Spans mark inline italic/bold runs in Src; omitted when there are none.
type Sentence struct {
	Src   string                 `json:"src"`
	Words [][2]int               `json:"words"`
	Tr    map[string]Translation `json:"tr"`
	Spans []Span                 `json:"spans,omitempty"`
}

// Paragraph roles. Body is the implicit default and is never emitted.
const (
	RoleBody       = "body"
	RoleSubtitle   = "subtitle"
	RoleHeading    = "heading"
	RoleSceneBreak = "sceneBreak"
)

// Chapter is a parsed+segmented chapter: paragraphs, each a list of sentences.
// ParagraphStyles is a parallel array of per-paragraph roles (same length as
// Paragraphs); an entry of "" or RoleBody means an ordinary body paragraph.
type Chapter struct {
	Title           string
	Paragraphs      [][]*Sentence
	ParagraphStyles []string
}

// Manifest mirrors manifest.json.
type Manifest struct {
	FormatVersion int          `json:"formatVersion"`
	Title         string       `json:"title"`
	Author        string       `json:"author"`
	SourceLang    string       `json:"sourceLang"`
	TargetLangs   []string     `json:"targetLangs"`
	Cover         *string      `json:"cover"`
	Chapters      []ChapterRef `json:"chapters"`
}

// ChapterRef is one entry in the manifest chapter index.
type ChapterRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	File  string `json:"file"`
}
