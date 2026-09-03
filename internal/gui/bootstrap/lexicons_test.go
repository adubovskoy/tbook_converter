package bootstrap

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dicGz builds an OPUS-style .dic.gz: count \t freq \t src \t tgt \t p1 \t p2.
func dicGz(t *testing.T, lines []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	for _, l := range lines {
		if _, err := w.Write([]byte(l + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readLexicon(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	sc := bufio.NewScanner(gz)
	for sc.Scan() {
		s, targets, _ := strings.Cut(sc.Text(), "\t")
		out[s] = targets
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFetchLexiconsFiltersAndDumpsBothDirections(t *testing.T) {
	lines := []string{
		"10\t0.5\thouse\tдом\tx\ty",
		"2\t0.1\trare\tредкий\tx\ty",  // count < 3 → dropped
		"5\t0.2\tho2use\tдом\tx\ty",   // digit in word → dropped
		"7\t0.3\thouse\tздание\tx\ty", // second target for house
		"9\t0.4\tHOME\tдом\tx\ty",     // lowered
		"4\t0.2\tbad",                 // short line → dropped
		"x\t0.2\thouse\tдом\tx\ty",    // bad count → dropped
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/en-ru.dic.gz" {
			http.NotFound(w, r)
			return
		}
		w.Write(dicGz(t, lines))
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Requested in reverse order: the alphabetical pair name must still be used.
	if err := FetchLexicons(context.Background(), dir, "ru", "en", srv.URL); err != nil {
		t.Fatal(err)
	}

	fwd := readLexicon(t, filepath.Join(dir, "en-ru.tsv.gz"))
	if fwd["house"] != "дом|здание" {
		t.Fatalf("house = %q, want дом|здание (count-desc)", fwd["house"])
	}
	if fwd["home"] != "дом" {
		t.Fatalf("home = %q, want дом (lowercased headword)", fwd["home"])
	}
	if _, ok := fwd["rare"]; ok {
		t.Fatal("count<3 entry survived")
	}
	if _, ok := fwd["ho2use"]; ok {
		t.Fatal("digit-bearing word survived")
	}

	rev := readLexicon(t, filepath.Join(dir, "ru-en.tsv.gz"))
	// дом ← house(10), home(9): count desc.
	if rev["дом"] != "house|home" {
		t.Fatalf("дом = %q, want house|home", rev["дом"])
	}
}

func TestFetchLexiconsTop12CutThenDedupe(t *testing.T) {
	// 13 distinct targets with descending counts: the 13th must be cut.
	var lines []string
	targets := []string{"aa", "bb", "cc", "dd", "ee", "ff", "gg", "hh", "ii", "jj", "kk", "ll", "mm"}
	for i, tgt := range targets {
		lines = append(lines, "100\t0\tword\t"+tgt+"\tx\ty")
		_ = i
	}
	// Give them distinct counts so order is deterministic: 113..101.
	for i := range lines {
		lines[i] = strings.Replace(lines[i], "100", string(rune('1'))+string(rune('0'+((13-i)/10)))+string(rune('0'+((13-i)%10))), 1)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(dicGz(t, lines))
	}))
	defer srv.Close()

	dir := t.TempDir()
	if err := FetchLexicons(context.Background(), dir, "en", "ru", srv.URL); err != nil {
		t.Fatal(err)
	}
	fwd := readLexicon(t, filepath.Join(dir, "en-ru.tsv.gz"))
	got := strings.Split(fwd["word"], "|")
	if len(got) != 12 {
		t.Fatalf("kept %d targets, want 12: %v", len(got), got)
	}
	if got[0] != "aa" || got[11] != "ll" {
		t.Fatalf("order wrong: %v", got)
	}
}

func TestFetchLexiconsSkipsUncoveredAndCached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write(dicGz(t, []string{"5\t0\thaus\tдом\tx\ty"}))
	}))
	defer srv.Close()

	dir := t.TempDir()
	if err := FetchLexicons(context.Background(), dir, "en", "zh", srv.URL); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("uncovered pair hit the network")
	}
	if err := FetchLexicons(context.Background(), dir, "de", "ru", srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := FetchLexicons(context.Background(), dir, "de", "ru", srv.URL); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("cached pair fetched again (%d calls)", calls)
	}
}

func TestFetchLexiconsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if err := FetchLexicons(context.Background(), t.TempDir(), "en", "ru", srv.URL); err == nil {
		t.Fatal("HTTP 500 not surfaced")
	}
}
