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

// Translation is one target-language rendering of a sentence. Q is the
// optional, informative alignment-coverage score in [0,1] (fraction of rendered
// translation words whose chunk highlights ≥1 source word); consumers ignore it.
type Translation struct {
	Text  string       `json:"text"`
	Align []AlignChunk `json:"align"`
	Q     float64      `json:"q,omitempty"`
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

// NoteRef is an inline footnote/endnote marker inside a sentence: the reader
// renders a superscript Label at rune offset P of Src (an insertion point,
// 0 ≤ P ≤ len(Src)); tapping it opens the book-level note with the given ID.
// The marker text is NOT part of Src — a consumer ignoring Notes renders clean
// prose with no stray digits.
type NoteRef struct {
	P     int    `json:"p"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Sentence is the atomic translatable unit. Words are [start,end) rune offsets
// of each tappable source word in Src. Tr maps a language code to its rendering.
// Spans mark inline italic/bold runs in Src; omitted when there are none.
// Notes are inline footnote markers; omitted when there are none.
type Sentence struct {
	Src   string                 `json:"src"`
	Words [][2]int               `json:"words"`
	Tr    map[string]Translation `json:"tr"`
	Spans []Span                 `json:"spans,omitempty"`
	Notes []NoteRef              `json:"notes,omitempty"`
}

// Paragraph roles. Body is the implicit default and is never emitted.
const (
	RoleBody       = "body"
	RoleSubtitle   = "subtitle"
	RoleHeading    = "heading"
	RoleSceneBreak = "sceneBreak"
	RoleFigure     = "figure" // paragraph = the figure's caption; see Chapter.Figures
	RoleTable      = "table"  // paragraph is empty; see Chapter.Tables
)

// Figure attaches an image to a chapter paragraph. Para indexes into
// Chapter.Paragraphs; that paragraph carries the caption sentences (possibly
// none) and has role RoleFigure. Image is the archive entry name
// (e.g. "images/img3.jpg"); Alt is the source alt text, if any.
type Figure struct {
	Para  int    `json:"para"`
	Image string `json:"image"`
	Alt   string `json:"alt,omitempty"`
}

// TableCell is one cell of an extracted table: a mini-paragraph of sentences,
// optionally a header cell.
type TableCell struct {
	Sentences []*Sentence `json:"sentences"`
	Header    bool        `json:"header,omitempty"`
}

// Table attaches a table to a chapter paragraph. Para indexes into
// Chapter.Paragraphs; that paragraph is empty with role RoleTable. Rows are in
// source order.
type Table struct {
	Para int           `json:"para"`
	Rows [][]TableCell `json:"rows"`
}

// Note is one footnote/endnote body: paragraphs of ordinary sentences that flow
// through the same translate/align pipeline as body prose. Kind is "note"
// (default, content note) or "citation" (bibliographic reference — a producer
// may leave citations untranslated).
type Note struct {
	Label      string        `json:"label"`
	Kind       string        `json:"kind,omitempty"`
	Paragraphs [][]*Sentence `json:"paragraphs"`
}

// Note kinds.
const (
	NoteKindNote     = "note"
	NoteKindCitation = "citation"
)

// Chapter is a parsed+segmented chapter: paragraphs, each a list of sentences.
// ParagraphStyles is a parallel array of per-paragraph roles (same length as
// Paragraphs); an entry of "" or RoleBody means an ordinary body paragraph.
// Figures and Tables reference paragraphs by index.
type Chapter struct {
	Title           string
	Paragraphs      [][]*Sentence
	ParagraphStyles []string
	Figures         []Figure
	Tables          []Table
}

// Book is the fully assembled document handed to Write: metadata plus chapters,
// book-level notes (id → note) and image bytes (archive entry name → bytes).
type Book struct {
	Title   string
	Author  string
	Source  string
	Targets []string
	Cover   []byte            // nil if none
	Images  map[string][]byte // "images/imgN.ext" → bytes; nil/empty if none
	Notes   map[string]*Note  // note id → body; nil/empty if none
	Chapters []Chapter
}

// Manifest mirrors manifest.json.
type Manifest struct {
	FormatVersion int          `json:"formatVersion"`
	Title         string       `json:"title"`
	Author        string       `json:"author"`
	SourceLang    string       `json:"sourceLang"`
	TargetLangs   []string     `json:"targetLangs"`
	Cover         *string      `json:"cover"`
	Notes         *string      `json:"notes,omitempty"`
	Chapters      []ChapterRef `json:"chapters"`
}

// ChapterRef is one entry in the manifest chapter index.
type ChapterRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	File  string `json:"file"`
}
