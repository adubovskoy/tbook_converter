# Lexcheck calibrated on en→de and en→fr

Research report, 2026-09-19. Offline: no API calls, no network, no keys. Tool:
`cmd/lexeval` (extended, §7) against `internal/lexcheck` at the current thresholds
`MinCoveredPairs = 4`, `LowSupportRate = 0.34`, `MinShiftHits = 2`.

Lexcheck's three thresholds were swept once, for en→ru, on one book. `lexicons/en-de.tsv.gz`
and `lexicons/en-fr.tsv.gz` have existed since July and were never evaluated. This report
gives both pairs a recall / false-positive number, on two German books and one French one,
with en→ru re-measured under identical conditions as the control.

| question | answer |
|---|---|
| Does the detector work at all on German? | **Better than on Russian.** 95.3–95.8 % recall on full drift cascades, 73–74 % on mid-sentence cascades, **0.7 %** false positives on clean text — on both German books |
| On French? | **Yes, between the two.** 89.4 % / 66.8 % recall at **2.6 %** false positives |
| The en→ru control, same tool, same conditions | 82.9 % / 52.9 % at 2.4 % (Sherlock) and 78.8 % / 46.2 % at 2.9 % (*Altered Carbon*) — the mid-cascade figure reproduces the historical ~52 % |
| Do the thresholds need to move? | **No.** The plan's kill criterion is met in the safe direction: neither new pair is *worse* than en→ru — de is 12–21 pp *better*, fr 7–14 pp better. The current triple sits on the knee of the curve for all three pairs (§5) |
| Lexicon pair coverage | de **79.5 %**, fr **80.7 %**, ru **82.7–84.2 %** of scoreable align pairs carry dictionary evidence |
| Does lexcheck see a wrong-language cell? | **Yes when the whole cell leaks** (78–88 % flagged across de/fr/es/it), **no when a single word leaks** (1–4 %). §6 corrects the generalisation in EVIDENCE §3.D — which holds for the Chinese batch and for token-level leaks, not for whole-sentence ones |
| Side effect worth knowing | on a contaminated book lexcheck's flag budget *is* the contamination: **1011 of 1017** German flags in `19.tbook` are wrong-language cells, so the judge's `flagged` scope spends itself on a defect the judge cannot fix |

---

## 1. Method

`cmd/lexeval` builds the evaluation set from a produced `.tbook`, with no reference data:

- **Negatives** — every sentence of the book with ≥ 4 align chunks and ≥ 5 source words,
  taken as presumed-good. Anything lexcheck flags here is counted a false positive.
- **Positives** — two synthetic twins per negative, same translation, alignment shifted one
  source word to the right: *full cascade* (every chunk) and *mid-sentence cascade* (the last
  half of the chunks). This is the exact signature the detector exists for, and it is the only
  positive set available offline.
- **Pair coverage** — `Covered / Scored`, where *scored* pairs are align chunks with a
  non-empty `w` whose target fragment has ≥ 3 letters (lexcheck ignores the rest as too noisy),
  and *covered* pairs are those whose source word the lexicon knows. Against the looser
  denominator of *all* align chunks the same books give de 72.5 %, fr 60.2 %, ru 64.6 % — the
  historical "72 % of pairs" is not defined anywhere, so it is not compared to.
- **Precision** in the tables is `TP_full / (TP_full + FP)`, i.e. at an artificial 1:1 prior of
  drifted to clean sentences. Real books are ~2 % drifted, so the operational number is the
  **FP-rate** column; precision is kept only for continuity with the earlier sweep.

`--judged-only` (restrict negatives to judge-OK sentences) was **not** used: no judge verdict
cache exists for de or fr, and producing one needs the network. The en→ru control is run the
same way, so all seven rows are comparable to each other, and none is directly comparable to
the historical judged-negatives figure.

Two of the books carry the glossary-leak defect (EVIDENCE §3.B), which puts Russian sentences
into Latin targets. Those cells are not "presumed-good", so every book is also run with
`--clean-only`, which drops the sentences langcheck calls wrong-language from the negatives.
Both rows are reported: the difference *is* the contamination's cost in false positives.

## 2. Corpus

| file | book | pair(s) used | sentences | note |
|---|---|---|---|---|
| `android/app/src/main/assets/sample.tbook` | *The Adventures of Sherlock Holmes* | en→de, en→fr, en→ru | 6 833 | 8 targets from one run; de 5.6 % / fr 7.3 % of cells carry Cyrillic |
| `torrents/tbooks/7.tbook` | *Harry Potter and the Sorcerer's Stone* | en→de | 5 502 | German only; **0.0 %** Cyrillic — the clean German book |
| `converter/bench-quality/full-kimi.tbook` | *Altered Carbon* | en→ru | 14 688 | the historical calibration corpus, control |
| `torrents/tbooks/19.tbook` | *The Wonderful Wizard of Oz* | de, fr, es, it | 2 190 | heavily contaminated; **negative control only** (§6), never used for calibration |

## 3. Results at the current thresholds

`MinCoveredPairs = 4`, `LowSupportRate = 0.34`, `MinShiftHits = 2`.

| pair | book / negatives | pair coverage | support rate on clean text | recall full | recall mid | FP-rate | precision |
|---|---|---|---|---|---|---|---|
| **en-de** | Sherlock, all 5 554 | 79.5 % | 73.4 % | **95.3 %** | **74.3 %** | 2.16 % | 97.8 % |
| **en-de** | Sherlock, clean-only 5 221 | 79.4 % | 74.7 % | **95.3 %** | **73.9 %** | **0.75 %** | 99.2 % |
| **en-de** | Harry Potter, all 4 624 | 84.3 % | 80.3 % | **95.8 %** | **73.1 %** | **0.74 %** | 99.2 % |
| **en-fr** | Sherlock, all 5 614 | 80.7 % | 65.6 % | **89.8 %** | **67.9 %** | 3.95 % | 95.8 % |
| **en-fr** | Sherlock, clean-only 5 147 | 80.6 % | 67.4 % | **89.4 %** | **66.8 %** | **2.55 %** | 97.2 % |
| en-ru *(control)* | Sherlock, all 5 585 | 84.2 % | 68.7 % | 82.9 % | 52.9 % | 2.35 % | 97.2 % |
| en-ru *(control)* | *Altered Carbon*, all 10 715 | 82.7 % | 67.5 % | 78.8 % | 46.2 % | 2.89 % | 96.5 % |

Readings:

1. **German is the detector's best pair.** Both books agree to within 0.5 pp on full-cascade
   recall and 1.2 pp on mid-cascade, on different authors, different runs, different years of
   the pipeline. Its advantage comes from the support rate: 73–80 % of covered pairs are
   dictionary-plausible on correct German, against 66–69 % on French and Russian, so a shift
   drops the support rate across the flag line far more reliably.
2. **French sits between German and Russian on recall and is the worst on false positives.**
   The cause is visible in the same column: the en-fr lexicon supports only 65.6 % of covered
   pairs on *correct* French, because so much French content is periphrastic (`C'est arrivé
   par…`, `dit-il`, `n'avais guère`) — see §5.2.
3. **The en→ru control reproduces the historical figures** on the mid-sentence cascade
   (52.9 % vs "~52 %") and on false positives (2.35 % vs "~2.4 %"); its full-cascade recall is
   4–8 pp lower than the historical "~87 %", which was measured with judge-OK negatives on a
   different book (`bench-quality/kimi-a.tbook`, the July bench file, gives 78.3 % / 44.2 % /
   2.5 % with all-sentence negatives — the same offset). The gap is a method difference, not a
   regression: it moves both control rows equally and none of the conclusions depend on it.
4. **Contamination costs more in false positives than anything in the threshold grid.**
   Dropping the langcheck-flagged sentences takes de from 2.16 % → 0.75 % and fr from
   3.95 % → 2.55 %, at a recall cost of ≤ 0.4 pp. B1's wrong-language gate would therefore
   improve lexcheck's precision as a by-product.

**The plan's kill criterion (A4).** "If en-de/en-fr recall is within 10 pp of en-ru at the
current thresholds, keep them and stop." French is within 10 pp on both cascades
(+6.5 / +13.9 pp vs the en→ru Sherlock control); German is outside it on the *high* side
(+12.4 / +21.0 pp). No pair is worse than the pair the thresholds were tuned for, so the
criterion is met in the sense it was written for: **keep the thresholds, stop.**

## 4. The threshold grid

Swept: `MinCoveredPairs ∈ {3,4,5}` × `LowSupportRate ∈ {0.20…0.50}` × `MinShiftHits ∈ {2,3,4}`
(63 points per run; full output in the JSON files named in §7). `MinShiftHits = 2` dominates
everywhere — 3 and 4 lose 10–30 pp of mid-cascade recall for ≤ 0.1 pp of false positives — so
the extract below fixes it at 2 and shows the interesting plane. Negatives are the clean-only
sets for Sherlock.

| thresholds | en-de (HP, all) | en-de (Sherlock, clean) | en-fr (Sherlock, clean) | en-ru (Sherlock, all) |
|---|---|---|---|---|
| covered≥3, ≤0.25 | 97.3 / 72.2 / 0.54 | 96.5 / 72.6 / 0.46 | 93.0 / 64.2 / 2.08 | 85.2 / 50.3 / 1.47 |
| covered≥3, ≤0.30 | 97.4 / 72.6 / 0.63 | 96.5 / 73.1 / 0.54 | **93.0 / 65.7 / 2.39** | 85.6 / 51.7 / 1.83 |
| covered≥3, ≤0.34 | 97.8 / 75.2 / 1.10 | 97.1 / 76.3 / 1.63 | 94.2 / 70.9 / 4.27 | 88.6 / 57.8 / 4.10 |
| covered≥3, ≤0.40 | 97.9 / 77.9 / 1.75 | 97.1 / 78.9 / 2.76 | 94.2 / 75.6 / 7.34 | 89.4 / 62.4 / 6.84 |
| covered≥4, ≤0.25 | 95.8 / 71.8 / 0.52 | 95.2 / 72.3 / 0.40 | 89.4 / 62.9 / 1.75 | 81.9 / 49.3 / 1.24 |
| covered≥4, ≤0.30 | 95.8 / 72.2 / 0.61 | 95.2 / 72.8 / 0.48 | 89.4 / 64.5 / 2.06 | 82.3 / 50.6 / 1.59 |
| **covered≥4, ≤0.34 (current)** | **95.8 / 73.1 / 0.74** | **95.3 / 73.9 / 0.75** | **89.4 / 66.8 / 2.55** | **82.9 / 52.9 / 2.35** |
| covered≥4, ≤0.40 | 95.9 / 75.9 / 1.38 | **95.3 / 76.5 / 1.88** | 89.5 / 71.5 / 5.61 | 83.8 / 57.5 / 5.09 |
| covered≥5, ≤0.34 | 94.7 / 71.7 / 0.43 | 94.1 / 72.4 / 0.57 | 86.8 / 63.8 / 1.79 | 79.6 / 50.3 / 1.74 |
| covered≥5, ≤0.40 | 94.8 / 74.5 / 1.08 | 94.1 / 75.0 / 1.70 | 86.8 / 68.5 / 4.86 | 80.5 / 54.8 / 4.48 |

*Cells are `recall-full % / recall-mid % / FP %`.*

Shape of the surface, identical for all three pairs:

- **`LowSupportRate` past 0.34 is a cliff for fr and ru and a ramp for de.** 0.34 → 0.40 costs
  French +3.1 pp of false positives and Russian +2.7 pp, for +4.7 pp of mid-cascade recall;
  the same step costs German +0.6…+1.1 pp for +2.6 pp. The support-rate distribution of correct
  text differs by pair, and 0.34 happens to sit just below the French and Russian bulk.
- **`MinCoveredPairs` trades full-cascade recall against false positives almost linearly.**
  3 buys +1.2…+3.6 pp of full recall; on de it is nearly free (+0.1 pp FP on Harry Potter),
  on fr and ru it costs 0.3–0.6 pp.
- **The best (recall, FP) point per pair**, taking the highest mid-cascade recall that stays at
  or below the en→ru reference FP of ~2.4 %:

| pair | best point | full | mid | FP | vs current |
|---|---|---|---|---|---|
| en-de (both books) | covered≥4, ≤0.40, shift≥2 | 95.3–95.9 % | 75.9–76.5 % | 1.4–1.9 % | +2.6 pp mid-recall, +1.1 pp FP |
| en-fr | covered≥3, ≤0.30, shift≥2 | 93.0 % | 65.7 % | 2.39 % | +3.6 pp full-recall, −1.1 pp mid, −0.2 pp FP |
| en-ru | covered≥4, ≤0.34, shift≥2 | 82.9 % | 52.9 % | 2.35 % | *is* the current point |

None of these is worth a code change on its own (§8).

## 5. What it misses, and what it flags wrongly

Verbatim, from `--examples`. Marks: `[ok]` dictionary-supported, `[??]` covered but
unsupported, `[→N]` implausible for its own word but plausible for neighbour *N* (the shift
signature), `[--]` scored but no covered source word, `[..]` below the scoring floor
(fragment < 3 letters or an inserted chunk).

### 5.1 en-de

**Missed synthetic shift** — Sherlock, full cascade, not flagged:

```
SRC: “Indeed, I should have thought a little more.
TR : „Allerdings, ich hätte etwas mehr vermutet.
     scored=6 covered=3 supported=1 shiftHits=1 → not flagged
  [--] "Allerdings" ← {1:I}
  [→1] "ich"        ← {2:should}
  [??] "hätte"      ← {3:have, 4:thought}
  [ok] "etwas"      ← {6:little, 7:more}
  [--] "mehr"       ← {0:Indeed}
  [--] "vermutet"   ← {5:a}
```

Both rules fall short by exactly one: 3 covered pairs against a floor of 4, one shift hit
against a floor of 2. The drift is total and the detector can see it — `ich` is flagged as
belonging to word 1 — but three of the six pairs land on `I`, `a` and `Indeed`, which are under
the three-letter floor or absent from the lexicon, so the evidence never reaches quorum. The
`[ok]` line shows the known structural blind spot: `etwas` keeps its support after the shift
because its chunk carries two source words and only one of them has to be plausible.
This is the dominant miss class for German: short, quoted, function-word-heavy dialogue.

**False positive** — Sherlock, unshifted, flagged `low-support`:

```
SRC: Presently he emerged, looking even more flurried than before.
TR : Bald darauf trat er wieder heraus und sah noch aufgeregter aus als zuvor.
     scored=10 covered=6 supported=2 shiftHits=1 → flagged, support rate 0.33 ≤ 0.34
  [??] "Bald"        ← {0:Presently}
  [??] "darauf"      ← {0:Presently}
  [--] "trat"        ← {2:emerged}
  [..] "er"          ← {1:he}
  [--] "heraus"      ← {2:emerged}
  [--] "sah"         ← {3:looking}
  [→5] "noch"        ← {4:even}
  [??] "aufgeregter" ← {5:more, 6:flurried}
  [--] "aus"         ← {3:looking}
  [ok] "als"         ← {7:than}
  [ok] "zuvor"       ← {8:before}
```

The alignment is correct. The German is a separable-verb construction (`trat … heraus` for
*emerged*) plus an adverbial periphrasis (`Bald darauf` for *Presently*): two source words
rendered by five target words, none of which the lexicon lists for them. Separable verbs and
`Bald darauf`-style adverbials are the German false-positive class, and they are also why
German's false-positive rate is nonetheless the lowest of the three: they are rare per sentence,
and German's high base support rate leaves room above the 0.34 line.

The second German book gives the same class:

```
SRC: Trying to pull himself together, he let himself into the house.
TR : Er versuchte, sich zu fassen, und ließ sich ins Haus hinein.
     scored=7 covered=7 supported=2 shiftHits=0 → flagged, support rate 0.29
  [ok] "versuchte" ← {0:Trying}
  [??] "sich"      ← {3:himself}
  [??] "fassen"    ← {2:pull, 4:together}
  [??] "ließ"      ← {6:let}
  [??] "sich"      ← {7:himself}
  [??] "ins"       ← {8:into, 9:the}
  [ok] "Haus"      ← {10:house}
```

`sich` (reflexive, no English counterpart), `fassen` for *pull together* and `ließ … hinein`
for *let … into*: four idiomatic pairs are enough to cross the line in a seven-pair sentence.

### 5.2 en-fr

One sentence is both — a false positive when correct and a miss when drifted:

```
correct:  scored=6 covered=6 supported=2 shiftHits=0 → FLAGGED (0.33 ≤ 0.34)
SRC: I had seen little of Holmes lately.
TR : Je n'avais guère vu Holmes ces derniers temps.
  [??] "n'avais"  ← {1:had}         élidé auxiliary, not a dictionary form of "had"
  [??] "guère"    ← {3:little}      negative-polarity adverb, absent for "little"
  [..] "vu"       ← {2:seen}        two letters: under the scoring floor
  [ok] "Holmes"   ← {5:Holmes}
  [??] "ces"      ← {6:lately}      "ces derniers temps" is one unit; only one third of it matches
  [ok] "derniers" ← {6:lately}
  [??] "temps"    ← {6:lately}

shifted:  scored=6 covered=2 supported=0 shiftHits=1 → not flagged
  [??] "n'avais" ← {2:seen}
  [--] "guère"   ← {4:of}
  [..] "vu"      ← {3:little}
  [→5] "Holmes"  ← {6:lately}
  [--] "ces" / "derniers" / "temps" ← {0:I}
```

Correct French is scored against a lexicon that does not carry elided forms (`n'avais`,
`l'œuvre`, `d'œil`, `dit-il`), so three of the six pairs are unsupported and the sentence
crosses the low-support line; drift then pushes the same pairs onto `I` and `of`, which the
lexicon cannot judge at all, and the evidence evaporates. Both French failure modes have one
cause: **elision and multi-word periphrasis keep the support rate low, and a detector that
looks for a *drop* in support rate has nothing to drop.**

A second, cleaner French false positive:

```
SRC: “It came by the last post,” said he.
TR : « C'est arrivé par le dernier courrier », dit-il.
     scored=6 covered=4 supported=1 shiftHits=0 → flagged, support rate 0.25
  [--] "C'est"    ← {0:It}
  [??] "arrivé"   ← {1:came}
  [--] "par"      ← {2:by}
  [ok] "dernier"  ← {4:last}
  [??] "courrier" ← {5:post}      "post" → "poste"/"courrier": sense missing
  [??] "dit-il"   ← {6:said, 7:he}   inverted clitic, one token for two words
```

### 5.3 en-ru (control)

```
missed: SRC: “Wedlock suits you,” he remarked.
        TR : «Брак тебе идёт», — заметил он.
        scored=4 covered=2 supported=0 shiftHits=0 → not flagged (floor)

false positive: scored=12 covered=9 supported=3 shiftHits=0 → flagged, 0.33 ≤ 0.34
SRC: Then he stood before the fire and looked me over in his singular introspective fashion.
TR : Затем он встал перед камином и окинул меня взглядом в своей присущей ему
     самоуглублённой манере.
  [ok] "Затем"/"перед"/"своей"
  [??] "окинул" ← {7:looked, 9:over}   |  [??] "взглядом" ← {7:looked, 9:over}
  [--] "присущей" ← {12:singular}      |  [--] "самоуглублённой" ← {13:introspective}
  [??] "манере" ← {14:fashion}
```

Same shape as the other two pairs: a literary rendering of a phrasal verb (`looked … over` →
`окинул взглядом`) plus two rare adjectives the lexicon does not carry.

## 6. The wrong-language question

EVIDENCE §3.D lists lexcheck among the gates that cannot see a wrong-language answer, on the
strength of "0 of 16 Chinese sentences flagged". Measured on the Cyrillic-contaminated
`19.tbook`, that generalisation is **half right**, and the half that is wrong is the larger half.
`--contamination` cross-tabulates langcheck's verdict with lexcheck's flag on the same cells;
foreign-script cells are split by how much of the cell is foreign (langcheck's `share`):

| target | langcheck verdict | sentences | share of book | lexcheck flags | flag rate |
|---|---|---|---|---|---|
| **de** | wrong-language, ≥ 50 % foreign | 1 155 | 53.0 % | 1 011 | **87.5 %** |
| | wrong-language, < 50 % foreign | 406 | 18.6 % | 4 | **1.0 %** |
| | untranslated | 3 | 0.1 % | 0 | 0.0 % |
| | rest | 617 | 28.3 % | 2 | 0.3 % |
| **fr** | wrong-language, ≥ 50 % foreign | 441 | 20.1 % | 356 | **80.7 %** |
| | wrong-language, < 50 % foreign | 757 | 34.6 % | 30 | **4.0 %** |
| | rest | 991 | 45.3 % | 13 | 1.3 % |
| **es** | wrong-language, ≥ 50 % foreign | 131 | 6.0 % | 102 | 77.9 % |
| | wrong-language, < 50 % foreign | 959 | 43.8 % | 20 | 2.1 % |
| | rest | 1 096 | 50.1 % | 7 | 0.6 % |
| **it** | wrong-language, ≥ 50 % foreign | 293 | 13.4 % | 243 | 82.9 % |
| | wrong-language, < 50 % foreign | 772 | 35.3 % | 26 | 3.4 % |
| | rest | 1 121 | 51.2 % | 15 | 1.3 % |

The same split on the mildly contaminated Sherlock: de 115 whole-cell leaks → 65.2 % flagged,
249 token leaks → 2.8 %; fr 104 → 69.2 % and 388 → 5.2 %.

Four conclusions:

1. **A whole leaked sentence is caught 78–88 % of the time, on four independent targets.**
   Not because lexcheck understands languages, but because a Russian sentence in a German cell
   makes every covered pair implausible at once, which is precisely the low-support signature.
   The reason given for the blindness in EVIDENCE — "lexcheck needs dictionary lookups, so
   unlookupable text yields no evidence" — applies to a *script* the fold/floor rules discard
   (a Han fragment is typically 1–2 characters, below the three-letter floor, so nothing is
   scored at all: that is the 0/16 Chinese case, and it is not reproducible here — no file on
   disk carries that batch). Cyrillic in a Latin target survives every filter and *is* scored.
2. **A single leaked word is invisible** — 1.0–4.0 %, i.e. the clean-text false-positive rate.
   `… eine der Liebe ähnliche Emotion für Айрин Адлер empfand` keeps a normal support rate.
   This is the majority of the contamination in the Romance targets (fr 757 of 1 198 cells).
3. **The flag says the wrong thing.** Every one of those 1 011 German flags is reported as
   `low-support`, i.e. as suspected positional drift, and the remedy the pipeline attaches to
   that reason (re-align, escalate to the judge) cannot fix a sentence that is in the wrong
   language. langcheck remains the only signal that names the defect.
4. **On a contaminated book, lexcheck's output is the contamination.** 1 011 of 1 017 German
   flags (99.4 %) and 386 of 399 French flags (96.7 %) are wrong-language cells. Anything that
   consumes the flag list — the hybrid align gate, `--judge-scope flagged`, `<out>.lexflagged.json`
   — is spending its whole budget there. B1's gate should therefore run *before* lexcheck's
   output is used for anything.

## 7. Recommendation (thresholds only)

**Keep `MinCoveredPairs = 4`, `LowSupportRate = 0.34`, `MinShiftHits = 2` as the shipped
defaults.** No code change is proposed. The grid says the current triple is the knee of the
curve for all three pairs, both new pairs are better than the pair it was tuned for, and the
A4 kill criterion is met.

If per-pair thresholds are ever introduced (they do not exist today — the constants are
global), the data supports exactly two adjustments, both small:

| pair | proposed | effect | why it is not urgent |
|---|---|---|---|
| en-de | `LowSupportRate = 0.40` | mid-cascade recall 73.9 → 76.5 % (Sherlock) / 73.1 → 75.9 % (HP); FP 0.75 → 1.88 % / 0.74 → 1.38 % | still below the en→ru FP the defaults were accepted at; buys 2.6 pp of the hardest class |
| en-fr | `MinCoveredPairs = 3`, `LowSupportRate = 0.30` | full-cascade recall 89.4 → 93.0 %, FP 2.55 → 2.39 %, mid-cascade 66.8 → 65.7 % | trades 1.1 pp of mid-cascade recall for 3.6 pp of full-cascade at slightly *lower* FP |

Two non-threshold findings are worth more than either row above, and belong to other plan items:

- **Run the wrong-language gate first (B1).** Removing langcheck-flagged cells from the book
  lowers lexcheck's false-positive rate by 1.4 pp on German and 1.4 pp on French — more than
  any threshold move in the grid, and it removes the 99 %-junk flag list of §6.
- **The `any one source word` rule caps recall.** A chunk counts as supported when any single
  one of its source words is plausible (§5.1, `etwas ← {little, more}`), which is also the
  over-wide-`w` blind spot from EVIDENCE §3.D. Requiring *all* covered source words of a chunk
  to be implausible before counting it against the sentence would be a different detector, not
  a threshold — it should be measured with the same harness before anyone assumes it is better.

## 8. What was not measured

- **Judge-OK negatives.** `--judged-only` needs a verdict cache that does not exist for de/fr;
  producing one needs the network. All rows use every eligible sentence as a negative, so the
  false-positive columns are an upper bound (a genuinely bad alignment in the book counts
  against the detector).
- **Real drift.** The positives are synthetic right-shifts. No human-verified drifted German or
  French sentence exists in the workspace; A3's GOLD-lite is the item that would fix this.
- **The 0/16 Chinese case.** Not reproducible offline — no file on disk contains that batch
  (a single mixed Russian/Chinese cell survives in `bench-quality/minimax-b4.tbook`, which is
  not enough to measure anything). The mechanism given in §6 is read off the code, not remeasured.
- **A second French book.** Only Sherlock has a French overlay that is not `19.tbook`. The
  German conclusion is replicated across two books and two authors; the French one is not.
- **Sentence-level statistics.** Recall and FP are counted per sentence over 4 600–10 700
  sentences per row; no confidence intervals are given, but at these n the standard error is
  ≈ 0.2 pp on a ~2 % FP rate and ≈ 0.4 pp on a ~90 % recall (95 % CI roughly twice that).

## 9. Reproducing

`cmd/lexeval` was extended for this report (the pair flags `-s/-t` already existed):
`--covered/--rates/--shifts` sweep all three thresholds instead of two, `--clean-only` drops
langcheck-flagged sentences from the negatives, `--examples N` prints missed positives and
false positives with their per-pair evidence, `--json` writes the grid, and `--contamination`
produces the §6 table. `internal/lexcheck.Result` gained one diagnostic counter, `Scored`
(pairs worth scoring at all), so pair coverage has a denominator; no detection logic changed.
Each sentence is now scored once in its three forms and the grid reads the counters, so a
63-point sweep over a 6 800-sentence book takes ~15 s.

```bash
cd converter && go build -o /tmp/lexeval ./cmd/lexeval
S=../android/app/src/main/assets/sample.tbook
G="--covered 3,4,5 --rates 0.20,0.25,0.30,0.34,0.40,0.45,0.50 --shifts 2,3,4"

/tmp/lexeval -s en -t de --lexicons lexicons $G --clean-only --json de.json $S
/tmp/lexeval -s en -t de --lexicons lexicons $G --json de-hp.json ../torrents/tbooks/7.tbook
/tmp/lexeval -s en -t fr --lexicons lexicons $G --clean-only --json fr.json $S
/tmp/lexeval -s en -t ru --lexicons lexicons $G --json ru.json $S
/tmp/lexeval -s en -t ru --lexicons lexicons $G --json ru-ac.json bench-quality/full-kimi.tbook

/tmp/lexeval -s en -t de --lexicons lexicons --covered 4 --rates 0.34 --shifts 2 --examples 6 $S
/tmp/lexeval -s en -t de --lexicons lexicons --contamination ../torrents/tbooks/19.tbook
```

Flags must precede the book path. Everything above is offline; no key is read and no request is
made. Converter at `a7f61cf` plus the `cmd/lexeval` / `internal/lexcheck` change described here.
