package translate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/tbook"
)

// The verbatim pair from the gonka probe: an en→ru batch that came back in
// fluent Chinese, in well-formed JSON, with every id present and the request a
// success (bench-quality/reports/gonka-models-2026-09-19.md).
const (
	guardSrcChinese = "It is a hobby of mine to have an exact knowledge of London."
	guardGotChinese = "我的一个爱好是对伦敦了如指掌。"
	guardSrcRussian = "“Wedlock suits you,” he remarked."
	guardGotRussian = "«Брак вам к лицу», — заметил он."
)

// guardServer answers the translate pass with a batch mixing Russian and
// Chinese, then — after relentCalls attempts — with Russian throughout. It
// records every system prompt it saw so the test can check the nudge.
func guardServer(t *testing.T, relentAfter int32, prompts *[]string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		system := req.Messages[0].Content
		userJSON := req.Messages[len(req.Messages)-1].Content
		var items []alignInput
		_ = json.Unmarshal([]byte(userJSON), &items)

		out := map[string]any{}
		if strings.Contains(userJSON, `"words"`) { // align pass: one chunk per token
			for _, it := range items {
				var chunks []map[string]any
				for tok := range strings.FieldsSeq(it.Tr) {
					chunks = append(chunks, map[string]any{"tgt": tok, "en": "0:" + strings.Fields(it.Src)[0]})
				}
				out[it.ID] = chunks
			}
		} else {
			n := calls.Add(1)
			*prompts = append(*prompts, system)
			for _, it := range items {
				switch {
				case it.Src != guardSrcChinese:
					out[it.ID] = guardGotRussian
				case n > relentAfter:
					out[it.ID] = "Одно из моих увлечений — досконально знать Лондон."
				default:
					out[it.ID] = guardGotChinese
				}
			}
		}
		content, _ := json.Marshal(out)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": string(content)}}}})
	}))
	return srv, &calls
}

func guardSentences() []*tbook.Sentence {
	return []*tbook.Sentence{
		{Src: guardSrcRussian, Words: [][2]int{{1, 8}}, Tr: map[string]tbook.Translation{}},
		{Src: guardSrcChinese, Words: [][2]int{{0, 2}}, Tr: map[string]tbook.Translation{}},
	}
}

func guardPipeline(t *testing.T, url, cacheDir string, retries int) *Pipeline {
	t.Helper()
	client := NewClient(Options{BaseURL: url, APIKey: "x", Model: "test-model",
		Temperature: 0, MaxRetries: retries, Timeout: 5 * time.Second})
	return &Pipeline{Client: client, CacheDir: cacheDir, Source: "en",
		BatchSize: 8, Concurrency: 1}
}

// TestGuardRefusesWrongLanguageBatch: one Chinese sentence fails the whole
// batch, the retry carries the nudge, and when the model never relents the
// wrong-language sentence is left untranslated rather than cached — while its
// batch-mate, which was always Russian, ships.
func TestGuardRefusesWrongLanguageBatch(t *testing.T) {
	var prompts []string
	srv, calls := guardServer(t, 1<<30, &prompts) // never relents
	defer srv.Close()
	dir := t.TempDir()
	pipe := guardPipeline(t, srv.URL, dir, 2)

	sents := guardSentences()
	if err := pipe.Run(context.Background(), sents, []string{"ru"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := pipe.WrongLangCount("ru"); got == 0 {
		t.Errorf("WrongLangCount = 0, want the Chinese sentence counted")
	}
	if _, ok := cache.Read(dir, cache.TrKey(guardSrcChinese, "en", "ru", "test-model")); ok {
		t.Error("the Chinese translation was cached; the guard must leave the sentence untranslated")
	}
	if tr, ok := cache.Read(dir, cache.TrKey(guardSrcRussian, "en", "ru", "test-model")); !ok || tr.Text != guardGotRussian {
		t.Errorf("the Russian batch-mate did not ship: %q, ok=%v", tr.Text, ok)
	}
	// FillFromCache is what the assembly reads: version 1's untranslated shape.
	FillFromCache(sents, []string{"ru"}, dir, "en", "test-model", "test-model", "")
	if got := sents[1].Tr["ru"]; got.Text != "" || len(got.Align) != 0 {
		t.Errorf("shipped %+v, want an empty translation", got)
	}
	// First attempt plain, every retry nudged.
	if len(prompts) < 2 {
		t.Fatalf("only %d translate attempts (%d calls); want a retry", len(prompts), calls.Load())
	}
	if strings.Contains(prompts[0], "Output MUST be in Russian.") {
		t.Error("the first attempt carried the nudge; it belongs on the retry")
	}
	if !strings.Contains(prompts[1], "Output MUST be in Russian.") {
		t.Error("the retry did not carry the language nudge")
	}
}

// TestGuardAcceptsAfterNudge: a model that answers in the target language on
// the retry has its batch cached as usual, with no residue.
func TestGuardAcceptsAfterNudge(t *testing.T) {
	var prompts []string
	srv, _ := guardServer(t, 1, &prompts) // Chinese once, then Russian
	defer srv.Close()
	dir := t.TempDir()
	pipe := guardPipeline(t, srv.URL, dir, 3)

	if err := pipe.Run(context.Background(), guardSentences(), []string{"ru"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := pipe.WrongLangCount("ru"); got != 0 {
		t.Errorf("WrongLangCount = %d, want 0 after the model relented", got)
	}
	tr, ok := cache.Read(dir, cache.TrKey(guardSrcChinese, "en", "ru", "test-model"))
	if !ok || strings.Contains(tr.Text, "爱好") {
		t.Errorf("cached %q, want the Russian retry", tr.Text)
	}
}

// TestFreezeUnrepairedRefusesWrongLanguage covers the structural trap: freezing
// raw text into the proofread namespace makes it proofread by definition, so a
// wrong-language raw translation would never be retried by any later run.
func TestFreezeUnrepairedRefusesWrongLanguage(t *testing.T) {
	dir := t.TempDir()
	pipe := &Pipeline{CacheDir: dir, Source: "en", Repair: true, CacheModel: "test-model"}
	items := []item{
		{key: "k1", s: &tbook.Sentence{Src: guardSrcRussian, Tr: map[string]tbook.Translation{}}},
		{key: "k2", s: &tbook.Sentence{Src: guardSrcChinese, Tr: map[string]tbook.Translation{}}},
	}
	raw := pipe.rawModel()
	_ = cache.Write(dir, cache.TrKey(guardSrcRussian, "en", "ru", raw), tbook.Translation{Text: guardGotRussian})
	_ = cache.Write(dir, cache.TrKey(guardSrcChinese, "en", "ru", raw), tbook.Translation{Text: guardGotChinese})

	frozen, refused := pipe.freezeUnrepaired(items, "ru")
	if frozen != 1 || len(refused) != 1 {
		t.Fatalf("frozen=%d refused=%v, want 1 and one sentence", frozen, refused)
	}
	if _, ok := cache.Read(dir, cache.RepairKey(guardSrcChinese, "en", "ru", pipe.repairModel())); ok {
		t.Error("the Chinese text was frozen as proofread; no later run would ever retry it")
	}
	if _, ok := cache.Read(dir, cache.TrKey(guardSrcChinese, "en", "ru", raw)); ok {
		t.Error("the Chinese raw translation survived; FillFromCache would ship it as a fallback")
	}
}

func TestDropForeignRenderings(t *testing.T) {
	entries := []GlossEntry{
		{Src: "Irene Adler", Tgt: "Айрин Адлер"},
		{Src: "Sherlock Holmes", Tgt: "Sherlock Holmes"},
		{Src: "Baker Street", Tgt: "Bakerstraße"},
	}
	got := DropForeignRenderings(entries, "de")
	if len(got) != 2 || got[0].Src != "Sherlock Holmes" {
		t.Errorf("kept %+v, want the two Latin renderings", got)
	}
	if len(DropForeignRenderings(entries, "ru")) != 1 {
		t.Error("for ru the Latin renderings are the foreign ones")
	}
}
