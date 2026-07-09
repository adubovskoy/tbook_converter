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
