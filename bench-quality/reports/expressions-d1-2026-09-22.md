# D1: expression tagging at translate time, DeepSeek on gonka, En→Ru/De/Fr

Plan item **D1** (task T3). I built it on 2026-09-22 and ran the arms on 2026-09-25. Blind judging and
analysis are in §5 onwards. All arms come from one frozen binary. Every arm records its
prompt versions and options in `manifest.meta`.

> Status: this file is written section by section. Any section still marked *pending* was
> not finished when the file was last written.

## 1. What was built

### 1.1 Spec check: `"k":"i"` is italic, not an idiom span

A2 §8 says the producer "already emits idiom spans (`"k":"i"` in `spans`)". **That is wrong.**
`doc/specs/tbook-format.md` §5.5 defines `spans` as *inline emphasis* runs over `src`. `k` is
the emphasis kind: `"i"` means **italic** and `"b"` means **bold** (spec line 600:
"Emphasis kind: `"i"` (italic) or `"b"` (bold)"). The format has no expression or idiom
span, and until this task the producer did no expression tagging. The only related piece was
the experimental aligner `--units-file` (expression spans read from a probe file, research
only). This task builds the tagging itself.

### 1.2 The expression pass (`internal/translate/expr.go`, new)

- **Source-side and paid once per book.** One LLM call per batch of `2 × --batch-size`
  sources (16 at the production batch of 8). The call uses the same client and settings as
  translation and is recorded as phase `expr` in `--stats`. For each sentence the model
  returns the non-compositional expressions (idioms, non-literal phrasal verbs, fixed
  expressions, culturally bound phrases) as `{words, meaning}`. For most sentences it
  returns an empty list. The tags do not depend on the target language, so a book with
  three targets pays for the pass once.
- **Per-sentence cache.** Tags are stored in `expr-<sha256(ExprPromptVersion|expr|model|source|src)>.json`
  in the cache dir, with the source kept for inspection. Prompt version:
  `ExprPromptVersion = "e1"`. If a sentence is never answered after 3 rounds, it is
  translated without tags and is *not* cached, so the next run asks again.
- **Span gates (`validExprs`).** Surrounding space and edge punctuation are trimmed first.
  A span must then be a **verbatim, word-bounded substring** of the source (`wordBounded`:
  no letter or digit directly before or after it). It must be 2 to 12 words long and have a
  non-empty meaning. Duplicates are dropped, at most 3 spans are kept per sentence, and the
  meaning is clipped to 100 runes. A span that fails a gate is counted as *rejected*. If a
  reply is malformed for one id, only that id is lost, not the whole batch.
- **Prompt.** The prompt is written without Holmes material, so the test set does not leak
  into it. Its examples are "spill the beans", "put up with" and "by and large". Before the
  binary was frozen, one tightening line was added after a pilot (§2.3).

### 1.3 Translate-prompt variants (`prompt.go`)

`translateSystemPromptOpts(…, TrOpts{Expressions, NoMirror})`. A test pins the zero value
to the **byte-identical** v5 prompt.

- `Expressions` adds one input line: items may carry `"expr":[{words, meaning}]`. It also
  adds one rule: *"render each tagged expression as the natural {TGT} expression of the same
  meaning — a {TGT} idiom, set phrase or plain wording a {TGT} author would use — NEVER word
  by word. The meaning is a hint, not text to insert. Translate the whole expression: none of
  its words may be dropped or left untranslated."* Only sentences that have tags carry the
  field. Untagged items serialise exactly as before (`omitempty`).
- `NoMirror` replaces the v5 clause *"Natural {TGT} comes first — but when two renderings
  are equally natural, prefer the one that MIRRORS the source: give each content word an
  explicit {TGT} counterpart, keep the source clause order, keep metaphors as images. Do not
  paraphrase freely; …"* with *"Natural {TGT} comes first. Do not paraphrase freely; split
  or merge clauses only where {TGT} grammar demands it."*
- The align pass is unchanged. Its IDIOMS rule (align v8) and the local aligner handle the
  expression words.

### 1.4 Flags, cache separation, provenance

- `--expressions` / `EXPRESSIONS` and `--no-mirror` / `TRANSLATE_NO_MIRROR` are both
  **off by default** and marked EXPERIMENTAL in `--help`.
- **Cache separation without bumping `TrPromptVersion`.** `translate.TranslateVariant`
  returns a suffix: `+x:e1` for expressions, `+nm` for no-mirror, and `""` when both are
  off. `cli.nsModel` adds the suffix to the model component that every namespace of the run
  builds on: raw, proofread and final entries, plus the glossary-hash suffix after it. The
  result is a namespace such as `deepseek-ai/DeepSeek-V4-Flash-0731+x:e1+g:8baef59de5f7`.
  Default runs keep their existing cache keys. That is why this is used instead of bumping
  `TrPromptVersion` globally, which would have re-translated every production book.
- **`manifest.meta`.** With the flags on, `prompts.translate` reads `v5+x:e1` or
  `v5+x:e1+nm`, and the run adds `prompts.expr: "e1"`, `models.expr` and
  `options.expressions` / `options.noMirror`. Default runs record exactly what they
  recorded before.
- **Tests.** `internal/translate/expr_test.go` covers the span gates, word boundaries,
  tolerant reply parsing, variant suffixes, the byte-identity of the prompt, the input JSON
  with and without tags, and the pass's caching against a mock server (a second run makes
  no calls). `internal/cli/expr_meta_test.go` covers the namespace and the provenance.

## 2. Setup and arms

| | |
|---|---|
| Binary | `bench-quality/.artifacts/d1/bin/convert-d1`, built 2026-09-25 16:50 local, sha256 `90ae6c7a…064d0a`, tree `a7f61cf` + `32 files changed, 1981 insertions(+), 314 deletions(-)` (`d1/BINARY.txt`). `internal/typography` (T1) existed in the tree but nothing in `cmd/convert` imports it, so no typography pass is in this binary. |
| Excerpt | `quality-plan/holmes.epub --limit-chapters 4`: 1 667 sentences, 1 624 unique non-empty sources |
| Settings | `--provider gonka --repair --expressions [--no-mirror] --embalign-method argmax`. Model `deepseek-ai/DeepSeek-V4-Flash-0731`, batch 8, concurrency 8, temperature 0.3, hybrid alignment. These are the dsr settings as recorded in its `manifest.meta`. The default aligner method has changed to `itermax-silent-cc` since A2, so `argmax` is pinned to match dsr. |
| Glossary | dsr's sidecar glossaries were copied to each arm (`*.tbook.glossary.en-{l}.json`), so the glossary hash is identical: 135 / 100 / 78 enforced terms. |
| Cache | `d1/cache`, a copy of `a2/cache-ds`. The arms read only their own `+x:e1` / `+x:e1+nm` namespaces, so no translation is shared with dsr. The glossary cache is reused. The expression tags are computed once, in the first arm (`d1-ru`), and shared by all six arms. |
| Run script | `d1/run_d1.sh`, wall times in `d1/wall-times.tsv` |

| slug | what | origin |
|---|---|---|
| `dsr` | DeepSeek + repair, v5 prompt (the A2 gonka default) | reused, `a2/holmes4-dsr-{l}.tbook` |
| `d1` | dsr + `--expressions` | this run |
| `d1m` | d1 + `--no-mirror` (same tags) | this run |
| `g31` | `google/gemini-3.1-flash-lite` (metered default anchor) | reused, `or/gflashlite-{l}.tbook` |

### 2.1 Operational table

| arm | lang | wall | expr pass | translate | repair | requests (expr / tr / rp / al) | errors | cost |
|---|---|--:|--:|--:|--:|--:|--:|--:|
| dsr | ru | 4m58s | — | cached (A2) | 2m47s | 224 (— / — / 207 / 17) | 4 | $0 |
| d1 | ru | 17m59s | 11m26s | 3m15s | 1m53s | 658 (222 / 215 / 203 / 18) | 121 | $0 |
| d1m | ru | 6m32s | cached | 3m06s | 2m18s | 447 (— / 216 / 208 / 23) | 18 | $0 |
| dsr | de | 3m16s | — | cached (A2) | 2m00s | 230 (— / 9 / 211 / 10) | 8 | $0 |
| d1 | de | 4m33s | cached | 1m38s | 1m23s | 420 (— / 204 / 203 / 13) | 0 | $0 |
| d1m | de | 4m21s | cached | 1m46s | 1m33s | 420 (— / 203 / 203 / 14) | 0 | $0 |
| dsr | fr | 3m49s | — | cached (A2) | 2m39s | 235 (— / — / 211 / 24) | 8 | $0 |
| d1 | fr | 5m00s | cached | 2m23s | 1m27s | 436 (— / 207 / 204 / 25) | 5 | $0 |
| d1m | fr | 5m45s | cached | 2m44s | 1m48s | 436 (— / 206 / 204 / 26) | 4 | $0 |

- **Expression-pass cost.** 222 requests, of which 112 returned 200 with a parseable reply.
  The ideal is 102 (1 624 sentences / 16). The other 110 were gateway failures that the
  retry policy absorbed: 64 × 429 "too many concurrent requests", 30 × 503 "no escrow can
  fund this request", 13 transport timeouts and 3 × 502, plus 11 replies that came back 200
  but were empty or truncated JSON. The pass took 11m26s. All 1 624 sentences were tagged
  after 3 rounds, with 0 left untagged. The gateway was in poor health during that window:
  the translate phase of the same arm had 11 errors in 215 requests. The pass costs about
  one translate phase in requests and is paid **once for all targets**. Per target language
  on a three-target book, that is about +0.33 of a translate phase.
- **gonka reports no cost**, so `$0` means "not reported / free to us". No OpenRouter
  request was made for any arm.
- The d1 and d1m translate and repair wall times are in line with A2's DeepSeek phases.
  The d1-ru wall time is dominated by the expression pass on a congested gateway.

### 2.2 Tag statistics

Tags are source-side, so they are the same for all three languages.

| | value |
|---|--:|
| unique sources | 1 624 |
| sentences with ≥ 1 tag | **479 (29.5 %)** |
| tags | **695 (42.8 per 100 sentences)** |
| tags per tagged sentence | 1: 303, 2: 136, 3: 40 (the cap) |
| span length (words) | 2: 184, 3: 242, 4: 136, 5: 75, 6: 34, 7: 17, 8+: 7 |
| spans rejected by the gates | 26 (3.6 % of returned spans) |
| sentences untagged after 3 rounds | 0 |

**Tag counts vary between runs.** Two pilots with the same model, batch and data gave
22.0 % (341 of 1 549 answered, pre-tightening prompt) and 26.0 % (422 of 1 624, the frozen
prompt, 30 sentences unanswered) before the frozen arm's 29.5 %. At temperature 0.3,
different request groupings produce different replies, so the rate should be read as
25–30 %, not as an exact figure. The set is broader than intended. It tags many
light-verb and phrasal constructions ("set off", "caught a glimpse of", "take a considerable
interest") alongside real idioms ("wash linen of the sort in public", "a clear field",
"crack a crib").

### 2.3 Pilot and the one prompt change before freezing

The first pilot (dev binary, scratch cache, not used in any arm) tagged 22 % of sentences.
In a 50-tag sample, about a quarter were literal phrases: "pay our debts", "fonder of",
"is serious", "own circle", "used to say". One line was then added to the "Do NOT list"
paragraph: *phrases whose words keep their everyday meaning ("pay the bill", "very fond of",
"it is serious") are not expressions. When unsure, leave it out.* The rate did **not** go
down (26 % and then 29.5 % in the frozen run). The line did not measurably tighten the set,
and the false-positive rate is measured on the frozen tags in §3.

## 4. Structural table

| arm | lang | validation | sentences | empty | glossary | align gated | coverage tgt/src | lexcheck flags | langcheck | tap clean / shared / none | cw-silent |
|---|---|---|--:|--:|--:|--:|--:|--:|---|--:|--:|
| dsr | ru | OK | 1667 | 0 | 135 | 34 | 98/96 % | 17 (17 low-support) | clean | 52.70 / 39.61 / 3.84 % | 2.72 % |
| d1 | ru | OK | 1667 | 0 | 135 | 36 | 98/96 % | 23 (23 low-support) | clean | 52.86 / 39.49 / 3.83 % | 2.81 % |
| d1m | ru | OK | 1667 | **1** | 135 | 45 | 98/95 % | 30 (30 low-support) | clean | 51.69 / 39.81 / 4.56 % | 3.15 % |
| dsr | de | OK | 1667 | 0 | 100 | 20 | 98/97 % | 9 (7 low-support, 2 shift) | clean | 74.79 / 18.07 / 2.64 % | 2.44 % |
| d1 | de | OK | 1667 | 0 | 100 | 26 | 98/97 % | 10 (8 low-support, 2 shift) | clean | 74.69 / 18.12 / 2.64 % | 2.51 % |
| d1m | de | OK | 1667 | 0 | 100 | 27 | 98/97 % | 11 (11 low-support) | clean | 73.47 / 18.90 / 2.96 % | 2.83 % |
| dsr | fr | OK | 1667 | 0 | 78 | 45 | 98/97 % | 28 (28 low-support) | clean | 66.60 / 22.57 / 2.83 % | 2.56 % |
| d1 | fr | OK | 1667 | 0 | 78 | 49 | 98/97 % | 36 (36 low-support) | clean | 66.62 / 22.51 / 2.93 % | 2.78 % |
| d1m | fr | OK | 1667 | 0 | 78 | 52 | 98/97 % | 35 (34 low-support, 1 shift) | clean | 65.38 / 23.27 / 3.27 % | 3.01 % |

(`d1/armstats-{l}.{json,txt}`, `d1/armstats_d1.py`; tap ledgers `d1/tapledger-{arm}-{l}.txt`.)

- **d1 is structurally indistinguishable from dsr.** Tap classes are within 0.2 pp, and
  coverage and validation are identical. Lexcheck flags go up slightly (ru +6, de +1,
  fr +8), which fits renderings by meaning that the bilingual lexicon covers less well. By
  the plan's rule 2 this is not evidence of quality either way. The language gate never
  fired (0/0/0 in all six arms).
- **d1m (no mirror clause) loosens the alignment a little in every language.** "None" taps
  rise by 0.3–0.7 pp and "clean" taps fall by 1.2 pp. The hybrid gate sends 3–9 more
  sentences to the LLM aligner. d1m-ru shipped **one empty cell**, the four-character fragment `s.’`. Its translate batch came
  back `finish_reason=length` in all three rounds, and the file is still valid. That is a less
  literal translation doing exactly what one would expect to a word aligner. Whether readers
  gain from it is §5's question.

## 5. Blind judging

Method as in A2 §4. `prepare_pairs_lang.py` with `--min-words 4 --batch 10 --seed 1`
excludes identical cells. Each batch is judged by one fresh `claude -p --model sonnet`
process in both presentation orders. `analyze_pairs.py` counts a pair as decisive only when
both orders pick the same side. Two-sided sign test.

**Rubric.** Every D1 set was judged with **A2's rubric**: `judge_pairs.py` as of HEAD
`a7f61cf`, copied to `d1/judge_pairs_a2rubric.py` with only the strict empty MCP config
(`--strict-mcp-config --mcp-config '{"mcpServers":{}}'`) added. That is the rubric the 38.1 /
35.1 / 39.4 % A2 shares were measured with. The working-tree `bench-quality/judge_pairs.py`
has since gained T1's quotation-mark and French-incise clauses. Using it here would have
made (b) incomparable with A2, so it was **not** used for any D1 set. `d1/judge_some.py`
drives the copy, judging at most N batches per call so a foreground call ends inside
`timeout 590`. The judge alias `sonnet` resolves to whatever Sonnet the CLI served on
2026-09-25/26. That may be a newer snapshot than A2's, a drift this report cannot control.

### 5.1 (a) d1 vs dsr on tagged sentences — the primary test

Tagged sentences: 479 unique sources. Pairs are the tagged sentences whose d1 and dsr cells
differ and whose source has ≥ 4 words. All of them are judged (fewer than 400 each).

| lang | pairs (tagged, differing) | identical excluded | ties | order-inconsistent | decisive | d1 : dsr | d1 share | p |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| ru | 379 | 97 | 201 | 24 | 154 | 76 : 78 | **49.4 %** | 0.94 |
| de | 355 | 124 | 178 | 23 | 154 | 65 : 89 | **42.2 %** | 0.063 |
| fr | 351 | 127 | 184 | 29 | 138 | 72 : 66 | **52.2 %** | 0.67 |

**d1 does not beat dsr on the sentences it tags in any language.** Russian is a dead heat.
German leans *against* tagging (42.2 %, p = 0.063, not significant at 0.05). French is a
statistically empty 52.2 %. The ship bar is ≥ 60 % in every language, and all three
languages miss it.

#### Defect classes on the tagged sets (`analyze_notes.py`, judgements naming each side as the loser)

| class | ru: vs d1 | ru: vs dsr | de: vs d1 | de: vs dsr | fr: vs d1 | fr: vs dsr |
|---|--:|--:|--:|--:|--:|--:|
| **idiom-calque** | **16** | **29** | **14** | **37** | **13** | **44** |
| fidelity | 68 | 75 | **78** | **44** | 65 | 58 |
| punctuation | 27 | 37 | **58** | **12** | **43** | **19** |
| naturalness | 43 | 38 | 32 | 24 | 22 | 19 |
| collocation | 30 | 26 | 22 | 27 | 20 | 29 |
| grammar-agreement | 6 | 0 | 6 | 6 | 10 | 5 |
| register | 6 | 5 | 17 | 15 | 7 | 8 |
| terminology | 9 | 10 | 8 | 9 | 9 | 8 |
| case-government, nonexistent-word, other | 14 | 8 | 9 | 10 | 14 | 14 |
| **total** | 219 | 228 | 244 | 184 | 203 | 204 |

**The mechanism works, and the verdict is still a tie or worse.** The idiom-calque class,
which is what the tags target, falls by 45 % in Russian (29 → 16), 62 % in German (37 → 14)
and 70 % in French (44 → 13). By this judge's own labels, tagging removes most calques on
the sentences it touches. The gain is then cancelled by defects the tags did not target.
German loses +34 fidelity and +46 punctuation judgements, and French loses +24 punctuation.
§6 looks at why.

For comparison with A2 (dsr vs g31, 400 random pairs), idiom-calque judgements against dsr
were 29 / 47 / 45 in ru / de / fr, and against g31 7 / 9 / 23. On the tagged subset alone,
d1 is now at 16 / 14 / 13 calque judgements against it. This is a different sample (tagged
sentences only), so the numbers are directional, not a like-for-like delta.

## 3. Tag examples and false-positive rate

For each language, 20 tags were drawn at random from the 695 frozen tags (`random.Random(100+i)`,
`d1/tag-samples.json`). The tags are source-side, so the three samples are three independent draws
from one set. A tag is counted as a **false positive** when the span is compositional, meaning a
competent translator renders it word by word with no loss. That is my own rating, and it is
marked **FP** below.

| # | ru sample | de sample | fr sample |
|--:|---|---|---|
| 1 | «made me mad» = angered me | «rise from crime to crime» = progress to worse crimes | **FP** «family circle» = close relatives |
| 2 | «come what may» = whatever happens | «I have the honour to wish you a very good morning» = formal farewell greeting | «wonder at» = be surprised about |
| 3 | «has a soul of steel» = is very determined and unyielding | «before I knew where I was» = suddenly, without realizing | «in judicial moods» = in a judging frame of mind |
| 4 | «I am lost without my Boswell» = I need my biographer | «quite peculiar to» = unique to | «just my point» = exactly what I mean |
| 5 | «well dressed and ill» = of varied social classes | «became entangled with» = had a romantic involvement with | «started off» = set out |
| 6 | **FP** «for an instant» = for a moment | «went off» = departed | **FP** «only a joke at first» = initially just a prank |
| 7 | «set eyes upon» = see or encounter | «refreshed his memory» = reminded himself | **FP** «far more daring» = much more bold |
| 8 | «in the dark» = uninformed, unaware | **FP** «pacing up and down» = walking back and forth | **FP** «speak as freely» = talk openly |
| 9 | «as brave as a bulldog» = very brave | «balancing whether» = deciding between | «seized with compunction» = felt remorse or guilt |
| 10 | **FP** «manufactory of artificial knee-caps» = factory making fake kneecaps | «getting out of it» = escaping or avoiding | «fell upon his knees» = knelt down |
| 11 | «shot a questioning glance» = looked questioningly at someone | **FP** «lay a whip across your shoulders» = whip you | «worked up to the highest pitch of tension» = made extremely tense |
| 12 | «spare you» = allow you time away | «started up» = suddenly rose or jumped | «made upon it» = attempted against it |
| 13 | «room for doubt» = possibility of uncertainty | «raise the cry of fire» = shout 'Fire!' to alert | «Voilà tout» = That is all |
| 14 | **FP** «hard to say» = difficult to determine | «have a job to» = find it difficult to | «Not in the least» = not at all |
| 15 | «went off» = departed | «daintiest thing under a bonnet» = most charming woman | «rise from crime to crime» = progress to worse crimes |
| 16 | «the other day» = recently | **FP** «false alarm» = unnecessary warning | «rest in peace» = be free from worry |
| 17 | **FP** «glancing out of the window» = looking out the window | «put ideas in his head» = suggest ambitions to him | «in passing» = incidentally |
| 18 | «might die for» = extremely attractive | «made up her mind» = decided firmly | «drove up» = arrived by vehicle |
| 19 | «with his head on one side» = tilting his head curiously | «made up his mind» = decided | «Well, I never» = expression of surprise |
| 20 | «scene of action» = place of events | **FP** «stuck his feet up» = placed feet elevated | «all right» = safe, unharmed |

| | ru | de | fr | total |
|---|--:|--:|--:|--:|
| false positives in 20 | 4 | 4 | 4 | 12 / 60 |
| FP rate | 20 % | 20 % | 20 % | **20 %** |

The false positives are literal phrases ("for an instant", "glancing out of the window",
"pacing up and down", "false alarm", "stuck his feet up", "speak as freely") and one noun
phrase ("manufactory of artificial knee-caps"). The samples do not show a second, more harmful
class: tags whose **meaning** is wrong or too generic. It turns up in the judged losses (§6):
«Half a guinea» and «half a sovereign» = *a small bet of money*, «had my rubber» = *played my
card game*, «buttoning up his pea-jacket» = *fastening his heavy coat*. The FP rate is under the
25 % bar, so this criterion passes.
