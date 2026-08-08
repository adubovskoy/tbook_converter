# Gonka provider benchmark — 2026-07-22

Comparison of the Gonka decentralized network (via the [proxy.gonka.gg](https://proxy.gonka.gg) gateway, `--provider gonka`) against OpenRouter models: the default `google/gemini-3.1-flash-lite` and `z-ai/glm-5.2` (added on request), for `.tbook` conversion.

## Setup

- **Book**: Altered Carbon (Richard K. Morgan), first 5 chapters — 1279 sentences, en→ru.
  Full book: 46 chapters, 14 688 sentences, 158 421 words.
- **Pipeline**: converter defaults — batch 16, concurrency 32, glossary on, hybrid align
  (local LaBSE + LLM fallback), lexcheck on. Fresh cache per model, `--stats` per run.
- **Quality judging**: blind — 81 sampled common sentences (≥6 words), model labels
  shuffled per item, 9 independent Claude (Fable) judge agents × 9 items,
  scored 1–10 for semantic accuracy, completeness, natural literary Russian.
  Judged twice: 3-way (gemini/kimi/minimax) and, after adding GLM-5.2, 4-way on the
  same seed and sample. Rankings agree; the 4-way numbers are reported below.
- Converter revision: gonka provider patch on top of b8bc069 (translate prompt v5).

## Gateway / model specifics

- **Gonka** (proxy.gonka.gg): OpenAI-compatible; serves `moonshotai/Kimi-K2.6` (ctx 240k)
  and `MiniMaxAI/MiniMax-M2.7` (ctx 180k), both `max_tokens` 16384.
  Pricing (from `/api/pricing`): **~$0.000117 per 1M tokens** — a whole book costs
  fractions of a cent; cost is never the constraint, latency is. Usage responses
  carry token counts but no cost field.
- All non-gemini models here are reasoning models; the pipeline never reads reasoning,
  so the converter disables it wherever possible:
  - **Kimi-K2.6**: reasoning switched off by the Anthropic-style top-level
    `"thinking": {"type": "disabled"}` — the only working switch
    (`chat_template_kwargs.enable_thinking`, top-level `enable_thinking`,
    `reasoning_effort` are all ignored). Effect: 975→30 output tokens, 16s→1s on a
    one-sentence probe. Sent on every gonka request.
  - **MiniMax-M2.7**: cannot stop thinking (architectural); emits `<think>…</think>`
    inside `message.content`. The converter strips it at parse time (`stripThink`).
  - **GLM-5.2** (OpenRouter, $0.79/M in / $2.49/M out, ctx 1M): reasons by default
    (229 reasoning tokens and 8× the cost on a one-sentence probe); OpenRouter's
    unified `"reasoning": {"enabled": false}` disables it cleanly. The converter now
    sends it on every OpenRouter request (no-op for non-reasoning models like gemini).

## Results (5 chapters, 1279 sentences)

| Metric | gemini-3.1-flash-lite (OpenRouter) | Kimi-K2.6 (gonka) | MiniMax-M2.7 (gonka) | GLM-5.2 (OpenRouter) |
|---|---|---|---|---|
| Wall time | **82 s** | 99 s | 485 s | 321 s |
| Translate throughput | **83.4 sent/s** | 41.4 sent/s | 5.2 sent/s | 5.4 sent/s ¹ |
| Translate batch p50 / p90 | 3.6 s / 7.7 s | 9.6 s / 16.8 s | 4.8 s / 40.0 s | 9.8 s / 23.9 s |
| Request errors / 429s | 0 / 0 | 0 / 0 | 225 / 0 (224 = `finish_reason=length`) | 3 / 0 (parse, retried OK) |
| Sentences lost after all retries | 0 | 0 | **113 / 1279 (8.8%)** | 0 |
| Output tokens (all passes) | 42.5k | 42.1k | **785k (18×)** | 32.0k |
| Blind quality, mean of 81 (4-way) | **7.95** | 7.14 | 7.37 ² | 7.59 |
| Sole wins / items ≤6 (4-way) | **19 (23%)** / 4 | 2 (2%) / 21 | 8 (10%) / 18 ² | 13 (16%) / 15 |
| Lexcheck flagged | 34/1241 (2.7%) | 20/1242 (1.6%) | 8/1103 (0.7%) ² | 21/1241 (1.7%) |
| Align-gate LLM fallbacks | 51 (4.0%) | 35 (2.7%) | 43 (3.7%) | **30 (2.3%)** |
| Word-align coverage | **96.6%** | **96.6%** | 93.2% | 96.5% |
| Cost, 5 chapters | $0.085 | ~$0.00002 | ~$0.00013 | $0.19 |
| **Full-book estimate** | **~12–13 min, ~$0.98** | **~16 min, ~$0.0002** | ~1.5 h, ~1100 sentences lost | ~25–60 min ¹, ~$2.21 |

¹ GLM-5.2's median batch is fine (9.8 s) but OpenRouter scattered the 83 requests
across **22 serving providers**, and the slowest few (60–120 s, one timeout) serialize
the end of the phase at this scale. Linear scaling of the measured phase gives ~60 min;
on a full book the tail amortizes across ~920 batches and mean-latency throughput
(~35 sent/s) predicts ~20–25 min. Pinning fast providers via `PROVIDER_ORDER` would
tighten this.

² MiniMax numbers are on the survivor subset — its 113 hardest (longest-reasoning)
sentences never got translated, which flatters both its quality score and its lexcheck rate.

3-way judging (before GLM was added) for reference: gemini 8.16, minimax 7.60, kimi 7.28 —
same ranking.

Full-book projection: phase times scaled by sentence ratio (14 688 / 1279 ≈ 11.5×); the
LaBSE embalign phase (~32 sent/s, local CPU) is identical for all providers and dominates
gemini's total.

## Quality notes

- **Kimi** (thinking off): gap vs gemini is fluency, not fidelity — meaning is preserved
  (top-tier lexcheck and alignment numbers), but grammar slips through: wrong gender
  agreement («под одним простынём»), calques («отмахивающейся небрежностью»), clumsy
  clause syntax («руки обхватили голову, и она была укрыта»).
- **GLM-5.2** (reasoning off): second-best judged quality, best align-gate rate, zero
  data loss — structurally the cleanest non-gemini output. Its weakness is occasional
  stiff phrasing, and operationally the provider-routing latency tail.
- **Gemini** reads like edited prose and keeps the sole-win lead in both judging rounds.

## Verdict

- **Quality default stays `google/gemini-3.1-flash-lite` on OpenRouter** (blind 7.95,
  fewest weak items, fastest, and 2.3× cheaper than GLM-5.2).
- **`--provider gonka` + Kimi-K2.6 is a solid near-free fallback**: zero errors at C=32,
  best-in-test alignability, ~25% slower per book, ~5000× cheaper than gemini. Use when
  volume matters more than polish. It is the `GONKA_MODEL` default.
- **GLM-5.2: usable but not compelling** — quality between Kimi and gemini at 2.3×
  gemini's price and 2–5× its wall time. No niche here: gemini beats it on quality/price,
  Kimi on price, both on speed.
- **MiniMax-M2.7: rejected at defaults.** Un-disableable reasoning eats the 16384-token
  output cap on 16-sentence batches → mass truncation and data loss. A batch size of 4
  would likely fix truncation but not the per-batch reasoning latency (~1.5 h/book).

## Artifacts

Session scratchpad `bench/` (session 0d6842c9): `run-*.log`, `stats-*.jsonl`,
`{gemini,kimi,minimax,glm}.tbook`, `judge_batch_*.json` / `judge4_batch_*.json`,
`judge_key.json` / `judge4_key.json`, `scores.json` / `scores4.json`,
`prepare_judging.py` / `prepare_judging4.py`, `analyze_stats.py`.
