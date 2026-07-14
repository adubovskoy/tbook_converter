package embalign

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChunks(t *testing.T) {
	// tr: "Утром июня" — words at rune offsets [0,5) and [6,10).
	trWords := [][2]int{{0, 5}, {6, 10}}
	pairs := [][2]int{{2, 0}, {4, 1}, {2, 0}, {7, 1}} // dup pair + two srcs for word 1
	chunks, q := Chunks(pairs, trWords)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want 2", len(chunks))
	}
	if chunks[0].T != [2]int{0, 5} || len(chunks[0].W) != 1 || chunks[0].W[0] != 2 {
		t.Errorf("chunk 0 = %+v, want T=[0,5) W=[2]", chunks[0])
	}
	if chunks[1].T != [2]int{6, 10} || len(chunks[1].W) != 2 || chunks[1].W[0] != 4 || chunks[1].W[1] != 7 {
		t.Errorf("chunk 1 = %+v, want T=[6,10) W=[4 7]", chunks[1])
	}
	if q != 1.0 {
		t.Errorf("q = %v, want 1.0", q)
	}
}

func TestChunksPartialAndInvalid(t *testing.T) {
	trWords := [][2]int{{0, 3}, {4, 8}, {9, 12}}
	pairs := [][2]int{{0, 1}, {1, 99}, {-1, 0}, {5, -2}} // only {0,1} is valid
	chunks, q := Chunks(pairs, trWords)
	if len(chunks) != 1 || chunks[0].T != [2]int{4, 8} {
		t.Fatalf("chunks = %+v, want one chunk at [4,8)", chunks)
	}
	if want := 1.0 / 3.0; q != want {
		t.Errorf("q = %v, want %v", q, want)
	}
}

func TestChunksEmpty(t *testing.T) {
	chunks, q := Chunks(nil, nil)
	if len(chunks) != 0 || q != 0 {
		t.Errorf("empty input: chunks=%v q=%v", chunks, q)
	}
}

func TestContentQ(t *testing.T) {
	// "Yo estaba nervioso": estaba is a stopword; only Yo is aligned.
	text := "Yo estaba nervioso"
	trWords := [][2]int{{0, 2}, {3, 9}, {10, 18}}
	chunks, _ := Chunks([][2]int{{0, 0}}, trWords)
	stop := map[string]bool{"estaba": true}

	// Content words: Yo, nervioso — 1 of 2 covered.
	if q := ContentQ(chunks, trWords, text, stop); q != 0.5 {
		t.Errorf("content q = %v, want 0.5", q)
	}
	// Nil stop set falls back to all-word coverage: 1 of 3.
	if q := ContentQ(chunks, trWords, text, nil); q != 1.0/3.0 {
		t.Errorf("all-word q = %v, want 1/3", q)
	}
	// All words stopped: falls back to all-word coverage too.
	all := map[string]bool{"yo": true, "estaba": true, "nervioso": true}
	if q := ContentQ(chunks, trWords, text, all); q != 1.0/3.0 {
		t.Errorf("all-stop q = %v, want 1/3", q)
	}
}

func TestWordStrings(t *testing.T) {
	text := "Утром июня"
	got := WordStrings(text, [][2]int{{0, 5}, {6, 10}, {8, 99}})
	if got[0] != "Утром" || got[1] != "июня" || got[2] != "" {
		t.Errorf("WordStrings = %q", got)
	}
}

// TestAlignerProtocol drives Start/Align/Close against a stub "aligner" shell
// script that speaks the JSONL protocol (no Python or model needed).
func TestAlignerProtocol(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "stub.py")
	stub := `#!/bin/sh
echo '{"ready": true}'
while read -r line; do
  echo '{"pairs": [[0, 0], [1, 1]]}'
done
`
	if err := os.WriteFile(script, []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	a, err := Start(Options{Python: "/bin/sh", Script: script})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	pairs, err := a.Align([]string{"The", "cat"}, []string{"Кот", "тут"})
	if err != nil {
		t.Fatalf("Align: %v", err)
	}
	if len(pairs) != 2 || pairs[0] != [2]int{0, 0} || pairs[1] != [2]int{1, 1} {
		t.Errorf("pairs = %v", pairs)
	}
	if err := a.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestAlignerBatchProtocol drives AlignBatch against a stub that answers the
// batch shape, including a per-item error (→ nil result for that item).
func TestAlignerBatchProtocol(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "stub.py")
	stub := `#!/bin/sh
echo '{"ready": true}'
while read -r line; do
  echo '{"results": [{"pairs": [[0, 0]]}, {"error": "boom"}, {"pairs": []}]}'
done
`
	if err := os.WriteFile(script, []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	a, err := Start(Options{Python: "/bin/sh", Script: script})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer a.Close()
	srcs := [][]string{{"a"}, {"b"}, {"c"}}
	tgts := [][]string{{"x"}, {"y"}, {"z"}}
	res, err := a.AlignBatch(srcs, tgts)
	if err != nil {
		t.Fatalf("AlignBatch: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("results = %d", len(res))
	}
	if len(res[0]) != 1 || res[0][0] != [2]int{0, 0} {
		t.Errorf("res[0] = %v", res[0])
	}
	if res[1] != nil {
		t.Errorf("res[1] = %v, want nil (per-item error)", res[1])
	}
	if res[2] == nil || len(res[2]) != 0 {
		t.Errorf("res[2] = %v, want empty non-nil", res[2])
	}
	// Mismatched result count is a request-level error.
	if _, err := a.AlignBatch(srcs[:2], tgts[:2]); err == nil {
		t.Error("AlignBatch accepted mismatched result count")
	}
}

func TestAlignerBadHandshake(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "stub.py")
	stub := "#!/bin/sh\necho '{\"error\": \"no torch\"}'\n"
	if err := os.WriteFile(script, []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(Options{Python: "/bin/sh", Script: script}); err == nil {
		t.Fatal("Start succeeded on error handshake")
	}
}
