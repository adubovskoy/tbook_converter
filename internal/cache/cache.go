// Package cache is the resumable on-disk translation cache. One JSON file per
// unique sentence/target, keyed by a SHA-256 of the same fields the Python tool
// used, so a run can be interrupted and resumed and adding a language only
// translates the new one.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/tbook"
)

// PromptVersion keys the ALIGN production contract; bump only if the alignment
// rules change (it invalidates cached aligned entries). v8 = explicit idiom /
// phrasal-verb rule: every word of a fixed expression maps to the {TGT} word(s)
// that render it ("piss off" → "отвали" claims both words), split particles
// included. v7 = raw-canonical text: the final translation text is the pass-1
// raw translation verbatim, and the align pass only locates echoed fragments
// inside it (whole-word matches only) — a sloppy echo can no longer strip
// punctuation or rewrite words. (v6 = same without word-boundary matching;
// v5 = numbered-echo alignment, text reconstructed from the echo; v4 = plain
// match-by-text.)
const PromptVersion = "v8"

// TrPromptVersion keys the TRANSLATE (pass 1) contract separately, so an
// align-only contract change re-aligns the book without re-translating it.
// Bump only if the translation prompt/rules change.
const TrPromptVersion = "v4"

// Key returns the cache key for one sentence's FINAL aligned translation.
func Key(src, source, target, model string) string {
	raw := PromptVersion + "|" + model + "|" + source + "|" + target + "|" + src
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// TrKey returns the cache key for a sentence's RAW translation text (pass 1),
// before pass 2 computes the alignment. A distinct namespace ("|tr|") from Key
// so a translated-but-not-yet-aligned sentence is never mistaken for a finished
// one.
func TrKey(src, source, target, model string) string {
	raw := TrPromptVersion + "|tr|" + model + "|" + source + "|" + target + "|" + src
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// Invalidate deletes the cached translation AND alignment for each source
// sentence (across all targets) — used by the verify/QA loop: a semantic check
// flags bad sentences, this clears them, and the next run redoes exactly those
// (e.g. with a stronger model). Returns the number of cache files removed.
func Invalidate(dir string, srcs, targets []string, source, model string) int {
	removed := 0
	for _, src := range srcs {
		for _, target := range targets {
			for _, key := range []string{Key(src, source, target, model), TrKey(src, source, target, model)} {
				if os.Remove(filepath.Join(dir, key+".json")) == nil {
					removed++
				}
			}
		}
	}
	return removed
}

// Remove deletes one cached entry (no-op if absent).
func Remove(dir, key string) {
	_ = os.Remove(filepath.Join(dir, key+".json"))
}

// Read returns the cached translation for a key, or ok=false if absent/corrupt.
func Read(dir, key string) (tbook.Translation, bool) {
	b, err := os.ReadFile(filepath.Join(dir, key+".json"))
	if err != nil {
		return tbook.Translation{}, false
	}
	var tr tbook.Translation
	if json.Unmarshal(b, &tr) != nil {
		return tbook.Translation{}, false
	}
	if tr.Align == nil {
		tr.Align = []tbook.AlignChunk{}
	}
	return tr, true
}

// Write stores a translation under key (UTF-8, no HTML escaping).
func Write(dir, key string, tr tbook.Translation) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if tr.Align == nil {
		tr.Align = []tbook.AlignChunk{}
	}
	b, err := jsonx.Marshal(tr)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, key+".json"), b, 0o644)
}
