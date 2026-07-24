package translate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/tbook"
)

// GlossEntry is one enforced term translation.
type GlossEntry struct {
	Src string `json:"src"`
	Tgt string `json:"tgt"`
}

// glossarySampleMax caps how many sentences are sent to the model when
// building the glossary (spread evenly across the book).
const glossarySampleMax = 200

// BuildGlossary asks the model for a book-wide glossary — recurring key terms
// and proper nouns whose translation must stay consistent across chapters —
// and caches it on disk. Returns the entries plus a short hash that namespaces
// the per-sentence translation cache while the glossary is enforced (a changed
// glossary must not reuse translations made under a different one).
func BuildGlossary(ctx context.Context, c *Client, cacheDir string, sentences []*tbook.Sentence,
	source, target, title, author string) ([]GlossEntry, string, error) {

	bookKey := fmt.Sprintf("%s|%s|%d", title, author, len(sentences))
	sum := sha256.Sum256([]byte(cache.PromptVersion + "|glossary|" + c.Model() + "|" + source + "|" + target + "|" + bookKey))
	cachePath := filepath.Join(cacheDir, "glossary-"+hex.EncodeToString(sum[:])+".json")

	var entries []GlossEntry
	if b, err := os.ReadFile(cachePath); err == nil && json.Unmarshal(b, &entries) == nil {
		return entries, GlossHash(entries), nil
	}

	step := max(1, len(sentences)/glossarySampleMax)
	var sample []string
	for i := 0; i < len(sentences); i += step {
		sample = append(sample, sentences[i].Src)
		if len(sample) >= glossarySampleMax {
			break
		}
	}
	userJSON, err := jsonx.Marshal(map[string]any{
		"title": title, "author": author, "sentences": sample,
	})
	if err != nil {
		return nil, "", err
	}

	var out struct {
		Glossary []GlossEntry `json:"glossary"`
	}
	sys := glossarySystemPrompt(LangName(source), LangName(target))
	if err := c.ChatJSON(WithPhase(ctx, "glossary"), sys, string(userJSON), &out); err != nil {
		return nil, "", err
	}
	// Keep only usable entries, deterministic order.
	for _, e := range out.Glossary {
		e.Src, e.Tgt = strings.TrimSpace(e.Src), strings.TrimSpace(e.Tgt)
		if e.Src != "" && e.Tgt != "" {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Src < entries[j].Src })

	if err := os.MkdirAll(cacheDir, 0o755); err == nil {
		if b, err := jsonx.Marshal(entries); err == nil {
			_ = os.WriteFile(cachePath, b, 0o644)
		}
	}
	return entries, GlossHash(entries), nil
}

// LoadGlossaryFile reads a user-editable glossary JSON file — the format
// WriteGlossaryFile produces: an array of {"src","tgt"} entries — trimming
// and dropping any entry left with an empty src or tgt (e.g. a term the user
// blanked out to stop enforcing it). Returns an error if path doesn't exist
// or isn't valid JSON of that shape.
func LoadGlossaryFile(path string) ([]GlossEntry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []GlossEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := entries[:0]
	for _, e := range entries {
		e.Src, e.Tgt = strings.TrimSpace(e.Src), strings.TrimSpace(e.Tgt)
		if e.Src != "" && e.Tgt != "" {
			out = append(out, e)
		}
	}
	return out, nil
}

// WriteGlossaryFile writes entries as pretty-printed JSON to path so a user
// can open and edit them (see --only-glossary) before translation runs.
func WriteGlossaryFile(path string, entries []GlossEntry) error {
	if entries == nil {
		entries = []GlossEntry{}
	}
	b, err := jsonx.MarshalIndent(entries, "  ")
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
// translation-cache namespace component.
func GlossHash(entries []GlossEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e.Src)
		sb.WriteByte('=')
		sb.WriteString(e.Tgt)
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
translation. At most 40 entries.

Reply with ONLY a JSON object: {"glossary":[{"src":"<{SRC} term>","tgt":"<{TGT} translation>"}, …]}.
No code fences, no commentary.`)
}
