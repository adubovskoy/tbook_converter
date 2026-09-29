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

	"github.com/dimando/reader/converter/internal/tbook"
)

func TestValidExprs(t *testing.T) {
	src := "I think it is far better not to wash linen of the sort in public, all the same."
	in := []Expr{
		{Words: "wash linen of the sort in public,", Meaning: "air private matters openly"}, // edge comma trimmed
		{Words: "wash linen of the sort in public", Meaning: "dup"},                          // duplicate
		{Words: "Wash linen", Meaning: "wrong case"},                                        // not verbatim
		{Words: "air dirty laundry", Meaning: "not in src"},                                 // paraphrase
		{Words: "better", Meaning: "single word"},                                           // too short
		{Words: "all the same", Meaning: ""},                                                // no meaning
		{Words: "ll the sam", Meaning: "mid-word"},                                          // not word-bounded
		{Words: "  all the same. ", Meaning: "nevertheless"},
	}
	out, rej := validExprs(src, in)
	want := []Expr{
		{Words: "wash linen of the sort in public", Meaning: "air private matters openly"},
		{Words: "all the same", Meaning: "nevertheless"},
	}
	if len(out) != len(want) {
		t.Fatalf("got %+v, want %+v", out, want)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("out[%d] = %+v, want %+v", i, out[i], want[i])
		}
	}
	if rej != 5 {
		t.Errorf("rejected = %d, want 5", rej)
	}
}

func TestValidExprsCapAndMeaningClip(t *testing.T) {
	src := "by and large, all the same, for the life of me, put up with it"
	long := strings.Repeat("é", 150)
	in := []Expr{{"by and large", long}, {"all the same", "x"}, {"for the life of me", "x"}, {"put up with", "x"}}
	out, rej := validExprs(src, in)
	if len(out) != exprMaxPerSent || rej != 0 {
		t.Fatalf("got %d kept, %d rejected; want %d kept, 0 rejected", len(out), rej, exprMaxPerSent)
	}
	if n := len([]rune(out[0].Meaning)); n != exprMaxMeaning {
		t.Errorf("meaning clipped to %d runes, want %d", n, exprMaxMeaning)
	}
}

func TestWordBounded(t *testing.T) {
	cases := []struct {
		src, w string
		want   bool
	}{
		{"He put up with it.", "put up with", true},
		{"setup without", "up with", false},
		{"setup without, then up with it", "up with", true}, // second occurrence is bounded
		{"«Всё равно», сказал он", "Всё равно", true},
		{"He gave it up.", "gave it up", true},
	}
	for _, c := range cases {
		if got := wordBounded(c.src, c.w); got != c.want {
			t.Errorf("wordBounded(%q, %q) = %v, want %v", c.src, c.w, got, c.want)
		}
	}
}

func TestParseExprReply(t *testing.T) {
	var raw map[string]json.RawMessage
	_ = json.Unmarshal([]byte(`{"1":[],"2":[{"words":"a b","meaning":"m"}],"3":{"words":"c d","meaning":"n"},"4":"junk"}`), &raw)
	got := parseExprReply(raw)
	if l, ok := got["1"]; !ok || len(l) != 0 {
		t.Errorf("id 1: %+v %v", l, ok)
	}
	if len(got["2"]) != 1 || len(got["3"]) != 1 {
		t.Errorf("ids 2/3: %+v", got)
	}
	if _, ok := got["4"]; ok {
		t.Errorf("malformed id 4 must be missing, got %+v", got["4"])
	}
}

func TestTranslateVariant(t *testing.T) {
	if v := TranslateVariant(false, false); v != "" {
		t.Errorf("production variant = %q, want empty (default runs keep their cache)", v)
	}
	a, b, c := TranslateVariant(true, false), TranslateVariant(false, true), TranslateVariant(true, true)
	if a == b || a == c || b == c || a == "" || b == "" {
		t.Errorf("variants must be distinct and non-empty: %q %q %q", a, b, c)
	}
}

// v5Prompt is the production translate prompt before prompt variants existed;
// the zero TrOpts must reproduce it byte for byte, or every cached book would
// silently change what it was translated with.
const v5Prompt = `You translate English → Russian for a language-learning reader: the app shows your
translation beside the original and highlights matching word pairs, so the reader
constantly compares the two texts word by word.

You receive a JSON array of sentences, each {id, src}.

For EACH sentence, write a faithful, natural literary Russian translation of src:
- Each sentence STANDS ALONE: translate exactly the content of its own src — never
  borrow, merge, or shift words or meaning from a neighboring sentence in the batch.
- COMPLETE and EXACT: every meaning element of src appears in the translation — no
  dropped clause, modifier, or negation; nothing invented; numbers, names, and
  quoted speech preserved.
- Natural Russian comes first — but when two renderings are equally natural, prefer
  the one that MIRRORS the source: give each content word an explicit Russian
  counterpart, keep the source clause order, keep metaphors as images. Do not
  paraphrase freely; split or merge clauses only where Russian grammar demands it.
- Match the source register and tone: slang stays slang, formal stays formal.
- Output PURE Russian — never leave English words in the translation.

Reply with ONLY a single JSON object mapping each "id" (exact string) to its Russian
translation as a STRING. No code fences, no commentary. Translate EVERY sentence.`

func TestTranslatePromptVariants(t *testing.T) {
	if got := translateSystemPrompt("English", "Russian", nil); got != v5Prompt {
		t.Fatalf("zero-option prompt drifted from v5:\n%s", got)
	}
	x := translateSystemPromptOpts("English", "Russian", nil, TrOpts{Expressions: true})
	for _, want := range []string{`carry "expr"`, "NEVER word by word", "MIRRORS the source"} {
		if !strings.Contains(x, want) {
			t.Errorf("expressions prompt lacks %q", want)
		}
	}
	if strings.Contains(x, "{SRC}") || strings.Contains(x, "{TGT}") {
		t.Error("unreplaced placeholder in expressions prompt")
	}
	m := translateSystemPromptOpts("English", "Russian", nil, TrOpts{Expressions: true, NoMirror: true})
	if strings.Contains(m, "MIRRORS") || !strings.Contains(m, "NEVER word by word") ||
		!strings.Contains(m, "Do not paraphrase freely") {
		t.Errorf("no-mirror prompt wrong:\n%s", m)
	}
}

func TestTranslateInputsCarryExpr(t *testing.T) {
	s1 := &tbook.Sentence{Src: "He put up with it."}
	s2 := &tbook.Sentence{Src: "He sat."}
	p := &Pipeline{Source: "en", exprOf: map[string][]Expr{s1.Src: {{"put up with", "tolerate"}}, s2.Src: {}}}
	js, ok := p.textInputs([]item{{s: s1}, {s: s2}}, "ru", phaseTranslate)
	if !ok {
		t.Fatal("no inputs")
	}
	want := `[{"id":"1","src":"He put up with it.","expr":[{"words":"put up with","meaning":"tolerate"}]},{"id":"2","src":"He sat."}]`
	if js != want {
		t.Errorf("inputs =\n%s\nwant\n%s", js, want)
	}
	p.exprOf = nil // expressions off: the production shape
	js, _ = p.textInputs([]item{{s: s1}}, "ru", phaseTranslate)
	if js != `[{"id":"1","src":"He put up with it."}]` {
		t.Errorf("untagged inputs = %s", js)
	}
}

func TestPrepareExpressionsCaches(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var items []map[string]string
		_ = json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &items)
		out := map[string]any{}
		for _, it := range items {
			if strings.Contains(it["src"], "put up with") {
				out[it["id"]] = []Expr{{"put up with", "tolerate"}, {"invented span", "x"}}
			} else {
				out[it["id"]] = []Expr{}
			}
		}
		content, _ := json.Marshal(out)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]any{"content": string(content)}}}})
	}))
	defer srv.Close()
	client := NewClient(Options{BaseURL: srv.URL, APIKey: "x", Model: "m", MaxRetries: 1, Timeout: 5 * time.Second})
	dir := t.TempDir()
	sents := []*tbook.Sentence{{Src: "He put up with it."}, {Src: "He sat."}, {Src: "He sat."}}
	p := &Pipeline{Client: client, CacheDir: dir, Source: "en", BatchSize: 8, Concurrency: 2}
	st, err := p.prepareExpressions(context.Background(), sents)
	if err != nil {
		t.Fatal(err)
	}
	if st.Unique != 2 || st.Tagged != 1 || st.Tags != 1 || st.Rejected != 1 || st.Missing != 0 {
		t.Errorf("stats = %+v", st)
	}
	if got := p.exprOf["He put up with it."]; len(got) != 1 || got[0].Words != "put up with" {
		t.Errorf("tags = %+v", got)
	}
	before := calls.Load()
	p2 := &Pipeline{Client: client, CacheDir: dir, Source: "en", BatchSize: 8, Concurrency: 2}
	st2, err := p2.prepareExpressions(context.Background(), sents)
	if err != nil || calls.Load() != before || st2.Cached != 2 {
		t.Errorf("second run must be fully cached: calls %d→%d, stats %+v, err %v", before, calls.Load(), st2, err)
	}
}
