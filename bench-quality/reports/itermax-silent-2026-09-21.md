# Recall for silent words only — `itermax-silent` and `itermax-silent-cc`, 2026-09-21

Plan item **D4d**, the variant `itermax-guard-2026-09-19.md` §8 pointed at after the closed-class
guard failed its kill criterion. Two new opt-in aligner methods, four arms, the grid's five
instruments, and D4b's blind judging protocol against the shipped `argmax` baseline.

**The result: the first aligner change in this series the judge prefers to `argmax`.** Restricting
`itermax`'s second round to source words that light *nothing* — and, in the `-cc` variant, to
*content* words that light nothing — keeps the whole coverage gain (content-word tap coverage
0.9784 → **0.9957** on AC and 0.9817 → **0.9957** on RS, `itermax`'s own number) while changing
**nothing** that base already lit: 0 widened, 0 narrowed, 0 moved, 0 darkened across 14 143 and
21 245 word taps. Judged blind in both orders, `itermax-silent-cc` beats base **83 : 11** on
*Altered Carbon* (94 decisive, p = 8.0e-15) and **115 : 15** on *Revelation Space* (130 decisive,
p = 2.8e-20); the unrestricted `itermax-silent` also passes, 60 : 17 and 71 : 13. The kill
criterion (base > 60 % of decisive pairs on either book) is cleared by both; `-cc` comes in at a
fifth of the threshold.

**Recommendation: `EMBALIGN_METHOD=itermax-silent-cc`.** The 168 and 278 closed-class recoveries
that separate the two variants are measurably worth nothing — on the sentences carrying only those,
the judge splits 11 : 12 (p = 1) and 9 : 13 (p = 0.52) with 53 and 54 further ties — while `-cc`
halves the blast radius, cuts the tie rate by two thirds and leaves lexcheck flat. §7 gives the
exact five-file change; **no default was changed in this task.**

---

## 1. What was built

### 1.1 Two method values

```
EMBALIGN_METHOD=itermax-silent        # recall for silent source words, any class
EMBALIGN_METHOD=itermax-silent-cc     # the same, content words only
```

The rule, stated as the contract the structural check tests: **a source word the `argmax` pipeline
lights keeps that highlight exactly — never widened, never narrowed, never moved — and only a
source word `argmax` leaves dark may take `itermax`'s.** `-cc` adds: a dark word in the source
language's closed-class list (`tools/stopwords.json`, exported from `internal/stopwords` by
`cmd/stopwordsdump`, D4c §1.2) stays dark, so only content words are ever recovered.

Both are distinct method values, like D4c's `itermax-guarded`. `argmax` and `itermax` are
untouched; §2 proves it.

### 1.2 The merge has to run after glue — measured, not assumed

The first implementation merged **at the matching stage**: keep `argmax`'s round-one word pairs for
any source word round one aligned, take `itermax`'s round-two pairs only for the rest, then let the
unchanged `glue_idioms` / `glue_target` steps run on the merged set — the same place D4c's guard
merges. It does not satisfy the contract, and the structural check on those arms
(`.artifacts/itermax-silent/preglue/`) says by how much:

| base-lit source words that changed anyway | ac-silent | ac-silent-cc | rs-silent | rs-silent-cc |
|---|---|---|---|---|
| moved to a disjoint fragment — closed-class | **466** | **0** | **711** | **0** |
| moved to a disjoint fragment — content | 83 | 83 | 87 | 87 |
| narrowed | 394 | 109 | 606 | 146 |
| shifted / widened | 8 | 1 | 26 | 6 |
| **total violations** | **951** | **193** | **1 430** | **239** |

The closed-class row is not a rounding error: 466 and 711 are *exactly* the closed-class disjoint
jumps D4b measured base winning 47 : 7 and 48 : 13 on, reintroduced in full by a variant whose
whole point is to never move anything. The mechanism: a source word can be unaligned after
round one and still be **lit in the base arm by a glue step**. A matching-stage rule cannot see
that, hands the word `itermax`'s round-two pair, and the glue that used to light it no longer
fires — so a word that had a highlight gets a different one. D4c §3 found the same leak from the
other side; here it is large enough to reintroduce the very failure the variant exists to avoid.

So the shipped rule runs **both pipelines in full** — `argmax` → its glue, `itermax` → its glue,
on the same similarity matrix — and merges the two **final** pair lists:

```python
if method in (SILENT, SILENT_CC):
    local = silent_only(
        glued(argmax_pairs(sim),  sim, s_wids, t_wids),
        glued(itermax_pairs(sim), sim, s_wids, t_wids),
        src_words or [], stop or set(), method == SILENT_CC)
else:
    ...unchanged...
```

`glued()` is the existing word conversion plus the existing two glue steps, factored out
verbatim; the extra cost is one more pass of two numpy loops over a similarity matrix that has
already been computed. `silent_only()` then walks the union of source words and takes `argmax`'s
set whenever it is non-empty.

### 1.3 Branch logging

The child prints a counter line to stderr (the converter's log) every 500 sentences and once at
EOF. From the four shipped arms:

```
ac-silent    1277 sentences, 13668 source words kept exactly as argmax aligned them; of the words
             argmax left dark, a pair was taken for 130 CONTENT and 168 closed-class words
rs-silent    1566 sentences, 20570 kept; 177 CONTENT and 278 closed-class words recovered
ac-silentcc  1277 sentences, 13668 kept; 130 CONTENT recovered, 168 closed-class left dark
rs-silentcc  1566 sentences, 20570 kept; 177 CONTENT recovered, 278 closed-class left dark
```

130 / 177 content and 168 / 278 closed-class are **exactly** the silent → lit counts D4b measured
for bare `itermax` (§2 of that report), which is the first sign that the rule recovers everything
`itermax` recovers and nothing else. §3 confirms it at the tap level.

For contrast, the rejected matching-stage build reported 385 / 482 content and 1 007 / 1 563
closed-class "recoveries" — words unaligned after round one, most of which the glue steps were
already lighting in the base arm. That gap *is* the leak §1.2 measures.

## 2. Default identity, again

The same two checks D4c used, both re-run against the **pre-D4c** file recovered from git
(`git show HEAD:tools/embalign.py`), plus a new one for D4c's own method:

```
default (argmax/itermax) mismatches vs the PRE-D4C file, 400 random pairs x 2 methods: 0
itermax-guarded mismatches vs the D4C file, 400 random pairs:                           0
itermax-silent / itermax-silent-cc merge-semantics violations:                          0
end-to-end: words argmax lights that a SILENT arm changes:                              0
end-to-end: closed-class words itermax-silent-cc newly lights (must be 0):              0
```

End to end on a whole book, `ac-base` re-run with the new script and a newly built converter:

```
ac-base.tbook vs ac-base-d4d.tbook: 0 / 1303 sentences with a different alignment chunk list;
0 translation texts differ; chapter payloads byte-identical: True
```

lexcheck on that arm: **30 flags**, the baseline's own number. `EMBALIGN_METHOD=argmax` is
unaffected, and so is `itermax-guarded`, so D4c's arms remain reproducible.

## 3. The structural check

`bench-quality/itermax_silentcheck.py` tests the contract of §1.1 on every source word of every
translated sentence of both books — 14 143 (AC) and 21 245 (RS) word taps, the tap ledger's own
populations.

| | ac-silent | ac-silent-cc | rs-silent | rs-silent-cc |
|---|---|---|---|---|
| translated sentences | 1 303 | 1 303 | 1 575 | 1 575 |
| sentences with a changed alignment | 226 | **117** | 336 | **164** |
| **base-lit source words that changed in any way** | **0** | **0** | **0** | **0** |
| newly lit CONTENT words | **130** | **130** | **177** | **177** |
|  … of those, in the tap-ledger population | 101 | 101 | 143 | 143 |
| newly lit closed-class words | 168 | **0** | 278 | **0** |
| newly lit fragments identical to `itermax`'s | **all 298** | **all 130** | **all 455** | **all 177** |
| content words still dark | 47 | 47 | 65 | 65 |

Every line of the contract holds exactly:

- **Nothing base lights changes.** 0 widened, 0 narrowed, 0 moved, 0 darkened, on four arms. The
  two mechanisms that sank `itermax` (D4b) and the closed-class guard (D4c §6) — a function word
  jumping to a disjoint fragment, a content word gaining a second unrelated one — are not merely
  rare here, they are impossible by construction.
- **The recovery is `itermax`'s, in full and unaltered.** 130 and 177 content words, the same
  words and the same fragments the grid and D4b measured; 101 and 143 of them in the tap-ledger
  population, the grid's own figures.
- **`-cc` recovers content words only.** 0 closed-class words lit, against 168 and 278 for the
  unrestricted variant — a population nobody has ever judged, which is why the variant exists.
- **The change surface is the smallest of any arm so far**: 117 and 164 sentences for `-cc`,
  against the guard's 321 / 436 and `itermax`'s 688 / 956.

Run independently through D4b's own change-classifier (`itermax_wordchanges.py`, which labels each
changed source word `lit` / `dark` / `widened` / `narrowed` / `shifted` / `moved`), the corpus-wide
tally has exactly one non-zero cell per class:

| arm | changed sentences | word changes | breakdown |
|---|---|---|---|
| `ac-silent` | 226 | 298 | `content/lit` 130 + `closed/lit` 168 |
| **`ac-silentcc`** | **117** | **130** | **`content/lit` 130** |
| `rs-silent` | 336 | 455 | `content/lit` 177 + `closed/lit` 278 |
| **`rs-silentcc`** | **164** | **177** | **`content/lit` 177** |

No `widened`, no `narrowed`, no `shifted`, no `moved`, no `dark` — on any arm, on either book. The
sentences the judge is about to see therefore differ from base in one way only: a word that lit
nothing now lights something. This is the population D4c §8 predicted would win, from a subset of
its own sample, 7 : 2 and 9 : 2.

## 4. Scores: the five instruments

Same invocations, same offline recipe (grid §9). Base, `itermax` and `guard` rows are recomputed
from the arms on disk and reproduce their published values exactly.

### Table A

| arm | knob | wall s | target cov all / content | source tap cov all / content | sentences tap < 0.80 | lexcheck flags | alignments changed vs base |
|---|---|---|---|---|---|---|---|
| `ac-base` | argmax | 265 | 0.9919 / 0.9930 | 0.9722 / **0.9784** | 30 / 1069 | 30 | 0 |
| `ac-itermax` | itermax | 520 | 0.9964 / 0.9969 | 0.9922 / **0.9957** | 4 / 1069 | 33 | 688 |
| `ac-guard` | itermax-guarded | 39 | 0.9939 / 0.9948 | 0.9808 / **0.9916** | 8 / 1069 | 31 | 321 |
| **`ac-silent`** | **itermax-silent** | 65 | 0.9949 / 0.9961 | 0.9922 / **0.9957** | 4 / 1069 | 32 | **226** |
| **`ac-silentcc`** | **itermax-silent-cc** | 71 | 0.9938 / 0.9948 | 0.9816 / **0.9957** | 4 / 1069 | **31** | **117** |
| `rs-base` | argmax | 335 | 0.9915 / 0.9929 | 0.9728 / **0.9817** | 24 / 1413 | 42 | 0 |
| `rs-itermax` | itermax | 684 | 0.9958 / 0.9972 | 0.9919 / **0.9957** | 3 / 1413 | 38 | 956 |
| `rs-guard` | itermax-guarded | 141 | 0.9933 / 0.9949 | 0.9797 / **0.9930** | 7 / 1413 | 41 | 436 |
| **`rs-silent`** | **itermax-silent** | 94 | 0.9946 / 0.9967 | 0.9919 / **0.9957** | 3 / 1413 | **42** | **336** |
| **`rs-silentcc`** | **itermax-silent-cc** | 100 | 0.9929 / 0.9946 | 0.9801 / **0.9957** | 3 / 1413 | **42** | **164** |

### Table B

| arm | clean | clean-multi | shared | smear | none | content-word silent | content-only clean / shared / none | probe unit % | probe word cov % |
|---|---|---|---|---|---|---|---|---|---|
| `ac-base` | 51.33 | 3.92 | 40.81 | 0.83 | 3.10 | **2.21** | 57.93 / 33.40 / 2.21 | 77.1 | 93.0 |
| `ac-itermax` | 54.66 | 2.86 | 40.69 | 0.79 | 1.00 | **0.62** | 56.83 / 37.33 / 0.62 | 82.4 | 97.5 |
| `ac-guard` | 51.10 | 3.56 | 42.11 | 0.93 | 2.31 | **1.11** | 56.36 / 36.23 / 1.11 | 79.4 | 95.2 |
| **`ac-silent`** | 51.23 | 3.17 | 43.75 | **0.85** | 1.00 | **0.62** | 57.66 / 36.10 / 0.62 | **87.0** | 97.5 |
| **`ac-silentcc`** | 51.35 | 3.63 | 41.99 | **0.84** | 2.18 | **0.71** | 57.92 / 35.21 / 0.71 | 82.4 | 95.2 |
| `rs-base` | 51.22 | 4.48 | 40.15 | 1.04 | 3.11 | **2.07** | 58.20 / 32.59 / 2.07 | n/a | n/a |
| `rs-itermax` | 54.80 | 3.40 | 39.91 | 0.92 | 0.97 | **0.58** | 57.35 / 36.20 / 0.58 | n/a | n/a |
| `rs-guard` | 50.87 | 4.23 | 41.46 | 1.08 | 2.36 | **0.95** | 56.67 / 35.33 / 0.95 | n/a | n/a |
| **`rs-silent`** | 51.15 | 3.70 | 43.13 | **1.04** | 0.97 | **0.58** | 57.92 / 35.18 / 0.58 | n/a | n/a |
| **`rs-silentcc`** | 51.24 | 4.23 | 41.20 | **1.04** | 2.28 | **0.73** | 58.23 / 34.12 / 0.73 | n/a | n/a |

### What the rows say

| | AC base → silent-cc → itermax | RS base → silent-cc → itermax |
|---|---|---|
| **content-word tap coverage** | 0.9784 → **0.9957** → 0.9957 | 0.9817 → **0.9957** → 0.9957 |
| share of `itermax`'s gain captured | **100 %** | **100 %** |
| content-word taps lighting nothing (ledger rule) | 2.21 % → **0.71 %** → 0.62 % | 2.07 % → **0.73 %** → 0.58 % |
| source words silent → lit (`cmd/score` list) | 0 → **130** → 130 | 0 → **177** → 177 |
| source words lit → silent, or re-mapped | 0 → **0** → (549 re-mapped) | 0 → **0** → (798 re-mapped) |
| sentences with content tap coverage < 0.80 | 30 → **4** → 4 | 24 → **3** → 3 |
| **lexcheck flags** | 30 → **31** → 33 | 42 → **42** → 38 |
| smear (all taps) | 0.83 → **0.84** → 0.79 | 1.04 → **1.04** → 0.92 |
| sentences whose alignment changed | 0 → **117** → 688 | 0 → **164** → 956 |
| hybrid gate q < 0.9 | 2.3 % → **1.7 %** → 0.9 % | 2.5 % → **2.0 %** → 1.1 % |

1. **The full coverage gain, this time actually in full.** Content-word tap coverage reaches
   `itermax`'s 0.9957 on both books — the number the closed-class guard reached only 76 % / 81 % of
   — and it does so while changing a *sixth* of the sentences `itermax` changes.
2. **Lexcheck is flat.** RS is **exactly** base's 42 flags, with 0 new and 0 gone; AC is +1, and
   `itermax_lexdelta.py` shows all three moved flags are recoveries, not regressions — the two new
   ones are sentences where a dark content word got lit (`feeling`/`like` → «хотелось»,
   `plan` → «стоит») and the one that disappears is another recovery (`maelstrom` → «вихрь»).
   The instrument is penalising thin dictionary support on a newly aligned pair, which is the
   grid §4.3 caveat, not a drift.
3. **Smear does not move.** 0.83 → 0.84 and 1.04 → 1.04, against the guard's 0.83 → 0.93. Nothing
   is being smeared across extra tokens, because nothing that already had a highlight is touched.
4. The residual difference from `itermax` is confined to the **all-word** columns (0.9816 vs
   0.9922 on AC), which is the 168 / 278 closed-class recoveries `-cc` deliberately declines.

## 5. Verdicts: base vs each variant

D4b's protocol unchanged: changed sentences sampled by the corrected even stride, both
presentation orders judged by a fresh `claude -p --model sonnet --tools ""` process in batches of
10, de-swapped, **decisive only when the two orders agree**. `ac-silentcc` has only 117 changed
sentences in the whole book, so its "sample" is the entire population — a census, not a sample.

| | `ac-silentcc` | `rs-silentcc` | `ac-silent` | `rs-silent` |
|---|---|---|---|---|
| changed sentences in the corpus | 117 | 164 | 226 | 336 |
| sampled | **117 (all)** | 150 | 150 | 150 |
| judged (no missing verdicts) | 117 | 150 | 150 | 150 |
| explicit ties | 18 | 14 | **63** | **63** |
| order-inconsistent (counted as ties) | 5 | 6 | 10 | 3 |
| tie rate | **19.7 %** | **13.3 %** | 48.7 % | 44.0 % |
| **decisive** | **94** | **130** | **77** | **84** |
| **base (`argmax`) wins** | **11 (11.7 %)** | **15 (11.5 %)** | **17 (22.1 %)** | **13 (15.5 %)** |
| **arm wins** | **83 (88.3 %)** | **115 (88.5 %)** | **60 (77.9 %)** | **71 (84.5 %)** |
| sign test, two-sided | **p = 8.0e-15** | **p = 2.8e-20** | **p = 8.9e-07** | **p = 7.9e-11** |

This is the first arm in the whole D4 series that the judge prefers to `argmax`, and it is not
close: on the shipping candidate the baseline wins about one decisive pair in nine, on both books,
at p < 1e-14. For comparison, on the same books and the same protocol, bare `itermax` lost 71 : 30
and 68 : 33 (D4b) and the closed-class guard lost 71 : 33 and 67 : 50 (D4c).

### 5.1 Where the two variants differ: the closed-class recoveries are worth nothing

`itermax-silent` recovers 168 (AC) and 278 (RS) **function** words that `-cc` leaves dark. That is
the only difference between them, so the subset of sentences whose changes are closed-class-only
is exactly the extra coverage, priced:

| subset of the sampled sentences | `ac-silent` dec | base : silent | p | `rs-silent` dec | base : silent | p |
|---|---|---|---|---|---|---|
| **only closed-class words newly lit** | **23** | **11 : 12** | **1** | **22** | **9 : 13** | **0.52** |
| only content words newly lit | 39 | 3 : 36 | 3.6e-08 | 42 | 3 : 39 | 5.6e-09 |
| a content word is newly lit (incl. mixed) | 54 | 6 : 48 | 3.3e-09 | 62 | 4 : 58 | 2.6e-13 |
| no content word is newly lit | 23 | 11 : 12 | 1 | 22 | 9 : 13 | 0.52 |

Read with the tie counts, the picture is unambiguous. Of the 150 sampled `ac-silent` sentences,
**53** whose only change is a newly lit function word are judged a **tie**, and the 23 decisive ones
split **11 : 12** — p = 1, the most exact null in this series. RS is the same: 54 ties and 9 : 13,
p = 0.52. Lighting a function word that was dark neither helps nor hurts; it adds noise to the
alignment and nothing else. Every decisive win the unrestricted variant has comes from the content
words it shares with `-cc`, and the closed-class half only dilutes it — which is why `ac-silent`
wins 77.9 % of a *smaller* decisive set while `ac-silentcc` wins 88.3 % of a larger one.

The same `word-change totals by outcome` cross-tab says it a second way: on `rs-silent`, the tie
bucket contains **74 closed_lit** word changes against 12 content_lit, while the arm's win bucket
contains 65 content_lit against 40 closed_lit.

### 5.2 What the 11–15 losses are, verbatim

Every base win on the `-cc` arms is the same thing: a recovery that is simply wrong — the
translation has no word for that source word and the aligner lights one anyway. These are the
16–18 % of `wrong` recoveries §6 measures, seen from the other side.

```
[RS 13] … overlaying the stereoscopic picture of buried masses on the normal view.
  CONTENT 'view': base (nothing) -> «картину»
  judge (both orders): "«картину» means 'picture', already claimed by «изображение»; base leaves
  'view' unmapped rather than misassign it"

[RS 15] The little spatula she had been using dropped …, beside the mukluks she wore on her feet.
  CONTENT 'wore': base (nothing) -> «пользовалась»
  judge: "«пользовалась» means 'was using' (the spatula), not 'wore'"

[RS 25] … trousers hooked into buccaneer-boots, his fingers aglint with jewels and metal.
  CONTENT 'aglint': base (nothing) -> «заправленные»
  judge: "«заправленные»→'hooked aglint' is stray drift; base keeps «заправленные»→'hooked' clean"

[AC 2]  May the road always rise to meet you,
  CONTENT 'meet': base (nothing) -> «под»
  judge: "«под» only means 'under/downhill'; 'meet' has no counterpart here"

[AC 41] The thick cropped hair was black shot through with grey.
  CONTENT 'grey': base (nothing) -> «чёрными»
  judge: "«чёрными» is 'black' only; 'grey' belongs with «проседью»"
```

### 5.3 Five verbatim, the arm better

```
[RS 1] The man fidgeted, drawing the collar of his jacket tighter around his neck.
  CONTENT 'drawing': base (nothing) -> «затягивая»
  judge (both orders): "«затягивая»→'drawing tighter' keeps the verb; base drops 'drawing'
  entirely, never assigned to any Russian word"

[RS 2] … the sides of each shaft brightly lit by the banks of floodlights …
  CONTENT 'banks': base (nothing) -> «рядами»
  judge: "«рядами»→'by the banks of' captures 'banks'; base's «рядами»→'by the of' drops it"

[RS 3] … it’s an order of magnitude above anything we’ve experienced before.
  CONTENT 'order', 'magnitude': base (nothing) -> «порядок»
  judge: "«порядок»→'order of magnitude' maps by meaning; base's «порядок»→'of' is a stray
  function word"

[AC 6] Beyond the wall I could see figures huddled against the expected grenade blast.
  CONTENT 'expected': base (nothing) -> «ожидании»
  judge: "«ожидании»→'the expected' captures 'expected' (anticipation); base leaves it unmapped"

[RS 4] “When we return, we can put a dome over the whole grid.
  CONTENT 'grid': base (nothing) -> «участок»
  judge: "the arm maps «участок»→'grid'; base leaves «участок» completely unmapped"
```

## 6. The newly lit words

**The set is D4b's set, byte for byte.** Every source word recovered here, and the fragment it
lights, is identical to what bare `itermax` recovered in `itermax-judge-2026-09-19.md` §4 — checked
by `(sentence index, source word index)`: 130 of 130 on AC and 177 of 177 on RS, with nothing on
either side the other lacks. The `-cc` set is the same set again (the population is content words by
construction), so the two variants share one judgement.

This pass therefore **replicates** D4b's measurement on the same items with a fresh judge run:

| | AC (n = 130) | RS (n = 177) |
|---|---|---|
| **D4d: correct** | **98 (75.4 %)** | **141 (79.7 %)** |
| D4d: partial | 8 (6.2 %) | 12 (6.8 %) |
| **D4d: wrong** | **24 (18.5 %)** | **24 (13.6 %)** |
| *D4b on the same words: correct* | *102 (78.5 %)* | *137 (77.4 %)* |
| *D4b: partial* | *7 (5.4 %)* | *19 (10.7 %)* |
| *D4b: wrong* | *21 (16.2 %)* | *21 (11.9 %)* |

Two independent runs of the same judge on the same 307 items agree to within 3 pp on `correct` and
2 pp on `wrong` — which is the only run-to-run stability estimate this series has for the
newly-lit instrument, and it is reassuring.

Restricted to the tap-ledger population (≥ 4 letters, ledger stoplist — the words behind the
2.21 → 0.71 % and 2.07 → 0.73 % headline):

| | AC (n = 101) | RS (n = 143) |
|---|---|---|
| correct | 76 (75.2 %) | 115 (80.4 %) |
| partial | 5 (5.0 %) | 9 (6.3 %) |
| wrong | 20 (19.8 %) | 19 (13.3 %) |

Pooled, **239 of 307 recoveries (77.9 %) are correct and 48 (15.6 %) are wrong.** The grid's
one-third kill threshold is not approached on either book, in either population, even counting
`partial` as failure (24.6 % and 20.3 %).

**The ~16–19 % of wrong recoveries is the honest cost of this change, and §5 prices it**: those are
precisely the sentences base wins, 11 and 15 of them in the decisive sets, against 83 and 115 the
arm wins. A learner who taps a word that used to light nothing now gets the right answer about
four times in five and a wrong one about once in six, where before they got nothing at all.

## 7. Decision

### SHIP `itermax-silent-cc`.

The kill criterion, applied exactly — *ship only if base takes at most 60 % of decisive pairs on
**both** books*:

| method | AC base share | RS base share | criterion |
|---|---|---|---|
| **`itermax-silent-cc`** | **11.7 %** | **11.5 %** | **passes — a fifth of the threshold** |
| `itermax-silent` | 22.1 % | 15.5 % | passes |
| *`itermax-guarded` (D4c)* | *68.3 %* | *57.3 %* | *failed* |
| *`itermax` bare (D4b)* | *70.3 %* | *67.3 %* | *failed* |

Both new methods clear it. **The recommendation is `itermax-silent-cc`**, on five measurements:

1. **It wins by more.** 88.3 % and 88.5 % of decisive pairs against 77.9 % and 84.5 %, on larger
   decisive sets (94 and 130 against 77 and 84).
2. **The coverage it gives up is worth nothing.** The 168 and 278 closed-class recoveries are the
   entire difference between the two methods, and on the sentences carrying only those, the judge
   splits **11 : 12 (p = 1)** and **9 : 13 (p = 0.52)** with 53 and 54 further sentences tied. That
   is not a small effect measured imprecisely; it is a precise null. The learner-facing metrics
   agree: content-word tap coverage is **0.9957 on both books for both methods**, because the
   metric counts content words, and so does the reader who taps one.
3. **Its tie rate is a third of the other's** (19.7 % / 13.3 % against 48.7 % / 44.0 %). Those ties
   are alignment churn that no instrument rewards — 74 closed-class word changes sit in `rs-silent`'s
   tie bucket alone.
4. **Half the blast radius.** 117 and 164 changed sentences against 226 and 336 — and against
   `itermax`'s 688 and 956. Fewer changed sentences at the same content coverage is strictly better:
   every changed sentence is a chance to be wrong.
5. **Lexcheck is flat.** RS is *exactly* base's 42 flags (0 new, 0 gone) and AC is +1, where the
   unrestricted variant is +2 on AC; all moved flags are recoveries, not regressions (§4).

The honest cost, stated plainly: **about one recovery in six is wrong** (§6, 18.5 % and 13.6 %),
and those wrong recoveries are what base wins its 11 and 15 decisive pairs on. The trade is 130 and
177 content words that lit nothing and now light the right thing four times in five, against a
sixth of them lighting the wrong thing — judged, on **224 decisive pairs** across two books
(198 : 26), at **88 : 12 in favour of making the trade**.

`itermax-silent` is not rejected — it passes the criterion too, and nothing here says it is harmful.
It simply buys nothing: the same content recoveries, plus a large block of function-word changes
that measure as noise.

### The shipping recipe

`EMBALIGN_METHOD=itermax-silent-cc` (equivalently `--embalign-method itermax-silent-cc`).
Recorded here in full so it can be applied without rediscovery. **None of it has been done in this
task; no default was changed.** Making a method the default for `emb` and `hybrid` is one default
value plus its documentation, because `llm` mode never starts the aligner and there is no per-mode
method setting:

| # | file | change |
|---|---|---|
| 1 | `internal/config/config.go:220` | `envOr("EMBALIGN_METHOD", "argmax")` → `envOr("EMBALIGN_METHOD", "itermax-silent-cc")` |
| 2 | `internal/embalign/embalign.go`, `Options.fill()` | `o.Method = "argmax"` → `o.Method = "itermax-silent-cc"`, so library callers (`cmd/alignbench`) match the CLI |
| 3 | `internal/embalign/embalign_test.go`, `TestStartChildArgs` | the `default` row's expected child command line gains `--methods itermax-silent-cc` (and `--src-lang` stays absent, since `Options{}` sets no source language) |
| 4 | `.env.example` | a commented `EMBALIGN_METHOD=` line next to `ALIGN_MODE`, naming `argmax` as the way back |
| 5 | `README.md:454` | "mutual argmax" in the *How it works* paragraph becomes "mutual argmax plus silent-word recall" |

Two things the change does **not** need, both verified here: no Go-side allowlist (the method
string reaches the child verbatim), and no cache or format bump (alignment is recomputed from the
cached pass-1 translations; `manifest.meta.runs[].options.embMethod` records the new value by
itself).

`tools/embalign.py` itself needs **no** default change: `--methods` defaults to `argmax` and the Go
side always passes the method explicitly, so the Python default is dead weight either way. What the
Python file does carry, and what must keep working, is `NEEDS_STOP` — the list of method values that
load `tools/stopwords.json`; `itermax-silent-cc` is in it, and a source language absent from that
file logs `no list for source language … — the guard is a no-op and this run is plain itermax` and
degrades to plain `itermax-silent` rather than silently shipping an unfrozen arm.

**Tests that cover the change.** `internal/embalign.TestStartChildArgs` pins the child command line
and would fail on the new default until its `default` row is updated — that is the test doing its
job, and updating it is step 3. `internal/stopwords.TestExportedFileMatches` keeps
`tools/stopwords.json` in sync with the Go lists in both directions, which is what the `-cc` freeze
reads. `internal/cli.TestRunMeta*` assert `manifest.meta.runs[].options.embMethod`, so a changed
default is recorded in every produced book without further work. Beyond the Go suite, the two
Python-level checks this pass used should be kept as the regression guard for the method itself:
`match_pairs` under `argmax`/`itermax` must stay identical to the pre-D4c file, and no source word
`argmax` lights may change under either SILENT method (§2).

**What a reader sees differently.** Nothing they already had moves. Every word that lit something
before lights exactly the same fragment, so no tap the reader has learned to trust changes its
answer, no highlight grows an extra word, and no function word jumps to an unrelated one — the
three failure shapes D4b and D4c measured are structurally impossible here (§3: 0 violations on
14 143 and 21 245 word taps). What changes is that 130 and 177 content words per book that answered
a tap with silence now answer it, correctly about four times in five. In tap-ledger terms the share
of content-word taps that light nothing falls from 2.21 % to 0.71 % on *Altered Carbon* and 2.07 %
to 0.73 % on *Revelation Space*, while `smear` — the "too much lit" failure — does not move at all
(0.83 → 0.84 and 1.04 → 1.04). The remaining visible cost is that roughly one in six of those newly
answered taps shows a word that does not translate the tapped one, where previously it showed
nothing.

## 8. Artifacts

- Arms: `bench-quality/.artifacts/align-grid/{ac,rs}-{silent,silentcc}.{tbook,log,wall}` plus their
  `.tbook.lexflagged.json` and glossary sidecars.
- The **rejected matching-stage build** and its structural check, kept as the evidence behind §1.2:
  `.artifacts/itermax-silent/preglue/`.
- Default-identity arm: `.artifacts/align-grid/rerun/ac-base-d4d.{tbook,log,wall}`; proof line in
  `.artifacts/itermax-silent/default-identity.txt`.
- Offline proof: `.artifacts/itermax-silent/offline-proof.txt` (`offline-probe.py`).
- Structural check: `.artifacts/itermax-silent/structural-check.txt`,
  `silentcheck-{ac,rs}-{silent,silentcc}.json` and `…-offenders.json`.
- Scores: `.artifacts/align-grid/scores/{score,tl,tlc,diff,probe}-{ac,rs}-{silent,silentcc}.*`;
  table rows in `.artifacts/itermax-silent/grid-rows.txt`; gate shares in `gated.txt`; lexcheck
  deltas in `lexflag-delta.txt`.
- Judging sets, verdicts and summaries: `.artifacts/itermax-silent/{ac,rs}-{silent,silentcc}/`
  (`changed.json`, `key.json`, `wordchanges.json`, `lit.json`, `batch-*-o{1,2}.json`,
  `verdict-*.json`, `notes-*.json` with the judges' one-line reasons, `lit-*.json`,
  `litverdict-*.json`, `summary-pairs.json`, `summary-lit.json`).
- Analyses: `final-verdicts.txt` (all four arms, both modes), `subset-analysis.txt` (the
  cross-tabs and sign tests, including the closed-class-only split), `lit-breakdown.txt`
  (newly-lit correctness, all words and tap-ledger subset), `wordclass-corpus.txt`,
  `examples-{ac,rs}-{silent,silentcc}.txt` and `litexamples-{ac,rs}.txt` (verbatim blocks).
- The newly-lit verdicts are one judgement shared by both variants of a book (the sets are
  byte-identical); `{ac,rs}-silentcc/litverdict-*.json` are copies of `{ac,rs}-silent/`'s.
- Scripts: `bench-quality/itermax_silentcheck.py` (new, the D4d contract check); D4b/D4c's
  `itermax_{prep,wordchanges,judge,analyze,subsets,examples,litexamples,aligndiff,guardrow,lexdelta}.py`
  reused unchanged. Run scripts and the pinned binary in `.artifacts/itermax-silent/`.

### Reproducing

```sh
cd converter
G=bench-quality/.artifacts/align-grid
O=bench-quality/.artifacts/itermax-silent

sh $O/run-ac.sh && sh $O/run-rs.sh     # arms + the argmax identity arm, offline
python3 bench-quality/itermax_aligndiff.py $G/ac-base.tbook $G/rerun/ac-base-d4d.tbook
python3 bench-quality/itermax_silentcheck.py $G/ac-base.tbook $G/ac-silentcc.tbook \
    --itermax $G/ac-itermax.tbook --book ac --arm-name silentcc --out $O
sh $O/score-all.sh && sh $O/prep-all.sh && sh $O/judge-all.sh && sh $O/analyze-all.sh
```

Each arm needs its own cache holding **only** pass-1 translations; a cache that already has the
book's final aligned entries makes the run a no-op and the arm a copy of the previous one.
