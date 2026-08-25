package translate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/tbook"
)

// GlossEntry is one enforced term translation. Gender and Kind are optional
// annotations produced by the render pass (see glossrender.go); an entry
// without them behaves exactly as entries did before they existed, and a
// hand-edited sidecar may set or clear them freely.
type GlossEntry struct {
	Src string `json:"src"`
	Tgt string `json:"tgt"`
	// Gender is "m" or "f" for a NAMED INDIVIDUAL whose gender the source does
	// not mark, so the target can agree with it. Empty means "unknown, do not
	// state" — the safe default: a wrong gender is worse than none, because it
	// forces the wrong agreement on every sentence about that character.
	Gender string `json:"gender,omitempty"`
	// Kind is "person" | "place" | "org" | "thing"; only "person" may carry a
	// gender.
	Kind string `json:"kind,omitempty"`
}

// glossarySampleMax caps how many sentences are sent to the model when
// building the glossary (spread evenly across the book). Deliberately NOT
// raised: measured on two books, 600 and 1200 sampled sentences return the same
// 20-34 entries as 200 and cost 3-6x the build prompt — the sample pass answers
// "name the key terms" with a short curated list whatever it is shown, which is
// why complete coverage comes from local mining instead (glossmine.go).
const glossarySampleMax = 200

// glossarySampleCap is how many entries the sample pass may return, and
// defaultGlossaryMax how many the merged glossary keeps. 300 is 1.5x the
// measured demand of a 15k-sentence novel (~200 terms) and far below anything
// that costs adherence: 244 and 1392 entries both score 98.6%, and 1148
// irrelevant entries around the real ones change nothing. What it does cost is
// ~9 prompt tokens per entry on every batch of every pass that carries the
// glossary.
const (
	glossarySampleCap  = 120
	defaultGlossaryMax = 300
)

// GlossaryBuild carries everything BuildGlossary needs beyond the sentences.
type GlossaryBuild struct {
	CacheDir string
	Source   string
	Target   string
	Title    string
	Author   string

	// Lexicon, when non-nil, enables coined-term mining: a frequent lowercase
	// word the bilingual lexicon does not know is invented terminology
	// (neurachem, needlecast). Without it only names are mined.
	Lexicon Lexicon

	// Gender annotates named individuals so the target can agree with them.
	// Ignored for targets that do not mark gender (GenderMarkingTarget).
	Gender bool

	// Max caps the merged glossary; 0 means defaultGlossaryMax.
	Max int
}

// Lexicon is the part of lexcheck.Dict the coined-term detector needs: does the
// bilingual dictionary know this source word at all?
type Lexicon interface {
	Covered(srcWord string) bool
}

// BuildGlossary produces the book-wide glossary — recurring key terms, proper
// nouns and invented terminology whose translation must stay consistent across
// chapters — and caches it on disk. Returns the entries plus a short hash that
// namespaces the per-sentence translation cache while the glossary is enforced
// (a changed glossary must not reuse translations made under a different one).
//
// Two sources, because measurement showed neither is sufficient:
//
//   - the SAMPLE pass asks the model for the key terms of a 200-sentence sample.
//     It is the only thing that finds ordinary words used in a book-specific
//     sense ("stack", "hull"), and it cannot enumerate: on two books it returned
//     18-34 entries whatever cap it was given, missing characters with 147 and
//     even 1031 occurrences.
//   - local frequency MINING enumerates names and coined terms exhaustively and
//     for free; one render call then filters and translates the candidates
//     ($0.01/book) and annotates gender.
//
// Full numbers: bench-quality/reports/glossary-scale-and-gender.md.
func BuildGlossary(ctx context.Context, c *Client, sentences []*tbook.Sentence,
	o GlossaryBuild) ([]GlossEntry, string, error) {

	bookKey := fmt.Sprintf("%s|%s|%d", o.Title, o.Author, len(sentences))
	key := cache.GlossPromptVersion + "|glossary|" + c.Model() + "|" + o.Source + "|" +
		o.Target + "|" + bookKey + "|" + strconv.FormatBool(o.wantGender())
	sum := sha256.Sum256([]byte(key))
	cachePath := filepath.Join(o.CacheDir, "glossary-"+hex.EncodeToString(sum[:])+".json")

	var entries []GlossEntry
	if b, err := os.ReadFile(cachePath); err == nil && json.Unmarshal(b, &entries) == nil {
		return entries, GlossHash(entries), nil
	}

	sample, sampleErr := buildSampleGlossary(ctx, c, sentences, o)
	if sampleErr != nil {
		return nil, "", sampleErr
	}
	mined, minedErr := buildMinedGlossary(ctx, c, sentences, o)
	if minedErr != nil {
		// Mining is an improvement, not a dependency: a failed render call must
		// not cost the book its glossary, so fall back to the sample pass.
		fmt.Printf("Glossary [%s]: mined pass failed (%v) — using the sampled terms only\n",
			o.Target, minedErr)
	}
	entries = mergeGlossaries(sample, mined, o)

	if err := os.MkdirAll(o.CacheDir, 0o755); err == nil {
		if b, err := jsonx.Marshal(entries); err == nil {
			_ = os.WriteFile(cachePath, b, 0o644)
		}
	}
	return entries, GlossHash(entries), nil
}

// wantGender reports whether this build annotates gender: asked for, and the
// target actually marks it.
func (o GlossaryBuild) wantGender() bool {
	return o.Gender && GenderMarkingTarget(o.Target)
}

func (o GlossaryBuild) max() int {
	if o.Max > 0 {
		return o.Max
	}
	return defaultGlossaryMax
}

// buildSampleGlossary is the original pass: one call over an even sample of the
// book. Its unique contribution is ordinary words used in a book-specific sense.
func buildSampleGlossary(ctx context.Context, c *Client, sentences []*tbook.Sentence,
	o GlossaryBuild) ([]GlossEntry, error) {

	step := max(1, len(sentences)/glossarySampleMax)
	var sample []string
	for i := 0; i < len(sentences); i += step {
		sample = append(sample, sentences[i].Src)
		if len(sample) >= glossarySampleMax {
			break
		}
	}
	userJSON, err := jsonx.Marshal(map[string]any{
		"title": o.Title, "author": o.Author, "sentences": sample,
	})
	if err != nil {
		return nil, err
	}
	var out struct {
		Glossary []GlossEntry `json:"glossary"`
	}
	sys := glossarySystemPrompt(LangName(o.Source), LangName(o.Target))
	if err := c.ChatJSON(WithPhase(ctx, "glossary"), sys, string(userJSON), &out); err != nil {
		return nil, err
	}
	entries := make([]GlossEntry, 0, len(out.Glossary))
	for _, e := range out.Glossary {
		e.Src, e.Tgt = strings.TrimSpace(e.Src), strings.TrimSpace(e.Tgt)
		if e.Src != "" && e.Tgt != "" {
			entries = append(entries, e)
		}
	}
	if len(entries) > glossarySampleCap {
		entries = entries[:glossarySampleCap]
	}
	return entries, nil
}

// mergeGlossaries combines the two sources, applies the entry gates and the
// cap, and returns a deterministically ordered glossary. Mined entries win ties
// because they carry frequency, kind and gender; a sample entry whose head word
// a mined entry already covers is dropped (phrase entries are followed only 62%
// of the time against 95% for head terms, and they crowd out real ones).
func mergeGlossaries(sample, mined []GlossEntry, o GlossaryBuild) []GlossEntry {
	seen := make(map[string]bool, len(sample)+len(mined))
	kept := make([]GlossEntry, 0, len(sample)+len(mined))
	add := func(list []GlossEntry) {
		for _, e := range list {
			k := strings.ToLower(e.Src)
			if seen[k] || !acceptGlossEntry(e, o.Source, o.Target) {
				continue
			}
			seen[k] = true
			if !o.wantGender() {
				e.Gender = ""
			}
			kept = append(kept, e)
		}
	}
	add(mined)
	add(sample)
	// Deterministic, and short entries first so the cap keeps head terms over
	// the phrases that survived.
	sort.SliceStable(kept, func(i, j int) bool {
		wi, wj := len(strings.Fields(kept[i].Src)), len(strings.Fields(kept[j].Src))
		if wi != wj {
			return wi < wj
		}
		return kept[i].Src < kept[j].Src
	})
	if len(kept) > o.max() {
		kept = kept[:o.max()]
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].Src < kept[j].Src })
	return kept
}

// GlossaryScope identifies the book and language pair a glossary was built
// for. It mirrors BuildGlossary's cache key (minus the model, which does not
// change what a term *means*): a glossary is only reusable for the same book
// translated into the same language.
type GlossaryScope struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Sentences int    `json:"sentences"`
}

// glossaryFile is the on-disk shape of the user-editable sidecar. The scope
// fields are what make reuse safe — without them a file written for en→ru
// would be enforced verbatim on a later en→es run of the same output path
// (the add-a-language flow defaults --out to the input .tbook), and a
// glossary built under --limit-chapters would silently carry into the full
// book.
type glossaryFile struct {
	GlossaryScope
	Terms []GlossEntry `json:"terms"`
}

// ErrGlossaryScope reports a sidecar that belongs to a different book or
// language pair than the current run. Callers rebuild instead of reusing it.
var ErrGlossaryScope = errors.New("glossary file was built for a different book or language pair")

// GlossaryFilePath is the user-editable glossary sidecar for an output
// .tbook. The language pair is part of the name so adding a language to an
// existing book gets its own file instead of clobbering (or inheriting) the
// previous one.
func GlossaryFilePath(out string, scope GlossaryScope) string {
	return fmt.Sprintf("%s.glossary.%s-%s.json", out, scope.Source, scope.Target)
}

// LoadGlossaryFile reads a user-editable glossary sidecar — the format
// WriteGlossaryFile produces — trimming and dropping any entry left with an
// empty src or tgt (e.g. a term the user blanked out to stop enforcing it).
// Returns fs.ErrNotExist when there is no file yet, ErrGlossaryScope when the
// file belongs to another book or language pair, and a parse error otherwise;
// in every case the caller falls back to building the glossary.
func LoadGlossaryFile(path string, scope GlossaryScope) ([]GlossEntry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f glossaryFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if f.GlossaryScope != scope {
		return nil, fmt.Errorf("%w: it holds %s→%s for %q by %s, %d sentences", ErrGlossaryScope,
			f.Source, f.Target, f.Title, f.Author, f.Sentences)
	}
	out := make([]GlossEntry, 0, len(f.Terms))
	for _, e := range f.Terms {
		e.Src, e.Tgt = strings.TrimSpace(e.Src), strings.TrimSpace(e.Tgt)
		if e.Src != "" && e.Tgt != "" {
			out = append(out, e)
		}
	}
	return out, nil
}

// WriteGlossaryFile writes the glossary as pretty-printed JSON to path so a
// user can open and edit it (see --only-glossary) before translation runs.
func WriteGlossaryFile(path string, scope GlossaryScope, entries []GlossEntry) error {
	if entries == nil {
		entries = []GlossEntry{}
	}
	b, err := jsonx.MarshalIndent(glossaryFile{GlossaryScope: scope, Terms: entries}, "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, b, 0o644)
}

// GlossHash is a short stable digest of the glossary content, used as a
// translation-cache namespace component. Gender is part of the digest: it
// changes the prompt, so a translation made under a different gender annotation
// must not be reused. Kind is not — it never reaches the prompt.
func GlossHash(entries []GlossEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e.Src)
		sb.WriteByte('=')
		sb.WriteString(e.Tgt)
		if e.Gender != "" {
			sb.WriteByte('/')
			sb.WriteString(e.Gender)
		}
		sb.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:6])
}

func glossarySystemPrompt(sourceName, targetName string) string {
	r := strings.NewReplacer("{SRC}", sourceName, "{TGT}", targetName)
	return r.Replace(`You prepare a translation glossary for a book ({SRC} → {TGT}).

You receive JSON {title, author, sentences} — a sample of the book's sentences.

Return the KEY RECURRING TERMS whose {TGT} translation must stay consistent across
the whole book: domain terminology, recurring concepts, and proper nouns that get
transliterated or translated. Skip everyday words and terms with only one obvious
translation.

Most valuable here are ORDINARY {SRC} WORDS THE BOOK USES IN ITS OWN SENSE — a
common word that means something specific in this book (a "stack" that stores a
mind, a "sleeve" that is a body). Nothing else can find those.

Give the HEAD TERM, never a phrase built around it: "Suntouch", not "Suntouch House";
"stack", not "cortical stack storage". One entry per term, in its base form
(nominative singular). A phrase whose head word is already an entry is not an entry.
At most ` + strconv.Itoa(glossarySampleCap) + ` entries.

Reply with ONLY a JSON object: {"glossary":[{"src":"<{SRC} term>","tgt":"<{TGT} translation>"}, …]}.
No code fences, no commentary.`)
}
