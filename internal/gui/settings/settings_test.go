package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	return NewStore(path), path
}

func TestRoundtrip(t *testing.T) {
	st, _ := newTestStore(t)

	on := true
	want := Defaults()
	want.DefaultProvider = "gonka"
	want.Providers["gonka"] = ProviderSettings{APIKey: "k-123", Model: "m"}
	want.DefaultTargets = []string{"de", "es"}
	want.Repair = &on
	want.RepairContext = 2
	want.Judge = true
	want.SetupCompleted = true
	want.UI.Theme = "dark"

	if err := st.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := st.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("roundtrip mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	st, _ := newTestStore(t)
	got, err := st.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, Defaults()) {
		t.Errorf("missing file: got %+v, want Defaults", got)
	}
	if got.Providers == nil {
		t.Error("Providers map must be non-nil")
	}
}

func TestLoadCorruptFile(t *testing.T) {
	st, path := newTestStore(t)
	garbage := []byte("{not json")
	if err := os.WriteFile(path, garbage, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := st.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, Defaults()) {
		t.Errorf("corrupt file: got %+v, want Defaults", got)
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("expected .bak with the corrupt content: %v", err)
	}
	if string(bak) != string(garbage) {
		t.Errorf(".bak content = %q, want %q", bak, garbage)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("corrupt file should have been moved away (stat err = %v)", err)
	}
}

func TestLoadMergesOverDefaults(t *testing.T) {
	st, path := newTestStore(t)
	if err := os.WriteFile(path, []byte(`{"version":1,"defaultSource":"fr"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.DefaultSource != "fr" {
		t.Errorf("DefaultSource = %q, want fr", got.DefaultSource)
	}
	if got.DefaultProvider != "openrouter" {
		t.Errorf("DefaultProvider = %q, want default openrouter", got.DefaultProvider)
	}
	if got.UI.Theme != "system" {
		t.Errorf("Theme = %q, want default system", got.UI.Theme)
	}
}

func TestSaveFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permissions")
	}
	st, path := newTestStore(t)
	if err := st.Save(Defaults()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %o, want 0600", perm)
	}
}

func TestSaveCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "settings.json")
	st := NewStore(path)
	if err := st.Save(Defaults()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestRepairContextSanitized(t *testing.T) {
	for in, want := range map[int]int{0: 0, 1: 0, 2: 2, 3: 0, -1: 0} {
		st, _ := newTestStore(t)
		cfg := Defaults()
		cfg.RepairContext = in
		if err := st.Save(cfg); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := st.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got.RepairContext != want {
			t.Errorf("RepairContext %d loaded as %d, want %d", in, got.RepairContext, want)
		}
	}
}

func TestNilProvidersSanitized(t *testing.T) {
	st, path := newTestStore(t)
	if err := os.WriteFile(path, []byte(`{"version":1,"providers":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Providers == nil {
		t.Error("Providers map must be non-nil after load")
	}
}
