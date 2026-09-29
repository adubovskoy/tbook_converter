package translate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/tbook"
	"golang.org/x/sync/errgroup"
)

// ExprPromptVersion keys the source-side EXPRESSION pass (--expressions): its
// prompt and the span gates below. Bumping it re-tags every sentence and, via
// TranslateVariant, re-translates every tagged run. e1 = first version.
const ExprPromptVersion = "e1"

// exprBatchFactor: the expression pass sends BatchSize*exprBatchFactor
// sentences per call. Its output is a few tokens per sentence, so it can take
// larger batches than translation.
const exprBatchFactor = 2

// Span gates. An expression is at least two words (one word is vocabulary,
// which the glossary and the aligner handle) and a phrase, not a clause.
const (
	exprMinWords   = 2
	exprMaxWords   = 12
	exprMaxPerSent = 3
	exprMaxMeaning = 100 // runes
)

// Expr is one non-compositional expression found in a source sentence: its
// exact source span and a plain-English gloss of what it means there.
type Expr struct {
	Words   string `json:"words"`
	Meaning string `json:"meaning"`
}

// TranslateVariant is the cache-key suffix of the translate-prompt variant:
// "" for the production prompt, so default runs keep their cache. The
// expression-tagged prompt and the no-mirror prompt produce different text and
// must never answer for the production one, or for each other.
func TranslateVariant(expressions, noMirror bool) string {
	s := ""
	if expressions {
		s += "+x:" + ExprPromptVersion
	}
	if noMirror {
		s += "+nm"
	}
	return s
}

func exprSystemPrompt(sourceName string) string {
	return strings.ReplaceAll(`You mark NON-COMPOSITIONAL expressions in {SRC} sentences for a literary translator,
who must render them by meaning instead of word by word.

You receive a JSON array of sentences, each {id, src}.

For EACH sentence, list the expressions in src whose meaning is NOT the sum of their words —
where a word-by-word translation would be wrong, odd or unidiomatic:
- idioms and figurative phrases ("spill the beans", "a dark horse");
- phrasal verbs with a non-literal meaning ("put up with", "turn out to be");
- fixed expressions and set phrases ("by and large", "all the same", "for the life of me");
- culturally bound phrases (period customs, games, social formulas) whose literal
  rendering would mislead a reader in another culture.
Do NOT list: literal phrases, names, titles, single words, a verb with a literal particle
("sat down", "went out"), or ordinary collocations any translator renders literally —
phrases whose words keep their everyday meaning ("pay the bill", "very fond of",
"it is serious") are not expressions. When unsure, leave it out.

Each expression is {"words": …, "meaning": …}:
- "words": the expression copied EXACTLY from src — same spelling, case and punctuation,
  one contiguous stretch of src. If other words interrupt it, give the shortest contiguous
  stretch that contains the whole expression.
- "meaning": what it means HERE, in plain {SRC}, at most eight words.
MOST SENTENCES CONTAIN NONE — give them an empty list. At most three per sentence.

Reply with ONLY a single JSON object mapping each "id" (exact string) to its list, e.g.
{"1":[],"2":[{"words":"put up with","meaning":"tolerate"}]}. No code fences, no commentary.`,
		"{SRC}", sourceName)
}

// validExprs keeps the model's spans that pass the gates: a verbatim,
// word-bounded substring of src, 2..12 words, with a non-empty meaning. The
// words are trimmed of surrounding space and edge punctuation first — a model
// that includes the sentence's trailing comma still named the right span.
// Duplicates are dropped and at most exprMaxPerSent survive. rejected counts
// the spans dropped by a gate (not by the cap).
func validExprs(src string, in []Expr) (out []Expr, rejected int) {
	seen := map[string]bool{}
	for _, e := range in {
		w := strings.TrimFunc(e.Words, func(r rune) bool {
			return unicode.IsSpace(r) || strings.ContainsRune(`,.;:!?"“”«»`, r)
		})
		m := strings.TrimSpace(e.Meaning)
		n := len(strings.Fields(w))
		if w == "" || m == "" || n < exprMinWords || n > exprMaxWords || !wordBounded(src, w) {
			rejected++
			continue
		}
		if seen[w] {
			continue
		}
		seen[w] = true
		if r := []rune(m); len(r) > exprMaxMeaning {
			m = string(r[:exprMaxMeaning])
		}
		if len(out) < exprMaxPerSent {
			out = append(out, Expr{Words: w, Meaning: m})
		}
	}
	return out, rejected
}

// wordBounded reports whether w occurs in src with no letter or digit
// directly before or after it ("up with" is not in "setup without").
func wordBounded(src, w string) bool {
	for from := 0; from <= len(src)-len(w); {
		i := strings.Index(src[from:], w)
		if i < 0 {
			return false
		}
		i += from
		before, after := rune(0), rune(0)
		if i > 0 {
			r := []rune(src[:i])
			before = r[len(r)-1]
		}
		if j := i + len(w); j < len(src) {
			after = []rune(src[j:])[0]
		}
		if !isWordRune(before) && !isWordRune(after) {
			return true
		}
		from = i + 1
	}
	return false
}

func isWordRune(r rune) bool { return r != 0 && (unicode.IsLetter(r) || unicode.IsDigit(r)) }

// exprEntry is the on-disk cache record of one sentence's tags. src is kept
// for inspection; the key already binds it.
type exprEntry struct {
	Src  string `json:"src"`
	Expr []Expr `json:"expr"`
}

func exprPath(dir, model, source, src string) string {
	sum := sha256.Sum256([]byte(ExprPromptVersion + "|expr|" + model + "|" + source + "|" + src))
	return filepath.Join(dir, "expr-"+hex.EncodeToString(sum[:])+".json")
}

func readExpr(path string) ([]Expr, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var e exprEntry
	if json.Unmarshal(b, &e) != nil {
		return nil, false
	}
	return e.Expr, true
}

// parseExprReply decodes one reply per id, tolerating a malformed value for one
// id (it is simply missing, and retried) instead of losing the whole batch.
func parseExprReply(raw map[string]json.RawMessage) map[string][]Expr {
	out := make(map[string][]Expr, len(raw))
	for id, v := range raw {
		var list []Expr
		if json.Unmarshal(v, &list) == nil {
			out[id] = list
			continue
		}
		var one Expr // a lone object instead of a one-element list
		if json.Unmarshal(v, &one) == nil && one.Words != "" {
			out[id] = []Expr{one}
		}
	}
	return out
}

// ExprStats summarizes one expression pass.
type ExprStats struct {
	Unique, Cached, Tagged, Tags, Rejected, Missing int
}

// prepareExpressions runs the expression pass over the unique sources (cached
// per sentence, so it is paid once per book, not per target) and loads the
// lookup the translate prompt reads. A sentence the pass never returned is
// translated without tags; it is not cached, so the next run asks again.
func (p *Pipeline) prepareExpressions(ctx context.Context, sentences []*tbook.Sentence) (ExprStats, error) {
	var st ExprStats
	model := p.Client.Model()
	var pending []string
	seen := map[string]bool{}
	tags := map[string][]Expr{}
	for _, s := range sentences {
		if seen[s.Src] || strings.TrimSpace(s.Src) == "" {
			continue
		}
		seen[s.Src] = true
		st.Unique++
		if e, ok := readExpr(exprPath(p.CacheDir, model, p.Source, s.Src)); ok {
			st.Cached++
			tags[s.Src] = e
			continue
		}
		pending = append(pending, s.Src)
	}
	if len(pending) > 0 {
		t0 := time.Now()
		fmt.Printf("Expressions: tagging %d sentences (%d cached) ...\n", len(pending), st.Cached)
		size := max(1, p.BatchSize*exprBatchFactor)
		sys := exprSystemPrompt(LangName(p.Source))
		var mu sync.Mutex
		for round := 0; round < maxRounds && len(pending) > 0; round++ {
			g, gctx := errgroup.WithContext(WithPhase(ctx, "expr"))
			g.SetLimit(max(1, p.Concurrency))
			for i := 0; i < len(pending); i += size {
				batch := pending[i:min(i+size, len(pending))]
				g.Go(func() error {
					in := make([]translateInput, len(batch))
					for j, src := range batch {
						in[j] = translateInput{ID: strconv.Itoa(j + 1), Src: src}
					}
					b, err := jsonx.Marshal(in)
					if err != nil {
						return nil
					}
					var raw map[string]json.RawMessage
					if err := p.Client.ChatJSON(gctx, sys, string(b), &raw); err != nil {
						p.noteErr(err)
						return abortOnly(err)
					}
					for id, list := range parseExprReply(raw) {
						n, err := strconv.Atoi(strings.TrimSpace(id))
						if err != nil || n < 1 || n > len(batch) {
							continue
						}
						src := batch[n-1]
						ok, rej := validExprs(src, list)
						eb, err := jsonx.Marshal(exprEntry{Src: src, Expr: ok})
						if err != nil {
							continue
						}
						if os.MkdirAll(p.CacheDir, 0o755) == nil &&
							os.WriteFile(exprPath(p.CacheDir, model, p.Source, src), eb, 0o644) == nil {
							mu.Lock()
							tags[src] = ok
							st.Rejected += rej
							mu.Unlock()
						}
					}
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				return st, err
			}
			if err := ctx.Err(); err != nil {
				return st, err
			}
			var still []string
			for _, src := range pending {
				if _, ok := tags[src]; !ok {
					still = append(still, src)
				}
			}
			pending = still
		}
		fmt.Printf("Expressions: pass took %s\n", time.Since(t0).Round(time.Second))
	}
	st.Missing = len(pending)
	for _, e := range tags {
		if len(e) > 0 {
			st.Tagged++
			st.Tags += len(e)
		}
	}
	p.exprOf = tags
	fmt.Printf("Expressions: %d unique sentences, %d tagged (%.1f%%), %d tags (%.1f per 100 sentences), "+
		"%d spans rejected, %d sentences untagged after %d rounds\n",
		st.Unique, st.Tagged, pct(st.Tagged, st.Unique), st.Tags, pct(st.Tags, st.Unique),
		st.Rejected, st.Missing, maxRounds)
	return st, nil
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}
