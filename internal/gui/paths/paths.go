// Package paths resolves the per-OS directories the GUI keeps its data in:
// config + state (settings.json, jobs.json) under os.UserConfigDir(),
// regenerables (cache, runs, work, venv, tools, lexicons, bin) under
// os.UserCacheDir().
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// appDir is the per-OS subdirectory name under the user config/cache roots.
const appDir = "tbook-converter"

// ConfigDir returns the app's config directory (not created).
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDir), nil
}

// CacheDir returns the app's cache directory (not created).
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDir), nil
}

// Dirs holds the resolved config and cache roots. Obtain via Resolve, which
// also creates every directory the accessors point into.
type Dirs struct {
	config string
	cache  string
}

// Resolve locates the config and cache roots and creates all app directories.
func Resolve() (Dirs, error) {
	config, err := ConfigDir()
	if err != nil {
		return Dirs{}, err
	}
	cache, err := CacheDir()
	if err != nil {
		return Dirs{}, err
	}
	d := Dirs{config: config, cache: cache}
	for _, dir := range []string{
		d.config,
		d.Cache(), d.Runs(), d.Work(), d.VenvDir(),
		d.ToolsDir(), d.LexiconsDir(), d.BinDir(),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Dirs{}, err
		}
	}
	return d, nil
}

// SettingsFile is the settings.json path.
func (d Dirs) SettingsFile() string { return filepath.Join(d.config, "settings.json") }

// JobsFile is the jobs.json path.
func (d Dirs) JobsFile() string { return filepath.Join(d.config, "jobs.json") }

// Cache is the shared converter --cache-dir.
func (d Dirs) Cache() string { return filepath.Join(d.cache, "cache") }

// Runs holds per-attempt progress/stats/log files.
func (d Dirs) Runs() string { return filepath.Join(d.cache, "runs") }

// Work is the child-process CWD: app-owned, guaranteed free of .env files.
func (d Dirs) Work() string { return filepath.Join(d.cache, "work") }

// VenvDir is the embalign python venv location.
func (d Dirs) VenvDir() string { return filepath.Join(d.cache, "venv-embalign") }

// ToolsDir holds extracted embedded tools (embalign.py).
func (d Dirs) ToolsDir() string { return filepath.Join(d.cache, "tools") }

// LexiconsDir holds fetched lexcheck lexicons.
func (d Dirs) LexiconsDir() string { return filepath.Join(d.cache, "lexicons") }

// BinDir holds downloaded helper binaries (uv).
func (d Dirs) BinDir() string { return filepath.Join(d.cache, "bin") }

// VenvPython returns the python interpreter path inside a venv.
func VenvPython(venvDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvDir, "Scripts", "python.exe")
	}
	return filepath.Join(venvDir, "bin", "python")
}

// DefaultOutputDir is where converted books go by default. Not created here —
// the first save creates it.
func DefaultOutputDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "tbook books")
}
