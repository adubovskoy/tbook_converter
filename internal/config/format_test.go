package config

import "testing"

func TestFormatDefaultV2(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("FORMAT", "")
	cfg, err := Load([]string{"book.epub"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Format != FormatV2 {
		t.Errorf("Format = %q, want %q by default", cfg.Format, FormatV2)
	}
	v1, err := Load([]string{"book.epub", "--format", "v1"})
	if err != nil {
		t.Fatalf("Load --format v1: %v", err)
	}
	if v1.Format != FormatV1 {
		t.Errorf("--format v1: Format = %q", v1.Format)
	}
	if _, err := Load([]string{"book.epub", "--format", "v3"}); err == nil {
		t.Error("--format v3 accepted, want an error")
	}
}
