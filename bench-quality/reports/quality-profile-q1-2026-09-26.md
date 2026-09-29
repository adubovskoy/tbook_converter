# Q1 — "Quality profile": gonka DeepSeek + repair + typography, gemini-3.7-flash judges and re-translates

Plan item **Q1**. It acts on A2 (`a2-holmes-deepseek-2026-09.md` §8: after typography, DeepSeek's
remaining gap to `gemini-3.1-flash-lite` is idiom calques, collocation and naturalness, which "need
a second model judging DeepSeek's output") and on T1 (`typography-2026-09-22.md` §10: with the
rubric fix and the pass, DeepSeek+repair is level with g31 in ru / fr and 46 % in de).

The question: does adding the converter's existing judge + escalate gates, with
`google/gemini-3.7-flash` (g37) on OpenRouter as both judge and re-translator, lift the gonka
DeepSeek pipeline measurably — and at what price per book?

Arms built 2026-09-26; blind judging 2026-09-26 … 2026-09-28 (interrupted once, resumed; nothing
lost). Status of every pair set is in §4; results are appended as they land.

## 1. Setup

| item | value |
|---|---|
| excerpt | Sherlock Holmes ch. 1–4 (`quality-plan/holmes.epub --limit-chapters 4`), 1 667 sentences, 1 624 translated cells per language |
| binary | `bin/convert-q1` (frozen 2026-09-26 14:53, built from the T1 tree; same producer as `convert-typo2` plus the typography `«—` follow-up) |
| translator | `deepseek-ai/DeepSeek-V4-Flash` on gonka, `--repair`, glossary = A2's dsr glossary (copied per arm), `--typography` |
| translations | taken from each arm's seeded cache (the A2 dsr cache): no arm re-translated with DeepSeek, so wall times below are gates + judge + escalation only |
| judge / escalation | `--judge --judge-provider openrouter --judge-model google/gemini-3.7-flash --escalate-provider openrouter --escalate-model google/gemini-3.7-flash` |
| scopes | **flagged**: `--judge-scope flagged` = lexcheck/low-coverage suspects + a fixed calibration sample of ~81 clean sentences; **all**: `--judge-scope all` = every translated cell |
| escalation set | judge-flagged ∪ lexcheck-flagged sentences, re-translated and re-aligned by g37, re-verified, one re-escalation round for what is still flagged; what stays flagged ships as listed in `*.unverified.json` |
| driver | `.artifacts/q1/run_q1.sh <base|flagged|all> <lang>`; wall times in `wall-times.tsv` |
| blind judge | `judge_pairs.py` fixed rubric (T1 §1), `claude -p --model sonnet`, one fresh process per batch of 10, both orders, `--strict-mcp-config --mcp-config '{"mcpServers":{}}'`; time-boxed windows via `typo/judge_window.py` (imports `judge_pairs.judge` unchanged); `analyze_pairs.py` (decisive only if both orders agree), `analyze_notes.py` |

**base** (`holmes4-q1base-{l}`) is `holmes4-dsr` + typography assembled offline by the same binary —
T1's `dsr-typo` text, except two Russian cells (ids 395, 399 of the g31 sample) where the frozen
binary's `«—` rule now differs from T1's text (§4.2).

### Pair sets

| set | what it measures | construction |
|---|---|---|
| `q1<arm>-vs-base-<l>` | does re-translation improve the cells it touches? | every cell whose arm text differs from base (`prepare_pairs_lang.py`, n ≤ 400, seed 1, min-words 1) |
| `q1<arm>-vs-g31-<l>` | the arm's share against g31 on the fixed A2/T1 400-pair sample | see "delta method" below |
| `base-vs-g37-<l>` | base against the upper anchor | A2's `dsr-vs-g37` sample (same 200 ids, swaps and g37 text), dsr text replaced by base text |

**The delta method** (`.artifacts/q1/build_sets.py`). The g31 comparison re-uses T1's already judged
`typo-vs-g31-<l>` set (de: `-v2`), i.e. A2's 400 ids, sources, g31 texts and swap flags, judged with
the fixed rubric on the `dsr-typo` text. For an arm, only the pairs whose text the arm changed
**visibly** (anything but whitespace / NBSP / U+202F) are rebuilt with the arm's text and judged
again: that is the `-delta` set. `build_sets.py merge` then writes `q1<arm>-vs-g31-<l>` = T1's
verdicts (both orders, both notes) for the unchanged pairs + the delta verdicts for the changed
ones, and the share is computed on the full 400 by the unchanged `analyze_pairs.py`. The share is
therefore T1's measured share adjusted exactly by the re-judged pairs, not an extrapolation. Two
consequences: (1) the carried-over pairs contain no new judge noise, the delta pairs do (T1 §5.3
measured the re-judging drift on unchanged pairs at ±5 pp, not significant); (2) the base arm gets
the same treatment (`q1base-vs-g31-<l>-delta`: 2 / 0 / 0 changed pairs), so base, flagged and all
are compared on one frame. `paired.py` reports, for each delta set, the before → after outcome of
each re-judged pair.

### Decision rule (set before judging)

Recommend a quality profile if **q1-flagged** beats base on the re-translated sentences
(`q1flagged-vs-base`) with ≥ 60 % of decisive pairs **in each language**, AND raises the share vs
g31 by ≥ 3 pp in at least two languages. The same test is stated for **q1-all** as the upper bound.

## 2. Arms

| arm | lang | judged | judge-flagged | escalated (round 1 / re-escalated) | left unverified | cells changed vs base (visible, emptied) | OpenRouter $ | requests (err) | wall |
|---|---|--:|--:|--:|--:|--:|--:|--:|--:|
| base | ru | — | — | — | — | — | 0 | 0 | 5 s |
| base | de | — | — | — | — | — | 0 | 0 | 4 s |
| base | fr | — | — | — | — | — | 0 | 0 | 5 s |
| flagged | ru | 100 (19 suspect + 81 calib.) | 14 | 30 / 9 | 8 | 29 (0) | 0.0899 | 85 (24) | 41 s |
| flagged | de | 91 (10 + 81) | 9 | 17 / 7 | 5 | 16 (0) | 0.0618 | 66 (19) | 31 s |
| flagged | fr | 109 (29 + 80) | 16 | 43 / 21 | 17 | 34 (1) | 0.1193 | 113 (29) | 45 s |
| all | ru | 1 624 | 277 | 294 / 28 | 13 | 288 (0) | 0.9807 | 722 (34) | 4 m 0 s |
| all | de | 1 624 | 289 | 298 / 30 | 19 | 280 (1) | 0.9996 | 718 (32) | 4 m 16 s |
| all | fr | 1 624 | 310 | 337 / 72 | 49 | 299 (1) | 1.1129 | 788 (40) | 5 m 31 s |

OpenRouter spend (`.artifacts/q1/LEDGER.md`, summed from the six `--stats` files, gonka requests
carry no price): **$3.3641 total** — flagged $0.2710 (ru 0.0899, de 0.0618, fr 0.1193), all $3.0932
(ru 0.9807, de 0.9996, fr 1.1129). Budget $5.00, hard stop $4.50; the blind judging is `claude -p`
and spends nothing on OpenRouter, so nothing further was charged. By phase, the all arms spend
≈ 45 % on judging, ≈ 45 % on **re-aligning** the escalated sentences (the escalation re-runs the
LLM aligner through OpenRouter) and ≈ 10 % on re-translation.

**Cost per 10 000-sentence book, per target language** (OpenRouter $ / 1 667 × 10 000; gonka
DeepSeek translation + repair is unpriced):

| profile | ru | de | fr | mean |
|---|--:|--:|--:|--:|
| flagged | $0.54 | $0.37 | $0.72 | **$0.54** |
| all | $5.88 | $6.00 | $6.68 | **$6.19** |
| for reference: g37 translating the whole book itself (A2 arm) | $2.21 | $1.77 | $1.90 | $1.96 |
| for reference: g31 (production default) | $0.85 | $0.69 | $0.83 | $0.79 |

Judging every cell with g37 and repairing a sixth of them costs about **three times** what g37
charges to translate and align the whole book directly.

Structural side effects:

- Every arm validates (0 offset / 0 structural errors); coverage is unchanged at 98 % target words.
- **Escalation can empty a good cell.** `flagged-fr`, `all-de` and `all-fr` each ship one sentence
  untranslated where base has a DeepSeek translation ("ship untranslated" in the escalation
  summary; `changed-*.jsonl` rows with an empty `q1`): fr "He was still, as ever, deeply attracted
  by the study of crime…", de "I had called upon my friend, Mr. Sherlock Holmes, one day in the
  autumn of last year…", fr "“Why, I have lost four pound a week.”". When escalation fails it
  should keep the original translation. This is a bug in the escalation path, not a quality
  trade-off.
- "Left unverified" sentences (still flagged after re-escalation) ship raw, with no word
  highlights: 8 / 5 / 17 (flagged), 13 / 19 / 49 (all). For a tap-to-translate reader that
  is a loss of function on those cells; the number of alignment-less sentences rises from
  19 / 18 / 18 (base) to 26 / 23 / 34 (flagged) and 31 / 36 / 66 (all).

## 3. Results already judged (as of 2026-09-28, start of this session)

### 3.1 q1-flagged vs base, on the cells it changed

| lang | pairs | ties | order-inconsistent | decisive | q1flagged : base | **q1flagged share** | p |
|---|--:|--:|--:|--:|--:|--:|--:|
| ru | 29 | 17 | 1 | 11 | 5 : 6 | **45.5 %** | 1.0 |
| de | 16 | 8 | 2 | 6 | 4 : 2 | **66.7 %** | 0.69 |
| fr | 33 | 13 | 2 | 18 | 13 : 5 | **72.2 %** | 0.096 |

Defect classes against each side (judgements, both orders): ru — q1flagged 17 (fidelity 12),
base 14 (fidelity 9); de — 8 vs 12; fr — 14 vs 31 (base loses on collocation 7, register 4,
punctuation 3). In Russian the re-translation is not preferred: it loses on fidelity as often as it
wins.

### 3.2 q1-flagged vs g31 (delta-merged, 400 pairs)

| lang | delta pairs re-judged | delta: before → after (arm : g31), better / worse | **q1flagged share (400)** | T1 dsr-typo share | Δ |
|---|--:|--:|--:|--:|--:|
| ru | 13 | 3 : 5 → 5 : 1, 5 / 1 | 81 : 72 = **52.9 %** (p = 0.52) | 51.0 % | +1.9 pp |
| de | 3 | 1 : 0 → 0 : 0, 0 / 1 | 70 : 82 = **46.1 %** (p = 0.37) | 46.4 % | −0.3 pp |
| fr | 17 | 2 : 4 → 4 : 4, 4 / 2 | 93 : 90 = **50.8 %** (p = 0.88) | 50.3 % | +0.5 pp |

The flagged scope touches only 3–17 of the 400 sampled pairs (0.8–4 %), so it cannot move the share
by 3 pp even when every touched pair improves.

### 3.3 q1-all vs g31, Russian (delta-merged)

| lang | delta pairs | delta: before → after, better / worse | **q1all share (400)** | Δ vs T1 |
|---|--:|--:|--:|--:|
| ru | 85 | 14 : 33 → 19 : 12, 35 / 11 | 84 : 55 = **60.4 %** (p = 0.017) | **+9.4 pp** |

The first significant win of a DeepSeek-based arm over g31 in any language. Defect classes (400
pairs): judgements against q1all 177 vs g31 242; q1all's fidelity losses fall from 89 (flagged) to
76, g31's rise to 106.

## 4. Judging in this session

(appended below as each set lands)

### 4.1 q1-all vs g31, German and French (delta-merged) — landed 2026-09-28

The remaining 1 (de) and 23 (fr) delta batches were judged; one fr batch returned a malformed
answer on the first try and was re-judged in the next window. No 0-byte verdict files.

| lang | delta pairs | delta: before → after (arm : g31), better / worse | **q1all share (400)** | p |
|---|--:|--:|--:|--:|
| de | 82 | 20 : 22 → 17 : 15, 20 / 19 | 68 : 75 = **47.6 %** | 0.62 |
| fr | 113 | 22 : 29 → 29 : 18, 36 / 22 | 98 : 79 = **55.4 %** | 0.18 |

Defect classes (400 pairs, judgements q1all : g31): de — naturalness 52 : 23, idiom-calque 30 : 7,
collocation 28 : 12 remain DeepSeek-side excesses almost unchanged from base (54 : 29, 34 : 7,
28 : 11); fr — fidelity 71 : 90 (base 83 : 80) and idiom-calque 34 : 25 (base 48 : 21) improve.

### 4.2 q1-base vs g31 (delta-merged) — landed 2026-09-28

Only two Russian pairs differ from T1's `dsr-typo` text (395, 399: the frozen binary's `«—` → `«`
rule removes the dialogue dash after the opening guillemet, T1 §7 follow-up); German and French
are byte-identical up to invisible spaces, so their base share is T1's.

| lang | delta pairs | delta before → after | **q1base share (400)** | p |
|---|--:|--:|--:|--:|
| ru | 2 | 0 : 2 → 0 : 0 (both now ties), 2 / 0 | 79 : 74 = **51.6 %** | 0.75 |
| de | 0 | — | 71 : 82 = **46.4 %** | 0.42 |
| fr | 0 | — | 91 : 90 = **50.3 %** | 1.0 |

These are the reference for the "≥ 3 pp vs g31" test. Every arm's delta set contains 395 and 399
in Russian (their text differs from T1's for the same reason), so the Δ below is arm − q1base:

| lang | q1base | q1flagged | Δ flagged | q1all | Δ all |
|---|--:|--:|--:|--:|--:|
| ru | 51.6 % | 52.9 % | +1.3 pp | 60.4 % | **+8.8 pp** |
| de | 46.4 % | 46.1 % | −0.3 pp | 47.6 % | +1.2 pp |
| fr | 50.3 % | 50.8 % | +0.5 pp | 55.4 % | **+5.1 pp** |

**q1-flagged fails the g31 half of the decision rule** (no language reaches +3 pp) and the
base half in Russian (45.5 %, §3.1). **q1-all passes the g31 half** (ru, fr); its base half is
§4.3.

### 4.3 q1-all vs base, on the cells it changed — landed 2026-09-28

All 174 batches judged in full (no sample was needed; the session was interrupted twice — a laptop
suspend and a spend-limit stop — and resumed from the verdict files on disk; no 0-byte files).
Four batches returned malformed JSON once and were re-judged (one with `judge_pairs.py`'s own
3-attempt loop).

| lang | pairs (all differing cells) | ties | order-inconsistent | decisive | q1all : base | **q1all share** | p |
|---|--:|--:|--:|--:|--:|--:|--:|
| ru | 288 | 153 | 14 | 121 | 82 : 39 | **67.8 %** | 1.2e-04 |
| de | 279 | 135 | 16 | 128 | 91 : 37 | **71.1 %** | 2.0e-06 |
| fr | 298 | 136 | 25 | 137 | 94 : 43 | **68.6 %** | 1.6e-05 |

Defect classes, judgements against q1all : against base — ru: fidelity 54 : 109, idiom-calque
7 : 22, terminology 2 : 11, case-government 1 : 8; de: fidelity 33 : 82, naturalness 27 : 45,
punctuation 1 : 13; fr: fidelity 46 : 97, idiom-calque 17 : 35, collocation 12 : 27,
grammar-agreement 1 : 14. Totals 118 : 219, 115 : 236, 144 : 237. The re-translations fix mainly
**fidelity** (the base loses on it twice as often) and then calques / collocations — exactly the
classes A2 said DeepSeek's own repair does not see. Naturalness is the one class where g37's
re-translation is not better (ru 36 : 34, fr 29 : 22).

Why flagged scope fails where all scope succeeds: the flagged scope's suspects are lexcheck
low-support cells (mostly names, archaisms and rare words, where DeepSeek is usually right and a
re-translation is a coin toss — ru 45.5 %), plus a ~81-sentence random calibration sample that
surfaces only 9–16 real judge flags. The all scope finds ≈ 17–19 % of cells flagged, and on those
the second model is clearly better.

German is the odd case: q1all wins 71 % of its changed cells against base, but on the g31 sample
the 82 re-judged pairs come out 20 better / 19 worse (+1.2 pp). This study does not explain the
gap; the paired view is in `paired.py`'s output for `q1all-vs-g31-de-delta`. It stays 47.6 % (n.s.).

### 4.4 base vs g37 (A2's 200-pair g37 sample, dsr text → base text) — landed 2026-09-28

| lang | A2 dsr vs g37 (old rubric) | ties | order-inconsistent | decisive | base : g37 | **base share** | p |
|---|--:|--:|--:|--:|--:|--:|--:|
| ru | 31 : 53 (36.9 %) | 113 | 12 | 75 | 34 : 41 | **45.3 %** | 0.49 |
| de | 30 : 60 (33.3 %) | 97 | 15 | 88 | 31 : 57 | **35.2 %** | 0.0073 |
| fr | 29 : 68 (29.9 %) | 106 | 8 | 86 | 30 : 56 | **34.9 %** | 0.0067 |

(The A2 column mixes the rubric change with typography; there is no fixed-rubric dsr-vs-g37
baseline, so the ru +8 pp / de +2 / fr +5 movement is not decomposed.) After typography, Russian
DeepSeek is no longer significantly behind g37; German and French still are, by ~30 pp. Defect
classes, judgements against base : against g37 — de idiom-calque 27 : 8, collocation 17 : 4;
fr fidelity 65 : 20, collocation 21 : 3, idiom-calque 20 : 12; ru is balanced (114 : 105).

Extra step, not in the original order (it needs base-vs-g37 as its baseline and costs one judging
window): `build_sets.py g37 all flagged` built the same delta sets against g37 — q1all changes
52 / 44 / 45 of the 200 pairs, q1flagged 5 / 0 / 6.

### 4.5 q1 arms vs g37 (delta-merged on the 200-pair g37 sample) — landed 2026-09-29

Same delta method as §1, with `base-vs-g37-<l>` as the baseline (its base text replaced by the
arm's text wherever it differs visibly). All 36 delta batches judged; q1flagged-de changes none of
the 200 pairs.

| lang | q1all delta pairs: before → after (arm : g37), better / worse | **q1all share** (p) | q1flagged delta | **q1flagged share** (p) | base share |
|---|--:|--:|--:|--:|--:|
| ru | 52: 6 : 14 → 6 : 9, 11 / 7 | 34 : 36 = **48.6 %** (0.91) | 5: 0 : 2 → 0 : 3 | 34 : 42 = 44.7 % (0.42) | 45.3 % |
| de | 44: 6 : 14 → 2 : 10, 11 / 10 | 27 : 53 = **33.8 %** (0.0049) | 0 | 31 : 57 = 35.2 % (0.0073) | 35.2 % |
| fr | 45: 6 : 14 → 4 : 6, 15 / 9 | 28 : 48 = **36.8 %** (0.029) | 6: 2 : 1 → 0 : 1 | 28 : 56 = 33.3 % (0.0030) | 34.9 % |

(The three "before 6 : 14" rows are a coincidence of three different id sets; the merged totals
reconcile exactly with §4.4, e.g. de 31 − 6 + 2 = 27, 57 − 14 + 10 = 53.) Even judging every cell
and re-translating a sixth of the book with g37, the DeepSeek pipeline stays significantly behind
g37 translating on its own in German and French, and reaches it only in Russian.

## 5. Decision

All pair sets required by the rule are complete (§3–§4); the merged `q1*-vs-g31-*` /
`q1*-vs-g37-*` directories hold only `verdict-all-*` / `notes-all-*` files by design — their
batches live in the `-delta` and baseline sets.

| test | ru | de | fr | verdict |
|---|--:|--:|--:|---|
| q1-flagged vs base, share on changed cells (≥ 60 % each) | 45.5 % | 66.7 % | 72.2 % | **fail** (ru; also no language significant, n = 11 / 6 / 18 decisive) |
| q1-flagged vs g31, Δ vs base (≥ +3 pp in two) | +1.3 | −0.3 | +0.5 | **fail** (0 of 3) |
| q1-all vs base, share on changed cells | 67.8 % | 71.1 % | 68.6 % | pass (all p ≤ 1.2e-04) |
| q1-all vs g31, Δ vs base | **+8.8** | +1.2 | **+5.1** | pass (ru, fr) |

**No quality profile is recommended on the flagged scope.** It is cheap ($0.54 per 10 000-sentence
book per language) but it touches 1–2 % of cells, and in Russian its re-translations are not
preferred; the effect on the book is within judge noise in all three languages. No
`--profile quality` preset was implemented.

**The all scope passes both halves as the upper bound**, and is the first DeepSeek-based
configuration to beat g31 significantly (Russian 60.4 %, p = 0.017). Its flag set, for the record:

```
--provider gonka --repair --typography \
--judge --judge-scope all --judge-provider openrouter --judge-model google/gemini-3.7-flash \
--escalate-provider openrouter --escalate-model google/gemini-3.7-flash
```

It is not recommended as a shipped profile either, for three reasons:

1. **Price.** $6.19 per 10 000-sentence book per language (ru 5.88 / de 6.00 / fr 6.68), about
   **3 × what g37 costs to translate and align the whole book itself** ($1.96) and 8 × g31 ($0.79).
   Half of it is re-aligning escalated sentences through OpenRouter.
2. **It does not reach g37.** Against g37 it is 48.6 / 33.8 / 36.8 % (de, fr significantly
   behind, §4.5). For a paid quality tier, g37 alone is cheaper and better in de / fr on this
   evidence (g37 + typography was not judged).
3. **Escalation defects.** One good cell is emptied in de and in fr (§2), and 13 / 19 / 49
   sentences ship without word highlights — a functional regression for a tap-to-translate book.
   The emptied-cell case is a bug to fix before any judge-scope-all use.

If a cheaper version is wanted, the data point to two levers, not tested here: a judge scope
between flagged and all (the judge flags ≈ 17–19 % of cells; a cheap first-pass judge could
select them), and escalation that re-uses the local aligner instead of the LLM aligner (≈ 45 % of
the all-scope cost).

## 6. Artefacts

All under `converter/bench-quality/.artifacts/q1/`:

| path | what |
|---|---|
| `run_q1.sh`, `wall-times.tsv` | arm driver and wall times |
| `holmes4-q1{base,flagged,all}-{ru,de,fr}.{tbook,log,stats.jsonl}`, `*.flagged.json`, `*.unverified.json`, `*.lexflagged.json` | arms, logs, per-request stats, gate outputs |
| `ledger.py`, `LEDGER.md` | OpenRouter cost ledger ($3.3641) |
| `changed-q1{flagged,all}-{l}.jsonl` | every cell the arm changed vs base (`src`, `base`, `q1`) |
| `build_sets.py` (`build`, `merge`, `g37 <arms>`) | all pair sets and the delta merge |
| `paired.py` | before → after per re-judged pair of a delta set |
| `pairs/q1{flagged,all}-vs-base-{l}/` | §3.1, §4.3 |
| `pairs/q1{base,flagged,all}-vs-g31-{l}{,-delta}/` | §3.2, §3.3, §4.1, §4.2 (merged + delta) |
| `pairs/base-vs-g37-{l}/`, `pairs/q1{flagged,all}-vs-g37-{l}{,-delta}/` | §4.4, §4.5 |

Every judged directory has `result.txt` (`analyze_pairs.py`) and `notes.txt` (`analyze_notes.py`).
