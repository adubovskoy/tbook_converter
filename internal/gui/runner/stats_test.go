package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSumStatsCostMissingFile(t *testing.T) {
	if got := SumStatsCost(filepath.Join(t.TempDir(), "nope.jsonl")); got != 0 {
		t.Errorf("missing file: got %v, want 0", got)
	}
}

func TestSumStatsCost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	data := `{"cost":0.5,"tokens":100}
not json at all

{"cost":0.25}
{"no_cost_field":true}
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got := SumStatsCost(path)
	if got < 0.7499 || got > 0.7501 {
		t.Errorf("got %v, want 0.75", got)
	}
}
