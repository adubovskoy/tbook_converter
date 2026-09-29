# Jev as judge router (`--judge-scope jev`) — scope experiment, 2026-09-22

**Question.** The Jev probe (`jev-systemone-2026-09-22.md`) found, on 300 GOLD-lite
sentences per language, that ranking by Jev's `faithful` probability puts about 2× the
defects into the bottom 15 % that a random sample would. It proposed replacing the 5 %
random half of `--judge-scope flagged` with the Jev-bottom *k* %. This experiment
reruns that on a whole book (1,642 sentences per language) to test it against a
pre-set kill criterion.

**Answer.** **FAIL**, using the kill criterion exactly as written. The
defect-enrichment half passes in all three languages: the Jev-bottom 15 % by
`faithful` has 1.67× / 1.78× / 1.76× the defects per judged sentence of a random 15 %.
The `wrong`-recall half fails in two of three: the bottom 30 % holds 64.3 % of the
Russian `wrong` sentences, but only **58.3 %** of the German and **52.2 %** of the
French, against a 60 % bar. `faithful` works as a defect router, but it misses too
many real mistranslations to be the only thing deciding what the judge sees. The two
other Jev signals, the `verdict` score and min(`faithful`, `natural`), clear both bars
in all three languages. That finding is **post hoc**: I chose those signals after
seeing the data. It justifies a pre-registered retest on a different book, not a
pass (§6).

## 0. Run record

| | |
|---|---|
| data | `.artifacts/quality-plan/a2/holmes4-dsr-{ru,de,fr}.tbook` (DeepSeek + repair arm, Holmes stories 1–4). All non-empty sentences: **1,642 per language**, extracted by `.artifacts/jev/scope/extract.py` into `sent-{lang}.jsonl` |
| Jev | `typesafe/jev-1.13-20260917` through the OpenRouter System One endpoint, the same four questions as the probe (`faithful`, `natural`, `target_lang` as noul; `verdict` as a 3-level score). Answers in `jev-{lang}.jsonl`: 4,926 requests, **0 errors**, p50 365–373 ms, p90 460–478 ms |
| Jev cost | ru $0.0443, de $0.0427, fr $0.0427: **$0.130 total** (≈ $0.026 per 1,000 sentences per language, ~605 input tokens per sentence). All of it was spent by the previous session. This session made **no Jev calls ($0.00)** |
| labels | Sonnet checker (`gold_check.check`, rubric ok / minor / wrong, batches of 10) driven by `.artifacts/jev/scope/run_check.py` through `claude -p --strict-mcp-config --mcp-config '{"mcpServers":{}}'`. 165 batches per language. ru and de were complete on disk. fr had 70 / 165; this session ran the remaining **95 batches** (414 s, 8 parallel jobs, 0 failures, 0 empty outputs). Labels are **silver**: they come from the checker, not a human |
| analysis | `.artifacts/jev/scope/analyse.py` → `analysis.json`, `tables.md`. Random baseline = mean of 5 seeded samples (`random.Random(0..4)`) of the same size. Ties in the ranking are broken by book order |
| lexcheck | `holmes4-dsr-{lang}.tbook.lexflagged.json` (list of flagged source sentences), matched to rows by source text |

## 1. Checker label distribution

| lang | n | ok | minor | wrong | defects | defects / 100 |
|---|--:|--:|--:|--:|--:|--:|
| ru | 1642 | 1352 (82.3%) | 248 (15.1%) | 42 (2.6%) | 290 | 17.7 |
| de | 1642 | 1291 (78.6%) | 291 (17.7%) | 60 (3.7%) | 351 | 21.4 |
| fr | 1642 | 1347 (82.0%) | 249 (15.2%) | 46 (2.8%) | 295 | 18.0 |

"Defect" = `minor` + `wrong`. There are 5–20× as many `wrong` labels as in the probe
(42 / 60 / 46 against 8 / 6 / 3), so the `wrong` numbers below have real sample
sizes.

## 2. Enrichment in the bottom k %

"Share" = the fraction of all defects (or all `wrong`) in the language that fall in
the slice. "×rnd" = that share divided by the share held by a random slice of the same
size (5-seed mean). Because the slice sizes are equal, this is also the ratio of
defects per 100 judged sentences. The last column compares defects per 100 judged
sentences directly.

### 2.1 By `faithful` (the signal under test)

| lang | k | slice | defects (share, ×rnd) | wrong (share, ×rnd) | random: defects / wrong | defects/100 Jev vs rnd |
|---|--:|--:|---|---|---|---|
| ru | 5 % | 82 | 30 (10.3%, ×1.92) | 11 (26.2%, ×4.58) | 15.6 / 2.4 | 36.6 vs 19.0 |
| ru | 10 % | 164 | 53 (18.3%, ×1.61) | 14 (33.3%, ×2.92) | 33.0 / 4.8 | 32.3 vs 20.1 |
| ru | **15 %** | 246 | 78 (26.9%, **×1.67**) | 16 (38.1%, ×2.16) | 46.8 / 7.4 | **31.7 vs 19.0** |
| ru | 20 % | 328 | 106 (36.6%, ×1.74) | 19 (45.2%, ×2.07) | 61.0 / 9.2 | 32.3 vs 18.6 |
| ru | **30 %** | 493 | 151 (52.1%, ×1.62) | 27 (**64.3%**, ×2.01) | 93.2 / 13.4 | 30.6 vs 18.9 |
| de | 5 % | 82 | 40 (11.4%, ×2.27) | 12 (20.0%, ×3.53) | 17.6 / 3.4 | 48.8 vs 21.5 |
| de | 10 % | 164 | 64 (18.2%, ×1.74) | 16 (26.7%, ×2.42) | 36.8 / 6.6 | 39.0 vs 22.4 |
| de | **15 %** | 246 | 93 (26.5%, **×1.78**) | 19 (31.7%, ×2.21) | 52.2 / 8.6 | **37.8 vs 21.2** |
| de | 20 % | 328 | 120 (34.2%, ×1.79) | 23 (38.3%, ×1.98) | 67.2 / 11.6 | 36.6 vs 20.5 |
| de | **30 %** | 493 | 169 (48.1%, ×1.62) | 35 (**58.3%**, ×2.01) | 104.6 / 17.4 | 34.3 vs 21.2 |
| fr | 5 % | 82 | 20 (6.8%, ×1.33) | 4 (8.7%, ×1.33) | 15.0 / 3.0 | 24.4 vs 18.3 |
| fr | 10 % | 164 | 44 (14.9%, ×1.48) | 9 (19.6%, ×2.14) | 29.8 / 4.2 | 26.8 vs 18.2 |
| fr | **15 %** | 246 | 78 (26.4%, **×1.76**) | 17 (37.0%, ×2.50) | 44.2 / 6.8 | **31.7 vs 18.0** |
| fr | 20 % | 328 | 103 (34.9%, ×1.74) | 18 (39.1%, ×2.00) | 59.2 / 9.0 | 31.4 vs 18.0 |
| fr | **30 %** | 493 | 151 (51.2%, ×1.72) | 24 (**52.2%**, ×2.00) | 88.0 / 12.0 | 30.6 vs 17.8 |

Compared with the whole-book defect rate instead of the random mean (17.7 / 21.4 /
18.0 per 100), the 15 % enrichment is ×1.80 / ×1.77 / ×1.76, so the verdict does not
depend on the choice of seeds. The whole-book enrichment is lower than the probe's
×2.0–2.2 at 15 %, and it is flat across k: about ×1.6–1.8 from 10 % to 30 %.

### 2.2 By `verdict` score (0–2 expected level)

| lang | k | slice | defects (share, ×rnd) | wrong (share, ×rnd) | random: defects / wrong | defects/100 Jev vs rnd |
|---|--:|--:|---|---|---|---|
| ru | 5 % | 82 | 28 (9.7%, ×1.79) | 13 (31.0%, ×5.42) | 15.6 / 2.4 | 34.1 vs 19.0 |
| ru | 10 % | 164 | 60 (20.7%, ×1.82) | 17 (40.5%, ×3.54) | 33.0 / 4.8 | 36.6 vs 20.1 |
| ru | 15 % | 246 | 98 (33.8%, ×2.09) | 24 (57.1%, ×3.24) | 46.8 / 7.4 | 39.8 vs 19.0 |
| ru | 20 % | 328 | 125 (43.1%, ×2.05) | 28 (66.7%, ×3.04) | 61.0 / 9.2 | 38.1 vs 18.6 |
| ru | 30 % | 493 | 170 (58.6%, ×1.82) | 29 (69.0%, ×2.16) | 93.2 / 13.4 | 34.5 vs 18.9 |
| de | 5 % | 82 | 34 (9.7%, ×1.93) | 11 (18.3%, ×3.24) | 17.6 / 3.4 | 41.5 vs 21.5 |
| de | 10 % | 164 | 69 (19.7%, ×1.88) | 16 (26.7%, ×2.42) | 36.8 / 6.6 | 42.1 vs 22.4 |
| de | 15 % | 246 | 102 (29.1%, ×1.95) | 23 (38.3%, ×2.67) | 52.2 / 8.6 | 41.5 vs 21.2 |
| de | 20 % | 328 | 138 (39.3%, ×2.05) | 30 (50.0%, ×2.59) | 67.2 / 11.6 | 42.1 vs 20.5 |
| de | 30 % | 493 | 184 (52.4%, ×1.76) | 42 (70.0%, ×2.41) | 104.6 / 17.4 | 37.3 vs 21.2 |
| fr | 5 % | 82 | 25 (8.5%, ×1.67) | 8 (17.4%, ×2.67) | 15.0 / 3.0 | 30.5 vs 18.3 |
| fr | 10 % | 164 | 58 (19.7%, ×1.95) | 13 (28.3%, ×3.10) | 29.8 / 4.2 | 35.4 vs 18.2 |
| fr | 15 % | 246 | 90 (30.5%, ×2.04) | 18 (39.1%, ×2.65) | 44.2 / 6.8 | 36.6 vs 18.0 |
| fr | 20 % | 328 | 117 (39.7%, ×1.98) | 22 (47.8%, ×2.44) | 59.2 / 9.0 | 35.7 vs 18.0 |
| fr | 30 % | 493 | 154 (52.2%, ×1.75) | 28 (60.9%, ×2.33) | 88.0 / 12.0 | 31.2 vs 17.8 |

### 2.3 By min(`faithful`, `natural`)

| lang | k | slice | defects (share, ×rnd) | wrong (share, ×rnd) | random: defects / wrong | defects/100 Jev vs rnd |
|---|--:|--:|---|---|---|---|
| ru | 5 % | 82 | 32 (11.0%, ×2.05) | 13 (31.0%, ×5.42) | 15.6 / 2.4 | 39.0 vs 19.0 |
| ru | 10 % | 164 | 57 (19.7%, ×1.73) | 15 (35.7%, ×3.12) | 33.0 / 4.8 | 34.8 vs 20.1 |
| ru | 15 % | 246 | 84 (29.0%, ×1.79) | 20 (47.6%, ×2.70) | 46.8 / 7.4 | 34.1 vs 19.0 |
| ru | 20 % | 328 | 117 (40.3%, ×1.92) | 22 (52.4%, ×2.39) | 61.0 / 9.2 | 35.7 vs 18.6 |
| ru | 30 % | 493 | 171 (59.0%, ×1.83) | 30 (71.4%, ×2.24) | 93.2 / 13.4 | 34.7 vs 18.9 |
| de | 5 % | 82 | 24 (6.8%, ×1.36) | 5 (8.3%, ×1.47) | 17.6 / 3.4 | 29.3 vs 21.5 |
| de | 10 % | 164 | 64 (18.2%, ×1.74) | 16 (26.7%, ×2.42) | 36.8 / 6.6 | 39.0 vs 22.4 |
| de | 15 % | 246 | 98 (27.9%, ×1.88) | 22 (36.7%, ×2.56) | 52.2 / 8.6 | 39.8 vs 21.2 |
| de | 20 % | 328 | 132 (37.6%, ×1.96) | 29 (48.3%, ×2.50) | 67.2 / 11.6 | 40.2 vs 20.5 |
| de | 30 % | 493 | 187 (53.3%, ×1.79) | 41 (68.3%, ×2.36) | 104.6 / 17.4 | 37.9 vs 21.2 |
| fr | 5 % | 82 | 26 (8.8%, ×1.73) | 7 (15.2%, ×2.33) | 15.0 / 3.0 | 31.7 vs 18.3 |
| fr | 10 % | 164 | 54 (18.3%, ×1.81) | 12 (26.1%, ×2.86) | 29.8 / 4.2 | 32.9 vs 18.2 |
| fr | 15 % | 246 | 85 (28.8%, ×1.92) | 19 (41.3%, ×2.79) | 44.2 / 6.8 | 34.6 vs 18.0 |
| fr | 20 % | 328 | 113 (38.3%, ×1.91) | 26 (56.5%, ×2.89) | 59.2 / 9.0 | 34.5 vs 18.0 |
| fr | 30 % | 493 | 159 (53.9%, ×1.81) | 30 (65.2%, ×2.50) | 88.0 / 12.0 | 32.3 vs 17.8 |

95 % Wilson intervals for the `wrong` share in the bottom 30 %: `faithful` ru
0.49–0.77, de 0.46–0.70, fr 0.38–0.66; `verdict` ru 0.54–0.81, de 0.57–0.80, fr
0.46–0.74; min(f, n) ru 0.56–0.83, de 0.56–0.79, fr 0.51–0.77. All three signals'
intervals straddle 60 % somewhere, so the 60 % bar is close to the noise floor at
these sample sizes, in both directions.

## 3. Separation (AUROC)

AUROC = P(a defect scores lower than an `ok` sentence).

| lang | signal | ok vs wrong | ok vs defect | ok vs minor |
|---|---|--:|--:|--:|
| ru | faithful | 0.741 | 0.705 | 0.699 |
| ru | verdict | 0.786 | 0.739 | 0.731 |
| ru | min(f, n) | 0.776 | 0.743 | 0.737 |
| ru | natural | 0.777 | 0.732 | 0.724 |
| de | faithful | 0.760 | 0.687 | 0.672 |
| de | verdict | 0.796 | 0.726 | 0.712 |
| de | min(f, n) | 0.796 | 0.723 | 0.708 |
| de | natural | 0.785 | 0.724 | 0.711 |
| fr | faithful | 0.710 | 0.696 | 0.693 |
| fr | verdict | 0.767 | 0.726 | 0.719 |
| fr | min(f, n) | 0.777 | 0.724 | 0.714 |
| fr | natural | 0.822 | 0.739 | 0.723 |

Two corrections to the probe's picture:

- The probe's Russian ok-vs-wrong AUROC of 0.88 came from 8 cases. On 42 cases it is
  **0.74**. On German and French the probe's 0.65 / 0.64 (6 and 3 cases) was too
  pessimistic: it is 0.76 / 0.71. The three languages now look alike, at about 0.70
  for defects and 0.71–0.76 for `wrong`.
- **`faithful` is the weakest of the four signals in every language.** `natural`
  alone beats it on `wrong` in all three languages (0.78 / 0.79 / 0.82). The reason is
  in §5.2: many of the book's `wrong` labels are grammatical errors (case, gender
  agreement, a missing determiner), which the meaning-only `faithful` question does
  not ask about.

## 4. Against lexcheck

| lang | lexcheck flagged | defects caught | wrong caught | Jev-bottom-15 % (f) defects / wrong | union size | union defects / wrong | added by lexcheck |
|---|--:|--:|--:|---|--:|---|--:|
| ru | 17 | 2 | 0 | 78 / 16 | 258 | 79 / 16 | 1 defect |
| de | 9 | 2 | 1 | 93 / 19 | 252 | 94 / 19 | 1 defect |
| fr | 27 (28 listed, one did not match a row) | 2 | 0 | 78 / 17 | 265 | 79 / 17 | 1 defect |

Lexcheck flags 9–27 sentences per language and 2 of them are defects: 11.8 / 22.2 /
7.4 per 100, about the base rate. In the union it adds exactly one defect per
language beyond Jev-bottom-15 %. This repeats the probe's finding (§1.5): the two
instruments look at different axes, and lexcheck was never a meaning check.

## 5. Examples (verbatim)

### 5.1 Ten lowest `faithful` per language

f = `faithful`, s = `verdict` score (0–2).

**ru**

| f | s | src | tr | checker | reason |
|--:|--:|---|---|---|---|
| 0.01 | 0.03 | ‘He has every requirement. | Не припомню, когда я видел что-либо столь прекрасное». | wrong | src «He has every requirement» is not rendered at all; translation «Не припомню, когда я видел что-либо столь прекрасное» belongs to the following sentence |
| 0.01 | 0.00 | I cannot recall when I have seen anything so fine.’ | Он сделал шаг назад, склонил голову набок и уставился на мои волосы, пока мне не стало совсем неловко. | wrong | src «I cannot recall when I have seen anything so fine» is untranslated; the Russian text instead renders the next sentence about stepping back and staring at hair |
| 0.01 | 0.00 | I cried. | воскликнул я. | ok | |
| 0.03 | 0.09 | She lives quietly, sings at concerts, drives out at five every day, and returns at seven sharp for dinner. | Она живёт тихо, поёт на концертах, выезжает каждый день в пять и возвращается ровно в семь к обеду. | ok | |
| 0.03 | 0.05 | I ejaculated after I had twice read over the extraordinary announcement. | воскликнул я, дважды перечитав это необычное объявление. | minor | «extraordinary» weakened to «необычное» (merely unusual), losing intensity |
| 0.04 | 0.13 | “‘And he is admirably suited for it,’ the other answered. | «И он превосходно для неё подходит, — ответил другой. — У него есть все необходимые качества. | wrong | translation appends «У него есть все необходимые качества», content from the next sentence, not present in this src line |
| 0.05 | 0.06 | “I fear not.” | «Боюсь, что нет». | ok | |
| 0.06 | 0.19 | I must compliment you.” | Должен вас поздравить. | minor | «поздравить» (congratulate) narrows «compliment» (praise/commend) to a specific speech act |
| 0.06 | 0.07 | “‘L’homme c’est rien—l’œuvre c’est tout,’ as Gustave Flaubert wrote to George Sand.” | «L’homme c’est rien—l’œuvre c’est tout», — как писала Жорж Санд Гюставу Флоберу. | wrong | reverses author/recipient: «как писала Жорж Санд Гюставу Флоберу» has Sand writing to Flaubert, but source says Flaubert wrote to Sand |
| 0.07 | 0.21 | “Indeed, I should have thought a little more. | «Право, я бы сказал, что немного больше. | minor | «я бы сказал, что» shifts nuance from «I should have thought» (a guessed estimate) to a flat statement |

**de**

| f | s | src | tr | checker | reason |
|--:|--:|---|---|---|---|
| 0.02 | 0.02 | We must be at Briony Lodge to meet her.” | Wir müssen um sieben bei Briony Lodge sein, um sie zu treffen.“ | minor | «um sieben» is added here; the English sentence itself gives no time (though consistent with the prior sentence) |
| 0.02 | 0.10 | About five ft. seven in. | Etwa fünf Fuß sieben Zoll groß; kräftig gebaut, fahler Teint, schwarzes Haar, in der Mitte etwas kahl, buschige schwarze Koteletten und Schnurrbart; getönte Brille, leichte Sprachbehinderung. | wrong | translation adds the entire content of the next sentence («kräftig gebaut, fahler Teint... Sprachbehinderung») not present in the source «About five ft. seven in.» |
| 0.04 | 0.09 | “I told you that I would call.” | „Ich sagte Ihnen, dass ich vorbeikommen würde.“ | ok | |
| 0.04 | 0.13 | Jump, Archie, jump, and I’ll swing for it!” | Spring, Archie, spring, und ich werde dafür büßen!“ | minor | «büßen» (atone/pay) loses the specific gallows sense of «swing for it» (be hanged) |
| 0.05 | 0.07 | “I will not look.” | „Ich werde nicht suchen.“ | ok | |
| 0.06 | 0.23 | Just hold out while I fix the derbies.” | Halten Sie nur durch, während ich die Handschellen anlege.“ | ok | |
| 0.08 | 0.32 | “I was afraid that you were engaged.” | „Ich fürchtete, dass Sie beschäftigt wären.“ | ok | |
| 0.09 | 0.45 | “There was nothing in the office but a couple of wooden chairs and a deal table, behind which sat a small man with a head that was even redder than mine. | "Im Büro gab es nichts als ein paar hölzerne Stühle und einen Tisch aus Tannenholz, hinter dem ein kleiner Mann saß, dessen Kopf sogar röter war als meiner." | ok | |
| 0.10 | 0.25 | Then Mr. Angel began to call, for it was obvious that the matter should be pushed as far as it would go if a real effect were to be produced. | Dann begann Mr. Angel zu Besuch zu kommen, denn es war offensichtlich, dass die Sache so weit vorangetrieben werden sollte, wie es nur gehen würde, wenn eine echte Wirkung erzielt werden sollte. | ok | |
| 0.12 | 0.37 | I had brought a pack of cards in my pocket, and I thought that, as we were a partie carrée, you might have your rubber after all. | Ich hatte ein Kartenspiel in der Tasche mitgebracht, und ich dachte, da wir eine partie carrée wären, könntest du am Ende doch deinen Whist haben. | minor | «deinen Whist» loses the specific sense of «rubber» as a match/set of games, not just the game itself |

**fr**

| f | s | src | tr | checker | reason |
|--:|--:|---|---|---|---|
| 0.02 | 0.05 | Jump, Archie, jump, and I’ll swing for it!” | Saute, Archie, saute, et je paierai pour ça ! | minor | «swing for it» (be hanged for it) is flattened to «je paierai pour ça», losing the gallows-specific meaning |
| 0.04 | 0.08 | I ejaculated after I had twice read over the extraordinary announcement. | m'écriai-je après avoir relu deux fois l'extraordinaire annonce. | ok | |
| 0.05 | 0.07 | “I fear not.” | « Je crains que non. » | ok | |
| 0.06 | 0.22 | As I passed the well-remembered door, which must always be associated in my mind with my wooing, and with the dark incidents of the Study in Scarlet, I was seized with a keen desire to see Holmes again, and to know how he was employing his extraordinary powers. | En passant devant la porte bien connue, qui doit toujours être associée dans mon esprit à mon mariage, et aux sombres incidents de l'Étude en rouge, je fus saisi d'un vif désir de revoir Holmes, et de savoir comment il employait ses extraordinaires pouvoirs. | minor | «mon mariage» (marriage) narrows 'my wooing' (courtship), shifting the nuance |
| 0.08 | 0.24 | “John Clay, the murderer, thief, smasher, and forger. | — John Clay, le meurtrier, le voleur, l'escroc et le faussaire. | minor | «escroc» is a generic swindler, losing the specific sense of «smasher» (a passer of counterfeit money) |
| 0.09 | 0.26 | “I was afraid that you were engaged.” | « Je craignais que vous ne soyez occupé. » | ok | |
| 0.09 | 0.38 | A few seconds sufficed to satisfy him, for he sprang to his feet again and put his glass in his pocket. | Quelques secondes suffirent pour le satisfaire, car il se releva d'un bond et remit sa loupe dans sa poche. | ok | |
| 0.10 | 0.33 | Just hold out while I fix the derbies.” | Tenez bon pendant que je fixe les menottes. » | ok | |
| 0.11 | 0.31 | “I will not look.” | « Je ne chercherai pas. » | ok | |
| 0.11 | 0.31 | “I am in hopes that she does.” | J'espère qu'elle l'aime. | ok | |

What the extremes show:

- **Sentence-boundary shifts are Jev's clearest catch.** The three lowest Russian
  sentences and the second-lowest German one are translations that slid one sentence
  forward or absorbed the next one. Jev gives them 0.01–0.04, where the probe's
  «Ещё один вопрос.» case (0.01) was the only example. No other gate in the pipeline
  sees these, because each translation is fluent, in the right language, and uses
  glossary terms.
- **The literal-reader false positives are the same as in the probe**, and they recur
  across languages because they sit on the source: *I would call* → «vorbeikommen» /
  visit, *I will not look* (the checker now calls «Ich werde nicht suchen» / «Je ne
  chercherai pas» `ok`, where the GOLD-lite pass called the German `wrong`: silver
  labels are not stable on this sentence either), *derbies* = handcuffs, *engaged* =
  busy, *I ejaculated* = exclaimed. Jev marks archaic or idiomatic English as risky
  whether or not the translation solved it.
- «I cried. → воскликнул я.» at 0.01 is a split dialogue tag, correctly translated.
  Jev penalises the change of person and verb that the split forces.

### 5.2 `wrong` sentences Jev ranked in the top half (misses)

"rank" = position in the book's `faithful` ordering (0 = lowest, 1 = highest).

| lang | f | s | rank | src | tr | checker reason |
|---|--:|--:|--:|---|---|---|
| ru | 0.93 | 1.85 | 0.909 | “You may address me as the Count Von Kramm, a Bohemian nobleman. | «Вы можете обращаться ко мне как граф фон Крамм, богемский дворянин. | case government error: «обращаться ко мне как граф фон Крамм» should be dative «как к графу фон Крамму» to agree with «ко мне» |
| ru | 0.92 | 1.82 | 0.893 | You just read it for yourself, sir.” | Вы только что сами это прочитали, сэр. | «Вы только что сами это прочитали» renders past tense, but source is an imperative invitation to read it now, not a statement about a completed action |
| ru | 0.90 | 1.74 | 0.834 | “Your Majesty must pay. | «Ваше Величество должно заплатить.» | «Ваше Величество должно» uses neuter grammatical agreement instead of the honorific masculine/plural agreement expected when addressing the King directly |
| ru | 0.89 | 1.74 | 0.818 | We know something of Saxe-Coburg Square. | Мы кое-что знаем о Сакс-Кобург-сквер. | «о Сакс-Кобург-сквер» lacks the prepositional case ending, should be «о Сакс-Кобург-сквере» |
| ru | 0.87 | 1.72 | 0.728 | I sent John, the coachman, to watch you, ran upstairs, got into my walking clothes, as I call them, and came down just as you departed. | Я послал Джона, кучера, наблюдать за вами, поднялся наверх, переоделся в свою прогулочную одежду, как я её называю, и спустился вниз как раз, когда вы уходили. | narrator is Irene Adler but verbs «послал», «поднялся», «переоделся», «спустился» use masculine past-tense forms |
| de | 0.95 | 1.83 | 0.932 | “No, my brougham is waiting.” | „Nein, meine Brougham wartet.“ | «meine Brougham» is wrong gender agreement — Brougham is masculine in German («der Brougham»), so it should be «mein Brougham» |
| de | 0.91 | 1.67 | 0.800 | If she does not love your Majesty, there is no reason why she should interfere with your Majesty’s plan.” | Wenn sie Eure Majestät nicht liebt, gibt es keinen Grund, warum sie sich in Eure Majestät Plan einmischen sollte. | wrong case: «Eure Majestät Plan» should be genitive «Eurer Majestät Plan» |
| de | 0.86 | 1.50 | 0.619 | Two thousand five hundred pounds was the amount, but I can only touch the interest.” | „Zweitausendfünfhundert Pfund war der Betrag, aber ich kann nur die Zinsen anfassen.“ | «anfassen» translates «touch» literally as physical touching, garbling the idiom for accessing/drawing the interest |
| de | 0.86 | 1.64 | 0.591 | ‘The Church of St. Monica,’ said I, ‘and half a sovereign if you reach it in twenty minutes.’ | „Kirche St. Monica“, sagte ich, „und ein halber Souverän, wenn Sie sie in zwanzig Minuten erreichen.“ | «ein halber Souverän» misuses «Souverän» (ruler/sovereign entity) for the coin «sovereign», producing a nonsensical reading |
| de | 0.85 | 1.51 | 0.558 | That trick of staining the fishes’ scales of a delicate pink is quite peculiar to China. | Dieser Trick, die Schuppen der Fische mit einem zarten Rosa zu färben, ist ganz eigenartig für China. | «eigenartig für China» means 'odd/strange for China', not 'peculiar to/characteristic of China' as in the source |
| fr | 0.94 | 1.87 | 0.920 | “None.” | « Aucun. » | «Aucun» is masculine but the antecedent «idée» (from the previous line) is feminine, so it should be «Aucune» |
| fr | 0.93 | 1.65 | 0.850 | “Pshaw! | « Pshaw ! | «Pshaw !» left untranslated in English instead of a French interjection |
| fr | 0.91 | 1.75 | 0.748 | “Such paper could not be bought under half a crown a packet. | « Un tel papier ne pouvait pas s'acheter à moins d'un demi-couronne le paquet. | gender agreement broken: «un demi-couronne» — couronne is feminine, should be «une demi-couronne» |
| fr | 0.89 | 1.76 | 0.707 | “Oh, yes, mother is alive and well. | « Oh, oui, mère est vivante et bien portante. | «mère» used as bare noun without determiner is ungrammatical French, should be «ma mère» |
| fr | 0.87 | 1.79 | 0.619 | I tried to puzzle it out, but gave it up in despair and set the matter aside until night should bring an explanation. | J'essayai de le démêler, mais j'y renonçai par dépit et mis la question de côté jusqu'à ce que la nuit apportât une explication. | «par dépit» means 'out of spite/pique', not 'in despair' (désespoir), a false-friend mistranslation of feeling |

The misses have a clear structure:

- **Morphology** (case, gender agreement, a missing determiner): 7 of the 15
  (ru 3, de 2, fr 2). This is grammar, not meaning. The `faithful` question as worded
  ("conveys exactly the meaning") does not ask about it, and Jev answers the question
  it was given. `natural` does ask, which is why min(f, n) recovers some of these.
- **Context outside the sentence**: 2 of the 15 (the narrator's gender, the
  antecedent «idée» in the previous line). They are invisible to any single-sentence
  scorer, the checker's labels themselves included. The checker only knew because the
  batch holds neighbouring sentences.
- **Lexical false friends and idioms rendered literally**: 4 of the 15 («anfassen»,
  «Souverän», «eigenartig für», «par dépit»). This is the one class `faithful` should
  catch and does not. Jev does not know that «par dépit» ≠ *in despair*.
- One ambiguous-source reading (*You just read it for yourself*: the checker reads an
  imperative, the translation reads a past tense) and one untranslated interjection
  («Pshaw !»). The checker counts both as `wrong`, and both are arguable.

## 6. Verdict

Kill criterion, applied exactly: pass only if the bottom-15 % slice holds ≥ 1.5× the
defects per judged sentence of a random slice in **all three** languages **and**
≥ 60 % of `wrong` sentences fall in the bottom 30 % **in each** language. The ranking
signal is Jev `faithful`, as specified in the probe's recommendation.

| lang | 15 % defects/100: Jev vs random | ratio | ≥ 1.5× | `wrong` in bottom 30 % | ≥ 60 % |
|---|---|--:|:-:|--:|:-:|
| ru | 31.7 vs 19.0 | 1.67 | yes | 27 / 42 = 64.3 % | yes |
| de | 37.8 vs 21.2 | 1.78 | yes | 35 / 60 = **58.3 %** | **no** |
| fr | 31.7 vs 18.0 | 1.76 | yes | 24 / 46 = **52.2 %** | **no** |

**Result: FAIL.** Per the "only if it passes" condition, this report contains no
design for `--judge-scope jev`, and `--judge-scope flagged` keeps its random 5 %.

What the data does support, stated as findings rather than a pass:

1. **Any Jev ranking is a real ~1.7–2× defect concentrator on a whole book, in every
   language.** A judge that sees the Jev-bottom 15 % instead of a random 15 % finds
   32–38 defects per 100 judged sentences instead of 18–21. The router half of the
   idea is sound.
2. **The signal was the wrong choice, not the idea.** Ranked by the `verdict` score or
   by min(`faithful`, `natural`), both criteria clear in all three languages:
   15 % ratio ×2.09 / ×1.95 / ×2.04 (verdict) and ×1.79 / ×1.88 / ×1.92 (min),
   `wrong` in the bottom 30 % 69.0 / 70.0 / 60.9 % (verdict) and 71.4 / 68.3 / 65.2 %
   (min). This is **post hoc**: two alternatives were picked after seeing results,
   French `verdict` passes by one sentence (28 / 46), and the Wilson intervals (§2.3)
   include 60 %. Treat it as a hypothesis.
3. **The next test, if one is wanted**, is a pre-registered repeat on a book that is
   not Holmes (a different register, so the Holmes-specific literal-reader traps do
   not dominate). Fix min(`faithful`, `natural`) as the signal, since it needs no
   score question and is the stronger of the two on `wrong` in de/fr. Keep the same
   kill criterion. Cost: about $0.03 of Jev per 1,000 sentences per language, plus
   one Sonnet checker pass (165 `claude -p` batches per 1,642 sentences per
   language).

## 7. Caveats

- **Labels are silver.** The Sonnet checker produced every ok / minor / wrong label,
  on its own batches of ten. It is not the converter's `j3` judge and not a human.
  The single sentence «Ich werde nicht suchen.» is `wrong` in GOLD-lite and `ok`
  here. Label noise pushes AUROCs and enrichments towards 1.0, so the real separation
  may be somewhat higher, but the kill decision was made against these labels.
- One book, one arm (DeepSeek + repair), one question wording per signal, no tuning.
  The Holmes source is heavy in Victorian idiom (*call*, *engaged*, *derbies*, *swing
  for it*), which feeds Jev's literal-reader false positives. A modern text might
  score better or worse.
- The experiment measures what a router *would hand* the judge, using the checker as
  a stand-in for the judge. It did not run the converter's judge on the Jev slice.
  That was deliberate, because the kill test failed first.
- Duplicate source sentences (chapter titles, repeated one-liners) appear once per
  occurrence, and lexcheck matching is by source text, so a flagged duplicated
  sentence counts at every occurrence.
- Artefacts (verbatim book text, gitignored): `bench-quality/.artifacts/jev/scope/`:
  `sent-*.jsonl`, `jev-*.jsonl`, `check-*/`, `analyse.py`, `analysis.json`,
  `tables.md`.
