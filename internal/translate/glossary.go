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
	"sync"

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
	// Suspect holds the near spelling the target lexicon does know, for an
	// entry whose own rendering it does not — the `escritoire → эскртуар`
	// class. The entry is still enforced (consistency is the point, and the
	// lexicon is not the authority on a book's terms) but it loses the
	// proofreader's "never change this" protection, so the repair pass may fix
	// the word instead of being told to keep it. Nothing rewrites it silently.
	Suspect string `json:"suspect,omitempty"`
}

// glossarySampleMax caps how many sentences are sent to the model when
// building the glossary (spread evenly across the book). Deliberately NOT
// raised: measured on two books, 600 and 1200 sampled sentences return the same
// 20-34 entries as 200 and cost 3-6x the build prompt — the sample pass answers
// "name the key terms" with a short curated list whatever it is shown, which is
// why complete coverage comes from local mining instead (glossmine.go).
const glossarySampleMax = 200

// glossarySampleCap is how many entries the sample pass may return in TOTAL,
// glossarySampleChunks how many calls it is split over, and
// glossaryChunkCap how many entries one call may return.
// defaultGlossaryMax is how many the merged glossary keeps.
//
// The split is not about prompt size, it is about the entry cap being the only
// number this pass's model actually obeys — and only while it is small.
// Measured on deepseek-ai/DeepSeek-V4-Flash-0731 over Sherlock ch.1-2
// (bench-quality/reports/b1-langgate-2026-09-19.md §9):
//
//	200 sentences, cap 120 → ran past the cap to 250+ entries and was CUT
//	                          (finish_reason=length), degenerating into ordinary
//	                          vocabulary — "police", "kingdom", "monarch"
//	100 sentences, cap  30 → 30 entries, 1.1 kB, 4 s
//	 50 sentences, cap  30 → 30 entries, 1.5 kB, 3 s
//	 50 sentences, cap  40 → 20 entries (it ran out of terms first), 0.8 kB, 2 s
//
// So four calls of 50 sentences at 30 entries each cover the same sample, stay
// inside the budget with room to spare, and can still return the full 120 —
// against 0 for the single call that overruns. 300 for the merged glossary is
// unchanged: 1.5x the measured demand of a 15k-sentence novel (~200 terms) and
// far below anything that costs adherence (244 and 1392 entries both score
// 98.6%). What a big glossary does cost is ~9 prompt tokens per entry on every
// batch of every pass that carries it.
const (
	glossarySampleCap    = 120
	glossarySampleChunks = 4
	glossaryChunkCap     = glossarySampleCap / glossarySampleChunks
	defaultGlossaryMax   = 300
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

	// Neither pass is a dependency of the other, and neither is worth a whole
	// book: the glossary is an improvement over no glossary. The sample pass
	// asks for up to glossarySampleCap entries in one reply, which is the
	// largest single output the pipeline ever requests — on
	// deepseek-ai/DeepSeek-V4-Flash-0731 it comes back finish_reason=length,
	// and because the gonka gateway answers an identical request from cache
	// every retry returns the same truncated reply in ~90 ms. Aborting there
	// would make the shipped gonka default unable to convert any book at all.
	sample, sampleErr := buildSampleGlossary(ctx, c, sentences, o)
	if sampleErr != nil {
		fmt.Printf("Glossary [%s]: sampled pass failed (%v) — mined terms only\n",
			o.Target, sampleErr)
	}
	if sampleErr == nil {
		fmt.Printf("Glossary [%s]: sampled %d terms over %d calls\n",
			o.Target, len(sample), min(glossarySampleChunks, len(sentences)))
	}
	mined, minedErr := buildMinedGlossary(ctx, c, sentences, o)
	if minedErr != nil {
		fmt.Printf("Glossary [%s]: mined pass failed (%v) — using the sampled terms only\n",
			o.Target, minedErr)
	}
	if sampleErr != nil && minedErr != nil {
		return nil, "", fmt.Errorf("both glossary passes failed: sample: %w; mined: %v", sampleErr, minedErr)
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

// evenSample spreads up to glossarySampleMax source sentences over the book.
func evenSample(sentences []*tbook.Sentence) []string {
	step := max(1, len(sentences)/glossarySampleMax)
	sample := make([]string, 0, glossarySampleMax)
	for i := 0; i < len(sentences); i += step {
		sample = append(sample, sentences[i].Src)
		if len(sample) >= glossarySampleMax {
			break
		}
	}
	return sample
}

// sampleChunks splits the even sample into glossarySampleChunks slices, each
// still spread across the whole book: chunk k takes every Nth sentence starting
// at k, so one call never sees only chapter one. A chunk that would be empty is
// not sent.
func sampleChunks(sample []string) [][]string {
	n := min(glossarySampleChunks, len(sample))
	if n <= 1 {
		if len(sample) == 0 {
			return nil
		}
		return [][]string{sample}
	}
	out := make([][]string, n)
	for i, s := range sample {
		out[i%n] = append(out[i%n], s)
	}
	return out
}

// buildSampleGlossary asks the model for the key terms of an even sample of the
// book — the one thing that finds ordinary words used in a book-specific sense
// ("stack", "hull"), which mining cannot.
//
// It is SPLIT over glossarySampleChunks calls, merged and de-duplicated by head
// term. One call over the whole sample asks for so many entries that this
// pass's model runs past the cap and is cut mid-reply; see glossaryChunkCap for
// the measurement. Chunks run concurrently and tolerate each other's failures:
// a partial sample is worth far more than none. Returns an error only when
// EVERY chunk failed, and never returns the content of a truncated reply.
func buildSampleGlossary(ctx context.Context, c *Client, sentences []*tbook.Sentence,
	o GlossaryBuild) ([]GlossEntry, error) {

	chunks := sampleChunks(evenSample(sentences))
	if len(chunks) == 0 {
		return nil, nil
	}
	sys := glossarySystemPrompt(LangName(o.Source), LangName(o.Target), glossaryChunkCap)

	var (
		mu      sync.Mutex
		perCall = make([][]GlossEntry, len(chunks))
		failed  int
		firstEr error
		wg      sync.WaitGroup
	)
	for i, chunk := range chunks {
		wg.Add(1)
		go func(i int, chunk []string) {
			defer wg.Done()
			entries, err := sampleGlossaryCall(ctx, c, sys, chunk, o)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				if firstEr == nil {
					firstEr = err
				}
				return
			}
			perCall[i] = entries
		}(i, chunk)
	}
	wg.Wait()

	if failed == len(chunks) {
		return nil, fmt.Errorf("all %d sample calls failed: %w", len(chunks), firstEr)
	}
	if failed > 0 {
		fmt.Printf("Glossary [%s]: %d of %d sample calls failed (%v) — keeping the rest\n",
			o.Target, failed, len(chunks), firstEr)
	}

	// Merge in call order so the result does not depend on which goroutine won.
	seen := make(map[string]bool, glossarySampleCap)
	merged := make([]GlossEntry, 0, glossarySampleCap)
	for _, entries := range perCall {
		for _, e := range entries {
			k := strings.ToLower(e.Src)
			if seen[k] {
				continue
			}
			seen[k] = true
			merged = append(merged, e)
			if len(merged) >= glossarySampleCap {
				return merged, nil
			}
		}
	}
	return merged, nil
}

// sampleGlossaryCall is one chunk's request. A truncated reply never reaches
// here: the client turns finish_reason=length into an error before parsing, so
// a cut-off list is a retried failure and never a silently accepted partial.
func sampleGlossaryCall(ctx context.Context, c *Client, sys string, sample []string,
	o GlossaryBuild) ([]GlossEntry, error) {

	userJSON, err := jsonx.Marshal(map[string]any{
		"title": o.Title, "author": o.Author, "sentences": sample,
	})
	if err != nil {
		return nil, err
	}
	var out struct {
		Glossary []GlossEntry `json:"glossary"`
	}
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
	if len(entries) > glossaryChunkCap {
		entries = entries[:glossaryChunkCap]
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
	var drops []GlossDrop
	add := func(list []GlossEntry) {
		for _, e := range list {
			k := strings.ToLower(e.Src)
			if seen[k] {
				continue
			}
			if why := rejectGlossEntry(e, o.Source, o.Target); why != "" {
				drops = append(drops, GlossDrop{Src: e.Src, Tgt: e.Tgt, Why: why})
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
	ReportGlossDrops(o.Target, drops)
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
	// Suspect repeats the entries whose rendering the target lexicon does not
	// know while it knows a near spelling, so a human sees them without reading
	// every term. Regenerated on every write from Terms; editing it changes
	// nothing — fix the term instead.
	Suspect []GlossEntry `json:"suspect,omitempty"`
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
	b, err := jsonx.MarshalIndent(glossaryFile{GlossaryScope: scope, Terms: entries,
		Suspect: Suspects(entries)}, "  ")
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

func glossarySystemPrompt(sourceName, targetName string, entryCap int) string {
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
At most ` + strconv.Itoa(entryCap) + ` entries.

Reply with ONLY a JSON object: {"glossary":[{"src":"<{SRC} term>","tgt":"<{TGT} translation>"}, …]}.
No code fences, no commentary.`)
}
