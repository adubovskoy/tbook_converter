# Glossary: how large it can grow, and carrying gender in it

Research report, 2026-08-25. Book: *Altered Carbon*, en→ru, 14,688 sentences
(`bench-quality/full-repair.tbook` as the reference translation). Probe model:
`google/gemini-3.1-flash-lite` (the production default), temperature 0.3, batch 16,
`response_format: json_object`, `reasoning: {enabled:false}` — the same request the
production translate pass sends. Total API spend for every arm in this report: **$0.59**.

Two questions:

1. **How many entries can the glossary hold** before adherence, cost, or latency break?
2. **Can the glossary carry the gender of a character** so that verbs and adjectives agree
   in languages that mark it?

Both are answered with measurements. The short version:

| question | answer |
|---|---|
| adherence ceiling | **none found up to 1,392 entries** — 98.6% full adherence at 244 entries and 98.6% at 1,392 (noise floor ±0.2 pp) |
| what a book actually needs | **~200–300 entries** for a 14.7k-sentence novel, against a cap of 40 today — and raising the cap alone does nothing: the sample-based builder returns 20–31 entries whatever it is allowed (§2.4) |
| what the cap costs today | 6 major characters (Elliott ×147, Jerry ×74, Irene ×53, Sarah, Milo, Quell) are unglossed and drift — «Айрин»/«Ирин»/«Ирен»/«Ирэн» for one woman in one book |
| price of growing | +$0.33 per book at 244 entries, +$1.86 at 1,392 (translate pass, this model); latency p50 within noise at 244, +0.3 s at 1,392 |
| what does limit size | local-provider context windows, cache-namespace churn, and glossary *quality* — not the model's ability to follow it |
| gender in the glossary | **works, and is worth a lot**: on text the model cannot recognise, gender agreement goes from **45.2% → 97.6%** (fixed 32, broke 1, sign test p = 7.9e-09) |
| replication | every finding above reproduced on *Revelation Space* (§8), which also exposed a render-pass defect: 12 of 128 entries glued a rendering to the wrong term until the reply was made to echo it |
| gender on a *famous* book | no measurable gain — a cast list of real names lets the model identify the novel and recall the genders itself (100% either way). The tag matters for books it does not know, which is most books |

---

## 1. Method

`bench-quality/gloss_scale_probe.py` speaks the production contract: its system-prompt
template is a copy of `translateSystemPrompt` in `internal/translate/prompt.go`, and
`--check` diffs the rendering against a Go dump of that function (identical, both with and
without a glossary block). Arms differ only in the glossary passed in.

Probe sets (all in `bench-quality/.artifacts/glossary-scale/`):

- `probe.json` — 465 sentences: 315 chosen greedily so that each of 238 mined terms gets up
  to 2 sentences (rare terms deliberately included), plus **150 control sentences containing
  no glossary term at all**, to measure collateral damage on text the glossary has no
  business touching.
- `probe-gender.json` — 98 sentences where a known-gender character is the subject of a
  past-tense verb **and the source never marks the gender** (no he/she/his/her anywhere in
  the sentence). Exactly one past-tense source verb, so the Russian form under test is
  unambiguous.
- `probe-deid.json` — the same 98 sentences with all 60 character names replaced by invented
  surnames (`bench-quality/gender_deid.py`). See §4.3 for why this is necessary.

Two independent scorers, which agree within ~5 pp everywhere: a deterministic Russian
past-tense classifier (`gender_probe.py`, free, reproducible) and a one-question LLM judge
(`gender_judge.py`, also used for Spanish where no regex can do the job). Judge numbers are
quoted as the primary figures; the regex figures are systematically ~5 pp lower because of
its own false positives («стола» is a noun, «Сначала» an adverb, «Шерил» a name that ends
in -ил).

---

## 2. Demand: how many terms does a book have?

`bench-quality/glossary_demand.py` mines the source side and then reads back, through the
stored word alignment, which target rendering every occurrence actually got.

Capitalised terms appearing mid-sentence at least twice: **382**, of which 195 are never
written lowercase (proper nouns). Their occurrence mass:

| cut | share of all term occurrences | min frequency in the cut |
|---|---|---|
| top 40 (**today's cap**) | 61.9% | 28 |
| top 100 | 79.8% | 11 |
| top 200 | 91.8% | 4 |
| top 300 | 97.2% | 2 |

Terms with frequency ≥ 10: **114**. With ≥ 5: 196.

Adding the invented terminology a bilingual lexicon does not know (`glossary_mine.py`,
frequency ≥ 4, filtered through `lexicons/en-ru.tsv.gz` plus light de-inflection) gives
**312 candidates**; the model keeps **189–201** of them when asked to filter and render
(§2.3). That is the natural size of this book's glossary: roughly **200**, against a cap of
40.

### 2.1 What the missing entries cost

Unglossed recurring names drift, and the drift is exactly what a tap-to-translate reader
sees:

```
Irene    freq  53   «айрин»×25, «ирин»×13, «ирен»×8, «ирэн»×7
Begin    freq  34   «бегин»×25, «бежан»×9
Quell    freq  10   «квелл»×7, «куэлл»×3
Milo     freq  10   «мило»×7, «майло»×3
Rawling  freq  12   «роулинга»×6, «ролинг»×6
JacSol   freq  19   «джаксол»×8, «джексол»×7, «jacsol»×3
Meth     freq  27   «мета»×14, «мет»×12
Suntouch freq  26   «солнечного»×14 + «касания»×12 (translated), «сантач»×10 (transliterated)
```

Aggregate off-dominant rate for proper nouns is 2.9% both for glossed and unglossed terms —
the *rate* does not separate them, because the model is fairly self-consistent either way.
What separates them is that the worst individual offenders are unglossed, and a reader
tapping the same character in chapters 3 and 11 gets two different words.

### 2.2 The current builder misses characters, and spends its budget on phrases

The shipped 56-entry glossary for this book (`.tbook_cache/glossary-96c6aad….json`, built
from a 200-sentence sample) contains entries like

```
Hendrix’s olfactory wake-up call → обонятельный будильник Хендрикса
digitise and freight the minds   → оцифровать и переправить сознания
first novel                      → дебютный роман
```

— one-off sentence fragments from the sample, not recurring terms. Meanwhile **Elliott**
(147 occurrences), **Jerry** (74), **Irene** (53), **Sarah**, **Milo**, **Quell** are absent.
A 200-sentence sample of a 14.7k-sentence book hits a character with 147 occurrences about
twice; frequency mining over the whole book cannot miss it.

Adherence measured on the shipped translation (`glossary_adherence.py`, 1,931 term
occurrences) shows where the budget is wasted:

| entry shape | entries | occurrences | full adherence |
|---|---|---|---|
| single-word term | 18 | 1,788 | **94.9%** |
| multi-word phrase | 36 | 143 | **61.5%** |

Phrase entries take two thirds of the entry budget, cover 7% of the occurrences, and are
followed 61% of the time. Single-word head terms are followed almost always (`Ortega →
Ортега` 456/456, `Bancroft → Бэнкрофт` 414/414, `Kawahara → Кавахара` 239/239).

Two instructive non-failures in that table: `stack → стэк` scores 0/84 and `re-sleeved →
пересажен в новое тело` 0/15, but the model is not drifting — it consistently writes «стек»
and «переоболочивание» instead. It overrides the enforced rendering with its own and then
sticks to it. `bubblefab → бабблфаб` is the real failure: «пузырьковых кабин», «пузырьковых
фабрикатов», «пузырькового модуля» — three renderings in five occurrences.

### 2.3 A frequency miner and the current sample pass are complementary

Mining candidates locally and asking the model only to filter and render them costs
**$0.011 and 7 requests** per book, and yields 201 entries — **175 single-word terms the
current glossary does not cover**, including every major character.

But 30 of the current glossary's entries cannot be proposed by frequency mining at all:
ordinary English words the book repurposes (`stack`, `hull → корпус`, `re-sleeved`), which
sit in the bilingual lexicon and are therefore invisible to an out-of-vocabulary detector.
An attempt to find those from rendering instability instead (`glossary_mine.py --drift`)
**failed** — see §5.

So the recommendation is not to replace the sample-based pass but to add mining beside it.

### 2.4 Raising the cap and the sample does not work — the builder does not enumerate

Added after the first version of this report, because it overturns the cheapest-looking
recommendation. Four builder arms, each one request through the production
`glossarySystemPrompt` with the cap text and sample size changed
(`bench-quality/gloss_builder_arms.py`), scored against the 159 mined names:

| builder arm | entries returned | single-word | phrases | frequent names found (freq ≥ 20) | all mined names |
|---|---|---|---|---|---|
| sample 200, cap 40 (**production**) | 20 | 14 | 6 | 9 / 46 | 10 / 159 |
| sample 600, cap 150 | 27 | 18 | 9 | 6 / 46 | 11 / 159 |
| sample 600, cap 150 + head-term rule | 28 | 18 | 10 | **18 / 46** | 21 / 159 |
| sample 1200, cap 250 + head-term rule | 31 | 16 | 15 | 18 / 46 | 20 / 159 |

**The model returns 20–31 entries no matter what the cap allows**, and a larger sample does
not help: the task it is given is "return the KEY RECURRING TERMS", which it answers with a
short curated list, not an enumeration. Even the best arm misses Elliott ×147, Prescott ×83,
Jerry ×74, Irene ×53. Run-to-run variance is large (9 → 6 → 18 → 18 of 46), so a single
build is not even reproducible.

Conclusions:

- **Raising `glossarySampleMax` (200 → 600 → 1200) buys nothing** and triples to sextuples
  the build prompt (3.2k → 9.3k → 18.5k tokens). Leave it at 200.
- Raising the cap alone buys nothing either — the ceiling is the model's willingness to
  enumerate, not the cap.
- The head-term rule is worth having on its own: frequent-name recall doubled (9 → 18 of 46).
- **Complete coverage can only come from local frequency mining**, which finds all 159 by
  construction for $0.011. This moves the miner from "nice to have" to required.

---

## 3. Capacity: adherence, cost and collateral versus glossary size

Six arms over the same 465-sentence probe. Arms `a600` and `a1392` are `a244` plus 356 or
1,148 entries of **unrelated technical terminology** (aviation, cardiology, volcanology,
heraldry…), each verified absent from the probe text — a dilution test: does the model still
find the 244 entries that matter inside a wall of ones that do not?

| arm | entries | own-glossary adherence | shared-244 adherence | prompt tok | cached | $/req | p50 |
|---|---|---|---|---|---|---|---|
| a0 | 0 | — | 81.8% full / 13.7% miss | 812 | 0 | 0.00109 | 2.6 s |
| a56 (production) | 56 | 100.0% | 88.1% / 11.3% | 1,422 | 0 | 0.00124 | 2.4 s |
| a244 | 244 | 98.6% | **98.6% / 1.2%** | 2,894 | 0 | 0.00160 | 2.5 s |
| a244b (repeat) | 244 | 98.6% | **98.6% / 1.2%** | 2,895 | 0 | 0.00160 | 2.5 s |
| a600 | 600 | 98.8% | **98.8% / 1.2%** | 6,260 | 672 | 0.00229 | 2.7 s |
| a1392 | 1,392 | 98.6% | **98.6% / 1.2%** | 13,223 | 4,065 | 0.00327 | 2.9 s |

- **No dilution.** 1,148 irrelevant entries around the 244 real ones change adherence by
  0.0 pp. The repeat arm fixes the noise floor at ±0.2 pp, so nothing here is even close to
  significant.
- **The glossary itself is worth +16.8 pp** of adherence (81.8% → 98.6%). The production
  56-entry glossary reaches only 88.1% of the same term set, because it does not contain the
  terms.
- **The residual 1.2% at 244 entries is bad entries, not model failure**: `JacSol → JacSol`,
  `Linkage → Data Linkage`, `San → Сан-Диего`, `IAD → отдел внутренних расследований`. Fix
  the entries and adherence is ~99.5%.
- Marginal prompt cost is **8.5–10.9 tokens per entry** (≈9), stable across sizes. Output
  tokens are flat (585–591) — a glossary does not inflate generation.
- **Implicit prompt caching starts paying above ~4k-token prompts**: 11% of the prompt served
  from cache at 600 entries, **31% at 1,392**. The cost curve is therefore sublinear at the
  top end.

Extrapolated to one whole book (918 translate batches of 16 sentences):

| entries | prompt tokens | translate pass | delta vs no glossary |
|---|---|---|---|
| 0 | 0.75 M | $1.00 | — |
| 56 | 1.31 M | $1.14 | +$0.14 |
| 244 | 2.66 M | $1.47 | +$0.47 |
| 600 | 5.75 M | $2.11 | +$1.11 |
| 1,392 | 12.14 M | $3.00 | +$2.00 |

`repairSystemPrompt` embeds the glossary too, so a run with `--repair` pays the delta twice.
The align pass does not (`alignSystemPrompt` takes no glossary), and neither does the judge.

### 3.1 Collateral on sentences with no glossary term

150 control sentences, mean 1 − similarity against every other arm:

```
             a0    a244   a244b    a600   a1392     a56
     a0       —   0.090   0.094   0.085   0.080   0.085
   a244   0.094       —   0.055   0.073   0.074   0.065
  a244b   0.095   0.056       —   0.067   0.067   0.068
   a600   0.081   0.072   0.063       —   0.063   0.076
  a1392   0.080   0.075   0.067   0.067       —   0.068
```

Two identically-configured runs already differ by **0.055** (temperature 0.3). Any two
glossary arms differ by 0.063–0.075, and every arm differs from the no-glossary arm by
0.080–0.095. Reading: *having* a glossary perturbs unrelated sentences slightly more than
sampling noise, and **growing it from 244 to 1,392 adds nothing measurable**. Whether that
small perturbation is better or worse in quality is not measured here — it would need blind
pairwise judging.

### 3.2 What does limit the size

Nothing in the model's behaviour, but three practical things:

1. **Local-provider context.** 9 tokens/entry means 1,000 entries ≈ 9k prompt tokens per
   request. Local defaults (`ollama`/`llamacpp`, batch 4) commonly run 4–8k contexts; a large
   glossary silently overflows them. Any cap should be provider-aware.
2. **Cache-namespace churn.** `GlossHash` namespaces every translation, so editing one entry
   re-translates the whole book. Only **26.7% of sentences contain any glossary term at all**
   — namespacing a sentence by the entries that actually occur in it would leave **73.3% of
   the book valid across a glossary edit**. That also defuses the trap documented in `LOG.md`
   (Stage 4), where a bumped `PromptVersion` regenerated the glossary and
   silently re-translated the book.
3. **Entry quality, which degrades with size.** 5 of 201 mined entries kept a Latin rendering
   (`JacSol → JacSol`, `WorldWeb → WorldWeb`), and Latin-script leakage into the Russian
   output rose from 1.5% of sentences (no glossary) to 3.2% (244 entries) — then stayed flat
   at 3.6% for 1,392. Every leaking sentence echoes a word from its own source, i.e. the
   *entries* teach the leak, not the size. The render pass also produced two outright
   mismatches (`reminiscent → PsychaSec`, `datastack → Urbline`). Both classes are catchable
   by a deterministic gate: reject an entry whose rendering is in the wrong script, or equals
   another entry's source.

**Recommended cap: 300 entries** — comfortably above the 200 a 14.7k-sentence novel needs,
1.5× the measured demand, +$0.6/book at worst, and far below anything that degrades
adherence. The technical ceiling is at least 1,392 and is set by context windows and money,
not by the model.

---

## 4. Gender in the glossary

### 4.1 The problem is real and the proofread pass does not fix it

`gender_probe.py --mine` derives each character's gender from the source, using only patterns
that bind to one name: an honorific directly before it, and the first gendered pronoun after
it with no other name in between. (Counting he/she anywhere in the sentence is not enough —
that noise put Laurens Bancroft at 0.52 *female*.) Of 20 hand-labelled characters, 12 correct,
1 wrong, 7 below the evidence threshold; **at confidence ≥ 0.8, 11 correct and 0 wrong**.

`--audit` then reads, through the stored alignment, whether the Russian past-tense verb agrees
where a confident-gender name is unambiguously the subject:

| book | audited pairs | wrong gender |
|---|---|---|
| `full-kimi` (no proofread) | 389 | 9 (2.3%) |
| `full-repair` (`--fluent`) | 390 | 8 (2.1%) |
| `full-ctx2` (`--repair --context 2`) | 389 | 9 (2.3%) |

Of the 8 flagged in the shipped book, 7 are real errors and 1 is a mining artefact:

```
Prescott (f)  “Prescott looked fixedly at a corner of the ceiling”  → «Прескотт неотрывно смотрел»
Ortega (f)    “Ortega tossed me the packet.”                        → «Ортега бросил мне пакетик»
Ortega (f)    “Ortega put a knee into the dealer’s neck”            → «Ортега вдавил колено»
Begin (f) ×4  “Begin looked me in the eyes.”                        → «Бегин посмотрел мне в глаза»
Tengu         “the Tengu tore her apart”                            → miner read the object pronoun
```

The strict subject test covers only a slice of the exposure (the loose test finds 546 pairs,
and adjectives and participles are not checked at all), so ~7 errors per book is a floor, not
a total. The important part is the middle column: **`--repair` and `--context 2` are flat on
gender** (9 → 8 → 9). The lever that was supposed to fix a wrong gender given context does
not, which is what makes a glossary-carried fact the right place to put it.

### 4.2 Where the gender comes from

Two independent sources, and they can be combined into a high-precision rule:

- **LLM tagging inside the render pass.** Asking the same call that renders a term to also
  classify it (`kind: person|place|org|thing`, plus `gender` for a *named individual* only,
  omitted whenever uncertain) costs nothing extra: $0.011, 7 requests, and it tagged 71 of
  189 entries.
- **The deterministic pronoun miner** above.

Where both are confident they **agree 16/16**. Against 17 hand-labelled characters the
combination is **17/17 correct**. The combination rule that matters is the veto:

> reject the tag when the miner has substantial but split evidence (≥15 observations,
> confidence < 0.8) — that is the signature of a surname shared by characters of different
> genders.

It fires exactly where it should: **Bancroft** (Laurens *and* Miriam Bancroft; 203
observations at 0.59) and **Elliott** (Victor, Elizabeth and Irene Elliott; 27 at 0.70) are
refused a gender, and the LLM alone would have tagged both masculine. Gender is a property of
a referent, not of a name.

Two mining gaps found on the way: a name that collides with a common English word is missed
by both miners (**Leila Begin** — «begin» appears lowercase, so the proper-noun filter drops
it, and the render pass read the bare candidate as the verb), and single-token mining splits
multi-word names (`San → Сан-Диего`, `Bay → Бэй-Сити`). Mining capitalised bigrams would fix
both.

### 4.3 The famous-book confound

Measured on the real probe, the gender tag does **nothing**:

| arm | judge accuracy | regex accuracy |
|---|---|---|
| no glossary | 78.0% | 74.1% |
| glossary, no gender | **100.0%** | 92.7% |
| glossary + gender tags | 100.0% | 92.8% |
| glossary + tags + rule line | 100.0% | 92.7% |

A glossary with no gender information in it fixes 16 of 21 wrong-gender sentences, e.g.

```
EN            Ortega tossed the printout back into oblivion.
no glossary   Ортега швырнул распечатку обратно в небытие.
+ glossary    Ортега швырнула распечатку обратно в небытие.
```

The rendering «Ортега» is the same in both. What changed is the rest of the prompt: a cast
list of Kovacs, Ortega, Kawahara, Bancroft, Miriam, Trepp, Kadmin **identifies the novel**,
and the model recalls from its own knowledge that Kristin Ortega is a woman. On a book the
model knows, the glossary supplies gender as a side effect and there is nothing left for a
tag to do.

That is not the converter's normal case. So the probe was de-identified: every character name
in the 98 sentences replaced by an invented surname, all with consonant-final Russian
renderings so the rendering itself signals nothing either (`gender_deid.py`).

| arm (de-identified) | judge accuracy | marked | correct | wrong |
|---|---|---|---|---|
| glossary, no gender | **45.2%** | 62 | 28 | 34 |
| glossary + gender tags | **96.3%** | 81 | 78 | 3 |
| glossary + tags + rule line | **97.6%** | 82 | 80 | 2 |

Paired, same sentences: tags **fixed 30, broke 1** (exact sign test **p = 3.0e-08**);
tags + rule line **fixed 32, broke 1** (**p = 7.9e-09**). The regex scorer independently gives
35.1% → 91.6% → 93.9% (fixed 44, broke 1).

Split by expected gender (regex, de-identified):

| arm | referent is female | referent is male |
|---|---|---|
| no gender | **11%** (6/53) | 88% (21/24) |
| + tags | 95% (57/60) | 83% (19/23) |
| + tags + rule | 98% (57/58) | 83% (20/24) |

Without the tag the model defaults to masculine: female characters are wrong nine times out
of ten. The apparent dip on male referents is 3 detector false positives out of 4 plus one
genuine over-correction (`Vantrell [male] → «сбросила»`), i.e. ~96% true.

```
EN                Nesbrand was sleeping, an assembly of low-frequency sine curves beneath the single sheet.
no gender         Несбранд спал, …
+ [female] tag    Несбранд спала, …
```

### 4.4 Another target language: Spanish

Same de-identified probe, en→es, scored by the judge (no regex can do Spanish):

| arm | sentences marking gender | correct | accuracy |
|---|---|---|---|
| glossary, no gender | 22 / 98 | 10 | 45.5% |
| glossary + tags + rule | 20 / 98 | 17 | **85.0%** |

Fixed 6, broke 0, p = 0.031. The same direction with a much smaller exposure: Spanish marks a
character's gender in about a fifth of these sentences (adjectives, participles, articles),
where Russian marks it in nearly all of them through past-tense verbs. Ranking of targets by
how much this matters: **ru, uk, pl, cs** (past-tense verbs and adjectives) ≫ **es, fr, it,
pt** (adjectives, participles, articles) ≫ **de** (pronouns, predicative adjectives rarely) ≫
**en, zh, ja, ko, tr** (nothing to gain).

### 4.5 The tag is free

Adding tags to a 189-entry glossary: prompt 2,322 → 2,660 tokens, i.e. **+4.0 tokens per
tagged entry** (71 tagged) plus **54 tokens once** for the rule line; latency p50 2.5 → 2.6 s;
adherence to the glossary **98.1% in both arms**; and the control-stratum distance to the
untagged arm (0.071) is indistinguishable from the distance between any two prompt variants
(0.070). No cost, no collateral.

The rule line — one sentence explaining what `[male]`/`[female]` means — is worth +1.3 pp on
its own (fixed 2, broke 1, p = 1.0): **not significant, keep it anyway** because it is 54
tokens once per request and it is what makes the tag self-describing for models that have not
seen this format.

---

## 5. Negative results

- **Mining book-specific senses of ordinary words from rendering drift: rejected.**
  `glossary_mine.py --drift` reads back what each ordinary word was rendered as and ranks by
  instability. The top of the list is legitimate polysemy and case inflection — `back`
  («обратно»×108, «вернулся»×67, «назад»×54), `across`, `thought`, `mouth`, `floor` — and the
  words we actually want (`stack`, `sleeve`) are nowhere near the top. Signal-to-noise is too
  poor for the signal to stand alone; the LLM-on-a-sample pass finds these terms and should be
  kept for exactly that job.
- **The gender tag on a book the model knows: no effect** (§4.3). Any future gender experiment
  on a famous book measures nothing; de-identify first.
- **The `[male]`/`[female]` rule line as a separate lever: no measurable effect** (p = 1.0).
- **Padding the glossary to prove a dilution ceiling: no ceiling found** — the experiment
  intended to find the breaking point (1,392 entries) never broke.

---

## 6. Recommended implementation

In `internal/translate`:

1. **`GlossEntry` gains two optional fields** (`glossary.go`):
   ```go
   type GlossEntry struct {
       Src    string `json:"src"`
       Tgt    string `json:"tgt"`
       Gender string `json:"gender,omitempty"` // "m" | "f" — named individuals only
       Kind   string `json:"kind,omitempty"`   // person | place | org | thing
   }
   ```
   Old sidecar files keep parsing; a file without the fields behaves exactly as today.
2. **`GlossHash` must hash the gender**, or an edited gender will not invalidate the
   translation cache. Note the trap from `LOG.md` (Stage 4): bumping `TrPromptVersion` at the
   same time changes the glossary cache path too, regenerates the glossary, and silently
   re-translates the whole book. Change one at a time, and pre-seed the new
   `glossary-<hash>.json` when only the prompt version moves.
3. **Prompt** (`translateSystemPrompt`, and the same block in `repairSystemPrompt`): keep the
   glossary header, add one line before the list when any entry carries a gender, and suffix
   the tagged entries:
   ```
   GLOSSARY — use these Russian translations consistently wherever the term appears:
   A [male] / [female] tag gives the gender of the person that term refers to. Every Russian
   word that agrees with that person — past-tense verb, adjective, participle, pronoun — must
   take that gender, even where English does not mark it.
   - Ortega → Ортега  [female]
   - Kovacs → Ковач  [male]
   - neurachem → нейрохим
   ```
   Emit the line and the tags only for targets that mark gender (`ru uk pl cs be bg sr hr es
   fr it pt ro ar he hi`); for `en zh ja ko tr fi hu` the field is carried in the file but not
   rendered.
4. **Raise the cap to 300 entries** in `glossarySystemPrompt`, add the head-term rule, keep
   `glossarySampleMax` at 200 (§2.4: a larger sample measurably buys nothing), and add
   frequency mining beside the sample pass — the cap only becomes reachable once mining
   supplies the entries:
   - mine the whole book locally for names (capitalised mid-sentence, never lowercase — plus
     capitalised bigrams, which the prototype lacks) and for out-of-lexicon coined terms;
   - send the candidates to one filter-and-render call that also classifies kind and gender
     ($0.011, 7 requests for 312 candidates);
   - merge with the existing sample-based glossary, which contributes the sense-shifted
     ordinary words (`stack`, `hull`) that mining cannot see;
   - prefer single-word head terms; drop phrase entries whose head word is already covered
     (61.5% adherence against 94.9%).
5. **Deterministic entry gates** before enforcement: reject an entry whose rendering is in the
   wrong script for the target, whose rendering equals another entry's source, or whose source
   is a function word. This is what causes the Latin leakage (1.5% → 3.2% of sentences).
6. **Gender acceptance rule**: LLM tag ∧ not vetoed by the pronoun miner (§4.2). Only for
   `kind == person` **and** a named individual — never a category of people, or
   «католичку» becomes «католика» exactly as the proofread pass once did.
7. **Optional, larger win**: namespace the translation cache by the glossary entries that
   occur in *that sentence* rather than by the whole glossary. 73.3% of a book survives a
   glossary edit.

Not recommended: `--context 2` as a gender fix (measured flat, §4.1), and any per-target
glossary sharing (already established: an en→ru glossary enforced on Latin targets puts
Russian into 50–73% of their sentences, `four-model-bench-2026-08.md` §8).

---

## 7. Reproduction

Scripts (in `bench-quality/`): `glossary_demand.py`, `glossary_mine.py`,
`glossary_adherence.py`, `gloss_probe_set.py`, `gloss_scale_probe.py`,
`gloss_scale_analyze.py`, `gender_probe.py`, `gender_deid.py`, `gender_score.py`,
`gender_judge.py`. Evidence, glossaries, probe sets and all arm outputs:
`bench-quality/.artifacts/glossary-scale/` (gitignored — verbatim book text).

```bash
# prompt fidelity against the Go source
python3 bench-quality/gloss_scale_probe.py --check

# demand and adherence on an existing book (free, no API)
python3 bench-quality/glossary_demand.py bench-quality/full-repair.tbook --json /tmp/d.json
python3 bench-quality/glossary_adherence.py bench-quality/full-repair.tbook GLOSSARY.json

# build a glossary the recommended way
python3 bench-quality/glossary_mine.py BOOK.tbook --out cand.json
python3 bench-quality/gloss_scale_probe.py --render cand.json --out gloss.json

# gender: mine, audit a shipped book, then A/B on de-identified text
python3 bench-quality/gender_probe.py BOOK.tbook --mine --out gender.json
python3 bench-quality/gender_probe.py BOOK.tbook --audit gender.json --window 1
python3 bench-quality/gender_deid.py probe-gender.json gloss-gender.json \
        --out-probe probe-deid.json --out-gloss gloss-deid.json
python3 bench-quality/gender_judge.py --dir ART --probe probe-deid.json --lang Russian dA:none dB:tags
```

---

## 8. Replication on a second book: *Revelation Space*

Everything above was measured on one book, so the whole chain was repeated on **Revelation
Space** by Alastair Reynolds (15,431 sentences, 207k words, en→ru) — a different author with
much heavier invented terminology. This book has no reference translation on disk, which
turned out not to matter: demand mining, builder arms, adherence and the gender A/B all need
source text plus the arms' own output. Sentences were dumped with the production parser and
segmenter (`epub.ParseOpts` + `segment.BuildSentenceObjects`).

Sentences were dumped with `bench-quality/dumpsents` (a research tool that calls
`epub.ParseOpts` + `segment.BuildSentenceObjects` and writes the same rune-offset word
spans the pipeline uses).

### 8.1 The builder still does not enumerate — and misses the protagonist

| builder arm | entries returned | frequent names found (freq ≥ 20) |
|---|---|---|
| sample 200, cap 40 (**production**) | 22 | 9 / 48 |
| sample 600, cap 150 | 34 | 22 / 48 |
| sample 600, cap 150 + head-term rule | 33 | 20 / 48 |
| sample 1200, cap 250 + head-term rule | 18 | 12 / 48 |

At the production setting the glossary misses **Sylveste ×1031, Volyova ×917, Khouri ×903,
Sajaki ×505** — the four main characters of the novel. Raising the sample to 1200 makes it
*worse* (18 entries), confirming §2.4: the ceiling is the model's willingness to enumerate,
and a single build is not reproducible.

### 8.2 Adherence and dilution reproduce

| arm | entries | adherence on the 113 mined terms |
|---|---|---|
| no glossary | 0 | 79.9% full / 18.3% miss |
| production glossary | 41 | 73.3% / 23.8% — no better than none, it does not contain the terms |
| mined | 113 | **95.9% / 3.4%** |
| mined + 472 unrelated padding entries | 600 | **93.8% / 5.2%** |

Same shape as book 1: the glossary is worth ~+16 pp of adherence, and 472 irrelevant entries
around the real ones cost nothing. Control-stratum distances (0.067–0.092) sit in the same
band as book 1's noise.

### 8.3 A defect the second book exposed: the render pass glues renderings to the wrong term

Mapping the reply by candidate id alone is not safe. Measured on the first render outputs:

| render output | entries mapping a lowercase English word to a capitalised Russian name |
|---|---|
| *Revelation Space*, 128 entries | **12** — «aside → Новый Комусо», «premonitory → Волёва», «sprang → Мадемуазель», «inflict → Сильвест» |
| *Altered Carbon*, 189 entries | **6** — «variant → Мир Харлана», «steady → Кристин Ортега», «nape → Трепп» |

This is the same positional-drift failure the align pass already guards against with its
"numbered echo" contract (`alignSystemPrompt`). Applying the same fix — the reply must echo
the candidate's term, and an entry whose echo does not match its id is discarded — gives:

| render output | misalignments | adherence on its own terms |
|---|---|---|
| RS, id mapping only | 12 / 128 | 92.2% |
| RS, **echo contract** | 1 / 113 | **95.9%** |
| AC, id mapping only | 6 / 189 | — |
| AC, **echo contract** | 0 / 192 | — |

The one survivor on RS (`safeguard → Институт Сильвеста`) plus two identity renderings
(`learnt → learnt`, `aside → aside`) are caught by the deterministic gates of §3.2 — wrong
script, or rendering equal to the source. Both guards are needed; neither is sufficient alone.

### 8.4 Gender: the effect is larger on the second book

*Revelation Space* yields a much bigger gender probe than *Altered Carbon* (393 sentences
against 98) because three high-frequency characters carry most of the narration. De-identified
as in §4.3:

| arm | judge accuracy | regex accuracy |
|---|---|---|
| glossary, no gender | **40.9%** | 42.7% |
| glossary + tags + rule line | **99.4%** | 98.0% |

Paired: judge **fixed 150, broke 0** (exact sign test **p = 1.4e-45**); regex fixed 195,
broke 2. Book 1 measured 45.2% → 97.6%; book 2 measures 40.9% → 99.4% on four times the
sample.

### 8.5 The merge rule was corrected by the second book

The first version of the combining rule rejected any name whose pronoun evidence was split
(≥15 observations at confidence < 0.8). On *Revelation Space* that threw away **Pascale**
(53 observations at 0.57 — she is female, and the LLM said so) and left **Sajaki** (505
occurrences) untagged. The split is not evidence of ambiguity by itself: a character who
shares most of her scenes with men has split pronoun evidence.

Corrected rule, now in `bench-quality/gender_merge.py`:

- accept the LLM tag **unless the miner's majority points the other way** on ≥15
  observations — direction, not spread;
- accept a miner-only tag when the entity is a person, evidence ≥ 15 and confidence ≥ 0.8
  (swept 40 / 25 / 15 on both books: lowering it adds tags and no errors).

Result across both books: **27 of 27 hand-checked characters correct, 0 wrong**, 64 tagged of
192 entries on *Altered Carbon* and 25 of 113 on *Revelation Space*. It still refuses exactly
the right names — **Bancroft** (LLM said male, miner majority female on 203 observations:
Miriam is named 114 times against Laurens' 43) and **Sluka** on *Revelation Space* — and
without that veto a wrong tag on Bancroft alone would have forced masculine agreement on
**124** subject-plus-past-verb pairs, against the ~7 real gender errors in the whole shipped
book.

---

## 9. What was implemented

Everything in §6 except the local-provider cap, which the user deprioritised (local
providers are barely used here). In `internal/`:

| change | file |
|---|---|
| `GlossPromptVersion` keys the glossary build separately from the align contract | `cache/cache.go` |
| `GlossEntry.Gender` / `.Kind`, hashed into `GlossHash` (gender only — `Kind` never reaches a prompt, so it must not invalidate translations) | `translate/glossary.go` |
| `BuildGlossary` merges the sample pass with mined+rendered candidates, applies the gates and the 300-entry cap | `translate/glossary.go` |
| local mining: names (unigrams + bigrams), coined terms via the lexicon, gender evidence | `translate/glossmine.go` (new) |
| render pass with the echoed-term contract, entry gates, gender merge rule | `translate/glossrender.go` (new) |
| `[male]`/`[female]` tags + the rule line, in the translate and proofread prompts | `translate/prompt.go` |
| `--no-glossary-gender` | `config/config.go`, `cmd/convert/main.go` |

Notes on what the implementation added beyond the prototype:

- **Sample size stays at 200** (§2.4) and the sample prompt now asks for head terms and
  says explicitly that its unique value is ordinary words used in a book-specific sense.
- **Bigram mining** was missing from the prototype and is what fixes `San → Сан-Диего`.
  A pair is not mined across a stripped possessive, or "Sky’s Edge" would be enforced as
  "Sky Edge", which never occurs in the book.
- **Name mining is skipped for sources that capitalise every noun** (de): there the
  never-lowercase test cannot tell a name from a noun. Coined-term mining still runs.
- **Gender mining counts sentence-initial names**, which the prototype ignored (it has to
  ignore them when *finding* candidates, but not when weighing an established name).
  Re-validated on both books after the change: still 0 errors on 30 hand-checked
  characters, still vetoes Bancroft and Sluka.
- The repair pass carries the tags too, and its context-free "never correct a gender" rule
  now has a carve-out for a tagged person (`RepairPromptVersion` r1 → r2). That carve-out
  is **not separately measured** — the 45% → 98% figure is the translate pass.
- One bug the smoke run caught: `Sentence.Words` holds **rune** offsets, and slicing `Src`
  by them as bytes mined mojibake fragments (`Sky��`, `e amarant` out of "the
  Amarantin"). Mining now uses `embalign.WordStrings`. Regression test:
  `TestMineGlossCandidatesHandlesNonASCII`.

End-to-end smoke run (Revelation Space, 3 chapters, 2276 sentences, en→ru): mined 78
candidates → kept 61 → merged to 72 enforced terms, 16 carrying gender, all correct by
hand (Volyova=f, Khouri=f, Pascale=f, Sylveste=m, …). Validation 0 empty / 0 offset / 0
struct, coverage 98% / 97%, lexcheck 35 of 2238 (1.6%), 2m11s wall. Agreement in the
shipped text: «Паскаль Дюбуа **была** молодым журналистом», «— **сказала** Паскаль» where
the English is "Pascale said".

---

## 10. The open question closed: a bigger glossary does not cost quality

§3.1 could show only that a glossary changes text it has no term in, not whether the change
is for the better. Blind pairwise judging answers that: 150 pairs sampled from the sentences
where the two arms differ (334 of 465 differed, 130 were identical), both presentation orders,
one fresh judge process per batch, de-swapped and analysed by `analyze_pairs.py`. Arms are
`a56` (the production 56-entry glossary) against `a244` (the mined 244-entry one), same book,
same model, same probe.

| | decisive | share |
|---|---|---|
| prod56 | 34 | 46.6% |
| **mined244** | **39** | **53.4%** |

71 ties + 6 order-inconsistent (counted as ties). **Sign test p = 0.64 — a clean tie.**

That is the result the change needed: the 4.4x bigger glossary carries no quality regression.
It is also, honestly, no general improvement — which is expected, because what it buys is
name consistency across chapters and gender agreement, and neither is visible to a judge
looking at one sentence in isolation. The defect classes the judges recorded for the losing
side are dominated by fidelity (73) and terminology (26), i.e. the ordinary noise of two
samples from the same model at temperature 0.3.

Reading of the three measurements together: the bigger glossary is **+16.8 pp of term
adherence, ±0 quality, +$0.33 per book**.
