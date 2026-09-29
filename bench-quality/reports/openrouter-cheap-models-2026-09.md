# Cheap OpenRouter models for translation — screening and blind bench, 2026-09-19

Plan item **C4**. Question: among OpenRouter models released in the last twelve
months and priced under $0.50 / M prompt and $2.00 / M completion, is there one
that could replace `google/gemini-3.1-flash-lite` as the production anchor, or
serve as a metered fallback when the gonka gateway is unavailable?

Method: produce the same excerpt with every candidate, then judge each candidate
against the anchor blind, both presentation orders, decisive only when the two
orders agree (the harness of `EVIDENCE §5`). The finalists are then measured on
German and French, which no model in this project has ever been measured on.

## 0. Run record

| item | value |
|---|---|
| excerpt | *The Adventures of Sherlock Holmes* (Gutenberg #1661), `--limit-chapters 4` |
| excerpt size | 4 chapters, **1667 sentences**, 24 734 words, 131 864 chars |
| binary | `bench-quality/.artifacts/quality-plan/bin/convert-c4`, built **2026-09-19 20:59:16 +02:00** |
| tree at build time | HEAD `a7f61cf` + `20 files changed, 925 insertions(+), 174 deletions(-)` (the in-flight wrong-language gate work) |
| flags, every arm | `--provider openrouter --model <id> --no-repair --limit-chapters 4 --cache-dir …/or/cache-<slug>` ; defaults otherwise (batch 16, concurrency 32, hybrid align, glossary on, lexcheck on) |
| arms, stats, logs | `bench-quality/.artifacts/quality-plan/or/` |
| ledger | `bench-quality/.artifacts/quality-plan/or/LEDGER.md` |
| Stage 1 judged subset | the 300 source sentences of `bench-quality/gold/en-ru.jsonl` (the A3 stride subset), taken verbatim so the two experiments coincide |
| budget | hard $15.00 OpenRouter, stop at $13.00 |

One binary was frozen at the start because other agents are editing
`internal/translate/*.go` concurrently; no arm in this report was produced by any
other build.

## 1. Candidate selection

Source: the public `https://openrouter.ai/api/v1/models` snapshot of 2026-09-19
(447 models). Filter, in order:

1. `created` ≥ 2025-09-19 (last twelve months);
2. output modality exactly `text`, input modalities include `text`;
3. context ≥ 32k;
4. `pricing.prompt` ≤ $0.50 / M **and** `pricing.completion` ≤ $2.00 / M;
5. drop `:free` variants (rate caps), `:batch` variants (not a live path for the
   converter), `~…-latest` aliases (a moving target cannot be a bench arm) and
   the `openrouter/*` meta-routers (negative price = auto-routing);
6. drop code-only, safety-classifier, extraction and audio-first models;
7. drop models already measured on this pipeline: `google/gemini-3.7-flash`,
   `z-ai/glm-5.2`, `google/gemma-4-26b-a4b-it`, `deepseek/deepseek-v4-flash-0731`,
   `minimax/minimax-m2.7` (`EVIDENCE §2.1`);
8. then pick for **breadth of family**, at most one model per family, preferring
   the newest model of that family that survives 1–7.

138 models survive 1–4. Families the price cap removes entirely, worth recording
because the brief asked for them: **Anthropic** (no Haiku-class model is inside
$0.50 / $2.00), **xAI** (no Grok mini inside the cap), **Amazon Nova** (absent
from the snapshot), **Moonshot / Kimi** (absent from the snapshot), **Cohere**
(only `north-mini-code:free`, a code model behind a rate cap). Google's own
`gemini-3.5-flash-lite` and newer flash tiers are above the completion cap except
as `:batch`.

| # | model id | family | created | prompt $/M | completion $/M | context | reasoning param | structured outputs | why picked |
|---|---|---|---|---|---|---|---|---|---|
| A | `google/gemini-3.1-flash-lite` | Google | 2026-05-07 | 0.250 | 1.500 | 1024k | yes | yes | anchor (production default) |
| 1 | `google/gemma-4-31b-it` | Google | 2026-04-02 | 0.090 | 0.340 | 256k | yes | yes | dense 31B; the 26B MoE sibling was measured, this one was not |
| 2 | `openai/gpt-5.6-luna` | OpenAI | 2026-07-09 | 0.200 | 1.200 | 1025k | yes | yes | cheapest OpenAI text model inside the cap |
| 3 | `meta/muse-spark-1.3-contributor` | Meta | 2026-09-02 | 0.100 | 0.200 | 1024k | yes | yes | newest Meta inside the cap |
| 4 | `mistralai/mistral-small-2603` | Mistral | 2026-03-16 | 0.150 | 0.600 | 256k | yes | yes | Mistral's small instruct line |
| 5 | `qwen/qwen3.8-flash` | Qwen | 2026-08-26 | 0.150 | 0.470 | 976k | yes | yes | newest Qwen flash |
| 6 | `deepseek/deepseek-v4.1-flash` | DeepSeek | 2026-09-10 | 0.150 | 0.600 | 1024k | yes | yes | successor of the v4-flash already measured |
| 7 | `z-ai/glm-5.3-flash` | Z.ai | 2026-08-26 | 0.090 | 0.300 | 1280k | yes | yes | flash sibling of glm-5.2 (7.59/10) |
| 8 | `minimax/minimax-m3` | MiniMax | 2026-05-31 | 0.300 | 1.200 | 1024k | yes | yes | M2.7 was rejected on gonka; the metered M3 is the retest |
| 9 | `bytedance-seed/seed-2.0-mini` | ByteDance | 2026-02-26 | 0.100 | 0.400 | 256k | yes | yes | Seed family, never measured here |
| 10 | `nvidia/nemotron-3.5-lightning` | NVIDIA | 2026-08-11 | 0.070 | 0.200 | 256k | yes | yes | Nemotron family, never measured here |
| 11 | `tencent/hy3` | Tencent | 2026-07-06 | 0.083 | 0.330 | 256k | yes | yes | Hunyuan family, never measured here |

Every model in the table advertises a `reasoning` parameter; whether reasoning can
actually be switched off is only observable at run time and is reported in §2.

## 2. Stage 1 — screening on Russian

Every candidate produced the full 4-chapter excerpt in **ru only**; judging is
restricted to the 300 sentences of the A3 stride subset (`gold/en-ru.jsonl`), so
this stage shares its sentence set with the GOLD-lite work.

### 2.1 Judging protocol (both stages)

`prepare_pairs_lang.py` (a `--lang`/`--only-src` wrapper around the repo's
`prepare_pairs.py`, added for this run because the original reads `tr['ru']`
only) samples pairs where the two arms' texts actually differ, emits both
presentation orders, and hides which arm is which. `judge_pairs.py` runs one
fresh `claude -p --model sonnet` process per batch of 10 items, `--jobs 5`,
30 s backoff. `analyze_pairs.py` de-swaps and counts a pair decisive only when
the two orders agree; anything else is a tie. Sign test, two-sided.

### 2.2 Arms that never reached a `.tbook`

| model | what happened | verdict |
|---|---|---|
| `meta/muse-spark-1.3-contributor` | first glossary request returned **403 `This model requires you to complete the following before use: 18+ age confirmation`** — an account-level setting, not a converter problem. Not changed; the model was replaced in the Meta slot by `meta/muse-glimmer-30b`. | dropped, not measured |
| `meta/muse-glimmer-30b` | **`400 Reasoning is mandatory for this endpoint and cannot be disabled`** on the first request; the client latches an effort and continues, but then **246 of 533 requests came back as unparseable JSON** (`invalid character ':' after object key:value pair` ×207). **1590 of 1642 sentences shipped empty (96.8 %)** and it was the single most expensive arm at **$0.83**. | dropped: reasoning cannot be disabled + JSON mode unusable |
| `google/gemma-4-31b-it` | 15 parse failures and one `finish_reason=length` truncation; retries at smaller batches did not recover. **594 of 1642 sentences empty (36.2 %)**, 12m27s wall. | dropped: ≥ 5 % sentences empty |
| `mistralai/mistral-small-2603` | **429 `Provider returned error` on essentially every request** at the default `--concurrency 32`, again at 6, and again at 2 with `--batch-size 8`; the model has a single upstream endpoint (Mistral) and no alternative provider to route to. | dropped: provider-side rate limit, arm never completed |

### 2.3 Operational and structural metrics, all arms (ru)

Blind column: candidate wins : anchor wins among decisive pairs, both
presentation orders, 100 pairs per comparison drawn from the A3 300-sentence
subset; `p` is the two-sided sign test. `cov tgt/src` is the converter's own
coverage line; `clean tap %` and `cw-silent %` come from `cmd/tapledger`.
`mistralsm` was **not judged** (dropped structurally); its row shows the empty
comparison that records the decision.

| arm | wall | translate sent/s | cost $ | req / err | empty | lexcheck | langcheck | align gate | cov tgt/src | clean tap % | cw-silent % | blind vs anchor (decisive) | p |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `dsv41f` | 7m54s | 20.3 | 0.0827 | 120 / 2 | 0 (0.0 %) | 32 | clean | 2.8 % | 98/96 | 52.13% | 2.82% | 22:18 = 55.0 % (40/100) | 0.6358 |
| `gemma431` | 12m27s | 5.1 | 0.0596 | 229 / 15 | 594 (36.2 %) | 9 | clean | 1.8 % | 98/96 | 53.36% | 2.85% | — | — |
| `gflashlite` | 7m32s | 158.7 | 0.1419 | 119 / 0 | 0 (0.0 %) | 34 | clean | 3.3 % | 98/96 | 51.84% | 2.96% | — | — |
| `glm53f` | 9m21s | 11.7 | 0.0552 | 135 / 16 | 0 (0.0 %) | 38 | clean | 3.3 % | 98/95 | 50.76% | 3.31% | 12:22 = 35.3 % (34/100) | 0.1214 |
| `gptluna` | 7m5s | 90.4 | 0.0967 | 128 / 1 | 0 (0.0 %) | 61 | 1 | 4.9 % | 98/95 | 48.92% | 3.34% | 18:21 = 46.2 % (39/100) | 0.7493 |
| `hy3` | 2m52s | 26.3 | 0.0484 | 127 / 3 | 19 (1.2 %) | 14 | clean | 1.7 % | 96/97 | 55.22% | 2.23% | 11:40 = 21.6 % (51/100) | 5.704e-05 |
| `minimaxm3` | 7m54s | 6.5 | 0.1404 | 141 / 11 | 0 (0.0 %) | 56 | clean | 5.2 % | 97/95 | 48.32% | 4.15% | 10:26 = 27.8 % (36/100) | 0.01133 |
| `mistralsm` | 33m40s | 0.8 | 0.0391 | 1175 / 1018 | 152 (9.3 %) | 40 | clean | 5.1 % | 97/94 | 48.08% | 4.52% | 0:0 = 0.0 % (0/0) | - |
| `museglimmer` | 9m36s | 3.3 | 0.8323 | 533 / 279 | 1590 (96.8 %) | 1 | clean | 2.1 % | 100/97 | 54.53% | 2.42% | — | — |
| `musespark` | — | — | 0.0000 | 1 / 1 | — | — | — | — | — | — | — | — | — |
| `nemotron35l` | 2m52s | — | 0.0313 | 200 / 58 | 0 (0.0 %) | 37 | 25 | 44.8 % | 97/94 | 50.63% | 4.71% | 3:63 = 4.5 % (66/100) | 1.3e-15 |
| `qwen38f` | 7m0s | 35.3 | 0.0396 | 117 / 0 | 0 (0.0 %) | 33 | clean | 2.7 % | 98/95 | 50.68% | 3.36% | 12:24 = 33.3 % (36/100) | 0.06525 |
| `seed20mini` | 1m57s | 80.2 | 0.0422 | 120 / 0 | 0 (0.0 %) | 23 | clean | 3.1 % | 98/96 | 51.65% | 3.11% | 4:63 = 6.0 % (67/100) | 1.107e-14 |

Notes per arm that the table cannot hold:

- `nvidia/nemotron-3.5-lightning` — the only arm whose **embedding aligner gated
  44.8 %** of sentences to the LLM align pass (every other arm is 1.7–5.2 %), and
  the only surviving arm with **25 langcheck flags**. Its first run was cut short
  by a 15-minute wrapper timeout; the number in the table is the resumed run from
  cache, which is why its translate rate is blank. Its first-pass rate was
  4.0 sent/s.
- `tencent/hy3` — 19 sentences (1.2 %) shipped empty; under the 5 % bar, so it was
  judged and lost on quality, not on structure.
- `minimax/minimax-m3` — reasoning could not be disabled but did not break the
  run: 0 empty sentences, 5.2 % align gate, $0.14 (the second most expensive
  candidate). It is the first MiniMax arm in this project that completed at all
  (`EVIDENCE §2.2 N7` rejected M2.7 at every batch size on gonka).
- `openai/gpt-5.6-luna` — highest lexcheck flag count of the survivors (61) yet
  statistically level with the anchor, which is a reminder that lexcheck counts
  are a structural signal, not a quality one (`EVIDENCE §5`).

### 2.4 Screening outcome

Kill rule as briefed: drop an arm whose decisive win share is ≤ 35 % **with
p < 0.05**; cap Stage 2 at the top three by win share.

| rank | model | decisive win share vs anchor | p | outcome |
|---|---|---|---|---|
| 1 | `deepseek/deepseek-v4.1-flash` | **55.0 %** | 0.636 | **finalist** |
| 2 | `openai/gpt-5.6-luna` | **46.2 %** | 0.749 | **finalist** |
| 3 | `z-ai/glm-5.3-flash` | **35.3 %** | 0.121 | **finalist** |
| 4 | `qwen/qwen3.8-flash` | 33.3 % | 0.065 | survives the kill rule, misses the top-3 cap |
| 5 | `minimax/minimax-m3` | 27.8 % | 0.011 | dropped (≤ 35 % at p < 0.05) |
| 6 | `tencent/hy3` | 21.6 % | 5.7e-05 | dropped |
| 7 | `bytedance-seed/seed-2.0-mini` | 6.0 % | 1.1e-14 | dropped |
| 8 | `nvidia/nemotron-3.5-lightning` | 4.5 % | 1.3e-15 | dropped |

Tie rate across the eight screening comparisons: 32–57 % (`EVIDENCE §5` says
50–57 %; the two arms that lost heavily also tie least, which is what a real
quality gap looks like in this harness).


## 3. Stage 2 — the three finalists on Russian, German and French

Arms: `deepseek/deepseek-v4.1-flash`, `openai/gpt-5.6-luna`, `z-ai/glm-5.3-flash`
against the anchor `google/gemini-3.1-flash-lite`, all four produced with the
same frozen binary and the same flags, one cache directory per model per
language. Judging is **400 pairs in both presentation orders per (model,
language)** over the whole excerpt — not the 300-sentence screening subset — and
a further **200 pairs in both orders for ru against `google/gemini-3.7-flash`**,
the upper anchor produced by the A2/A3 run (`.artifacts/quality-plan/a2/`).

**These are the first German and French translation-quality numbers this project
has ever produced** (`EVIDENCE §6.2`: zero blind comparisons, zero defect counts
for either pair before today). There is no gold reference for De or Fr, so every
number below is relative to the anchor and judged by one LLM judge.

### 3.1 Structural metrics, every Stage 2 arm

Every one of the twelve arms shipped **0 empty sentences**; nothing in this stage
was dropped for structure.

| language | model | wall | translate sent/s | cost $ | req / err | empty | lexcheck | langcheck | align gate | cov tgt/src | clean tap % | cw-silent % |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| ru | `google/gemini-3.1-flash-lite (anchor)` | 7m32s | 158.7 | 0.1419 | 119 / 0 | 0 | 34 | clean | 3.3 % | 98/96 | 51.84% | 2.96% |
| ru | `deepseek/deepseek-v4.1-flash` | 7m54s | 20.3 | 0.0827 | 120 / 2 | 0 | 32 | clean | 2.8 % | 98/96 | 52.13% | 2.82% |
| ru | `openai/gpt-5.6-luna` | 7m5s | 90.4 | 0.0967 | 128 / 1 | 0 | 61 | 1 | 4.9 % | 98/95 | 48.92% | 3.34% |
| ru | `z-ai/glm-5.3-flash` | 9m21s | 11.7 | 0.0552 | 135 / 16 | 0 | 38 | clean | 3.3 % | 98/95 | 50.76% | 3.31% |
| de | `google/gemini-3.1-flash-lite (anchor)` | 2m31s | 46.0 | 0.1143 | 115 / 1 | 0 | 16 | 1 | 2.2 % | 97/97 | 73.88% | 2.74% |
| de | `deepseek/deepseek-v4.1-flash` | 7m12s | 19.8 | 0.0727 | 113 / 2 | 0 | 9 | 1 | 1.3 % | 98/97 | 74.75% | 2.48% |
| de | `openai/gpt-5.6-luna` | 1m53s | 95.7 | 0.0827 | 118 / 0 | 0 | 28 | 1 | 2.9 % | 96/96 | 70.49% | 3.58% |
| de | `z-ai/glm-5.3-flash` | 2m59s | 1.7 | 0.0409 | 117 / 5 | 0 | 10 | clean | 1.6 % | 97/97 | 73.40% | 2.67% |
| fr | `google/gemini-3.1-flash-lite (anchor)` | 4m52s | 54.6 | 0.1382 | 124 / 3 | 0 | 30 | clean | 3.5 % | 98/97 | 66.34% | 2.90% |
| fr | `deepseek/deepseek-v4.1-flash` | 4m12s | 18.7 | 0.0885 | 120 / 3 | 0 | 31 | clean | 2.8 % | 98/97 | 65.85% | 2.61% |
| fr | `openai/gpt-5.6-luna` | 1m46s | 104.7 | 0.1139 | 125 / 0 | 0 | 61 | clean | 4.9 % | 97/96 | 62.64% | 3.16% |
| fr | `z-ai/glm-5.3-flash` | 10m2s | 4.3 | 0.0508 | 162 / 41 | 0 | 39 | clean | 3.7 % | 98/97 | 65.20% | 2.95% |

Three things the table says that the judged numbers do not:

- **German taps are much cleaner than Russian taps for every model** — 70–75 %
  `clean` against 49–52 % for ru — and French sits between them at 63–66 %. That
  is a property of the language pair and the aligner, not of the model: the
  spread *between* models inside one language is 3–4 pp, the spread *between*
  languages is 20 pp.
- `openai/gpt-5.6-luna` has the **worst tap profile in all three languages**
  (lowest `clean`, highest `cw-silent`) and the **highest lexcheck count** in all
  three, while being statistically level with the anchor on judged quality. The
  two instruments disagree, which is exactly the separation `EVIDENCE §5`
  prescribes: structural numbers go in their own column.
- `z-ai/glm-5.3-flash` is the cheapest arm in every language ($0.041–0.055) but
  also the slowest and least reliable: 41 request errors on fr, 1.7 sent/s on de,
  and a `400 Reasoning is mandatory for this endpoint and cannot be disabled` on
  its first request of every run, which the client absorbs by latching an effort.

