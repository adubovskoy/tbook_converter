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

// PromptVersion keys the alignment-prompt contract; bump only if the alignment
// rules change (it invalidates cached entries). v3 = word-level by source TEXT.
const PromptVersion = "v3"

// Key returns the cache key for one sentence in one language.
func Key(src, source, target, model string) string {
	raw := PromptVersion + "|" + model + "|" + source + "|" + target + "|" + src
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
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
