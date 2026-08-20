# Four translate models on one excerpt — gemini-3.7-flash, gemini-3.1-flash-lite, Kimi-K2.6, DeepSeek-V4-Flash-0731

Date: 2026-08-20. HEAD `b0ee45d` (+ the working-tree patch of §5.1 for the gemini-3.7 arm).
Dev set: **Alastair Reynolds, "Revelation Space", chapters ONE+TWO** (`--limit-chapters 2`)
= 1575 sentences / ~21,245 words, en→ru — the same excerpt as
[`kimi-vs-gemma4.md`](kimi-vs-gemma4.md), so the numbers there are directly comparable.

All arms ran the same pipeline with the same flags: glossary on, `hybrid` alignment, lexcheck on,
**`--no-repair`** (the proofread pass is a pipeline lever, not a model property — leaving it on for
gonka would have hidden what the models themselves produce), per-provider default batch/concurrency
(openrouter 16/32, gonka 8/32).

Five arms, because the model the request names as "DeepSeek-V4-Flash-0731 (gonka)" had to be
measured twice: gonka does serve it, but only at 24% request success (§5.2), so the same weights
were also run through OpenRouter — and the two runs are *not* interchangeable (§2.3).

| arm | provider | model |
|---|---|---|
| **REF31** | openrouter | `google/gemini-3.1-flash-lite` (current production default) |
| **G37** | openrouter | `google/gemini-3.7-flash` |
| **KIMI** | gonka | `moonshotai/Kimi-K2.6` (current gonka default) |
| **DSV4** | gonka | `deepseek-ai/DeepSeek-V4-Flash-0731` |
| **DSOR** | openrouter | `deepseek/deepseek-v4-flash-0731` (same weights, metered) |
| **KREP** | gonka | Kimi-K2.6 **+ the proofread pass** (`--repair`, the gonka production recipe — §7) |

Artifacts (gitignored — verbatim book text): `bench-quality/.artifacts/model-bench-2026-08/`
— the five `.tbook` files, `*-stats.jsonl`, `score-*.json`, glossaries, lexcheck reports, and every
judge batch/key/verdict/notes file, so `analyze_pairs.py` reproduces §3 exactly.

## Verdict

**Quality order on this text (blind pairwise, both presentation orders, sign test):**

```
gemini-3.7-flash  >  gemini-3.1-flash-lite  ≳  DeepSeek-V4-Flash-0731  ≫  Kimi-K2.6
     59.9% (p=0.016)         58.6% (p=0.038)              77.3% (p=2.8e-07)
```

1. **`gemini-3.7-flash` is the new quality leader** — 94:63 of 157 decisive pairs (59.9%, p=0.016)
   over the production default, **replicated across two disjoint rounds** (59.7% and 60.0%). It is
   just as fast and costs ~25% more per book (~$1.39 vs ~$1.11). It could not run on the client at
   all when this bench started (400 `Reasoning is mandatory`); the fallback shipped in `c60bf8a`
   and is verified end to end in §5.1.
2. **`DeepSeek-V4-Flash-0731` is the big finding for gonka**: it beats Kimi-K2.6 **77.3% : 22.7%
   (p=2.8e-07)** and lands statistically level with the metered gemini default (41.4% : 58.6% over
   both DeepSeek runs, p=0.038 — a small real gap, an order of magnitude smaller than Kimi's).
   Kimi-K2.6 is the weakest of the four by a wide margin against every opponent.
3. **But DeepSeek is only theoretically free.** On gonka today it ran at 0.5 sent/s with 656×429
   rejections — 1h07m for 1575 sentences, ~11 h projected per book. Via OpenRouter the same weights
   cost ~$0.23/book (5× cheaper than gemini) and convert in 7m45s — and one 16-sentence batch came
   back **in Chinese**, which neither validation nor lexcheck notices (§2.3).
4. **Alignment is identical across all five arms** (third-decimal differences); the entire spread is
   in the translation text.

5. **The proofread pass (`--repair`) is excellent and still not enough** (§7): on the 4.2% of
   sentences it rewrites it wins **37:0 of 37 decisive pairs** (p=1.5e-11, zero regressions), for
   6m22s of free gonka tokens and no alignment cost. But it narrows Kimi's gap to the gemini default
   only from 75.0% to 68.1% (p=0.0006), and repaired Kimi still trails *un-repaired*
   DeepSeek-V4-Flash 60.4% : 39.6% (p=0.052) — the pass touches 4% of the book, the model decides
   the other 96%.

Practical reading: keep `gemini-3.1-flash-lite` as today's default; adopt `gemini-3.7-flash` for a
"best quality" recipe once the reasoning flag is fixed; **replace Kimi-K2.6 with
DeepSeek-V4-Flash-0731 as the gonka default only when the gateway can actually serve it** — and keep
`--repair` on top of whichever gonka model wins, since it is free and never made anything worse.

## 1. Operational numbers

| arm | translate phase | wall (full pipeline) | requests ok/total | in/out tokens | excerpt cost | book cost¹ |
|---|---|---|---|---|---|---|
| **REF31** | 21s (73.6 sent/s), p50 2.4s | **1m53s** | 118/118 | 111k / 57k | $0.114 | **~$1.11** |
| **G37** | 19s (80.7 sent/s), p50 4.4s | **1m52s** | 114/115 (1×400²) | 120k / 52k | $0.142 | ~$1.39 |
| **KIMI** | degraded (see below) | 13m42s over two runs | 221/1591 (**1370×502**) | 205k / 64k | $0.00 | **$0.00** |
| **DSV4** | 56m10s (**0.5 sent/s**) | **1h07m18s** | 219/928 (**656×429**, 23×502) | 260k / 54k | $0.00 | $0.00 |
| **DSOR** | 3m27s (7.6 sent/s), p50 13.4s | 7m45s | 122/122 | 132k / 57k | $0.024 | ~$0.23 |

¹ linear extrapolation ×9.8 (whole book = 15,431 sentences). ² the 400 is §5.1.

**What matters:**

- **The two geminis are in a class of their own on speed**: the whole excerpt converts in under two
  minutes, translation itself in ~20 s. G37 is marginally faster per sentence than the current
  default and costs 25% more per book (higher list price, but ~10% *fewer* output tokens, so the gap
  is smaller than the price sheet suggests).
- **DeepSeek-V4-Flash is cheap and slow.** Via OpenRouter it is ~5× cheaper per book than gemini
  ($0.23 vs $1.11) but 4× slower wall-clock, and its requests fan out over 16 different providers
  (p90 51.6 s, two 120 s timeouts in the LLM-align tail). Via gonka it is free and unusable today:
  0.5 sent/s, ~11 h projected for a whole book.
- **Both gonka arms fought the network, in two different ways.** KIMI hit a 502 storm
  (`redundancy: no currently-available…`, 1370 of 1591 requests) that left 100 sentences
  untranslated on the first pass; re-running the same command finished them from cache in 3m49s, so
  the final book is complete. DSV4 hit a 429 wall (`too many concurrent requests`) — single
  sequential curl probes were refused too, so it is gateway saturation, not our concurrency.
  Neither number describes the models; both describe gonka on 2026-08-20.

## 2. Structural quality

### 2.1 Alignment and the free gates

| metric | REF31 | G37 | KIMI | DSV4 | DSOR |
|---|---|---|---|---|---|
| validation (empty / offset / struct) | 0/0/0 | 0/0/0 | 0/0/0 | 0/0/0 | 0/0/0 |
| untranslated, empty | 0 | 0 | 0 | 0 | 0 |
| lexcheck flags | **19** | 41 | 29 | 22 | 25 |
| gated to the LLM align pass | 45 (2.9%) | 55 (3.5%) | 47 (3.0%) | **39 (2.5%)** | 41 (2.6%) |
| tgt coverage, all / content words | .988 / .993 | .988 / .992 | .989 / .993 | **.991 / .996** | .991 / .995 |
| src tap coverage, all / content | .971 / .981 | .973 / **.984** | .971 / .980 | .972 / .981 | .969 / .980 |
| sentences with content tap < 0.8 | 29 (2.1%) | 24 (1.7%) | 31 (2.2%) | **23 (1.6%)** | 28 (2.0%) |

**Alignment is a dead heat across all five arms** — they differ in the third decimal, which
reproduces every earlier finding: the local LaBSE aligner does not much care which of these models
wrote the sentence. Note that lexcheck flags are *not* a quality ranking — G37's 41 flags are mostly
its bolder lexical choices, and §2.3 shows what lexcheck misses entirely.

### 2.2 Text profile and glossary

| metric | REF31 | G37 | KIMI | DSV4 | DSOR |
|---|---|---|---|---|---|
| glossary terms built | 20 | 33 | 39 | **72** | 42 |
| mean translation length (chars) | 80.2 | 81.3 | 78.8 | 78.2 | 77.8 |
| type-token ratio | .352 | .359 | .359 | .356 | .360 |
| Latin-script tokens left in the Russian | 0 | 0 | 0 | 4 | 8 |
| sentences in the **wrong script** (CJK) | 0 | 0 | 0 | 0 | **16** |
| dialogue openers (450): guillemets / em-dash / other | 157 / 293 / 0 | 46 / 401 / 3 | 156 / 279 / 15 | 381 / 42 / 27 | 345 / 82 / 23 |

Proper names are the one book-wide consistency defect the glossary does not cover (character names
never make it in). Counting how each arm renders the same name across the excerpt:

| name | REF31 | G37 | KIMI | DSV4 | DSOR |
|---|---|---|---|---|---|
| Khouri | Хури 43 | Хаури 43 | Хури 43 | Хури 43 | **Хоури 28 / Хури 15** |
| Sajaki | Саджаки 12 | **Саяки** 12 | Садзаки 10 / Саджаки 2 | Саджаки 12 | Саджаки 7 / Садзаки 4 |
| Janequin | Жанекен 13 / Жанкен 9 | Жанекен 15 / Жанкен 7 | **Жанекен 22** | Жанкен 7 / Жанекен 6 / Жанакен 4 | Жанкен 11 / Жанекен 11 |
| Calvin | **Кэлвин 26 / Кальвин 9** | Кельвин 45 | Кальвин 45 | Кальвин 45 | Кальвин 44 |

Read as "how often the reader meets a second spelling of the same character": KIMI and G37 are the
most self-consistent, REF31 wobbles on Calvin, and both DeepSeek runs wobble on most names — they
build the biggest glossaries and still do not hold the names steady. G37's *consistent* `Саяки` is
the other failure mode: stable and wrong (the *j* disappears).

### 2.3 The defect that no gate catches: DSOR wrote 16 sentences in Chinese

The OpenRouter DeepSeek arm translated **sentences 420–435 — exactly one 16-sentence batch — into
Chinese**, and one further sentence degenerated into JavaScript tokens mid-word:

```
EN    “No,” Sylveste said, through clenched teeth.
DSOR  “不，”西尔维斯特咬着牙说。

EN    She looked up into a suspended cloud of rusted, damaged sculptures hung on copper cables …
DSOR  Она подняла взгляд на парящее облако ржавых, повреждённых скульптур, подвеimport { Component: (function ( (
```

Five more sentences kept raw English words inside the Russian («стереоскопическое слияние легко
accommodated в его собственных…», «обнажив echoing глубины шахты», «вставить dummy-имплантаты»).

**Both quality gates pass this book as fine**: validation prints
`1575 sentences, 0 empty, 0 offset_errors, 0 struct_errors — OK`, and lexcheck flags **0 of the 16**
Chinese sentences (and 1 of the 5 Latin leaks) — a bilingual dictionary scores what it can look up,
and Chinese text is simply unlookupable, so it yields no evidence either way.

The gonka run of the same weights has **0 CJK sentences**, and the two runs agree verbatim on only
26.2% of sentences, so this is an OpenRouter routing artifact (one host in the 16-provider fan-out),
not an intrinsic property of DeepSeek-V4-Flash. That distinction does not help a reader: with
default routing it is what you get, and nothing in the pipeline says so.

## 3. Blind pairwise judging — the main result

Repo methodology (`prepare_pairs.py` → judges → `analyze_pairs.py`): only sentences where the two
arms actually differ, 200 pairs per comparison, each pair judged in **both presentation orders** by
an independent judge; decisive only when both orders agree, disagreement = tie; sign test,
two-sided. Rubric: fidelity > Russian grammar > idiom/register > naturalness.

New this round: the judges are separate `claude -p --model sonnet` processes driven by
`bench-quality/judge_pairs.py` (one fresh process per batch of 10 pairs, no shared context), which
makes §3 reproducible from the artifacts without hand-orchestrated agents.

| comparison | pairs | ties (incl. order-inconsistent) | decisive | result | p |
|---|---|---|---|---|---|
| **G37 vs REF31**, round 1 (seed 1) | 200 | 115 + 13 | 72 | **g37 43 : 29** (59.7%) | 0.125 |
| **G37 vs REF31**, round 2 (disjoint, `--skip 200`) | 200 | 101 + 14 | 85 | **g37 51 : 34** (60.0%) | 0.082 |
| **G37 vs REF31, pooled** | **400** | — | **157** | **g37 94 : 63 (59.9%)** | **0.016** |
| DSV4 vs REF31 | 200 | 113 + 11 | 76 | ref31 43 : 33 (56.6%) | 0.302 |
| DSOR vs REF31 | 200 | 102 + 17 | 81 | ref31 49 : 32 (60.5%) | 0.075 |
| **DeepSeek (both runs) vs REF31, pooled** | **400** | — | **157** | **ref31 92 : 65 (58.6%)** | **0.038** |
| REF31 vs KIMI | 200 | 104 + 12 | 84 | **ref31 63 : 21 (75.0%)** | 5.0e-06 |
| G37 vs KIMI | 200 | 95 + 12 | 93 | **g37 62 : 31 (66.7%)** | 0.0017 |
| **DSV4 vs KIMI** | 200 | 99 + 13 | 88 | **dsv4 68 : 20 (77.3%)** | **2.8e-07** |

**The ranking is transitive and internally consistent**: G37 > REF31 > DeepSeek ≫ KIMI, with the
Kimi gap (66–77% against every opponent) an order of magnitude larger than any gap between the other
three.

**The replication matters more than the p-values.** [`kimi-vs-gemma4.md`](kimi-vs-gemma4.md) §3.1
found the sign of a close comparison flipping between two disjoint 200-pair rounds (p=0.011 for the
between-round difference) and concluded that ~400 pairs is the minimum for a claim. Here the same
two-round protocol on G37 vs REF31 returned 59.7% and 60.0% — the lead reproduces to within 0.3 pp,
which is what turns a p=0.125 hint into a usable result at p=0.016 pooled.

Note also the **tie rate: 50–57% of pairs** in every comparison. The arms produce byte-identical
text on only 15.3% of sentences (REF31/G37), 11.8% (REF31/KIMI), 9.9% (REF31/DSV4) and 8.6%
(G37/KIMI) — yet half of the sentences where they differ are judged equally good, so a model swap
changes far less of the reading experience than the raw diff suggests.

## 4. Defect profile

Defect class attributed by the judges to the **losing** side of every decisive judgement (two
judgements per pair, so counts are judgements, not pairs). Exposure differs per arm — KIMI appears
in three comparisons, REF31 in four — so compare *within* a table, not across.

### 4.1 Kimi-K2.6 against the field (3 comparisons)

| defect class | KIMI | REF31 | G37 | DSV4 |
|---|---|---|---|---|
| fidelity | 144 | 34 | 52 | 35 |
| naturalness | **105** | 16 | 9 | 10 |
| collocation | 55 | 7 | 8 | 7 |
| idiom-calque | **42** | 2 | 1 | 2 |
| terminology | 41 | 3 | 5 | 8 |
| nonexistent-word | **31** | 2 | 0 | 1 |
| punctuation | 29 | 3 | 2 | 5 |
| grammar-agreement | 22 | 1 | 5 | 3 |
| register | 15 | 2 | 2 | 2 |
| case-government | 13 | 2 | 0 | 1 |
| other | 6 | 0 | 4 | 0 |
| **total** | **503** (≈168 per comparison) | 72 | 88 | 74 |

Kimi's signature defects are exactly the ones the earlier reports named — invented or mangled words,
calqued idioms, and stiff phrasing:

| EN | KIMI | opponent |
|---|---|---|
| …as he moved to the room's **escritoire** | «направлялся к **эскртуару**» (non-word; it is in Kimi's own glossary) | «к бюро в комнате» (REF31) |
| it might **take the edge off** his cockiness | «слегка убавило бы его самоуверенности» | «сбить с него спесь» (G37) |
| Taraschi was **as good as dead** already | «уже **как бы** мёртв» | «уже считай покойник» (REF31) |
| it would take only **one good dustfall** | «**понадобилась бы лишь один** хороший пылепад» (agreement) | «хватило бы всего одного сильного пылепада» (REF31) |
| exotic **particle radiation** | «экзотической **частичной** радиации» | «экзотического корпускулярного излучения» (REF31) |
| the Mixmasters | «**Смесителей**» | «Миксмастеров» (REF31) |

### 4.2 gemini-3.7-flash vs gemini-3.1-flash-lite (pooled, 2 rounds)

| defect class | G37 | REF31 |
|---|---|---|
| fidelity | 100 | 132 |
| naturalness | 33 | 35 |
| **terminology** | 12 | **41** |
| collocation | 17 | 15 |
| idiom-calque | 8 | 12 |
| case-government | 9 | 8 |
| grammar-agreement | 1 | 7 |
| punctuation | 4 | 7 |
| other / register / nonexistent-word | 5 | 10 |
| **total** | **189** | **267** |

The two geminis fail in the same places (fidelity dominates both columns), and G37's edge is
concentrated in **terminology** (41 losses vs 12): where flash-lite reaches for a wrong or flat term,
3.7 more often finds the right one. Both still mangle names, and 3.7 has its own calques — «It was
**bitterly cold** near the reefer» → «было **горько холодно** возле **рефрижератора**» (REF31:
«нестерпимо холодно… рядом с криокапсулой»), plus the stable-but-wrong `Саяки` for Sajaki.

### 4.3 DeepSeek-V4-Flash vs gemini-3.1-flash-lite

| defect class | DSV4 (gonka) | DSOR (openrouter) | REF31 (2 comparisons) |
|---|---|---|---|
| fidelity | 40 | 48 | 116 |
| **naturalness** | **32** | **28** | 22 |
| terminology | 7 | 9 | 23 |
| collocation | 17 | 8 | 12 |
| **punctuation** | 12 | 13 | 7 |
| idiom-calque | 1 | 7 | 16 |
| case-government | 3 | 6 | 4 |
| nonexistent-word | 2 | 4 | 0 |
| other / register / agreement | 7 | 14 | 9 |
| **total** | **121** | **137** | **209** (≈105 each) |

DeepSeek is *more literal* than gemini: it wins on idiom handling («It can't have been in living
memory» → «Это наверняка было ещё до того, как ты себя помнишь» against REF31's calque «Этого не
может быть в живой памяти») and loses on phrasing («Маска тяжёлого дыхания человека из
Администрации», «вытер… о **голенища** своих брюк», «вирусными **оружиями**»). Its one systematic
formatting defect: em dashes glued to the words («своё падение—своё кажущееся падение—включив»),
which the reader sees on every dashed sentence.

## 5. Findings that are not about translation quality

### 5.1 `gemini-3.7-flash` needed a client fix (shipped)

Every OpenRouter request carries `"reasoning":{"enabled":false}` (added for glm-5.2, a no-op for
non-reasoning models). `google/gemini-3.7-flash` rejects it:

```
error: glossary: openrouter 400: Reasoning is mandatory for this endpoint and cannot be disabled.
```

Probes on a one-sentence translation batch:

| request | output tokens | reasoning tokens | cost |
|---|---|---|---|
| `reasoning:{enabled:false}` | — | — | **400** |
| `reasoning:{max_tokens:0}` | — | — | **400** |
| no `reasoning` field | 653 | **635** | $0.00123 |
| `reasoning:{effort:"minimal"}` | 21 | 0 | $0.000048 |
| `reasoning:{effort:"low"}` | 18 | 0 | $0.000042 |

So the model is usable, but only via the lowest effort tier: dropping the field entirely costs
**26× more** on the same sentence (635 reasoning tokens the pipeline never reads). The arm in this
report ran with `effort:"minimal"` through a locally patched client — 51.5k output tokens for the
excerpt against 57.3k for flash-lite, i.e. no reasoning inflation at all.

**Shipped fix** (`c60bf8a`): the client keeps sending `enabled:false`, and when a 400 says reasoning
is mandatory it retries that request at `{"effort":"minimal"}` and latches the choice for the rest of
the run, so the hundreds of concurrent batches that follow never pay for the rejected shape. Nothing
to configure.

Verified end to end after the fix, chapter ONE (470 sentences) with `--model google/gemini-3.7-flash`
on a fresh cache: **exactly one rejected request** (the glossary call, `400`), then 34/34 requests
`200`; 470 sentences, 0 empty, 0 offset/struct errors, coverage 98%/97%, 12.5k output tokens for
$0.043 — no reasoning inflation, no manual flag.

### 5.2 gonka on 2026-08-20: two different network failures

- `moonshotai/Kimi-K2.6`: **1370 of 1591 requests → 502** `redundancy: no currently-available…`.
  The first run left 100 of 1575 sentences untranslated (`Validation: … 100 empty`); re-running the
  same command finished them from cache in 3m49s. The resumable cache is what saved this arm.
- `deepseek-ai/DeepSeek-V4-Flash-0731`: **656 requests → 429** `too many concurrent requests`, even
  for a single sequential probe. Translate ran at 0.5 sent/s (24% of requests succeeded) and the
  excerpt took 1h07m — **~11 h projected for a whole book**. The arm only completed because the run
  was wrapped in an outer retry loop.
- The converter's retry budget is too small for this: gonka answers 429 with a short `Retry-After`,
  which `backoff()` honours, so the default 8 gonka retries are spent in ~19 s and the run dies on
  the glossary call. This bench needed `--max-retries 12` plus an outer loop.

### 5.3 Harness additions (this report's tooling)

- `bench-quality/judge_pairs.py` — runs the blind pairwise judging over `prepare_pairs.py` batches
  with N parallel `claude -p` processes; writes `verdict-*.json` (read by `analyze_pairs.py`) and
  `notes-*.json` (the loser's defect class, read by `analyze_notes.py`). Re-running fills only the
  gaps. Judge calls are burst-rate-limited — `--jobs 5` and the 30 s backoff between attempts exist
  because a burst of 8 lost 80 batches in one minute.
- `bench-quality/analyze_notes.py` — the defect-class table of §4.
- `bench-quality/stats_summary.py` — per-phase requests/errors/tokens/cost/latency from a `--stats`
  NDJSON log (§1).
- `bench-quality/tbook_profile.py` — text profile of a `.tbook`: length, TTR, script leakage,
  dialogue punctuation, per-name rendering counts (§2.2, §2.3).
- `prepare_pairs.py --skip N` — a second round with the same seed that is *disjoint* from the first
  (round 2 of §3 used `--seed 1 --skip 200`; overlap verified to be 0 pairs).

## 6. Recommendations

1. **Take `gemini-3.7-flash` as the quality option** — the reasoning fallback it needs has shipped
   (§5.1), so it is now a one-word config change. It is the only measured way to buy ~60:40 better
   translations at unchanged speed; the bill goes from ~$1.11 to ~$1.39 per book. Named in
   `.env.example` and in the README's metered-provider section.
2. **Change the gonka default from Kimi-K2.6 to DeepSeek-V4-Flash-0731 — but gate it on
   availability.** 77:23 on blind pairs is the largest quality gap in this bench, and it comes for
   free; 0.5 sent/s and 24% request success is not shippable. Probe the endpoint before committing a
   book to it (a single request suffices — it either 429s or it does not), and keep Kimi as the
   fallback.
3. **Keep `--repair` on for gonka, but do not expect it to fix the model choice** (§7). It is free,
   alignment-neutral and it made *nothing* worse in 37 decisive judgements — take it. It just cannot
   carry Kimi: after the pass Kimi still loses 68.1% to the gemini default and 60.4% to un-repaired
   DeepSeek. Switching the model beats adding a pass; do both.
   Two bugs to fix before relying on the pass: the pending-namespace bug (§7.4.1, patch included) and
   the silent freeze of unproofread sentences on a flaky gateway (§7.4.2).
4. **Spell-check glossary targets against the lexcheck dictionary** before enforcing them: Kimi's
   `escritoire → эскртуар` typo is enforced on all 9 occurrences and is immune to the proofread pass
   (§7.3). The lexicons are already on disk and the check is one lookup per term.
5. **Add a target-script check to validation** (§2.3): count sentences whose translation carries no
   character of the target script (or a majority of another). It is free, and it catches the one
   failure both existing gates are structurally blind to — a whole batch answered in the wrong
   language. Also worth flagging: Latin-script runs inside a Cyrillic target.
6. **If DeepSeek is used through OpenRouter, pin the provider** (`--provider-order`). The default
   fan-out reached 16 providers, produced the Chinese batch, the JS-token degeneration and two 120 s
   align timeouts. At ~$0.23 per book it is otherwise the cheapest metered option in the bench and
   statistically level with the current default.
7. **Keep using the two-round protocol for any comparison under ~65:35** (§3). One round of 200
   pairs left the headline result at p=0.125; the disjoint replication is what made it citable.

## 7. Kimi-K2.6 with the proofread pass (`--repair`, the gonka production recipe)

Measured on the same excerpt: the raw translations of the KIMI arm were reused verbatim (cache copy
+ the same glossary sidecar, so the namespace hash does not move and nothing is re-translated), and
`convert --provider gonka --repair` ran the pass and re-aligned. Arm **KREP**.

### 7.1 Cost and structure

| | KIMI (raw) | **KREP (+ proofread)** |
|---|---|---|
| proofread phase | — | **6m22s (4.1 sent/s)**, 210 requests, 198 ok (6×429, 6×502) |
| pass tokens (free) | — | 273k in / 57k out |
| wall for this run¹ | — | 8m18s (proofread + embalign 1m02s + LLM align 42s) |
| sentences changed | — | **66 / 1566 = 4.2%** |
| validation (empty/offset/struct) | 0/0/0 | 0/0/0 |
| lexcheck flags | 29 | 28 |
| tgt coverage all / content | .989 / .993 | .989 / .994 |
| src tap coverage all / content | .971 / .980 | .970 / .980 |
| sentences with content tap < 0.8 | 31 | 29 |
| mean translation length | 79.2 chars | 79.2 chars |

¹ the translate phase came from cache; the full recipe is translate + these 8m18s.

**The pass is alignment-neutral and free** — same coverage to the third decimal, same lexcheck level.
The 4.2% edit rate matches the July measurement (3.9–4.8% in `LOG.md`) and **contradicts the 16.3%
reported in [`kimi-vs-gemma4.md`](kimi-vs-gemma4.md) §5** on this very book; that figure came from a
run in a 429 storm and should be treated as an artifact.

### 7.2 Are the edits good? — 37:0

Blind pairwise on **only the changed sentences** (62 of the 66 have ≥6 source words), both
presentation orders:

| comparison | pairs | ties (incl. inconsistent) | decisive | result | p |
|---|---|---|---|---|---|
| **KREP vs KIMI, changed sentences only** | 62 | 18 + 7 | 37 | **KREP 37 : 0 (100%)** | **1.5e-11** |

Zero regressions in 37 decisive judgements — the highest precision any arm of this pass has shown
(July: 87.1%). Defect classes the judges attributed to the raw text it replaced: grammar-agreement
20, fidelity 17, punctuation 16, case-government 7, idiom-calque 6, collocation 6, non-words 2.

What that looks like:

| EN | KIMI (raw) | KREP (proofread) |
|---|---|---|
| She wore a **greatcoat** | «На ней **был** шинель» | «На ней **была** шинель» |
| it would take only **one good dustfall** | «**понадобилась** бы лишь один хороший пылепад» | «**понадобился** бы…» |
| The shit's about to **match coordinates with the fan** | «Дерьмо вот-вот **сойдётся в координатах с** вентилятором» | «Дерьмо вот-вот **врежется в** вентилятор» |
| muster what little **support** you may have left | «собрать **те немногие сторонники**» | «собрать **тех немногих сторонников**» |
| on approximately one thousand previous **occasions** | «**на тысяче предыдущих случаях**» | «**в тысяче предыдущих случаев**» |
| You've just pushed yours **over the precipice** | «Вы только что **испытали свою за пропастью**» (nonsense) | «Вы только что **перешли черту**» |
| two **stone-lined** burial chambers | «две **каменные** погребальные камеры» | «две **выложенные камнем** погребальные камеры» |
| Sylveste was increasingly thinking of as **her mob** | «которую Сильвест всё чаще **думал о ней как о её толпе**» | «…всё чаще **считал её толпой**» |

### 7.3 Does it close the model gap? — no

| comparison | pairs | decisive | result | p |
|---|---|---|---|---|
| KIMI vs REF31 (from §3, for reference) | 200 | 84 | ref31 63 : 21 (75.0%) | 5.0e-06 |
| **KREP vs REF31** | 200 | 94 | **ref31 64 : 30 (68.1%)** | 0.0006 |
| **KREP vs DSV4** (un-repaired DeepSeek) | 200 | 96 | **dsv4 58 : 38 (60.4%)** | 0.052 |

The arithmetic is unforgiving: the pass rewrites 4.2% of sentences, so it can move a whole-book
comparison by at most that much. Measured: Kimi's deficit against the gemini default shrinks from
75.0% to 68.1% of decisive pairs, and **repaired Kimi still loses to plain DeepSeek-V4-Flash**
(60.4%, p=0.052). Of the 200 pairs judged in §3 for KIMI vs REF31, only **9** sit on sentences this
pass actually touched.

Residual defects after the pass (judged against REF31): **non-existent words 22**, terminology 18,
idiom-calque 12 — i.e. exactly the class the pass is supposed to fix, still there. One reason is
structural: **the glossary guard protects Kimi's own typos.** Kimi's glossary contains
`escritoire → эскртуар`, the proofread prompt is forbidden to touch enforced glossary terms, and all
**9** occurrences survive verbatim. That is the strongest argument yet for the README's advice to run
`--only-glossary` and fix the glossary before translating — and for spell-checking glossary targets
against the lexcheck dictionary, which would have caught this one for free.

### 7.4 Two operational traps found while measuring this

1. **`--repair` over an existing cache silently produced an empty book** — the bug
   [`kimi-vs-gemma4.md`](kimi-vs-gemma4.md) §6 reported on 2026-08-08 is still present at HEAD
   `b0ee45d`: `countPending()` (`cmd/convert/main.go:246`) counts the **raw** namespace while
   assembly reads the **proofread** one, so adding the pass to an already-translated book prints
   "All sentences already cached — assembling offline" and writes 1575 empty translations, exit 0.
   **Fixed in the repo** after this bench: `cmd/convert/main.go` now counts through
   `pendingFinal`/`pendingRepair`/`pendingText` (the namespace assembly reads from, and the
   proofread text for the `--align-mode emb` branch), backed by `translate.CountPendingRepair`
   and the regression tests in `cmd/convert/pending_test.go`. A `--repair` run over an
   un-proofread cache now correctly reports `0 to translate, 1566 to proofread, 0 to align`,
   and the banner names the phase that actually needs the LLM instead of always saying
   "Translating".
2. **A flaky gateway silently ships the book unproofread.** The first attempt ran during a 429/502
   storm: 56 of 2722 repair requests succeeded, and `freezeUnrepaired` wrote the **raw** text as the
   proofread text for **1118 of 1566 sentences** — by design ("freezing keeps every later run
   consistent"), but that means **a re-run never retries them**: the cache now says those sentences
   are proofread. The run exits 0; only the line `1118 sentences kept their unproofread text`
   betrays it, and the align phase additionally lost 36 sentences to 429s (they ship with no
   highlights, and lexcheck then flags 8 instead of 29 — 19 of the 21 "vanished" flags are just
   unscoreable unaligned sentences, not repaired ones).
   A complete arm needed **6 health-gated attempts**, each starting from a fresh copy of the raw
   cache. Worth having in the tool: refuse to freeze (or write frozen sentences to a retry list)
   when the failure was a transport error rather than a dropped id.

## 8. Postscript: the multi-language glossary leak

Found afterwards, in a book produced with the same gonka/Kimi recipe on
2026-08-02 (`19.tbook`, "The Wonderful Wizard of Oz", 2143 sentences,
`en → ru,fr,pt,tr,uk,es,de,it`, `glossaryTerms: 38`, proofread pass on). It is
not a model defect — the converter did it — but it is the sharpest illustration
of §2.3's point that the pipeline cannot see a wrong-language answer.

### 8.1 What the file contains

| target | sentences carrying Cyrillic | fully Cyrillic | clean |
|---|---|---|---|
| de | **1565 (73.0%)** | 1161 (54.2%) | 27.0% |
| tr | 1453 (67.8%) | 1161 (54.2%) | 32.2% |
| pt | 1333 (62.2%) | 755 (35.2%) | 37.8% |
| fr | 1197 (55.9%) | 441 (20.6%) | 44.1% |
| es | 1091 (50.9%) | 132 (6.2%) | 49.1% |
| it | 1069 (49.9%) | 300 (14.0%) | 50.1% |

The book title is Russian in all six, byte-identical and with an identical
alignment. Two populations underneath: sentences that are wholly Russian, and —
the majority — correct target prose with Russian proper nouns and terms wedged
in:

```
EN    Uncle Henry and Aunt Em had a big bed in one corner, and Dorothy a little bed in another.
DE    Дядя Генри und тётя Эм hatten ein großes Bett in einer Ecke, und Дороти ein kleines Bett …
ES    Дороти vivía en medio de las grandes praderas de Канзас, con дядя Генри, y тётя Эм …
```

The most frequent Cyrillic tokens in each Latin target are the same handful —
`Дороти`, `Пугало`, `Дровосек`, `Оз`, `Тотошка` — i.e. the book's *glossary*.

### 8.2 Cause: one glossary, eight languages

`cmd/convert/main.go` built exactly one glossary, scoped to `cfg.Targets[0]`
(here `ru`), and `translate.Pipeline.Glossary` — a single list — was injected
into the translate prompt *and* the proofread prompt for every target. A glossary
is a list of source→**target** terms, so targets 2..8 were handed Russian
renderings and instructed to use them; Kimi obliged, and often kept going in
Russian for the rest of the sentence. The proofread pass then protected the
result, since its prompt forbids touching enforced glossary terms.

Reproduced at HEAD with the production default model (`gemini-3.1-flash-lite`,
one chapter, `-t ru,de`): **17.2% of German sentences carried Russian**
(«Sylveste stand am Rand der археологические раскопки»), from an en→ru glossary
of 36 terms. A cheaper model is not the requirement — the leak is structural.

### 8.3 Why every gate passed it

- **the alignment `q`** (0.97 mean) scores how well the words line up with
  whatever text arrived, and the Russian text aligns to the English fine;
- **validation** checks structure and offsets: `0 empty, 0 offset_errors, 0 struct_errors — OK`;
- **lexcheck** ran on `cfg.Targets[0]` only — it examined `ru`, the one healthy
  language, and reported the book clean;
- **`--invalidate`**, the documented repair flow, computed cache keys from the
  plain model id while every glossary-on book (the default since July) stores
  them under `model+g:<hash>`: measured **0 of 1869 cache files deleted** for 20
  real sentences. The repair step was a no-op long before anyone needed it.

### 8.4 Fixed, and what now catches it

`45f7f98` gives every target its own glossary, sidecar and cache namespace (the
pipeline, the fill, the escalation and the judge all run per target), makes
lexcheck check every target, and makes `--invalidate` clear the namespaces the
run actually reads — raw text, proofread text and alignment — per target,
accepting a `{language: [sentences]}` report. The two-target reproduction goes
from **17.2% → 0.0%** Russian in German, with the de glossary now German
(`Event → Ereignis`, `Mercator maps → Mercator-Karten`).

`c17de79` adds `internal/langcheck`, which runs on every conversion and writes
`<out>.langflagged.json` (per language, ready for `--invalidate`): words in a
script neither language uses, the source copied through, and one text served for
two targets. Over the broken book it flags:

| target | flagged | breakdown |
|---|---|---|
| de | 2036 (95.0%) | foreign-script 1548, duplicate-target 485, untranslated 3 |
| tr | 1829 (85.3%) | foreign-script 1436, duplicate-target 390, untranslated 3 |
| pt | 1625 (75.8%) | foreign-script 1320, duplicate-target 305 |
| it | 1237 (57.7%) | foreign-script 1060, duplicate-target 174 |
| fr | 1352 (63.1%) | foreign-script 1186, duplicate-target 166 |
| es | 1163 (54.3%) | foreign-script 1086, duplicate-target 74, untranslated 3 |
| uk | 69 (3.2%) | duplicate-target 69 — same script as ru, invisible to every other signal |
| ru | 0 | the language the answers were actually generated for |

**Repairing the shipped book** takes the two steps the README now documents:
`convert 19.tbook --invalidate 19.tbook.langflagged.json` and a re-run — with the
per-target glossaries in place, only the flagged (sentence, language) pairs are
redone.
