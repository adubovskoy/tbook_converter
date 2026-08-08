# Quality research on gonka — what almost-free tokens change — 2026-07-25

A continuation of [`gonka.md`](gonka.md), which established: `--provider gonka` + Kimi-K2.6
converts a book for a fraction of a cent, but in blind evaluation scores 7.14/10 against 7.95 for
`google/gemini-3.1-flash-lite`. Price is no longer the constraint (~$0.000117 per 1M tokens), so
this round answers the question: what can the freed-up budget buy for the two things a language
learner actually needs from a reader:

- **(a) markup** — every source word highlights the word (or words) of the translation that
  translate it;
- **(b) translation** — accurate, safe-to-imitate Russian where idioms, phrasal verbs, slang
  and set expressions are rendered as units and in the right register.

Hard constraint: **≤30 min per book**. All production arms were run on gonka.

## Setup

- **Book / dev set**: *Altered Carbon* (Richard K. Morgan), chapters 1-5 — 1303 sentences
  (1279 unique), en→ru. Full book: 46 chapters, 14 688 sentences (14 082 unique),
  158 421 words.
- **Pipeline**: converter defaults — batch 8 (gonka), concurrency 32, glossary on,
  hybrid alignment (local LaBSE + LLM fallback), lexcheck on. A fresh `--cache-dir`
  per arm, `--stats` on every run.
- **Judging**: blind pairwise. Each pair is judged **in both presentation orders** by
  independent Claude agents; a pair counts as *decisive* only if both orders agree after
  de-swapping — any disagreement or an explicit tie goes to the ties. The significance criterion
  is a sign test over decisive pairs. Rubric priority: correct Russian > fidelity and terminology >
  idiom and register > naturalness. Harness: `bench-quality/prepare_pairs.py`,
  `bench-quality/analyze_pairs.py`.
- **New tools** (in the repo, offline): `cmd/tbookdiff` (word-level diff of two `.tbook`s),
  `cmd/score` (tap coverage on both sides + alignment of expressions as units),
  `bench-quality/probe_align.py` (word coverage inside expressions),
  `bench-quality/probe-dev.json` (194 manually checkable probes for idioms, phrasal verbs,
  slang and set expressions, mined from the book).
- Converter revision: 11f2f5b.

## 1. Throughput calibration (Stage 0)

| arm | config | translate | align | judge | wall | coverage tgt/src | lexcheck |
|---|---|---|---|---|---|---|---|
| kimi-base | defaults, hybrid | 32 s, 40.4 sent/s, p50 4.8 s | emb 48 s; 36 sent. (2.8%) to LLM | — | **2m02s** | 99% / 97% | 24 (1.9%) |
| align-llm b4 | `--align-mode llm --align-batch 4` | from cache | 3m40s, **5.8 sent/s** | — | 3m53s | 94% / 89% | 28 |
| align-llm b8 | `--align-batch 8` | from cache | 2m38s, **8.1 sent/s** | — | 2m54s | 95% / 90% | 31 |
| judge-all j3 | `--judge-scope all`, batch 4 | from cache | from cache | 695 requests, **418× 429**, 238 retries | 4m17s | — | flagged **635/1279 = 50%** |
| judge-all j3 | judge batch 16 | from cache | from cache | 201 requests, 98× 429, p90 **277 s** | 5m50s | — | flagged 608 = 48% |
| MiniMax-M2.7 | `--batch-size 4` | 6m07s, **3.5 sent/s**, 63 truncations | 4m48s on 39 sent. | — | 12m23s | 98% / 96%, **16 empty** | 27 |

Four rejections, all on measurements:

1. **LLM-align-all: rejected.** 5.8-8.1 sent/s (28-42 min for alignment alone on a book) *and*
   coverage worse than hybrid (94-95%/89-90% against 99%/97%). Dual-align reconcile dies with
   it — the LLM pass has nothing to add on coverage.
2. **Judge-all (j3): rejected.** 45-65 min per book — the network answers small judge batches with
   hundreds of 429s and a p90 tail of 277 s — and it flags ~50% of its own arm, reproducing on
   gonka the very over-flagging that killed escalation in the speed report.
3. **MiniMax-M2.7: definitively dead.** Batch 4 does not cure it: 3.5 sent/s (worse than 5.2 at
   batch 16), 63 truncations, 16 sentences lost, alignment tail at 0.1 sent/s → ~2.4 h/book.
4. **The gonka gateway caches identical requests** (a repeat comes back in 1.0 s with byte-identical
   text; temperature 0.90 → 0.91 forces a fresh generation). Consequence for the methodology: paired
   arms come out noise-free (translations behave like a fixed seed), and any best-of-N experiment
   must vary temperature or a nonce per sample.

Kimi baseline on the full book: translate 5m20s (44.0 sent/s) — see §5.

## 2. Re-baseline: Kimi against gemini with the new tools (Stage 2)

Same 5 chapters, reference arm gemini-3.1-flash-lite: 1m10s, $0.086, coverage 98%/96%, lexcheck 33,
55 sentences went to the LLM aligner (against 36 for Kimi). The translation text differs in 84.3% of
sentences; on alignment quality — a tie (content-word coverage 0.993 for Kimi against 0.989
for gemini).

**Blind pairwise, 300 pairs, both orders, 11 judges: gemini 97 — Kimi 47 of 144 decisive
(67.4% against 32.6%), p = 3.8e-05** (126 ties + 30 order-inconsistent).

The judges' notes localize the gap precisely, and it is *not* mainly idioms: Kimi loses on
**grammar** — agreement («мы вошёл», «была высокомерие», «три высокие силуэты», «широкая
щелевая рот»), non-existent words («ударала», «дожемиллениальные»), broken collocations,
case government («мне заверили»). gemini's own losses are lexical misses (lift→«лифт»,
mohican→«могиканин», particle→«частичный»). This reoriented all of Stage 3.

## 3. Translation-quality levers (Stage 3)

### 3a. Idiom-aware translate prompt v6 — REJECTED

An explicit block about idioms, phrasal verbs, slang and set expressions was added, plus an
exception to the existing rule "prefer a rendering that MIRRORS the source" (which, at first
glance, itself invites calques).

- Changed **48.9%** of sentences (637/1303), wall 1m49s, coverage 99%/96%.
- **Blind pairwise: base 69 — v6 67 of 136 decisive (50.7% / 49.3%), p = 0.93. A clean zero.**
- Idiom-targeted judging (131 probes, judges are told which expression to assess):
  base 9 wins, v6 8 wins, 113 "both good", 1 "both bad" — also zero. v6 did cure
  "the shit hit the fan" (previously the calque «дерьмо ударило в вентилятор»), but lost
  elsewhere.
- Word coverage inside expressions is slightly *worse*: 92.2% against 93.5% (a freer translation
  aligns worse).

Conclusion: Kimi already renders most idioms acceptably (113/131 "both good"), and the mirroring
rule was not the cause of the calques the judges saw. The patch is kept in
`bench-quality/translate-v6.patch`, it does not go to production.

### 3b. Repair (proofread) pass — ACCEPTED

A new harness, `cmd/repair`: it re-reads the raw translations already cached by a run, sends
batches of `{id, src, tr}` through a proofreader prompt and writes the result into a *different*
cache dir under the same raw-translation key — so `convert --cache-dir <dst>` aligns and assembles
the repaired text **without re-translating** (verified: "0 to translate"), and the two books differ
by exactly one variable. Judging is over the sentences it actually changed:

| mode | changed | decisive | repair wins | p | net (improved − degraded) |
|---|---|---|---|---|---|
| v1, no glossary guard | 49 (3.8%) | 42 | 83.3% | 1.5e-05 | ~33 sentences |
| strict + glossary | 23 (1.8%) | 16 | **87.5%** | 0.004 | ~17 |
| **`--fluent` + glossary — ACCEPTED** | 38 (3.0%) | 31 | **87.1%** | 3.4e-05 | **~28** |
| `--fluent --bold` | 58 (4.5%) | 43 | 76.7% | 0.0006 | ~31 |

- **Fixes** exactly the defect classes the re-baseline exposed: agreement («железное
  стойкость»→«железную», «зигзагообразная шрам»→«зигзагообразный», «мы вошёл»→«мы вошли»),
  government («Внутри дом»→«Внутри дома», «отпустят под арестом»→«из-под ареста»),
  non-existent words («задранила»→«задрала»), word order («Обычно я такой не»→«Обычно я не
  такой») and some idioms («Огненный взрыв»→«Осторожно, взрыв» for *Fire in the hole*;
  «Где посуда?»→«Где железо?» for *'ware*).
- **The glossary guard is load-bearing.** Without it the pass "corrected" the book's own term
  «оболочка» (*sleeve* here means a body) into the literal «рукав», guessed a character's gender
  and broke an already-correct idiom. Injecting a forced glossary plus the rule *"you see a single
  sentence without context — never 'fix' a person's gender, a pronoun's referent, or a recurring
  term"* removed all three classes.
- **Coverage is limited, and buying more of it costs precision:** removing the "most edits are not
  needed" anchor raises coverage from 3.0% to 4.5%, but drops precision from 87% to 77% at the same
  net gain. A second pass over the repaired text yields only +14 edits (1.1%).
- **Cost:** +32 s on 1279 sentences (~+6 min per book). Alignment is untouched
  (93.5%/82.4% expression coverage — same as the base); lexcheck 25 against 22-24.

The honest boundary: repair removes gross errors on ~3% of sentences at 87% precision. It does
**not** close the 67/33 gap with gemini — that gap consists of thousands of small lexical and
stylistic preferences across the whole book, and a proofreader forbidden to paraphrase is obliged
to leave them alone.

### 3c. Repair with neighbouring-sentence context (`--context N`) — best on quality, rejected on time

The repair pass's residual losses were context-free guesses: a character's gender where English is
neutral, a pronoun's referent. Hypothesis test: a `prev` field in the batch — N preceding
sentences with their FINISHED translations, read-only; in this mode the rule changes from
"never fix gender" to "where prev settles it — fix; where it does not — leave it alone".

| mode | changed | decisive | precision | p | net* |
|---|---|---|---|---|---|
| fluent + glossary | 38 (3.0%) | 31 | **87.1%** | 3.4e-05 | 28.2 |
| fluent + bold | 58 (4.5%) | 43 | 76.7% | 0.0006 | 31.0 |
| fluent + **ctx1** | 34 (2.7%) | 25 | **80.0%** | 0.004 | 20.4 |
| fluent + **ctx2** | 59 (4.6%) | 45 | **84.4%** | **3.1e-06** | **40.6** |

*`changed × (wins − losses) / decisive`.

**The main finding is non-monotonicity: one sentence of context is WORSE than none** (80.0%
against 87.1%). One sentence back gives the model enough confidence to reach for the character's
gender, but not enough information to guess right. A direct illustration: «врач встала» →
«встал» (the character is female in the book) was broken by v1, fluent **and** ctx1 — while ctx2
did not touch it at all. Two sentences give a real anchor; partial context is worse than none.

Context opens a **new** class of corrections rather than replaying the old ones: the intersection
of ctx2's edits with fluent's is only 25 of 59.

**But the time cost kills the lever.** On the full book the repair phase: **15m13s against 5m09s** —
three times more expensive (p50 11.8 s against 5.2 s, 245 429 refusals, 4.08M input tokens, +51%).
Book-level totals:

| mode | changed (book) | precision | net sentences | repair phase | whole recipe |
|---|---|---|---|---|---|
| fluent (no context) | 544 (3.9%) | 87.1% | ~404 | 5m09s | **18m36s** ✓ |
| fluent + ctx2 | 672 (4.6%) | 84.4% | ~463 | 15m13s | **31m41s** ✗ |

**ctx2 buys +59 sentences out of 14 688 (+0.42% of the book) and blows the 30-minute budget.**
The dev set overestimated the lever: there ctx2 changed 59 against 38 for fluent (+55% coverage),
while on the full book it is 672 against 544 — only +24%, and at lower precision the advantage
nearly dissolved. On alignment ctx2 is in fact slightly the best of all (expression word coverage
94.0% against 93.5% for the base and 93.0% for fluent), and content-word coverage is balanced
(dropped in 30 sentences, rose in 29).

Verdict: **the default is `--fluent` without context**; keep `--context 2` as a documented option
for runs where time is not constrained; never use `--context 1`.

## 4. Alignment levers (Stage 4)

The expression-level metric (`probe_align.py`): what share of the words *inside* a known
multi-word expression highlight anything at all.

| arm | expression word coverage | fully "tappable" | lexcheck |
|---|---|---|---|
| **kimi-a, hybrid (default)** | **93.5%** | 82.4% | 22-24 |
| gemini-ref (its own translations) | 95.0% | 86.3% | 33 |
| kimi-v6 (its own translations) | 92.2% | 80.2% | 26 |
| glue threshold 0.3 against 0.2 | 92.2% (identical) | 80.2% | 22 |
| emb-only, no LLM tail | 91.5% | 79.4% | 32 |
| LLM-align-all b8 | 93.2% | 85.5% | 31 |

- **Glue thresholds do nothing.** `EMBALIGN_GLUE_MIN` 0.3 → 0.2 gave a byte-identical result:
  the threshold is not the binding constraint.
- **The LLM fallback pays for itself.** A paired comparison on the same text (0 translation
  differences): hybrid 92.2% against emb-only 91.5% expression coverage and **22 against 32
  lexcheck flags**.
- **The residual gap is content words inside idioms**, not only function words: *day* in
  "Don't give up the day job" → «основную работу», *fair* in "my fair share of" → «немало»,
  *point* in "on the point of". LaBSE does not link them (the cosine really is low) — which is
  exactly why lowering the threshold changes nothing.

### 4c. Gluing by expression boundaries (`EMBALIGN_UNIT_GLUE`) — implemented, off by default

A deterministic step: if at least one word of a known expression is aligned, all words of the
expression are attached to the same target set (protection against "smearing" — a cap on set
size). Same translations, zero token cost.

| metric | glue off | glue on | Δ |
|---|---|---|---|
| expression word coverage | 93.5% | **100%** (circular — the boundaries come from the same eval file) | +6.5 pp |
| expression unit-rate (`cmd/score --probe`) | 77.9% | **93.9%** | +16 pp |
| source content-word tap coverage (whole book) | 0.979 | 0.982 | +0.3 pp |
| sentences with tap coverage below 0.80 | 30 | 25 | −5 |
| sentences with changed alignment | — | 88, **all of them probe ones, 0 collateral** | — |

A manual check of 23 new glue events: **18 correct, 2 partly wrong, 1 wrong, 2 incomplete**
(~78-83%). Every error reinforces an already-bad "anchor" rather than inventing a new one.

Two caveats are on record so that nobody cites this as proof of safety: the 100% figure is
circular by construction, and **lexcheck is structurally blind to this lever** (`CheckSentence`
counts a chunk as supported if *any one* of its source words is plausible, and unit-glue only
widens that set). Before rollout we need: (1) a real source of boundaries — an LLM pre-pass tagging
expressions on the source side, amortized across all target languages, or an idiom dictionary;
(2) a guard that actually fires (a target-side adjacency requirement would have turned most of the
residual errors into "no change"); (3) judging the taps themselves instead of coverage.

## 5. Full-book dress rehearsal (Stage 5)

The full *Altered Carbon* — 46 chapters, 14 688 sentences (14 082 unique), 158 421 words — on
gonka with converter defaults.

| phase | base | accepted recipe (`--fluent`) | rejected (`--context 2`) |
|---|---|---|---|
| translate | 5m20s (44.0 sent/s) | 5m20s (reused) | 5m20s (reused) |
| **repair** | — | **5m09s (45.6 sent/s), 544 changed (3.9%)** | 15m13s (15.4 sent/s), 672 changed (4.6%) |
| embalign (local CPU) | 7m24s (31.7 sent/s), 480 gated (3.4%) | 7m12s (32.6 sent/s), 489 gated (3.5%) | 8m25s (27.9 sent/s), 482 gated |
| LLM alignment tail | 54 s (9.0 sent/s) | 42 s (11.5 sent/s) | 2m26s (3.3 sent/s) |
| lexcheck + assembly | ~12 s | ~12 s | ~12 s |
| **total** | **14m07s** | **≈18m36s** ✓ | **≈31m41s** ✗ |

The spread of the alignment phase across arms (8m07s against 11m08s on identical work) is network
429s and machine load. That is a separate argument for keeping budget headroom instead of
squeezing it dry.

Both runs: validation **0 empty, 0 offset errors, 0 structural errors**; coverage **99% of
translation words aligned, 97% of source words highlightable**. Lexcheck 323 → 332 flags
(+9 of 13 847). `tbookdiff`: 545 sentences changed (3.7%), all alignment averages identical to the
third decimal (Δ = −0.000), content-word coverage dropped by more than 0.1 in 25 sentences and rose
in 11. Expression word coverage 93.5% → 93.0%.

`cmd/score` over the whole book confirms that repair is neutral to alignment:

| `cmd/score` (12 002 aggregated sentences) | base | with repair |
|---|---|---|
| target coverage, all / content words | 0.990 / 0.993 | 0.989 / 0.993 |
| source tap coverage, all / content words | 0.971 / 0.979 | 0.970 / 0.978 |
| sentences with content-word tap coverage below 0.80 | 316 (2.6%) | 321 (2.7%) |
| empty translations / sentences without alignment | 0 / 0 | 0 / 0 |

**The 30-minute budget holds with ~11 minutes to spare**, and at the measured 87% precision the
repair pass moves ~470 sentences from wrong to right against ~70 in the other direction — a net
~400 sentences (2.7% of the book) for 5 minutes and a fraction of a cent.

A note on cost: gonka returns token counters but no cost field, so `--stats` shows $0. At the
gateway's stated ~$0.000117 per 1M tokens the whole recipe costs noticeably less than a cent per
book; the gemini reference arm cost $0.086 for 5 chapters alone.

## 6. Verdicts

**Accepted**
- **The repair pass with the glossary guard, `--fluent`** — +6 min per book, 87% precision on the
  ~3% of sentences it touches, at no cost in alignment. This is exactly what almost-free tokens
  buy.
- Hybrid alignment stays the default; the LLM fallback on the ~3% gated sentences is justified.

**Rejected on measurements**
- The idiom-aware translate prompt v6 (null result, p = 0.93).
- LLM-align-all and dual-align reconcile (slower *and* worse on coverage).
- Judge-all with j3 (45-65 min per book, flags 50% of its own output).
- MiniMax-M2.7 at any batch size.
- Glue-threshold tuning (no effect).
- A second repair pass and the high-coverage `--bold` mode (no net gain).
- Repair with context `--context 2` — best on quality (84.4% precision, net +463
  sentences against +404), but three times more expensive in time and over budget (31m41s). Kept
  as an option. `--context 1` is worse than no context — do not use.

**Next**
- An expression-tagging pass to give unit-glue a real source of boundaries, plus an adjacency
  guard.
- Judge the taps themselves (not coverage) to honestly assess unit-glue precision.
- A phrasebook (a per-book glossary of idioms and slang) — untested; the glossary machinery
  already supports it via cache pre-seeding.

## Reproduction

The tools and per-arm artifacts live in `converter/bench-quality/`: `LOG.md` (raw run
log), `prepare_pairs.py` / `analyze_pairs.py` (the blind pairwise judging harness),
`probe_align.py`, `probe-dev.json` (the probe set), `ac-sentences.json`, `*-diff.json` (repair
diffs), `pairs-*/` (judge batches, keys and verdicts), `translate-v6.patch` (the rejected prompt).
