package translate

import (
	"context"
	"fmt"
	"testing"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/embalign"
	"github.com/dimando/reader/converter/internal/tbook"
)

// fakeAligner returns fixed pairs (or a fixed error) for every sentence.
type fakeAligner struct {
	pairs [][2]int
	err   error
}

func (f fakeAligner) Align(src, tgt []string) ([][2]int, error) { return f.pairs, f.err }

func embTestSentence() (*tbook.Sentence, string) {
	s := &tbook.Sentence{
		Src:   "The cat",
		Words: [][2]int{{0, 3}, {4, 7}},
		Tr:    map[string]tbook.Translation{},
	}
	return s, "Кот тут"
}

func seedRaw(t *testing.T, dir string, s *tbook.Sentence, raw string) item {
	t.Helper()
	if err := cache.Write(dir, cache.TrKey(s.Src, "en", "ru", "m"), tbook.Translation{Text: raw}); err != nil {
		t.Fatal(err)
	}
	return item{key: cache.Key(s.Src, "en", "ru", "m"), s: s}
}

// TestEmbedAlignWritesFinalEntry: emb mode writes a final cache entry with
// per-word chunks and coverage Q — indistinguishable from an LLM-aligned one.
func TestEmbedAlignWritesFinalEntry(t *testing.T) {
	dir := t.TempDir()
	s, raw := embTestSentence()
	it := seedRaw(t, dir, s, raw)
	p := &Pipeline{
		CacheDir: dir, Source: "en", CacheModel: "m",
		AlignMode:  AlignEmb,
		EmbAligner: fakeAligner{pairs: [][2]int{{0, 0}, {1, 1}}},
	}
	leftover := p.embedAlign(context.Background(), []item{it}, "ru")
	if len(leftover) != 0 {
		t.Fatalf("leftover = %d, want 0 (emb mode never falls back)", len(leftover))
	}
	tr, ok := cache.Read(dir, it.key)
	if !ok {
		t.Fatal("no final cache entry written")
	}
	if tr.Text != raw || len(tr.Align) != 2 || tr.Q != 1.0 {
		t.Errorf("entry = %+v, want text %q, 2 chunks, Q=1", tr, raw)
	}
	if tr.Align[0].T != [2]int{0, 3} || tr.Align[0].W[0] != 0 ||
		tr.Align[1].T != [2]int{4, 7} || tr.Align[1].W[0] != 1 {
		t.Errorf("chunks = %+v", tr.Align)
	}
}

// TestEmbedAlignHybridGate: an alignment below the coverage threshold is NOT
// cached and returns as leftover for the LLM align pass.
func TestEmbedAlignHybridGate(t *testing.T) {
	dir := t.TempDir()
	s, raw := embTestSentence()
	it := seedRaw(t, dir, s, raw)
	p := &Pipeline{
		CacheDir: dir, Source: "en", CacheModel: "m",
		AlignMode:  AlignHybrid,
		EmbAligner: fakeAligner{pairs: [][2]int{{0, 0}}}, // 1 of 2 tr words → Q=0.5 < 0.7
	}
	leftover := p.embedAlign(context.Background(), []item{it}, "ru")
	if len(leftover) != 1 {
		t.Fatalf("leftover = %d, want 1 (gated to LLM)", len(leftover))
	}
	if _, ok := cache.Read(dir, it.key); ok {
		t.Error("gated sentence must not be cached as final")
	}
	// The same alignment passes a permissive threshold.
	p.EmbQMin = 0.4
	if leftover = p.embedAlign(context.Background(), []item{it}, "ru"); len(leftover) != 0 {
		t.Fatalf("leftover = %d, want 0 at EmbQMin=0.4", len(leftover))
	}
	if _, ok := cache.Read(dir, it.key); !ok {
		t.Error("accepted sentence must be cached as final")
	}
}

// TestEmbedAlignDeadProcess: a dead aligner sends everything remaining to the
// LLM pass in hybrid mode, and nowhere in emb mode.
func TestEmbedAlignDeadProcess(t *testing.T) {
	dir := t.TempDir()
	s, raw := embTestSentence()
	it := seedRaw(t, dir, s, raw)
	dead := fakeAligner{err: fmt.Errorf("%w: broken pipe", embalign.ErrDead)}

	p := &Pipeline{CacheDir: dir, Source: "en", CacheModel: "m", AlignMode: AlignHybrid, EmbAligner: dead}
	if leftover := p.embedAlign(context.Background(), []item{it}, "ru"); len(leftover) != 1 {
		t.Errorf("hybrid: leftover = %d, want 1", len(leftover))
	}
	p.AlignMode = AlignEmb
	if leftover := p.embedAlign(context.Background(), []item{it}, "ru"); len(leftover) != 0 {
		t.Errorf("emb: leftover = %d, want 0", len(leftover))
	}
	if _, ok := cache.Read(dir, it.key); ok {
		t.Error("nothing must be cached when the aligner is dead")
	}
}
