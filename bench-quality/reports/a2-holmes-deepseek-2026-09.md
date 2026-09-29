# A2 — DeepSeek on gonka against the metered default, En→Ru, En→De, En→Fr

Plan item **A2** (`panel/quality/PLAN.md` §2). Runs 2026-09-20, blind judging and analysis
completed 2026-09-22. Producer at `a7f61cf87c9f-dirty`; every arm carries that commit, the prompt
versions and the option set in `manifest.meta`.

This is the first blind translation-quality measurement of **En→De** and **En→Fr** in the
workspace, and a fresh En→Ru datapoint on the model gonka serves today. Before it, EVIDENCE §6.2
recorded zero blind comparisons, zero defect counts and zero repair figures for German and French.

Four results, up front:

1. **DeepSeek-V4-Flash + repair on gonka is behind the metered production default
   (`gemini-3.1-flash-lite`) in all three languages, and the gap is significant in every one**:
   38.1 % of decisive pairs in Russian (p = 0.0037), 35.1 % in German (p = 1.4e-04), 39.4 % in
   French (p = 0.0039). Against the upper anchor `gemini-3.7-flash` it wins 36.9 / 33.3 / 29.9 %
   (§4).
2. **The excess defects are concentrated, not diffuse.** Against g31, punctuation and idiom calques
   together are 64 % of DeepSeek's excess losses in Russian; idiom calques, naturalness and
   collocation are 64 % of them in German; punctuation, idiom calques and collocation are 75 % of
   them in French (§5).
3. **Most of the punctuation penalty is real, not a convention quarrel** — 35 of 39 Russian
   punctuation judgements against DeepSeek point at wrong or missing marks (quotation marks that
   do not match the source cell, missing attribution dashes, `.»`, ASCII `"`). In German and French
   about a third of the penalty is the judge preferring self-contained quotes over the
   source-mirroring convention (§6).
4. **The repair pass on DeepSeek is a near no-op.** It changes 11 / 13 / 8 of 1 602 comparable
   sentences (0.69 / 0.81 / 0.50 %), the judge prefers the repaired version in 13 and the original
   in 7, and six of those seven losses are the repair model deleting a closing quote (ASCII `"` in
   five of them) instead of converting it (§7).

Nothing structural separates the arms: all twelve `.tbook`s validate at 1 667 sentences with 0 empty
cells; langcheck is clean everywhere except one untranslated German cell that both Google arms
share; the wrong-language leak that dominated the De/Fr evidence base is at zero (§3).

## 1. Setup

| | |
|---|---|
| Binary | `bench-quality/.artifacts/quality-plan/bin/convert-a2`, built 2026-09-19 22:15 local |
| Tree at build time | `git diff --stat` → `28 files changed, 1562 insertions(+), 281 deletions(-)` on `a7f61cf` |
| B1c marker | `bench-quality/.artifacts/quality-plan/B1C_DONE` present (glossary sample pass fixed, `g2`) |
| Excerpt | `bench-quality/.artifacts/quality-plan/holmes.epub`, `--limit-chapters 4` — *The Adventures of Sherlock Holmes*, **1 667 sentences** (1 642 with prose source text, 25 contents-table cells) |
| Arms directory | `bench-quality/.artifacts/quality-plan/a2/` |
| Translation cache | `a2/cache-ds` (shared by the two DeepSeek arms, so `dsr` re-uses `ds`'s translations and only the repair pass is new) |
| Judge | `bench-quality/judge_pairs.py`, `claude -p --model sonnet`, one fresh process per batch of 10, each pair judged in both presentation orders |

Production settings are the gonka defaults in force at HEAD (`b1-langgate-2026-09-19.md` §5,
`gonka-models-2026-09-19.md` § *Production settings today*): `--provider gonka`, model
`deepseek-ai/DeepSeek-V4-Flash-0731`, batch 8, concurrency 8, 60 s request timeout, 3 retries,
langcheck gate on, translate prompt v5, repair prompt r2, align prompt v8, hybrid alignment,
temperature 0.3. Concurrency was left at 8 throughout: the gateway's ~111-request budget is shared
across its clients.

### The four arms

| slug | model | provider | repair | origin |
|---|---|---|---|---|
| `ds` | `deepseek-ai/DeepSeek-V4-Flash-0731` | gonka | off | this run |
| `dsr` | same | gonka | **on** (the gonka default) | this run, same cache as `ds` |
| `g31` | `google/gemini-3.1-flash-lite` | openrouter | off | **reused** from C4: `…/quality-plan/or/gflashlite-{ru,de,fr}.tbook`, same excerpt, same binary family, same defaults |
| `g37` | `google/gemini-3.7-flash` | openrouter | off | **reused** from A3: `…/quality-plan/a2/holmes4-g37-{ru,de,fr}.tbook` |

`g31` is the production default the paid pipeline ships with; `g37` is A3's upper anchor. Neither
was re-produced, so this report spent **$0** of metered budget — every new translation request in
it went to gonka.

## 2. Per-arm operational table

| arm | lang | wall | translate | repair | requests | errors | success | cost $ | gate flagged/fixed/retrans |
|---|---|--:|--:|--:|--:|--:|--:|--:|--:|
| ds | ru | 4m37s | 3m21s / 8.1 s⁻¹ | — | 230 | 3 | 98.7 % | 0.0000 | 0/0/0 |
| dsr | ru | 4m58s | cached | 2m47s / 9.7 s⁻¹ | 224 | 4 | 98.2 % | 0.0000 | 0/0/0 |
| g31 | ru | 7m32s | 10s / 158.7 s⁻¹ | — | 119 | 0 | 100.0 % | 0.1419 | 0/0/0 |
| g37 | ru | 4m34s | 21s / 78.3 s⁻¹ | — | 122 | 1 | 99.2 % | 0.3688 | 0/0/0 |
| ds | de | 1m49s | cached | — | 228 | 17 | 92.5 % | 0.0000 | 0/0/0 |
| dsr | de | 3m16s | cached | 2m0s / 10.7 s⁻¹ | 230 | 8 | 96.5 % | 0.0000 | 0/0/0 |
| g31 | de | 2m31s | 35s / 46.0 s⁻¹ | — | 115 | 1 | 99.1 % | 0.1143 | 0/0/0 |
| g37 | de | 3m56s | 19s / 86.1 s⁻¹ | — | 115 | 1 | 99.1 % | 0.2956 | 0/0/0 |
| ds | fr | 9m17s | 5m8s / 5.3 s⁻¹ | — | 258 | 24 | 90.7 % | 0.0000 | 0/0/0 |
| dsr | fr | 3m49s | cached | 2m39s / 10.2 s⁻¹ | 235 | 8 | 96.6 % | 0.0000 | 0/0/0 |
| g31 | fr | 4m52s | 30s / 54.6 s⁻¹ | — | 124 | 3 | 97.6 % | 0.1382 | 0/0/0 |
| g37 | fr | 4m45s | 58s / 28.1 s⁻¹ | — | 127 | 5 | 96.1 % | 0.3164 | 0/0/0 |

## 3. Structural table

| arm | lang | validation | sentences | empty | glossary enforced | align gated | coverage tgt/src | lexcheck flags | langcheck |
|---|---|---|--:|--:|--:|--:|--:|--:|---|
| ds | ru | OK | 1667 | 0 | 135 | 34 | 98/96 % | 18 (18 low-support, 0 shift-pattern) | clean |
| dsr | ru | OK | 1667 | 0 | 135 | 34 | 98/96 % | 17 (17 low-support, 0 shift-pattern) | clean |
| g31 | ru | OK | 1667 | 0 | 62 | 53 | 98/96 % | 34 (34 low-support, 0 shift-pattern) | clean |
| g37 | ru | OK | 1667 | 0 | 80 | 60 | 98/96 % | 37 (37 low-support, 0 shift-pattern) | clean |
| ds | de | OK | 1667 | 0 | 100 | 20 | 98/97 % | 9 (7 low-support, 2 shift-pattern) | clean |
| dsr | de | OK | 1667 | 0 | 100 | 20 | 98/97 % | 9 (7 low-support, 2 shift-pattern) | clean |
| g31 | de | OK | 1667 | 0 | 24 | 35 | 97/97 % | 16 (15 low-support, 1 shift-pattern) | 1 untranslated |
| g37 | de | OK | 1667 | 0 | 47 | 33 | 97/97 % | 23 (20 low-support, 3 shift-pattern) | 1 untranslated |
| ds | fr | OK | 1667 | 0 | 78 | 45 | 98/97 % | 28 (28 low-support, 0 shift-pattern) | clean |
| dsr | fr | OK | 1667 | 0 | 78 | 45 | 98/97 % | 28 (28 low-support, 0 shift-pattern) | clean |
| g31 | fr | OK | 1667 | 0 | 41 | 57 | 98/97 % | 30 (29 low-support, 1 shift-pattern) | clean |
| g37 | fr | OK | 1667 | 0 | 25 | 66 | 98/97 % | 54 (53 low-support, 1 shift-pattern) | clean |

Reading notes for the two tables.

- **`cached` in the translate column** means the phase found every sentence in `a2/cache-ds` and
  issued no translate request. That is by design for `dsr` (it re-uses `ds`'s translations) and it
  is an artefact for **`ds`/de**: that arm's translate phase ran in a session that was interrupted
  after it had filled the cache, and the resumed run only aligned. Its request and error counts
  are the sum over both attempts and are real; its **wall time is not comparable** with the
  other two languages.
- **gonka returns no cost field**, so `$0.0000` for `ds`/`dsr` means "not reported", not "free at
  the provider" — it is free to us. The two Google arms cost `$0.394` (g31) and `$0.981` (g37) in
  total and were paid for by C4 and A3 respectively; A2 re-used the files.
- **`ds`/fr at 90.7 % request success** sits on the plan's quarantine line (§8: a run under 90 % is
  quarantined). It is above it, and the arm has 0 empty cells, so it is reported rather than
  quarantined — but it is the weakest operational arm here. All 24 failures are transport errors
  (`Post "https://proxy.gonka.gg/v1/chat/completions"`, plus two 429s) that the 3-retry policy
  absorbed.
- **The language gate never fired.** `0/0/0` in every row: no wrong-language cell was flagged in
  any of the twelve arms, so no re-translation budget was spent. The 55–73 % Cyrillic overlays that
  dominate the De/Fr evidence base (EVIDENCE §3.B, §3.C) do not reproduce anywhere at HEAD. This is
  the second independent confirmation after B1.
- **Validation does not catch everything.** `ds`/de shipped one cell with a leaked JSON fragment
  (`…zu dem Thema geben.“} 9:`, §7); the file still validated. The repair pass removed it.
- **Glossary size differs per arm because each model builds its own.** DeepSeek's sample pass
  (fixed in B1c) yields a much larger enforced set than the Google models': ru 135 vs 62/80,
  de 100 vs 24/47, fr 78 vs 41/25. That is the pipeline as it ships for each model, so the
  comparison is arm-as-shipped rather than glossary-controlled; it is a confound worth naming.
- **lexcheck flags fewer DeepSeek sentences than Google's** in every language (ru 18/17 vs 34/37,
  de 9 vs 16/23, fr 28 vs 30/54). Per the plan's rule 2 this is a structural number and is not
  evidence of quality; §4 shows it points the opposite way from the judged result.

### Tap ledger (`cmd/tapledger`, all taps, per language)

| lang | arm | clean | clean-multi | shared | smear | none | dead-sentence | content-word silent |
|---|---|--:|--:|--:|--:|--:|--:|--:|
| ru | ds | 52.71 % | 3.22 % | 39.61 % | 0.59 % | 3.83 % | 0.04 % | 2.71 % |
| ru | dsr | 52.70 % | 3.22 % | 39.61 % | 0.59 % | 3.84 % | 0.04 % | 2.72 % |
| ru | g31 | 51.84 % | 3.62 % | 39.39 % | 0.88 % | 4.27 % | 0.00 % | 2.96 % |
| ru | g37 | 52.31 % | 3.80 % | 38.95 % | 0.94 % | 3.99 % | 0.02 % | 3.07 % |
| de | ds | 74.78 % | 3.93 % | 18.07 % | 0.58 % | 2.64 % | 0.00 % | 2.45 % |
| de | dsr | 74.79 % | 3.92 % | 18.07 % | 0.58 % | 2.64 % | 0.00 % | 2.44 % |
| de | g31 | 73.88 % | 4.73 % | 17.95 % | 0.73 % | 2.70 % | 0.00 % | 2.74 % |
| de | g37 | 73.64 % | 4.91 % | 17.80 % | 0.83 % | 2.83 % | 0.00 % | 2.63 % |
| fr | ds | 66.60 % | 6.57 % | 22.57 % | 1.44 % | 2.83 % | 0.00 % | 2.56 % |
| fr | dsr | 66.60 % | 6.57 % | 22.57 % | 1.44 % | 2.83 % | 0.00 % | 2.56 % |
| fr | g31 | 66.34 % | 7.16 % | 21.66 % | 1.73 % | 2.95 % | 0.15 % | 2.90 % |
| fr | g37 | 65.44 % | 7.49 % | 22.13 % | 1.96 % | 2.97 % | 0.00 % | 2.91 % |

All four arms land within ~1 pp of each other in every class in every language: **the aligner does
not care which of these models wrote the text**, and the learner-visible tap distribution is not a
discriminator between them. `cmd/consensus` is not applicable — each arm carries one target.

## 4. Blind judging

Method. `prepare_pairs_lang.py` takes the sentences where the two arms' texts differ (identical
cells excluded), samples a fixed-seed subset, randomises which side is shown first (`key.json`),
and writes batches of 10 in two presentation orders. Each batch goes to a fresh Sonnet process
with the rubric in `judge_pairs.py` (fidelity > grammar > idiom/register > naturalness; *tie* for
dash-vs-guillemet style and neutral synonyms; name the loser's worst defect class).
`analyze_pairs.py` counts a pair as **decisive only when both orders pick the same side**; an
explicit tie in either order, or disagreement between the orders, is a tie. Two-sided sign test on
the decisive pairs.

Two batches of the g37 sets (`dsr-vs-g37-de/batch-17-o1`, `dsr-vs-g37-fr/batch-05-o1`) had been left
as 0-byte verdict files by an interrupted judging session; they were moved aside and re-judged
on 2026-09-22 with the same script and model, so all 200 pairs are judged in both orders.

`share` is the DeepSeek (`dsr`) share of decisive pairs; 50 % is parity.

### Russian

| comparison | pairs (of differing) | ties | order-inconsistent | decisive | dsr : anchor | dsr share | p |
|---|--:|--:|--:|--:|--:|--:|--:|
| dsr vs g31 | 400 (of 1 340) | 211 | 34 | 155 | 59 : 96 | **38.1 %** | 0.0037 |
| dsr vs g37 | 200 (of 1 363) | 102 | 14 | 84 | 31 : 53 | **36.9 %** | 0.021 |
| ds vs dsr | 11 (of 11) | 2 | 0 | 9 | ds 3 : dsr 6 | dsr 66.7 % | 0.51 |

Restricted to GOLD-lite ids (`gold/en-ru.jsonl`, the only gold file worth using here):

| comparison | gold pairs | ties | order-inconsistent | decisive | dsr : anchor | dsr share | p |
|---|--:|--:|--:|--:|--:|--:|--:|
| dsr vs g31 | 82 | 42 | 9 | 31 | 10 : 21 | 32.3 % | 0.071 |
| dsr vs g37 | 42 | 21 | 3 | 18 | 7 : 11 | 38.9 % | 0.48 |
| ds vs dsr | 2 | 0 | 0 | 2 | ds 0 : dsr 2 | — | 0.5 |

The gold subsets point the same way as the full samples; they are too small to be significant on
their own.

### German

| comparison | pairs (of differing) | ties | order-inconsistent | decisive | dsr : anchor | dsr share | p |
|---|--:|--:|--:|--:|--:|--:|--:|
| dsr vs g31 | 400 (of 1 205) | 199 | 33 | 168 | 59 : 109 | **35.1 %** | 1.4e-04 |
| dsr vs g37 | 200 (of 1 247) | 94 | 16 | 90 | 30 : 60 | **33.3 %** | 0.0021 |
| ds vs dsr | 13 (of 13) | 4 | 1 | 8 | ds 3 : dsr 5 | dsr 62.5 % | 0.73 |

Silver-id restriction (`gold/en-de.jsonl`, silver quality — shown for direction only):
dsr vs g31 17 : 21 (44.7 %, p = 0.63, 38 decisive of 87); dsr vs g37 7 : 13 (35.0 %, p = 0.26,
20 decisive of 47).

### French

| comparison | pairs (of differing) | ties | order-inconsistent | decisive | dsr : anchor | dsr share | p |
|---|--:|--:|--:|--:|--:|--:|--:|
| dsr vs g31 | 400 (of 1 198) | 192 | 15 | 193 | 76 : 117 | **39.4 %** | 0.0039 |
| dsr vs g37 | 200 (of 1 213) | 96 | 7 | 97 | 29 : 68 | **29.9 %** | 9.3e-05 |
| ds vs dsr | 8 (of 8) | 5 | 0 | 3 | ds 1 : dsr 2 | dsr 66.7 % | 1.0 |

Silver-id restriction (`gold/en-fr.jsonl`, silver): dsr vs g31 19 : 25 (43.2 %, p = 0.45,
44 decisive of 82); dsr vs g37 9 : 19 (32.1 %, p = 0.087, 28 decisive of 44).

### Book-scale reading

Scaling the sample's net margin to the differing sentences of the whole excerpt (an
extrapolation, not a measurement): against g31, DeepSeek + repair loses a net ≈ 124 sentences in
Russian (37/400 × 1 340), ≈ 151 in German (50/400 × 1 205) and ≈ 123 in French (41/400 × 1 198);
against g37, ≈ 150 / 187 / 237. There is no g31-vs-g37 pair set on this excerpt, so the distance
between the two anchors is not measured here; the DeepSeek shares only say that g37 sits further
above DeepSeek than g31 does in French and German, and about as far in Russian.

## 5. Defect taxonomy

Source: `notes-*.json` in each set. The judge returns, for every non-tie judgement, the winner
and **one defect-class label for the loser** — there is no free-text note; the "note" quoted
below is that label, per presentation order (`o1/o2`). Two counts per class:

- **judgements** — `analyze_notes.py`: every decisive single-order judgement, so a pair judged
  consistently contributes twice; *excess* = judgements against dsr minus judgements against g31.
- **pairs lost** — consistent decisive pairs lost with that class named in at least one order.

Quote marks differ between the two sides in 38 % (ru), 20 % (de) and 28 % (fr) of all sampled
pairs, and in 31 / 16 / 29 % of the fidelity losses — so fidelity losses are not a disguised
quotation-mark effect.

### 5.1 Russian — dsr vs g31

| class | judgements vs dsr | vs g31 | excess | pairs lost dsr | pairs lost g31 | vs g37: judgements dsr / g37 |
|---|--:|--:|--:|--:|--:|--:|
| fidelity | 89 | 84 | +5 | 39 | 32 | 46 / 39 |
| naturalness | 55 | 44 | +11 | 25 | 17 | 30 / 24 |
| punctuation | 39 | 15 | **+24** | 14 | 6 | 20 / 7 |
| idiom-calque | 29 | 7 | **+22** | 13 | 4 | 14 / 2 |
| collocation | 18 | 12 | +6 | 9 | 4 | 9 / 8 |
| grammar-agreement | 8 | 2 | +6 | 3 | 1 | 2 / 3 |
| register | 7 | 10 | −3 | 3 | 3 | 5 / 3 |
| terminology | 7 | 7 | 0 | 3 | 3 | 7 / 1 |
| case-government, nonexistent-word, other | 8 | 7 | +1 | 5 | 2 | 11 / 8 |
| **total** | **260** | **188** | **+72** | | | 144 / 95 |

Examples (consistent g31 wins; punctuation examples are in §6):

**fidelity**
- [248] note `fidelity/fidelity` — the one gross cell shift in the 400-pair sample: dsr carries a
  neighbouring sentence's translation.
  - src: ‘He has every requirement.
  - dsr: Не припомню, когда я видел что-либо столь прекрасное».
  - g31: «У него есть все необходимые качества.
- [240] note `fidelity/fidelity`
  - src: With hardly a word spoken, but with a kindly eye, he waved me to an armchair, threw across his case of cigars, and indicated a spirit case and a gasogene in the corner.
  - dsr: Почти не говоря ни слова, но с добрым взглядом, он поманил меня к креслу, бросил мне свой портсигар с сигарами и указал на графин со спиртным и сифон в углу.
  - g31: Почти не проронив ни слова, но с добрым взглядом, он жестом пригласил меня в кресло, бросил мне свой портсигар и указал на ящик со спиртным и сифон в углу.
- [18] note `fidelity/fidelity` — the surname Angel translated as a noun.
  - src: Angel—was a cashier in an office in Leadenhall Street—and—”
  - dsr: Ангел — был кассиром в конторе на Лиденхолл-стрит — и—»
  - g31: Энджел — был кассиром в конторе на Леденхолл-стрит — и…

**naturalness**
- [16] note `naturalness/naturalness`
  - src: And your address had been given me.
  - dsr: И ваш адрес мне был дан.
  - g31: И мне дали ваш адрес.
- [232] note `naturalness/naturalness`
  - src: What could it be, once more?
  - dsr: Что это могло быть, опять же?
  - g31: Что бы это могло быть, ещё раз?
- [246] note `naturalness/naturalness`
  - src: Your recent services to one of the royal houses of Europe have shown that you are one who may safely be trusted with matters which are of an importance which can hardly be exaggerated.
  - dsr: Ваши недавние услуги одному из королевских домов Европы показали, что вы — человек, которому можно безопасно доверить дела, важность которых трудно переоценить.
  - g31: Ваши недавние услуги одному из королевских домов Европы показали, что вам можно смело доверить дела, важность которых трудно переоценить.

**idiom-calque**
- [80] note `idiom-calque/idiom-calque`
  - src: I am sorry that Miss Sutherland has troubled you about this little matter, for I think it is far better not to wash linen of the sort in public.
  - dsr: Мне жаль, что мисс Сазерленд побеспокоила вас по этому пустяковому делу, ибо я думаю, что гораздо лучше не стирать такое бельё на людях.
  - g31: Мне жаль, что мисс Сазерленд побеспокоила вас по этому пустяковому делу, ибо я считаю, что гораздо лучше не выносить сор из избы.
- [173] note `idiom-calque/idiom-calque`
  - src: She will not be up, so that we shall have a clear field.
  - dsr: Она ещё не встанет, так что у нас будет свободное поле.
  - g31: Она еще не встанет, так что у нас будет полная свобода действий.
- [155] note `idiom-calque/idiom-calque`
  - src: Now, when you see that a young lady, otherwise neatly dressed, has come away from home with odd boots, half-buttoned, it is no great deduction to say that she came away in a hurry.”
  - dsr: Итак, когда вы видите, что молодая леди, в остальном опрятно одетая, ушла из дома в разных ботинках, наполовину застёгнутых, нет большой дедукции сказать, что она ушла в спешке.»
  - g31: Теперь, когда видишь, что молодая леди, в остальном опрятно одетая, ушла из дома в разных ботинках, застегнутых наполовину, нетрудно сделать вывод, что она ушла в спешке.

**collocation**
- [52] note `collocation/collocation`
  - src: He made a sweeping bow to the three of us and walked quietly off in the custody of the detective.
  - dsr: Он сделал широкий поклон нам троим и спокойно ушёл под конвоем сыщика.
  - g31: Он отвесил нам троим широкий поклон и тихо удалился под конвоем детектива.
- [132] note `collocation/collocation`
  - src: It’s not a very large affair, and of late years it has not done more than just give me a living.
  - dsr: Это не очень большое предприятие, и в последние годы оно приносило мне лишь на жизнь.
  - g31: Это не очень большое дело, и в последние годы оно едва позволяет мне сводить концы с концами.

**grammar-agreement**
- [198] note `grammar-agreement/grammar-agreement` — the speaker is Irene Adler; the masculine
  verb is wrong in the book, though the sentence alone does not settle the gender.
  - src: Then I, rather imprudently, wished you good-night, and started for the Temple to see my husband.
  - dsr: Затем я, довольно неосмотрительно, пожелал вам доброй ночи и отправился в Темпл, чтобы повидать своего мужа.
  - g31: Затем я, довольно неосмотрительно, пожелала вам доброй ночи и направилась в Темпл, чтобы повидаться с мужем.
- [218] note `fidelity/grammar-agreement`
  - src: They were admirable things for the observer—excellent for drawing the veil from men’s motives and actions.
  - dsr: Они были превосходной вещью для наблюдателя — отличной для срывания покрова с мотивов и поступков людей.
  - g31: Они были замечательными вещами для наблюдателя — превосходными для того, чтобы срывать завесу с побуждений и поступков людей.

**register**
- [68] note `register/register`
  - src: He’s a young man, Mr. Merryweather, but he is at the head of his profession, and I would rather have my bracelets on him than on any criminal in London.
  - dsr: Он молод, мистер Мерриуэзер, но он стоит во главе своей профессии, и я бы предпочёл надеть на него свои наручники, чем на любого преступника в Лондоне».
  - g31: Он молодой человек, мистер Мерриуэзер, но он стоит во главе своей профессии, и я предпочёл бы защёлкнуть на нём свои браслеты, чем на любом другом преступнике в Лондоне.
- [373] note `register/register`
  - src: “Why did you come away to consult me in such a hurry?” asked Sherlock Holmes, with his finger-tips together and his eyes to the ceiling.
  - dsr: «Почему вы пришли ко мне за консультацией в такой спешке?» — спросил Шерлок Холмс, сложив кончики пальцев вместе и устремив взгляд в потолок.
  - g31: — Почему вы так поспешно приехали ко мне за советом? — спросил Шерлок Холмс, соединив кончики пальцев и устремив взгляд в потолок.

**terminology**
- [216] note `terminology/terminology` — an English word left in the Russian text.
  - src: They got £ 4700 for the goodwill and interest, which wasn’t near as much as father could have got if he had been alive.”
  - dsr: Они получили 4700 фунтов за goodwill и долю, что было далеко не столько, сколько отец мог бы получить, если бы был жив.
  - g31: Они выручили 4700 фунтов за дело и проценты, что было совсем не так много, как мог бы получить отец, если бы был жив».
- [207] note `fidelity/terminology`
  - src: There is a date, you see, but no superscription except Leadenhall Street, which is rather vague.
  - dsr: Видите, есть дата, но нет надписи, кроме Лиденхолл-стрит, что довольно неопределённо.
  - g31: Там есть дата, как видите, но нет адреса, кроме Леденхолл-стрит, что довольно расплывчато.

### 5.2 German — dsr vs g31

| class | judgements vs dsr | vs g31 | excess | pairs lost dsr | pairs lost g31 | vs g37: judgements dsr / g37 |
|---|--:|--:|--:|--:|--:|--:|
| fidelity | 64 | 57 | +7 | 33 | 25 | 36 / 25 |
| naturalness | 61 | 37 | **+24** | 23 | 14 | 34 / 19 |
| punctuation | 43 | 25 | +18 | 12 | 8 | 15 / 10 |
| idiom-calque | 47 | 9 | **+38** | 20 | 6 | 25 / 10 |
| collocation | 38 | 16 | **+22** | 17 | 6 | 24 / 3 |
| grammar-agreement | 18 | 7 | +11 | 9 | 4 | 7 / 6 |
| register | 10 | 12 | −2 | 4 | 6 | 5 / 6 |
| terminology | 20 | 6 | +14 | 14 | 3 | 7 / 3 |
| case-government, nonexistent-word, other | 11 | 12 | −1 | 7 | 7 | 9 / 9 |
| **total** | **312** | **181** | **+131** | | | 162 / 91 |

**fidelity**
- [175] note `fidelity/fidelity` — the Temple (the Inns of Court) rendered as a temple.
  - src: On the issue of this question depended whether I should continue my work at Briony Lodge, or turn my attention to the gentleman’s chambers in the Temple.
  - dsr: Von der Entscheidung dieser Frage hing es ab, ob ich meine Arbeit in Briony Lodge fortsetzen oder meine Aufmerksamkeit den Gemächern des Herrn im Tempel zuwenden sollte.
  - g31: Von der Klärung dieser Frage hing ab, ob ich meine Arbeit in Briony Lodge fortsetzen oder meine Aufmerksamkeit auf die Räumlichkeiten des Herrn im Temple richten sollte.
- [1] note `fidelity/fidelity`
  - src: A blow was struck, and in an instant the lady, who had stepped from her carriage, was the centre of a little knot of flushed and struggling men, who struck savagely at each other with their fists and sticks.
  - dsr: Ein Schlag wurde ausgeteilt, und im Nu war die Dame, die aus ihrem Wagen gestiegen war, die Mitte eines kleinen Knäuels erregter und kämpfender Männer, die wild mit Fäusten und Stöcken aufeinander einschlugen.
  - g31: Ein Schlag wurde geführt, und im Nu war die Dame, die aus ihrer Kutsche gestiegen war, das Zentrum eines kleinen Knäuels aus geröteten und sich prügelnden Männern, die mit ihren Fäusten und Stöcken wild aufeinander einschlugen.
- [368] note `fidelity/fidelity`
  - src: “Well, perhaps, after all, it is of some little use,” he remarked.
  - dsr: „Nun, vielleicht ist es doch von einigem Nutzen“, bemerkte er.
  - g31: „Nun, vielleicht ist es schließlich von einigem kleinen Nutzen“, bemerkte er.

**naturalness**
- [274] note `naturalness/naturalness`
  - src: “All right,” said Jones with a stare and a snigger.
  - dsr: „In Ordnung“, sagte Jones mit einem Starren und einem Kichern.
  - g31: „Schon gut“, sagte Jones mit einem starren Blick und einem Kichern.
- [179] note `naturalness/naturalness`
  - src: Presently he emerged, looking even more flurried than before.
  - dsr: Gleich darauf kam er heraus, noch verstörter aussehend als zuvor.
  - g31: Kurz darauf kam er heraus und sah noch aufgeregter aus als zuvor.
- [111] note `naturalness/naturalness`
  - src: I wasn’t best pleased, Mr. Holmes, when she married again so soon after father’s death, and a man who was nearly fifteen years younger than herself.
  - dsr: Ich war nicht gerade erfreut, Mr. Holmes, als sie so bald nach Vaters Tod wieder heiratete, und einen Mann, der fast fünfzehn Jahre jünger war als sie selbst.
  - g31: Ich war nicht gerade erfreut, Mr. Holmes, als sie so bald nach Vaters Tod wieder heiratete, und zwar einen Mann, der fast fünfzehn Jahre jünger war als sie selbst.

**idiom-calque**
- [246] note `idiom-calque/idiom-calque`
  - src: When I had listened to all they had to tell, I began to walk up and down near Briony Lodge once more, and to think over my plan of campaign.
  - dsr: Als ich alles angehört hatte, was sie zu erzählen hatten, begann ich wieder in der Nähe von Briony Lodge auf und ab zu gehen und über meinen Plan der Kampagne nachzudenken.
  - g31: Als ich mir alles angehört hatte, was sie mir zu sagen hatten, begann ich wieder in der Nähe der Briony Lodge auf und ab zu gehen und über meinen Schlachtplan nachzudenken.
- [249] note `idiom-calque/idiom-calque`
  - src: When you raise your cry of fire, it will be taken up by quite a number of people.
  - dsr: Wenn Sie Ihren Feuerschrei erheben, wird er von einer ganzen Reihe von Leuten aufgenommen.
  - g31: Wenn Sie Ihren Ruf „Feuer“ ausstoßen, wird er von einer ganzen Reihe von Leuten aufgegriffen werden.
- [205] note `idiom-calque/idiom-calque`
  - src: The larger crimes are apt to be the simpler, for the bigger the crime the more obvious, as a rule, is the motive.
  - dsr: Die größeren Verbrechen sind geneigt, die einfacheren zu sein, denn je größer das Verbrechen, desto offensichtlicher ist in der Regel das Motiv.
  - g31: Die größeren Verbrechen sind meist die einfacheren, denn je größer das Verbrechen, desto offensichtlicher ist in der Regel das Motiv.

**collocation**
- [164] note `collocation/collocation`
  - src: Mr. Jabez Wilson laughed heavily.
  - dsr: Mr. Jabez Wilson lachte schwer.
  - g31: Mr. Jabez Wilson lachte schwerfällig.
- [252] note `collocation/collocation`
  - src: Women are naturally secretive, and they like to do their own secreting.
  - dsr: Frauen sind von Natur aus verschwiegen, und sie verrichten ihre eigene Geheimhaltung gern selbst.
  - g31: Frauen sind von Natur aus verschwiegen und sie verbergen ihre Dinge gerne selbst.
- [47] note `collocation/collocation`
  - src: From all I hear it is splendid pay and very little to do.’
  - dsr: Nach allem, was ich höre, ist der Lohn prächtig und die Arbeit sehr gering.“
  - g31: Nach allem, was ich höre, ist es eine glänzende Bezahlung und man hat nur sehr wenig zu tun.“

**grammar-agreement**
- [54] note `grammar-agreement/grammar-agreement`
  - src: He is as brave as a bulldog and as tenacious as a lobster if he gets his claws upon anyone.
  - dsr: Er ist so mutig wie ein Bulldogge und so zäh wie ein Hummer, wenn er seine Krallen an jemanden bekommt.
  - g31: Er ist so mutig wie eine Bulldogge und so hartnäckig wie ein Hummer, wenn er seine Klauen in jemanden geschlagen hat.
- [162] note `grammar-agreement/grammar-agreement` (excerpt of the dsr/g31 difference: „mit einem schweren Pelzboa“ vs „mit einer schweren Pelzboa“)
  - src: Looking over his shoulder, I saw that on the pavement opposite there stood a large woman with a heavy fur boa round her neck, and a large curling red feather in a broad-brimmed hat which was tilted in a coquettish Duchess of Devonshire fashion over her ear.
  - dsr: Über seine Schulter blickend, sah ich, dass auf dem gegenüberliegenden Bürgersteig eine große Frau stand, mit einem schweren Pelzboa um den Hals und einer großen, sich kräuselnden roten Feder in einem breitkrempigen Hut, der in einer koketten Herzogin-von-Devonshire-Manier über ihr Ohr geneigt war.
  - g31: Als ich über seine Schulter blickte, sah ich, dass auf dem gegenüberliegenden Bürgersteig eine große Frau stand, mit einer schweren Pelzboa um den Hals und einer großen, sich kräuselnden roten Feder an einem breitkrempigen Hut, der kokett im Stil der Herzogin von Devonshire über ihrem Ohr saß.
- [180] note `grammar-agreement/grammar-agreement`
  - src: Saturday would suit them better than any other day, as it would give them two days for their escape.
  - dsr: Samstag würde ihnen besser passen als jeder andere Tag, da es ihnen zwei Tage für ihre Flucht geben würde.
  - g31: Der Samstag würde ihnen besser passen als jeder andere Tag, da er ihnen zwei Tage für ihre Flucht geben würde.

**register**
- [301] note `register/fidelity` — Watson addresses a client with *du*.
  - src: “I can quite understand your thinking so,” I said.
  - dsr: „Ich kann deine Denkweise gut verstehen“, sagte ich.
  - g31: „Ich kann durchaus verstehen, dass Sie das denken“, sagte ich.
- [336] note `register/register`
  - src: “Oh, if you say so, Mr. Jones, it is all right,” said the stranger with deference.
  - dsr: „Oh, wenn Sie das sagen, Mr. Jones, ist es in Ordnung“, sagte der Fremde mit Respekt.
  - g31: „Oh, wenn Sie das sagen, Mr. Jones, dann ist es in Ordnung“, sagte der Fremde ehrerbietig.

**terminology**
- [277] note `terminology/terminology`
  - src: “And now I will show you what is really a very interesting study, Mr. Windibank,” Holmes continued.
  - dsr: „Und nun werde ich Ihnen zeigen, was wirklich ein sehr interessantes Studium ist, Herr Windibank“, fuhr Holmes fort.
  - g31: „Und nun werde ich Ihnen zeigen, was wirklich eine sehr interessante Studie ist, Herr Windibank“, fuhr Holmes fort.
- [307] note `terminology/terminology`
  - src: “I started off, Mr. Holmes, but when I got to that address it was a manufactory of artificial knee-caps, and no one in it had ever heard of either Mr. William Morris or Mr. Duncan Ross.”
  - dsr: „Ich machte mich auf den Weg, Mr. Holmes, aber als ich an diese Adresse kam, war es eine Fabrik für künstliche Kniekappen, und niemand dort hatte jemals von Mr. William Morris oder Mr. Duncan Ross gehört.“
  - g31: „Ich machte mich auf den Weg, Mr. Holmes, aber als ich an diese Adresse kam, war es eine Fabrik für künstliche Kniescheiben, und niemand dort hatte jemals von Mr. William Morris oder Mr. Duncan Ross gehört.“
- [52] note `terminology/terminology`
  - src: He disappeared into his bedroom and returned in a few minutes in the character of an amiable and simple-minded Nonconformist clergyman.
  - dsr: Er verschwand in seinem Schlafzimmer und kehrte in wenigen Minuten in der Gestalt eines liebenswürdigen und einfältigen nonkonformistischen Geistlichen zurück.
  - g31: Er verschwand in seinem Schlafzimmer und kehrte nach wenigen Minuten in der Gestalt eines liebenswürdigen und einfältigen freikirchlichen Geistlichen zurück.

### 5.3 French — dsr vs g31

| class | judgements vs dsr | vs g31 | excess | pairs lost dsr | pairs lost g31 | vs g37: judgements dsr / g37 |
|---|--:|--:|--:|--:|--:|--:|
| fidelity | 85 | 72 | +13 | 48 | 32 | 59 / 23 |
| naturalness | 34 | 44 | −10 | 17 | 23 | 18 / 17 |
| punctuation | 45 | 16 | **+29** | 20 | 7 | 17 / 12 |
| idiom-calque | 45 | 23 | **+22** | 24 | 11 | 22 / 8 |
| collocation | 29 | 11 | **+18** | 17 | 5 | 26 / 4 |
| grammar-agreement | 11 | 2 | +9 | 6 | 1 | 4 / 0 |
| register | 7 | 11 | −4 | 4 | 5 | 5 / 7 |
| terminology | 18 | 11 | +7 | 11 | 6 | 10 / 2 |
| case-government, nonexistent-word, other, unspecified | 23 | 15 | +8 | 13 | 7 | 4 / 11 |
| **total** | **297** | **205** | **+92** | | | 165 / 84 |

French is the one language where DeepSeek is judged *more* natural than g31 (naturalness −10),
and the only one where fidelity is a sizeable part of the excess — and against g37 fidelity is
the dominant gap (59 vs 23 judgements).

**fidelity**
- [161] note `fidelity/fidelity` — "the paper" is the newspaper.
  - src: Mr. Jabez Wilson started up in his chair, with his forefinger upon the paper, but his eyes upon my companion.
  - dsr: M. Jabez Wilson se redressa sur sa chaise, l'index sur le papier, mais les yeux sur mon compagnon.
  - g31: Monsieur Jabez Wilson sursauta sur sa chaise, l'index sur le journal, mais les yeux fixés sur mon compagnon.
- [397] note `fidelity/fidelity`
  - src: “‘It’s only four hours a day, and I should not think of leaving,’ said I.
  - dsr: « Ce n’est que quatre heures par jour, et je ne devrais pas songer à partir, dis-je.
  - g31: « Ce n’est que quatre heures par jour, et je ne songerais pas à partir », dis-je.
- [220] note `fidelity/fidelity`
  - src: This American had started from London when he was young, and he wanted to do the old town a good turn.
  - dsr: Cet Américain avait commencé à Londres quand il était jeune, et il voulait rendre un bon service à la vieille ville. »
  - g31: « Cet Américain était parti de Londres quand il était jeune, et il voulait rendre un bon service à la vieille ville. »

**naturalness**
- [209] note `naturalness/naturalness`
  - src: There was the end of this tangled clue.
  - dsr: Là était le bout de cet indice embrouillé.
  - g31: C'était là le bout de cet indice embrouillé.
- [257] note `naturalness/naturalness`
  - src: You, of course, saw that everyone in the street was an accomplice.
  - dsr: Vous, bien sûr, avez vu que tout le monde dans la rue était un complice.
  - g31: Vous avez, bien sûr, vu que tout le monde dans la rue était un complice.

**idiom-calque**
- [109] note `idiom-calque/idiom-calque`
  - src: I shall be all anxiety.”
  - dsr: Je serai tout anxiété. »
  - g31: Je serai dans l'attente la plus anxieuse. »
- [83] note `idiom-calque/idiom-calque`
  - src: I am sorry that Miss Sutherland has troubled you about this little matter, for I think it is far better not to wash linen of the sort in public.
  - dsr: Je suis désolé que miss Sutherland vous ait dérangé pour cette petite affaire, car je pense qu'il est bien mieux de ne pas laver ce genre de linge en public.
  - g31: Je regrette que Mlle Sutherland vous ait importuné au sujet de cette petite affaire, car je pense qu'il vaut bien mieux ne pas laver le linge de ce genre en public.
- [305] note `idiom-calque/idiom-calque`
  - src: “I make nothing of it,” I answered frankly.
  - dsr: « Je n'en fais rien, répondis-je franchement.
  - g31: « Je n'en pense rien, » répondis-je franchement.

**collocation**
- [139] note `collocation/collocation`
  - src: It is the first Saturday night for seven-and-twenty years that I have not had my rubber.”
  - dsr: C'est le premier samedi soir depuis vingt-sept ans que je n'ai pas eu mon whist.
  - g31: C'est le premier samedi soir depuis vingt-sept ans que je n'ai pas fait ma partie de whist. »
- [259] note `collocation/collocation`
  - src: Your wedding was arranged, then, for the Friday.
  - dsr: Votre mariage était donc arrangé pour le vendredi.
  - g31: Votre mariage était donc prévu pour le vendredi.
- [332] note `collocation/collocation`
  - src: “No legal papers or certificates?”
  - dsr: « Aucun papier légal ni certificat ? »
  - g31: « Aucun document juridique ou certificat ? »

**grammar-agreement**
- [18] note `grammar-agreement/grammar-agreement`
  - src: As you observe, this woman had plush upon her sleeves, which is a most useful material for showing traces.
  - dsr: Comme vous le remarquez, cette femme avait du peluche sur ses manches, ce qui est un matériau très utile pour révéler des traces.
  - g31: Comme vous l'observez, cette femme avait de la peluche sur ses manches, ce qui est une matière très utile pour révéler des traces.
- [169] note `grammar-agreement/grammar-agreement` (the difference: « une redingote d'un respectabilité oppressante » vs « d'une respectabilité oppressante »)
  - src: On entering his room, I found Holmes in animated conversation with two men, one of whom I recognised as Peter Jones, the official police agent, while the other was a long, thin, sad-faced man, with a very shiny hat and oppressively respectable frock-coat.
  - dsr: En entrant dans sa chambre, je trouvai Holmes en conversation animée avec deux hommes, dont l'un que je reconnus comme Peter Jones, l'agent de police officiel, tandis que l'autre était un homme long, mince, au visage triste, avec un chapeau très brillant et une redingote d'un respectabilité oppressante.
  - g31: En entrant dans sa chambre, j'ai trouvé Holmes en pleine conversation animée avec deux hommes, dont l'un que j'ai reconnu comme étant Peter Jones, l'agent de police officiel, tandis que l'autre était un homme grand, mince, au visage triste, avec un chapeau très brillant et une redingote d'une respectabilité oppressante.
- [323] note `grammar-agreement/grammar-agreement`
  - src: “It missed him, then?”
  - dsr: « Il l'a manquée, alors ? »
  - g31: « Elle l'a donc manqué ? »

**register**
- [330] note `idiom-calque/register`
  - src: “My dear Holmes,” said I, “this is too much.
  - dsr: « Mon cher Holmes, dis-je, c'est trop.
  - g31: « Mon cher Holmes, dis-je, c'en est trop.
- [346] note `fidelity/register`
  - src: “Smart fellow, that,” observed Holmes as we walked away.
  - dsr: « Garçon intelligent, celui-là », observa Holmes tandis que nous nous éloignions.
  - g31: « Un type intelligent, celui-là, » fit remarquer Holmes tandis que nous nous éloignions.

**terminology**
- [307] note `terminology/terminology`
  - src: “I started off, Mr. Holmes, but when I got to that address it was a manufactory of artificial knee-caps, and no one in it had ever heard of either Mr. William Morris or Mr. Duncan Ross.”
  - dsr: « Je suis parti, monsieur Holmes, mais quand je suis arrivé à cette adresse, c'était une fabrique de genouillères artificielles, et personne là-bas n'avait jamais entendu parler ni de M. William Morris ni de M. Duncan Ross. »
  - g31: « Je suis parti, Monsieur Holmes, mais quand je suis arrivé à cette adresse, c'était une fabrique de rotules artificielles, et personne là-dedans n'avait jamais entendu parler ni de Monsieur William Morris ni de Monsieur Duncan Ross. »
- [46] note `terminology/terminology` (the difference: « un masque noir » vs « un loup noir » for *vizard mask*)
  - src: He carried a broad-brimmed hat in his hand, while he wore across the upper part of his face, extending down past the cheekbones, a black vizard mask, which he had apparently adjusted that very moment, for his hand was still raised to it as he entered.
  - dsr: Il tenait un chapeau à larges bords à la main, tandis qu'il portait sur la partie supérieure de son visage, s'étendant au-delà des pommettes, un masque noir, qu'il avait apparemment ajusté à l'instant même, car sa main était encore levée vers lui lorsqu'il entra.
  - g31: Il tenait à la main un chapeau à larges bords, tandis qu'il portait sur la partie supérieure de son visage, descendant jusqu'au-delà des pommettes, un loup noir, qu'il avait apparemment ajusté à cet instant même, car sa main était encore levée vers lui alors qu'il entrait.
- [383] note `terminology/terminology`
  - src: “You may address me as the Count Von Kramm, a Bohemian nobleman.
  - dsr: « Vous pouvez m'appeler le comte Von Kramm, un noble bohémien.
  - g31: « Vous pouvez m'appeler le comte Von Kramm, un noble de Bohême. »

## 6. The punctuation question

The Russian notes count punctuation 39 : 15 against dsr. Every one of the 54 judgements
(26 + 12 pairs) was read and sorted. The rubric already tells the judge to tie on dash-vs-guillemet
dialogue style, and no punctuation judgement falls on a pair that differs only in ё/е or spacing.

### Russian — 39 judgements against dsr

| kind | judgements | real defect? |
|---|--:|---|
| closing/opening quote missing where the source cell has one | 12 | yes |
| stray closing `»` or dash-inside-guillemet where the source cell has none | 7 | yes |
| attribution fragment without its dialogue dash (`спросил я, входя.`) | 5 | yes |
| period inside the closing guillemet (`.»` instead of `».`) | 6 | yes (typography) |
| ASCII `"` instead of `«»` | 2 | yes (typography) |
| doubled `»»` after a nested quote | 2 | yes (typography) |
| missing comma around a parenthetical (`вернее мадам`) | 1 | yes |
| guillemet vs dash dialogue layout, split-quote layout | 4 | **no — convention** |
| **total** | **39** | **35 real, 4 convention** |

Against g31 the 15 judgements split 12 real (unclosed or doubled `— «`, missing closing marks)
and 3 that penalise the correct side (e.g. [177] marks g31 down for the `»` the source's `”`
demands).

Examples, all consistent g31 wins with note `punctuation/punctuation`:

- missing closing mark — [130]
  - src: It would be a pity to miss it.”
  - dsr: Было бы жаль это пропустить.
  - g31: Было бы жаль пропустить это».
- quotes dropped entirely — [327]
  - src: “Not in the least.”
  - dsr: Нисколько.
  - g31: «Нисколько».
- stray closing mark — [65]
  - src: Her jacket was black, with black beads sewn upon it, and a fringe of little black jet ornaments.
  - dsr: Её жакет был чёрным, с нашитыми чёрными бусинами и бахромой из маленьких чёрных гагатовых украшений».
  - g31: Ее жакет был черным, с нашитыми на него черными бусинами и бахромой из маленьких черных гагатовых украшений.
- stray dash inside guillemets — [395]
  - src: “‘Oh, never mind about that, Mr. Wilson!
  - dsr: «— Ах, не беспокойтесь об этом, мистер Уилсон!»
  - g31: «О, не беспокойтесь об этом, мистер Уилсон!
- missing attribution dash — [82]
  - src: I asked as I entered.
  - dsr: спросил я, входя.
  - g31: — спросил я, входя.
- `.»` — [229]
  - src: We must be at Briony Lodge to meet her.”
  - dsr: Мы должны быть в Брайони-Лодж, чтобы встретить её.»
  - g31: Мы должны быть у Брайони-Лодж, чтобы встретить ее».
- doubled `»»` — [48]
  - src: Have the goodness, also, when you address me always to say ‘sir’ and ‘please.’”
  - dsr: Будьте любезны также, обращаясь ко мне, всегда говорить «сэр» и «пожалуйста»».
  - g31: Будьте добры также, обращаясь ко мне, всегда говорить «сэр» и «пожалуйста».
- convention only (counted as not a defect) — [353]
  - src: “This is a very unexpected turn of affairs,” said I; “and what then?”
  - dsr: «Это очень неожиданный поворот событий», — сказал я; «и что же дальше?»
  - g31: «Это весьма неожиданный поворот дел, — сказал я, — и что же дальше?»

**Verdict for Russian: the punctuation penalty is a real defect (35 of 39), not a convention
the judge penalises.** Its core is that DeepSeek does not carry the source cell's quotation marks
across into the target cell — it drops closing marks, invents them, and leaves speech-tag
fragments without the dash.

### German — 43 against dsr, 25 against g31

| kind (against dsr) | judgements |
|---|--:|
| stray closing `“` where the source cell has none | 13 |
| missing closing/opening mark the source cell has | 5 |
| wrong nesting level (`„` for `‘`, `“` for `’`) | 3 |
| typography: ASCII `"` (2), no comma after `„…!“` before the speech tag (1), unspaced `—` (1) | 4 |
| **judge prefers a self-contained quote over a source-faithful partial one** | **16** |
| judge penalises the balanced side (noise) | 2 |

Real: 25 of 43. The 16 "self-contained preference" judgements are cells inside a multi-sentence
quotation where dsr mirrors the source (opens without closing, or closes without opening) and g31
wraps the cell in its own pair of marks — e.g. [143], note `punctuation/punctuation`:

- src: It was a pretty expensive joke for them, for it cost them two and thirty pounds.”
- dsr: Es war ein ziemlich teurer Scherz für sie, denn er kostete sie zweiunddreißig Pfund.“
- g31: „Es war ein ziemlich teurer Scherz für sie, denn er kostete sie zweiunddreißig Pfund.“

and a real one, [202], note `punctuation/punctuation`:

- src: The driver looked twice at such a shabby fare, but I jumped in before he could object.
- dsr: Der Fahrer sah zweimal auf solch einen schäbigen Fahrgast, aber ich sprang hinein, bevor er Einwand erheben konnte.“
- g31: Der Kutscher sah sich einen so schäbigen Fahrgast zweimal an, aber ich sprang hinein, bevor er Einwände erheben konnte.

The 25 against g31 were not sorted one by one; they are of the same two kinds (stray closing `“`,
e.g. [227]; mismatched wrap, e.g. [196]).

### French — 45 against dsr, 16 against g31

| kind (against dsr) | judgements |
|---|--:|
| English comma inside the closing guillemet (`…, » dit-il`) | 10 |
| unspaced `—` | 2 |
| missing mark the source cell has | 13 |
| stray `»` where the source cell has none | 5 |
| **judge prefers a self-contained quote over a source-faithful partial one** | **11** |
| **judge penalises the standard French incise inside one guillemet pair** | **4** |

Real: 30 of 45. Example of the commonest real one, [324], note `punctuation/punctuation`:

- src: “It was only a joke at first,” groaned our visitor.
- dsr: « Ce n'était qu'une plaisanterie au début, » gémit notre visiteur.
- g31: « Ce n'était qu'une plaisanterie au début », gémit notre visiteur.

and of a convention the judge penalised, [331], note `punctuation/punctuation` — dsr's single
guillemet pair around the incise is standard French:

- src: “Never mind,” said Holmes, laughing; “it is my business to know things.
- dsr: « Peu importe, dit Holmes en riant ; c'est mon métier de savoir les choses.
- g31: « Peu importe », dit Holmes en riant ; « c'est mon métier de savoir les choses. »

g31 has the same comma-inside error ([343], [360], [366] — 6 of its 16 judgements).

### Whole-book cross-check

The judged sample is 400 pairs; a mechanical audit over all 1 624 cells whose source has Latin
letters (`a2/quote_audit.py`, output `a2/quote-audit.txt`) shows the same typographic tics at book
scale. Only columns that do not depend on the dialogue convention are shown.

| lang | arm | cells with ASCII `"` | `“`-opened source, target opens with neither mark nor dash | target starts lower-case | ru `.»` | fr `, »` | unspaced `word—word` |
|---|---|--:|--:|--:|--:|--:|--:|
| ru | ds | 30 | 44 | 12 | 58 | — | 2 |
| ru | dsr | 27 | 44 | 12 | 58 | — | 2 |
| ru | g31 | 1 | 8 | 6 | 2 | — | 0 |
| ru | g37 | 2 | 16 | 4 | 3 | — | 0 |
| de | ds | 27 | 6 | 22 | — | — | 21 |
| de | dsr | 25 | 6 | 22 | — | — | 21 |
| de | g31 | 0 | 4 | 18 | — | — | 1 |
| de | g37 | 0 | 31 | 23 | — | — | 0 |
| fr | ds | 11 | 16 | 19 | — | 20 | 19 |
| fr | dsr | 6 | 16 | 19 | — | 20 | 19 |
| fr | g31 | 0 | 2 | 17 | — | 11 | 1 |
| fr | g37 | 1 | 7 | 22 | — | 4 | 0 |

The convention difference is also visible: g31/fr ends 225 cells with `»` whose source cell has no
closing `”` (dsr: 49) — it wraps every sentence of a multi-sentence quotation in its own pair, which
is exactly what the judge rewards in §6's "self-contained" rows.

**Verdict overall:** in Russian punctuation is a real, mostly mechanical defect; in German and
French it is about two-thirds real (25/43, 30/45) and one-third a judge preference that the rubric
should settle. The real part is cheap to fix and the judge part is cheap to neutralise (§8).

## 7. Repair on DeepSeek

`ds` and `dsr` share one translation cache, so every difference between them is the repair pass.
Of the 1 602 comparable sentences the repair changed **11 in Russian (0.69 %), 13 in German
(0.81 %) and 8 in French (0.50 %)** — against 3.0–4.2 % on record for Kimi (EVIDENCE §4.3). All
changed sentences were judged blind, both orders:

| lang | changed | judge prefers dsr (repaired) | prefers ds (original) | tie |
|---|--:|--:|--:|--:|
| ru | 11 | 6 | 3 | 2 |
| de | 13 | 5 | 3 | 5 |
| fr | 8 | 2 | 1 | 5 |
| **all** | **32** | **13** | **7** | **12** |

Every changed sentence, verbatim (verdict = consistent result, then the loser's class per order):

**Russian**

| # | src | ds | dsr | verdict |
|--:|---|---|---|---|
| 1 | Anybody bringing,” &c, &c. | Всякий, кто принесёт,” и т. д., и т. д. | «Всякий, кто принесёт», и т. д., и т. д. | dsr (punctuation/punctuation) |
| 2 | I had the hint from Holmes that this smooth-faced pawnbroker’s assistant was a formidable man—a man who might play a deep game. | Я получил от Холмса намёк, что этот гладколицый помощник ростовщика был formidable человеком — человеком, который мог вести глубокую игру. | Я получил от Холмса намёк, что этот гладколицый помощник ростовщика был грозным человеком — человеком, который мог вести глубокую игру. | dsr (nonexistent-word/other) |
| 3 | I should not have thought there were so many in the whole country as were brought together by that single advertisement. | Я не думал, что во всей стране найдётся столько, сколько собрала одна эта объявление. | Я не думал, что во всей стране найдётся столько, сколько собрало одно это объявление. | dsr (grammar-agreement ×2) |
| 4 | If you two will take the first hansom, Watson and I will follow in the second.” | Если вы двое возьмёте первый кэб, мы с Уотсоном последуем во втором." | Если вы двое возьмёте первый кэб, мы с Уотсоном последуем во втором. | ds (punctuation ×2) |
| 5 | In the present instance I am forced to admit that the facts are, to the best of my belief, unique.” | В настоящем же случае я вынужден признать, что факты, насколько я могу судить, единственны в своём роде." | В настоящем же случае я вынужден признать, что факты, насколько я могу судить, единственны в своём роде. | ds (punctuation ×2) |
| 6 | It looked as if the pair might take an immediate departure, and so necessitate very prompt and energetic measures on my part. | Похоже было, что пара может немедленно уехать, и потому требует от меня самых быстрых и энергичных мер. | Похоже было, что пара может немедленно уехать, и потому это потребует от меня самых быстрых и энергичных мер. | dsr (fidelity ×2) |
| 7 | The King took a heavy chamois leather bag from under his cloak and laid it on the table. | Король достал из-под плаща тяжёлый кожаный мешок из замши и положил его на стол. | Король достал из-под плаща тяжёлый замшевый мешок и положил его на стол. | dsr (collocation/naturalness) |
| 8 | You just read it for yourself, sir.” | Вы только что сами это прочитали, сэр." | Вы только что сами это прочитали, сэр. | ds (punctuation ×2) |
| 9 | “She showed me, as I told you she would.” | «Она показала мне, как я и говорил вам, что покажет». | «Она показала мне, как я и говорил вам, что покажет» | tie |
| 10 | “Then, when the row broke out, I had a little moist red paint in the palm of my hand. | «Затем, когда вспыхнула ссора, у меня в ладони была немного влажной красной краски. | «Затем, когда вспыхнула ссора, у меня в ладони было немного влажной красной краски. | dsr (grammar-agreement ×2) |
| 11 | “Yes, sir; for he had started to England just before it arrived.” | «Да, сэр; ведь он уехал в Англию как раз перед тем, как оно пришло». | «Да, сэр; ведь он уехал в Англию как раз перед тем, как оно пришло» | tie |

**German**

| # | src | ds | dsr | verdict |
|--:|---|---|---|---|
| 1 | And yet I believe that my eyes are as good as yours.” | Und doch glaube ich, dass meine Augen so gut sind wie die Ihren." | Und doch glaube ich, dass meine Augen so gut sind wie die Ihren. | ds (punctuation ×2) |
| 2 | At his fall the guardsmen took to their heels in one direction and the loungers in the other, while a number of better dressed people, who had watched the scuffle without taking part in it, crowded in to help the lady and to attend to the injured man. | Bei seinem Sturz ergriffen die Gardisten in der einen Richtung die Flucht und die Herumtreiber in der anderen, während eine Anzahl besser gekleideter Leute, die den Handgemenge zugesehen hatten, ohne daran teilzunehmen, herbeiströmten, um der Dame zu helfen und sich um den Verletzten zu kümmern. | Bei seinem Sturz ergriffen die Gardisten in der einen Richtung die Flucht und die Herumtreiber in der anderen, während eine Anzahl besser gekleideter Leute, die dem Handgemenge zugesehen hatten, ohne daran teilzunehmen, herbeiströmten, um der Dame zu helfen und sich um den Verletzten zu kümmern. | dsr (case-government ×2) |
| 3 | But that was not quite good enough, Mr. Holmes. | Aber das war nicht gut genug, Mr. Holmes. | Aber das war nicht ganz gut genug, Mr. Holmes. | dsr (fidelity ×2) |
| 4 | For example, you have frequently seen the steps which lead up from the hall to this room.” | Zum Beispiel haben Sie oft die Stufen gesehen, die von der Halle zu diesem Zimmer führen." | Zum Beispiel haben Sie oft die Stufen gesehen, die von der Halle zu diesem Zimmer führen. | ds (punctuation ×2) |
| 5 | He slipped an emerald snake ring from his finger and held it out upon the palm of his hand. | Er streifte einen Smaragd-Schlangenring von seinem Finger und hielt ihn auf der Handfläche seiner Hand hin. | Er streifte einen Smaragd-Schlangenring von seinem Finger und hielt ihn auf der Handfläche hin. | tie (orders disagree) |
| 6 | I shall be happy to give you an opinion upon the subject in the course of a day or two. | Ich werde Ihnen im Laufe von ein oder zwei Tagen gerne eine Meinung zu dem Thema geben.“} 9: | Ich werde Ihnen im Laufe von ein oder zwei Tagen gerne eine Meinung zu dem Thema geben.“ | dsr (other ×2) |
| 7 | I’ve been on his track for years and have never set eyes on him yet.” | Ich bin ihm seit Jahren auf der Spur und habe ihn noch nie zu Gesicht bekommen.“ | Ich bin ihm seit Jahren auf der Spur und habe ihn noch nie zu Gesicht bekommen. | ds (punctuation ×2) |
| 8 | My life is spent in one long effort to escape from the commonplaces of existence. | Mein Leben verbringt ich in einer langen Anstrengung, den Gemeinplätzen des Daseins zu entfliehen. | Mein Leben verbringe ich in einer langen Anstrengung, den Gemeinplätzen des Daseins zu entfliehen. | dsr (grammar-agreement ×2) |
| 9 | There were several people on the pavement at the time, but the greeting appeared to come from a slim youth in an ulster who had hurried by. | Es waren mehrere Leute zu dieser Zeit auf dem Bürgersteig, aber der Gruß schien von einem schlanken jungen Mann in einem Ulster zu kommen, der vorbeigeeilt war. | Es waren zu dieser Zeit mehrere Leute auf dem Bürgersteig, aber der Gruß schien von einem schlanken jungen Mann in einem Ulster zu kommen, der vorbeigeeilt war. | tie |
| 10 | “But, surely, it was obvious. | „Aber, sicherlich, das war offensichtlich.“ | „Aber sicherlich war das offensichtlich.“ | dsr (naturalness ×2) |
| 11 | “My dear doctor, this is a time for observation, not for talk. | „Mein lieber Doktor, dies ist eine Zeit für Beobachtung, nicht für Reden. | „Mein lieber Doktor, dies ist eine Zeit für Beobachtung, nicht für Reden.“ | tie |
| 12 | “On the contrary, my dear sir,” cried the King; “nothing could be more successful. | „Im Gegenteil, mein lieber Herr“, rief der König; „nichts könnte erfolgreicher sein. | „Im Gegenteil, mein lieber Herr“, rief der König; „nichts könnte erfolgreicher sein.“ | tie |
| 13 | “To the Leadenhall Street Post Office, to be left till called for. | „An das Postamt in der Leadenhall Street, postlagernd. | „An das Postamt in der Leadenhall Street, postlagernd.“ | tie |

**French**

| # | src | ds | dsr | verdict |
|--:|---|---|---|---|
| 1 | Be in your chamber then at that hour, and do not take it amiss if your visitor wear a mask.” | Soyez dans votre chambre à cette heure-là, et ne prenez pas mal que votre visiteur porte un masque." | Soyez dans votre chambre à cette heure-là, et ne prenez pas mal que votre visiteur porte un masque. | tie (o1: ds) |
| 2 | If you two will take the first hansom, Watson and I will follow in the second.” | Si vous deux prenez le premier hansom, Watson et moi suivrons dans le second." | Si vous deux prenez le premier hansom, Watson et moi suivrons dans le second. | tie (o1: ds) |
| 3 | It must be bought.” | Il faut l'acheter." | Il faut l'acheter. | tie (o1: ds) |
| 4 | There was a double stream upon the stair, some going up in hope, and some coming back dejected; but we wedged in as well as we could and soon found ourselves in the office.” | Il y avait un double courant sur l'escalier, certains montant pleins d'espoir, et d'autres redescendant déçus ; mais nous nous faufilâmes du mieux que nous pûmes et nous nous retrouvâmes bientôt dans le bureau." | Il y avait un double courant sur l'escalier, certains montant pleins d'espoir, et d'autres redescendant déçus ; mais nous nous faufilâmes du mieux que nous pûmes et nous nous retrouvâmes bientôt dans le bureau. | tie (o1: ds) |
| 5 | There’s no vice in him.” | Il n'y a aucun vice en lui." | Il n'y a aucun vice en lui. | tie (o1: ds) |
| 6 | Your Majesty has indeed committed an indiscretion.” | Votre Majesté a en effet commis une indiscretion. » | Votre Majesté a en effet commis une indiscrétion. » | dsr (nonexistent-word ×2) |
| 7 | “Well, it is just as I have been telling you, Mr. Sherlock Holmes,” said Jabez Wilson, mopping his forehead; “I have a small pawnbroker’s business at Coburg Square, near the City. | « Eh bien, c'est exactement ce que je vous disais, monsieur Sherlock Holmes », dit Jabez Wilson en s'essuyant le front ; « j'ai un petit commerce de prêteur sur gages place Cobourg, près de la Cité. | « Eh bien, c'est exactement ce que je vous disais, monsieur Sherlock Holmes », dit Jabez Wilson en s'essuyant le front ; « j'ai un petit commerce de prêteur sur gages place Saxe-Cobourg, près de la Cité. | ds (fidelity ×2) |
| 8 | “Yes; and he seemed to think, with me, that something had happened, and that I should hear of Hosmer again. | « Oui ; et il semblait penser, comme moi, que quelque chose était arrivé, et que j'aurais de mes nouvelles de Hosmer. | « Oui ; et il semblait penser, comme moi, que quelque chose était arrivé, et que j'aurais de ses nouvelles de Hosmer. | dsr (fidelity ×2) |

What the pass does on this model:

- **Content fixes: 12 sentences, all preferred by the judge** — an English word left in Russian
  (ru 2), agreement and government (ru 3, 10; de 2, 8), a dropped modifier (de 3), a missing
  accent (fr 6), a wrong possessive (fr 8), clumsy phrasing (ru 6, 7; de 10), and a leaked JSON
  fragment `} 9:` (de 6) that validation had let through.
- **Quotation-mark changes: 17 sentences, 1 preferred, 6 lost, 10 tied.** The one win converts a
  stray `”` into `«»` (ru 1). Otherwise the repair model *deletes* a trailing ASCII `"` or `“`
  instead of converting it to the target's closing mark (ru 4, 5, 8; de 1, 4, 7; fr 1–5), drops a
  final period (ru 9, 11), or adds a closing mark the source does not have (de 11–13). Six of the
  seven decisive losses are here.
- **Two neutral edits** (de 5, 9: a redundancy removed, a word order changed), both tied.
- **One glossary-driven regression** (fr 7: `Cobourg` → `Saxe-Cobourg`, which the source does not
  say).

**Verdict: on DeepSeek the repair pass is a no-op at book scale.** It is net positive where it
acts (13 : 7), but it acts on 32 of 4 806 sentence-cells across three languages; against a net
deficit of ≈ 120–150 sentences per language versus g31 (§4), a net +6 is noise. It is free on
gonka, so there is no reason to turn it off, and one cheap fix (convert, never delete, stray
closing quotes) would turn its six losses into wins — but it is not the lever for closing the gap.

## 8. Conclusion

**Does DeepSeek + repair on gonka reach the gemini-3.1-flash-lite anchor? No, in any of the three
languages.**

| lang | vs g31: dsr share (p) | net book-scale deficit vs g31 (extrapolated) | vs g37: dsr share (p) |
|---|--:|--:|--:|
| ru | 38.1 % (0.0037) | ≈ 124 sentences | 36.9 % (0.021) |
| de | 35.1 % (1.4e-04) | ≈ 151 sentences | 33.3 % (0.0021) |
| fr | 39.4 % (0.0039) | ≈ 123 sentences | 29.9 % (9.3e-05) |

German is the furthest from g31; French is the furthest from g37. The Russian gold subset agrees
with the full sample (32.3 %, not significant at n = 31).

What to attack next, per language, ranked by excess judgements against dsr (§5), with the lever
each class calls for:

**Russian**

1. **Punctuation (+24 excess; 35 of 39 real).** Lever: *deterministic typography pass after
   translation* — ASCII `"` → `«»`, `.»` → `».`, collapse `»»`, and a source-mirroring quote rule
   (the target cell carries an opening/closing mark iff the source cell does; an attribution
   fragment after a quote gets `— `), plus one line of prompt guidance saying the same. Evidence:
   §6 table; book scale 58 `.»`, 27 ASCII `"`, 44 unopened quotes in dsr vs 2 / 1 / 8 in g31.
2. **Idiom calques (+22).** Lever: *expression tagging feeding the translate prompt* — the producer
   has no expression tagging yet (`"k":"i"` in `spans` is an italic emphasis run, spec §5.5, not an
   idiom span — corrected by the coordinator 2026-09-22); build a source-side expression pass and pass tagged expressions with "render as the
   target-language idiom, never word by word". The repair prompt already asks for calque fixes and
   fixed none of them on this model, so the fix has to happen at translate time or through
   judge + re-translate. Evidence: [80] «не стирать такое бельё на людях», [173] «свободное поле»,
   [155] «нет большой дедукции сказать».
3. **Naturalness (+11).** Lever: *prompt guidance* — A/B the v5 "prefer the rendering that MIRRORS
   the source" clause on DeepSeek, which appears to follow it more literally than Gemini
   (passive mirrors like [16] «И ваш адрес мне был дан»).

**German**

1. **Idiom calques (+38), the largest single excess in the study.** Lever: *expression tagging +
   judge + re-translate* on tagged sentences. Evidence: [246] „Plan der Kampagne“, [249]
   „Feuerschrei“, [205] „sind geneigt, die einfacheren zu sein“.
2. **Naturalness (+24) and collocation (+22).** Lever: *judge + re-translate* on sentences the
   judge flags, since these are sentence-level choices no glossary covers (e.g. [274] „mit einem
   Starren und einem Kichern“, [164] „lachte schwer“, [252] „verrichten ihre eigene Geheimhaltung“);
   the same mirror-clause A/B as for Russian.
3. **Punctuation (+18) and terminology (+14).** Punctuation: the same typography pass (ASCII `"`,
   spaced dash, comma after `„…!“`) and the source-mirroring rule; *and fix the judge rubric*
   (16 of 43 judgements penalise source-faithful partial quotes). Terminology: *glossary* for
   period and institutional terms (the Temple, Nonconformist, knee-caps) — DeepSeek already
   enforces 100 terms, so the check is whether they are the right ones, not whether there are
   enough.

**French**

1. **Punctuation (+29; 30 of 45 real).** Lever: *deterministic typography pass* (`, »` → ` »,`,
   ASCII `"`, spaced `—`) plus the source-mirroring quote rule; fix the judge rubric so the
   standard French incise and source-faithful partial quotes are ties (15 of 45 judgements).
   Evidence: §6; book scale 20 `, »` and 19 unspaced dashes in dsr vs 11 / 1 in g31.
2. **Idiom calques (+22) and collocation (+18).** Lever: *expression tagging* at translate time
   and *judge + re-translate*. Evidence: [109] « Je serai tout anxiété », [83] « laver ce genre
   de linge en public », [139] « je n'ai pas eu mon whist », [259] « mariage … arrangé ».
3. **Fidelity (+13 vs g31; 59 : 23 judgements vs g37).** Lever: *judge + re-translate* with the
   fidelity rubric — these are single-word meaning errors ([161] « papier » for the newspaper,
   [397] « je ne devrais pas songer » for *I should not think of*) that neither glossary nor
   repair catches.

Cross-language: the typography pass and the rubric fix are the cheapest items on the list and are
worth doing first; they address the only defect class where a large part of the gap is
mechanical. Everything after that (calques, collocation, naturalness) needs a second model
judging DeepSeek's output, because DeepSeek's own repair pass does not see these defects (§7).

## 9. Artifacts and reproduction

| what | where (under `bench-quality/.artifacts/quality-plan/`) |
|---|---|
| Arms | `a2/holmes4-{ds,dsr,g37}-{ru,de,fr}.tbook`, `or/gflashlite-{ru,de,fr}.tbook` |
| Logs, stats, glossaries | `a2/holmes4-*.log`, `a2/holmes4-*stats.jsonl`, `a2/*.glossary.*.json`, `a2/*.lexflagged.json` |
| Operational / structural tables | `a2/ARMS.md`, `a2/armstats-{ru,de,fr}.{json,txt}` (`a2/armstats_a2.py`) |
| Pair sets | `a2/pairs/{dsr-vs-g31,dsr-vs-g37,ds-vs-dsr}-{ru,de,fr}/` — `key.json`, `batch-*`, `verdict-*`, `notes-*`, `result.txt`, `notes.txt`, `gold.txt`; sampling logs `a2/pairs/*.prep.txt` |
| Re-judged empty batches (originals, 0 bytes) | moved aside to the session scratchpad; the replacements are `dsr-vs-g37-de/{verdict,notes}-17-o1.json`, `dsr-vs-g37-fr/{verdict,notes}-05-o1.json` |
| Merged per-pair view (src + both texts + both verdicts + classes) | `a2/merged/<set>.json` (`a2/merge_pairs.py`) |
| Whole-book quote audit | `a2/quote_audit.py` → `a2/quote-audit.txt` |

Reproduction, from `converter/`:

```bash
P=bench-quality/.artifacts/quality-plan/a2/pairs
# judge (only fills missing verdict files; the script already passes --strict-mcp-config)
python3 bench-quality/judge_pairs.py $P/dsr-vs-g37-de --jobs 5 --target-lang German
# per-set analysis
for s in $P/*/; do
  python3 bench-quality/analyze_pairs.py $s | tee $s/result.txt
  python3 bench-quality/analyze_notes.py $s | tee $s/notes.txt
done
for l in ru de fr; do for c in dsr-vs-g31 dsr-vs-g37 ds-vs-dsr; do
  [ -d $P/$c-$l ] && python3 bench-quality/.artifacts/quality-plan/a2/analyze_gold.py \
      $P/$c-$l bench-quality/gold/en-$l.jsonl | tee $P/$c-$l/gold.txt
done; done
# merged view and quote audit
python3 bench-quality/.artifacts/quality-plan/a2/merge_pairs.py
python3 bench-quality/.artifacts/quality-plan/a2/quote_audit.py
```

(`ds-vs-dsr-de/fr` have no `gold.txt`; the silver de/fr gold files are used only for the
direction-only lines in §4.)
