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
const TrPromptVersion = "v5"

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

// RepairPromptVersion keys the PROOFREAD (pass 1.5) contract. Bump only if the
// repair prompt/rules change: it re-proofreads from the cached raw translations
// without re-translating them. r1 = the adopted recipe — grammar, calqued
// expressions, register and fidelity, with the book glossary injected and no
// context-free guessing of gender, referents or recurring terms.
const RepairPromptVersion = "r1"

// RepairKey returns the cache key for a sentence's PROOFREAD translation, the
// text the align pass consumes when repair is on. Its own namespace ("|rp|")
// keyed by the RAW model component on purpose: the raw translation is identical
// with and without repair, so toggling repair costs one proofread pass, never a
// re-translation. What must not be shared is the FINAL aligned entry — Key gets
// the repair marker from the caller's model string instead.
func RepairKey(src, source, target, model string) string {
	raw := RepairPromptVersion + "|rp|" + model + "|" + source + "|" + target + "|" + src
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// Invalidate deletes the cached translation AND alignment for each source
// sentence (across all targets) — used by the verify/QA loop: a semantic check
// flags bad sentences, this clears them, and the next run redoes exactly those
// (e.g. with a stronger model). Returns the number of cache files removed.
// The namespaces are the run's own: raw is where pass-1 text lives, final where
// the aligned entry lives (they differ when the run proofreads), and repair
// where the proofread text lives ("" when the run does not proofread). Passing
// the plain model id where the run used a glossary-scoped namespace deletes
// nothing — the keys simply do not exist.
func Invalidate(dir string, srcs, targets []string, source, raw, final, repair string) int {
	removed := 0
	for _, src := range srcs {
		for _, target := range targets {
			keys := []string{Key(src, source, target, final), TrKey(src, source, target, raw)}
			if final != raw {
				keys = append(keys, Key(src, source, target, raw))
			}
			if repair != "" {
				keys = append(keys, RepairKey(src, source, target, repair))
			}
			for _, key := range keys {
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
