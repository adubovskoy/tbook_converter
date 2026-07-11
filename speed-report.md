# Speed report: full-book conversion under 30 minutes

Research goal: convert *Der Name der Rose* (de→ru, 11,034 sentences, ~210k words)
in **< 30 min** and *Isaac Asimov's Worlds SF* (en→ru, 9,396 sentences, ~124k words)
in **< 1.5 h**, without degrading translation quality or word-highlighting UX.
Experiment budget: ~$20 OpenRouter; spent **$15.10** total, of which ~$7 went to
the two full-book runs on the rejected escalation config — a clean book on the
final recipe is **~$1.7**.

**Result: Rose in 13m35s (target 30 min), Asimov in 19m25s — and ~8m without the
rejected escalation config (target 90 min)** (§6). Both targets pass with >2×
margin, at unchanged quality gates: validation 0 errors, 90–91% aligned-word
coverage, ~$1.7 per book with the final recipe.

## 1. Where the time went (before)

The pre-research pipeline (defaults: `gemini-2.5-flash`, batch 16, concurrency 6,
`--align-mode llm`) spent wall-clock in three serial blocks for an 11k-sentence book:

| block | estimate before | after | how |
|---|---|---|---|
| translate pass | ~9 min (C=6) | ~3 min | concurrency 6→24/32 (§3) |
| align pass | ~40+ min (LLM align all) / ~57 min (embalign at 3.2 pairs/s) | ~10 min | hybrid LaBSE + embalign fixes (§4) |
| judge over all sentences (if on) | ≈ translate cost again | ~1 min | `--judge-scope flagged` (§5) |

## 2. Instrumentation added (`--stats`)

`--stats file.jsonl` logs one record per LLM request attempt: latency, HTTP status,
attempt number, serving provider, `finish_reason`, prompt/completion tokens and
**exact cost** (OpenRouter `usage.include` accounting). Phase wall-time summaries
print for translate / embalign / LLM-align / judge / escalation, plus total.
`finish_reason=length` (provider output cap truncation) is now a typed, visible
error instead of a silent retry-burner.

Also: batch items are now sent with short per-batch ids `"1".."n"` instead of
64-hex cache keys (the judge already did this). The hex ids were ~6% of translate
input tokens and got mangled by cheap models; echoed ids are half the fix, the
other half is that output tokens dominate latency.

## 3. Concurrency and batch size (Rose 6-chapter subset, 1,159 sentences)

Translate phase, `gemini-2.5-flash`, B=16, fresh cache per cell:

| C | sent/s | p50/req | p95/req | 429s | lexcheck flagged |
|---|---|---|---|---|---|
| 6 | 20.6 | 4.3 s | 7.5 s | 0 | 1.1% |
| 12 | 37.6 | 4.3 s | 7.1 s | 0 | 1.0% |
| 24 | 53.3 | 4.4 s | 9.5 s | 0 | 1.0% |
| 48 | 61.7 | 5.0 s | 7.5 s | 0 | 1.4% |

Zero provider pushback up to C=48; scaling is near-linear to C=24, then wave
quantization dominates on a small subset. **Chosen: C=32 for full runs** (24–48 all
safe; quality independent of C as expected).

Batch size at C=24: B=32 → 45.8 sent/s, B=64 → 34.5 sent/s **worse than B=16**
(53.3): generation is output-token-bound, so bigger batches only raise per-request
latency and drop risk (B=64 produced the first dropped sentences). **B=16 stays.**

## 4. Alignment: hybrid LaBSE validated for de→ru; embalign 5.5× faster

### 4.1 The embalign throughput bug hunt

Original `tools/embalign.py` on this 22-thread CPU: **3.2 sentence-pairs/s**
(≈ 57 min for Rose — worse than the LLM align pass it replaces). Three fixes,
all output-identical:

1. **Thread oversubscription**: torch defaulted to 16 threads, `os.cpu_count()`
   gives 22 — both catastrophically slow (batch-1 forward: 125 ms at 22 threads vs
   20 ms at 8). Physical cores (11) is the knee. Now `--threads`, default
   `cpu_count/2`, env `EMBALIGN_THREADS`.
2. **Layer truncation**: only `hidden_states[8]` is used → drop encoder layers
   8–11 (`del model.encoder.layer[8:]`). Verified bit-identical output, ~1.4×.
3. **Batched + length-sorted encoding**: new batch protocol
   (`{"batch": [...]}` → `{"results": [...]}`); the Go side sends 256 pairs per
   request, python encodes each side in length-sorted padded sub-batches of 32.
   Sorting alone is ~1.9× on real book sentences (mixed lengths pad terribly).
   Verified identical pairs vs the one-by-one path (100/100 sentences).

Net: **3.2 → ~18 pairs/s** on real literary sentences ⇒ Rose embalign ≈ 10 min.

### 4.2 de→ru arm comparison (identical translations, lexeval + coverage)

The prior LaBSE-beats-LLM result was en→ru only. Rerun for de→ru on 1,007
identical gemini-2.5-flash translations:

| arm | lexeval support | drift recall (full) | FP | lexcheck flags | tgt coverage | speed | cost |
|---|---|---|---|---|---|---|---|
| **LaBSE f32 (prod)** | **74%** | 90.9% | 1.1% | 1.4% | 90% | 17.7 sent/s | $0 |
| LaBSE int8 | 74% | 89.2% | 1.2% | 1.8% | **79%** | 27.7 sent/s | $0 |
| LLM align (flash) | 73% | 90.9% | 1.1% | 1.5% | 94% | 7.0 sent/s | ~$0.35/1k |

- **Hybrid LaBSE holds for de→ru**: same dictionary support as the LLM pass at
  2.5× speed and zero cost. Gate G4 passed.
- **INT8 rejected**: +56% speed but highlight coverage drops 90→79% — a visible
  UX regression. Left in the code env-gated (`EMBALIGN_INT8=1`) for future
  re-evaluation, off by default.
- Hybrid gate rate (lexcheck+Q<0.7): 4–5% of sentences go to the LLM align pass.

## 5. Model sweep (translate pass, same subset, paired design)

| model | sent/s | p50/req | drops/retries | lexcheck | emb-gate | LaBSE cos (mean) | judge: mistrans/drift* | $/1k sents |
|---|---|---|---|---|---|---|---|---|
| **gemini-2.5-flash** (default) | 53.3 | 4.4 s | 0 | **1.0%** | **4.1%** | 0.898 | **36 / 13** | $0.19 |
| gemini-2.5-flash-lite | 63.6 | 3.5 s | 3 parse | 1.8% | 5.6% | 0.917 | 48 / 26 | **$0.04** |
| gemini-3.1-flash-lite | 43.9 | 3.7 s | 4 parse | 2.1% | 6.8% | 0.910 | 58 / 23 | $0.14 |
| deepseek-v4-flash | DNF | 37 s | hung, timeouts | — | — | — | — | — |
| qwen3.5-flash-02-23 | 2.8 | 53 s | 22 retries | — | — | — | — | — |
| gpt-oss-120b | 3.9 | 30 s | 60 dropped | — | — | — | — | — |

\* same judge (gemini-2.5-flash) over all arms — comparative only, see §5.2.

- **deepseek-v4-flash / qwen3.5-flash / gpt-oss-120b all disqualify on latency**:
  they emit 3–12× the output tokens (hidden reasoning) and route across a dozen
  variable-quality providers (p50 30–53 s vs 4 s). qwen3.5-flash actually costs
  *more* than gemini-2.5-flash per book despite 10× cheaper list price — reasoning
  tokens are billed output. This extends the old "gemini ≫ deepseek" memory to the
  entire non-Gemini cheap tier, for this JSON-batch workload.
- **gemini-2.5-flash stays the default** (best on every quality proxy).
- **gemini-2.5-flash-lite is a legitimate budget option**: 5× cheaper (~$0.40/book),
  slightly faster, quality measurably but modestly worse (+0.8pp lexcheck,
  +1.2pp judge-mistranslation — not significant at n=1007, but every proxy points
  the same direction). Its higher LaBSE cosine reflects more literal phrasing,
  not higher quality.

### 5.2 Judge finding: unusable as an escalation gate over embalign output (de→ru)

gemini-2.5-flash as judge flags **53%** of its own arm — nearly all
`wrong-mapping` — on alignments that manual inspection shows are correct
(pedantry about per-word granularity, e.g. flawless "Но←Aber / дело←Sache /
произошло←zugetragen" chains get flagged). Coupling judge→escalation would redo
half the book and *degrade* fine sentences to no-highlights fallback on re-verify.

Consequence, implemented: `--judge-scope flagged` judges only
lexcheck-flagged ∪ low-coverage (Q<0.7) sentences plus a fixed-seed 5%
calibration sample, and the full-run recipe keeps judge as a **report-only**
second invocation (no `--escalate-model` in the same run). Judge batch is now
capped at 16. On the full Rose run the scoped judge covered 547 of 9,391
sentences in **5 seconds**; its calibration sample re-confirmed the pedantry
baseline (57% of filter-clean sentences flagged).

### 5.3 Escalation finding: lexcheck low-support flags on embalign output are not defects

The Rose take-1 run used the documented escalation loop
(`--escalate-model google/gemini-2.5-pro`, lexcheck-gated). Autopsy:

- Only 70 of 9,176 sentences (0.76%) were flagged — 67 `low-support`, 3 `shift-pattern`.
- Manual inspection of flagged sentences shows **correct, often flawless
  alignments**: they are the novel's macaronic passages — Latin ("vade retro"),
  Occitan verse, Salvatore's mixed-language speech — which a de-ru dictionary
  cannot score. With embalign (structurally immune to positional cascades),
  `low-support` measures *dictionary coverage*, not drift.
- Escalating them is worse than useless: gemini-2.5-pro spent **32m11s** and
  **$3.02** on 72 sentences (reasoning latency: p95 90 s/request), and its freer
  re-translations *still* failed the dictionary gate — 49 sentences were then
  "degraded" to raw text with no highlights. Escalation destroyed 49 perfectly
  good alignments and tripled the run's wall time.

The escalation loop was designed for the LLM-align era, where lexcheck flags
meant positional cascades. Under `--align-mode hybrid` the gate has already
LLM-re-aligned every lexcheck/low-Q sentence *inside* the pipeline; the
post-run flags that remain are almost entirely coverage artifacts.
**Recommendation: run without `--escalate-model`; treat post-run lexcheck +
scoped judge as reports** (`.lexflagged.json` feeds `--invalidate` for a
targeted re-run if a human confirms real defects).

## 6. Full-book validation runs

Config: `gemini-2.5-flash`, C=32, B=16, `--align-mode hybrid`, `--glossary`,
plus a judge-report pass (`--judge --judge-scope flagged`).

| book | sentences | wall time | target | cost | coverage | lexcheck | validation |
|---|---|---|---|---|---|---|---|
| **Rose take-2 (final recipe)** | 11,034 | **13m35s** (1m22s translate + 10m40s embalign + 41s LLM-align + 17s judge report) | <30 min ✓ | **$1.69** | 90% / 79% | 0.81% | OK, 0 errors |
| Asimov (with pro escalation — rejected config) | 9,396 | 19m25s (7m48s pipeline + 11m19s escalation) | <90 min ✓ | $4.55 | 91% / 80% | 1.2% | OK, 0 errors |
| Rose take-1 (with pro escalation — rejected config) | 11,034 | 43m40s (11m04s pipeline + 32m escalation) | <30 min ✗ | $4.90 | 91% / 79% | 0.76% | OK, 0 errors |

Translate phase at C=32, zero 429s: Rose **9,391 sentences in 1m22s
(115 sent/s)**; Asimov 9,237 in 54s (172 sent/s). Asimov on the final recipe
(minus the 11m19s escalation) is ~8 min. The "N sentences carry no alignment"
in validation is dominated by punctuation-only pseudo-sentences (`«` → `»`,
13% of Rose's sentence objects — no tappable words, no UX impact).

Spot-checks (10 random sentences per book): faithful natural translations,
correct alignments including multi-word units (`hast geprüft←проверил`) and
untouched Latin titles; glossary keeps names consistent (William→Вильгельм).
Residual defects are isolated single-pair looseness (e.g. `объясняло←for`),
never cascades.

## 7. Recommended recipe — now the built-in default

As of this research the winning configuration **is the flag-free default**:
`--align-mode hybrid`, `--concurrency 32`, `--batch-size 16`, glossary on
(`--no-glossary` opts out), lexcheck on, judge scope `flagged`, no escalation.
A missing embalign venv degrades to `--align-mode llm` with a notice instead
of failing.

```bash
./convert book.epub -s de -t ru -o book.tbook          # that's the whole recipe

# optional quality report (report-only; feeds --invalidate for targeted re-runs):
./convert book.epub -s de -t ru --judge --cache-dir <same> -o book.tbook
```

Do **not** add `--escalate-model` to a hybrid-align run (see §5.3) — and never a
reasoning-tier model (pro) as escalator in a timed run.

Budget variant: `--model google/gemini-2.5-flash-lite` (~$0.40/book, ~1pp more
flagged sentences).

Note: glossary-on writes to a new cache namespace (`model+g:<hash>`) — resuming
a cache made before glossary became the default needs `--no-glossary`.

## 8. Not pursued / rejected

- **Batch size 32/64**: no throughput gain (output-bound), more drops. Rejected.
- **INT8 embalign**: −11pp highlight coverage. Rejected (env-gated off).
- **Local LLM translation**: no GPU on this machine; CPU LLMs are orders of
  magnitude too slow for a 30-min book. Local NMT (MADLAD/Opus-MT) untested —
  literary quality bar makes it unlikely; revisit only if API cost ever matters.
- **Phase overlap (embalign streaming behind translate)**: would hide ~3 min;
  unnecessary after the embalign fixes. ~100 LOC producer/consumer refactor in
  `pipeline.go` if ever needed.
- **`--provider-sort throughput`**: previously measured slower for bulk (pins a
  single provider, serializes); default routing already lands on Google at 4 s p50.
