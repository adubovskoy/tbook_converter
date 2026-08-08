# Kimi-K2.6 (gonka) vs Gemma-4-26B-A4B (OpenRouter) — en→ru, excerpt from a real book

Date: 2026-08-08. HEAD `b267352`. Dev set: **Alastair Reynolds, "Revelation Space", chapters ONE+TWO**
(`--limit-chapters 2`) = 1575 sentences / ~21,245 words, en→ru.
Run artifacts: scratchpad `run/` (`rs-kimi.tbook`, `rs-gemma.tbook`, `rs-gemini.tbook`,
`rs-kimi-repair.tbook`), pairs and verdicts — `run/pairs-*`.

## Verdict

**Kimi-K2.6 and Gemma-4-26B-A4B are indistinguishable in translation quality on this text.**
Pooled over 420 blind pairs: **96 : 100 of 196 decisive (49.0% : 51.0%), p = 0.83.**
Parity also holds for the full gonka production recipe (Kimi + repair pass): 33 : 34, p = 1.0.
Both models are noticeably behind the current production default `google/gemini-3.1-flash-lite`.

Practical takeaway: Gemma-4-26B-A4B is Kimi-level quality for $0.29 per book against $1.08 for gemini,
but **not** a replacement for gemini if quality is the priority. Gonka remains the free option at the
same level as Gemma.

## 1. Operational numbers (same excerpt, same binary)

| arm | provider / model | translate | wall (full pipeline) | requests | 429 | in/out tokens | excerpt cost | book cost¹ |
|---|---|---|---|---|---|---|---|---|
| **K** | gonka / `moonshotai/Kimi-K2.6` | 1m38s (16.0 sent/s) | **4m08s** | 223 | 0 | 216k / 66k | **$0.00** | **$0.00** |
| **G4** | openrouter / `google/gemma-4-26b-a4b-it` | 1m50s (14.2 sent/s) | 7m45s | 119 | 0 | 105k / 48k | $0.0295 | ~$0.29 |
| **REF** | openrouter / `google/gemini-3.1-flash-lite` | **12s (132.8 sent/s)** | **1m33s** | 112 | 0 | 106k / 56k | $0.110 | ~$1.08 |
| **K+rep** | gonka / Kimi-K2.6 + `--repair` | (gateway cache) | 15m20s | 1291 | **832** | 524k / 131k | $0.00 | $0.00 |

¹ linear extrapolation ×9.8 (whole book = 15,431 sentences).

**What matters:**
- **Gemma's translation speed is comparable to Kimi's** (14.2 vs 16.0 sent/s), but **gemini is 9× faster**
  (132.8 sent/s, request p50 2.5s vs 13.7s for Gemma).
- **Gemma's bottleneck is not translation but the LLM alignment phase**: 43 sentences in **4m22s (0.2 sent/s)**,
  two requests hit the 120s timeout and went to retries. The same phase on Kimi — 50 sentences in 50s,
  on gemini — 48 in 7s. That is exactly what inflates Gemma's wall to 7m45s.
- **The sentence loss noted in `.env` for `gemma-4-31b` ("dropped ~9%") does NOT happen with `gemma-4-26b-a4b`**:
  98/98 requests 200, 0 errors, 0 empties. The model holds JSON mode.
- Kimi on gonka ran the first pass without a single error; **the `--repair` run hit a 429 storm**
  (832 of 1291 requests) — a transient of the gonka network, but it cost 16 lost sentences.

## 2. Structural quality and alignment — no difference

| metric | K (Kimi) | G4 (Gemma) | REF (gemini) |
|---|---|---|---|
| validation (empty / offset / struct) | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 |
| untranslated, empty | 0 | 0 | 0 |
| tgt content coverage (mean) | 0.994 | **0.995** | 0.992 |
| src tap content coverage (mean) | 0.980 | **0.983** | 0.979 |
| sentences with tap coverage < 0.8 | 30 (2.1%) | **23 (1.6%)** | 32 (2.3%) |
| lexcheck flags | 26 | 29 | 25 |
| glossary terms | **40** | 20 | 19 |
| Latin script in the translation | 0 | 4 | 0 |

Alignment across the three arms is statistically indistinguishable — **the difference between the models
is entirely in the translation text, not in tap quality**. The one objective asymmetry: **Kimi builds a
twice-as-rich glossary** (40 vs 20 terms) and catches the book's multi-word neologisms
(`Chasm City` → «Город Пропасти», `Monument to the Eighty`, `Shrouders`, `Ultras`, `Conjoiners`),
which Gemma does not pick out at all. The counter-example: Kimi writes `escritoire` → «эскртуар»
(typo) in the glossary, Gemma the correct «эскритуар».

Stylometry nearly coincides: mean translation length 79.6 / 81.6 / 80.6 characters, TTR 0.361 / 0.349 / 0.352.
The only divergence is **dialogue formatting**: of the 444 sentences where the source opens a quote,
Gemma is more consistent (86% guillemets), Kimi mixes styles (65% em dash / 31% guillemets).

## 3. Blind pairwise judging — the main result

Repo methodology (`prepare_pairs.py` → judges → `analyze_pairs.py`): each pair is judged
**in both presentation orders** by independent agents (Sonnet); a pair counts as decisive only
if both orders agree; disagreement = tie. Rubric: fidelity > Russian grammar >
idiom/register > naturalness. Sign test, two-sided.

### 3.1 Kimi vs Gemma

| round | pairs | ties (incl. disagreements) | decisive | Kimi | Gemma | p |
|---|---|---|---|---|---|---|
| 1 (seed 1) | 200 | 98 + 12 | 90 | **53 (58.9%)** | 37 (41.1%) | 0.11 |
| 2 (seed 11, disjoint) | 220 | 97 + 17 | 106 | 43 (40.6%) | **63 (59.4%)** | 0.064 |
| **pooled** | **420** | — | **196** | **96 (49.0%)** | **100 (51.0%)** | **0.83** |

**Key observation: the second round flipped the sign.** The difference between the rounds is itself
significant (z = 2.56, p = 0.011), even though the rounds are comparable in difficulty (mean source
length 17.1 vs 17.0 words) and drawn from the same pool. That is, **the noise in the judge signal at
this scale is larger than the difference being measured between the models** — and a single round of
200 pairs is not enough to claim anything about the Kimi/Gemma pair. Both readings lead to the same
practical conclusion: **there is no significant quality difference**.

### 3.2 Anchor: both against the production default

| comparison | pairs | decisive | winner | p |
|---|---|---|---|---|
| gemini-3.1-flash-lite vs **Gemma** | 120 | 52 | **gemini 34 : 18 (65.4%)** | **0.036** |
| gemini-3.1-flash-lite vs **Kimi** | 120 | 56 | **gemini 35 : 21 (62.5%)** | 0.081 |

The anchors are consistent and transitive: gemini beats both by about the same margin, Kimi ≈ Gemma.
This reproduces the July result (gemini 67% over Kimi on Altered Carbon) on a different text
and a different HEAD.

### 3.3 The gonka production recipe (Kimi + `--repair`) against Gemma

| comparison | pairs | ties | decisive | Kimi+repair | Gemma | p |
|---|---|---|---|---|---|---|
| Kimi + proofread vs Gemma | 140 | 71 + 2 | 67 | 33 (49.3%) | 34 (50.7%) | **1.00** |

**The free gonka repair pass does not put Kimi ahead of Gemma** — parity holds
(33:34 at p=1.0). So the choice between the two providers does not change under the full
production recipe either.

## 4. Defect profile (what exactly breaks)

Defect classes attributed to the **losing** arm in decisive pairs, pooled over both rounds:

| defect class | Kimi loses | Gemma loses |
|---|---|---|
| fidelity | **82** | 65 |
| **nonexistent-word** | **19** | 4 |
| collocation | 20 | 16 |
| naturalness | 17 | **27** |
| **case-government** | 5 | **17** |
| other (punctuation, verb tense) | 10 | **18** |
| grammar-agreement | 15 | 15 |
| terminology | 15 | 10 |
| idiom-calque | 13 | 9 |
| register | 3 | 4 |
| **total** | 199 | 185 |

**The error signatures differ; the total level does not.**

**Kimi — invented and mangled words (×5 vs Gemma):**

| EN | Kimi | Gemma |
|---|---|---|
| The apparition was more realistic… | «**Аппарицция** была реалистичнее…» | «Призрак был реалистичнее…» |
| …since my revival. | «…с момента моего **ревая**» | «…с момента моего оживления» |
| sprouting bulbous protrusions | «пуская **бульбовые** наросты» | «порождая вздутые выступы» |
| imaging gravitometers | «гравиметрических **изображателей**» | «гравитометров визуализации» |
| Let Nagorny spend… in reefersleep | «**Нагорни**… в **риферсоне**» | «Нагорный… в риферслипе» |
| He was ill himself now; sick in the head. | «болен **головой**» (calque) | «болен рассудком» |
| Sudjic might be a problem… she and Nagorny | «Суджик… **мог** стать… **она**…» | «Суджик… могла стать… она…» |

**Gemma — Russian morphology and translationese officialese:**

| EN | Gemma | Kimi |
|---|---|---|
| Sylveste had no need for… | «**У Сильвесте** не было нужды» (genitive «Сильвеста» required) | «Сильвесту не требовались» |
| Sylveste was eight years into his third century | «**Сильвесте шел** восьмой год» (dative required) | «Сильвест находился на восьмом году» |
| its lower levels barnacled in slums | «нижние уровни **были обросли**» | «нижние ярусы обросли» |
| computers had cracked the Amarantin language | «**язык Амарантин** взломали» (not declined) | «взломали язык амарантинов» |
| “Oh,” Janequin said | «**сказала** Жанекен» (the character is male) | «сказал Жанекен» |
| Cal said | «сказал **Кал**» | «сказал Кэл» |
| “No,” Sylveste said | «сказал **Сильвестр**» (a different name) | «сказал Сильвест» |
| It was bitterly cold | «было **горько холодно**» (calque) | «было пронзительно холодно» |
| Now she had to steel herself to act. | «**мобилизовать свои силы**» | «собраться с духом» |
| Without answering he walked past her, towards noise. | «**по направлению к** шуму» | «к шуму» |

Both models mangle proper names, but differently: Kimi — «Нагорни», «Садзаки»;
Gemma — «Жирдо», «Сильвестр», «Кал». For a book where the names recur hundreds of times
this is more noticeable than any stylistic difference, and **the glossary does not cure it**
(character names never make it into the glossary).

## 5. Repair (proofread) pass (`--repair`) on Kimi

The gonka production recipe enables the proofread pass by default. On this excerpt it **changed
253 of 1550 sentences (16.3%)** — against the ~4% documented in `LOG.md` on Altered Carbon.
The edits are correct in character: «На ней **был** шинель» → «была», «Миллион лет… **прижималось**» →
«прижимался», restoring a dropped "stood up" («вздрогнула» → «вздрогнув, встала»),
«тишина опустилась на комнату» → «в комнате повисла тишина».

The 16.3% vs 4% divergence is worth checking separately: either Reynolds's text simply offers more
occasions, or the `--repair` built into `convert` is bolder than `cmd/repair --fluent`, on which
July's measurement was made.

⚠️ The run hit a 429 storm (832 rejections) and **lost 16 sentences** — at a 15m20s wall this
makes the repair recipe on gonka noticeably less predictable than the single-pass one.

## 6. Bug found: `--repair` on top of an un-proofread cache silently produces an empty book

`countPending()` (`cmd/convert/main.go:246`) checks the **raw** cache namespace
(`cacheModel`), whereas with `--repair` assembly reads from the **proofread** one
(`finalModel` = `translate.RepairCacheModel(...)`, same file, lines 796–812).

Reproduction (both commands exited 0):

```
# 1) ordinary run without repair — fills the cache with raw translations
convert book.epub --provider gonka --no-repair --cache-dir C -o a.tbook   # OK

# 2) same cache + --repair
convert book.epub --provider gonka --repair   --cache-dir C -o b.tbook
#   All sentences already cached — assembling offline (no API calls).
#   Filled 0 translations from cache (1575 missing).
#   Validation: 1575 sentences, 1575 empty, ... — OK (some sentences untranslated)
#   Coverage: 100% of target words aligned, 100% of source words highlighted
```

The repair pass never runs at all; the output is a 115 KB .tbook with 100% empty translations,
yet "OK" and "Coverage: 100%" are printed (the low-coverage warning is suppressed by the
`rep.Empty == 0` branch in `main.go:505`). The scenario is a real one: "translated the book →
decided to add the proofread pass". The fix is to count pending in the same namespace that
assembly later reads from, i.e. `finalModel(cfg, cacheModel)` (and `repairModel` for the text),
not `cacheModel`.

## 7. Recommendations

1. **There is no case for replacing gemini-3.1-flash-lite with Gemma-4-26B-A4B for quality**: gemini
   wins 65:35 (p=0.036) and on top of that is 9× faster. Gemma's point is price: ~$0.29 vs ~$1.08
   per book.
2. **Gemma vs Kimi — parity on quality.** The choice between them is dictated not by quality but by
   the fact that gonka is free but unstable (429 storm), while Gemma costs $0.29 and is stable on
   translation but very slow on LLM alignment.
3. If Gemma is taken into production anyway — **raise `REQUEST_TIMEOUT_SEC` for the align phase or run
   it with `--align-mode emb`**: 0.2 sent/s on alignment means ~40 minutes per book for the tail alone.
4. **The proofread pass is worth aiming at Gemma, not only at Kimi**: Gemma's defect profile
   (case government 17, naturalness 27, punctuation/tense 18) is exactly what this pass
   knows how to fix, whereas Kimi's signature defect (invented words) it catches less well.
5. **Do not draw conclusions from a single round of 200 pairs.** Between two disjoint rounds the
   sign flipped at p=0.011 — the minimum reliable volume for a pair this close
   is ≈ 400+ pairs in both orders.
