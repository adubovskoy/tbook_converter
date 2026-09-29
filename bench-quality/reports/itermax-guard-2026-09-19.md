# The closed-class guard, built and judged — 2026-09-19/20

Plan item **D4c**: build the guard that `itermax-judge-2026-09-19.md` §6 proposed, measure it
offline, and re-judge it blind against the shipped `argmax` baseline under that report's kill
criterion.

**The guard works and it is not enough.** Built as specified, it removes D4b's losing mechanism
completely — **0 closed-class source words** move to a disjoint fragment on either book, against
466 and 711 under bare `itermax` — and it keeps three quarters of the coverage gain (content-word
tap coverage 0.9784 → **0.9916** on AC, 0.9817 → **0.9930** on RS, against `itermax`'s 0.9957).
Judged blind in both orders on 300 changed sentences, base still wins **71 : 33 on *Altered
Carbon* (68.3 %, p = 2.5e-04)** and 67 : 50 on *Revelation Space* (57.3 %, p = 0.14). D4b's kill
criterion — *base above 60 % of decisive pairs on either book* — is met on AC, so **`itermax` does
not ship in any form, guarded or bare**, and `EMBALIGN_METHOD` stays at `argmax`.

The reason is a **second mechanism D4b never separated**: `itermax`'s second round also *widens*
content words, attaching a word that already has a correct highlight to a second, unrelated
translation word. Split this pass's sample on it and base wins those sentences **52 : 1** and
**53 : 5** — while on the rest the guard **wins**, 32 : 19 and 45 : 14. The same split applied to
D4b's own verdicts shows the effect was already there (19 : 1, 23 : 3). §8 names the variant that
would cut both mechanisms and keep the whole coverage gain.

---

## 1. What was built

### 1.1 The method value

The guard is an **opt-in third value of the existing matching-method knob**, not a flag on
`itermax`:

```
EMBALIGN_METHOD=itermax-guarded        # == --embalign-method itermax-guarded
```

`tools/embalign.py` gained `GUARDED = "itermax-guarded"` and, in `match_pairs`, one branch that
runs **both** matchers on the same similarity matrix and merges them per source word:

```python
if method == GUARDED:
    local = guard_closed_class(
        to_word_pairs(argmax_pairs(sim), s_wids, t_wids),
        to_word_pairs(itermax_pairs(sim), s_wids, t_wids),
        src_words or [], stop or set())
else:
    sub = itermax_pairs(sim) if method == "itermax" else argmax_pairs(sim)
    local = [[s, t] for s, t in to_word_pairs(sub, s_wids, t_wids)]
```

`guard_closed_class` walks the union of source-word indices that either matcher touched and, for
each, keeps **argmax's target set when the source word is closed class and itermax's otherwise**.
The merged set is then handed to `glue_idioms`, `glue_target`, the request-index mapping and
`glue_units` **unchanged** — exactly the wording of D4b §6 ("feed the union to the existing
target-glue and chunk-grouping steps unchanged"). §3 measures what that decision costs.

### 1.2 Where the closed-class list comes from

`internal/stopwords` is a Go package and the aligner is a Python child process, so the list is
exported rather than duplicated:

- `cmd/stopwordsdump` — `go run ./cmd/stopwordsdump -o tools/stopwords.json` writes every
  language's list as sorted JSON (11 languages, 17 KB).
- `tools/stopwords.json` — the checked-in copy the aligner reads, lazily, **only** when the
  guarded method is selected.
- `internal/stopwords.Langs()` — new accessor the dumper needs.
- `internal/stopwords.TestExportedFileMatches` — fails if the file and the Go lists disagree in
  either direction (missing language, extra language, missing word, extra word).

The lookup mirrors `cmd/score`'s own source-content rule exactly: `strings.ToLower(word)` against
`stopwords.Set(source)` becomes `word.lower()` against the exported set, with no length rule and
no punctuation stripping. English ships 125 words.

### 1.3 Getting the source language to the child

The method string already reached the child verbatim — `internal/embalign` passes
`Options.Method` straight through to `--methods` and has **no allowlist**, so `itermax-guarded`
needed nothing "allowed" there. What it did need is the *source language*, which the child had no
way to know. Minimal wiring:

- `embalign.Options.SrcLang` (new field) → `--src-lang <code>`, appended **only when non-empty**,
  so the default child command line is byte for byte what it was.
- `internal/cli/cli.go` sets `SrcLang: cfg.Source` when it starts the aligner.
- `tools/embalign.py` takes `--src-lang` (default `$EMBALIGN_SRC_LANG`, else `en`).
- `TestStartChildArgs` (new) pins both command lines:
  `--serve --model … --layer 8 --methods argmax` for the default and
  `… --methods itermax-guarded --src-lang de` for the guarded case.

A language with no list logs `no list for source language … — the guard is a no-op and this run
is plain itermax` and degrades to bare `itermax` rather than silently guarding nothing.

### 1.4 Branch logging

The guarded child prints a counter line to stderr (the converter's log) every 500 sentences and
once at EOF. From the two arms:

```
ac-guard: closed-class guard (total): 1277 sentences, 5695 closed-class source words frozen to
          argmax (1007 of them differed from itermax), 6994 content source words taken from
          itermax (613 of them differed from argmax)
rs-guard: closed-class guard (total): 1566 sentences, 8575 closed-class source words frozen to
          argmax (1563 of them differed from itermax), 10665 content source words taken from
          itermax (817 of them differed from argmax)
```

(The counters see only source words that at least one matcher paired, before glue; §3's corpus
figures are over every source word after the whole pipeline.)

## 2. Default identity: the proof C4 needs

The constraint on this pass was that another agent is running a **frozen** converter binary that
executes `tools/embalign.py` at runtime, so the default (`argmax`) behaviour had to stay
byte-identical. Two independent checks:

**2.1 Module-level, over random similarity matrices.** The pre-change file was kept and both
modules loaded side by side; `match_pairs` was called on 300 random encoded pairs (random
subword counts, random word groupings) for `argmax` and for `itermax`:

```
default-path mismatches over 300 random pairs x 2 methods: 0
guard semantics ok: True
```

**2.2 End-to-end, a whole book.** `ac-base` was re-run under the grid's own offline recipe with
the **new** `tools/embalign.py` and a **newly built** converter (which now also passes
`--src-lang en`), into `align-grid/rerun/ac-base-d4c.tbook`, and compared with the grid's
`ac-base.tbook` — produced hours earlier by the *old* script and the *old* binary:

```
ac-base.tbook vs ac-base-d4c.tbook: 0 / 1303 sentences with a different alignment chunk list;
0 translation texts differ; chapter payloads byte-identical: True
```

**Byte-identical chapter payloads** is stronger than the "0 changed sentences" the task asked
for: every offset, every chunk, every `q` in every chapter JSON of the archive is the same file
content. `cmd/tbookdiff` agrees (`Δmean +0.000` on all five coverage means, 0 regressed, 0
improved). The arm also reproduced the baseline's lexcheck count exactly (30 flags), which is the
one metric produced by the converter rather than by re-scoring.

So `EMBALIGN_METHOD=argmax` — everything C4 is running — is unaffected by this pass.

## 3. The structural check

D4b §6 step 1 asked to confirm that, on the guarded arm, **closed-class source words carry
exactly `argmax`'s pairs and content words exactly `itermax`'s**. `bench-quality/itermax_guardcheck.py`
checks that for every source word of every translated sentence of both books — 14 143 (AC) and
21 245 (RS) word taps, the same populations the tap ledger counts.

| | AC closed | AC content | RS closed | RS content |
|---|---|---|---|---|
| source words | 6 791 | 7 352 | 10 171 | 11 074 |
| identical to the arm it must equal (base / itermax) | **6 706 (98.75 %)** | **7 222 (98.23 %)** | **10 067 (98.98 %)** | **10 897 (98.40 %)** |
| differ — widened | 0 | 97 | 0 | 141 |
| differ — narrowed | 69 | 0 | 85 | 0 |
| differ — newly lit | 16 | 0 | 19 | 0 |
| differ — darkened | 0 | 33 | 0 | 36 |
| **differ — moved to a disjoint fragment** | **0** | **0** | **0** | **0** |

**The headline the check was built for holds: 0 disjoint jumps, on either class, on either book.**
Not one closed-class source word lights a fragment disjoint from what `argmax` gave it — which is
precisely the mechanism D4b measured base winning 47 : 7 and 48 : 13 on. Nor does any source word
of either class go from lit to silent against base (0 and 0).

**The literal claim — "exactly argmax's pairs" — does not hold, and the residue is instructive.**
1.25 % of closed-class words and 1.77 % of content words differ from the arm they were supposed
to equal, and every one of those differences is produced by the **glue steps, which D4b's guard
deliberately left unchanged**. The merge happens after matching and before glue; glue's decisions
depend on a word's *neighbours*, and the guard changes the neighbours:

- `closed / narrowed` (69, 85) — in the base arm `glue_target` attached an otherwise-uncovered
  translation word to a function word; under the guard that translation word is already claimed by
  a content word's `itermax` pair, so the mirror step never fires and the function word lights
  less. Example (AC): `what` lights «то что» under base and «что» under the guard.
- `closed / lit` (16, 19) — `glue_idioms` attaches a function word `argmax` left dark, landing on
  the same fragment `itermax` chose. These are D4b §5.2's genuine closed-class wins (`before` →
  «до», `is` → «что») surviving the freeze by the back door.
- `content / widened` (97, 141) and `content / darkened` (33, 36) — **every one of these is a
  reversion to base, not an invention.** Checked directly: 130 of 130 content deviations on AC and
  175 of 177 on RS have `guard == base` exactly; only 2 words in the whole of RS land somewhere
  that is neither arm's answer. A content word whose `itermax` highlight was produced by a *glue*
  step anchored on a function word's `itermax` pair loses it when that pair is frozen, and falls
  back to what `argmax` gave it — dark if `argmax` had it dark (`expected` → «ожидании», one of
  D4b §5.3's showcase recoveries, is in this set), wider if `argmax` had it wider (AC: `published`
  lights «выходом печать» under `itermax` and «в выходом печать» under base and the guard).

So the guarded arm is strictly *between* base and `itermax` on both classes: on closed-class words
it is base's answer or (16 / 19 times) `itermax`'s; on content words it is `itermax`'s answer or
(130 / 175 times) base's. Nothing in it is new. But D4b §6's claim that the coverage gain is
"preserved by construction" is **false as stated** — §4 measures how much of it actually survives.
The guard is not a pure subset operation: freezing a function word changes what the unchanged glue
steps do around it.

## 4. Scores: the guard on the grid's five instruments

Same five instruments, same invocations, same offline recipe as `align-knob-grid-2026-09-19.md`
§9 (`.cache-d4` seeded with pass-1 translations only, `--align-mode emb`, `unshare -r -n`,
`GONKA_BASE_URL=http://127.0.0.1:9/v1`, `HF_HUB_OFFLINE=1`). The base and `itermax` rows below were
**recomputed from the arms on disk with today's code**, not copied from the grid, and they
reproduce the grid's published values to the last digit — which is what makes the new row
comparable.

### Table A

| arm | knob | wall s | target cov all / content | source tap cov all / content | sentences tap < 0.80 | lexcheck flags | alignments changed vs base |
|---|---|---|---|---|---|---|---|
| `ac-base` | defaults: argmax | 265 | 0.9919 / 0.9930 | 0.9722 / **0.9784** | 30 / 1069 | 30 | 0 / 1279 |
| `ac-itermax` | `METHOD=itermax` | 520 | 0.9964 / 0.9969 | 0.9922 / **0.9957** | 4 / 1069 | 33 | 688 / 1279 |
| **`ac-guard`** | **`METHOD=itermax-guarded`** | **39** | **0.9939 / 0.9948** | **0.9808 / 0.9916** | **8 / 1069** | **31** | **321 / 1279** |
| `rs-base` | defaults: argmax | 335 | 0.9915 / 0.9929 | 0.9728 / **0.9817** | 24 / 1413 | 42 | 0 / 1566 |
| `rs-itermax` | `METHOD=itermax` | 684 | 0.9958 / 0.9972 | 0.9919 / **0.9957** | 3 / 1413 | 38 | 956 / 1566 |
| **`rs-guard`** | **`METHOD=itermax-guarded`** | **141** | **0.9933 / 0.9949** | **0.9797 / 0.9930** | **7 / 1413** | **41** | **436 / 1566** |

### Table B

| arm | clean | clean-multi | shared | smear | none | content-word silent | content-only clean / shared / none | probe unit % | probe word cov % |
|---|---|---|---|---|---|---|---|---|---|
| `ac-base` | 51.33 | 3.92 | 40.81 | 0.83 | 3.10 | **2.21** | 57.93 / 33.40 / 2.21 | 77.1 | 93.0 |
| `ac-itermax` | 54.66 | 2.86 | 40.69 | 0.79 | 1.00 | **0.62** | 56.83 / 37.33 / 0.62 | 82.4 | 97.5 |
| **`ac-guard`** | **51.10** | **3.56** | **42.11** | **0.93** | **2.31** | **1.11** | **56.36 / 36.23 / 1.11** | **79.4** | **95.2** |
| `rs-base` | 51.22 | 4.48 | 40.15 | 1.04 | 3.11 | **2.07** | 58.20 / 32.59 / 2.07 | n/a | n/a |
| `rs-itermax` | 54.80 | 3.40 | 39.91 | 0.92 | 0.97 | **0.58** | 57.35 / 36.20 / 0.58 | n/a | n/a |
| **`rs-guard`** | **50.87** | **4.23** | **41.46** | **1.08** | **2.36** | **0.95** | **56.67 / 35.33 / 0.95** | n/a | n/a |

### What the row says

| | AC base → guard → itermax | RS base → guard → itermax |
|---|---|---|
| content-word tap coverage | 0.9784 → **0.9916** → 0.9957 | 0.9817 → **0.9930** → 0.9957 |
| share of `itermax`'s gain captured | **+1.32 of +1.73 pp = 76 %** | **+1.13 of +1.40 pp = 81 %** |
| content-word taps lighting nothing | 149 → **75** → 42 | 220 → **101** → 62 |
| source words silent → lit (`cmd/score` list) | 0 → **97** → 130 | 0 → **141** → 177 |
| source words lit → silent | 0 → **0** → 0 | 0 → **0** → 0 |
| sentences with content tap coverage < 0.80 | 30 → **8** → 4 | 24 → **7** → 3 |
| lexcheck flags | 30 → **31** → 33 | 42 → **41** → 38 |
| sentences whose alignment changed vs base | 0 → **321** → 688 | 0 → **436** → 956 |
| hybrid gate q < 0.9 | 2.3 % → **1.8 %** → 0.9 % | 2.5 % → **1.9 %** → 1.1 % |

Three things to read off it.

1. **The coverage gain is mostly, not wholly, preserved.** The expectation stated in the task —
   "coverage gain ≈ itermax's (the recovered words are all content words)" — is **not met**: the
   guard delivers 76 % (AC) and 81 % (RS) of it. §3 names the mechanism: 33 and 36 content words
   owed their `itermax` highlight to a glue step anchored on a function word's `itermax` pair.
2. **The lexcheck expectation is half met.** `flags ≤ base` holds on RS (42 → 41) and fails by one
   on AC (30 → 31). The delta is tiny and legible (`itermax_lexdelta.py`): AC gains 2 and loses 1,
   RS gains 1 and loses 2. Of AC's two new flags one is a genuine closed-class-adjacent regression
   (`own`: «собой» → «с», in a sentence where `my` also narrows) and one is a **recovery**
   (`feeling`/`like` → «хотелось», both previously dark) that lexcheck dislikes because the
   recovered pair has thin dictionary support — the grid's §4.3 warning that "flag count falls when
   an arm aligns *less*" cuts both ways.
3. **`tbookdiff` finds no regression at all**: 0 sentences regressed >0.1 on content coverage on
   either book, 6 (AC) and 8 (RS) improved, translation text changed 0 / 1303 and 0 / 1575.
   The change surface is roughly half `itermax`'s: 321 vs 688 sentences on AC, 436 vs 956 on RS.

Wall times are not comparable across rows — the grid's were measured under concurrent load, and
the guarded arms ran on a near-idle machine. The calibration is the identity arm: `ac-base-d4c`,
the *baseline* method under this pass's conditions, took **116 s** against `ac-base`'s recorded
265 s, so `ac-guard`'s 39 s is a machine difference, not a method difference. The guard itself
costs one extra `argmax` pass over an already-computed similarity matrix, which is not measurable
next to the encoder.

### 4.1 The shortfall is fixable, and the fix is a different guard

D4b §6 specified the merge **before** glue ("feed the union to the existing target-glue and
chunk-grouping steps unchanged"), and that is what was built and judged. The alternative is to run
the whole pipeline twice — both glue chains — and merge **after**: per source word, base's final
pairs if it is closed class, `itermax`'s final pairs otherwise. That variant has the guard's
contract exactly, by construction: closed-class words get base's highlights and nothing else,
content words get `itermax`'s and nothing else, so 0 deviations of either kind.

It can be evaluated without building it, because the quantity in question is a function of the two
arms already on disk. `cmd/score`'s source content-word tap coverage was re-implemented in Python
and checked against the shipped tool on all six arms — it reproduces every published figure to
four decimals — then applied to the synthesized post-glue merge
(`.artifacts/itermax-guard/postglue-variant.txt`):

| source tap coverage, all / content | AC | RS |
|---|---|---|
| base | 0.9722 / 0.9784 | 0.9728 / 0.9817 |
| `itermax` | 0.9922 / 0.9957 | 0.9919 / 0.9957 |
| guard, match-stage merge (**built and judged here**) | 0.9808 / 0.9916 | 0.9797 / 0.9930 |
| guard, post-glue merge (*synthesized*) | — / **0.9957** | — / **0.9957** |
| *silent-content-only* (*synthesized*, see §8) | 0.9816 / **0.9957** | 0.9801 / **0.9957** |

So the 76 % / 81 % shortfall is an artefact of where the merge sits, not of the guard idea, and a
post-glue merge would recover all of it at the cost of a second glue pass (cheap: the glue steps
are numpy loops over an already-computed similarity matrix). **Neither synthesized row is what this
report's verdicts are about** — the post-glue merge lights 130 and 177 content words where the
judged arm lights 97 and 141, so it would need its own judging pass. They are here because §8's
recommendation has to say what to do next, and because §6 turns out to make the post-glue merge the
*wrong* next thing.

## 5. What the guard changes, counted before it is judged

Every source word whose tap lights a different fragment under the guard than under base
(`bench-quality/itermax_wordchanges.py`, the same instrument D4b §2 used), with D4b's `itermax`
column next to it:

| | AC guard | AC itermax | RS guard | RS itermax |
|---|---|---|---|---|
| changed sentences | **321** | 688 | **436** | 956 |
| word changes | **506** | 1 448 | **707** | 2 199 |
| content words touched | 420 | 420 | 600 | 599 |
| closed-class words touched | **86** | 898 | **107** | 1 423 |
| closed-class: silent → lit | 16 | 168 | 19 | 278 |
| closed-class: widened | 0 | 0 | 0 | 0 |
| closed-class: narrowed | 69 | 255 | 85 | 405 |
| closed-class: shifted | 1 | 7 | 3 | 20 |
| **closed-class: moved to a disjoint fragment** | **0** | **466** | **0** | **711** |
| content: silent → lit | 97 | 130 | 141 | 177 |
| **content: widened (keeps its target and gains another)** | **174** | 174 | **260** | 259 |
| content: narrowed | 64 | 161 | 101 | 240 |
| content: moved to a disjoint fragment | 83 | 83 | 87 | 87 |
| sentences changing closed-class words only | **6** | (67 of 150 sampled) | **6** | (68 of 150 sampled) |
| sentences with a newly lit content word | 88 | — | 129 | — |

This is the guard doing exactly the job it was designed for, before any judge sees it. The class of
change D4b measured base winning **47 : 7** and **48 : 13** on — a function word jumping to an
unrelated translation word — is **gone entirely: 466 → 0 and 711 → 0**. The class D4b measured
`itermax` winning **12 : 4** and **18 : 5** on — a content word recovered — is almost intact
(130 → 97, 177 → 141), and the content words that merely *move* are untouched (83 → 83, 87 → 87).
The sentences that changed nothing but function words, which base took 29 : 7 and 30 : 8, fall from
about 45 % of the changed population to **6 sentences per book**.

What remains to be judged is therefore a different population from D4b's: 321 and 436 sentences
that are overwhelmingly content-word changes — and the biggest single class in it is not the
recovery the guard was adopted for (97 / 141 words) but **`itermax` widening a content word that
already had a highlight** (174 / 260 words), which the guard leaves entirely alone. §6 is about
that row.

## 6. Verdicts: base vs the guard

D4b's protocol, unchanged: 150 changed sentences per book by the corrected even stride
(`k → floor(k·L/n)`, `L` = 321 / 436, so stride 2.14 and 2.91, spanning document positions 2–1 293
and 1–1 567), both presentation orders judged by a fresh `claude -p --model sonnet --tools ""`
process in batches of 10 with `--jobs 5`, de-swapped, **decisive only when the two orders agree**.
The same scripts (`itermax_prep.py`, which gained `--b-label` and nothing else,
`itermax_wordchanges.py`, `itermax_judge.py`, `itermax_analyze.py`).

**Judge-call bookkeeping.** 85 batch judgements on disk: 60 pairwise (15 batches × 2 orders × 2
books = 600 pair-order judgements) and 25 newly-lit (10 AC + 15 RS = 238 word judgements), with no
missing verdicts anywhere. Two honest notes on the harness. One RS newly-lit batch (`lit-13`)
returned an object with an id missing on all three attempts of one runner and was written by the
other; and because the AC and RS newly-lit queues were started twice to shorten the tail, **some
RS newly-lit batches were judged by two independent processes and the file on disk is the later
writer's** — one verdict per item either way, but not a single-draw sample for those batches. The
pairwise judging, which every claim in §6 rests on, was not affected: it ran once, in order, with
a fresh process per batch. No API other than the `claude` CLI was used, no `.env` was read, and
nothing but the local `.tbook` files was consumed.

| | *Altered Carbon* | *Revelation Space* |
|---|---|---|
| sampled changed sentences | 150 | 150 |
| judged (no missing verdicts) | 150 | 150 |
| explicit ties | 38 | 28 |
| order-inconsistent (counted as ties) | 8 | 5 |
| **decisive** | **104** | **117** |
| **base (`argmax`) wins** | **71 (68.3 %)** | **67 (57.3 %)** |
| **guard wins** | **33 (31.7 %)** | **50 (42.7 %)** |
| sign test, two-sided | **p = 2.47e-04** | **p = 0.139** |

Against D4b's unguarded arm (71 : 30 and 68 : 33, both p < 1e-3), the guard moves the needle in the
right direction and does not move it far enough. RS is now a statistical tie; **AC is still a loss
at 68.3 %, above the 60 % kill threshold D4b set.**

### 6.1 The decomposition — and the mechanism D4b missed

| subset of the 150 sampled sentences | AC dec | AC base : guard | AC p | RS dec | RS base : guard | RS p |
|---|---|---|---|---|---|---|
| all | 104 | **71 : 33** | 2.5e-04 | 117 | **67 : 50** | 0.139 |
| a closed-class word moved to a **disjoint** fragment | **0** | — | — | **0** | — | — |
| only closed-class words changed at all | 0 | — | — | 2 | 1 : 1 | 1 |
| only content words changed at all | 76 | 57 : 19 | 1.5e-05 | 89 | 61 : 28 | 6.1e-04 |
| a content word is newly lit | 29 | **8 : 21** | 0.024 | 43 | **5 : 38** | 2.5e-07 |
| no content word is newly lit | 75 | 63 : 12 | 1.7e-09 | 74 | 62 : 12 | 2.9e-09 |
| a content word moved to a disjoint fragment | 30 | 22 : 8 | 0.016 | 20 | 12 : 8 | 0.50 |
| **a content word WIDENED (kept its target, gained another)** | **53** | **52 : 1** | **1.2e-14** | **58** | **53 : 5** | **3.5e-11** |
| **no content word widened** | **51** | **19 : 32** | **0.092** | **59** | **14 : 45** | **6.5e-05** |

Two rows carry everything.

1. **The guard did its job.** The subset D4b measured base winning 47 : 7 and 48 : 13 on — a
   function word jumping to a disjoint fragment — **does not exist** in either sample, because it
   does not exist in the corpus (§3, §5). The closed-class mechanism is closed. So is
   "closed-class-only sentences", which base took 29 : 7 and 30 : 8 in D4b: 0 and 2 decisive
   sentences here.
2. **A second, independent mechanism was underneath it, and D4b never separated it.** Split the
   sample on whether `itermax`'s second round **widened** a content word — left its existing
   highlight in place and attached a *second* target word to it — and base wins **52 : 1** and
   **53 : 5**. Take those sentences out and the guard **wins**: 32 : 19 (AC, p = 0.09) and
   45 : 14 (RS, p = 6.5e-05).

The same split applied to **D4b's own verdicts** shows the mechanism was already there and already
significant, hidden under the closed-class effect that dominated its sample:

| | AC base : itermax | AC p | RS base : itermax | RS p |
|---|---|---|---|---|
| a content word widened | **19 : 1** | 4.0e-05 | **23 : 3** | 8.8e-05 |
| no content word widened | 52 : 29 | 0.014 | 45 : 30 | 0.105 |

D4b's §5 asked "did a *closed-class* word jump?" and got a clean answer, so it stopped there. Ask
"did a *content* word widen?" of the same data and the answer is just as clean. The guard fixes the
first question and leaves the second untouched, which is exactly what §5's count predicted: the
guard keeps all 174 (AC) and 260 (RS) content-word widenings.

### 6.2 What a widening looks like

The failure is always the same shape: the source word already has a correct highlight, and the
second matching round attaches it to a second translation word that does not translate it. The
learner taps one word and two unrelated words light.

```
[RS 2] The man’s voice was muffled behind his breather mask.
  TR   Голос человека был приглушён за его дыхательной маской.
  CONTENT 'man’s': base «человека»  -> guard «человека приглушён»  (widened)
  judge (both orders): "«приглушён»→'man's muffled' wrongly reattaches 'man's' (already covered by
  «человека»); the plain «приглушён»→'muffled' is correct"

[RS 5] “Speak for yourself,” Sylveste said, noticing the way the man’s gaze snapped …
  CONTENT 'man’s': base «мужчины» -> guard «мужчины метнулся» (widened)
  judge: "«метнулся»→'man's snapped' wrongly reattaches 'man's' (already on «мужчины»)"

[AC 7] “Don’t give me that shit.
  CONTENT 'Don’t': base «Не» -> guard «Не дерьма» (widened)
  judge: "«дерьма»→'Don't shit' wrongly reattaches 'Don't' (already «Не») to an unrelated word"

[AC 8] “A little something with your phallic substitute, ma’am?”
  CONTENT 'ma’am': base «мэм» -> guard «суррогату мэм» (widened)
  judge: "«суррогату»→'substitute ma'am' wrongly tacks unrelated 'ma'am' onto 'substitute'"

[AC 9] The commando that came through the ruined wall was stocky and insect-eyed in full gas
       attack rig, hefting a snub-barrelled Kalashnikov …
  CONTENT 'insect-eyed':   base «глазами насекомого» -> guard «коренастый глазами насекомого»
  CONTENT 'snub-barrelled': base «укороченный»       -> guard «противогазном укороченный»
  judge: "wrongly attaches unrelated words: «коренастый»→'stocky insect-eyed' and
  «противогазном»→'gas attack snub-barrelled'"
```

### 6.3 Five verbatim, the guard better

```
[AC 12] The commando kept his aim low, holding the Kalashnikov down against the … recoil …
  CONTENT 'holding': base (nothing) -> guard «держал прижимая»
  judge (both orders): "base drops «прижимая» entirely from the alignment; the guard correctly
  pairs «прижимая»→'holding'"

[AC 18] Out in the Reach power lashed down from an orbital and lit the kitchen in tones of blue.
  CONTENT 'Out': base (nothing) -> guard «Там»
  judge: "base omits «Там»→'Out' entirely, leaving it unmapped"

[AC 29] I pressed one nostril closed and snorted tank gel out of the other.
  CONTENT 'pressed': base (nothing) -> guard «зажал»
  judge: "base's «зажал»→'nostril' is a category mismatch (verb mapped to noun); the guard's
  «зажал»→'pressed closed' is correct"

[RS 21] That’s why we’re here, breaking our backs for you.
  CONTENT 'backs': base (nothing) -> guard «вкалываем»
  judge: "«вкалываем»→'we're breaking our backs' is complete; base drops 'backs'"

[RS 22] Sluka pushed past him to ascend the ladder up the side of the pit.
  CONTENT 'pushed': base (nothing) -> guard «прошла»
  judge: "«прошла»→'pushed' is the verb match; base wastes it on 'past' (already covered by
  «мимо»), leaving 'pushed' unaligned"
```

Every one of the guard's wins is a **recovery** — a word that lit nothing now lights the right
thing. Every one of base's wins is a **widening** or a content-word jump. That is the whole
result in one sentence.

## 7. The newly lit words

The guard's recoveries, judged the same way D4b judged `itermax`'s: not blind (there is no rival
fragment — the word lit nothing), one call per 10 items, `correct | partial | wrong` against the
same j3 criterion. Every source word that **no chunk referenced under base and at least one chunk
references under the guard**, by `cmd/score`'s content rule.

| | *Altered Carbon* (n = 97) | *Revelation Space* (n = 141) |
|---|---|---|
| **correct** | **73 (75.3 %)** | **114 (80.9 %)** |
| partial | 8 (8.2 %) | 9 (6.4 %) |
| **wrong** | **16 (16.5 %)** | **18 (12.8 %)** |
| *D4b, `itermax`, for comparison* | *102 / 130 = 78.5 % correct, 16.2 % wrong* | *137 / 177 = 77.4 % correct, 11.9 % wrong* |

Restricted to the tap-ledger population (≥ 4 letters, ledger stoplist — the words behind the
2.21 → 1.11 % and 2.07 → 0.95 % headline):

| | AC (n = 74) | RS (n = 117) |
|---|---|---|
| correct | 56 (75.7 %) | 96 (82.1 %) |
| partial | 6 (8.1 %) | 9 (7.7 %) |
| wrong | 12 (16.2 %) | 12 (10.3 %) |

Pooled, **187 of 238 recoveries (78.6 %) are correct renderings and 34 (14.3 %) are wrong** —
statistically indistinguishable from D4b's 78.5 % / 77.4 %, on a set that is a strict subset of
D4b's. The grid's one-third kill threshold is not approached on either book, in either population,
even counting `partial` as failure (24.7 % and 19.1 %).

The guard's recovery set being smaller is **not** a quality filter. Cross-referencing D4b's
verdicts on the same words (`lit-crossref-d4b.txt`): of `itermax`'s 130 AC recoveries the guard
keeps 97 and drops 33, and D4b had judged the *dropped* ones slightly **better** (81.8 % correct
vs 77.3 % for the kept). RS is flat (77.8 % vs 77.3 %). The 33 and 36 words the guard loses are
ordinary recoveries lost to a glue anchor, not bad ones the guard filtered out.

### 7.1 Five verbatim, newly lit and correct

```
'clip' -> «магазина»
  SRC … the butt gaping open for its clip.
  TR  … рукоятка зияла открытой для магазина.

'caught' -> «попавшая»
  SRC … then Sarah, caught in the storm as she turned.
  TR  … затем Сара, попавшая в эту бурю, когда повернулась.

'ground' -> «давил»
  SRC I ground down against the crunch, lifted and smashed again.
  TR  Я давил вниз на хруст, поднимал и снова разбивал.

'drawing' -> «затягивая»   (RS)
  SRC The man fidgeted, drawing the collar of his jacket tighter around his neck.
  TR  Человек ёрзал, затягивая воротник куртки плотнее к шее.

'banks' -> «рядами»   (RS)
  SRC … the sides of each shaft brightly lit by the banks of floodlights …
  TR  … стенки каждой шахты ярко освещённые рядами прожекторов …
```

### 7.2 Five verbatim, newly lit and wrong

```
'gaping' -> «поблескивал»
  SRC … the shard pistol glinted dully at me …, the butt gaping open for its clip.
  judge: "«поблескивал» translates 'glinted', not 'gaping'"

'hole' -> «взрыв»
  SRC “Fire in the hole.”  /  TR «Огненный взрыв.»
  judge: "«взрыв» (explosion) doesn't translate 'hole'; the free rendering echoes 'Fire' instead"

'something' -> «кто-то»
  SRC … like someone straining to lift something heavy …
  judge: "«кто-то» translates 'someone', not 'something' (rendered as «тяжесть»)"

'wore' -> «пользовалась»   (RS)
  SRC … beside the mukluks she wore on her feet.
  judge: "'wore' has no rendering here; «пользовалась» renders 'had been using' (the spatula)"

'aglint' -> «заправленные»   (RS)
  SRC … buccaneer-boots, his fingers aglint with jewels and metal.
  judge: "«заправленные» translates 'hooked into' (the boots), not 'aglint'; expected «сверкали»"
```

The dominant failure shape is D4b's, unchanged: not positional drift but **the translation having
no word for the source word at all**, and the aligner lighting something anyway.

## 8. Decision

### DO NOT SHIP.

D4b's kill criterion, quoted verbatim:

> **Kill criterion: if base still wins more than 60 % of decisive pairs on either book, the closed
> class is not the whole mechanism and `itermax` does not ship in any form.**

On *Altered Carbon* base wins **68.3 %** of 104 decisive pairs (p = 2.5e-04). That is above 60 %,
and the criterion is explicitly "on either book", so *Revelation Space*'s 57.3 % (p = 0.139) does
not rescue it. **`EMBALIGN_METHOD` stays at `argmax` in every profile, `emb` and `hybrid` alike.**
The `itermax-guarded` method value ships as code — opt-in, off by default, byte-identical default
path (§2) — because the next experiment needs it, not because anything should select it.

The criterion's own reading is also confirmed in full: **the closed class is not the whole
mechanism.** §6.1 names the other half and shows it was measurable in D4b's data all along.

### What we now know that D4b did not

`itermax`'s second matching round hurts the alignment in **two** independent ways:

| mechanism | who it hits | base : arm (D4c guard) | the guard |
|---|---|---|---|
| a **closed-class** word jumps to a disjoint fragment | 466 / 711 words | *(eliminated: 0 such words)* | **fixes it** |
| a **content** word keeps its highlight and gains a second, unrelated one | 174 / 260 words | **52 : 1**, **53 : 5** | **leaves it** |
| everything else | — | 19 : 32, 14 : 45 (**arm wins**) | — |

Remove both mechanisms and what is left is the thing the knob was adopted for — a source word that
lit nothing now lights something — and on that population the arm wins on both books.

### The next variant, and why it is not the post-glue guard

§4.1's post-glue merge fixes the coverage shortfall and is now clearly the **wrong** next step: it
would restore *more* of `itermax`'s content-word behaviour, and the content-word behaviour is what
loses 52 : 1 and 53 : 5.

The variant the evidence points at is narrower — call it **`itermax-silent`**: run the second
matching round, then keep a new pair **only when the source word was completely unaligned after
round one**. Never widen, never narrow, never move; only light what was dark. Properties, all
either provable by construction or measured here:

- **Coverage: the full gain.** Synthesized from the arms on disk and scored with the verified
  replica of `cmd/score`, content-word tap coverage is **0.9957 on both books** — exactly
  `itermax`'s, against base's 0.9784 / 0.9817 and the built guard's 0.9916 / 0.9930
  (`.artifacts/itermax-guard/synth-variants.txt`). All 130 and 177 content words are recovered.
- **Blast radius: only silent words.** No source word that base lights can change at all, so both
  losing mechanisms are structurally impossible, not merely improbable.
- **Quality of what it lights: ~78 %.** D4b judged all 130 / 177 of those recoveries at 78.5 % and
  77.4 % correct; §7 re-judges the guard's 97 / 141 subset at a consistent rate.
- **Judged, as far as this sample goes: it wins.** The sentences in this pass's sample whose
  *only* change is silent → lit go **7 : 2** (AC) and **9 : 2** (RS) for the arm — pooled
  16 : 4, p = 0.012, though each book alone is underpowered (`litonly-subset.txt`).
- It should also be run with the closed-class freeze on top, so the 168 / 278 newly lit *function*
  words — never judged by anyone — stay out until someone measures them.

That is a D5 experiment with its own arms and its own judging pass, not a change to make now.

### If it had shipped, this is the change it would have been

Recorded so the next pass does not have to rediscover it. Making a method the default for `emb`
and `hybrid` is a one-line default plus its documentation, because `llm` mode never starts the
aligner and there is no per-mode method setting:

1. `internal/config/config.go:220` — `envOr("EMBALIGN_METHOD", "argmax")` → the new value.
2. `internal/embalign/embalign.go` `Options.fill()` — `o.Method = "argmax"` → the new value, so
   library callers (`cmd/alignbench`) match the CLI.
3. `internal/embalign/embalign_test.go` `TestStartChildArgs` — the `default` row's expected child
   command line.
4. `.env.example` — a commented `EMBALIGN_METHOD=` line next to `ALIGN_MODE`, naming `argmax` as
   the way back.
5. `README.md:454` — "mutual argmax" in the *How it works* paragraph.

**None of these has been made.**

## 9. What this pass cannot say

- **One language pair, one direction, one translator model.** en→ru only, two English SF novels,
  Kimi-K2.6 prompt v5. The closed-class list the guard freezes is English's; `de`, `fr` and the
  rest are untested, and LaBSE's subword behaviour against a different target is not assumed to
  transfer.
- **One judge, one model.** Every verdict is `claude -p --model sonnet`, the same harness D4b used.
  The tie rate and the order-inconsistency rate are the only internal consistency evidence; there
  is no human adjudication and no second judge model.
- **150 sampled sentences per book**, not the 400 `PLAN` §1.1 asks for a relative claim — D4b's
  budget, kept identical so the two passes are comparable. The headline and the widening split
  clear p < 1e-3 on their own; the smaller splits are labelled with their own n and p.
- **The newly-lit judging is not blind and has no control**, exactly as in D4b: there is no rival
  fragment to compare against, because the word lit nothing. Read those percentages as an upper
  bound.
- **The two synthesized variants in §4.1 and §8 are arithmetic on the arms already on disk, not
  arms.** Their content-word tap coverage is exact (the replica reproduces `cmd/score` to four
  decimals on all six real arms), but nothing else about them is measured: no lexcheck count, no
  tap-ledger class mix, no judged verdicts, no wall time.
- **The guard's own residue is unexplained at the level of individual glue decisions.** §3 says
  every content deviation from `itermax` is a reversion to base and names the mechanism; it does
  not prove that no other mechanism contributes.

## 10. Artifacts

- Guarded arms: `bench-quality/.artifacts/align-grid/{ac,rs}-guard.{tbook,log,wall}` plus their
  `.tbook.lexflagged.json` and glossary sidecars.
- Default-identity arm: `.artifacts/align-grid/rerun/ac-base-d4c.{tbook,log,wall}`;
  proof line in `.artifacts/itermax-guard/default-identity.txt`, full diff in
  `diff-identity-ac.json`.
- Offline proof for this pass: `.artifacts/itermax-guard/offline-proof.txt` (namespace probe
  `offline-probe.py`).
- Structural check: `.artifacts/itermax-guard/structural-check.txt`,
  `guardcheck-{ac,rs}.json`, `guardcheck-{ac,rs}-offenders.json` (every deviating word with its
  three fragment sets).
- Scores, five per arm: `.artifacts/align-grid/scores/{score,tl,tlc,diff,probe}-{ac,rs}-guard.*`;
  table rows in `.artifacts/itermax-guard/grid-rows.txt`; gate shares in `gated.txt`;
  lexcheck delta with per-word changes in `lexflag-delta.txt`.
- Judging sets, verdicts and summaries: `.artifacts/itermax-guard/{ac,rs}/`.
- Run scripts: `.artifacts/itermax-guard/run-arm.sh` (+ `run-a/b/c.sh`, `judge-all.sh`) and the
  pinned binary `convert-d4c`.
- Verdict analyses: `subset-analysis.txt` (both books' cross-tabs with sign tests),
  `widening-subset.txt` (the widening split, D4b's verdicts and this pass's),
  `litonly-subset.txt` (the pure-recovery subset), `loss-attribution-ac.txt`,
  `lit-breakdown.txt`, `lit-crossref-d4b.txt`, `examples-{ac,rs}.txt`,
  `litexamples-{ac,rs}.txt`, `wordclass-corpus.txt`.
- Synthesized variants (arithmetic on the arms, no runs): `postglue-variant.txt`,
  `synth-variants.txt`.
- New/changed bench scripts: `bench-quality/itermax_guardcheck.py` (structural check),
  `itermax_aligndiff.py` (alignment identity), `itermax_guardrow.py` (grid table rows),
  `itermax_lexdelta.py` (flag delta), `itermax_subsets.py` (subset sign tests),
  `itermax_examples.py` / `itermax_litexamples.py` (verbatim blocks); `itermax_prep.py` gained
  `--b-label`.

### Reproducing

```sh
cd converter
G=bench-quality/.artifacts/align-grid
O=bench-quality/.artifacts/itermax-guard

# arms (offline; each needs its own cache holding ONLY pass-1 translations —
# a cache with this book's FINAL aligned entries makes the run a no-op)
$O/run-arm.sh $G/ac-guard.tbook bench-quality/kimi-a.tbook \
    $G/ac-base.tbook.glossary.en-ru.json itermax-guarded .cache-d4-ac-guard
$O/run-arm.sh $G/rs-guard.tbook ./alaster_rejnolds-revelation_space-1512290726.epub \
    $G/rs-base.tbook.glossary.en-ru.json itermax-guarded .cache-d4 --limit-chapters 2

# checks
python3 bench-quality/itermax_aligndiff.py  $G/ac-base.tbook $G/rerun/ac-base-d4c.tbook
python3 bench-quality/itermax_guardcheck.py $G/ac-base.tbook $G/ac-itermax.tbook \
    $G/ac-guard.tbook --book ac --out $O

# judging set, judging, analysis
python3 bench-quality/itermax_prep.py        $G/ac-base.tbook $G/ac-guard.tbook \
    --book ac --out $O/ac --b-label guard
python3 bench-quality/itermax_wordchanges.py $G/ac-base.tbook $G/ac-guard.tbook --out $O/ac
sh $O/judge-all.sh && sh $O/analyze-all.sh
```

The set construction is deterministic (D4b's stride rule, swap seed 19); the judging is not —
re-running it re-judges and will move the counts by a few pairs.
