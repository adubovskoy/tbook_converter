package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/epub"
	"github.com/dimando/reader/converter/internal/langcheck"
	"github.com/dimando/reader/converter/internal/lexcheck"
	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
	"github.com/dimando/reader/converter/internal/tbook/v2/migrate"
	"github.com/dimando/reader/converter/internal/translate"
)

// This file is the --format v2 output path. The pipeline is unchanged: it still
// builds the version-1 *tbook.Book, and the version-2 archive is derived from
// it at write time by internal/tbook/v2/migrate, with an Annotate hook that
// supplies what a migration tool cannot know — which gate flagged which cell,
// which translation shipped without an alignment, which was left unverified.
//
// Spec §11.2 rule 2: a status is written ONLY from recorded pipeline state.
// Nothing here inspects the translated text to guess at one.

// gateJudgeScoped is the gate id used when the judge ran over a risk-scoped
// subset (--judge-scope flagged, the default) rather than every cell. §6.8
// reserves the registered id `judge` for a gate that covered EVERY translation
// of the overlay — claiming it for a sampled pass would make the absence of a
// verdict mean "judged and passed" for cells the judge never saw.
const gateJudgeScoped = "x-judge-scoped"

// escalationOutcome is what --escalate-model left behind: the only place the
// pipeline distinguishes "flagged, regeneration exhausted, shipped as is"
// (unverified) from "translation dropped" (rejected). Escalation itself never
// rejects: a cell it cannot fix keeps its original translation, unverified.
type escalationOutcome struct {
	unverified map[string]map[string]bool              // target → src
	verdicts   map[string]map[string]translate.Verdict // target → src → failing verdict of the shipped cell
	rejected   map[string]map[string]bool              // target → src
}

func newEscalationOutcome() *escalationOutcome {
	return &escalationOutcome{unverified: map[string]map[string]bool{},
		verdicts: map[string]map[string]translate.Verdict{}, rejected: map[string]map[string]bool{}}
}

// markUnverified records a cell as flagged but not fixed. v, when non-nil, is
// the failing judge verdict of the text that ships, kept so the outcome does
// not depend on re-reading the text-keyed verdict cache.
func (e *escalationOutcome) markUnverified(target, src string, v *translate.Verdict) {
	if e.unverified[target] == nil {
		e.unverified[target] = map[string]bool{}
	}
	e.unverified[target][src] = true
	if v != nil {
		if e.verdicts[target] == nil {
			e.verdicts[target] = map[string]translate.Verdict{}
		}
		e.verdicts[target][src] = *v
	}
}

func (e *escalationOutcome) reject(target, src string) {
	if e.rejected[target] == nil {
		e.rejected[target] = map[string]bool{}
	}
	e.rejected[target][src] = true
}

// gateState holds every gate outcome for the cells about to be written, keyed
// by (target, source sentence), plus the gate list each overlay may claim.
type gateState struct {
	gates     map[string][]string                     // target → gates that covered every cell
	lang      map[string]map[string]langcheck.Flag    // target → src → language flag
	lex       map[string]map[string]lexcheck.Result   // target → src → flagged result
	judge     map[string]map[string]translate.Verdict // target → src → failing verdict
	judgeBy   string                                  // gate id the judge verdicts carry
	aligned   map[string]bool                         // targets whose align pass ran in THIS run
	citations map[*tbook.Sentence]bool                // deliberately untranslated
	esc       *escalationOutcome
}

// newGateState collects the gate outcomes for every cell that will ship.
//
// It must run BEFORE the typography pass (finalizeCells): every outcome is
// derived from the text the gates saw, and the result is keyed by source
// sentence so it carries over to the post-typography cell.
//
// langcheck and lexcheck are pure offline checks, so they are re-run here over
// the final gated text of every sentence: an overlay may list a gate only if
// the gate saw all of its translations, and the passes during the run covered
// only the translatable subset. Judge verdicts cannot be re-derived — they are
// read back from the verdict cache, which keys on the exact text and alignment
// the judge saw, so an escalated rewrite simply has none.
func newGateState(cfg *config.Config, sents []*tbook.Sentence, targets []string,
	citations map[*tbook.Sentence]bool, esc *escalationOutcome) *gateState {

	g := &gateState{
		gates:     map[string][]string{},
		lang:      map[string]map[string]langcheck.Flag{},
		lex:       map[string]map[string]lexcheck.Result{},
		judge:     map[string]map[string]translate.Verdict{},
		judgeBy:   v2.GateJudge,
		aligned:   map[string]bool{},
		citations: citations,
		esc:       esc,
	}
	if g.esc == nil {
		g.esc = newEscalationOutcome()
	}
	if cfg.JudgeScope != config.JudgeScopeAll {
		g.judgeBy = gateJudgeScoped
	}
	for _, t := range cfg.Targets {
		g.aligned[t] = true
	}

	for _, f := range langcheck.Check(sents, cfg.Source, targets, langcheck.Options{}) {
		if g.lang[f.Target] == nil {
			g.lang[f.Target] = map[string]langcheck.Flag{}
		}
		if _, dup := g.lang[f.Target][f.Src]; !dup {
			g.lang[f.Target][f.Src] = f
		}
	}

	for _, t := range targets {
		gates := []string{v2.GateLangcheck} // always runs; costs nothing
		if cfg.Lexcheck {
			if dict, err := lexcheck.Load(cfg.LexiconDir, cfg.Source, t); err == nil && dict != nil {
				flags := map[string]lexcheck.Result{}
				for _, s := range sents {
					if r := dict.CheckSentence(s, t); r.Flagged {
						flags[s.Src] = r
					}
				}
				g.lex[t] = flags
				gates = append(gates, v2.GateLexcheck)
			}
		}
		// A carried-over language was not judged in this run, and an empty
		// verdict cache would read as "judged and passed".
		if cfg.Judge && g.aligned[t] {
			bad := map[string]translate.Verdict{}
			for _, s := range sents {
				if v, ok := translate.VerdictFor(cfg.CacheDir, cfg.JudgeModel, cfg.Source, t, s); ok && !v.OK {
					bad[s.Src] = v
				}
			}
			for src, v := range g.esc.verdicts[t] {
				if _, ok := bad[src]; !ok {
					bad[src] = v
				}
			}
			g.judge[t] = bad
			gates = append(gates, g.judgeBy)
		}
		g.gates[t] = gates
	}
	return g
}

func (g *gateState) gatesFor(lang string) []string { return g.gates[lang] }

func (g *gateState) rejected(lang, src string) bool { return g.esc.rejected[lang][src] }

func (g *gateState) judgeFlagged(lang, src string) bool {
	_, ok := g.judge[lang][src]
	return ok
}

// skipped reports a cell deliberately left untranslated: a citation under
// --skip-citations, or a translation the gates threw away.
func (g *gateState) skipped(s *tbook.Sentence, lang string) bool {
	return g.citations[s] || g.rejected(lang, s.Src)
}

// annotate writes s, lang and v for one translation from the gate state.
// Precedence follows how badly the cell misleads a learner: a cell in the wrong
// language first, then one the gates threw away, then one they flagged and
// shipped, and only then the alignment states.
func (g *gateState) annotate(s *tbook.Sentence, lang string, tr *v2.Translation, hasLinks bool) {
	if tr.T == "" {
		// Without t only skipped and rejected are legal (§6.7); migrate has
		// already set skipped.
		if g.rejected(lang, s.Src) {
			tr.S = v2.StatusRejected
		}
		return
	}
	flag, flagged := g.lang[lang][s.Src]
	switch {
	case flagged && langcheck.Gating(flag.Kind):
		tr.S = v2.StatusWrongLang
		tr.Lang = detectedLang(flag)
	case flagged:
		// An untranslated-only flag has no measured precision and its known
		// failure shape is a correct translation of a proper-noun sentence
		// (langcheck.Gating). It says "look at this", not "this is another
		// language", so it badges as unverified and claims no lang.
		tr.S = v2.StatusUnverified
	case g.rejected(lang, s.Src):
		tr.S = v2.StatusRejected
	case g.esc.unverified[lang][s.Src], g.judgeFlagged(lang, s.Src):
		// A gate flagged the translation itself and it ships as it is.
		tr.S = v2.StatusUnverified
	case hasLinks:
		// Aligned and not flagged into a status: nothing to say.
	case len(v2.TargetTokens(tr)) == 0:
		// Nothing tappable to link (punctuation only) — not a failure.
	case g.aligned[lang] && len(s.Tr[lang].Align) == 0:
		// The align pass ran for this language and produced nothing for this
		// cell: it ships as a raw translation (§6.7 raw).
		tr.S = v2.StatusRaw
	default:
		// No links and no recorded cause — all this run can honestly say.
		tr.S = v2.StatusUnaligned
	}
	tr.V = g.verdict(lang, s.Src)
}

// verdict returns the result of the gate that did not pass this cell, strongest
// gate first. Only one verdict fits in a translation (§6.8).
func (g *gateState) verdict(lang, src string) *v2.Verdict {
	if f, ok := g.lang[lang][src]; ok {
		return &v2.Verdict{By: v2.GateLangcheck, Why: f.Kind}
	}
	if v, ok := g.judge[lang][src]; ok {
		return &v2.Verdict{By: g.judgeBy, Why: why(v.Why)}
	}
	if r, ok := g.lex[lang][src]; ok {
		return &v2.Verdict{By: v2.GateLexcheck, Why: r.Reason}
	}
	return nil
}

// whyMax keeps a verdict reason short; the judge writes free text.
const whyMax = 120

func why(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > whyMax {
		s = strings.TrimSpace(s[:whyMax])
	}
	return s
}

// detectedLang names the language the flagged text is actually in, when the
// check knows it: a duplicate cell is in the language it was copied from, and
// a foreign script names one only where the script itself identifies a
// language (Han → zh). Latin and Cyrillic are shared by many languages, so
// nothing is claimed for them and the cell ships with no lang at all (§6.9).
// Only a gating kind reaches here — an untranslated flag badges as unverified
// and a lang without s: wrongLang is a §11.1 violation.
func detectedLang(f langcheck.Flag) string {
	if f.Kind == langcheck.KindDuplicateTarget {
		return f.Other
	}
	return f.Lang
}

// sourceIdentity records which document, and which file, the book was converted
// from (spec §3.6).
//
// textSha256 and docs are omitted: the parser keeps no byte-faithful copy of
// each source document's text stream, and hashing the cleaned, re-segmented
// sentences would produce a number that looks like the edition fingerprint the
// recipe defines without being one.
func sourceIdentity(path string, b *epub.Book) *v2.Source {
	s := &v2.Source{
		Kind:       sourceKind(path),
		Identifier: b.Identifier,
		Title:      b.Title,
		Language:   b.Language,
	}
	if sum, err := fileSHA256(path); err == nil {
		s.SHA256 = sum
	}
	return s
}

func sourceKind(path string) string {
	switch lower := strings.ToLower(path); {
	case strings.HasSuffix(lower, ".fb2"), strings.HasSuffix(lower, ".fb2.zip"):
		return "fb2"
	case strings.HasSuffix(lower, ".epub"):
		return "epub"
	}
	return ""
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// writeV2 assembles, writes and validates the version-2 archive. Validation is
// not advisory here: a file that violates §11.1 must never be reported as a
// successful conversion.
func writeV2(cfg *config.Config, b *tbook.Book, src *v2.Source, run tbook.RunRecord, g *gateState) error {
	arc, stats, err := migrate.Convert(b, migrate.Options{
		Annotate: g.annotate,
		Skipped:  g.skipped,
		Gates:    g.gatesFor,
		Source:   src,
		Run:      &run,
	})
	if err != nil {
		return fmt.Errorf("assemble v2: %w", err)
	}
	if err := arc.Write(cfg.Out); err != nil {
		return fmt.Errorf("assemble v2: %w", err)
	}
	if fi, err := os.Stat(cfg.Out); err == nil {
		fmt.Printf("Done. %s (%d KB, formatVersion 2)\n", cfg.Out, fi.Size()/1024)
	}
	fmt.Printf("  %d sentences, %d translations, %d links, %d escapes, %d unaligned; gates %s\n",
		stats.Sentences, stats.Translations, stats.TokenLinks, stats.Escapes, stats.Unaligned,
		gatesSummary(g, arc.Manifest.Languages.Targets))
	fmt.Println(statusSummary(arc))
	return validateV2(cfg.Out)
}

// validateV2 runs the §11.1 validator over a written file and fails on any
// violation.
func validateV2(path string) error {
	rep, err := v2.Validate(path)
	if err != nil {
		return fmt.Errorf("validate v2: %w", err)
	}
	for _, w := range rep.Warnings {
		fmt.Println("  warning:", w)
	}
	if !rep.OK() {
		for _, v := range rep.Violations {
			fmt.Println("  VIOLATION:", v)
		}
		return fmt.Errorf("validation failed: %d violations of spec §11.1", len(rep.Violations))
	}
	fmt.Printf("Validation: valid version-2 archive (%d warnings)\n", len(rep.Warnings))
	return nil
}

func gatesSummary(g *gateState, targets []string) string {
	parts := make([]string, 0, len(targets))
	for _, t := range targets {
		list := g.gates[t]
		if len(list) == 0 {
			list = []string{"none"}
		}
		parts = append(parts, fmt.Sprintf("%s=[%s]", t, strings.Join(list, " ")))
	}
	return strings.Join(parts, " ")
}

// statusSummary counts the non-ok statuses actually written, so a run says out
// loud how many cells a reader will see badged.
func statusSummary(arc *v2.Archive) string {
	counts := map[string]int{}
	var walk func(ps []v2.GlossParagraph)
	walk = func(ps []v2.GlossParagraph) {
		for _, p := range ps {
			for _, tr := range p.S {
				if tr != nil && tr.S != "" {
					counts[tr.S]++
				}
			}
			for _, row := range p.Rows {
				for _, cell := range row {
					for _, tr := range cell.S {
						if tr != nil && tr.S != "" {
							counts[tr.S]++
						}
					}
				}
			}
		}
	}
	for _, per := range arc.Overlays {
		for _, ov := range per {
			walk(ov.Paragraphs)
		}
	}
	for _, ov := range arc.NotesOverlays {
		for _, ps := range ov.Notes {
			walk(ps)
		}
	}
	if len(counts) == 0 {
		return "Statuses: every cell ok"
	}
	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
	}
	return "Statuses: " + strings.Join(parts, ", ")
}
