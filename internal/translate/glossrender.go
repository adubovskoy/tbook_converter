package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/tbook"
)

// The render pass: one cheap model call per batch of mined candidates that
// filters them and produces the target rendering, the entity kind, and — for a
// named individual — the gender. Measured at $0.01 and 7 requests for a
// 15k-sentence novel.

const (
	glossRenderBatch   = 50
	glossRenderWorkers = 4
	// genderMinEvidence is how many pronoun observations the miner needs before
	// it may veto the model's gender tag. Below it the miner is silent, not
	// contradicting.
	genderMinEvidence = 15
	// genderMinerOnly* accept a gender the model did not offer. Swept on two
	// books: lowering the evidence bar from 40 to 15 adds tags and no errors.
	genderMinerOnlyEvidence   = 15
	genderMinerOnlyConfidence = 0.8
)

// genderMarkingTargets are the target languages where a character's gender
// changes the words around them, so a [male]/[female] tag has something to do:
// past-tense verbs and adjectives in the Slavic group, adjectives, participles
// and articles in the Romance one. Everything else (en, zh, ja, ko, tr, fi, hu)
// carries the annotation in the sidecar but never renders it.
var genderMarkingTargets = words("ru uk be pl cs sk sl bg sr hr mk es fr it pt ca ro ar he hi")

// GenderMarkingTarget reports whether gender annotation reaches the prompt for
// this target.
func GenderMarkingTarget(target string) bool {
	return genderMarkingTargets[strings.ToLower(strings.TrimSpace(target))]
}

// glossRenderReply is one rendered candidate. Term is echoed back on purpose:
// mapping the reply by id alone is not safe — measured on Revelation Space, 12
// of 128 entries glued a rendering to the WRONG candidate ("aside → Новый
// Комусо", "sprang → Мадемуазель"), the same positional drift the align pass
// guards against with its numbered-echo contract. An entry whose echo does not
// match its id is discarded.
type glossRenderReply struct {
	Term   string `json:"term"`
	Tgt    string `json:"tgt"`
	Kind   string `json:"kind"`
	Gender string `json:"gender"`
}

func glossRenderSystemPrompt(sourceName, targetName string, gender bool) string {
	r := strings.NewReplacer("{SRC}", sourceName, "{TGT}", targetName)
	s := r.Replace(`You prepare a translation glossary for a book ({SRC} → {TGT}).

You receive a JSON array of CANDIDATE terms mined from the book by frequency, each
{id, term, freq, examples}.

KEEP every proper noun unconditionally — characters, places, organisations, ships,
products. A name has NO single obvious {TGT} rendering: it can be transliterated
several ways, and the reader must meet the same one in every chapter.
KEEP invented or domain terminology and titles.
DROP only ordinary {SRC} words and function words that happen to be capitalised or
frequent ("into", "going", "Real", "Good").

For each KEPT candidate give the {TGT} rendering in its BASE form (nominative
singular for nouns and names) — the form a reader would meet in a dictionary. Write
it in the script of {TGT}; never leave the {SRC} spelling as the rendering.

Also classify each kept candidate:
  "kind": "person" | "place" | "org" | "thing"`)
	if gender {
		s += r.Replace(`
  "gender": for a NAMED INDIVIDUAL ONLY — one specific character — and only when
  the examples or the name itself make it certain: "m" or "f". Omit gender for a
  category of people ("a Catholic", "a Mongol", "the crew"), for a name that could
  belong to more than one character, and whenever the evidence is not there. A
  wrong gender is worse than no gender: it forces the wrong agreement on every
  sentence about that character.`)
	}
	s += r.Replace(`

Reply with ONLY a JSON object mapping each kept "id" (exact string) to
{"term":"…","tgt":"…","kind":"…"`)
	if gender {
		s += `,"gender":"m|f"`
	}
	s += `}, where "term" is the candidate's term COPIED EXACTLY from the input. Drop a
candidate by omitting its id. No code fences, no commentary.

The echoed "term" is checked against the id: an entry whose term does not match is
thrown away, so copy it, do not retype it from memory.`
	return s
}

// buildMinedGlossary mines candidates locally and renders them in one pass.
// Returns nil (and no error) when there is nothing to mine.
func buildMinedGlossary(ctx context.Context, c *Client, sentences []*tbook.Sentence,
	o GlossaryBuild) ([]GlossEntry, error) {

	cands := mineGlossCandidates(sentences, o.Source, o.Lexicon)
	if len(cands) == 0 {
		return nil, nil
	}
	rendered, dropped, err := renderCandidates(ctx, c, cands, o)
	if err != nil {
		return nil, err
	}
	var gender map[string]genderEvidence
	if o.wantGender() {
		names := make(map[string]bool, len(cands))
		for _, cd := range cands {
			if cd.Kind == "name" {
				names[cd.Term] = true
			}
		}
		gender = mineGender(sentences, o.Source, names)
	}
	entries, tagged := mergeGender(cands, rendered, gender, o.wantGender())
	msg := fmt.Sprintf("Glossary [%s]: mined %d candidates, kept %d", o.Target, len(cands), len(entries))
	if dropped > 0 {
		msg += fmt.Sprintf(", dropped %d unusable replies", dropped)
	}
	if o.wantGender() {
		msg += fmt.Sprintf(", %d carry gender", tagged)
	}
	fmt.Println(msg)
	return entries, nil
}

// renderCandidates runs the render pass. Returns term -> reply (keyed by the
// candidate's own term, so a mismatched echo simply never lands) and how many
// replies were discarded for a bad echo.
func renderCandidates(ctx context.Context, c *Client, cands []glossCandidate,
	o GlossaryBuild) (map[string]glossRenderReply, int, error) {

	sys := glossRenderSystemPrompt(LangName(o.Source), LangName(o.Target), o.wantGender())
	type batch struct {
		items []glossCandidate
		base  int
	}
	var batches []batch
	for i := 0; i < len(cands); i += glossRenderBatch {
		end := min(i+glossRenderBatch, len(cands))
		batches = append(batches, batch{items: cands[i:end], base: i})
	}

	var (
		mu      sync.Mutex
		out     = make(map[string]glossRenderReply, len(cands))
		dropped int
		firstEr error
		wg      sync.WaitGroup
	)
	sem := make(chan struct{}, glossRenderWorkers)
	for _, b := range batches {
		wg.Add(1)
		go func(b batch) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			type item struct {
				ID       string   `json:"id"`
				Term     string   `json:"term"`
				Freq     int      `json:"freq"`
				Examples []string `json:"examples,omitempty"`
			}
			items := make([]item, len(b.items))
			for i, cd := range b.items {
				items[i] = item{ID: strconv.Itoa(b.base + i), Term: cd.Term,
					Freq: cd.Freq, Examples: cd.Examples}
			}
			userJSON, err := jsonx.Marshal(items)
			if err != nil {
				return
			}
			var raw map[string]json.RawMessage
			if err := c.ChatJSON(WithPhase(ctx, "glossary"), sys, string(userJSON), &raw); err != nil {
				mu.Lock()
				if firstEr == nil {
					firstEr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for id, msg := range raw {
				n, convErr := strconv.Atoi(strings.TrimSpace(id))
				if convErr != nil || n < 0 || n >= len(cands) {
					dropped++
					continue
				}
				rep, ok := parseRenderReply(msg)
				if !ok || strings.TrimSpace(rep.Tgt) == "" {
					continue
				}
				// Trust the id only when the echoed term matches it.
				if echo := strings.TrimSpace(rep.Term); echo != "" &&
					!strings.EqualFold(echo, cands[n].Term) {
					dropped++
					continue
				}
				rep.Tgt = strings.TrimSpace(rep.Tgt)
				out[cands[n].Term] = rep
			}
		}(b)
	}
	wg.Wait()
	// A partial result is still worth having: one failed batch costs its 50
	// candidates, not the pass. Only a total failure is an error.
	if len(out) == 0 && firstEr != nil {
		return nil, dropped, firstEr
	}
	return out, dropped, nil
}

// parseRenderReply accepts both the object form and a bare rendering string,
// which cheap models fall back to.
func parseRenderReply(msg json.RawMessage) (glossRenderReply, bool) {
	var rep glossRenderReply
	if err := json.Unmarshal(msg, &rep); err == nil {
		return rep, true
	}
	var s string
	if err := json.Unmarshal(msg, &s); err == nil {
		return glossRenderReply{Tgt: s}, true
	}
	return rep, false
}

// mergeGender turns rendered candidates into entries, combining the two gender
// sources. The combining rule is a veto by DIRECTION, not by spread: an earlier
// version rejected any name whose pronoun evidence was split, which threw away
// real characters (Pascale, 53 observations at 0.57, is female and the model
// said so). What a disagreement in direction means is that the surname belongs
// to characters of both genders — Bancroft, where a wrong tag would have forced
// masculine agreement on 124 subject-plus-past-verb pairs.
func mergeGender(cands []glossCandidate, rendered map[string]glossRenderReply,
	gender map[string]genderEvidence, wantGender bool) (entries []GlossEntry, tagged int) {

	entries = make([]GlossEntry, 0, len(rendered))
	for _, cd := range cands {
		rep, ok := rendered[cd.Term]
		if !ok {
			continue
		}
		e := GlossEntry{Src: cd.Term, Tgt: rep.Tgt, Kind: rep.Kind}
		if wantGender {
			if g := acceptGender(cd.Term, rep, gender); g != "" {
				e.Gender = g
				tagged++
			}
		}
		entries = append(entries, e)
	}
	return entries, tagged
}

// acceptGender applies the validated combination rule and returns "" when the
// gender must stay unstated.
func acceptGender(term string, rep glossRenderReply, gender map[string]genderEvidence) string {
	if rep.Kind != "person" {
		return "" // only a named individual may carry one
	}
	ev, haveEv := gender[term]
	if !haveEv {
		// A full name ("Ilia Volyova") carries no evidence of its own — the
		// miner keys single tokens — so consult the surname, which is where a
		// shared-surname veto lives.
		if i := strings.LastIndex(term, " "); i > 0 {
			ev, haveEv = gender[term[i+1:]]
		}
	}
	switch rep.Gender {
	case "m", "f":
		if haveEv && ev.Evidence >= genderMinEvidence && ev.Gender != rep.Gender {
			return "" // the source itself points the other way
		}
		return rep.Gender
	}
	// The model declined; the miner may still be sure enough on its own.
	if haveEv && ev.Evidence >= genderMinerOnlyEvidence &&
		ev.Confidence >= genderMinerOnlyConfidence {
		return ev.Gender
	}
	return ""
}

// acceptGlossEntry is the deterministic gate every entry passes before it is
// enforced, whichever pass produced it. Each rule closes a measured failure:
//
//   - an empty or self-equal rendering ("learnt → learnt") teaches the model to
//     leave source words in the translation: Latin-script leakage into Russian
//     rose from 1.5% of sentences to 3.2% on the back of five such entries;
//   - a rendering with no letter of the target's own script is the same failure
//     in its pure form ("JacSol → JacSol");
//   - a lowercase source term whose rendering is capitalised is the signature of
//     a render-pass mix-up ("safeguard → Институт Сильвеста"), the residue the
//     echo check does not catch. Skipped for targets that capitalise nouns.
func acceptGlossEntry(e GlossEntry, source, target string) bool {
	src, tgt := strings.TrimSpace(e.Src), strings.TrimSpace(e.Tgt)
	if src == "" || tgt == "" || len(src) < 2 {
		return false
	}
	if strings.EqualFold(src, tgt) {
		return false
	}
	if stopSet(source)[strings.ToLower(src)] {
		return false
	}
	if !hasTargetScript(tgt, target) {
		return false
	}
	if !capitalisesNouns(target) && isLowerFirst(src) && isUpperFirst(firstWord(tgt)) {
		return false
	}
	return true
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t«\"("); i > 0 {
		return s[:i]
	}
	return s
}

// hasTargetScript reports whether the rendering contains at least one letter of
// the target language's own script. A mixed rendering is fine — a brand or a
// formula may keep Latin — but an all-Latin "translation" into Russian is not.
func hasTargetScript(tgt, target string) bool {
	want := scriptsOf(target)
	if len(want) == 0 {
		return true // Latin-script or unknown target: no opinion
	}
	for _, r := range tgt {
		if !unicode.IsLetter(r) {
			continue
		}
		for _, tbl := range want {
			if unicode.Is(tbl, r) {
				return true
			}
		}
	}
	return false
}

// scriptsOf maps a target language to the unicode tables its script may use, or
// nil when the target uses the Latin alphabet (where the check cannot
// discriminate — a Latin rendering of a Latin source term is legitimate there).
func scriptsOf(target string) []*unicode.RangeTable {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "ru", "uk", "be", "bg", "sr", "mk", "kk", "ky", "mn", "tg":
		return []*unicode.RangeTable{unicode.Cyrillic}
	case "el":
		return []*unicode.RangeTable{unicode.Greek}
	case "he", "yi":
		return []*unicode.RangeTable{unicode.Hebrew}
	case "ar", "fa", "ur", "ps":
		return []*unicode.RangeTable{unicode.Arabic}
	case "hy":
		return []*unicode.RangeTable{unicode.Armenian}
	case "ka":
		return []*unicode.RangeTable{unicode.Georgian}
	case "hi", "mr", "ne":
		return []*unicode.RangeTable{unicode.Devanagari}
	case "th":
		return []*unicode.RangeTable{unicode.Thai}
	case "ja": // a rendering may be all kanji, all kana, or mixed
		return []*unicode.RangeTable{unicode.Han, unicode.Hiragana, unicode.Katakana}
	case "zh":
		return []*unicode.RangeTable{unicode.Han}
	case "ko":
		return []*unicode.RangeTable{unicode.Hangul, unicode.Han}
	}
	return nil
}
