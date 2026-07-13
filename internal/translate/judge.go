// The semantic verification pass (spec §10.4): an independent LLM judge reads
// each sentence's source, translation, and word-alignment pairs, and flags
// mistranslations and drifted/wrong word-mappings — the failures structural
// validation and coverage are blind to. Flagged sources feed --invalidate (or
// --judge-invalidate) so the next run redoes exactly those.
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
	"sync/atomic"

	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/progress"
	"github.com/dimando/reader/converter/internal/tbook"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"
)

// judgeVersion namespaces judge verdicts on disk; bump when the judge prompt
// or verdict semantics change. j3 = j1's alignment rules plus an explicit
// verdict-shape example (cheap models otherwise answer with an array) and
// "lenient about granularity". An intermediate j2 that flagged only
// reader-visible defects zeroed the cheap judge's recall — leniency wording
// must not give a weak judge an excuse to approve everything.
const judgeVersion = "j3"

// Verdict is the judge's decision for one (sentence, target).
type Verdict struct {
	OK  bool   `json:"ok"`
	Why string `json:"why,omitempty"`
}

// JudgeReport summarizes a judge pass.
type JudgeReport struct {
	Checked     int
	Flagged     int
	Unjudged    int      // sentences the judge never returned a verdict for
	FlaggedSrcs []string // unique source sentences to re-translate
	Reasons     map[string]int
}

// judgeItem is one sentence as sent to the judge.
type judgeItem struct {
	ID    string      `json:"id"`
	Src   string      `json:"src"`
	Tr    string      `json:"tr"`
	Pairs [][2]string `json:"pairs"`
}

// Judge runs the semantic verification pass over every unique sentence with a
// non-empty translation, per target. Verdicts are cached in cacheDir (keyed by
// judge model + src + translation + alignment), so re-runs only judge what
// changed. jc is a client configured with the judge model. prog, when non-nil,
// receives per-batch progress events (--progress-file).
func Judge(ctx context.Context, jc *Client, cacheDir, source string, targets []string,
	sentences []*tbook.Sentence, batchSize, concurrency int, prog *progress.Sink) (JudgeReport, error) {

	rep := JudgeReport{Reasons: map[string]int{}}
	flagged := map[string]bool{}

	for _, target := range targets {
		type pending struct {
			key  string
			item judgeItem
			src  string
		}
		var todo []pending
		seen := map[string]bool{}
		for _, s := range sentences {
			tr, ok := s.Tr[target]
			if !ok || tr.Text == "" {
				continue
			}
			pairs := alignPairs(s, tr)
			key := judgeKey(jc.Model(), source, target, s.Src, tr.Text, pairs)
			if seen[key] {
				continue
			}
			seen[key] = true
			rep.Checked++
			if v, ok := readVerdict(cacheDir, key); ok {
				tally(&rep, flagged, s.Src, v)
				continue
			}
			todo = append(todo, pending{key: key, item: judgeItem{ID: key, Src: s.Src, Tr: tr.Text, Pairs: pairs}, src: s.Src})
		}
		if len(todo) == 0 {
			continue
		}

		var batches [][]pending
		for i := 0; i < len(todo); i += batchSize {
			batches = append(batches, todo[i:min(i+batchSize, len(todo))])
		}
		bar := progressbar.Default(int64(len(batches)), "judge "+target)
		var jdone atomic.Int64
		step := func() { // one batch processed: human bar + machine progress
			_ = bar.Add(1)
			prog.Update("judge", target, int(jdone.Add(1)), len(batches))
		}
		sys := judgeSystemPrompt(LangName(source), LangName(target))
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(max(1, concurrency))
		for _, batch := range batches {
			g.Go(func() error {
				// Short per-batch ids ("1", "2", …): a cheap model reliably
				// echoes those, where 64-hex cache keys get mangled/dropped.
				items := make([]judgeItem, len(batch))
				for i, p := range batch {
					items[i] = p.item
					items[i].ID = strconv.Itoa(i + 1)
				}
				userJSON, err := jsonx.Marshal(items)
				if err != nil {
					return nil
				}
				var raw json.RawMessage
				if err := jc.ChatJSON(WithPhase(gctx, "judge"), sys, string(userJSON), &raw); err != nil {
					step()
					return abortOnly(err) // usage limit aborts; else left unjudged, reported below
				}
				out := parseVerdicts(raw)
				for id, v := range out {
					n, err := strconv.Atoi(strings.TrimSpace(id))
					if err != nil || n < 1 || n > len(batch) {
						continue
					}
					_ = writeVerdict(cacheDir, batch[n-1].key, v)
				}
				step()
				return nil
			})
		}
		gerr := g.Wait()
		_ = bar.Finish()
		prog.Final("judge", target, int(jdone.Load()), len(batches))
		if gerr != nil {
			return rep, gerr
		}
		if err := ctx.Err(); err != nil {
			return rep, err
		}

		for _, p := range todo {
			if v, ok := readVerdict(cacheDir, p.key); ok {
				tally(&rep, flagged, p.src, v)
			} else {
				rep.Unjudged++
			}
		}
	}

	for src := range flagged {
		rep.FlaggedSrcs = append(rep.FlaggedSrcs, src)
	}
	return rep, nil
}

// parseVerdicts decodes the judge reply tolerantly: the requested object shape
// {"1":{"ok":true}, …}, or the array shape [{"id":"1","ok":true}, …] some
// models emit regardless of instructions (ids arrive as strings or numbers).
func parseVerdicts(raw []byte) map[string]Verdict {
	var m map[string]Verdict
	if json.Unmarshal(raw, &m) == nil {
		return m
	}
	var arr []struct {
		ID any `json:"id"`
		Verdict
	}
	if json.Unmarshal(raw, &arr) == nil {
		m = make(map[string]Verdict, len(arr))
		for _, e := range arr {
			m[strings.Trim(fmt.Sprint(e.ID), `"`)] = e.Verdict
		}
		return m
	}
	return nil
}

func tally(rep *JudgeReport, flagged map[string]bool, src string, v Verdict) {
	if v.OK {
		return
	}
	rep.Flagged++
	if !flagged[src] {
		flagged[src] = true
	}
	why := strings.TrimSpace(v.Why)
	if why == "" {
		why = "unspecified"
	}
	if len(why) > 60 {
		why = why[:60]
	}
	rep.Reasons[why]++
}

// alignPairs renders a sentence's alignment as [target fragment, source words]
// pairs for the judge. Chunks with no source mapping appear with "" so inserted
// words are visible too.
func alignPairs(s *tbook.Sentence, tr tbook.Translation) [][2]string {
	srcRunes := []rune(s.Src)
	trRunes := []rune(tr.Text)
	wordText := func(i int) string {
		if i < 0 || i >= len(s.Words) {
			return ""
		}
		a, b := s.Words[i][0], s.Words[i][1]
		if a < 0 || b > len(srcRunes) || a >= b {
			return ""
		}
		return string(srcRunes[a:b])
	}
	pairs := make([][2]string, 0, len(tr.Align))
	for _, c := range tr.Align {
		a, b := c.T[0], c.T[1]
		if a < 0 {
			a = 0
		}
		if b > len(trRunes) {
			b = len(trRunes)
		}
		if a >= b {
			continue
		}
		var ws []string
		for _, wi := range c.W {
			if w := wordText(wi); w != "" {
				ws = append(ws, w)
			}
		}
		pairs = append(pairs, [2]string{string(trRunes[a:b]), strings.Join(ws, " ")})
	}
	return pairs
}

func judgeKey(model, source, target, src, trText string, pairs [][2]string) string {
	pj, _ := json.Marshal(pairs)
	raw := judgeVersion + "|" + model + "|" + source + "|" + target + "|" + src + "|" + trText + "|" + string(pj)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func verdictPath(dir, key string) string {
	return filepath.Join(dir, "judge-"+key+".json")
}

func readVerdict(dir, key string) (Verdict, bool) {
	b, err := os.ReadFile(verdictPath(dir, key))
	if err != nil {
		return Verdict{}, false
	}
	var v Verdict
	if json.Unmarshal(b, &v) != nil {
		return Verdict{}, false
	}
	return v, true
}

func writeVerdict(dir, key string, v Verdict) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(verdictPath(dir, key), b, 0o644)
}

// VerdictFor returns the cached judge verdict for a sentence's CURRENT
// translation+alignment, if one exists (used by offline tools such as lexeval).
func VerdictFor(cacheDir, judgeModel, source, target string, s *tbook.Sentence) (Verdict, bool) {
	tr, ok := s.Tr[target]
	if !ok || tr.Text == "" {
		return Verdict{}, false
	}
	key := judgeKey(judgeModel, source, target, s.Src, tr.Text, alignPairs(s, tr))
	return readVerdict(cacheDir, key)
}

// WriteFlagged writes the flagged source sentences as a JSON array — the exact
// format --invalidate consumes.
func WriteFlagged(path string, srcs []string) error {
	b, err := jsonx.MarshalIndent(srcs, "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func judgeSystemPrompt(sourceName, targetName string) string {
	r := strings.NewReplacer("{SRC}", sourceName, "{TGT}", targetName)
	return r.Replace(`You verify {SRC}→{TGT} translations for a language-learning reader where readers tap a
{SRC} word to see which {TGT} word(s) translate it.

You receive a JSON array of items {id, src, tr, pairs}:
- src: the {SRC} sentence; tr: its {TGT} translation
- pairs: the word alignment as ["<{TGT} fragment>", "<{SRC} word(s)>"] mappings ("" = inserted word)

For EACH item, decide:
1) TRANSLATION — is tr a faithful, natural {TGT} rendering of src? (Mistranslation, omission of
   meaning, or leftover {SRC} words → not ok. Free but faithful literary phrasing → ok.)
2) ALIGNMENT — is each pair mapped BY MEANING? ({TGT} word mapped to a {SRC} word that does not
   translate it — e.g. positional drift, where words are paired by position instead of meaning —
   → not ok. Unmapped words, function words attached to a neighboring content word, and loose
   but related renderings → ok.)

Be strict about real errors, lenient about style and granularity. Reply with ONLY a JSON
object mapping each "id" (exact string) to a verdict — EXACTLY this shape:
{"1":{"ok":true},"2":{"ok":false,"why":"drift"}}
(why is a short reason: mistranslation|drift|wrong-mapping|other). Not an array.
No code fences, no commentary. Judge EVERY item.`)
}
