# T1 — Typography pass for translated text, and the blind-judge rubric fix

Plan item **T1** of the translation-quality plan. It acts on A2
(`a2-holmes-deepseek-2026-09.md` §5, §6, §8): punctuation is DeepSeek's largest excess defect
class against `gemini-3.1-flash-lite` in Russian (+24) and French (+29) and the fourth in German
(+18), and the A2 judge itself marked down source-faithful partial quotes.

Work 2026-09-22 … 2026-09-26 (interrupted by spend limits and process restarts; nothing was
lost). No
translation or provider runs: everything here is offline text transformation of existing
`.tbook`s plus blind judging with `claude -p --model sonnet`, always with
`--strict-mcp-config --mcp-config '{"mcpServers":{}}'`.

## 0. Run record

| item | value |
|---|---|
| arms | `quality-plan/a2/holmes4-dsr-{ru,de,fr}.tbook` (DeepSeek-V4-Flash + repair on gonka, producer `a7f61cf87c9f-dirty`), anchor `quality-plan/or/gflashlite-{ru,de,fr}.tbook` (g31) |
| excerpt | Sherlock Holmes ch. 1–4, 1 642 translated cells per language (1 624 with Latin letters in the source, the quote-audit population) |
| pass | `internal/typography` (new package), offline driver `cmd/typofix`, converter flag `--typography` (§8) |
| judge | `bench-quality/judge_pairs.py` with the fixed rubric (§1), Sonnet, one fresh process per batch of 10, both presentation orders; `analyze_pairs.py` (a pair is decisive only if both orders agree), `analyze_notes.py` |
| window driver | `.artifacts/typo/judge_window.py` — imports `judge_pairs.judge` unchanged (same rubric, same flags) and only limits how long new batches are started, so each foreground call fits in 590 s |
| artefacts | everything under `bench-quality/.artifacts/typo/` (§9) |

## 1. Part A — the rubric fix and its calibration

### The change

One paragraph added to `RUBRIC` in `judge_pairs.py`; nothing else in the rubric changed (the
existing dash-vs-guillemet tie rule is kept):

> Quotation marks: every sentence is cut out of a longer paragraph, so a source sentence may open
> a quotation without closing it, close one it did not open, or sit inside a multi-sentence
> quotation with no marks at all. A translation whose quotation marks mirror the source
> sentence's marks (opening mark iff the source opens, closing mark iff the source closes, none
> if the source has none) is correct; never count such an open-only, close-only or unmarked cell
> as a defect, and never prefer a side for adding a complete pair the source sentence lacks.
> In French, the standard incise inside one guillemet pair
> (« Peu importe, dit Holmes, c'est mon métier. ») is correct, as is the dash-marked incise;
> neither is a defect.

### Calibration: re-judging A2's punctuation pairs

Every A2 `dsr-vs-g31` pair whose recorded defect class was `punctuation` in either order
(`a2/merged/dsr-vs-g31-{lang}.json`) was copied — same ids, texts and swap flags — into
`.artifacts/typo/rubric-recheck-{lang}/` and re-judged in both orders with the fixed rubric
(`build_recheck.py`, `calib.py`; the A2 directories were not touched). 28 / 28 batches judged.

| lang | pairs | A2 decisive → tie | stayed (same winner) | tie → decisive | winner flipped | punctuation judgements vs dsr : vs g31, A2 → fixed |
|---|--:|--:|--:|--:|--:|--:|
| ru | 36 | 6 (all g31 wins) | 13 (dsr 6, g31 7) | 8 (dsr 5, g31 3) | 1 (g31 → dsr) | 39 : 15 → 26 : 20 |
| de | 49 | 11 (dsr 4, g31 7) | 9 (dsr 4, g31 5) | 10 (dsr 3, g31 7) | 0 | 43 : 25 → 23 : 14 |
| fr | 44 | 9 (dsr 3, g31 6) | 16 (dsr 4, g31 12) | 9 (dsr 5, g31 4) | 2 (g31 → dsr) | 45 : 16 → 29 : 26 |

(8 / 19 / 8 pairs were ties before and after.) The tie → decisive moves show that part of the
change is ordinary re-judging noise, not only the rubric.

**What-if (clearly a what-if: only these pairs were re-judged, everything else is A2 as
recorded):**

| lang | A2 as recorded: dsr : g31 (share, p) | what-if with the fixed-rubric outcomes substituted |
|---|--:|--:|
| ru | 59 : 96 (38.1 %, p = 0.0037) | 65 : 92 (41.4 %, p = 0.038) |
| de | 59 : 109 (35.1 %, p = 1.4e-04) | 58 : 109 (34.7 %, p = 9.7e-05) |
| fr | 76 : 117 (39.4 %, p = 0.0039) | 80 : 113 (41.5 %, p = 0.021) |

**No A2 result changes sign in this what-if**; DeepSeek stays significantly behind g31 in all
three languages. This what-if underestimates the rubric's effect, though: pairs that A2 did not
label `punctuation` (ties, other classes) can also move under the new rubric. §5.3 measures that
directly by re-judging the full A2 sample.

## 2. Part B — the typography pass (`internal/typography`)

`typography.Apply(Input{PrevSrc, Src, Tr, Lang}) Result{Text, Rules}` (and the shorthand
`Fix(src, tr, lang)`). Deterministic; changes only quotation marks, dashes, punctuation
placement and whitespace. Languages without rules (anything but ru / de / fr, region suffixes
accepted) are returned unchanged. The rule sequence is repeated until the text is stable (at
most three rounds), which makes the pass idempotent even where one rule enables another.
`PrevSrc` (the previous source sentence of the paragraph) is used by one rule only.

| rule | languages | what it does | guards (negative cases) |
|---|---|---|---|
| `ascii-quote` | all | ASCII `"` → the target's outer marks (open after start/space/bracket/dash, close otherwise) | a `"` after a digit is an inch mark and stays; no conversion in a cell containing a backtick (code) |
| `dash` | all | `--` and ` - ` → the target dash; unspaced `word—word` spaced; ru/fr: leading `- ` / `–` / tight `—` at the start → `— `; de: every `—` → spaced ` – ` (Duden) | hyphens inside words (`Бейкер-стрит`), numeric ranges (`1888—1890`), existing German `–` (`Berlin–Paris`) untouched |
| `nest-inner` | all | a quote nested inside a quote with the same marks → inner style: ru „…“, de ‚…‘, fr “…” | only when the mark sequence nests to depth exactly 2 and either the source has single quotes or the nesting is balanced |
| `double-close` | all | `»»` (de `““`) → one mark | runs only where `nest-inner` could not resolve the nesting |
| `add-open` | all | source cell opens with `“`/`‘`, target has no opening mark → prepend one | not when the target starts with a dialogue dash (ru/fr) or any quote mark |
| `add-close` | all | source cell's final `”`/`’` closes a quotation opened *before* the cell (or the cell's own opening one), target has no final close → append after the final punctuation; ru: a lone final period goes outside (`».`) | not for a quote opened and closed inside the cell (`a shriek of “Fire!”`, `our ‘Co.’`); not in dash dialogue; for a self-contained source quote only when the target has an unclosed opening mark; a target ending in the inner close mirrors a source ending in `’` |
| `stray-open` | all | target opens, source does not: remove an unmatched opening mark, or the opening half of a pair wrapping the whole cell when the source closes only | not when the source has quote marks inside the cell |
| `stray-close` | all | target ends with a closing mark, source ends with none: remove it if unmatched, or if it closes the cell-wrapping pair of an open-only source cell | never when the source ends with any closing mark (inner `’` included); a close matching a mid-cell open (a title: `в «Кроникл».`) stays |
| `unwrap` | all | source has no quote marks at all, target wraps the whole cell in a pair → remove both | — |
| `ru-comma-close` | ru | `,»` → `»,` | — |
| `ru-period-close` | ru | `.»` → `».` at the end of the cell or before a new sentence inside it | ellipsis (`...»`, `…»`) and one-letter abbreviations (`т. д.»`) stay; `.» —` before a speech tag is left alone (would need `.`→`,`) |
| `ru-tag-dash` | ru | speech-tag fragment split off after a quoted cell gets `— ` (`спросил я, входя.` → `— спросил я, входя.`) | only if the previous source sentence ends in `?”` `!”` `.”` `,”` `…”` `—”`, the source cell does not start with a quote, and the target starts with a lower-case Cyrillic letter |
| `fr-comma-close` | fr | English-style `, »` → ` »,` | — |
| `fr-nnbsp` | fr | U+202F inside `« »` and before `; : ! ?`; inner `“ ”` kept tight | `10:30`, `http://`, the second mark of `?!` |
| `space` | all | collapse runs of spaces, trim; no space before `,` `.`; ru/de also before `; : ! ?` and inside the outer marks | a space before `...` stays |

The source-mirroring rule is the combination of `add-open`, `add-close`, `stray-open`,
`stray-close` and `unwrap`. In ru and fr a leading dialogue dash counts as the opening mark and
dash dialogue needs no closing mark, so dash-style dialogue is never forced into guillemets.
Punctuation-only cells (a lone `’` → `»`) are left alone.

**Tests** (`internal/typography/typography_test.go`): 60+ table cases covering every rule and
every guard above (inch mark, backtick code, compound hyphen, numeric range, `Berlin–Paris`,
ellipsis, abbreviation, time, URL, title in guillemets, dash dialogue, French incise, quotes
opened and closed inside the cell, inner close at the end, unsupported language), plus
invariants run over every case: idempotence (`Apply(Apply(x)) == Apply(x)`) and
"no letters or digits changed" (the sequence of letters and digits is identical). `cmd/typofix`
re-checks both invariants over the whole book: 0 non-idempotent cells and 0 cells with changed
letters in all three languages.

## 3. Offline run on `holmes4-dsr-{ru,de,fr}`

`cmd/typofix -lang L -o fix/dsr-L.changed.jsonl -all fix/dsr-L.all.jsonl holmes4-dsr-L.tbook`
applies the pass to every translation cell and writes `{id, src, before, after, rules}` (id =
`chapter:paragraph:sentence`); the book is not rewritten and nothing is re-aligned.

| lang | cells | changed | visibly changed¹ | rules fired (cells) |
|---|--:|--:|--:|---|
| ru | 1 642 | 273 (16.6 %) | 269 | add-close 110, ru-period-close 56, add-open 47, stray-close 46, ascii-quote 27, dash 6, ru-tag-dash 6, unwrap 3, stray-open 2, nest-inner 2, ru-comma-close 2 |
| de | 1 642 | 176 (10.7 %) | 176 | stray-close 88, dash 27, ascii-quote 25, add-close 23, add-open 6, stray-open 6, space 4, unwrap 3, nest-inner 1 |
| fr | 1 642 | 977 (59.5 %) | 150 | fr-nnbsp 945, add-close 65, space 46, stray-close 44, dash 21, fr-comma-close 20, add-open 19, ascii-quote 6, stray-open 2, nest-inner 2 |

¹ The text differs once all ordinary, no-break and narrow no-break spaces are ignored. In French
827 cells change only by U+202F spacing, which a reader does not see as a change in wording or
marks; those are excluded from the judged set (a) below.

For comparison, the same pass (v2, §4) would change 169 / 147 / 1 047 cells of the g31 arm (fr
again mostly U+202F).

### Quote audit before / after (definitions of `a2/quote_audit.py`, `audit_typo.py`)

| lang | arm | cells with ASCII `"` | source closes, target does not | target closes, source does not | `“`-opened source, target opens with neither mark nor dash | target starts lower-case | fr `, »` | ru `.»` | unspaced `word—word` |
|---|---|--:|--:|--:|--:|--:|--:|--:|--:|
| ru | dsr | 27 | 145 | 56 | 44 | 12 | — | 58 | 2 |
| ru | **dsr-typo** | **0** | **39** | **7** | **0** | **6** | — | **1** | **0** |
| ru | g31 | 1 | 254 | 49 | 8 | 6 | — | 2 | 0 |
| de | dsr | 25 | 25 | 107 | 6 | 22 | — | — | 21 |
| de | **dsr-typo** | **0** | **3** | **16** | **0** | 22 | — | — | **0** |
| de | g31 | 0 | 37 | 105 | 4 | 18 | — | — | 1 |
| fr | dsr | 6 | 65 | 49 | 16 | 19 | 20 | — | 19 |
| fr | **dsr-typo** | **0** | **2** | **5** | **0** | 19 | **0** | — | **0** |
| fr | g31 | 0 | 26 | 225 | 2 | 17 | 11 | — | 1 |

Every column A2 singled out drops to or below g31. The remaining ru "source closes, target does
not" cells are dash-dialogue cells (no closing mark by convention); the remaining ru `.»` is the
segmentation fragment `s.’` → `с.»`, which the one-letter guard keeps. The audit's "closes" test is end-of-cell only, so it
still counts e.g. a German cell ending in a quoted title.

## 4. A bug the judge found, fixed before the verdicts below

The first judged version (v1) treated any source cell ending in `”` or `.’` as closing a
quotation. Five German cells where that final mark closes a quote *opened inside the same cell*
(`… a cry of “Fire!”`, `Now for the ‘Eg.’`, `the tail of the ‘r.’`) got a spurious closing `“`,
and the judge rejected all five consistently in set (a). v2 matches the source's final mark
against openings inside the cell (`srcPartner`) and adds `add-close` only for a quotation opened
before the cell; `stray-close` never fires when the source ends in any closing mark. v2 changes
exactly these five German cells relative to v1 (0 in ru and fr). The German numbers below are v2:
the five pairs are dropped from set (a) (the texts are now identical) and the two affected set
(b) pairs (171, 213) were re-judged with the v2 text (`patch_v2.py`, `pairs/*-de-v2/`). v1
results are kept on disk for reference (`pairs/typo-vs-dsr-de`, `pairs/typo-vs-g31-de`).

## 5. Blind judging (fixed rubric)

### 5.1 (a) dsr-typo vs dsr — does the pass ever make a cell worse?

All visibly changed cells (§3), deduplicated by source text as `prepare_pairs.py` does, no
minimum length, seed 1, both orders (`build_sets.py`, `pairs/typo-vs-dsr-{lang}`).

| lang | pairs | ties | order-inconsistent | decisive | dsr-typo : dsr | **dsr share** | p | punctuation judgements vs dsr-typo : vs dsr |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| ru | 259 | 59 | 2 | 198 | 198 : 0 | **0.0 %** | 5.0e-60 | 2 : 421 |
| de (v2) | 175 | 76 | 0 | 99 | 99 : 0 | **0.0 %** | 3.2e-30 | 0 : 241 |
| fr | 150 | 13 | 2 | 135 | 135 : 0 | **0.0 %** | 4.6e-41 | 2 : 283 |

(v1 German: 99 : 5, dsr 4.8 %, the five losses being the bug of §4.)

**Kill criterion 1 (dsr wins > 40 % of decisive pairs): not triggered in any language.** No
consistent loss remains; the four single-order punctuation judgements against dsr-typo are
listed in §7.

Caveat: this judge now reads a rubric that states the same mirroring convention the pass
implements, so set (a) shows that the pass applies the convention without collateral damage —
not independent evidence that readers prefer the convention.


### 5.2 (b) dsr-typo vs g31, against a fixed-rubric baseline

Set (b) is the exact A2 `dsr-vs-g31` sample (same 400 ids, sources, anchor texts and swap
flags) with dsr's text replaced by dsr-typo's (`build_sets.py`, `pairs/typo-vs-g31-{lang}`).
Because the rubric changed too, A2's recorded numbers are not a valid "before": the same 400
pairs were re-judged with **dsr's original text and the fixed rubric**
(`pairs/baseline-dsr-vs-g31-{lang}`; the key files are byte-identical in `pairs` to A2's). The
only difference between the baseline and set (b) is the pass.

| lang | A2 as recorded (old rubric): dsr : g31 | **baseline** (fixed rubric, dsr): dsr : g31, share, p | **dsr-typo** (fixed rubric): dsr-typo : g31, share, p | ties / order-inconsistent, baseline → typo |
|---|--:|--:|--:|--:|
| ru | 59 : 96 (38.1 %) | 61 : 107 (**36.3 %**, p = 4.8e-04) | 79 : 76 (**51.0 %**, p = 0.87) | 209 / 23 → 214 / 31 |
| de (v2) | 59 : 109 (35.1 %) | 70 : 99 (**41.4 %**, p = 0.031) | 71 : 82 (**46.4 %**, p = 0.42) | 207 / 24 → 217 / 30 |
| fr | 76 : 117 (39.4 %) | 80 : 96 (**45.5 %**, p = 0.26) | 91 : 90 (**50.3 %**, p = 1.0) | 198 / 26 → 193 / 26 |

With the same rubric, the pass moves DeepSeek from significantly behind g31 to level in Russian
and French, and to not-significantly behind in German. The rubric alone moved A2's
38.1 / 35.1 / 39.4 % to 36.3 / 41.4 / 45.5 % (ru unchanged within noise; de and fr +6 pp).

German v1 (before the §4 fix) was 70 : 82 (46.1 %). `typo-vs-g31-de-fix` holds only the two
pairs whose dsr-typo text v2 changed back to dsr's original (171 and 213; 2 / 2 batches):
171 is a consistent g31 win in both v1 and v2; 213 was order-inconsistent in v1 and is a
consistent dsr-typo win in v2 — hence 70 → 71. `typo-vs-g31-de-v2` is set (b) with those two
verdicts merged in, and `typo-vs-dsr-de-v2` is set (a) with the five §4 pairs dropped.

### 5.3 Rubric vs pass vs judge noise (paired decomposition)

All three runs (A2, baseline, typo) judge the same pairs, so each pair can be followed
(`.artifacts/typo/decompose.py` → `decompose.txt`). Outcome per pair: dsr side wins / tie / g31
wins. Pairs whose dsr text the pass did not change are a control: any movement there is
re-judging noise.

| lang | visibly changed pairs: better / worse (sign p) | unchanged pairs (control): better / worse (p) | fr U+202F-only pairs: better / worse |
|---|--:|--:|--:|
| ru | 63: **29 / 0** (3.7e-09) | 334: 39 / 25 (0.10) | — |
| de | 47: **14 / 5** (0.064) | 353: 48 / 40 (0.46) | — |
| fr | 36: **17 / 0** (1.5e-05) | 175: 19 / 14 (0.49) | 189: 24 / 28 |

The pass never makes a Russian or French pair worse. The five German "worse" pairs (95, 194,
259, 278, 393) are single-step moves (win → tie or tie → loss), and in all five the change is a
correct one: an ASCII `"` became `“` (259), a final `“` that the source does not have was
removed (194, 278, 393), a close-only source got its `“` (95). Only one judgement among them
names `punctuation` (95, one order); the others name `naturalness`, `register`, `collocation`,
`fidelity`, `idiom-calque` or `other` — the same wording judged differently, i.e. noise of the
size the control shows. French U+202F-only changes are invisible and behave like the control.

Attributing the change in dsr's share of decisive pairs (counterfactual = baseline with only the
visibly changed pairs replaced by their typo outcome):

| lang | A2 | → rubric | baseline | → pass (visible changes) | counterfactual | → U+202F-only + control drift | typo run |
|---|--:|--:|--:|--:|--:|--:|--:|
| ru | 38.1 % | −1.8 | 36.3 % | **+9.3** | 45.6 % (73 : 87, p = 0.30) | +5.3 | 51.0 % |
| de | 35.1 % | +6.3 | 41.4 % | **+2.8** | 44.2 % (72 : 91, p = 0.16) | +2.2 | 46.4 % |
| fr | 39.4 % | +6.1 | 45.5 % | **+5.1** | 50.6 % (86 : 84, p = 0.94) | −0.4 | 50.3 % |

(The rubric column also contains A2-vs-baseline re-judging noise: 41 / 50, 62 / 42, 66 / 45
pairs moved up / down in ru / de / fr, sign p 0.40 / 0.062 / 0.057.)

So the conservative estimate of the pass itself is **+9 / +3 / +5 pp**; the typo run's headline
numbers additionally contain +1 … +5 pp of favourable re-judging drift on untouched pairs (ru's
+5.3 is the largest and within noise, p = 0.10). Either way the conclusion holds: after the
rubric fix and the pass, DeepSeek+repair is no longer significantly behind g31 in any of the
three languages (it was in all three in A2).

## 6. Kill criterion 2 — does the punctuation excess fall by ≥ 50 %?

"Punctuation excess" is A2's measure: decisive judgements (either order) whose losing-side
defect class is `punctuation`, dsr side minus g31 side (`analyze_notes.py`).

| lang | A2 (old rubric) dsr : g31 → excess | baseline (fixed rubric) dsr : g31 → excess | **dsr-typo** : g31 → excess | dsr-side punctuation judgements, A2 → typo / baseline → typo |
|---|--:|--:|--:|--:|
| ru | 39 : 15 → **+24** | 62 : 28 → **+34** | 11 : 45 → **−34** | −72 % / −82 % |
| de | 43 : 25 → **+18** | 40 : 48 → −8 | 10 : 71 → **−61** | −77 % / −75 % |
| fr | 45 : 16 → **+29** | 30 : 57 → −27 | 1 : 62 → **−61** | −98 % / −97 % |

**Kill criterion 2: not triggered.** The excess falls by more than 100 % in every language —
it changes sign — whichever "before" is used. In de and fr the fixed rubric alone already
removed the excess (the old rubric's partial-quote and incise penalties were most of it); in ru
the excess was real (+34 even under the fixed rubric) and the pass removes it. The number of
punctuation judgements against DeepSeek falls by 72–98 %.

A side effect worth noting: once DeepSeek's punctuation is clean, the judges name punctuation as
g31's worst defect more often (28 → 45, 48 → 71, 57 → 62). g31 has the same kind of defects
(§3 audit table); the pass would presumably help it too, but that was not judged.

Other defect classes stay close to the baseline (total judgements against the DeepSeek side
277 → 212, 267 → 240, 259 → 235; the drop is almost entirely punctuation). DeepSeek's
remaining excess classes against g31 are `idiom-calque` (+19 / +27 / +30 under the typo run),
`collocation` and, in German, `naturalness` — the targets of T3 (expressions) and of the
prompt work, not of typography.

## 7. What the judge still disliked

The complete lists are in `.artifacts/typo/examples.md`.

- **Set (a), single-order punctuation judgements against dsr-typo (4; none consistent):**
  ru 154 and 160 (`… о бумаге и дате».`, `«Совершенно никаких».` — one order preferred the period
  inside, `.»`; the pass applies the Russian rule and the other order agreed with it); fr 19
  (inner `“monsieur”` instead of `« monsieur »` inside a guillemet quote — nesting convention);
  fr 55 (`Maintenant »,` from `fr-comma-close`; one order preferred the English-style `, »`).
  None of these is a wrong output; they are convention choices on which the judge disagrees with
  itself.
- **Set (b), consistent losses of dsr-typo with class punctuation:** ru 353 (DeepSeek wraps the
  second half of a split quote in its own guillemets after a semicolon), ru 395 and 399
  (`«— Ах, …` — a quotation mark *and* a dialogue dash; DeepSeek's text, the pass only removed a
  stray closing mark). These are real model defects the pass does not yet repair; a `«—` → `«`
  rule for ru/fr is a cheap follow-up. ru 108 is a capitalisation choice (`ваше Величество`), not
  punctuation.
- **The v1 bug (§4)** — five consistent German losses — was the only systematic defect the judge
  found in the pass, and it is fixed.

## 8. Integration: `--typography` (default OFF)

Both kill criteria passed, so the pass is wired into the converter behind a flag that is off by
default:

- `--typography` (env `TYPOGRAPHY`), `config.Config.Typography`; README flag list and
  `.env.example` document it.
- It runs **at assembly**, after the language gate, lexcheck, judge and escalation and just
  before the v1 or v2 writer (`cli.applyTypography`), for the run's targets that have rules
  (ru / de / fr; others print "no rules … skipped"), over body paragraphs and note paragraphs,
  with the previous source sentence of each paragraph as context. Gates and aligners therefore
  see the raw model text; the cache is untouched and the pass is recomputed on every assembly
  (it is deterministic, idempotent and costs milliseconds).
- `typography.ApplyParagraphs` remaps alignment offsets through the unchanged sequence of
  letters and digits (`typography.Remap`): each chunk is narrowed to its first and last letter or
  digit, a chunk with none is dropped, and a cell is left untouched if its letters or digits
  would differ (by construction they never do). Tests: `internal/typography/book_test.go`,
  `internal/config/config_test.go` (`TestTypographyDefaultOff`).
- Provenance: the run record's `options` gets `"typography": true` when the flag is on.
- Checked on the six Holmes books (dsr and g31, ru / de / fr): the same 273 / 176 / 977 and
  169 / 147 / 1 047 cells change as with `cmd/typofix` (no cell skipped by the remap guard),
  `tbook.Validate` reports 0 offset and 0 structural errors after the pass, and target-word
  alignment coverage is unchanged to four decimals.
- `go build ./...`, `go vet ./...` and `go test ./...` are green in `converter/`.

Why default OFF and not ON: the evidence is one book (Holmes ch. 1–4, a quotation-heavy 19th-
century text) with one model (DeepSeek), and the set (a) judge reads a rubric that states the
convention the pass implements (§5.1 caveat). Turning it on by default — probably for gonka,
like `--repair` — should wait for one more book and for the g31 arm (§6 side effect).

## 9. Artefacts

All under `converter/bench-quality/.artifacts/typo/`:

| path | what |
|---|---|
| `rubric-recheck-{ru,de,fr}/`, `build_recheck.py`, `calib.py`, `calib.json` | §1 calibration |
| `fix/dsr-{lang}.{changed,all}.jsonl`, `fix/arm-*.json` | §3 pass output and judge arms |
| `audit_typo.py`, `audit.{txt,json}` | §3 quote audit |
| `build_sets.py` | builds sets (a) and (b) |
| `pairs/typo-vs-dsr-{ru,fr}/`, `pairs/typo-vs-dsr-de-v2/` | §5.1 set (a); `typo-vs-dsr-de` is v1 |
| `pairs/typo-vs-g31-{ru,fr}/`, `pairs/typo-vs-g31-de-v2/` | §5.2 set (b); `typo-vs-g31-de` is v1, `typo-vs-g31-de-fix` the two re-judged v2 pairs |
| `pairs/baseline-dsr-vs-g31-{ru,de,fr}/` | §5.2 fixed-rubric baseline on A2's sample |
| `patch_v2.py` | §4 v1 → v2 set surgery |
| `decompose.py`, `decompose.txt` | §5.3 paired decomposition |
| `examples.py`, `examples.md` | §7 examples |
| `judge_window.py` | time-boxed driver over `judge_pairs.judge` |

Every pair directory has `key.json`, `batch-*`, `verdict-*`, `notes-*`, `result.txt`
(`analyze_pairs.py`) and `notes.txt` (`analyze_notes.py`).

## 10. Conclusions

1. **The A2 judge over-penalised source-faithful quotes.** With the rubric fixed, DeepSeek's
   deficit against g31 shrinks from 35 / 39 % to 41 / 46 % in German / French; Russian is
   unchanged (36 %). The fixed rubric stays in `judge_pairs.py` for all later judging.
2. **The typography pass is safe and effective.** It never lost a consistent judgement against
   the untouched text after the v2 fix (0 of 432 decisive pairs), and on A2's sample it lifts
   DeepSeek by +9 / +3 / +5 pp against g31 (paired, conservative), removing Russian's real
   punctuation excess.
3. **Together:** DeepSeek+repair on gonka goes from significantly behind g31 in all three
   languages (A2) to level in ru and fr (51 %, 50 %) and 46 % (n.s.) in de.
4. **Both kill criteria passed**; the pass ships behind `--typography`, default off.
5. **Follow-ups:** a `«—` → `«` rule (ru / fr); judge the pass on g31; one more book before
   considering default-on for gonka; DeepSeek's remaining gap is idioms and collocations (T3).

## 11. Follow-up (2026-09-26 … 09-28): `«—` rule, the g31 arm, a second book, the default

The §10 follow-ups, done with frozen binaries `bin/convert-typo2` (arms) and
`bin/typofix-typo2` (pass), both built 2026-09-26 from the same tree. Artefacts:
`bench-quality/.artifacts/typo2/` (§11.7). Judge unchanged: `judge_pairs.judge` (fixed rubric,
Sonnet, `--strict-mcp-config --mcp-config '{"mcpServers":{}}'`), via `judge_window.py`, both
orders, a pair decisive only if both orders agree. Changed-cell sets follow T1 set (a): every
visibly changed cell (U+202F / U+00A0-only changes left out), deduplicated by source, no
minimum length.

### 11.1 The `«—` rule

`internal/typography`: `quoteDash` (rule name `quote-dash`) drops a dialogue dash right after
the opening mark when the cell opens with `«` and its source opens a quotation:
«— Ах, не беспокойтесь об этом, мистер Уилсон!» → «Ах, не беспокойтесь об этом, мистер Уилсон!».
A mid-cell incise (`, — сказал я. — А плата?`) is kept, and a cell whose source has no quotation
is left alone (`typography_test.go`, the `ru quote dash*` cases). The pass version with this
rule (and a French fix: an English `”` closing a French cell becomes ` »`) is "v4"; cells
whose v4 output differs from the version judged earlier were re-judged as supplement sets that
override the base set by source (`build_supp.py`, `pairs/*-v4`): Holmes dsr ru 7 and de 1,
Holmes g31 de 4, Revelation Space g31 fr 8 — all 20 are ties against the untouched text. One
Holmes dsr de dash-only cell missing from T1's set (a) was judged too (`*-de-miss`, tie).

### 11.2 The pass on gemini-3.1-flash-lite (g31) output, and on a second book

Second book: *Revelation Space* (Alastair Reynolds), `--limit-chapters 2`: 1 575 sentences
(~21 k words), modern prose with dialogue in English curly quotes — a very different text from
Holmes. g31 arms `rs2-g31-{ru,de,fr}`: OpenRouter, `google/gemini-3.1-flash-lite`,
`--no-repair` (as the T1 anchor), production defaults otherwise (`run_arms.sh g31`); 1m23s …
1m29s each, $0.366 in total, `Validation … OK`, coverage 97–98 % target / 97–99 % source.

Changed cells per arm (`fix/*.summary.txt`; no cell non-idempotent, no letter ever changed):

| arm | cells | changed | visibly | top rules |
|---|--:|--:|--:|---|
| Holmes dsr ru / de / fr | 1 642 | 277 / 177 / 977 | 263 / 177 / 150 | add-close, ru-period-close / stray-close, dash / fr-nnbsp, add-close |
| Holmes g31 ru / de / fr | 1 642 | 170 / 151 / 1 047 | 170 / 150 / 303 | add-close / stray-close / fr-nnbsp, space, stray-close, unwrap |
| RS g31 ru / de / fr | 1 575 | 105 / 77 / 723 | 103 / 77 / 106 | add-close / stray-close / fr-nnbsp, space, stray-close |

Changed-cell judging, pass vs untouched text of the same arm (supplements merged):

| book | arm | lang | pairs | typo : base | ties | order-inconsistent | base share | sign p |
|---|---|---|--:|--:|--:|--:|--:|--:|
| Holmes | dsr | ru | 263 | **195 : 0** | 66 | 2 | 0.0 % | 4.0e-59 |
| Holmes | dsr | de | 177 | **99 : 0** | 78 | 0 | 0.0 % | 3.2e-30 |
| Holmes | dsr | fr | 150 | **135 : 0** | 13 | 2 | 0.0 % | 4.6e-41 |
| Holmes | g31 | ru | 170 | **110 : 0** | 60 | 0 | 0.0 % | 1.5e-33 |
| Holmes | g31 | de | 150 | **78 : 0** | 70 | 2 | 0.0 % | 6.6e-24 |
| Holmes | g31 | fr | 303 | **100 : 0** | 199 | 4 | 0.0 % | 1.6e-30 |
| RS | g31 | ru | 103 | **88 : 0** | 15 | 0 | 0.0 % | 6.5e-27 |
| RS | g31 | de | 77 | **19 : 0** | 53 | 5 | 0.0 % | 3.8e-06 |
| RS | g31 | fr | 106 | **40 : 0** | 66 | 0 | 0.0 % | 1.8e-12 |

(Holmes dsr is T1's set (a) with the v4 supplements; the ru row moves from 198 : 0 over 259
pairs to 195 : 0 over 263 because the `«—` cells became ties.) Before the v4 French fix, RS g31
fr was 40 : 4: the four consistent losses were all the same pass defect — an English `”`
closing the cell was kept and a French ` »` added after it:

- src: `Is that understood?”` — pass v3: `Est-ce compris ?” »` — g31: `Est-ce compris ?”`
  (likewise `Dig.”` → `Fouiller.” »`, ids 13 and 54). v4 gives `Est-ce compris ?»` / `Fouiller.»`
  (U+202F before `»`), and all eight cells v4 changed are ties.

Single-order losses remain only where the judge disagrees with itself on the source-mirrored
convention, e.g. RS de 21, 24–28: src `It was too distracting.”` (close-only), pass
`Es war zu ablenkend.“` vs g31 `„Es war zu ablenkend.“` — one order prefers the complete pair
the source lacks, the other agrees with the rubric. Holmes g31 de 2, fr 5, RS de 5; none
consistent.

The §6 side effect is confirmed: the pass helps g31 as much as it helps DeepSeek. In every
book × language × model cell it wins every decisive changed pair.

### 11.3 DeepSeek arms on the second book

`rs2-dsr-{ru,de,fr}`: `bin/convert-typo2 … --limit-chapters 2 -t L --provider gonka --model
deepseek-ai/DeepSeek-V4-Flash-0731`, production defaults (proofread/repair ON for gonka,
typography OFF), cache `cache-dsr` (`run_dsr_lang.sh`, same flags as `run_arms.sh dsr`). A first
ru attempt (2026-09-26 … 09-28) finished translating (1 566 sentences, 22 min) but lost 1 254
of 1 566 repair calls to gonka transport errors and 502/429s and was killed
(`rs2-dsr-ru.attempt1.{log,stats.jsonl}`); the rerun reused its translations from the cache.

| arm | lang | wall | requests | non-200 | OpenRouter $ | validation | coverage tgt/src |
|---|---|--:|--:|--:|--:|---|---|
| dsr | ru | 2m17s (rerun) | 174 | 0 | 0 | 1 575 sentences, 0 empty / offset / struct errors — OK | 99 / 98 % |
| dsr | de | 4m59s | 417 | 0 | 0 | OK | 98 / 99 % |
| dsr | fr | 4m19s | 422 | 0 | 0 | OK | 98 / 99 % |
| g31 | ru / de / fr | 1m23s … 1m29s | 120 / 114 / 119 | 0 | 0.128 / 0.113 / 0.125 | OK | 98/97, 97/99, 98/99 % |

OpenRouter spend of the whole follow-up: $0.366 (the g31 arms); the DeepSeek arms ran on gonka
only; judging is `claude -p`.

Pass on these arms (`fix/rs2-dsr-*.summary.txt`): ru 155 cells changed (152 visibly;
`ru-period-close` 74, `add-close` 28, `ascii-quote` 26, `stray-close` 23), de 105 (105; `dash` 40,
`stray-close` 32, `add-close` 15), fr 807 (85; `fr-nnbsp` 702, `dash` 124, `space` 34,
`stray-close` 28, `add-close` 26). No cell non-idempotent, no letter changed.

### 11.4 (a) rs2-dsr-typo vs rs2-dsr on changed cells

| lang | pairs | dsr-typo : dsr | ties | order-inconsistent | dsr share | sign p |
|---|--:|--:|--:|--:|--:|--:|
| ru | 152 | **124 : 0** | 21 | 7 | 0.0 % | 9.4e-38 |
| de | 105 | **48 : 0** | 57 | 0 | 0.0 % | 7.1e-15 |
| fr | 85 | **50 : 0** | 34 | 1 | 0.0 % | 1.8e-15 |

Single-order losses: ru 8, all `ru-period-close` on a fully quoted sentence — src
`“I realise that.”`, pass `«Я понимаю».`, DeepSeek `«Я понимаю.»`; one order prefers the period
inside, the other the Russian rule (the T1 §7 pattern). fr 1 (id 81, the incise
`«⍽Quand je serai saisie d'un ennui existentiel⍽», dit Khouri, «⍽je pourrais essayer.⍽»` vs
DeepSeek's `« … existentiel, » dit Khouri, « … »`). No consistent loss.

Across both books, both models and all three languages the changed-cell record is therefore
**1 086 : 0** decisive pairs (Holmes dsr 195 / 99 / 135, Holmes g31 110 / 78 / 100, RS g31
88 / 19 / 40, RS dsr 124 / 48 / 50); every one of the 12 comparisons has better ≥ worse and
p < 1e-5 in favour of the pass.

### 11.5 (c) rs2-dsr and rs2-dsr-typo vs rs2-g31

`build_c.py`: `prepare_pairs_lang.py rs2-dsr-L.tbook dsr rs2-g31-L.tbook g31 --n 400 --seed 1
--batch 10` (default min-words), then the same 400 ids, sources and swap flags with dsr's text
replaced by the full pass output (as T1 set (b)); the pass changed 36 / 27 / 177 of the 400 dsr
texts (fr: 153 of them spacing-only).

| lang | dsr : g31 (share, p) | dsr-typo : g31 (share, p) | Δ share | ties / incons., dsr → typo |
|---|--:|--:|--:|--:|
| ru | 73 : 104 (**41.2 %**, p = 0.024) | 80 : 89 (**47.3 %**, p = 0.54) | **+6.1** | 197 / 26 → 199 / 32 |
| de | 64 : 113 (**36.2 %**, p = 2.8e-04) | 60 : 115 (**34.3 %**, p = 3.9e-05) | **−1.9** | 207 / 16 → 204 / 21 |
| fr | 68 : 101 (**40.2 %**, p = 0.014) | 68 : 96 (**41.5 %**, p = 0.035) | **+1.3** | 209 / 22 → 216 / 20 |

Paired decomposition (`decompose_c.py` → `decompose_c.txt`; outcome per pair = dsr side wins /
tie / g31 wins, typo run vs dsr run):

| lang | visibly changed: better / worse (p) | spacing-only changed | unchanged (control): better / worse (p) |
|---|--:|--:|--:|
| ru | 36: **12 / 2** (0.013) | — | 364: 48 / 39 (0.39) |
| de | 27: **9 / 0** (0.0039) | — | 373: 31 / 45 (0.14) |
| fr | 24: **9 / 1** (0.021) | 153: 20 / 15 (0.50) | 223: 12 / 21 (0.16) |

Wherever the pass touched the text it helped, significantly in all three languages; the German
drop of the headline share comes entirely from re-judging noise on the 373 pairs whose text is
identical in both runs (31 up / 45 down, p = 0.14 — the same size as the T1 §5.3 controls). The
three "worse" changed pairs: ru 304 (`приятным."` → `приятным».`, win → tie, one order named
punctuation), ru 381 (`«Это как вытягивать зубы, Илия.»` → `«Это как вытягивать зубы, Илия.` for
the open-only source `“This is like drawing teeth, Ilia.`, tie → loss, classes naturalness /
collocation), fr 391 (`"Là où la petite souris l'a laissée, ma chère."` →
`« Là où la petite souris l'a laissée, ma chère. »`, tie → loss, classes fidelity / fidelity —
the wording, not the marks).

Punctuation judgements against the DeepSeek side vs against g31 (`analyze_notes.py`):
ru 30 : 24 → 8 : 28, de 24 : 5 → 8 : 14, fr 31 : 20 → 1 : 26 — the second-book punctuation
excess (+6 / +19 / +11) becomes −20 / −6 / −25. The remaining gap is `fidelity` (de 128 : 71),
`naturalness`, `collocation`, `idiom-calque` — not typography. On this book DeepSeek+repair is
significantly behind g31 in all three languages before the pass, and after it still in de and
fr (ru level, p = 0.54).

### 11.6 Decision

The rule, applied as stated: ON for every provider if, in all three languages and on both books,
(1) the changed-cell comparisons have better ≥ worse with no significant loss, and (2) the
second-book dsr-typo share vs g31 is ≥ the dsr share; ON for gonka only if only DeepSeek output
qualifies; otherwise OFF.

| condition | ru | de | fr |
|---|---|---|---|
| (1) Holmes dsr / g31, RS dsr / g31 changed cells | 195:0, 110:0, 124:0, 88:0 ✓ | 99:0, 78:0, 48:0, 19:0 ✓ | 135:0, 100:0, 50:0, 40:0 ✓ |
| (2) RS dsr-typo share ≥ dsr share | 47.3 ≥ 41.2 ✓ | **34.3 < 36.2 ✗** | 41.5 ≥ 40.2 ✓ |

Condition (2) fails in German, and it is a DeepSeek condition, so DeepSeek output does not
qualify either. **`--typography` stays default OFF for every provider**; `internal/config`,
the help text, README, `.env.example` and `TestTypographyDefaultOff` are unchanged, and
`go build ./... && go vet ./... && go test ./...` is green.

Read the failure for what it is: by the paired decomposition (§11.5) the pass improved every
German pair it changed (9 / 0) and the −1.9 pp is drift on untouched pairs; pooled over the
three languages the shares are 39.2 % (205 : 318) → 40.9 % (208 : 300), which would pass. A
share-level condition on 400 pairs of which only 27 change cannot resolve a +3 pp effect —
condition (2) is dominated by judge noise. If the rule is revisited, the better test is the
paired one (changed pairs better ≥ worse, sign test), which the pass meets in every language on
both books; with that criterion the evidence supports default ON for every provider.

### 11.7 Artefacts (`bench-quality/.artifacts/typo2/`)

| path | what |
|---|---|
| `run_arms.sh`, `run_dsr_lang.sh`, `wall-times.tsv`, `rs2-{dsr,g31}-{ru,de,fr}.{tbook,log,stats.jsonl}`, `armsum.py` | §11.2–11.3 arms (`cache-{dsr,g31}/`, `rs2-dsr-ru.attempt1.*`) |
| `fixsets.py`, `fix/{h,rs2}-{dsr,g31}-*.{all,changed}.jsonl`, `fix/*.summary.txt`, `fix/arm-*.json`; `fix/v3/` | pass output (v4) and judge arms; `v3/` the earlier pass version |
| `pairs/{h,rs2}-{dsr,g31}-typo-vs-*-{ru,de,fr}` | changed-cell sets; `*-v4` / `*-de-miss` supplements (`build_supp.py`), `h-dsr-v3-vs-v2-ru` |
| `pairs/rs2-dsr-vs-g31-*`, `pairs/rs2-dsrtypo-vs-g31-*`, `build_c.py`, `decompose_c.py`, `decompose_c.txt` | §11.5 set (c) |
| `tally.py`, `losses.py`, `judge_window.py` | tallies with supplements merged, loss listings, time-boxed judge driver (a 0-byte verdict counts as missing) |

Holmes dsr set (a) itself stays in `.artifacts/typo/pairs/typo-vs-dsr-{ru,fr}` and
`typo-vs-dsr-de-v2`.
