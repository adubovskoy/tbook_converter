package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveCreatesDirs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("sandboxing os.UserConfigDir/UserCacheDir relies on XDG env vars")
	}
	cfgRoot, cacheRoot := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgRoot)
	t.Setenv("XDG_CACHE_HOME", cacheRoot)

	d, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	for name, dir := range map[string]string{
		"config":   filepath.Dir(d.SettingsFile()),
		"cache":    d.Cache(),
		"runs":     d.Runs(),
		"work":     d.Work(),
		"venv":     d.VenvDir(),
		"tools":    d.ToolsDir(),
		"lexicons": d.LexiconsDir(),
		"bin":      d.BinDir(),
	} {
		fi, err := os.Stat(dir)
		if err != nil {
			t.Errorf("%s dir %s: %v", name, dir, err)
			continue
		}
		if !fi.IsDir() {
			t.Errorf("%s: %s is not a directory", name, dir)
		}
	}

	wantCfg := filepath.Join(cfgRoot, appDir)
	if got := filepath.Dir(d.SettingsFile()); got != wantCfg {
		t.Errorf("SettingsFile parent = %s, want %s", got, wantCfg)
	}
	if got := filepath.Dir(d.JobsFile()); got != wantCfg {
		t.Errorf("JobsFile parent = %s, want %s", got, wantCfg)
	}
	wantCache := filepath.Join(cacheRoot, appDir)
	if got := filepath.Dir(d.Cache()); got != wantCache {
		t.Errorf("Cache parent = %s, want %s", got, wantCache)
	}
}

func TestVenvPython(t *testing.T) {
	venv := filepath.Join("some", "venv")
	want := filepath.Join(venv, "bin", "python")
	if runtime.GOOS == "windows" {
		want = filepath.Join(venv, "Scripts", "python.exe")
	}
	if got := VenvPython(venv); got != want {
		t.Errorf("VenvPython = %s, want %s", got, want)
	}
}

func TestDefaultOutputDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.UserHomeDir is not driven by HOME on windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := DefaultOutputDir()
	want := filepath.Join(home, "Documents", "tbook books")
	if got != want {
		t.Errorf("DefaultOutputDir = %s, want %s", got, want)
	}
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Errorf("DefaultOutputDir must not create the directory (stat err = %v)", err)
	}
}
