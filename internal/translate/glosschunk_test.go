package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dimando/reader/converter/internal/tbook"
)

func sampleSentences(n int) []*tbook.Sentence {
	out := make([]*tbook.Sentence, n)
	for i := range out {
		out[i] = &tbook.Sentence{Src: fmt.Sprintf("Sentence number %d of the book.", i),
			Tr: map[string]tbook.Translation{}}
	}
	return out
}

// TestSampleChunksSpreadOverTheBook: every chunk must see the whole book, not
// one chapter of it, or the terms of the last chapters are never asked for.
func TestSampleChunksSpreadOverTheBook(t *testing.T) {
	sample := evenSample(sampleSentences(400))
	if len(sample) != glossarySampleMax {
		t.Fatalf("sample = %d, want %d", len(sample), glossarySampleMax)
	}
	chunks := sampleChunks(sample)
	if len(chunks) != glossarySampleChunks {
		t.Fatalf("%d chunks, want %d", len(chunks), glossarySampleChunks)
	}
	total := 0
	for i, c := range chunks {
		total += len(c)
		if len(c) != glossarySampleMax/glossarySampleChunks {
			t.Errorf("chunk %d has %d sentences", i, len(c))
		}
		// Interleaved, so the first and last sentence of the book are in
		// different chunks and each chunk reaches the end.
		if c[0] != sample[i] || c[len(c)-1] != sample[len(sample)-glossarySampleChunks+i] {
			t.Errorf("chunk %d does not span the sample: %q … %q", i, c[0], c[len(c)-1])
		}
	}
	if total != len(sample) {
		t.Errorf("chunks cover %d of %d sentences", total, len(sample))
	}
	// Degenerate inputs must not produce empty calls.
	if got := sampleChunks(nil); got != nil {
		t.Errorf("sampleChunks(nil) = %v", got)
	}
	if got := sampleChunks([]string{"one"}); len(got) != 1 || len(got[0]) != 1 {
		t.Errorf("a one-sentence book made %v", got)
	}
}

// glossServer answers each sample call with n entries, or with the given cut
// reply and finish_reason=length.
func glossServer(t *testing.T, entries int, cut string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		msg := map[string]any{"content": ""}
		finish := "stop"
		if cut != "" {
			msg["content"] = cut
			finish = "length"
		} else {
			list := make([]map[string]string, entries)
			for i := range list {
				// Overlapping head terms across calls, so the merge has to dedupe.
				list[i] = map[string]string{
					"src": fmt.Sprintf("term%d", i), "tgt": fmt.Sprintf("Begriff%d-%d", i, n)}
			}
			b, _ := json.Marshal(map[string]any{"glossary": list})
			msg["content"] = string(b)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": msg, "finish_reason": finish}}})
	}))
	return srv, &calls
}

func glossClient(t *testing.T, url string, retries int) *Client {
	t.Helper()
	return NewClient(Options{BaseURL: url, APIKey: "x", Model: "test-model",
		JSONMode: true, MaxRetries: retries, Timeout: 5 * time.Second})
}

// TestSampleGlossaryMergesChunks: four calls, merged and de-duplicated by head
// term, the first call's rendering winning.
func TestSampleGlossaryMergesChunks(t *testing.T) {
	srv, calls := glossServer(t, 3, "")
	defer srv.Close()

	got, err := buildSampleGlossary(context.Background(), glossClient(t, srv.URL, 0),
		sampleSentences(400), GlossaryBuild{Source: "en", Target: "de", Title: "T", Author: "A"})
	if err != nil {
		t.Fatalf("buildSampleGlossary: %v", err)
	}
	if calls.Load() != glossarySampleChunks {
		t.Errorf("%d calls, want %d", calls.Load(), glossarySampleChunks)
	}
	if len(got) != 3 {
		t.Fatalf("%d entries, want 3 de-duplicated head terms: %+v", len(got), got)
	}
	for i, e := range got {
		if e.Src != fmt.Sprintf("term%d", i) {
			t.Errorf("entry %d = %q, want merge order", i, e.Src)
		}
	}
}

// TestSampleGlossaryRejectsATruncatedReply is the measured failure: the model
// runs past the entry cap and the reply is cut mid-list. It is a retried
// failure, never a silently accepted partial — the cut list below parses as
// nothing, and half a glossary enforced on a whole book is worse than none.
func TestSampleGlossaryRejectsATruncatedReply(t *testing.T) {
	// Verbatim shape of the real one (Sherlock ch.1-2, en→fr, 8 879 bytes,
	// 250 entries): a well-formed opening that simply stops.
	const cut = `{"glossary":[{"src":"Sherlock Holmes","tgt":"Sherlock Holmes"},` +
		`{"src":"Irene Adler","tgt":"Irène Adler"},{"src":"monarch","tgt":"monarq`
	srv, calls := glossServer(t, 0, cut)
	defer srv.Close()

	_, err := buildSampleGlossary(context.Background(), glossClient(t, srv.URL, 1),
		sampleSentences(400), GlossaryBuild{Source: "en", Target: "fr", Title: "T", Author: "A"})
	if err == nil {
		t.Fatal("a truncated reply was accepted")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("err = %v, want it to name the truncation", err)
	}
	// Every chunk retried its one attempt: 4 chunks × (1 try + 1 retry).
	if want := int32(glossarySampleChunks * 2); calls.Load() != want {
		t.Errorf("%d calls, want %d — a truncated reply must be retried", calls.Load(), want)
	}
}

// A chunk that fails must not cost the book the rest of the sample.
func TestSampleGlossarySurvivesAFailedChunk(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 { // the first chunk to arrive is unrecoverable
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"nope"}}`))
			return
		}
		b, _ := json.Marshal(map[string]any{"glossary": []map[string]string{
			{"src": "stack", "tgt": "Stapel"}}})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{
			{"message": map[string]any{"content": string(b)}, "finish_reason": "stop"}}})
	}))
	defer srv.Close()

	got, err := buildSampleGlossary(context.Background(), glossClient(t, srv.URL, 0),
		sampleSentences(400), GlossaryBuild{Source: "en", Target: "de", Title: "T", Author: "A"})
	if err != nil {
		t.Fatalf("one failed chunk cost the whole pass: %v", err)
	}
	if len(got) != 1 || got[0].Src != "stack" {
		t.Errorf("entries = %+v, want the surviving chunk's term", got)
	}
}
