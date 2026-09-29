package translate

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/langcheck"
	"github.com/dimando/reader/converter/internal/tbook"
)

// The target-script guard on the translate and repair replies.
//
// A model can answer a whole batch in another language in well-formed JSON:
// every id present, the request a success, coverage 100%, q = 1.000, and the
// .tbook validates. Measured twice on deepseek-ai/DeepSeek-V4-Flash-0731 an
// en→ru batch of eight came back in fluent Chinese
// (bench-quality/reports/gonka-models-2026-09-19.md), and an en→ru glossary
// enforced on Latin targets put Russian into 55–73% of two overlays. Nothing
// downstream sees it: the alignment scores how the words line up with whatever
// text arrived, and lexcheck needs dictionary lookups a foreign script denies.
//
// So the check runs where the answer can still be thrown away: after a batch
// parses, before any of it is cached. One wrong-script sentence fails the whole
// batch, because the defect arrives per batch, not per sentence. The retry
// carries one added line naming the language; when the budget is spent the
// wrong-script sentences are NOT written, so the run leaves them untranslated
// instead of freezing another language into the cache as if it were the book.

// langNudge is the one line a retry adds after a batch came back outside the
// target's script. It also changes the request body, which matters on a gateway
// that answers an identical request from cache.
func langNudge(target string) string {
	return "\n\nOutput MUST be in " + LangName(target) + "."
}

// wrongLangNote is what a target's guard left behind, for the run summary. The
// count is a SET of source sentences: the round loop re-queues a refused
// sentence, so counting rejections would report one bad sentence three times.
type wrongLangNote struct {
	srcs     map[string]bool
	exSrc    string
	exText   string
	exPhase  string
	reported bool
}

// langGuardRetries is how many extra attempts a wrong-script batch gets. It is
// the client's own retry budget, so the guard costs what any other failed
// attempt costs and is sized by --max-retries like everything else.
func (p *Pipeline) langGuardRetries() int {
	if p.Client == nil {
		return 0
	}
	return max(0, p.Client.MaxRetries())
}

// langGuardOn reports whether the script tables can judge this pair at all. An
// unknown language disables the guard rather than guessing — a wrong guess
// would throw away a whole correct book.
func (p *Pipeline) langGuardOn(target string) bool {
	return langcheck.Known(p.Source, target)
}

// textInputs builds the user JSON for a translate or repair batch. ok is false
// when there is nothing to send (every repair item lost its raw translation).
func (p *Pipeline) textInputs(batch []item, target string, ph phase) (string, bool) {
	if ph == phaseTranslate {
		inputs := make([]translateInput, len(batch))
		for i, it := range batch {
			inputs[i] = translateInput{ID: strconv.Itoa(i + 1), Src: it.s.Src, Expr: p.exprOf[it.s.Src]}
		}
		b, err := jsonx.Marshal(inputs)
		return string(b), err == nil
	}
	inputs := make([]repairInput, 0, len(batch))
	for i, it := range batch {
		raw, ok := cache.Read(p.CacheDir, cache.TrKey(it.s.Src, p.Source, target, p.rawModel()))
		if !ok || strings.TrimSpace(raw.Text) == "" {
			continue // nothing to proofread; freezeUnrepaired handles the gap
		}
		in := repairInput{ID: strconv.Itoa(i + 1), Src: it.s.Src, Tr: strings.TrimSpace(raw.Text)}
		if p.prevOf != nil {
			in.Prev = p.prevOf(it.s)
		}
		inputs = append(inputs, in)
	}
	if len(inputs) == 0 {
		return "", false
	}
	b, err := jsonx.Marshal(inputs)
	return string(b), err == nil
}

// textPhaseBatch runs one translate- or repair-phase batch behind the guard.
func (p *Pipeline) textPhaseBatch(ctx context.Context, system string, batch []item,
	target string, ph phase, byID func(string) (item, bool)) error {

	userJSON, ok := p.textInputs(batch, target, ph)
	if !ok {
		return nil
	}
	sys := system
	if p.LangNudge {
		sys += langNudge(target)
	}
	for attempt := 0; ; attempt++ {
		out, err := p.Client.TranslateText(ctx, sys, userJSON)
		if err != nil {
			p.noteErr(err)
			return abortOnly(err)
		}
		bad := p.wrongScriptIDs(out, byID, target)
		if len(bad) == 0 || attempt >= p.langGuardRetries() {
			p.writeTextReplies(out, bad, byID, target, ph)
			return nil
		}
		sys = system + langNudge(target)
	}
}

// wrongScriptIDs returns the reply ids whose text is not in the target's
// script, in id order so the reported example is deterministic.
func (p *Pipeline) wrongScriptIDs(out map[string]string, byID func(string) (item, bool),
	target string) map[string]bool {

	if !p.langGuardOn(target) {
		return nil
	}
	var bad map[string]bool
	for id, text := range out {
		it, ok := byID(id)
		if !ok {
			continue
		}
		if text = strings.TrimSpace(text); text == "" {
			continue
		}
		if wrong, _ := langcheck.WrongScript(it.s.Src, text, p.Source, target); wrong {
			if bad == nil {
				bad = map[string]bool{}
			}
			bad[id] = true
		}
	}
	return bad
}

// writeTextReplies caches every returned sentence the guard did not reject and
// records what it did reject. A rejected sentence is left with no cache entry
// at all: the round loop re-queues it, and if that fails too the run ships it
// untranslated rather than shipping another language.
func (p *Pipeline) writeTextReplies(out map[string]string, bad map[string]bool,
	byID func(string) (item, bool), target string, ph phase) {

	for id, text := range out {
		it, ok := byID(id)
		if !ok {
			continue
		}
		if text = strings.TrimSpace(text); text == "" || suspiciousTranslation(text) {
			continue // dropped/artifact output — retried next round
		}
		if bad[id] {
			continue
		}
		if ph == phaseTranslate {
			_ = cache.Write(p.CacheDir, cache.TrKey(it.s.Src, p.Source, target, p.rawModel()),
				tbook.Translation{Text: text})
			// The proofread text and the final aligned entry are both DERIVED
			// from the raw translation just replaced — drop them or the later
			// phases would skip the sentence and FillFromCache would keep serving
			// the stale text (this is how escalated sentences used to ship with
			// the old model's content).
			if p.Repair {
				cache.Remove(p.CacheDir, cache.RepairKey(it.s.Src, p.Source, target, p.repairModel()))
			}
			cache.Remove(p.CacheDir, it.key)
			continue
		}
		_ = cache.Write(p.CacheDir, cache.RepairKey(it.s.Src, p.Source, target, p.repairModel()),
			tbook.Translation{Text: text})
		cache.Remove(p.CacheDir, it.key) // stale alignment of the pre-proofread text
	}
	if len(bad) == 0 {
		return
	}
	ids := make([]string, 0, len(bad))
	for id := range bad {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	srcs := make([]string, 0, len(ids))
	for _, id := range ids {
		if it, ok := byID(id); ok {
			srcs = append(srcs, it.s.Src)
		}
	}
	if len(srcs) == 0 {
		return
	}
	p.noteWrongLang(target, srcs, out[ids[0]], ph)
}

// noteWrongLang records the sentences the guard refused for a target and keeps
// one verbatim example for the run summary.
func (p *Pipeline) noteWrongLang(target string, srcs []string, text string, ph phase) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.wrongLang == nil {
		p.wrongLang = map[string]*wrongLangNote{}
	}
	note := p.wrongLang[target]
	if note == nil {
		note = &wrongLangNote{srcs: map[string]bool{}}
		p.wrongLang[target] = note
	}
	for _, src := range srcs {
		note.srcs[src] = true
	}
	if note.exSrc == "" && len(srcs) > 0 {
		note.exSrc, note.exText, note.exPhase = srcs[0], strings.TrimSpace(text), ph.String()
	}
}

// reportWrongLang prints what the guard refused for one target, with one
// example. The count stays for WrongLangCount; only the printing is once.
func (p *Pipeline) reportWrongLang(target string) {
	p.mu.Lock()
	note := p.wrongLang[target]
	if note != nil && !note.reported {
		note.reported = true
	} else {
		note = nil
	}
	p.mu.Unlock()
	if note == nil || len(note.srcs) == 0 {
		return
	}
	fmt.Printf("[%s] target-script guard: %d sentence(s) refused as wrong-language "+
		"and left untranslated (first seen in the %s phase)\n", target, len(note.srcs), note.exPhase)
	if note.exSrc != "" {
		fmt.Printf("[%s]   e.g. %q -> %q\n", target, clipLine(note.exSrc), clipLine(note.exText))
	}
}

// WrongLangCount is how many sentences the guard refused for a target in this
// run, for the caller's own summary.
func (p *Pipeline) WrongLangCount(target string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if note := p.wrongLang[target]; note != nil {
		return len(note.srcs)
	}
	return 0
}

func clipLine(s string) string {
	const maxLine = 100
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxLine {
		return string(r[:maxLine]) + "…"
	}
	return s
}

// DropForeignRenderings removes the glossary entries whose rendering is in a
// script the target does not write — the entries that teach the leak. Used to
// re-translate a wrong-language cell without the prompt that caused it.
func DropForeignRenderings(entries []GlossEntry, target string) []GlossEntry {
	out := make([]GlossEntry, 0, len(entries))
	for _, e := range entries {
		if langcheck.OutsideScript(e.Tgt, target) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// SuspectLexicon is the part of lexcheck.Dict the suspect gate needs.
type SuspectLexicon interface {
	Covered(srcWord string) bool
	Supports(srcWord, tgt string) bool
	NearMiss(srcWord, tgt string) (string, bool)
}

// GateGlossary applies the entry gates to a glossary from ANY source — freshly
// built, or loaded verbatim from the user-editable sidecar, which nothing gated
// before. It drops what must not be enforced and returns the dropped entries
// with their reason so the caller can say so out loud.
//
// It also marks the suspect class: a one-word rendering the target lexicon does
// not support while it does know a spelling one or two letters away. Those
// entries stay enforced — the lexicon is not the authority on a book's own
// terms — but they carry the near spelling, which takes them out of the
// proofreader's "never change these" list. The pipeline never corrects them
// itself: a glossary is a decision, and a dictionary does not get to overrule
// one silently.
func GateGlossary(entries []GlossEntry, source, target string, lex SuspectLexicon) (kept []GlossEntry, dropped []GlossDrop) {
	kept = make([]GlossEntry, 0, len(entries))
	for _, e := range entries {
		if why := rejectGlossEntry(e, source, target); why != "" {
			dropped = append(dropped, GlossDrop{Src: e.Src, Tgt: e.Tgt, Why: why})
			continue
		}
		e.Suspect = ""
		if lex != nil && len(strings.Fields(e.Tgt)) == 1 && isLowerFirst(strings.TrimSpace(e.Src)) &&
			lex.Covered(e.Src) {
			if near, ok := lex.NearMiss(e.Src, e.Tgt); ok {
				e.Suspect = near
			}
		}
		kept = append(kept, e)
	}
	return kept, dropped
}

// GlossDrop is one entry the gates refused, with the reason.
type GlossDrop struct {
	Src string `json:"src"`
	Tgt string `json:"tgt"`
	Why string `json:"why"`
}

// ReportGlossDrops prints one line per REASON with an example, not one line per
// entry: on en→de the "rendering equals the source term" rule alone fires 30
// times in a two-chapter book (every English name a German text keeps), which
// would bury everything else the run says.
func ReportGlossDrops(target string, drops []GlossDrop) {
	if len(drops) == 0 {
		return
	}
	order := make([]string, 0, 4)
	byWhy := map[string][]GlossDrop{}
	for _, d := range drops {
		if _, seen := byWhy[d.Why]; !seen {
			order = append(order, d.Why)
		}
		byWhy[d.Why] = append(byWhy[d.Why], d)
	}
	for _, why := range order {
		list := byWhy[why]
		fmt.Printf("Glossary [%s]: dropped %d entr%s — %s, e.g. %q → %q\n",
			target, len(list), map[bool]string{true: "y", false: "ies"}[len(list) == 1],
			why, list[0].Src, list[0].Tgt)
	}
}

// Suspects lists the entries carrying a near spelling, for the sidecar and the
// run report.
func Suspects(entries []GlossEntry) []GlossEntry {
	var out []GlossEntry
	for _, e := range entries {
		if e.Suspect != "" {
			out = append(out, e)
		}
	}
	return out
}
