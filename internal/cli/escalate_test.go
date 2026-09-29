package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
	"github.com/dimando/reader/converter/internal/translate"
)

// Under --repair the escalated translation must replace the proofread text
// that assembly reads first; a sentence the escalation did not change keeps
// its proofread entry.
func TestPromoteEscalatedUnderRepair(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{CacheDir: dir, Source: "en", Repair: true}
	g := &glossaryNS{target: "ru", cacheModel: "cheap+g:1"}
	final, rp := finalModel(cfg, g.cacheModel), repairModel(cfg, g.cacheModel)
	mk := func(src string) *tbook.Sentence {
		return &tbook.Sentence{Src: src, Tr: map[string]tbook.Translation{}}
	}
	esc, kept, unaligned := mk("Fire in the hole."), mk("He left."), mk("Hush.")
	subset := []*tbook.Sentence{esc, kept, unaligned}
	aligned := func(text string) tbook.Translation {
		return tbook.Translation{Text: text, Align: []tbook.AlignChunk{{T: [2]int{0, 1}, W: []int{0}}}}
	}
	for _, s := range subset { // the cheap model's run: raw + proofread + final aligned
		_ = cache.Write(dir, cache.TrKey(s.Src, "en", "ru", g.cacheModel), tbook.Translation{Text: "old " + s.Src})
		_ = cache.Write(dir, cache.RepairKey(s.Src, "en", "ru", rp), tbook.Translation{Text: "fixed " + s.Src})
		_ = cache.Write(dir, cache.Key(s.Src, "en", "ru", final), aligned("fixed "+s.Src))
	}
	before := rawTexts(cfg, g, subset)
	// The escalation pipeline (no repair) rewrites the raw namespace.
	_ = cache.Write(dir, cache.TrKey(esc.Src, "en", "ru", g.cacheModel), tbook.Translation{Text: "Берегись!"})
	_ = cache.Write(dir, cache.Key(esc.Src, "en", "ru", g.cacheModel), aligned("Берегись!"))
	_ = cache.Write(dir, cache.TrKey(unaligned.Src, "en", "ru", g.cacheModel), tbook.Translation{Text: "Тише."})

	promoteEscalated(cfg, g, subset, before)
	translate.FillFromCache(subset, []string{"ru"}, dir, "en", final, g.cacheModel, rp)

	if got := esc.Tr["ru"]; got.Text != "Берегись!" || len(got.Align) == 0 {
		t.Errorf("escalated = %+v, want the aligned escalated text", got)
	}
	if got := unaligned.Tr["ru"]; got.Text != "Тише." || len(got.Align) != 0 {
		t.Errorf("unaligned escalated = %+v, want the raw escalated text without highlights", got)
	}
	if got := kept.Tr["ru"].Text; got != "fixed He left." {
		t.Errorf("unchanged = %q, want its proofread text", got)
	}
}

func TestClientOptionsVia(t *testing.T) {
	t.Setenv("OPENROUTER_BASE_URL", "")
	t.Setenv("OPENROUTER_API_KEY", "ork")
	cfg := &config.Config{Provider: config.ProviderGonka, BaseURL: "https://proxy.gonka.gg/v1", APIKey: "gk"}
	o := clientOptionsVia(cfg, config.ProviderOpenRouter, "google/gemini-3.7-flash", 0)
	if o.Provider != config.ProviderOpenRouter || o.BaseURL != "https://openrouter.ai/api/v1" || o.APIKey != "ork" {
		t.Errorf("judge on openrouter = %q %q key %q", o.Provider, o.BaseURL, o.APIKey)
	}
	o = clientOptionsVia(cfg, config.ProviderGonka, "m", 0)
	if o.Provider != config.ProviderGonka || o.APIKey != "gk" {
		t.Errorf("same provider = %q key %q, want the run's own endpoint", o.Provider, o.APIKey)
	}
}

// fakeLLM stands in for the escalator and the judge on one endpoint. Each
// hook answers for one item; a nil hook (or ok=false) leaves the item out of
// the reply, which is how an empty or unusable reply looks to the pipeline.
type fakeLLM struct {
	translate func(src string) (string, bool)
	align     func(src, tr string) bool // true: word-by-word chunks
	judge     func(src, tr string) translate.Verdict
	status    int // non-zero: every translate request answers this HTTP status
}

func (f *fakeLLM) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var items []map[string]any
		_ = json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &items)
		out := map[string]any{}
		for _, it := range items {
			id, _ := it["id"].(string)
			src, _ := it["src"].(string)
			tr, _ := it["tr"].(string)
			_, isJudge := it["pairs"]
			switch {
			case isJudge:
				if f.judge != nil {
					out[id] = f.judge(src, tr)
				}
			case it["tr"] != nil:
				if f.align != nil && f.align(src, tr) {
					srcW, trW := strings.Fields(src), strings.Fields(tr)
					var chunks []map[string]any
					for i, w := range trW {
						if i < len(srcW) {
							chunks = append(chunks, map[string]any{"tgt": w, "en": strings.Trim(srcW[i], ".")})
						}
					}
					out[id] = chunks
				}
			default:
				if f.status != 0 {
					w.WriteHeader(f.status)
					_, _ = w.Write([]byte(`{"error":{"message":"no such model"}}`))
					return
				}
				if f.translate != nil {
					if text, ok := f.translate(src); ok {
						out[id] = text
					}
				}
			}
		}
		content, _ := json.Marshal(out)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// escFixture is one judge-flagged cell as the base run shipped it: a French
// translation with its alignment and a failing verdict in the cache.
type escFixture struct {
	cfg  *config.Config
	g    *glossaryNS
	s    *tbook.Sentence
	orig tbook.Translation
	esc  *escalationOutcome
}

const (
	escSrc  = "The old man sat down."
	escOrig = "Le vieil homme assis."
)

func newEscFixture(t *testing.T, baseURL, origWhy string) *escFixture {
	t.Helper()
	statsLog, progressLog = nil, nil
	dir := t.TempDir()
	cfg := &config.Config{
		Provider: config.ProviderOpenRouter, BaseURL: baseURL, APIKey: "test-key",
		Model: "cheap", EscalateModel: "strong", JudgeModel: "judge", Judge: true,
		JudgeScope: config.JudgeScopeAll,
		Source:     "en", Targets: []string{"fr"}, CacheDir: dir,
		Out:       filepath.Join(t.TempDir(), "book.tbook"),
		BatchSize: 4, Concurrency: 1, Format: config.FormatV2,
	}
	g := &glossaryNS{target: "fr", cacheModel: "cheap"}
	s := &tbook.Sentence{Src: escSrc, Words: segment.Tokenize(escSrc), Tr: map[string]tbook.Translation{}}
	orig := alignedTr(escOrig, len(s.Words))
	_ = cache.Write(dir, cache.TrKey(s.Src, "en", "fr", g.cacheModel), tbook.Translation{Text: escOrig})
	_ = cache.Write(dir, cache.Key(s.Src, "en", "fr", g.cacheModel), orig)
	s.Tr["fr"] = orig
	if err := translate.SaveVerdict(dir, cfg.JudgeModel, "en", "fr", s, translate.Verdict{Why: origWhy}); err != nil {
		t.Fatal(err)
	}
	return &escFixture{cfg: cfg, g: g, s: s, orig: orig, esc: newEscalationOutcome()}
}

func (f *escFixture) escalate(t *testing.T, emb func() translate.WordAligner) {
	t.Helper()
	err := escalate(context.Background(), f.cfg, []*glossaryNS{f.g}, []*tbook.Sentence{f.s},
		[]string{f.s.Src}, f.esc, emb)
	if err != nil {
		t.Fatalf("escalate: %v", err)
	}
}

// wantOriginal: the cell ships the base translation with its alignment, in
// memory and in the cache a re-assembly reads, and is recorded unverified.
func (f *escFixture) wantOriginal(t *testing.T) {
	t.Helper()
	got := f.s.Tr["fr"]
	if got.Text != escOrig || len(got.Align) != len(f.orig.Align) || len(got.Align) == 0 {
		t.Errorf("shipped %+v, want the original %q with its alignment", got, escOrig)
	}
	re := &tbook.Sentence{Src: f.s.Src, Words: f.s.Words, Tr: map[string]tbook.Translation{}}
	translate.FillFromCache([]*tbook.Sentence{re}, []string{"fr"}, f.cfg.CacheDir, "en",
		finalModel(f.cfg, f.g.cacheModel), f.g.cacheModel, "")
	if re.Tr["fr"].Text != escOrig || len(re.Tr["fr"].Align) == 0 {
		t.Errorf("cache re-assembles %+v, want the original with its alignment", re.Tr["fr"])
	}
	if !f.esc.unverified["fr"][f.s.Src] {
		t.Error("cell not recorded as unverified")
	}
	if len(f.esc.rejected["fr"]) != 0 {
		t.Errorf("rejected = %v, want none", f.esc.rejected)
	}
}

// wantV2 checks the version-2 status and verdict the cell would be written with.
func (f *escFixture) wantV2(t *testing.T, status, why string) {
	t.Helper()
	g := newGateState(f.cfg, []*tbook.Sentence{f.s}, []string{"fr"}, nil, f.esc)
	tr := &v2.Translation{T: f.s.Tr["fr"].Text}
	g.annotate(f.s, "fr", tr, len(f.s.Tr["fr"].Align) > 0)
	if tr.S != status {
		t.Errorf("v2 status = %q, want %q", tr.S, status)
	}
	if why == "" {
		if tr.V != nil {
			t.Errorf("v2 verdict = %+v, want none", tr.V)
		}
	} else if tr.V == nil || tr.V.Why != why {
		t.Errorf("v2 verdict = %+v, want %q", tr.V, why)
	}
}

// The q1 case (flagged-fr, all-de, all-fr): the judge rejects the escalated
// text as a mistranslation in both rounds. It used to ship untranslated; the
// base translation must ship instead, flagged with its own verdict.
func TestEscalationRejectedKeepsOriginal(t *testing.T) {
	llm := &fakeLLM{
		translate: func(string) (string, bool) { return "Le vieux monsieur assis.", true },
		align:     func(string, string) bool { return true },
		judge: func(_, tr string) translate.Verdict {
			return translate.Verdict{Why: "mistranslation: invented detail"}
		},
	}
	f := newEscFixture(t, llm.serve(t), "mistranslation: wrong verb")
	f.escalate(t, nil)
	f.wantOriginal(t)
	f.wantV2(t, v2.StatusUnverified, "mistranslation: wrong verb")
}

// "Left unverified" (wrong-mapping both rounds) used to ship the escalated
// text with its alignment stripped. The original ships, aligned.
func TestEscalationUnverifiedKeepsAlignment(t *testing.T) {
	llm := &fakeLLM{
		translate: func(string) (string, bool) { return "Le vieux monsieur assis.", true },
		align:     func(string, string) bool { return true },
		judge:     func(string, string) translate.Verdict { return translate.Verdict{Why: "wrong-mapping"} },
	}
	f := newEscFixture(t, llm.serve(t), "wrong-mapping")
	f.escalate(t, nil)
	f.wantOriginal(t)
	f.wantV2(t, v2.StatusUnverified, "wrong-mapping")
}

// Every way the escalator can fail to deliver text keeps the original.
func TestEscalationFailureKeepsOriginal(t *testing.T) {
	cases := map[string]*fakeLLM{
		"empty reply":    {},
		"wrong language": {translate: func(string) (string, bool) { return "老人坐了下来。", true }},
		"model 404":      {status: http.StatusNotFound},
	}
	for name, llm := range cases {
		t.Run(name, func(t *testing.T) {
			llm.align = func(string, string) bool { return true }
			llm.judge = func(string, string) translate.Verdict { return translate.Verdict{OK: true} }
			f := newEscFixture(t, llm.serve(t), "mistranslation: wrong verb")
			f.escalate(t, nil)
			f.wantOriginal(t)
			f.wantV2(t, v2.StatusUnverified, "mistranslation: wrong verb")
		})
	}
}

// Escalated text the LLM aligner cannot align, judged fine: the local
// aligner supplies the alignment, so the escalated text ships aligned.
func TestEscalationUnalignedFallsBackToEmbedding(t *testing.T) {
	const escText = "Le vieux monsieur assis."
	llm := &fakeLLM{
		translate: func(string) (string, bool) { return escText, true },
		judge: func(_, tr string) translate.Verdict {
			return translate.Verdict{OK: tr == escText}
		},
	}
	f := newEscFixture(t, llm.serve(t), "mistranslation: wrong verb")
	emb := func() translate.WordAligner { return pairAligner{} }
	f.escalate(t, emb)
	got := f.s.Tr["fr"]
	if got.Text != escText || len(got.Align) == 0 {
		t.Fatalf("shipped %+v, want the escalated text with an embedding alignment", got)
	}
	if f.esc.unverified["fr"][f.s.Src] {
		t.Error("a verified escalation must not be recorded unverified")
	}
	f.wantV2(t, "", "")
}

// Same, with no local aligner available: the original ships aligned rather
// than the escalated text with no highlights, and stays flagged.
func TestEscalationUnalignedWithoutAlignerKeepsOriginal(t *testing.T) {
	llm := &fakeLLM{
		translate: func(string) (string, bool) { return "Le vieux monsieur assis.", true },
		judge: func(_, tr string) translate.Verdict {
			return translate.Verdict{OK: tr != escOrig}
		},
	}
	f := newEscFixture(t, llm.serve(t), "mistranslation: wrong verb")
	f.escalate(t, func() translate.WordAligner { return nil })
	f.wantOriginal(t)
	f.wantV2(t, v2.StatusUnverified, "mistranslation: wrong verb")
}

// pairAligner maps word i to word i.
type pairAligner struct{}

func (pairAligner) Align(src, tgt []string) ([][2]int, error) {
	var out [][2]int
	for i := range min(len(src), len(tgt)) {
		out = append(out, [2]int{i, i})
	}
	return out, nil
}

// Under --repair the cell lives in more namespaces (proofread text, repaired
// final entry); a rejected escalation must put all of them back.
func TestEscalationRejectedKeepsOriginalUnderRepair(t *testing.T) {
	llm := &fakeLLM{
		translate: func(string) (string, bool) { return "Le vieux monsieur assis.", true },
		align:     func(string, string) bool { return true },
		judge:     func(string, string) translate.Verdict { return translate.Verdict{Why: "mistranslation"} },
	}
	f := newEscFixture(t, llm.serve(t), "mistranslation: wrong verb")
	f.cfg.Repair = true
	dir, cm := f.cfg.CacheDir, f.g.cacheModel
	_ = cache.Write(dir, cache.RepairKey(escSrc, "en", "fr", repairModel(f.cfg, cm)), tbook.Translation{Text: escOrig})
	_ = cache.Write(dir, cache.Key(escSrc, "en", "fr", finalModel(f.cfg, cm)), f.orig)
	f.escalate(t, nil)
	if got := f.s.Tr["fr"]; got.Text != escOrig || len(got.Align) == 0 {
		t.Fatalf("shipped %+v, want the original with its alignment", got)
	}
	re := &tbook.Sentence{Src: escSrc, Words: f.s.Words, Tr: map[string]tbook.Translation{}}
	translate.FillFromCache([]*tbook.Sentence{re}, []string{"fr"}, dir, "en",
		finalModel(f.cfg, cm), cm, repairModel(f.cfg, cm))
	if re.Tr["fr"].Text != escOrig || len(re.Tr["fr"].Align) == 0 {
		t.Errorf("cache re-assembles %+v, want the original with its alignment", re.Tr["fr"])
	}
}
