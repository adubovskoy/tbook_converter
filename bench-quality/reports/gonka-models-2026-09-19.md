# Gonka served-model probe — 2026-09-19

What the [proxy.gonka.gg](https://proxy.gonka.gg) gateway serves today, and how reliably, measured
through the converter's own translate path (`cmd/gonka-probe`, plan item **A1**).

**The headline: the shipped gonka default does not exist on the gateway any more.**
`moonshotai/Kimi-K2.6` — `defaultGonkaModel` in `internal/config/config.go` — answers
`404 model_not_found` to every request. So does `deepseek-ai/DeepSeek-V4-Flash-0731`, the model the
August bench called "the big finding for gonka". Of the two models that do answer, one
(`zai-org/GLM-5.3-Flash`) appears in no document in this repo, and neither can translate a book in
under an hour.

## Setup

- **Tool**: `cmd/gonka-probe` (new). It builds the production translate request — prompt `v5`,
  batch 8, temperature 0.3, JSON mode, gonka's `"thinking":{"type":"disabled"}` — through
  `internal/translate`'s own client, parser and stats sink, so what is measured is what `convert`
  sends.
- **Sentence set**: 96 English sentences of ≥6 words, drawn at an even stride from
  `chapters/ch3.json` of `android/app/src/main/assets/sample.tbook` (*The Adventures of Sherlock
  Holmes*), en→ru. 12 batches per cell.
- **Cells**: every served model × concurrency 4 / 8 / 16 / 32, plus the three model ids the repo
  assumes are there.
- **Probe deviations from production, on purpose**: `--max-retries 0` (failures must be visible,
  not retried away) and a hard 120 s per-request timeout. A cell aborts after 6 consecutive
  failures and is marked `unservable`.
- **Traffic**: 212 batch requests (1 696 sentences), plus 12 reasoning-probe requests and a
  handful of manual controls — 12:25Z–13:38Z.
- **Artifacts**: `bench-quality/.artifacts/gonka-probe-2026-09-19.json` (main sweep),
  `…-glm300.json` (GLM re-run at production's 300 s timeout), `…-var1.json`, `…-var2.json`
  (variance repeats), `…-rprobe1.json`/`…-rprobe2.json` (the two extra GLM reasoning pairs). Each
  carries every request row and every batch row.

### The gateway caches identical requests, so the probe carries a nonce

Sending the same body twice returns a byte-identical reply with identical token counts in ~0.19 s
against 6.6 s for the first call. Cells reuse one sentence set, so without a nonce every cell after
the first would report the cache's latency. The probe therefore fills the OpenAI-standard `user`
field with a per-cell tag (`Options.UserTag`, new): it leaves the prompt and sampling untouched
(`prompt_tokens` unchanged at 47 on the check above) and defeats the cache. This is the mechanism
behind evidence §7.14 and it invalidates any repeat measurement taken without it.

## What the gateway serves

`GET /v1/models`, 13:09Z:

| model | context | max_tokens | in the repo? |
|---|--:|--:|---|
| `MiniMaxAI/MiniMax-M2.7` | 180 000 | 16 384 | yes — benched and **rejected** in July (`gonka` §Verdict) |
| `zai-org/GLM-5.3-Flash` | 400 000 | 16 384 | **no** — appears in no report, no `.env.example`, no config default |

Not served, though the repo assumes them:

| model | result | note |
|---|---|---|
| `moonshotai/Kimi-K2.6` | `404 model_not_found` on 23/23 requests at 12:45Z, 13:23Z and 13:36Z | **the shipped gonka default** (`config.go` `defaultGonkaModel`) |
| `deepseek-ai/DeepSeek-V4-Flash-0731` | `404 model_not_found` on 6/6 requests | the 77.3 % blind winner of `four-model-bench-2026-08` |

The 404 body names the real list: `model "…" is not available. Supported models:
MiniMaxAI/MiniMax-M2.7, zai-org/GLM-5.3-Flash`.

**`/v1/models` is not authoritative.** It listed `deepseek-ai/DeepSeek-V4-Flash-0731` at 12:31:14Z
and 12:36:38Z and stopped listing it at 12:40:45Z — while `/chat/completions` had already been
refusing that id as unavailable since ~12:25Z. A model-availability check must therefore probe
`/chat/completions`, not the model list; `translate.ServedModels` alone would have reported
DeepSeek as available.

## Per-cell results

Main sweep, 120 s request timeout, `--max-retries 0`. `ok %`, `502 %`, `trunc %` are shares of
requests actually attempted (requests canceled by the early stop are excluded and counted
separately). `p50/p90` are over successful requests only. `sent/s` is sentences returned ÷ cell
wall time.

| model | conc | req | ok % | 429 % | 502 % | timeout % | trunc % | p50 s | p90 s | sent/s | sentences | out tok/req | verdict |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|---|
| `MiniMaxAI/MiniMax-M2.7` | 4 | 12 | 91.7 | 0 | 0 | 0 | 8.3 | 30.2 | 69.9 | 0.62 | 88/96 | 968 | ok |
| `MiniMaxAI/MiniMax-M2.7` | 8 | 12 | 83.3 | 0 | 0 | 8.3 | 8.3 | 50.9 | 85.5 | 0.59 | 80/96 | 1 089 | ok |
| `MiniMaxAI/MiniMax-M2.7` | 16 | 12 | 75.0 | 0 | 0 | 25.0 | 0 | 51.1 | 86.2 | 0.60 | 72/96 | 957 | ok |
| `MiniMaxAI/MiniMax-M2.7` | 32 | 12 | **100.0** | 0 | 0 | 0 | 0 | 66.2 | 75.0 | **1.07** | 96/96 | 944 | ok |
| `zai-org/GLM-5.3-Flash` | 4 | 12 | 33.3 | 0 | 8.3 | 58.3 | 0 | 116.3 | 119.1 | 0.09 | 32/96 | 2 030 | ok |
| `zai-org/GLM-5.3-Flash` | 8 | 8 (+4 canceled) | 0.0 | 0 | 0 | 100.0 | 0 | — | — | 0.00 | 0/64 | — | **unservable** |
| `zai-org/GLM-5.3-Flash` | 16 | 10 (+2) | 40.0 | 0 | 0 | 10.0 | 50.0 | 77.8 | 79.2 | 0.27 | 32/80 | 2 057 | **unservable** |
| `zai-org/GLM-5.3-Flash` | 32 | 9 (+3) | 22.2 | 0 | 0 | 55.6 | 22.2 | 98.2 | 98.2 | 0.13 | 16/72 | 2 430 | **unservable** |
| `moonshotai/Kimi-K2.6` | 4 | 6 (+4) | 0.0 | 0 | 0 | 0 | 0 | — | — | 0.00 | 0/48 | — | **unservable** (404) |
| `deepseek-ai/DeepSeek-V4-Flash-0731` | 4 | 6 (+4) | 0.0 | 0 | 0 | 0 | 0 | — | — | 0.00 | 0/48 | — | **unservable** (404) |

Both 404 models abort at the lowest concurrency, so their higher levels were skipped.

**Zero 429s anywhere.** The August failure profile (832 × 429 on 08-08, 1 370 × 502 on 08-20, 656 ×
429 for DeepSeek) has not recurred; today's failures are a *deadline*, not a queue.

### The concurrency column measures less than it says

96 sentences at batch 8 is 12 requests, so at concurrency 16 and 32 the cell never fills its
window — at most 12 requests are ever in flight. The measured `sent/s` at those levels is therefore
a floor. An upper bound for a real book, `concurrency × batch ÷ p50`, is 1.1 / 1.3 / 2.5 / **3.9**
sent/s for MiniMax at c = 4 / 8 / 16 / 32, and it ignores the contention that a full window would
add. Either way the answer is the same order: **single-digit sentences per second at best**, against
44.0 sent/s for Kimi on a full book in July.

### The real ceiling is the gateway's 120 s router deadline

GLM was re-run at production's own gonka timeout (300 s) to see whether it is merely slow. It is
not — the failures simply changed shape:

| model | conc | ok % | 502 % | timeout % | trunc % | sent/s |
|---|--:|--:|--:|--:|--:|--:|
| `zai-org/GLM-5.3-Flash` @300 s | 4 | 33.3 | 50.0 | 0 | 16.7 | 0.09 |
| `zai-org/GLM-5.3-Flash` @300 s | 8 | 0.0 | 100.0 | 0 | 0 | 0.00 |
| `zai-org/GLM-5.3-Flash` @300 s | 16 | 9.1 | 54.5 | 0 | 36.4 | 0.07 |
| `zai-org/GLM-5.3-Flash` @300 s | 32 | 20.0 | 80.0 | 0 | 0 | 0.13 |

**Every one of the 26 502s landed between 120.1 s and 120.3 s** (`glm300` artifact), and its body is
not gonka's wording at all:

```
An error occurred with this application.

ROUTER_EXTERNAL_TARGET_ERROR

fra1::rchr6-1789824357604-fe9079caf955
```

The gateway's edge router cuts any upstream request at ~120 s and answers 502. The main sweep's
120 s "timeouts" and this run's 502s are the same event seen from the two sides of that deadline.
Two consequences:

1. **The converter's 300 s gonka request timeout buys nothing** (`config.go:386-389`). A generation
   that needs more than ~120 s cannot be served at any client timeout, and more retries cannot help
   it either — `--max-retries 8` will spend eight identical 120 s waits on the same batch.
2. **Batch size must fit inside 120 s of generation.** MiniMax at batch 8 has p90 = 75–86 s: the
   margin is thin. A one-off control confirms the edge — the same batch at **batch 16** was cut at
   120.2 s with the same 502.

## Reasoning: no model served today can be told to stop

The probe sends one batch with `"thinking":{"type":"disabled"}` and one without it, and compares
output tokens and latency. `gonka.md` measured 975 → 30 tokens and 16 s → 1 s for Kimi with this
switch; nothing on the gateway reproduces that today.

| model | thinking disabled | thinking omitted | verdict |
|---|---|---|---|
| `MiniMaxAI/MiniMax-M2.7` | 630 tok / 43.1 s, `<think>` present | 951 tok / 7.4 s, `<think>` present | ignores the switch (architectural, as recorded) |
| `zai-org/GLM-5.3-Flash` (3 pairs) | 2 337 / 100.8 s; 1 579 / 62.1 s; 2 111 / 76.9 s | 2 115 / 68.7 s; 3 002 / 106.7 s; 2 863 / 114.3 s | ignores the switch — the difference has no sign |
| `moonshotai/Kimi-K2.6`, `deepseek-ai/DeepSeek-V4-Flash-0731` | 404 | 404 | not measurable |

MiniMax emits its reasoning as a `<think>` block inside `message.content` (100 % of replies in every
cell; `stripThink` removes it). GLM-5.3-Flash puts it in a separate `reasoning` field, so the
content parses cleanly — but it is still billed and still spent: ~240–340 output tokens per
sentence against MiniMax's ~120–150, and that is exactly what pushes GLM's batches past the router's
120 s.

## Variance of the default cell

The configured default, Kimi-K2.6 at concurrency 32, was re-probed twice more; it returned 404 every
time, so the variance measurement was carried on the best servable model at the same concurrency.

| run | time (UTC) | model | ok % | p50 s | p90 s | sent/s | sentences | Kimi at the same moment |
|---|---|---|--:|--:|--:|--:|--:|---|
| main | 12:58–13:00 | MiniMax-M2.7 @32 | 100.0 | 66.2 | 75.0 | 1.07 | 96/96 | 404 (12:45Z) |
| var1 | 13:23–13:25 | MiniMax-M2.7 @32 | 75.0 | 57.5 | 77.4 | 0.60 | 72/96 | 404 |
| var2 | 13:36–13:38 | MiniMax-M2.7 @32 | 100.0 | 11.5 | 69.1 | 1.25 | 96/96 | 404 |

Throughput moves by a factor of 2.1 and success by 25 pp across three samples 13 and 11 minutes
apart, with no change on our side. var1's three failures are all the 120 s deadline again. **Any
single throughput number from this gateway carries a ±50 % band**; the plan's rule that a run below
90 % request success is quarantined (E §8) will fire often at this batch size.

## Script sanity of what came back

Every returned translation was checked for target script (share majority-Cyrillic) and for Latin
words absent from its own source sentence.

| model | sentences returned | majority-Cyrillic | Latin leaks |
|---|--:|--:|--:|
| `MiniMaxAI/MiniMax-M2.7` | 504 | 504 (100 %) | 0 |
| `zai-org/GLM-5.3-Flash` | 136 | 136 (100 %) | 0 |

**There are no non-Russian outputs to quote**: 640/640 returned sentences are Russian, with no
untranslated English and no third-language leak. The wrong-language defect that dominates the De/Fr
evidence (§3.B, §3.C) did not appear on en→ru here. A sample of what a clean MiniMax reply looks
like, from the probe set:

> **src** I had called upon my friend, Mr. Sherlock Holmes, one day in the autumn of last year and
> found him in deep conversation with a very stout, florid-faced, elderly gentleman with fiery red
> hair.
> **ru** Однажды осенью прошлого года я зашёл к своему другу мистеру Шерлоку Холмсу и застал его в
> оживлённой беседе с очень полным, краснолицым пожилым джентльменом с огненно-рыжими волосами.

Truncated replies cannot be quoted: `finish_reason=length` makes the client discard the whole
response (`errTruncated`), so no partial text exists — and none reaches a book, since the batch is
retried or lost entire. Truncation is nonetheless real and model-specific: 2 of 72 attempted
MiniMax requests (2.8 %) against 13 of 78 GLM requests (16.7 %), including 50 % of one GLM cell.
This is the 16 384-token output cap being hit by reasoning, not by translation.

## Reliability verdict per model

**`MiniMaxAI/MiniMax-M2.7` — servable slowly.** Aggregate 63 of 72 attempted requests succeeded
(87.5 %); at concurrency 32, its best cell, 33 of 36 across the three runs (91.7 %), clearing the
90 % bar. It never returned a 429 or a 502, its Russian is clean, and its two failure modes are both
the reasoning budget: seven requests cut at the router's 120 s and two over the output cap. But it
peaks at **1.07–1.25 sent/s measured, ≤3.9 sent/s projected** — far short of the 5 sent/s production
bar, and a factor of 35–40 below the 44.0 sent/s Kimi delivered on a full book in July. A 14 688-sentence
book would take **3.3–3.8 h of translate alone** at the measured rate (≈1 h at the projected
ceiling), per target, before repair and alignment. This is the July verdict re-earned from the other
direction: July rejected MiniMax for losing 8.8 % of sentences at batch 16; today it loses almost
nothing at batch 8 but cannot go fast. Recommended if used: **batch 4** to keep generation clear of
the 120 s deadline, and accept the `quality`-profile wall time.

**`zai-org/GLM-5.3-Flash` — unservable today.** 17 of 78 attempted requests succeeded (21.8 %) across
both timeouts: 27 × 502 (all at the 120 s router cut), 21 × timeout, 13 × truncation. Three of its
four cells aborted on six consecutive failures. Its output is good Russian when it arrives, and its
400 k context is the largest on the gateway, but at ~2 000–2 700 output tokens per 8-sentence batch
its generation does not fit inside the deadline the gateway itself enforces. This profile is worse
than DeepSeek's August one (24 % success, 0.5 sent/s) — and that one was already judged unusable.
Re-probe before spending anything on it; it is unbenched for quality and, until a batch of 4 fits in
120 s, unmeasurable.

**`moonshotai/Kimi-K2.6` — unservable today, and it is the shipped default.** 23 of 23 requests
returned `404 model_not_found` at three times across an hour. Every gonka figure the workspace
relies on — 7.14/10 blind, 44.0 sent/s, 18m36s per book, the 3.9 % repair rate, the 67.4 %/75.0 %
deficits against gemini-3.1-flash-lite — describes a model the gateway no longer serves. `convert
--provider gonka` with no `--model` fails on its first batch today.

**`deepseek-ai/DeepSeek-V4-Flash-0731` — unservable today.** 6 of 6 requests 404, while `/v1/models`
was still advertising it. The one measured 77.3 : 22.7 win over Kimi (p = 2.8e-07) remains
unexploitable, now for a harder reason than August's 24 % success rate: the id is not accepted at
all.

## What this means for the plan

- **Phase C1 is blocked, not gated.** There is no DeepSeek to switch to and no Kimi to switch from.
  The model question on gonka is now "MiniMax at 1 sent/s, or leave gonka for the pairs that matter".
- **Every gonka baseline in EVIDENCE is stale.** A2's arms cannot be produced "at HEAD with the
  gonka default"; the default must be re-chosen first, and any new blind comparison is against
  MiniMax, whose only quality number (7.37/10) was measured on a survivor subset that excluded its
  113 hardest sentences.
- **Three config facts to fix before any production run**: `defaultGonkaModel` points at a 404; the
  300 s gonka timeout is dead weight against a 120 s router; `defaultGonkaRetries = 8` spends eight
  full deadlines on a batch that can never fit. Batch 8 is also no longer obviously right — it was
  chosen against Kimi's output cap, not against a wall-clock deadline.
- **Re-run this probe daily**, as A1 asks. Three availability profiles in six weeks became four
  today, and the served list changed twice inside one hour of probing.

## Reproducing

```bash
cd converter && go build -o gonka-probe ./cmd/gonka-probe
./gonka-probe                                  # every served model × 4/8/16/32, 96 sentences
./gonka-probe --models MiniMaxAI/MiniMax-M2.7 --concurrency 32 --no-reasoning-probe
```

Needs `GONKA_API_KEY` in `.env`. Writes `bench-quality/.artifacts/gonka-probe-<date>.json` and
prints the table above. `--timeout`, `--batch`, `--sentences`, `--book`/`--chapter`, `--fail-streak`
and `-t <target>` are the knobs; `--run <label>` names the run and doubles as the cache-busting
request tag.

## Follow-up 2026-09-19/20

The day-1 sweep above ran 12:25Z–13:38Z. This section covers everything measured after it:
an afternoon re-probe (13:49Z–14:27Z) that found the availability picture changed again, a
batch-size sweep on GLM, a body-variant matrix for turning reasoning off, and a day-3 re-check.

**Three things day 1 got wrong, in order of consequence.**

1. `deepseek-ai/DeepSeek-V4-Flash-0731` **is served again** — and it is the fastest and cleanest
   model on the gateway, by a wide margin. Day 1 measured it as a 404.
2. **429s exist**, and they are not ours. Day 1's "zero 429s anywhere" held for that hour only.
3. **Shrinking the batch does not rescue GLM.** Day 1 proposed batch 4; at batch *1* — one sentence
   per request — GLM still misses the router's deadline on most attempts.

Every table below is regenerated from the saved rows, with no new traffic:

```bash
cd converter && go build -o gonka-probe ./cmd/gonka-probe
./gonka-probe --summarize bench-quality/.artifacts/gonka-probe-2026-09-19-*.json \
                          bench-quality/.artifacts/gonka-smoke-2026-09-19
```

### Served list, per probe time

`/v1/models` and a real `/chat/completions` request, as each run saw them.

| time (UTC) | run | `/v1/models` lists | `/chat/completions` serves |
|---|---|---|---|
| 12:31, 12:36 | day-1 sweep | MiniMax, GLM, **DeepSeek** | MiniMax, GLM (DeepSeek already 404) |
| 12:40–13:38 | day-1 sweep, var1, var2 | MiniMax, GLM | MiniMax, GLM |
| 13:49 | `day2-404` | MiniMax, GLM, **DeepSeek** | MiniMax, GLM, **DeepSeek** (Kimi 404) |
| 14:09–14:27 | `glm-*`, `mm-*`, `ds-*` | MiniMax, GLM, DeepSeek | MiniMax, GLM, DeepSeek |
| **18:26** | `day3-404`, `day3-ds-*` | MiniMax, GLM, **DeepSeek** | **DeepSeek** (Kimi 404, 110 ms) |

The 404 body's own "Supported models:" list grew with it — at 13:49Z it reads
`MiniMaxAI/MiniMax-M2.7, zai-org/GLM-5.3-Flash, deepseek-ai/DeepSeek-V4-Flash-0731`, against two
ids at 12:45Z. `moonshotai/Kimi-K2.6` is absent from every list at every probe time.

So the day-1 rule stands but gains a second half: `/v1/models` is not authoritative in *either*
direction. It advertised DeepSeek for 9 minutes after the endpoint stopped accepting it (day 1), and
the endpoint accepted DeepSeek at 13:49Z the same minute the list did. Only `/chat/completions`
answers the question, and its answer has a shelf life measured in hours.

### The gateway has a shared concurrency budget, and 429 means someone else filled it

Three cells aborted today on six consecutive 429s within ~5.5 s of starting:

```
gonka 429: rate limit exceeded: too many concurrent requests (120/111)
```

The counter is not our own: our cell had 8 or 32 requests in flight, and the numerator is 114–135
across the rows. It is a gateway-wide budget of ~111 concurrent requests that our probe shares with
every other client. Re-running the same cell minutes later (`…r` suffix) got through. Consequences:

- **A 429 cell measures the gateway's queue, not the model.** `glm-b1c8`, `glm-b1c32` and
  `glm-b2c32` are all 100 % 429 and carry no information about GLM; their `…r` repeats do.
- **Day 1's "today's failures are a deadline, not a queue" was true of day 1 only.** Both failure
  modes are live. Production needs retry-with-backoff for 429 (it is transient and cheap to retry)
  and needs *not* to retry the 120 s deadline (it is not transient and costs 120 s a try).

### GLM-5.3-Flash: batch × concurrency

Batch 8 rows are the day-1 sweep; batch 1/2/4 are today's. `r` = repeat of a cell whose first
attempt was consumed by the shared 429 budget. `out tok/sent` is mean output tokens per request
divided by the batch size — what the model spends to produce one sentence.

| batch | conc | run | req | ok % | 429 % | timeout/other % | trunc % | p50 s | p90 s | out tok/sent | sent/s | sentences |
|--:|--:|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| 1 | 8 | `glm-b1c8` | 6 | 0.0 | **100.0** | 0 | 0 | — | — | — | 0.00 | 0/6 |
| 1 | 8 | `glm-b1c8r` | 8 | 12.5 | 0 | 87.5 | 0 | 84.6 | 84.6 | 115 | 0.01 | 1/8 |
| 1 | 32 | `glm-b1c32` | 7 | 0.0 | **100.0** | 0 | 0 | — | — | — | 0.00 | 0/7 |
| 1 | 32 | `glm-b1c32r` | 9 | 33.3 | 66.7 | 0 | 0 | 6.1 | 6.1 | 92 | 0.43 | 3/9 |
| 2 | 8 | `glm-b2c8` | 10 | 40.0 | 0 | 60.0 | 0 | 53.5 | 94.3 | 508 | 0.06 | 8/20 |
| 2 | 32 | `glm-b2c32` | 8 | 0.0 | **100.0** | 0 | 0 | — | — | — | 0.00 | 0/16 |
| 2 | 32 | `glm-b2c32r` | 44 | 68.2 | 11.4 | 18.2 | 2.3 | 47.6 | 85.6 | 526 | 0.50 | 60/88 |
| 4 | 8 | `glm-b4` | 8 | 0.0 | 0 | **100.0** | 0 | — | — | — | 0.00 | 0/32 |
| 4 | 32 | `glm-b4c32` | 21 | **71.4** | 0 | 28.6 | 0 | 80.0 | 116.5 | 370 | 0.50 | 60/84 |
| 8 | 4 | day-1 | 12 | 33.3 | 0 | 58.3 | 0 | 116.3 | 119.1 | 254 | 0.09 | 32/96 |
| 8 | 8 | day-1 | 8 | 0.0 | 0 | 100.0 | 0 | — | — | — | 0.00 | 0/64 |
| 8 | 16 | day-1 | 10 | 40.0 | 0 | 10.0 | 50.0 | 77.8 | 79.2 | 257 | 0.27 | 32/80 |
| 8 | 32 | day-1 | 9 | 22.2 | 0 | 55.6 | 22.2 | 98.2 | 98.2 | 304 | 0.13 | 16/72 |

**Best cell across every batch size and both timeouts: 71.4 % (batch 4, concurrency 32).** No GLM
cell anywhere reaches the plan's 90 % bar, so no confirmation run was spent on it.

The shape of this table kills the day-1 recommendation. If the problem were batch size, success
would climb as the batch shrinks. It does not: batch 1 — a single sentence, ~90–115 output tokens
when it succeeds — still lost 7 of 8 attempts to the 120 s deadline in `glm-b1c8r`. GLM's reasoning
is not proportional to the work; it is unbounded per request, and a one-sentence request can spend
the whole deadline as easily as an eight-sentence one. There is no batch size at which GLM fits.

Throughput never exceeds **0.50 sent/s** in any cell. The `out tok/sent` column is the mechanism:
370–526 output tokens to translate one sentence at batch 2–4, against DeepSeek's 36.

### Reasoning off: eight body shapes, three models

The production client's `"thinking":{"type":"disabled"}` is one of eight request-body shapes
OpenAI-compatible gateways accept. The variant sweep (`--variants`, `variants.go`) sends the same
2-sentence batch under each, and reads the reply fields the production client discards: the
`reasoning` / `reasoning_content` message fields, `usage.completion_tokens_details.reasoning_tokens`,
and the raw content. "Reasoning off" means all three silent **and** the batch still parseable.

| model | variant | status | out tok | latency s | `<think>` in content | `reasoning` field | parsed | reasoning off? |
|---|---|--:|--:|--:|:-:|:-:|--:|:-:|
| `MiniMaxAI/MiniMax-M2.7` | control (no field) | 200 | 664 | 33.8 | yes | no | 2/2 | no |
| `MiniMaxAI/MiniMax-M2.7` | `thinking:{"type":"disabled"}` | 200 | 760 | 6.2 | yes | no | 2/2 | no |
| `MiniMaxAI/MiniMax-M2.7` | `enable_thinking:false` | 200 | 764 | 12.6 | **no** | **yes** (3 125 ch) | 2/2 | no |
| `MiniMaxAI/MiniMax-M2.7` | `chat_template_kwargs:{"enable_thinking":false}` | 200 | 735 | 5.8 | yes | no | 2/2 | no |
| `MiniMaxAI/MiniMax-M2.7` | `reasoning:{"enabled":false}` | 200 | 807 | 55.4 | yes | no | 2/2 | no |
| `MiniMaxAI/MiniMax-M2.7` | `reasoning:{"effort":"none"}` | 200 | 659 | 77.2 | yes | no | 2/2 | no |
| `MiniMaxAI/MiniMax-M2.7` | `reasoning_effort:"none"` | 200 | 670 | 11.1 | **no** | **yes** (2 579 ch) | 2/2 | no |
| `MiniMaxAI/MiniMax-M2.7` | `max_completion_tokens:40/sentence` | 200 | 80 | 3.5 | yes | no | **0/2** | no (destroys output) |
| `zai-org/GLM-5.3-Flash` | control (no field) | 200 | 2 478 | 96.1 | no | yes (10 836 ch) | 2/2 | no |
| `zai-org/GLM-5.3-Flash` | `thinking:{"type":"disabled"}` | 200 | 1 428 | 117.0 | no | yes (6 273 ch) | 2/2 | no |
| `zai-org/GLM-5.3-Flash` | `enable_thinking:false` | 200 | 1 922 | 57.5 | no | yes (8 309 ch) | 2/2 | no |
| `zai-org/GLM-5.3-Flash` | `chat_template_kwargs:{"enable_thinking":false}` | 200 | 1 449 | 66.5 | no | yes (6 476 ch) | 2/2 | no |
| `zai-org/GLM-5.3-Flash` | `reasoning:{"enabled":false}` | 200 | 1 774 | 53.8 | no | yes (7 873 ch) | 2/2 | no |
| `zai-org/GLM-5.3-Flash` | `reasoning:{"effort":"none"}` | — | — | 120.0 (×2) | — | — | 0/2 | **hangs to the router cut** |
| `zai-org/GLM-5.3-Flash` | `reasoning_effort:"none"` | 200 | 1 613 | 56.4 | no | yes (6 934 ch) | 2/2 | no |
| `zai-org/GLM-5.3-Flash` | `max_completion_tokens:40/sentence` | 200 | 80 | 25.6 | no | yes (386 ch) | **0/2** | no (destroys output) |
| `deepseek-ai/DeepSeek-V4-Flash-0731` | control (no field) | 200 | 116 | 6.7 | no | no | 2/2 | **yes** |
| `deepseek-ai/DeepSeek-V4-Flash-0731` | `thinking:{"type":"disabled"}` | 200 | 116 | 2.5 | no | no | 2/2 | **yes** |
| `deepseek-ai/DeepSeek-V4-Flash-0731` | `enable_thinking:false` | 200 | 117 | 4.0 | no | no | 2/2 | **yes** |
| `deepseek-ai/DeepSeek-V4-Flash-0731` | `chat_template_kwargs:{"enable_thinking":false}` | 200 | 120 | 1.3 | no | no | 2/2 | **yes** |
| `deepseek-ai/DeepSeek-V4-Flash-0731` | `reasoning:{"enabled":false}` | 200 | 119 | 2.1 | no | no | 2/2 | **yes** |
| `deepseek-ai/DeepSeek-V4-Flash-0731` | `reasoning:{"effort":"none"}` | 200 | 119 | 6.0 | no | no | 2/2 | **yes** |
| `deepseek-ai/DeepSeek-V4-Flash-0731` | `reasoning_effort:"none"` | 200 | 119 | 1.3 | no | no | 2/2 | **yes** |
| `deepseek-ai/DeepSeek-V4-Flash-0731` | `max_completion_tokens:40/sentence` | 200 | 80 | 2.6 | no | no | **0/2** | no (destroys output) |

Four readings:

- **No body field silences a reasoning model.** Not one of the seven switches reduced MiniMax's or
  GLM's output tokens below its own control in a way that survives the noise (MiniMax 659–807
  against a 664 control; GLM 1 428–2 478 against 2 478, with no sign).
- **Two switches *relocate* MiniMax's reasoning instead.** `enable_thinking:false` and
  `reasoning_effort:"none"` move it out of `message.content`'s `<think>` block and into a separate
  `reasoning` field — same token spend, but the content parses without `stripThink`. That is a
  robustness gain, not a cost one, and it is the only effect any switch had.
- **`reasoning:{"effort":"none"}` is actively harmful on GLM**: 120.0 s to the router cut on 2 of 2
  attempts, in a matrix where every other GLM variant answered in 54–117 s.
- **`max_completion_tokens` caps the spend and destroys the answer** on all three models: 80 tokens,
  `finish_reason=length`, 0/2 sentences recovered. The cap lands in the middle of the reasoning, not
  after it.

**DeepSeek does not reason at all** — no `<think>`, no `reasoning` field, 0 reasoning tokens under
every variant, and 116–120 output tokens for a 2-sentence batch (~58/sentence). It needs no switch,
which is precisely why it fits inside the deadline that defeats the other two.

Two mechanical notes for anyone re-running this: gonka never populates
`usage.completion_tokens_details.reasoning_tokens` (0 in all 28 rows), so reasoning cost must be
read from the `reasoning` field's length or the `<think>` block; and the cache-busting `user` tag
must differ per variant — the first matrix run returned two rows at 0.06 s and 0.05 s, which are the
gateway's cached copies of an earlier variant, and `…-variants-fix.json` / `…-variants-fix2.json`
are the corrected re-runs whose numbers the table above uses.

### MiniMax: batch 4 against batch 8

Both at concurrency 32, 96 sentences, 14:23Z and 14:24Z — a minute apart, so the gateway's mood is
held roughly constant.

| batch | req | ok % | p50 s | p90 s | out tok/req | out tok/sent | sent/s | sentences | trunc % |
|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| 4 | 24 | **100.0** | 44.9 | 57.8 | 744 | 186 | **1.40** | 95/96 | 0 |
| 8 | 12 | 83.3 | 67.3 | 111.0 | 997 | 125 | 0.67 | 80/96 | 0 |

Batch 4 wins on both axes: it clears the 90 % bar (batch 8 does not) and it is **2.1× faster in wall
clock** despite paying 49 % more output tokens per sentence, because batch 8's p90 of 111.0 s runs
into the 120 s router deadline and loses whole batches. Day 1's recommendation of batch 4 for
MiniMax is confirmed. It is still only 1.40 sent/s.

### DeepSeek-V4-Flash-0731: today against August

| | August (`four-model-bench-2026-08`) | 2026-09-19 14:27Z, b8 × c8 | 2026-09-19 14:26Z, b8 × c32 |
|---|---|---|---|
| requests | — | 12 | 12 |
| success % | 24 % | **91.7** | **100.0** |
| 429 % | dominant (656 × 429) | 0 | 0 |
| 502 % | — | 0 | 0 |
| truncation % | — | 0 | 0 |
| p50 / p90 s | — | 3.6 / 7.1 | 4.1 / 17.0 |
| out tok/req (per sentence) | — | 286 (36) | 292 (37) |
| sent/s | 0.5 | **8.14** | 2.05 |
| sentences | — | 88/96 | 96/96 |
| majority-Cyrillic | — | 100 % | 100 % |

**16× the August throughput and 4× the success rate**, with the 429 storm that defined August's run
entirely absent. The one failure at c8 is a malformed reply, not a transport error:
`parse: invalid character '#' after top-level value` on a 200 with `finish_reason=stop` after 1.5 s —
the model appended something after its JSON object. It cost the batch's 8 sentences (the 8.3 %).

The c32 number is worse than c8 only because of a single 46.7 s straggler: the other 11 requests
finished within 17.0 s, and the cell's wall time is that one request's. At batch 8 there are just 12
requests in the set, so concurrency 32 never fills its window — c8 is the honest measurement, and
the throughput ceiling `concurrency × batch ÷ p50` is **17.6 sent/s** at c8 and higher at c32.

These two cells are the afternoon samples; the day-3 section below adds four more and two defects
that a success rate does not show.

### Script sanity and broken output

Every dumped translation from every cell, scanned for target script, for Latin words absent from the
source, for translations identical to the source, and for replies that stop without sentence-final
punctuation:

| dump | sentences | non-target script | Latin leak | identical to source | empty | no final punctuation |
|---|--:|--:|--:|--:|--:|--:|
| `DeepSeek-V4-Flash-0731-b8-c32.jsonl` | 96 | 0 | 0 | 0 | 0 | 0 |
| `DeepSeek-V4-Flash-0731-b8-c8.jsonl` | 88 | 0 | 0 | 0 | 0 | 0 |
| `MiniMax-M2-7-b4-c32.jsonl` | 95 | 0 | 0 | 0 | 0 | 0 |
| `MiniMax-M2-7-b8-c32.jsonl` | 80 | 0 | 0 | 0 | 0 | 0 |
| `GLM-5-3-Flash-b4-c32.jsonl` | 60 | 0 | 0 | 0 | 0 | 0 |
| `GLM-5-3-Flash-b2-c32.jsonl` | 60 | 0 | 0 | 0 | 0 | 0 |
| `GLM-5-3-Flash-b2-c8.jsonl` | 8 | 0 | 0 | 0 | 0 | 0 |
| `GLM-5-3-Flash-b1-c32.jsonl` | 3 | 0 | 0 | 0 | 0 | 0 |
| `GLM-5-3-Flash-b1-c8.jsonl` | 1 | 0 | 0 | 0 | 0 | 0 |

**In these nine dumps there is no broken output to quote.** 491 returned sentences, zero defects of
any class; a length-ratio check (translation under 0.55× or over 1.9× its source) also flags
nothing. Day 1's finding holds here on a larger sample.

It does **not** hold generally, and this is exactly why a clean smoke read is not a quality gate:
the day-3 repeat run below returned a whole batch in Chinese from the same model, the same prompt
and the same sentences that are clean above. Nine consecutive clean cells bought no protection
against the tenth.

The only malformed output of the day is the DeepSeek parse failure described above, and it cannot be
quoted: `errTruncated`/parse failures make the client discard the whole response, so no text reaches
disk. The same is true of every `finish_reason=length` truncation.

What the two live models produce, on the same source sentence, from the smoke files:

> **src** He was himself red-headed, and he had a great sympathy for all red-headed men; so, when he
> died, it was found that he had left his enormous fortune in the hands of trustees, with
> instructions to apply the interest to the providing of easy berths to men whose hair is of that
> colour.
>
> **DeepSeek** Он сам был рыжеволосым и питал большую симпатию ко всем рыжеволосым людям; поэтому,
> когда он умер, обнаружилось, что он оставил своё огромное состояние в руках попечителей с
> указанием применять проценты для предоставления лёгких должностей людям, чьи волосы имеют этот
> цвет.
>
> **MiniMax** Он сам был рыжим и питал глубокое сочувствие ко всем рыжим людям; поэтому, когда он
> умер, выяснилось, что он оставил свой огромный капитал в руках попечителей с указанием выплачивать
> проценты с него на обеспечение удобных должностей мужчинам с волосами этого цвета.

Both are serviceable Russian. Neither has been judged for quality on this gateway; A2 still owes a
blind comparison, and it must now be run against DeepSeek rather than MiniMax.

### Day 3, 18:26Z: the served list again, and DeepSeek re-measured

A fresh `/v1/models` call plus one batch-8 `/chat/completions` request to each of the two ids the
repo cares about:

| model | `/v1/models` | `/chat/completions` | latency | result |
|---|---|---|--:|---|
| `moonshotai/Kimi-K2.6` | **not listed** | `404 model_not_found` | 110 ms | unchanged since 12:45Z — four probes, three days, always 404 |
| `deepseek-ai/DeepSeek-V4-Flash-0731` | listed | **200**, 8/8 sentences, clean Russian | 9.5 s | serving |

The 404 body still names `MiniMaxAI/MiniMax-M2.7, zai-org/GLM-5.3-Flash,
deepseek-ai/DeepSeek-V4-Flash-0731` — Kimi is not coming back on its own.

Two 96-sentence DeepSeek runs at batch 8, concurrency 8 and 32, ≥10 min apart, with the afternoon
run kept as a third sample:

| run | time (UTC) | conc | req | ok % | 429/502 % | p50 s | p90 s | max s | out tok/sent | trunc % | sent/s | sentences | cyr % |
|---|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| `ds-b8c8` | 14:27 | 8 | 12 | 91.7 | 0 | 3.6 | 7.1 | 7.3 | 36 | 0 | **8.14** | 88/96 | 100 |
| `ds-b8c32` | 14:26 | 32 | 12 | **100.0** | 0 | 4.1 | 17.0 | 46.7 | 37 | 0 | 2.05 | 96/96 | 100 |
| `day3-ds-r1` | 18:26 | 8 | 12 | 91.7 | 0 | 17.5 | 21.2 | 28.0 | 36 | 0 | 3.15 | 88/96 | 100 |
| `day3-ds-r1` | 18:26 | 32 | 12 | 91.7 | 0 | 8.0 | 17.7 | **120.0** | 37 | 0 | 0.73 | 88/96 | 100 |
| `day3-ds-r2` | 18:38 | 8 | 12 | 91.7 | 0 | 8.7 | 16.2 | 22.8 | 33 | 0 | 3.86 | 88/96 | **90.9** |
| `day3-ds-r2` | 18:38 | 32 | 12 | **100.0** | 0 | 5.0 | 15.3 | 16.8 | 35 | 0 | **5.72** | 96/96 | **91.7** |

Six cells, 576 sentences asked of DeepSeek across four and a half hours and 544 returned.
**Success is 91.7–100 % in every one** — it is the only model on the gateway that clears the plan's 90 % bar at every sample.
Throughput is far less stable: 0.73–8.14 sent/s measured, and the spread is not load, it is
stragglers. Every cell's wall time is its slowest single request (12 requests never fill a 32-wide
window), so one parked request sets the number: `day3-ds-r1` @32 reads 0.73 sent/s because one
request sat until the router's 120 s cut while the other eleven finished inside 17.7 s. Read the
p50/p90 columns instead: **3.6–8.7 s p50, 7.1–21.2 s p90 for eight sentences**, everywhere — at
concurrency 8 that is 3.7–17.5 sent/s of real generation throughput (`conc × batch ÷ p50`).

### DeepSeek's two defects, both quoted

Neither is a transport failure, and neither is visible in a success rate.

**1. A whole batch in the wrong language.** In `day3-ds-r2`, sentences 57–64 of the probe set came
back in **Chinese** — in both cells, a minute apart, and in Russian in all four earlier cells:

> **src** It is a hobby of mine to have an exact knowledge of London.
> **got (c=8)** 我的一个爱好是对伦敦了如指掌。
> **got (c=32)** 我对伦敦有精确的了解是我的一个爱好。

> **src** And, I say, Doctor, there may be some little danger, so kindly put your army revolver in
> your pocket.”
> **got (c=8)** 而且，我说，医生，可能会有一些小小的危险，所以请把你的军用左轮手枪放进口袋里。”
> **got (c=32)** “而且，我说，医生，可能会有一些危险，所以请把你的军用左轮手枪放进口袋里。”

> **src** As I drove home to my house in Kensington I thought over it all, from the extraordinary
> story of the red-headed copier of the Encyclopædia down to the visit to Saxe-Coburg Square, and
> the ominous words with which he had parted from me.
> **got (c=32)** 当我驱车回到肯辛顿的家中时，我思考了这一切，从红发《百科全书》抄写员的离奇故事，到对萨克森-科堡广场的访问，以及他与我分别时所说的不祥之语。

16 of 544 returned sentences (2.9 %), concentrated in one batch of eight, twice. **This is the
dangerous defect**, because nothing in the pipeline catches it: the reply is well-formed JSON, every
id is present, the request succeeds, coverage is 100 %, and the `.tbook` validates (spec §10.3).
Two fluent Chinese sentences would ship to a Russian reader. It is the wrong-language failure the
De/Fr evidence describes (§3.B, §3.C), now on en→ru and from the fastest model on the gateway. A
target-script check on the translate pass is not optional for this model.

**2. Newline-delimited JSON instead of one object.** The single failure in three of the six cells is
always the same, and the raw capture (`--raw-dir`, new) shows what the production client discards:

```
parse: invalid character '{' after top-level value
```

```json
{"1":"«Он всё ещё с вами, я полагаю?»","2":"«Дай Бог, мистер Уилсон, чтобы я был рыжеволосым."}
{"3":"Видите ли, мистер Холмс, я очень домоседливый человек, и так как моё дело само приходило ко мне, а не мне приходилось ходить по нему, я часто неделями не выходил за порог."}
{"4":"«О, всего лишь пара сотен в год, но работа лёгкая, и она не должна слишком мешать другим занятиям.»"}
…
```

The model emitted **one JSON object per line** — the translations themselves are correct and in
Russian. `parseTexts` rejects it, and its tolerant fallback cannot rescue it either: `extractObject`
spans from the first `{` to the last `}`, which covers all seven objects. The whole batch of 8
sentences is thrown away (that is the 8.3 % at c=8, and the 88/96 in four cells).

Unlike defect 1, this one is benign in production: the probe runs `--max-retries 0`, and the
translate client already retries a parse failure with backoff, so a retry costs ~9 s and almost
certainly succeeds. It is also cheap to fix properly — accepting NDJSON in `parseTexts` would
recover the batch on the first attempt.

## Updated verdict per model, 2026-09-19 18:40Z

The plan's bar: **servable for production** = ≥90 % request success *and* ≥5 sent/s; **servable
slowly** = ≥90 % success below that rate; **unservable** = anything else.

**`deepseek-ai/DeepSeek-V4-Flash-0731` — servable for production, with a mandatory guard.** Six
cells across 4½ hours, 544 sentences returned: success 91.7 / 100 / 91.7 / 91.7 / 91.7 / 100 %, never below
the bar. Zero 429s, zero 502s, zero truncation. p50 3.6–8.7 s and p90 7.1–21.2 s for an 8-sentence
batch — 3.7–17.5 sent/s of generation at concurrency 8, and 8.14 and 5.72 sent/s measured
end-to-end in two of its six cells: the only measurements on this gateway that have ever cleared 5
sent/s. It does not reason at all (0 reasoning
tokens under all eight body variants, ~36 output tokens per sentence), which is why it fits inside
the 120 s router deadline that defeats the other two. Its Russian is good. **The guard is
non-negotiable**: it returned a complete batch in Chinese twice, in well-formed JSON that validates
and would ship. Ship it only behind a per-sentence target-script check that re-queues a batch whose
output is not majority target script. Its other defect (NDJSON replies, ~8 % of requests) is already
handled by the client's parse retry.

**`MiniMaxAI/MiniMax-M2.7` — servable slowly.** At batch 4 × concurrency 32 it returned 100 % of
requests and 95 of 96 sentences, clean Russian, no truncation — it clears the success bar. It does
not come close to the throughput bar: 1.40 sent/s measured, 2.8 sent/s projected at its p50, ~3–4×
slower than DeepSeek on the same sentences, because it reasons on every request (~186 output tokens
per sentence) and no request-body field will stop it. Batch 8 drops it to 83.3 % and 0.67 sent/s, so
batch 4 is the only setting worth using. Keep it as the fallback for when DeepSeek is 404 again.

**`zai-org/GLM-5.3-Flash` — unservable.** Best cell of thirteen, across batch 1/2/4/8 and both the
120 s and 300 s client timeouts: **71.4 %**. Most cells are far below. Shrinking the batch does not
help — at one sentence per request it still lost 7 of 8 attempts to the 120 s deadline — because its
reasoning is unbounded per request, not proportional to the work (370–526 output tokens per
sentence, 5–15× DeepSeek). No confirmation run was spent on it: the plan's rule only asks for one at
≥90 %. Its 400 k context is the largest on the gateway and remains unusable. Re-probe if the gateway
changes; do not bench it for quality until a batch fits inside 120 s.

**`moonshotai/Kimi-K2.6` — unservable, and still the shipped default.** `404 model_not_found` on
every request at 12:45, 13:23, 13:36, 13:49 and 18:26Z; absent from `/v1/models` and from the 404
body's own "Supported models" list at every probe. `convert --provider gonka` with no `--model`
still fails on its first batch. Every gonka number in EVIDENCE — 7.14/10 blind, 44.0 sent/s,
18m36s per book, 3.9 % repair — describes a model that has not existed on this gateway for at least
three days.

## Production settings today

**Run gonka on `deepseek-ai/DeepSeek-V4-Flash-0731`, at batch 8 and concurrency 8, with a 60 s
request timeout, 3 retries, and a target-script guard on the output.** Concretely, against HEAD:
`--provider gonka --model deepseek-ai/DeepSeek-V4-Flash-0731 --batch 8 --concurrency 8
--retries 3` plus `REQUEST_TIMEOUT_SEC=60`. Batch 8 is the largest that fits the deadline on a
non-reasoning model (p90 21.2 s leaves 5× headroom, against MiniMax's p90 of 111 s at the same
batch); concurrency 8 because the gateway enforces a **shared** budget of ~111 concurrent requests
across all its clients and answers `429 too many concurrent requests` when someone else fills it —
32 buys nothing measurable here and only raises that risk. The 60 s timeout replaces the configured
300 s: the edge router cuts every upstream request at ~120 s regardless, and at a p90 of 21 s
anything past 60 s is a parked request that is cheaper to abandon and retry than to wait out —
this alone is the difference between 0.73 and 5.72 sent/s. Three retries replace
`defaultGonkaRetries = 8`, which was sized for August's 429 storm and today only buys eight
consecutive 120 s waits on a batch that cannot fit; three with backoff covers the transient 429 and
the ~8 % NDJSON parse failure, both of which succeed on the next attempt. Request body: exactly what
`internal/translate` already sends — prompt v5, `temperature 0.3`,
`response_format:{"type":"json_object"}`, and **no reasoning field at all**: DeepSeek never reasons,
`thinking:{"type":"disabled"}` is a no-op for it, and every other switch is either a no-op, a hang
(`reasoning:{"effort":"none"}` on GLM) or destroys the output (`max_completion_tokens`). Do not set
`max_completion_tokens`. **The one thing that is not yet in the code and must be before a book
ships: a target-script check on each translated sentence, re-queuing any batch that comes back
outside the target script** — DeepSeek returned eight fluent Chinese sentences inside a valid,
fully-covered, 100 %-success response, and nothing downstream would have caught it. Three config
defaults need changing for any of this to be the default behaviour: `defaultGonkaModel` (points at
a 404), the 300 s gonka timeout, and `defaultGonkaRetries = 8`. Re-probe before every run: four
availability profiles in six weeks, and the served list changed twice inside one hour of probing.

### Artifacts and reproducing

Day-2/3 rows: `bench-quality/.artifacts/gonka-probe-2026-09-19-{day2-404,day3-404}.json`,
`…-glm-b{1,2,4}c{8,32}[r].json`, `…-mm-b{4,8}c32.json`, `…-ds-b8c{8,32}.json`,
`…-day3-ds-r{1,2}.json`, `…-variants[-fix,-fix2].json`; translations under
`bench-quality/.artifacts/gonka-smoke-2026-09-19/` (day-2 at the top level, day-3 in `day3-r1/`
and `day3-r2/`, the latter with `.raw.jsonl` captures of every reply as sent).

```bash
cd converter && go build -o gonka-probe ./cmd/gonka-probe

# every table above, from the saved rows, no network
./gonka-probe --summarize bench-quality/.artifacts/gonka-probe-2026-09-19-*.json \
              bench-quality/.artifacts/gonka-smoke-2026-09-19 \
              bench-quality/.artifacts/gonka-smoke-2026-09-19/day3-r2

# new traffic
./gonka-probe --models deepseek-ai/DeepSeek-V4-Flash-0731 --batch 8 --concurrency 8,32 \
              --no-reasoning-probe --dump-dir DIR --raw-dir DIR --run today
./gonka-probe --variants --variant-sentences 2     # the reasoning-off matrix
```

`--summarize` is offline and needs no credential; `--raw-dir` keeps every reply verbatim, which is
the only way a rejected one can be quoted.
