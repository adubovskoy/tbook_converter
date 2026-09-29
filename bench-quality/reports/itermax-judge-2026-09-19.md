# Is `itermax` actually better? Blind alignment judging — 2026-09-19

Plan item **D4b**, the kill criterion the align-knob grid left open
(`align-knob-grid-2026-09-19.md` §7: *"blind judging of the … newly flagged AC sentences plus a
sample of the 130 recovered taps. If more than a third of the recovered taps are
wrong-but-present mappings … the +1.7 pp is an illusion and `itermax` goes back."*).

**The headline is a split verdict, and the split is the finding.** Judged blind, in both
presentation orders, on 300 changed sentences, **`argmax` (base) beats `itermax` 71 : 30 on
*Altered Carbon* (p = 5.5e-05) and 68 : 33 on *Revelation Space* (p = 6.4e-04)**. But the loss is
not spread across the alignment — it is carried entirely by one class of word. On the sentences
where a **closed-class** source word moves to a target fragment disjoint from the one `argmax`
gave it, base wins **47 : 7** and **48 : 13**. On the sentences where no closed-class word moves
that way, the two arms are **24 : 23** and **20 : 20** — dead even, p = 1 on both books. And the
thing the knob was adopted for survives judging: of the 130 (AC) and 177 (RS) source words that
were silent under `argmax` and light up under `itermax`, **78.5 % and 77.4 % are correct
renderings**, 16.2 % and 11.9 % wrong — far inside the grid's one-third kill threshold.

So the grid's coverage gain is real and its cost is real, and they sit in different places.
The recommendation (§6) is **adopt with a closed-class guard**, not bare.

## 1. Method

### 1.1 The arms

Two arms per book, already on disk, produced by the grid and byte-reproducible (grid §3):

| | base (`argmax`) | `itermax` |
|---|---|---|
| *Altered Carbon* ch. 1–5 | `.artifacts/align-grid/ac-base.tbook` | `ac-itermax.tbook` |
| *Revelation Space* ch. 1–2 | `.artifacts/align-grid/rs-base.tbook` | `rs-itermax.tbook` |

`bench-quality/itermax_prep.py` walks both files of a pair in `cmd/tbookdiff`'s `collect` order and
asserts, sentence by sentence, that `src` **and** the Russian translation text are identical. They
are: **0 of 1 303 (AC) and 0 of 1 575 (RS) translations differ**. Every difference judged below is
the aligner's and nothing else's.

A sentence counts as **changed** when its alignment chunk list `[(t0, t1, w…)]` differs between the
arms: **688 / 1 303 on AC** and **956 / 1 575 on RS**, reproducing the grid's counts exactly.

### 1.2 The sampling rule

150 changed sentences per book, by even stride over the changed list **in document order**:
index `k → floor(k · L / n)` for `k = 0 … 149`, `L` = 688 (AC) / 956 (RS). That is a stride of
4.59 and 6.37 and it spans the whole list — the AC sample covers changed-list positions 0 … 683,
RS 0 … 949. No randomness in the selection; only the A/B presentation swap is seeded
(`random.Random(19)`, one draw per sampled sentence).

### 1.3 How an arm is shown to the judge

Exactly as `internal/translate/judge.go` `alignPairs` renders it for the j3 prompt: the align
chunks in file order as `["<target fragment>", "<source word(s)>"]`, `""` for a chunk that claims
no source word. The judge sees the English sentence, the **single** Russian translation, and two
competing pair lists `X` and `Y` — never a label, never which arm is which, and it is told
explicitly that the translation is identical on both sides and is not to be graded.

The question is the j3 ALIGNMENT criterion, quoted verbatim into the rubric from `judge.go`:

> 2) ALIGNMENT — is each pair mapped BY MEANING? (Russian word mapped to an English word that does
> not translate it — e.g. positional drift, where words are paired by position instead of meaning —
> → not ok. Unmapped words, function words attached to a neighboring content word, and loose but
> related renderings → ok.)

Verdict `X | Y | tie`, plus a one-line English reason quoting the decisive pair(s). `tie` is
explicitly offered for "they differ only in which function word rides along with a content word".

### 1.4 Rate discipline and blindness

The harness discipline of `judge_pairs.py`, unchanged: `claude -p --model sonnet --tools ""`,
batches of 10 items, `--jobs 5`, 3 attempts with a 30 s × attempt backoff, a **fresh process per
batch** so the two presentation orders of a pair are independent judgements. Both orders are
judged; a pair is **decisive only when the two orders agree after de-swapping** — an explicit tie
in either order, or a disagreement between orders, counts as a tie. Two-sided sign test over the
decisive pairs (`PLAN` §1.1).

`PLAN` §1.1 asks for ≥ 400 blind pairs for a *relative* claim. This is 150 per book, 300 across
both — the task's brief. Both books clear p < 0.001 on their own and agree in direction, so the
headline claim is not at risk from N; the *subset* splits in §5 are where 150 starts to bite, and
they are labelled with their own n and p.

### 1.5 The newly-lit set

Separately and not blind — there is nothing to compare against — every source word that **no chunk
referenced under base and at least one chunk references under `itermax`**, with the target fragment
its tap now highlights. Content words by `cmd/score`'s rule (not in `internal/stopwords` `en`, no
length rule): **130 on AC** and **177 on RS**. AC reproduces the grid's 130 exactly; RS is one
above the grid's 176, which recomputation from the artifacts does not reproduce (the grid's figure
came from a diff of `uncoveredSrcContent` word *lists*, where a word repeated inside one sentence
can collapse).

The judge is asked, one call per 10 items, whether the highlighted fragment is a `correct`,
`partial` or `wrong` rendering of that word, under the same j3 criterion.

The other population the grid reports is also carried: by the **tap ledger's** content rule (≥ 4
letters and not in `internal/learner`'s stoplist) the same recovery is 101 words on AC and 143 on
RS — those are the 2.21 → 0.62 % and 2.07 → 0.58 % headline. Both are judged; the ledger subset is
flagged per row in `summary-lit.json` and reported separately in §4.

### 1.6 Judge-call count

**91 batch judgements delivered**: 60 pairwise (15 batches × 2 orders × 2 books) covering **600
pair-order judgements**, and 31 newly-lit (13 AC + 18 RS) covering **307 word judgements**. Those
came from **94 `claude -p` invocations** — one AC newly-lit batch (`lit-01`) returned an incomplete
object on all three of its attempts and was re-judged in a separate pass. 94 is a lower bound:
silent in-batch retries are not logged separately. No API other than the `claude` CLI was used, no
`.env` was read, and nothing but the local `.tbook` files was consumed.

## 2. What `itermax` changes, counted before it is judged

Every source word of every changed sentence, classified by what happened to the fragment its tap
lights (`.artifacts/itermax-judge/wordclass-corpus.txt`):

| | AC content | AC closed-class | RS content | RS closed-class |
|---|---|---|---|---|
| silent → lit | **130** | 168 | **177** | 278 |
| lit → silent | **0** | **0** | **0** | **0** |
| re-mapped (lights something else) | 420 | 730 | 599 | 1 145 |
| total words touched | 550 | 898 | 776 | 1 423 |

Two facts the grid could not see:

1. **Nothing goes dark.** Not one source word, of either class, loses its highlight. The grid's
   "0 lit → silent" holds for closed-class words too.
2. **Most of what `itermax` touches is closed class** — 62 % of the touched words on AC, 65 % on
   RS — and the re-mapped closed-class words are the ones that move *away*:

| how a re-mapped word's highlight moved | AC content | AC closed | RS content | RS closed |
|---|---|---|---|---|
| widened (old fragment kept, more added) | 174 | **0** | 259 | **0** |
| narrowed (a subset of the old fragment) | 161 | 255 | 240 | 405 |
| shifted (overlaps the old fragment) | 1 | 7 | 8 | 20 |
| **moved to a disjoint fragment** | **83** | **466** | **87** | **711** |

A content word that changes usually grows or shrinks its highlight — it stays on the same
translation word. A closed-class word that changes usually *leaves*: 466 of 730 on AC and 711 of
1 145 on RS light a fragment with nothing in common with what they lit before, and not one
closed-class word ever merely widens. That is the `I → «бы»`, `the → «перестраивает»` shape the
grid quoted, counted rather than anecdotal.

## 3. Verdicts: base vs `itermax`, per book

Both presentation orders judged; decisive only where they agree.

| | *Altered Carbon* | *Revelation Space* |
|---|---|---|
| sampled changed sentences | 150 | 150 |
| judged (no missing verdicts) | 150 | 150 |
| explicit ties | 36 | 40 |
| order-inconsistent (counted as ties) | 13 | 9 |
| **decisive** | **101** | **101** |
| **base (`argmax`) wins** | **71 (70.3 %)** | **68 (67.3 %)** |
| **`itermax` wins** | **30 (29.7 %)** | **33 (32.7 %)** |
| sign test, two-sided | **p = 5.533e-05** | **p = 6.412e-04** |

Two independent books, two independent judgements per pair, the same direction and the same
magnitude. Under the j3 criterion as the shipped judge applies it, **`itermax`'s alignment is worse
than `argmax`'s on the sentences it changes**, about 70 : 30 among decisive pairs.

The tie rate (33 % explicit + 7–9 % order-inconsistent) is well below the 50–57 % the translation
harness sees, which is expected: alignments differ in discrete, nameable ways, so the judge has
more to hold onto than it does between two fluent translations.

## 4. The newly lit words

307 source words, each judged once against its own sentence and its own newly highlighted
fragment. No missing verdicts.

| | *Altered Carbon* (n = 130) | *Revelation Space* (n = 177) |
|---|---|---|
| **correct** | **102 (78.5 %)** | **137 (77.4 %)** |
| partial | 7 (5.4 %) | 19 (10.7 %) |
| **wrong** | **21 (16.2 %)** | **21 (11.9 %)** |

Restricted to the tap-ledger population (≥ 4 letters, ledger stoplist — the words behind the
2.21 → 0.62 % and 2.07 → 0.58 % headline):

| | AC (n = 101) | RS (n = 143) |
|---|---|---|
| correct | 78 (77.2 %) | 112 (78.3 %) |
| partial | 6 (5.9 %) | 15 (10.5 %) |
| wrong | 17 (16.8 %) | 16 (11.2 %) |

**The grid's kill criterion is not met.** It asked whether more than a third of the recovered taps
are wrong-but-present mappings; the measured figure is 16.2 % and 11.9 % wrong (21.6 % and 22.6 %
if `partial` is counted as a failure, still under a third on both books and both readings). A tap
that lit nothing now lights the right thing roughly four times in five.

The dominant failure shape is not drift but **the translation having no word for the source word at
all**, and `itermax` lighting something anyway — `Care` → «Не», `all` (in *all innocence*) → «с»,
`best` (in *at best*) → «работавшую». Genuine drift is the minority: `across` → «полотенцем»
(towel), `pushing` → «пересёк» (crossing), `aglint` → «заправленные» (tucked in).

## 5. The losing classes

Cross-tabbing every decisive verdict against what actually changed inside that sentence
(`.artifacts/itermax-judge/subset-analysis.txt`; sentence counts, not word counts):

| subset of the 150 sampled sentences | AC decisive | AC base : itermax | AC p | RS decisive | RS base : itermax | RS p |
|---|---|---|---|---|---|---|
| all | 101 | **71 : 30** | 5.5e-05 | 101 | **68 : 33** | 6.4e-04 |
| a closed-class word moved to a **disjoint** fragment | 54 | **47 : 7** | **2.3e-08** | 61 | **48 : 13** | **7.7e-06** |
| **no** closed-class word moved that way | 47 | **24 : 23** | **1** | 40 | **20 : 20** | **1** |
| only closed-class words changed at all | 36 | 29 : 7 | 3.1e-04 | 38 | 30 : 8 | 4.7e-04 |
| only content words changed at all | 24 | 15 : 9 | 0.31 | 14 | 8 : 6 | 0.79 |
| a content word is newly lit | 16 | **4 : 12** | 0.077 | 23 | **5 : 18** | **0.011** |
| no content word is newly lit | 85 | 67 : 18 | 8.4e-08 | 78 | 63 : 15 | 3.7e-08 |

Read down that table and the whole result decomposes:

- **The loss is one mechanism.** Remove the sentences where a function word jumps to an unrelated
  translation word and the two aligners are indistinguishable — 24 : 23 and 20 : 20, p = 1 on both
  books. There is no residual content-word penalty hiding underneath.
- **Closed-class-only sentences are a large share of the change and a near-total loss.** 67 of 150
  sampled AC sentences and 68 of 150 RS sentences change *nothing but* function words; base takes
  those 29 : 7 and 30 : 8. Those sentences buy the learner nothing and cost alignment quality.
- **Content-only sentences are a wash**, 15 : 9 and 8 : 6, neither significant.
- **Where a content word is actually recovered, `itermax` wins** — 12 : 4 and 18 : 5. That is the
  learner-facing claim of §4, seen a second time through the blind comparison.

One AC/RS asymmetry worth recording: one RS sentence changed its chunk list without changing any
source word's highlight at all (a target-side-only boundary shift) and was judged for base.

### 5.1 Five verbatim, base better

Quoted from the judges' own reasons; `→` is the target fragment the source word lights.

```
[AC 26] The air was chilly and the walls unpainted.
  closed 'the': base «стены»  -> itermax «неокрашенными»
  judge (both orders): "base keeps «стены»→'the walls'; itermax moves it to
  «неокрашенными»→'the unpainted', a misplacement"

[AC 13] I saw one leg turned to pulp below the knee, and then the body hits, …
  closed 'the': base «колена»  -> itermax «превратилась»
  closed 'the': base «телу»    -> itermax «по»
  judge: "itermax strands 'the' on «превратилась»→'turned the' and «по»→'the' instead of its
  real neighbours; base correctly gives «колена»→'the knee' and «телу»→'the body'"

[AC 14] I lifted the head again and smashed it down again, feeling the skull give soggily with the impact.
  closed 'with': base «от»  -> itermax «прогибается»
  judge: "itermax breaks 'with' off onto «прогибается»→'with' leaving «от»→'the' wrong;
  base keeps «от»→'with the' correctly together"

[RS 4] The man looked back at the grid.
  closed 'The': base «Мужчина»  -> itermax «взглянул»
  judge: "itermax's «взглянул»→'The looked' wrongly attaches 'The' to the verb instead of to
  «Мужчина»='man'"

[RS 5] Inform all dig teams that they carry on working until I say otherwise, …
  CONTENT 'carry': base «продолжают»  -> itermax «раскопочным»
  closed  'there': base «быть»        -> itermax «никаких»
  judge: "itermax's «раскопочным»→'dig carry' steals 'carry' from «продолжают» ('carry on'),
  and «никаких»→'there' misses the actual meaning 'no' that base gives"
```

### 5.2 Five verbatim, `itermax` better

```
[AC 7] Voice husky with the residue of sleep.
  closed 'with': base (nothing)  -> itermax «от»
  judge: "itermax splits 'with the residue of' across «от»→'with the' / «остатков»→'residue of';
  base drops 'with' entirely and duplicates 'the'"

[AC 15] An exit hole big enough to take a golf ball.
  CONTENT 'take': base (nothing)  -> itermax «пролезть»
  judge: "itermax's «пролезть»→'take' matches meaning (fit/take); base maps «пролезть» to a bare article"

[AC 17] Less than two minutes ago I could have picked up the gun, …
  closed 'have': base (nothing)  -> itermax «бы»
  judge: "itermax splits «мог»→'could' / «бы»→'have' correctly for 'could have'; base's
  «бы»→'could' duplicates 'could' and drops 'have' entirely"

[AC 19] In the Envoy Corps they teach you to let go before storage.
  closed 'before': base (nothing)  -> itermax «до»
  judge: "base's «до»→'go' drops 'before' (до's real meaning) entirely; itermax's «до»→'go before'
  at least keeps 'before'"

[AC 10] Ears ringing, still on the floor, I flung the H grenade up at him.
  CONTENT 'ringing': base «звенело лёжа»  -> itermax «звенело»
  judge: "base's «лёжа»→'ringing' reuses the content word already given to «звенело»→'ringing';
  itermax's «лёжа»→'the' is only a stray function word"
```

Note the shape of `itermax`'s wins: they are mostly the *mirror* of its losses — a function word
that base had left dangling on the wrong neighbour gets put back. The second matching round is not
systematically wrong about closed-class words; it is systematically *aggressive* about them, and
aggression on a class where the right answer is often "leave it alone" loses more often than it wins.

### 5.3 Five verbatim, newly lit and correct

```
'maelstrom' -> «вихрь»
  SRC I could hear the maelstrom calling.
  TR  Я слышал, как зовёт вихрь.

'expected' -> «ожидании»
  SRC Beyond the wall I could see figures huddled against the expected grenade blast.
  TR  За стеной я видел фигуры, сжавшиеся в ожидании взрыва гранаты.

'banks' -> «рядами»   (RS)
  SRC … the sides of each shaft brightly lit by the banks of floodlights …
  TR  … стенки каждой шахты ярко освещённые рядами прожекторов …

'beaklike' -> «клювовидной»   (RS)
  SRC … a prominent cranial ridge which extended … beaklike …
  TR  … с выступающим теменным гребнем … клювовидной …

'brought' -> «спущены»   (RS)
  SRC Extra floods had been brought down to shine light into the gloom.
  TR  Дополнительные прожекторы были спущены вниз, чтобы осветить мрак.
```

### 5.4 Five verbatim, newly lit and wrong

```
'Care' -> «Не»
  SRC “Care to tell me why?”     judge: "«Не» is just the negation particle; 'Care' (want) renders
                                 as «хотите», not «не»"

'across' -> «полотенцем»
  SRC … someone wiped a towel roughly across my face …
                                 judge: "'across' should relate to «лицо»; «полотенцем» is 'towel',
                                 an unrelated word"

'pushing' -> «пересёк»
  SRC … crossing the reception area and pushing through the door …
                                 judge: "«пересёк» translates 'crossing', not 'pushing'; expected «распахнул»"

'aglint' -> «заправленные»   (RS)
  SRC … buccaneer boots, aglint …
                                 judge: "'aglint' means sparkling → «сверкали»; «заправленные»
                                 ('tucked into') belongs to 'hooked'"

'many' -> «дюжина»   (RS)
  SRC … at least a dozen other names, in as many languages …
                                 judge: "'many' should map to «стольких же»; «дюжина» translates
                                 'dozen', a different source word"
```

### 5.5 And what lexcheck saw

Every lexcheck flag `itermax` gains or loses, with the word-level change list of that sentence, is
in `.artifacts/itermax-judge/lexflag-delta.txt`. Of AC's 5 newly flagged sentences, exactly **one**
(`I paused to watch the thing realign itself`, `the`: «штука» → «перестраивает») is a pure
closed-class regression; three mix a closed-class move with a content-word move (`put`: «вернуть» →
«спокойное»; `own`: «собой» → «с»; `precautions` on RS), and one is the grid's known false positive
on two *correct* recoveries. A closed-class guard would therefore remove about one of AC's five new
flags outright and change three more — it is not a complete answer to the flag count, and the flag
count was never the right instrument anyway (grid §4.3: flags fall when an arm aligns *less*).

## 6. Recommendation

**Adopt `itermax` only with a closed-class guard. Do not adopt it bare, in `hybrid` or in `emb`.**

This overrides the grid's §7 recommendation ("`itermax` as the `hybrid` default in the `quality`
profile"), which was explicitly conditional on this judging pass and named exactly this evidence as
what would retire it. The retirement condition it named — *more than a third of recovered taps
wrong* — is **not** met (16.2 % / 11.9 %), so the coverage gain stands. But the pass also measured
something the grid did not ask for and could not see: the alignment `itermax` ships is judged
**worse overall, 70 : 30, on both books, p < 0.001**. Shipping it bare trades 130 + 177 recovered
content taps for a broad degradation of function-word mapping across 688 + 956 sentences. That is
not a trade the plan's rule endorses, and the plan's own §1.2 forbids netting a structural gain
against a judged loss.

### The guard

> **Closed-class freeze.** Run the second (`itermax`) matching round as today, then, before chunk
> assembly, discard every new or changed pair whose **source** side is a closed-class word, unless
> `argmax` produced the same pair. Concretely: for each source word `w`, if `w` is in
> `internal/stopwords`'s list for the source language, take `argmax`'s pair set for `w`; otherwise
> take `itermax`'s. Feed the union to the existing target-glue and chunk-grouping steps unchanged.
> This is well-defined because `itermax` only ever *adds* subword pairs to `argmax`'s
> (`tools/embalign.py:80`), so `argmax`'s pair set for any word is always available.

**What supports it, all measured here:**

1. **The loss is entirely inside the guarded class.** On sentences where a closed-class word moves
   to a disjoint fragment, base wins 47 : 7 (AC) and 48 : 13 (RS), p = 2.3e-08 / 7.7e-06. On
   sentences where none does, the arms are 24 : 23 and 20 : 20 — **p = 1 on both books**. There is
   no separate content-word penalty to fix.
2. **The gain is entirely outside it.** All 130 + 177 newly lit words are content words by
   construction, judged 78.5 % / 77.4 % correct; on the sentences carrying them `itermax` wins
   12 : 4 and 18 : 5. Freezing closed-class words does not touch any of them, so the grid's
   +1.73 pp / +1.40 pp content-word tap coverage and the 2.21 → 0.62 % / 2.07 → 0.58 % silence cut
   are preserved by construction — those metrics count content words only.
3. **The class is mechanically distinct, not just statistically.** Not one closed-class word in
   either book merely *widens* its highlight under `itermax`; 466 of 730 (AC) and 711 of 1 145 (RS)
   re-mapped closed-class words jump to a fragment disjoint from the old one, against 83 of 420 and
   87 of 599 for content words. The guard is cutting along a seam the data already has.
4. **Nothing is lost by freezing.** No source word of either class ever goes from lit to silent
   under `itermax`, so freezing closed-class words back to `argmax` cannot darken anything that
   `argmax` lit — and `argmax` is today's shipped default.
5. **It shrinks the change surface by about two thirds.** 898 of 1 448 (AC) and 1 423 of 2 199 (RS)
   of the word-level changes are closed-class. 67 of 150 sampled AC sentences and 68 of 150 RS
   sentences change nothing else at all, and those are the sentences base wins 29 : 7 and 30 : 8.

**What the guard costs, honestly:** `itermax`'s genuine closed-class *wins* go with it — §5.2's
`with` → «от», `have` → «бы», `before` → «до», all of which are function words `argmax` had left
unmapped or misattached. Those are real improvements and the guard throws them away. The blind
numbers say that is the right trade by roughly 47 : 7, but it is a trade, not a free lunch.

**Status of the guard itself: proposed, not measured.** Implementing it needs a change in
`tools/embalign.py` (Go changes and aligner edits were out of scope for this pass), so no guarded
arm exists and none has been judged. The guard's predicted behaviour above is derived from the two
arms on disk, not observed on a third.

### What must happen before it ships

1. Build the guarded arm offline (`EMBALIGN_METHOD=itermax` + a closed-class freeze flag) on both
   books and confirm by `tbookdiff` that, against `ac-itermax` / `rs-itermax`, **no content word's
   highlight changes** and, against `ac-base` / `rs-base`, **no closed-class word's does**. That is
   a pure structural check and needs no API.
2. Re-score it: content-word tap coverage must reproduce `itermax`'s 0.9957 / 0.9957 and
   content-word silence its 0.62 % / 0.58 %. If it does not, the guard has side effects through
   chunk grouping and the premise above is wrong.
3. Re-judge 150 changed sentences per book, guarded arm vs base, same harness. **Kill criterion:
   if base still wins more than 60 % of decisive pairs on either book, the closed class is not the
   whole mechanism and `itermax` does not ship in any form.** The prediction from this pass is
   roughly 50 : 50.
4. Until (1)–(3) exist, **`EMBALIGN_METHOD` stays at `argmax`** in every profile, `emb` and
   `hybrid` alike.

### What is now settled either way

- The grid's coverage numbers are not an illusion: the recovered taps are ~78 % correct on both
  books, so **coverage and correctness agree on the content-word population**. `E` §3.D's warning
  that coverage is blind to correctness holds in general but does not void this particular gain.
- **`itermax` bare is retired.** Both books, both orders, p < 0.001. No further pairwise judging of
  the unguarded arm is needed.
- The grid's split verdict ("adopt for `hybrid`, not for `emb`") is superseded: the reason to
  hesitate was never the lexcheck count, it was the closed-class mapping, and that is now measured
  directly rather than inferred from a flag total that falls when an arm aligns less.

## 7. What this pass cannot say

- **One language pair, one direction, one translator model.** en→ru only, two English SF novels,
  Kimi-K2.6 prompt v5. `itermax`'s closed-class behaviour is a property of LaBSE's subword
  similarities against Russian; de and fr are untested and the guard's stoplist would have to be
  `internal/stopwords`'s own per-source-language list.
- **One judge, one model.** Every verdict here is `claude -p --model sonnet`. The tie rate and the
  order-inconsistency rate (9–13 of 150) are the only internal consistency evidence; there is no
  human adjudication and no second judge model. `PLAN` §8 quarantines relative wins as "judged",
  and this one is judged.
- **150 per book, not the 400 `PLAN` §1.1 asks for a relative claim.** The headline survives that
  (p < 0.001 twice, same direction) but the subset rows in §5 do not all: the content-only split
  (15 : 9, 8 : 6) is underpowered and is reported as a null, not as a result.
- **The newly-lit judging is not blind and has no control.** There is no `argmax` fragment to
  compare against — the word lit nothing. A judge asked "is this right?" with no alternative may be
  more permissive than one asked to choose. The 78 % should be read as an upper-ish bound.
- **The guard is unbuilt.** §6's prediction that the guarded arm judges ~50 : 50 is an inference
  from a subset of this pass's own data, on the same sentences. It is a hypothesis with a kill
  criterion, not a measurement.

## 8. Artifacts

- Scripts (all offline except the `claude` CLI): `bench-quality/itermax_prep.py` (set
  construction), `itermax_wordchanges.py` (exact per-word change lists), `itermax_judge.py` (both
  judging modes), `itermax_analyze.py` (de-swap, sign test, class cross-tab).
- Sets, verdicts and summaries: `bench-quality/.artifacts/itermax-judge/{ac,rs}/` —
  `changed.json`, `key.json`, `wordchanges.json`, `batch-*-o{1,2}.json`, `verdict-*.json`,
  `notes-*.json` (the judges' one-line reasons), `lit-*.json`, `litverdict-*.json`,
  `summary-pairs.json`, `summary-lit.json`.
- Corpus-wide word-class counts: `.artifacts/itermax-judge/wordclass-corpus.{txt,json}` and
  `{ac,rs}-wordclass.json`.
- Subset cross-tabs with their sign tests: `.artifacts/itermax-judge/subset-analysis.txt`.
- Per-book verbatim win lists with both orders' reasons: `.artifacts/itermax-judge/examples.txt`.
- Newly-lit breakdown by content-list population: `.artifacts/itermax-judge/lit-breakdown.txt`.
- Every lexcheck flag `itermax` gains or loses, with that sentence's word-level changes:
  `.artifacts/itermax-judge/lexflag-delta.txt`.

### Reproducing

```sh
cd converter
G=bench-quality/.artifacts/align-grid
O=bench-quality/.artifacts/itermax-judge

python3 bench-quality/itermax_prep.py        $G/ac-base.tbook $G/ac-itermax.tbook --book ac --out $O/ac
python3 bench-quality/itermax_wordchanges.py $G/ac-base.tbook $G/ac-itermax.tbook --out $O/ac
python3 bench-quality/itermax_judge.py   $O/ac --mode pairs --jobs 5
python3 bench-quality/itermax_judge.py   $O/ac --mode lit   --jobs 5
python3 bench-quality/itermax_analyze.py $O/ac --mode pairs
python3 bench-quality/itermax_analyze.py $O/ac --mode lit
# …same four commands with rs-base / rs-itermax and --book rs --out $O/rs
```

The set construction is deterministic (stride rule in §1.2, swap seed 19); the judging is not —
re-running it re-judges and will move the counts by a few pairs.
