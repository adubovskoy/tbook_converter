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
		return entries, glossHash(entries), nil
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
	return entries, glossHash(entries), nil
}

// glossHash is a short stable digest of the glossary content, used as a
// translation-cache namespace component.
func glossHash(entries []GlossEntry) string {
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
