// Package settings persists the GUI configuration as a single settings.json:
// atomic writes (tmp + rename) with 0600 permissions since it holds API keys,
// and forgiving reads — a missing or corrupt file yields defaults instead of
// blocking app startup.
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// ProviderSettings holds one provider's user-entered values. Empty fields mean
// "use the converter's provider default".
type ProviderSettings struct {
	APIKey  string `json:"apiKey,omitempty"`
	BaseURL string `json:"baseURL,omitempty"` // ollama/llamacpp only
	Model   string `json:"model,omitempty"`
}

// Settings is the persisted app configuration.
type Settings struct {
	Version         int                         `json:"version"`
	DefaultProvider string                      `json:"defaultProvider"`
	Providers       map[string]ProviderSettings `json:"providers"`
	ClaudeBin       string                      `json:"claudeBin,omitempty"`
	DefaultSource   string                      `json:"defaultSource"`
	DefaultTargets  []string                    `json:"defaultTargets"`
	OutputDir       string                      `json:"outputDir,omitempty"`
	CacheDir        string                      `json:"cacheDir,omitempty"`
	EmbalignPython  string                      `json:"embalignPython,omitempty"`
	EmbalignScript  string                      `json:"embalignScript,omitempty"`
	LexiconsDir     string                      `json:"lexiconsDir,omitempty"`
	Repair          *bool                       `json:"repair,omitempty"`        // nil = provider default
	RepairContext   int                         `json:"repairContext,omitempty"` // 0|2 only
	Judge           bool                        `json:"judge,omitempty"`
	SetupCompleted  bool                        `json:"setupCompleted"`
	UI              UISettings                  `json:"ui"`
}

// UISettings holds presentation preferences.
type UISettings struct {
	Theme string `json:"theme"` // system|light|dark
}

// Defaults returns the settings a fresh install starts with.
func Defaults() Settings {
	return Settings{
		Version:         1,
		DefaultProvider: "openrouter",
		Providers:       map[string]ProviderSettings{},
		DefaultSource:   "en",
		DefaultTargets:  []string{"ru"},
		UI:              UISettings{Theme: "system"},
	}
}

// Store reads and writes the settings file. Safe for concurrent use.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore returns a Store around the settings file at path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load reads the settings, merging the file over Defaults so fields absent
// from an older file keep their defaults. A missing file yields Defaults; a
// corrupt file is set aside as <path>.bak and Defaults are returned — a bad
// file must never block app startup.
func (s *Store) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Defaults(), err
	}
	cfg := Defaults()
	if err := json.Unmarshal(data, &cfg); err != nil {
		_ = os.Rename(s.path, s.path+".bak")
		return Defaults(), nil
	}
	sanitize(&cfg)
	return cfg, nil
}

// sanitize coerces loaded values back into their valid domain.
func sanitize(cfg *Settings) {
	if cfg.RepairContext != 2 {
		cfg.RepairContext = 0
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]ProviderSettings{}
	}
}

// Save writes the settings atomically (tmp + rename) with 0600 permissions,
// creating the parent directory if needed.
func (s *Store) Save(cfg Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
