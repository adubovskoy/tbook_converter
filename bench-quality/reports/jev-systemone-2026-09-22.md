# TypeSafe Jev as a translation-quality gate — probe, 2026-09-22

**Question.** TypeSafe's Jev (`~typesafe/jev-latest` on OpenRouter, beta since
2026-09-18) is a "System One" model: it does not generate text, it answers typed
questions about a state with calibrated probabilities, at $0.042 per million input
tokens and 70–500 ms. Does it buy the converter anything?

**Answer in one paragraph.** Yes, in exactly one place, and no everywhere else it was
tried. As an *absolute* faithfulness scorer on a (source, translation) pair Jev is an
informative but imprecise risk signal: on the Russian GOLD-lite set it separates
`wrong` from `ok` with AUROC 0.88 (0.91 for its three-level score), catches the one
sentence that passed every existing gate (`h4-259`, «Ещё один вопрос.», faithful =
0.01), and ranking sentences by its `faithful` probability puts about twice as many
defects into the bottom 15 % as a random sample would — for $0.03 per 1,000
sentences. That makes it a candidate replacement for the 5 % random half of
`--judge-scope flagged`, not for the judge. As a *pairwise* judge it fails: 47 % of
pairs flip with presentation order, it picks the second candidate 64–67 % of the time,
it agrees with the Sonnet judge on only 62–72 % of the pairs where both are decisive,
and on German it inverts the blind result (DeepSeek "wins" 55–78 % where the LLM judge
had it losing 35 : 65). Its own documentation predicts both findings: translation
faithfulness of a single sentence is close to a "five-second expert judgement";
choosing between two literary translations is not.

## 0. Run record

| | |
|---|---|
| model served | `typesafe/jev-1.13-20260917` (via `~typesafe/jev-latest`, OpenRouter beta) |
| endpoint | `POST https://openrouter.ai/api/v1/systemone` — a dedicated System One API, not chat completions; the model is absent from `/api/v1/models` and only reachable there |
| tool | `cmd/jev-probe` (this repo), key through the converter's `.env` loader, never printed |
| data | GOLD-lite `bench-quality/gold/en-{ru,de,fr}.jsonl` (300 sentences each, labels from the Sonnet checker: `ok` / `minor` / `wrong`, **silver, not human-verified**); the A2 blind pair sets `.artifacts/quality-plan/a2/pairs/dsr-vs-g31-{ru,de,fr}` (400 pairs × 2 orders each, Sonnet verdicts on disk) |
| requests | 900 absolute + 2,400 pairwise; 0 errors, 0 × 429 at concurrency 8 |
| cost | absolute $0.0239 total (≈ 620 input tokens per sentence, four questions); pairwise $0.0490 total; output tokens free |
| latency | p50 380–400 ms, p90 500–700 ms, max 2.5 s |
| raw answers | `bench-quality/.artifacts/jev/gold-{ru,de,fr}.jsonl`, `pairs-dsr-vs-g31-{ru,de,fr}.jsonl` |

Facts about the product, verified against `docs.typesafe.ai` and the OpenRouter
endpoint listing on 2026-09-22: three question types — `noul` (probability that a
statement is true), `choice` (≤ 255 options, probabilities + confidence), `score`
(2–10 ordered levels, expected level + probabilities); questions are evaluated in
parallel against one state, so extra questions cost tokens but almost no latency;
64 k-token request window (32 k for state); rate limit 250 k tokens/s, 1,200 req/min;
no multilingual claim anywhere in the documentation; the "jaggedness" page warns that
the model "answers the question you wrote, not the one you meant", reads negations and
scoping literally, and gives no structural guarantees between related questions.

## 1. Absolute mode: four questions per (src, tr) pair

State `{src, tr}`. Questions (verbatim in `cmd/jev-probe/main.go`):

- `faithful` (noul): "The translation `tr` conveys exactly the meaning of the English
  sentence `src`: nothing is added, nothing is omitted, and no word is mistranslated."
- `natural` (noul): "`tr` reads as natural, grammatical *Language* that a native speaker
  would write."
- `target_lang` (noul): "`tr` is written in *Language*."
- `verdict` (score, 3 levels): wrong / minor / ok, "as a professional literary
  translation reviewer would".

### 1.1 Means by checker class

| lang | class | n | mean faithful | mean natural | mean target_lang | mean score (0–2) |
|---|---|--:|--:|--:|--:|--:|
| ru | ok | 243 | 0.721 | 0.779 | 0.989 | 1.553 |
| ru | minor | 49 | 0.604 | 0.694 | 0.990 | 1.330 |
| ru | wrong | 8 | **0.365** | 0.689 | 0.986 | **0.795** |
| de | ok | 254 | 0.781 | 0.748 | 0.988 | 1.577 |
| de | minor | 40 | 0.673 | 0.623 | 0.989 | 1.362 |
| de | wrong | 6 | 0.598 | 0.668 | 0.988 | 1.190 |
| fr | ok | 259 | 0.775 | 0.792 | 0.987 | 1.594 |
| fr | minor | 38 | 0.644 | 0.713 | 0.989 | 1.373 |
| fr | wrong | 3 | 0.677 | 0.730 | 0.990 | 1.383 |

`target_lang` is 0.99 for every class in every language: Jev handles Cyrillic, German
and French input without any language-specific setup, and the question is useless
because `langcheck` already answers it deterministically.

The ordering is right in all three languages, but the *absolute* level is low: the
median `faithful` on sentences the checker passed is 0.77 (ru), 0.83 (de), 0.82 (fr),
and 30 / 18 / 17 of the `ok` sentences score below 0.5. Jev is unsure about literary
translation as such; it is not a calibrated "this is fine" detector.

### 1.2 Separation (AUROC, `ok` vs defect class)

| lang | signal | ok vs wrong | ok vs minor |
|---|---|--:|--:|
| ru | faithful | **0.876** (243 vs 8) | 0.673 |
| ru | verdict score | **0.914** | 0.716 |
| ru | natural | 0.756 | 0.682 |
| de | faithful | 0.648 (254 vs 6) | 0.684 |
| de | verdict score | 0.655 | 0.702 |
| fr | faithful | 0.635 (259 vs 3) | 0.707 |
| fr | verdict score | 0.649 | 0.726 |

Russian is the only language with a usable `wrong` signal, and it is also the only
language with enough `wrong` labels (8) to say anything; 6 and 3 cases give
confidence intervals wider than the effect. `minor` is separated weakly everywhere
(0.67–0.73), which is the honest ceiling for a model asked to notice «более нежных
страстях» adding a comparative.

### 1.3 As a gate: operating points on `faithful`

| lang | threshold | flagged | wrong caught | minor caught | false flags on ok |
|---|---|--:|--:|--:|--:|
| ru | < 0.5 | 52 / 300 (17.3 %) | 4 / 8 | 18 / 49 | 30 / 243 (12.3 %) |
| ru | < 0.7 | 113 / 300 (37.7 %) | 7 / 8 | 26 / 49 | 80 / 243 (32.9 %) |
| ru | < 0.8 | 191 / 300 (63.7 %) | 8 / 8 | 42 / 49 | 141 / 243 (58.0 %) |
| de | < 0.5 | 29 / 300 (9.7 %) | 2 / 6 | 9 / 40 | 18 / 254 (7.1 %) |
| de | < 0.7 | 72 / 300 (24.0 %) | 3 / 6 | 18 / 40 | 51 / 254 (20.1 %) |
| fr | < 0.5 | 27 / 300 (9.0 %) | 1 / 3 | 9 / 38 | 17 / 259 (6.6 %) |
| fr | < 0.7 | 83 / 300 (27.7 %) | 1 / 3 | 19 / 38 | 63 / 259 (24.3 %) |

No threshold gives a gate: catching 7 of 8 Russian `wrong` costs a 33 % false-flag
rate. What the numbers do support is **ranking**:

### 1.4 As a ranker: defect enrichment in the bottom k % by `faithful`

| lang | defects total | bottom 5 % | bottom 10 % | bottom 15 % | bottom 20 % | bottom 40 % |
|---|--:|---|---|---|---|---|
| ru | 57 (8 wrong) | 6 (10.5 %), ×2.1 | 11 (19.3 %), ×1.9 | **18 (31.6 %), ×2.1**, 4 wrong | 24 (42.1 %), ×2.1 | 35 (61.4 %), ×1.5, all 8 wrong |
| de | 46 (6 wrong) | 7 (15.2 %), ×3.0 | 11 (23.9 %), ×2.4 | **15 (32.6 %), ×2.2**, 3 wrong | 17 (37.0 %), ×1.9 | 32 (69.6 %), ×1.7 |
| fr | 41 (3 wrong) | 6 (14.6 %), ×2.9 | 11 (26.8 %), ×2.7 | **12 (29.3 %), ×2.0**, 1 wrong | 15 (36.6 %), ×1.8 | 27 (65.9 %), ×1.6 |

"×2.1" = the share of defects in that slice divided by the share a random slice of
the same size would hold. A random 15 % holds 15 % of the defects; Jev's bottom 15 %
holds 30–33 %. The enrichment is stable across the three languages even where the
`wrong` AUROC is not, because it is carried mostly by `minor`.

### 1.5 Against the existing flag list

`lexcheck` on the same reference arms (`holmes4-g37-*.tbook.lexflagged.json`) flags
5 / 6 / 12 of the 300 gold sentences and catches **1 / 1 / 2** of the 57 / 46 / 41
defects (0 / 0 / 1 `wrong`). Jev's bottom 15 % catches 18 / 15 / 12. The union catches
19 / 15 / 12 — lexcheck adds one Russian sentence. The two instruments are looking at
different things: lexcheck at glossary drift and leaks, Jev at meaning. This is not a
criticism of lexcheck (it was never a meaning check, and A4 measured what it *is*
for); it is the reason a meaning-side risk signal has been missing.

### 1.6 What Jev flags wrongly — the literal reader

The lowest-scoring `ok` sentences show one mechanism, the one TypeSafe's own
limitations page names ("answers the question you wrote, not the one you meant"):

| lang | faithful | src | tr | why Jev is wrong |
|---|--:|---|---|---|
| ru | 0.08 | “I told you that I would call.” | — Я говорил вам, что зайду. | *call* = visit; «зайду» is right, Jev reads *call* literally |
| de | 0.07 | “I told you that I would call.” | „Ich hatte Ihnen ja mitgeteilt, dass ich vorsprechen würde.“ | same word, same error |
| fr | 0.02 | Jump, Archie, jump, and I’ll swing for it!” | Saute, Archie, saute, et je serai pendu pour ça ! » | *swing for it* = be hanged; the French is exact |
| ru | 0.18 | It seldom was; but he was glad, I think, to see me. | Она редко бывала таковой; но он был рад, я думаю, меня видеть. | elliptical *it seldom was* resolved correctly; Jev penalises the ellipsis |
| fr | 0.14 | ’ he asked with his eyes open. | ” demanda-t-il en ouvrant de grands yeux. | idiomatic rendering scored as an addition |

And what it catches that nothing else did:

| lang | faithful | score | src | tr | checker |
|---|--:|--:|---|---|---|
| ru | **0.01** | 0.00 (conf 0.99) | How did your mother take the matter?” | «Ещё один вопрос. | wrong — the A3 "gate blindness" case |
| ru | 0.02 | 0.04 | Jump, Archie, jump, and I’ll swing for it!” | Прыгай, Арчи, прыгай, а я возьму вину на себя! | wrong (*swing for it* ≠ take the blame) |
| ru | 0.07 | 0.16 | “I will not look.” | «Я не буду искать». | wrong (*look* ≠ search) |
| de | 0.06 | 0.08 | “I will not look.” | „Ich werde nicht suchen.“ | wrong |

Note the asymmetry on *swing for it*: Jev gives 0.02 to both the correct French and the
wrong Russian. It detects that the idiom is *hard*, not whether it was solved. That is
exactly the property a risk router wants and a gate cannot use.

## 2. Pairwise mode: X vs Y vs tie on the A2 blind sets

State `{src, X, Y}`; one `choice` question: "Which is the better literary translation:
more faithful to the meaning, and more natural *Language*?" with options X / Y / tie.
Scored with the A2 protocol (decisive only when both presentation orders agree after
de-swapping; sign test), then compared with the Sonnet verdicts already on disk.

| lang | Jev decisive | Jev dsr share | Sonnet decisive | Sonnet dsr share | Jev order-inconsistent | agree where both decisive |
|---|--:|--:|--:|--:|--:|--:|
| ru | 200 / 400 | 47.5 % (p = 0.52) | 155 | **38.1 %** (p = 0.004) | **196 / 400** | 57 / 85 = 67.1 % |
| de | 202 / 400 | **55.4 %** (p = 0.14) | 168 | **35.1 %** (p = 1.4e-04) | 190 / 400 | 58 / 94 = 61.7 % |
| fr | 208 / 400 | 47.6 % (p = 0.53) | 193 | **39.4 %** | 183 / 400 | 73 / 108 = 67.6 % |

Position: across all 2,400 presentations Jev chose the second candidate (`Y`) 64 % /
67 % / 65 % of the time and `tie` 1–2 %. Requiring a winning probability ≥ 0.7 removes
the inconsistency (13 / 21 / 23 pairs) but leaves 35–45 decisive pairs of 400, and on
German the surviving verdicts go 71 % *for* DeepSeek (p = 0.014) where the LLM judge
found 35 % (p = 1.4e-04). Agreement with Sonnet where both are decisive never exceeds
72 % below the ≥ 0.8 cut, and at ≥ 0.8 there are 9–14 pairs left.

Verdict on pairwise use: **do not use Jev to choose between translations** — not for
best-of-N selection (C3), not as a cheap stand-in for the blind judge, not for
repair-vs-original decisions. The A2 result stands on Sonnet; Jev would have reported
a tie on Russian and the opposite sign on German.

## 3. What this means for the pipeline

Where the converter takes an LLM-shaped decision today, and whether Jev fits:

| decision | today | Jev? | evidence |
|---|---|---|---|
| wrong language / script | `langcheck` (deterministic gate) | no — solved, and Jev's `target_lang` adds nothing | §1.1 |
| glossary drift, leaks | `lexcheck` (deterministic) | no — different axis; Jev catches meaning, not terms | §1.5 |
| **which sentences the judge / repair sees** | `--judge-scope flagged` = lexcheck suspects + **5 % random sample** | **yes** — replace the random 5 % by the Jev-bottom 5–15 % by `faithful`: 2–3× the defects per judged sentence, $0.03 / 1,000 sentences, +0.4 s per batch | §1.4 |
| is this translation acceptable (judge as gate, B4) | LLM judge `j3` on the scoped set | no — 33 % false flags at 7 / 8 recall (ru), AUROC 0.65 on de/fr | §1.2–1.3 |
| best of N / repair vs original (C3, B3) | LLM judge, blind pairs | **no** — position bias, order inconsistency, wrong sign on de | §2 |
| expression tagging (D1) | not built | untested — plausible as a "this sentence contains a non-compositional expression" noul, since Jev flags idioms whether or not they were solved (§1.6); needs a probe against the 194-expression set | — |
| gender decisions (B2) | glossary pass | untested; a `choice` over grammatical gender for a name is inside Jev's design envelope, but the probes need de-identification first | — |

**Recommended next step (small):** a `--judge-scope jev` option (or `flagged+jev`) that
scores every translated sentence with the two noul questions and hands the bottom
*k* % to the judge in place of the random sample. Measure on the Holmes DeepSeek arms
what the judge then finds per 100 judged sentences, against the random-sample
baseline. Kill criterion: fewer than 1.5× the defects per judged sentence. Cost of the
experiment: under $0.20 of Jev plus the judge tokens already budgeted.

**Not recommended:** anything where Jev's answer *is* the decision. Its probabilities
order sentences well and decide them badly; the product's own suitability test says
so, and 3,300 requests here agree.

## 4. Caveats

- The GOLD-lite labels are the Sonnet checker's, marked `silver`; the author's Russian
  pass (pending since A3) may move some `ok`/`minor` boundaries. The `wrong` counts are
  8 / 6 / 3 — Russian is the only language where the `wrong` separation is more than
  anecdote.
- One prompt wording per question, no tuning. Jev's documentation says wording matters
  ("answers the question you wrote"); a second wording round could move the AUROC
  either way and would be the first thing to try before building the scope option.
- Beta on OpenRouter; the `~typesafe/jev-latest` alias moves when TypeSafe ships a
  new release. Pin `typesafe/jev-1.13` in anything reproducible.
- Cost figures are OpenRouter's `usage.cost` field summed per run; per-sentence input
  is ~620 tokens with four questions, ~330 with the two noul questions alone.
