# CLAUDE.md — converter

Repo-specific guidance. The workspace-wide file is `../CLAUDE.md`; the normative format spec is
`../doc/specs/tbook-format.md`. Day-to-day usage, flags and tuning live in `README.md`.

## Language: everything written is English

**All written artefacts in this repo are in English** — commit messages, code comments,
documentation, and **every benchmark or research report** — regardless of the language the work
was discussed in.

The one exception is **linguistic data**: source/target sentences, glossary entries, model output
quoted as evidence, and error examples stay verbatim in their own language. A report on Russian
translation quality must quote «мы вошёл» exactly, never a translation of it, and must not "fix"
its spelling — the misspelling is the finding. Data is quoted; narration is English.

## Benchmark reports

Prose write-ups go in `bench-quality/reports/`, the analysis scripts stay in `bench-quality/`, and
the evidence they consume goes in **`bench-quality/.artifacts/`, which is gitignored** — judge
batches/keys/verdicts, probe sets, diffs, `.tbook` outputs, `*-stats.jsonl`, mined sentence dumps.

That directory is never published: it is verbatim copyrighted book text. It is still kept on disk,
because the reports cite it and reproducing their numbers needs it — do not put research evidence
in `/tmp`, which is wiped between sessions.

The blind pairwise harness used by those reports is `bench-quality/prepare_pairs.py` (sample pairs
from two `.tbook` files, both presentation orders) → judges → `bench-quality/analyze_pairs.py`
(de-swap, decisive/tie, sign test). A pair counts as decisive only when both presentation orders
agree; disagreement is a tie.
