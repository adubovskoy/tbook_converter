# Offline align-knob grid — 2026-09-19

Plan item **D4**: sweep the alignment knobs that were never swept — `EMBALIGN_METHOD`,
`EMBALIGN_LAYER`, `EMBALIGN_TGT_GLUE_MIN`, `EmbQMin`, `--align-batch` — offline, with no API
calls, and adopt whatever raises content-word tap coverage without raising lexcheck flags
(`PLAN` §5 D4; evidence base `E` §4.4, §7.10).

**The headline: one knob moves, and it moves a lot.** `EMBALIGN_METHOD=itermax` cuts the share of
content-word taps that light nothing from **2.21 % to 0.62 %** on *Altered Carbon* and from
**2.07 % to 0.58 %** on *Revelation Space* — 130 and 176 source words that were silent under a tap
and are not any more, and none that go the other way — against a run-to-run noise floor of
**exactly zero**. The price is 5 new lexcheck flags on AC (30 → 33) against 2 that disappear, and
a shift of recovered content taps into the `shared` class. Every other knob is either a no-op on
this data (`TGT_GLUE_MIN` between 0.2 and 0.4), a regression (every layer but 8), or unmeasurable
here (unit glue, whose boundaries come from the file that scores it).

## 1. Method

### 1.1 What an arm is

An arm is one book re-**aligned** — never re-translated. Both books already had their full Russian
translation in the cache from earlier runs, so every arm is `convert … --align-mode emb`, which
reports `0 to translate, N to align`: identical text, one alignment knob changed. That is what
makes the grid interpretable — `tbookdiff` reports `translation text changed: 0 / 1303` for every
AC arm and `0 / 1575` for every RS arm, so every difference in every metric below is the aligner's.

| | *Altered Carbon*, ch. 1–5 (`ac-*`) | *Revelation Space*, ch. 1–2 (`rs-*`) |
|---|---|---|
| source | `bench-quality/kimi-a.tbook` (already carries ru) | `alaster_rejnolds-revelation_space-1512290726.epub`, `--limit-chapters 2` |
| sentences | 1 303 (1 279 unique to align) | 1 575 (1 566 unique) |
| taps scored | 14 143 (6 746 content) | 21 245 (10 609 content) |
| glossary | 40 terms, `g:6c8e9c945df1` | 39 terms, `g:e2c9bc7b9a7a` |
| translate model | `moonshotai/Kimi-K2.6`, prompt v5, batch 8, temp 0.3, no repair | same |

### 1.2 How offline-ness was verified

Every arm ran inside its own network namespace (`unshare -r -n`), where the only interface is a
down `lo`. From inside, `proxy.gonka.gg:443`, `huggingface.co:443` and `1.1.1.1:53` all fail with
`OSError [Errno 101] Network is unreachable`. On top of that the converter was handed
`GONKA_BASE_URL=http://127.0.0.1:9/v1` (the discard port) with `GONKA_API_KEY=offline-no-key`, and
the aligner `HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1` so the LaBSE encoder can only come from the
local cache. No `.env` file was read at any point.

The predecessor's proof of this is `bench-quality/.artifacts/align-grid/offline-proof.txt`; it was
re-verified for this report by re-running arms under the same namespace, which is also how the
settings below were recovered. An arm that needed the network fails loudly rather than quietly
falling back: the first reconstruction attempt, before the translation cache was wired up
correctly, decided it had 1 279 sentences to translate and every one of those requests died on
`dial tcp 127.0.0.1:9: connect: connection refused`. A run that reports `0 to translate` and
finishes, as every arm here does, has provably made no API call.

### 1.3 Recovering each arm's settings

The arms carry their settings unevenly, and this matters for how much of the grid can be trusted:

- **Recorded in the log**: the matching method (`Starting embedding aligner (…, argmax|itermax)`),
  the encoder depth (`encoder truncated to N layers`) and unit glue
  (`EXPERIMENTAL unit glue enabled (EMBALIGN_UNIT_GLUE)` plus the span file it read).
- **Recorded in `manifest.meta.runs[].options`**: `embMethod`, `embQMin` (0.7 in every arm),
  `alignMode`, `batchSize`, `concurrency`, `glossaryTerms`, `limitChapters`.
- **Recorded nowhere**: `EMBALIGN_TGT_GLUE_MIN`. The `tgt02` / `tgt04` / `tgtoff` arms and the
  `combo2` arms could only be read off their names.

Rather than infer, **every arm in the grid was re-run from its hypothesised settings and
reproduced**. The aligner is deterministic (§3), so a re-run that reproduces an artifact's
alignment chunk-for-chunk identifies its settings:

| re-run with | reproduces | sentences with a changed alignment |
|---|---|---|
| defaults, ×3 | `ac-base` | 0 / 1 279 each |
| defaults, ×2 | `rs-base` | 0 / 1 566 each |
| `EMBALIGN_METHOD=itermax` | `ac-itermax`, `rs-itermax` | 0 / 1 279, 0 / 1 566 |
| `EMBALIGN_LAYER=6\|7\|9\|10` | `ac-l6…l10`, `rs-l6…l10` | 0 in all 8 |
| `EMBALIGN_TGT_GLUE_MIN=0.2` | `ac-tgt02`, `rs-tgt02` | 0, 0 |
| `EMBALIGN_TGT_GLUE_MIN=0.4` | `ac-tgt04`, `rs-tgt04` | 0, 0 |
| `EMBALIGN_TGT_GLUE_MIN=0` | `ac-tgtoff`, `rs-tgtoff` | 0, 0 |
| `EMBALIGN_UNIT_GLUE=1 --units-file probe-dev.json` | `ac-unit` | 0 / 1 279 |
| itermax + unit glue | `ac-combo1` | 0 / 1 279 |
| itermax + `TGT_GLUE_MIN=0` | `ac-combo2`, `rs-combo2` | 0, 0 |

All 24 reproductions are in `bench-quality/.artifacts/align-grid/rerun/`. So the grid's two
undocumented knobs are now pinned by experiment: `tgt02` = 0.2, `tgt04` = 0.4, `tgtoff` = 0, and
`combo2` = `itermax` with target glue off.

`TGT_GLUE_MIN=0` is not "threshold zero" but the off switch — `tools/embalign.py:225` returns the
pair set untouched when the value is `<= 0`. The reproductions also prove the environment variable
reaches the Python child at all, which is what makes the `tgt02`/`tgt04` null result (§4.3) a
finding rather than a broken arm.

### 1.4 The two arms the grid was missing

The grid intended `unit` and `combo1` on both books and only ever produced them for AC. Both were
run for RS here, and both are **null by construction**: unit glue needs expression spans, the only
span file in the workspace is `probe-dev.json`, and none of its 194 expressions occurs in
*Revelation Space* ch. 1–2. `rs-unit` reproduces `rs-base` and `rs-combo1` reproduces
`rs-itermax`, 0 / 1 566 sentences changed in both. They are carried in the grid below for
completeness; they measure nothing about the lever.

### 1.5 Instruments

Every arm was scored with exactly the same five commands, all offline, all written to
`bench-quality/.artifacts/align-grid/scores/`:

```sh
go run ./cmd/score     <arm>.tbook -t ru --probe bench-quality/.artifacts/probe-dev.json \
                       --dump scores/score-<arm>.json
go run ./cmd/tapledger <arm>.tbook --lang ru                --json scores/tl-<arm>.json
go run ./cmd/tapledger <arm>.tbook --lang ru --content-only --json scores/tlc-<arm>.json
go run ./cmd/tbookdiff -t ru --worst 15 --dump scores/diff-<arm>.json <corpus>-base.tbook <arm>.tbook
python3 bench-quality/probe_align.py <arm>.tbook -t ru --probe bench-quality/.artifacts/probe-dev.json --cover
```

Lexcheck counts are the length of each arm's `<arm>.tbook.lexflagged.json`, written by the
pipeline's own lexcheck pass during the run. `scores/grid-metrics.json` is the aggregate.

**The probe set is AC-only.** `probe-dev.json` holds 194 expressions mined from *Altered Carbon*;
131 of them match AC ch. 1–5 and **0 of 194 match RS ch. 1–2**. `probe_align.py --cover` divides by
zero on an empty match set, so the RS probe files in `scores/` record `probes matched 0
(skipped 194)` and the traceback. The workspace's other RS probe set,
`.artifacts/glossary-scale/rs-probe.json`, is a glossary-term set (328 items keyed by `terms`), not
an expression set, so it cannot stand in. **No probe metric exists for RS**, and none of the
probe columns below should be read as a cross-book result.

## 2. The grid

### Table A — settings, cost, coverage, flags

| arm | knob | wall s | target cov all / content | source tap cov all / content | sentences tap < 0.80 | lexcheck flags | alignments changed vs base |
|---|---|---|---|---|---|---|---|
| `ac-base` | defaults: argmax, layer 8, tgt-glue 0.3 | 265 | 0.9919 / 0.9930 | 0.9722 / **0.9784** | 30 / 1069 | 30 | 0 / 1279 |
| `ac-base7` | defaults (repeat) | 315 | 0.9919 / 0.9930 | 0.9722 / **0.9784** | 30 / 1069 | 30 | 0 / 1279 |
| `ac-base11` | defaults (repeat) | 231 | 0.9919 / 0.9930 | 0.9722 / **0.9784** | 30 / 1069 | 30 | 0 / 1279 |
| `ac-itermax` | `METHOD=itermax` | 520 | 0.9964 / 0.9969 | 0.9922 / **0.9957** | 4 / 1069 | 33 | 688 / 1279 |
| `ac-itermax11` | `METHOD=itermax` (repeat) | 234 | 0.9964 / 0.9969 | 0.9922 / **0.9957** | 4 / 1069 | 33 | 688 / 1279 |
| `ac-l6` | `LAYER=6` | 326 | 0.9886 / 0.9901 | 0.9679 / **0.9748** | 37 / 1069 | 32 | 549 / 1279 |
| `ac-l7` | `LAYER=7` | 419 | 0.9918 / 0.9929 | 0.9712 / **0.9784** | 27 / 1069 | 32 | 439 / 1279 |
| `ac-l9` | `LAYER=9` | 502 | 0.9905 / 0.9915 | 0.9704 / **0.9764** | 34 / 1069 | 31 | 443 / 1279 |
| `ac-l10` | `LAYER=10` | 554 | 0.9889 / 0.9902 | 0.9620 / **0.9697** | 48 / 1069 | 32 | 601 / 1279 |
| `ac-tgt02` | `TGT_GLUE_MIN=0.2` | 409 | 0.9919 / 0.9930 | 0.9722 / **0.9784** | 30 / 1069 | 30 | 0 / 1279 |
| `ac-tgt04` | `TGT_GLUE_MIN=0.4` | 454 | 0.9919 / 0.9930 | 0.9722 / **0.9784** | 30 / 1069 | 30 | 0 / 1279 |
| `ac-tgtoff` | `TGT_GLUE_MIN=0` (off) | 416 | 0.9145 / 0.9404 | 0.9722 / **0.9784** | 30 / 1069 | 15 | 581 / 1279 |
| `ac-unit` | `UNIT_GLUE=1` + probe-dev spans | 404 | 0.9919 / 0.9930 | 0.9747 / **0.9810** | 25 / 1069 | 30 | 88 / 1279 |
| `ac-combo1` | itermax + `UNIT_GLUE=1` | 509 | 0.9964 / 0.9969 | 0.9929 / **0.9962** | 4 / 1069 | 33 | 707 / 1279 |
| `ac-combo2` | itermax + `TGT_GLUE_MIN=0` | 506 | 0.9552 / 0.9703 | 0.9922 / **0.9957** | 4 / 1069 | 22 | 793 / 1279 |
| `rs-base` | defaults: argmax, layer 8, tgt-glue 0.3 | 335 | 0.9915 / 0.9929 | 0.9728 / **0.9817** | 24 / 1413 | 42 | 0 / 1566 |
| `rs-base7` | defaults (repeat) | 442 | 0.9915 / 0.9929 | 0.9728 / **0.9817** | 24 / 1413 | 42 | 0 / 1566 |
| `rs-itermax` | `METHOD=itermax` | 684 | 0.9958 / 0.9972 | 0.9919 / **0.9957** | 3 / 1413 | 38 | 956 / 1566 |
| `rs-l6` | `LAYER=6` | 577 | 0.9884 / 0.9897 | 0.9646 / **0.9762** | 31 / 1413 | 36 | 801 / 1566 |
| `rs-l7` | `LAYER=7` | 571 | 0.9905 / 0.9914 | 0.9702 / **0.9819** | 25 / 1413 | 39 | 653 / 1566 |
| `rs-l9` | `LAYER=9` | 763 | 0.9889 / 0.9895 | 0.9656 / **0.9750** | 38 / 1413 | 41 | 661 / 1566 |
| `rs-l10` | `LAYER=10` | 834 | 0.9862 / 0.9878 | 0.9577 / **0.9687** | 51 / 1413 | 35 | 868 / 1566 |
| `rs-tgt02` | `TGT_GLUE_MIN=0.2` | 641 | 0.9915 / 0.9929 | 0.9728 / **0.9817** | 24 / 1413 | 42 | 0 / 1566 |
| `rs-tgt04` | `TGT_GLUE_MIN=0.4` | 608 | 0.9915 / 0.9929 | 0.9728 / **0.9817** | 24 / 1413 | 42 | 0 / 1566 |
| `rs-tgtoff` | `TGT_GLUE_MIN=0` (off) | 652 | 0.9028 / 0.9375 | 0.9728 / **0.9817** | 24 / 1413 | 28 | 874 / 1566 |
| `rs-combo2` | itermax + `TGT_GLUE_MIN=0` | 631 | 0.9458 / 0.9647 | 0.9919 / **0.9957** | 3 / 1413 | 32 | 1105 / 1566 |
| `rs-unit` | `UNIT_GLUE=1` + probe-dev spans | 241 | 0.9915 / 0.9929 | 0.9728 / **0.9817** | 24 / 1413 | 42 | 0 / 1566 |
| `rs-combo1` | itermax + `UNIT_GLUE=1` | 308 | 0.9958 / 0.9972 | 0.9919 / **0.9957** | 3 / 1413 | 38 | 956 / 1566 |

### Table B — tap ledger and probes

| arm | clean | clean-multi | shared | smear | none | content-word silent | content-only clean / shared / none | probe unit % | probe word cov % |
|---|---|---|---|---|---|---|---|---|---|
| `ac-base` | 51.33 | 3.92 | 40.81 | 0.83 | 3.10 | **2.21** | 57.93 / 33.40 / 2.21 | 77.1 | 93.0 |
| `ac-base7` | 51.33 | 3.92 | 40.81 | 0.83 | 3.10 | **2.21** | 57.93 / 33.40 / 2.21 | 77.1 | 93.0 |
| `ac-base11` | 51.33 | 3.92 | 40.81 | 0.83 | 3.10 | **2.21** | 57.93 / 33.40 / 2.21 | 77.1 | 93.0 |
| `ac-itermax` | 54.66 | 2.86 | 40.69 | 0.79 | 1.00 | **0.62** | 56.83 / 37.33 / 0.62 | 82.4 | 97.5 |
| `ac-itermax11` | 54.66 | 2.86 | 40.69 | 0.79 | 1.00 | **0.62** | 56.83 / 37.33 / 0.62 | 82.4 | 97.5 |
| `ac-l6` | 50.92 | 4.28 | 40.20 | 0.64 | 3.97 | **2.86** | 57.80 / 32.69 / 2.86 | 74.0 | 90.7 |
| `ac-l7` | 51.16 | 4.14 | 40.56 | 0.72 | 3.42 | **2.21** | 57.90 / 33.19 / 2.21 | 74.8 | 91.7 |
| `ac-l9` | 50.94 | 3.80 | 40.79 | 0.92 | 3.55 | **2.64** | 57.68 / 33.10 / 2.64 | 75.6 | 90.7 |
| `ac-l10` | 50.58 | 3.95 | 40.08 | 0.95 | 4.44 | **3.29** | 58.81 / 31.54 / 3.29 | 71.8 | 89.4 |
| `ac-tgt02` | 51.33 | 3.92 | 40.81 | 0.83 | 3.10 | **2.21** | 57.93 / 33.40 / 2.21 | 77.1 | 93.0 |
| `ac-tgt04` | 51.33 | 3.92 | 40.81 | 0.83 | 3.10 | **2.21** | 57.93 / 33.40 / 2.21 | 77.1 | 93.0 |
| `ac-tgtoff` | 55.16 | 0.62 | 41.12 | 0.01 | 3.10 | **2.21** | 62.82 / 33.72 / 2.21 | 75.6 | 93.0 |
| `ac-unit` | 50.58 | 3.85 | 40.67 | 1.98 | 2.92 | **2.03** | 57.10 / 33.40 / 2.03 | 93.1 | 99.5 |
| `ac-combo1` | 53.67 | 2.81 | 40.51 | 2.09 | 0.93 | **0.56** | 55.81 / 37.24 / 0.56 | 88.5 | 100.0 |
| `ac-combo2` | 57.05 | 0.77 | 40.91 | 0.27 | 1.00 | **0.62** | 59.71 / 37.70 / 0.62 | 80.9 | 97.5 |
| `rs-base` | 51.22 | 4.48 | 40.15 | 1.04 | 3.11 | **2.07** | 58.20 / 32.59 / 2.07 | n/a | n/a |
| `rs-base7` | 51.22 | 4.48 | 40.15 | 1.04 | 3.11 | **2.07** | 58.20 / 32.59 / 2.07 | n/a | n/a |
| `rs-itermax` | 54.80 | 3.40 | 39.91 | 0.92 | 0.97 | **0.58** | 57.35 / 36.20 / 0.58 | n/a | n/a |
| `rs-l6` | 50.76 | 4.79 | 39.36 | 0.96 | 4.14 | **2.76** | 58.07 / 31.84 / 2.76 | n/a | n/a |
| `rs-l7` | 51.08 | 4.73 | 39.75 | 0.96 | 3.48 | **2.19** | 58.63 / 31.89 / 2.19 | n/a | n/a |
| `rs-l9` | 50.60 | 4.66 | 39.79 | 1.03 | 3.92 | **2.83** | 57.80 / 32.14 / 2.83 | n/a | n/a |
| `rs-l10` | 50.48 | 4.67 | 39.03 | 1.05 | 4.76 | **3.41** | 58.34 / 31.18 / 3.41 | n/a | n/a |
| `rs-tgt02` | 51.22 | 4.48 | 40.15 | 1.04 | 3.11 | **2.07** | 58.20 / 32.59 / 2.07 | n/a | n/a |
| `rs-tgt04` | 51.22 | 4.48 | 40.15 | 1.04 | 3.11 | **2.07** | 58.20 / 32.59 / 2.07 | n/a | n/a |
| `rs-tgtoff` | 55.84 | 0.47 | 40.53 | 0.05 | 3.11 | **2.07** | 63.85 / 33.06 / 2.07 | n/a | n/a |
| `rs-combo2` | 57.97 | 0.62 | 40.11 | 0.33 | 0.97 | **0.58** | 61.04 / 36.50 / 0.58 | n/a | n/a |
| `rs-unit` | 51.22 | 4.48 | 40.15 | 1.04 | 3.11 | **2.07** | 58.20 / 32.59 / 2.07 | n/a | n/a |
| `rs-combo1` | 54.80 | 3.40 | 39.91 | 0.92 | 0.97 | **0.58** | 57.35 / 36.20 / 0.58 | n/a | n/a |

`alignments changed vs base` counts sentences whose set of alignment chunks differs from the
corpus baseline, computed directly from the two `.tbook` files (`tbookdiff`'s own
regressed/improved counts are in `scores/diff-<arm>.txt`). `content-word silent` is the tap
ledger's headline learner metric: the share of content-word taps (≥ 4 letters, not a stopword)
that light nothing at all. **`wall s` is each arm's own recorded time and is not comparable across
rows** — the grid's arms were timed under concurrent load, and `rs-unit` / `rs-combo1`, produced
for this report, were not. §6 gives the wall times measured under one set of conditions.

## 3. The noise floor: exactly zero

The grid carries three baseline repeats on AC (`ac-base`, `ac-base7`, `ac-base11`) and two on RS
(`rs-base`, `rs-base7`), run hours apart with different aligner thread counts (11 / 7 / 11). They
are **not merely close — they are identical**:

| compared | sentences | alignment chunks changed |
|---|---|---|
| `ac-base` vs `ac-base7` | 1 279 | **0** |
| `ac-base7` vs `ac-base11` | 1 279 | **0** |
| `rs-base` vs `rs-base7` | 1 566 | **0** |
| `ac-itermax` vs `ac-itermax11` | 1 279 | **0** |
| 24 clean re-runs vs the arms they reproduce (§1.3) | 1 279 / 1 566 | **0 in all 24** |

So the run-to-run noise floor on every quality metric in the grid is **0.0000 coverage,
0.00 pp on every tap class, 0 lexcheck flags, 0 probe verdicts**. The local aligner is
deterministic: the same text, the same encoder, the same knobs give the same pairs regardless of
thread count or machine load — and, as §1.3 shows, across a reboot and a different day. A knob
therefore "moves" a metric if it changes it *at all*; the usual question of whether a difference
exceeds the spread does not arise, and the baseline repeats are best read as proof of that rather
than as a spread. Everything the grid reports as a difference is a real difference.

The one thing that is **not** reproducible is wall time. The recorded `.wall` values in Table A
were taken while a second grid arm *and* the A1 gonka probe sweeps were running on the same
machine; the same arms re-run alone under identical conditions are 2–5× faster. Wall time in
Table A is therefore comparable only within a row-group, and §6 gives the clean numbers.

## 4. What each knob does

### 4.1 `EMBALIGN_METHOD`: argmax → itermax — the one real lever

`itermax` is the recall-first matcher of SimAlign (Jalili Sabet et al., 2020): where `argmax`
keeps only the mutual-argmax subword pairs, `itermax` runs a second round over the rows and
columns still unmatched, so a source word whose best target was already claimed gets another
chance among what is left (`tools/embalign.py:80`). It only ever adds pairs — and on this data it
is **purely additive for content words**:

| | AC | RS |
|---|---|---|
| content-word tap coverage | 0.9784 → **0.9957** (+1.73 pp) | 0.9817 → **0.9957** (+1.40 pp) |
| content-word taps that light nothing | 149 / 6 746 → **42** (2.21 % → **0.62 %**) | 220 / 10 609 → **62** (2.07 % → **0.58 %**) |
| source words silent → lit (`cmd/score` content list) | **130** | **176** |
| source words lit → silent | **0** | **0** |
| sentences with content tap coverage < 0.80 | 30 → 4 | 24 → 3 |
| target coverage (content) | 0.9930 → 0.9969 | 0.9929 → 0.9972 |
| probe unit rate / word coverage | 77.1 → 82.4 % / 93.0 → 97.5 % | no probes |
| lexcheck flags | 30 → **33** (5 new, 2 gone) | 42 → **38** (2 new, 6 gone) |
| wall | see §6 | see §6 |

Two honest qualifications:

1. **The recovered taps arrive as `shared`, not as `clean`.** On the content-only ledger, AC goes
   clean 57.93 → 56.83, shared 33.40 → 37.33, none 2.21 → 0.62 (RS: 58.20 → 57.35, 32.59 → 36.20,
   2.07 → 0.58). A word that lit nothing now lights a token that also answers another word's
   question. By the ledger's own ordering that is a clear improvement over silence and a small
   step down from an exclusive highlight.
2. **Function-word mappings degrade.** Four of the 5 sentences AC's lexcheck newly flags carry a
   genuine wrong closed-class mapping — §5.2 quotes three. The fifth is a lexcheck false positive
   on a *correct* recovery (§5.2, last block). This is the price of a recall-first matcher: it
   pairs words it previously left alone, and some of those pairs are wrong.

`itermax` also **halves the hybrid gate's load**: the share of sentences whose stored `q` falls
below 0.9 drops from 2.3 % to 0.9 % (AC, 29 → 12 sentences) and 2.5 % to 1.1 % (RS, 39 → 17).

### 4.2 `EMBALIGN_LAYER`: 8 is already the optimum

| layer | AC content tap cov | AC cw-silent | AC probe unit % | RS content tap cov | RS cw-silent |
|---|---|---|---|---|---|
| 6 | 0.9748 | 2.86 % | 74.0 | 0.9762 | 2.76 % |
| 7 | 0.9784 | 2.21 % | 74.8 | 0.9819 | 2.19 % |
| **8 (default)** | **0.9784** | **2.21 %** | **77.1** | **0.9817** | **2.07 %** |
| 9 | 0.9764 | 2.64 % | 75.6 | 0.9750 | 2.83 % |
| 10 | 0.9697 | 3.29 % | 71.8 | 0.9687 | 3.41 % |

Layer 7 ties the default on AC and is 0.02 pp above it on RS source tap coverage while being
0.12 pp worse on content-word silence; layer 8 wins the probe unit rate outright by 2.3 pp. Depth
also costs wall time monotonically. **Nothing to adopt** — but the knob is now swept, and the
default is confirmed rather than assumed.

### 4.3 `EMBALIGN_TGT_GLUE_MIN`: inert above zero, harmful at zero

`glue_target` is the mirror of the idiom glue for uncovered **target** words: it attaches a
translation word that nothing points at to its own best source word when their similarity clears
the floor (default: `GLUE_MIN` = 0.3). It can only change **target** coverage — source tap
coverage is untouched in every arm below, exactly as the contract says.

| value | AC target cov (all) | AC clean-multi | AC lexcheck | RS target cov (all) | RS lexcheck | alignments changed, AC / RS |
|---|---|---|---|---|---|---|
| 0.2 | 0.9919 | 3.92 | 30 | 0.9915 | 42 | **0 / 0** |
| 0.3 (default) | 0.9919 | 3.92 | 30 | 0.9915 | 42 | — |
| 0.4 | 0.9919 | 3.92 | 30 | 0.9915 | 42 | **0 / 0** |
| 0 (off) | **0.9145** | **0.62** | **15** | **0.9028** | **28** | 581 / 874 |

Between 0.2 and 0.4 the arms are byte-identical on both books: every pair the mirror step accepts
already clears 0.4, so the threshold has no purchase in that range. Turning it off costs 7.7 pp
(AC) and 8.9 pp (RS) of target coverage and nearly all `clean-multi` taps.

This row also carries the grid's most useful warning about its own instrument: **`tgtoff` raises
no lexcheck flag at all and loses many** (30 → 15 on AC, 42 → 28 on RS) because there are fewer
aligned pairs left to check. Flag count falls when an arm aligns *less*. A rule of the form "adopt what does not raise
lexcheck flags" must be read together with the coverage columns, never alone.

### 4.4 Unit glue: not adoptable on this data, by construction

`EMBALIGN_UNIT_GLUE` makes every word of a known multi-word expression tap whatever the expression
rendered as. Its arms look excellent and the numbers cannot be believed:

| | `ac-base` | `ac-unit` | `ac-combo1` (itermax + unit) |
|---|---|---|---|
| probe unit rate | 77.1 % | **93.1 %** | **88.5 %** |
| probe word coverage | 93.0 % | **99.5 %** | **100.0 %** |
| content tap coverage | 0.9784 | 0.9810 | 0.9962 |
| smear (all taps) | 0.83 % | **1.98 %** | **2.09 %** |
| smear (content taps) | 1.10 % | **2.18 %** | **2.58 %** |
| lexcheck flags | 30 | **30** (0 new, 0 gone) | 33 |

The expression spans are read from `bench-quality/.artifacts/probe-dev.json` — *the same file*
`cmd/score --probe` and `probe_align.py --cover` score the arm against. A 100.0 % word coverage
against the boundaries you were handed measures nothing (`E` §7.8). And lexcheck cannot arbitrate:
`CheckSentence` counts a chunk supported if **any one** of its source words is plausible, and unit
glue only ever widens `w` — the arm's `0 new, 0 gone` is that blindness, observed.

What is visible is the cost: smear roughly doubles, on all taps and on content taps alike.
**Do not adopt.** The lever waits on D1's source-side expression tagging for a boundary source
independent of the eval.

### 4.5 `EmbQMin`: measured without an arm

`EmbQMin` decides which sentences a `hybrid` run hands to the LLM align pass. It changes no
alignment by itself, so instead of arms the grid reports the gated share implied by each arm's
stored `q`:

| arm | q < 0.6 | q < 0.7 (default) | q < 0.8 | q < 0.9 |
|---|---|---|---|---|
| `ac-base` | 0.1 % | 0.2 % | 0.4 % | 2.3 % |
| `ac-itermax` | 0.1 % | 0.2 % | 0.3 % | 0.9 % |
| `ac-tgtoff` | 1.2 % | 3.7 % | 11.4 % | 34.2 % |
| `ac-combo2` | 0.4 % | 1.3 % | 4.4 % | 15.7 % |
| `rs-base` | 0.0 % | 0.0 % | 0.1 % | 2.5 % |
| `rs-itermax` | 0.0 % | 0.0 % | 0.1 % | 1.1 % |
| `rs-tgtoff` | 1.7 % | 4.1 % | 13.2 % | 39.8 % |
| `rs-combo2` | 0.5 % | 1.1 % | 4.9 % | 21.6 % |

At the default 0.7 these five and two chapters gate 0.2 % / 0.0 % of sentences, well under the
3.4–3.5 % `E` §4.4 records for a full book — a corpus effect, not a knob effect. Nothing here
argues for moving the threshold: what 0.7 costs and buys is a question about the LLM tail, and
this grid deliberately has no LLM tail. The one transferable number is that `itermax` cuts the
q < 0.9 population by about 2.4× (AC 29 → 12 sentences, RS 39 → 17), so adopting it also
shrinks whatever tail a `hybrid` run pays for.

## 5. Verbatim: what changed, for better and for worse

### 5.1 `itermax`, better

```
word: "object"
SRC:  “Money wasn’t an object, Mr Kovacs.”
TR:   «Деньги не имели значения, мистер Ковач.»
base:    lights nothing        itermax: lights «значения»

word: "showed"
SRC:  The words showed an edge of frustration that she shouldn’t have been letting me see.
TR:   В её словах прозвучала нотка раздражения, которую ей не следовало бы показывать мне.
base:    lights nothing        itermax: lights «прозвучала»

word: "magnitude"  (RS)
SRC:  “…it’s an order of magnitude above anything we’ve experienced before.”
TR:   — …это на порядок выше всего, что мы испытывали раньше.
base:    lights nothing        itermax: lights «порядок»
```

### 5.2 `itermax`, worse

All three sit on sentences AC's lexcheck newly flags.

```
SRC:  I paused to watch the thing realign itself.
TR:   Я остановился, чтобы посмотреть, как штука перестраивает себя.
  "the":  base → «штука»          itermax → «перестраивает»

SRC:  Fascinated at why someone would lie so ineptly to a total stranger, I tried to put her back at her ease.
TR:   Поражённый тем, что кто-то так неумело лжёт совершенно незнакомому человеку, я попытался вернуть её в спокойное состояние.
  "put":  base → «вернуть»        itermax → «спокойное»
  "why":  base → (nothing)        itermax → «что»            ← the same sentence also improves

SRC:  I am not the kind of man to take my own life, and even if I were, I would not have bungled it in this fashion.
TR:   Я не из тех, кто покончит с собой, а даже если бы и был, я бы не облажался вот так.
  "I":    base → «я»              itermax → «бы»
  "own":  base → «собой»          itermax → «с»
```

The shape is consistent: what `itermax` breaks is determiners, prepositions and pronouns — the
closed class — while what it fixes is content words. That is the trade the numbers in §4.1 make
in aggregate, visible one tap at a time.

The fifth newly flagged AC sentence is not a regression at all — it is lexcheck's own
low-support false positive on two taps `itermax` recovered correctly, which is why "5 new flags"
is not the same as "5 new defects":

```
SRC:  I wasn’t feeling much like human interaction.
TR:   Мне не очень хотелось человеческого общения.
  "feeling": base → (nothing)     itermax → «хотелось»
  "like":    base → (nothing)     itermax → «хотелось»
```

### 5.3 Target glue, what the default buys

Turning `TGT_GLUE_MIN` off leaves these translation words unreachable by any tap:

```
SRC:  They shredded invisibly through the attack suit and into the flesh beneath.
TR:   Они невидимо разорвали боевой костюм и вонзились в плоть под ним.
  dark without target glue: «вонзились», «ним»

SRC:  I lifted the head again and smashed it down again, feeling the skull give soggily with the impact.
TR:   Я поднял голову снова и снова разбил её о пол, чувствуя, как череп податливо прогибается от удара.
  dark without target glue: «пол», «прогибается»

SRC:  Give them their due, on Harlan’s World at least the re-sleeving rooms are decked out in pastel
      colours and the attendants are pretty.
TR:   Должное им отдав, на мире Харлана по крайней мере комнаты для инкапсуляции отделаны в
      пастельных тонах, а обслуживающий персонал — симпатичный.
  dark without target glue: «крайней», «персонал»
```

### 5.4 Unit glue, both faces in two taps

```
correct — the expression becomes one unit
SRC:  The thick cropped hair was black shot through with grey.
TR:   Густые короткие волосы были чёрными с проседью.
  "shot"/"through": base → «с»    unit → «с проседью»

wrong — one bad pair widened into two
SRC:  I became abruptly aware that I was swinging a hard-on like a filled fire hose.
TR:   Я внезапно осознал, что у меня стоит так, будто я размахиваю заполненным пожарным шлангом.
  "a":       base → «размахиваю»            unit → «размахиваю заполненным»
  "hard-on": base → «заполненным»           unit → «размахиваю заполненным»
```

`hard-on` was already mapped to the wrong word; unit glue spread the error to a second one, and
neither lexcheck nor the probe metrics can see it.

## 6. Wall time

Every knob was re-run once more with nothing on the machine except the arm and its pair partner
(7 aligner threads each). Even so, **wall time is the one number in this report that should be
read loosely.** `tgt02` does exactly the computation the default does — its alignment is
byte-identical — and came out at 162 s against the default's 71 / 77 / 82 s, because it happened
to be paired with the heaviest RS arm. The clean column is useful for ordering, not for ratios
finer than about 1.5×.

| knob | AC wall s (alone) | RS wall s (alone) | AC wall s (as recorded in the grid) |
|---|---|---|---|
| defaults (argmax, layer 8, tgt-glue 0.3) | 71 / 77 / 82 | 114 / 146 | 265 / 315 / 231 |
| `METHOD=itermax` | 105 | 118 | 520 |
| `LAYER=6` | 70 | 116 | 326 |
| `LAYER=7` | 83 | 213 | 419 |
| `LAYER=9` | 98 | 238 | 502 |
| `LAYER=10` | 138 | 219 | 554 |
| `TGT_GLUE_MIN=0.2` | 162 | 193 | 409 |
| `TGT_GLUE_MIN=0.4` | 125 | 196 | 454 |
| `TGT_GLUE_MIN=0` | 117 | 117 | 416 |
| `UNIT_GLUE=1` | 104 | 241 | 404 |
| itermax + `UNIT_GLUE=1` | 178 | 308 | 509 |
| itermax + `TGT_GLUE_MIN=0` | 87 | 165 | 506 |
| `--align-batch 1` | 133 | — | — |
| `--align-batch 8` | 134 | — | — |
| `--align-batch 16` | 117 | — | — |
| `--align-batch 32` | 112 | — | — |

What survives that caveat:

- **Encoder depth costs, above layer 8.** AC runs 77 s at the default depth, 98 s at layer 9 and
  138 s at layer 10. Below 8 the arms (70 s at layer 6, 83 s at layer 7) sit inside the
  baseline's own 71–82 s spread, so shallower is not measurably cheaper here.
- **`itermax` is cheap.** 105 s against 71–82 s on AC; on RS its 118 s is inside the baseline's
  114–146 s. It is a second matching round over what the first left unmatched, reusing the
  similarity matrix the encoder already produced. Call it "under 1.5×" and no finer.
- **Target glue and unit glue are pure post-processing** over an already-computed pair set; every
  difference their arms show is smaller than the spread between two identical runs.
- **`--align-batch` changes nothing but throughput.** The embedding aligner scores every sentence
  pair independently inside a batch, and `--align-batch 1 / 8 / 16 / 32` all reproduce the
  default's alignment chunk for chunk (0 of 1 279 sentences changed in all four). Their wall times
  — 133 / 134 / 117 / 112 s — do not resolve a difference either. The default on gonka is
  `batch-size / 4` = 2, and nothing here argues for moving it.

## 7. Recommendation per knob

The plan's rule for D4: *adopt any setting that improves content-word tap coverage without raising
lexcheck flags.* Applied literally, and then with what the grid learned about the rule itself.

| knob | moves the metric? | lexcheck | verdict |
|---|---|---|---|
| `EMBALIGN_METHOD=itermax` | **yes**, +1.73 pp AC / +1.40 pp RS against a 0.0000 floor | **30 → 33 on AC**, 42 → 38 on RS | **Split.** Adopt for `hybrid`; do not adopt for `emb`-only. See below. |
| `EMBALIGN_LAYER` | no — 8 already wins or ties every column | 30 → 31…32 (AC), 42 → 35…41 (RS) | **Keep 8.** Swept and confirmed. |
| `EMBALIGN_TGT_GLUE_MIN` 0.2 / 0.4 | no — byte-identical to 0.3 | unchanged | **Keep the default.** The knob is inert in this range. |
| `EMBALIGN_TGT_GLUE_MIN=0` | target coverage −7.7 / −8.9 pp | 30 → 15 *because it aligns less* | **Do not adopt.** |
| `EMBALIGN_UNIT_GLUE` | probe metrics are circular; smear doubles | structurally blind (0 new, 0 gone) | **Do not adopt** (`E` §7.8). Waits on D1. |
| `EmbQMin` | no arm; gated share reported per value | — | **No change from this data.** |
| `--align-batch` | no — 1 / 8 / 16 / 32 all reproduce the default alignment exactly | unchanged | **Keep the default** (`batch-size / 4` = 2). Swept and confirmed. |

**On `itermax`, precisely.** Under the rule as written it is adopted on RS and retired on AC,
because AC's flag count rises by 3. That split is itself the finding: the rule's second clause is
book-dependent and, as §4.3 shows, the flag count is not monotone in quality. What the evidence
supports is the narrower claim:

- In the shipped `hybrid` recipe a lexcheck-flagged sentence is sent to the LLM align pass anyway
  (`E` §4.4). The 5 newly flagged AC sentences therefore cost 5 extra LLM align calls — and
  `itermax` simultaneously cuts the *coverage*-gated population by about 2.4×, so the tail gets
  smaller overall. The 130 + 176 recovered content taps are kept. **Recommend `itermax` as the `hybrid`
  default in the `quality` profile.**
- In `emb`-only mode there is no tail to absorb them, and the rule's literal verdict stands:
  **do not change the `emb`-only default on this evidence.**
- **What would retire it:** blind judging of the 5 newly flagged AC sentences plus a sample of the
  130 recovered taps. If more than a third of the recovered taps are wrong-but-present mappings —
  the class coverage is blind to (`E` §3.D) — the +1.7 pp is an illusion and `itermax` goes back.
  That judgement needs the API and is out of scope here.

## 8. What this grid cannot say

- **No RS probe metric exists.** `probe-dev.json` is mined from *Altered Carbon*; 0 of its 194
  expressions occur in *Revelation Space* ch. 1–2. Every probe number is single-book.
- **No `rs-unit` / `rs-combo1` arm is meaningful.** Unit glue needs expression spans, and the only
  span source in the workspace is the AC probe file. Both arms were produced for completeness
  (§1.4, §2) and are identical to `rs-base` / `rs-itermax`, 0 of 1 566 sentences changed: with no
  spans the glue step is a no-op. The lever is single-book on this corpus.
- **Coverage is not correctness.** Every metric here counts whether something lights up, not
  whether the right thing lights up; a permuted alignment scores identically (`E` §3.D). The
  `itermax` recommendation rests on a coverage gain whose correctness has not been judged.
- **`EMBALIGN_TGT_GLUE_MIN` was recovered, not recorded.** It is written into no log and no
  `manifest.meta`; this report pinned it by reproduction. Worth fixing at the source: the aligner's
  effective glue thresholds belong in `manifest.meta.runs[].options` next to `embMethod`.
- **The grid is two books, one pair, one model.** en→ru only, both English science fiction, one
  translator model. `EMBALIGN_LAYER`'s optimum in particular is a property of LaBSE against these
  two languages and is not transferable to de or fr without re-running.

## 9. Reproducing one arm

```sh
cd converter
go build -o convert ./cmd/convert

# a cache holding ONLY the pass-1 translations, so the run has nothing to
# translate and everything to align
python3 - <<'PY'
import hashlib, json, os, zipfile
m = "moonshotai/Kimi-K2.6+g:6c8e9c945df1"
os.makedirs(".cache-d4", exist_ok=True)
with zipfile.ZipFile("bench-quality/kimi-a.tbook") as z:
    for n in sorted(z.namelist()):
        if n.startswith("chapters/") and n.endswith(".json"):
            for para in json.loads(z.read(n))["paragraphs"]:
                for s in para:
                    if not s.get("src"):
                        continue
                    k = hashlib.sha256(("v5|tr|" + m + "|en|ru|" + s["src"]).encode()).hexdigest()
                    src = os.path.join(".cache-exp-kimi-a", k + ".json")
                    if os.path.exists(src) and not os.path.exists(os.path.join(".cache-d4", k + ".json")):
                        os.link(src, os.path.join(".cache-d4", k + ".json"))
PY

ARM=bench-quality/.artifacts/align-grid/rerun/ac-itermax.tbook   # never overwrite a grid arm
cp bench-quality/.artifacts/align-grid/ac-base.tbook.glossary.en-ru.json "$ARM.glossary.en-ru.json"

GONKA_BASE_URL=http://127.0.0.1:9/v1 GONKA_API_KEY=offline-no-key \
HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 EMBALIGN_THREADS=7 \
EMBALIGN_METHOD=itermax \
  unshare -r -n ./convert bench-quality/kimi-a.tbook -o "$ARM" -t ru \
    --provider gonka --align-mode emb --no-repair --batch-size 8 --cache-dir .cache-d4

# scoring — identical for every arm
S=bench-quality/.artifacts/align-grid/scores
go run ./cmd/score     "$ARM" -t ru --probe bench-quality/.artifacts/probe-dev.json --dump $S/score-ac-itermax.json
go run ./cmd/tapledger "$ARM" --lang ru                --json $S/tl-ac-itermax.json
go run ./cmd/tapledger "$ARM" --lang ru --content-only --json $S/tlc-ac-itermax.json
go run ./cmd/tbookdiff -t ru --worst 15 --dump $S/diff-ac-itermax.json \
    bench-quality/.artifacts/align-grid/ac-base.tbook "$ARM"
python3 bench-quality/probe_align.py "$ARM" -t ru --probe bench-quality/.artifacts/probe-dev.json --cover
```

Knob overrides for the other arms: `EMBALIGN_LAYER=6|7|9|10`,
`EMBALIGN_TGT_GLUE_MIN=0.2|0.4|0`, `EMBALIGN_UNIT_GLUE=1 --units-file
bench-quality/.artifacts/probe-dev.json`. Swap `bench-quality/kimi-a.tbook` for
`./alaster_rejnolds-revelation_space-1512290726.epub --limit-chapters 2`, the glossary sidecar for
`rs-base.tbook.glossary.en-ru.json` and the seed cache for `.cache-mb-kimi` (glossary hash
`e2c9bc7b9a7a`) to get the RS arms.

## 10. Artifacts

- Arms, logs and wall times: `bench-quality/.artifacts/align-grid/<arm>.{tbook,log,wall}` plus
  `<arm>.tbook.lexflagged.json`.
- Scores, one set of five per arm: `bench-quality/.artifacts/align-grid/scores/`
  (`score-`, `tl-`, `tlc-`, `diff-`, `probe-`), aggregated in `scores/grid-metrics.json`.
- Clean wall-time re-runs and the settings reproductions:
  `bench-quality/.artifacts/align-grid/rerun/`.
- Offline proof: `bench-quality/.artifacts/align-grid/offline-proof.txt`.
