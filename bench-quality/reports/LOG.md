# gonka-quality research working log (2026-07-23/24)

Plan: ~/.claude/plans/gonka-ai-prancy-mist.md. Report on completion: ~/Develop/reader/benchmarks/gonka-quality.md.
Dev set: Altered Carbon ch.1-5 = 1303 sentences (1279 unique ≥ threshold), en→ru. HEAD 11f2f5b.
NB: gonka.md had 1279 sentences — segmentation has changed since then (chapter headings + quote splitting).

## Stage 0 — calibration (DONE, except MiniMax)

| Run | Config | translate | align | judge | wall | coverage tgt/src | lexcheck |
|---|---|---|---|---|---|---|---|
| A kimi-base | defaults (B=8 C=32 hybrid) | 32s, 40.4 sent/s, p50 4.8s | emb 48s 26.5 s/s; LLM 36 sent. (2.8%) | — | 2m02s | 99% / 97% | 24 (1.9%) |
| B retest | identical to A | 8s (155 s/s) — GATEWAY CACHE | same 36 gated | — | 1m17s | 99% / 97% | 24 |
| C align-llm-b4 | --align-mode llm --align-batch 4 | cache | 3m40s, **5.8 sent/s**, p50 16.8s | — | 3m53s | **94% / 89%** | 28 |
| D align-llm-b8 | --align-batch 8 | cache | 2m38s, **8.1 sent/s**, p50 25.3s | — | 2m54s | **95% / 90%** | 31 |
| E judge-all j3 | judge batch 4 | cache | cache | 695 req, 238 retry, **418×429**, p50 5.7s | 4m17s | — | **flags 635/1279 = 50%** |
| E2 judge-all j3 | judge batch 16 (--batch-size 32) | cache | cache | 201 req, 98×429, p50 8.1s, **p90 277s** | 5m50s | — | flags 608/1279 = 48% |
| F minimax-b4 | MiniMax --batch-size 4 | 6m07s, **3.5 sent/s**, 63 trunc, 49×429 | emb 43s; LLM 39 sent. in 4m48s (**0.1 s/s**) | — | 12m23s | 98% / 96%, **16 empty** | 27 |

### Stage 0 conclusions
1. **The gonka gateway caches identical requests** (confirmed by probe: repeat 1.0s + identical text; temp 0.90→0.91 → fresh generation). Consequences: (a) noise floor for paired A/B = 0 — translations are "frozen" like a fixed seed; (b) best-of-N requires varying temperature/nonce across samples; (c) repeat runs do not measure latency.
2. **LLM-align-all: REJECTED** (gate ≥20 sent/s): 5.8-8.1 sent/s (42/28 min per book for align alone) and coverage is WORSE than the hybrid (94-95%/89-90% vs 99%/97%). LaBSE hybrid stays. Dual-align loses its point (LLM-align adds no coverage).
3. **Judge-all j3: REJECTED for prod**: 45-65 min/book (the network answers 429 on small batches, p90 tail up to 277s) and a flag rate of ~50% — over-flagging reproduced on gonka (Kimi judges itself just as pedantically as gemini did in the speed report). j4 (quote-evidence) to be tested on the current scope: flagged + sample.
4. Kimi translate on gonka: steady ~40 sent/s at B=8 C=32, p50 4.8s, 0 errors, 0 truncations. Full book: translate ~6 min + embalign ~9 min + LLM-align tail ~1-2 min ≈ **17-19 min** — ~11-13 min of headroom.
5. Test translation of *kicked the bucket* without the learner prompt → «Старик пнул ведро» (calque) — a direct illustration of the research target.
6. **MiniMax-M2.7: DEFINITIVELY REJECTED.** Batch 4 does not cure it: 3.5 sent/s (worse than 5.2 at B=16), 63 truncations, 16 sentences lost, align tail 0.1 sent/s (reasoning). Projection ~2.4 h/book. All Stage-3 arms are Kimi-K2.6 only; MiniMax at most as a one-off merge/judge outside the budget.

## Stage 1 — tooling
- `cmd/tbookdiff` READY (self-diff clean; mutation test catches regressions). Matching by src+occurrence, coverage recomputed identically for both sides.
- `cmd/score` READY. Full July .tbook: tgt content coverage 94.1%, **src tap content coverage 90.1%**, 16.7% of sentences < 0.8 — the Stage 4 gap. Probe mode (unit/partial/unaligned) works.
- PROBE mining: 864 unique candidates (idiom 340 / phrasal 233 / fixed 154 / slang 137) in scratchpad/probe-raw.json; chunks 0, 2, 5 NOT covered (Claude session limit, reset 05:30) — and chunk 0 covers the dev-set chapters: only 21 probes in ch1-5 so far. Mine chunks 0/2/5 with three agents after the reset, then assemble PROBE-300 (category balance, ch1-5 subset for score --probe).
- GOLD-lite and the pairwise Claude harness — TODO (after the limit resets).
- Data: scratchpad = /tmp/claude-1000/-home-adubovskoy-Develop-reader-converter/0885fa0b-ae38-4344-99c4-be33e4295d59/scratchpad (ac-sentences.json — 9599 sentences with chapter numbers; probe-chunk-NN.json; probe-raw.json).

## Stage 2 — re-baseline (in progress)
- gemini-3.1-flash-lite arm: 1m10s, $0.086, coverage 98%/96%, lexcheck 33 (2.7%), gated 55 (vs Kimi 36) — gemini's freer translation aligns slightly worse.
- tbookdiff kimi-a vs gemini-ref: text differs in 84.3% of sentences (194 identical!); coverage practically equal, Kimi slightly higher (content 0.993 vs 0.989; tap content 0.983 vs 0.981).
- Blind pairwise: 300 pairs (seed 1, ≥6 words, both presentation orders), 10 judge agents (Sonnet), rubric fidelity > idioms/register > naturalness. Directory bench-quality/pairs-kimi-gemini/. Analysis: analyze_pairs.py (de-swap, decisive/tie, sign test).
- **RESULT: gemini 97 / kimi 47 out of 144 decisive (67.4% vs 32.6%), p=3.8e-05; 126 ties + 30 inconsistent.** Stage 3 target: raise Kimi to ≥45% of decisive pairs (parity gate). Judge notes: Kimi has idiom calques («дерьмо ударило в вентилятор» ← shit hit the fan; «он терял меня» ← he was losing me; «сорвать крышку с усталости» ← uncap my weariness) and agreement errors («мы вошёл», «была высокомерие»); gemini has lexical misses (lift→«лифт», mohican→«могиканин», particle→«частичный»), direction errors/tautologies.

## Stage 3 — translation quality levers

### 3a. Translate v6 (idiom block + carve-out from the mirror rule)
Patch: bench-quality/translate-v6.patch (edits prompt.go, applied to the working tree; binary bench-quality/convert-v6).
Arm kimi-v6: 28s translate (45.1 sent/s), wall 1m49s, coverage 99%/96%, lexcheck 26, gated 38. Text changed in **48.9%** of sentences (637/1303).
- Alignment: barely touched (content coverage 0.993→0.992, tap content 0.983→0.981).
- **Idiom metric (131 probes, blind judges who know the expr): base 9 wins / v6 8 wins / 113 both-good / 1 both-bad — ZERO effect.** v6 won on "the shit hit the fan" (previously the calque «дерьмо ударило в вентилятор») but lost on "put itself to bed", "gave a damn about", "break the habit" and others.
- Alignment probe unit-rate: base 77.9% / v6 74.0% / gemini 78.6% (v6 slightly worse — freer translation).
- **Overall pairwise (300 pairs, both orders, 11 judges): base 69 / v6 67 out of 136 decisive (50.7% vs 49.3%), p=0.93 — A CLEAN ZERO.** 138 ties + 26 inconsistent.
- **VERDICT 3a: REJECTED.** The idiom block + carve-out change 49% of translations and buy nothing: neither overall quality nor the idiom metric; alignment is slightly worse (92.2% vs 93.5% expression word coverage). Explanatory hypothesis: Kimi already renders most idioms acceptably (113/131 both-good), and the "mirror" rule was not the cause of the observed calques. Patch kept (translate-v6.patch), does NOT go to prod.

### 3b. Repair (proofread) pass — WINNER
Tool `cmd/repair` (new, additive): reads raw translations from an arm's cache, runs batches of {id,src,tr} through a proofreader prompt, writes the result into another cache dir under the same TrKey → `convert --cache-dir dst` finishes alignment without re-translating (verified: "0 to translate"). Flags: `--fluent` (also fix non-idiomatic phrasing), `--bold` (drop the "most sentences need no edit" anchor), `--no-gloss-guard`, `--diff`. A trap the tool closes on its own: it copies `glossary-*.json` into dst, otherwise the next convert generates a new glossary, gets a different namespace and re-translates the book.

Judging — only on CHANGED sentences (blind, both presentation orders, independent agents):

| mode | changed | decisive | repair wins | p | net gain (improved−degraded) |
|---|---|---|---|---|---|
| v1 without glossary guard | 49 (3.8%) | 42 | 83.3% | 1.5e-05 | ~33 sent. |
| strict + glossary | 23 (1.8%) | 16 | **87.5%** | 0.004 | ~17 |
| **fluent + glossary ← ACCEPTED** | 38 (3.0%) | 31 | **87.1%** | 3.4e-05 | **~28** |
| fluent+bold | 58 (4.5%) | 43 | 76.7% | 0.0006 | ~31 |

- **What it fixes:** agreement («железное стойкость»→«железную», «зигзагообразная шрам»→«зигзагообразный», «мы вошёл»→«мы вошли», «любимый звезда»→«любимая»), case government («Внутри дом»→«Внутри дома», «отпустят под арестом»→«из-под ареста»), non-existent words («задранила»→«задрала»), broken word order («Обычно я такой не»→«Обычно я не такой»), idiom calques («Огненный взрыв»→«Осторожно, взрыв» for *Fire in the hole*), slang («Где посуда?»→«Где железо?» for *'ware*).
- **What it broke without the guard (and stopped breaking with it):** the book's term «оболочка» (sleeve = body) → «рукав»; guessing a character's gender without context; corrupting an already correct idiom. Cured by injecting the glossary plus the rule "you see a single sentence without context — do not 'fix' gender, referent or a recurring term".
- Residual losses even with the guard: character gender where English is neutral («католичку»→«католика» — the character is female per the book), rare invented details.
- **Cost:** +32 s on 1279 sentences (≈+6 min per book), alignment does NOT degrade (93.5%/82.4% — exactly like the base), lexcheck 25 vs 22-24.
- A second pass on top of the first yields only +14 edits (1.1%) — it fades out, not taking it.
- `bold` (dropping the anchor) raises scope 3.0%→4.5%, but precision falls 87%→77% at the same net gain → **not taking it**.

### 3c. Repair with neighbouring-sentence context (`--context N`) — BEST RESULT
Hypothesis: the repair pass's residual losses come from guessing without context (character gender when English is neutral, pronoun referent). Implementation: a `prev` field in the batch — N preceding sentences with their FINISHED translations, read-only; in this mode the prompt changes the rule from "never fix gender" to "where prev settles it, fix; where it does not, leave it alone".

| mode | changed | decisive | precision | p | net gain |
|---|---|---|---|---|---|
| fluent + glossary | 38 (3.0%) | 31 | 87.1% | 3.4e-05 | 28.2 |
| fluent + bold | 58 (4.5%) | 43 | 76.7% | 0.0006 | 31.0 |
| fluent + **ctx1** | 34 (2.7%) | 25 | **80.0%** | 0.004 | 20.4 |
| **fluent + ctx2** | **59 (4.6%)** | **45** | **84.4%** | **3.1e-06** | **40.6** |

- **Non-monotonicity is the main finding: ctx1 is WORSE than no context at all** (80.0% vs 87.1%). One sentence back gives the model enough confidence to meddle with character gender, but not enough information to guess right. Direct illustration: «врач встала»→«встал» (the character is female per the book) was broken by v1, fluent AND ctx1, while **ctx2 left it alone**.
- ctx2 dominates `--bold`: same scope (59 vs 58) at 84.4% precision vs 76.7%. And it gives the best net gain of all arms.
- Edit overlap: ctx2 ∩ fluent = 25, ctx2-only = 34 — i.e. context opens a new class of fixes rather than replaying the old ones.
- ⚠️ The throughput measurement on the dev set is **unusable**: ctx1 5.6 sent/s, ctx2 7.6 sent/s vs 45.6 without context — but the cause is not input length (input tokens +33%/+62%), it is transient 429s from the gonka network (340 and 124 refusals vs 0 for the context-free arm at a different time of day). Only a full-book run measures the real cost of context.

#### Full-book check of ctx2 — and a reversal of the verdict
Full book, repair with `--context 2`: **15m13s (15.4 sent/s)**, 672 changed (4.8%), p50 11.8s, 245×429, 125 retries, 4.08M input tokens (+51% over context-free). Against **5m09s (45.6 sent/s)** without context — **three times more expensive in time**.

| mode | changed (book) | precision | net sentences* | repair phase |
|---|---|---|---|---|
| fluent (no context) | 544 (3.9%) | 87.1% | ~404 | 5m09s |
| fluent + ctx2 | 672 (4.8%) | 84.4% | ~463 | 15m13s |

*estimate `changed × (wins − losses) / decisive`.

**Bottom line: ctx2 buys +59 sentences out of 14,082 (+0.42% of the book) for +10 minutes** and eats the whole budget headroom (recipe ~28m40s against a 30 min limit). The dev set overestimated the lever: there ctx2 changed 59 vs 38 for fluent (+55% scope), but on the full book it is 672 vs 544 (only +24%), and at lower precision the advantage almost dissolved.

Full ctx2 recipe: translate 5m20s + repair 15m13s + embalign 8m25s + LLM-align 2m26s + assembly = **31m41s — over budget**. Against 18m36s for `--fluent`. The spread of the alignment phase between arms (8m07s vs 11m08s on the same work) is network 429s and machine load; one more argument for keeping headroom.
Alignment quality for ctx2 is meanwhile the best of all: expression word coverage 94.0% (base 93.5%, fluent 93.0%), content-word coverage balanced (dropped in 30, rose in 29 sentences), validation 0/0/0, lexcheck 327 vs 323 for the base.

**Recommendation: default is `--fluent` WITHOUT context; keep `--context 2` as an option** for runs where time does not matter. Never use `--context 1` (worse than no context).

### Key conclusion from the judge notes (re-orients Stage 3)
Kimi's defects are mostly **grammatical**, not idiomatic: agreement («мы вошёл», «была высокомерие», «три высокие силуэты», «широкая щелевая рот»), non-existent words («ударала», «дожемиллениальные»), broken collocations, case government («мне заверили»). Idioms are the smaller share, and v6 did not cure them. → Priority: **a repair (proofread) pass** on gonka's free tokens, not idiom prompting. Tool cmd/repair (in development): reads raw translations from an arm's cache, runs batches of {id,src,tr} through a proofreader prompt, writes the corrected text into another cache dir under TrKey → an ordinary convert finishes alignment without re-translating.

## Stage 4 — alignment (goal "a")
- LLM-align-all and dual-align were dropped back in Stage 0 (worse and slower than the hybrid).
- Residual gap: **22% of idiomatic expressions are not aligned as a compact unit** (unit-rate 78% over 131 probes), even though word-level coverage is already 99%/97%.
- Trick for re-aligning without re-translating: a binary with a bumped `PromptVersion` ("v8g") + a copy of the cache dir per arm (raw translations under TrKey stay valid, final records miss). Binary: bench-quality/convert-realign.
- Metric for goal "a": `bench-quality/probe_align.py --cover` — coverage of words inside expressions (fairer than unit/partial in cmd/score: that one marks `partial` even for legitimate discontinuous translations such as «Для меня это было новостью»).

| arm (same kimi-a translations unless noted) | expression word coverage | fully | lexcheck |
|---|---|---|---|
| kimi-a hybrid (default) | **93.5%** | 82.4% | 22-24 |
| gemini-ref (own translations) | 95.0% | 86.3% | 33 |
| kimi-v6 (own translations) | 92.2% | 80.2% | 26 |
| glue 0.3 / 0.2 (re-alignment) | 92.2% | 80.2% | 22 |
| emb-only (no LLM tail) | 91.5% | 79.4% | 32 |
| LLM-align-all b8 | 93.2% | 85.5% | 31 |

- ⚠️ **THE TRAP THAT INVALIDATED SOME OF THE FIRST MEASUREMENTS:** the glossary cache key (`glossary.go`) includes `cache.PromptVersion`. A binary with a bumped version misses on the glossary, generates a new one, gets a **different namespace hash** and silently RE-TRANSLATES the book (the glue/emb arms had 6226 cache files vs 3667 for kimi-a, and 47.7% of sentences with different text). The right way is to pre-seed the file `glossary-<new hash>.json` with a copy of the old one (done in the unit arms: 0 text discrepancies).
- **Glue threshold sweep: ZERO effect** — valid: both arms (0.3 and 0.2) had identical text and produced a bit-identical result.
- **Hybrid vs emb-only is a valid paired check** (`glue-g-base` vs `glue-emb`, 0 text discrepancies): expression word coverage 92.2% vs 91.5%, lexcheck **22 vs 32 flags**. The LLM tail on 3% of gated sentences is justified, the hybrid remains the default.
- The previously claimed "noise floor of ≈1.3 pp from LLM-align variability" is **wrong**: the discrepancy was explained by re-translation caused by the glossary trap. The kimi-a/kimi-b retest (identical text) produced identical alignment, i.e. given identical input the pipeline is deterministic.

### 4c. Unit-glue: gluing alignment to expression boundaries (implemented, default-off)
Mechanism: if at least one word of a known expression is aligned to the target, all words of the expression are attached to the same target set (deterministic, no threshold; guard: skip when the set is too large). Env `EMBALIGN_UNIT_GLUE=1`, boundary source is `--units-file` (probe JSON). Files: `tools/embalign.py` (`glue_units`), `internal/embalign/{embalign.go,units_exp.go}`, `internal/translate/pipeline.go` (`Pipeline.Units`), `internal/config/config.go`, `cmd/convert/main.go`. Backward compatibility proven bit-for-bit (three configurations identical to HEAD).

| metric (same translations, 0 text discrepancies) | glue OFF | glue ON | Δ |
|---|---|---|---|
| expression word coverage | 93.5% | **100%** (circular: boundaries from the same file) | +6.5 pp |
| unit-rate (`cmd/score --probe`) | 77.9% | **93.9%** | +16 pp |
| tap coverage of content words (whole book) | 0.979 | 0.982 | +0.3 pp |
| sentences with tap coverage <0.8 | 30 | 25 | −5 |
| lexcheck flags | 25 | 25 | 0 |
| changed sentences | — | 88, **all of them probes, 0 collateral** | — |

- Manual check of 23 new glues: **18 correct**, 2 partial, 1 wrong, 2 incomplete (~78-83% precision). All errors are amplifications of an already bad "anchor" (e.g. *has to do with* → «вы»); the mechanism does not invent new errors.
- ⚠️ lexcheck is structurally **blind** to this lever: `CheckSentence` counts a chunk as supported if AT LEAST ONE of its source words fits, and unit-glue only widens `c.W`. You cannot cite "lexcheck did not go up" as proof of safety.
- What is needed before rollout: (1) a real boundary source (an LLM pre-pass tagging expressions on the source — one per book, amortized across all target languages, or an idiom dictionary); (2) a guard that actually fires — requiring the target set to be contiguous (all 3 bad cases have a non-contiguous/non-dictionary anchor); (3) a judge check of the taps themselves instead of coverage.
- Diagnosis of the residual gap: what is uncovered is not only function words but also **content words inside idioms**, where Russian has no word-for-word correspondence: «day» in *Don't give up the day job* → «основную работу», «fair» in *my fair share of* → «немало», «point» in *on the point of*. LaBSE will not glue them (cosine is small), lowering the threshold does not help → what is needed is **gluing to expression boundaries** (unit-glue, under test).

## Stage 5 — full book (14,688 sentences, 14,082 unique)
Kimi base on gonka, defaults: translate 5m20s (44.0 sent/s) + embalign 7m24s (31.7 sent/s, 480 gated = 3.4%) + LLM-align 54s (9.0 sent/s) = pipeline 13m38s, **14m07s total**. Validation: 0 empty, 0 offset/structure errors. Coverage 99% / 97%. Lexcheck 323/13846 (2.3%). Full-book glossary hash: b5bd47201eeb (computed from content: sha256 of «src=tgt\n» lines, first 12 hex).
**Recipe with repair:** translate 5m20s + repair 5m09s (45.6 sent/s, 544 changed = 3.9%) + embalign 7m12s (489 gated) + LLM-align 42s + assembly ≈ **18m45s** — the 30 min budget holds with ~11 min of headroom.
Recipe quality: validation 0/0/0, coverage 99%/97% (same as base), lexcheck 323→332, all alignment averages identical to three decimals (Δ=−0.000), expression word coverage 93.5%→93.0%. At 87% precision that is ≈+470 fixed vs ≈−70 spoiled, net ~400 sentences (2.7% of the book) for 5 minutes.
`cmd/score` over the whole book (12,002 aggregated sentences) — repair is neutral to alignment: target 0.990/0.993 → 0.989/0.993, source tap 0.971/0.979 → 0.970/0.978, sentences with tap coverage <0.8: 316 (2.6%) → 321 (2.7%), empty translations and unaligned sentences 0/0 in both.
Artifacts: bench-quality/full-kimi.tbook, full-repair.tbook, full-repair-diff.json (544 edits), full-*-stats.jsonl.

## Hygiene notes
- **The scratchpad in /tmp is wiped between sessions** — probe-chunk/probe-raw/probe-all and the mined chunks were lost (except bench-quality/probe-dev.json saved in the repo, 194 probes). Keep all research assets in bench-quality/. Restored: bench-quality/ac-sentences.json (9599 sentences with chapter numbers).
- Claude session limits hit judge batches hard: 10 agents × 60 items ≈ 800k tokens. For the next comparisons — fewer pairs/agents, or one combo arm against two reference arms.
- One cache dir per arm: .cache-exp-*. Delete the judge caches judge-*.json when the judge prompt changes, or use a fresh dir.
- gonka stats have no cost field; price ~$0.000117/1M — not counting it.
- 429s on gonka are network-side (not from our concurrency): the judge with 320 small requests catches them by the hundreds, translate with 160 catches zero.
