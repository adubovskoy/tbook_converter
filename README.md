# EPUB/FB2 → `.tbook` converter (Go)

A single self-contained Go binary that turns a standard `.epub` (or `.fb2` /
`.fb2.zip`) into a **`.tbook`** archive for the [Reader app](../android): every
sentence gets a translation **and word-level alignment**, so the app shows the
full-sentence translation with the tapped word highlighted — fully offline.

It does the whole job — **parse → translate → assemble → validate** — with two
translation backends: **OpenRouter** (metered API, any model) or
**`--provider claude`**, which shells out to the `claude` CLI in print mode so
every batch runs on your logged-in **Claude subscription** — no API key, no
per-token billing (it consumes the subscription's usage windows instead; on a
limit the run stops with the reset time and resumes from cache). The translation cache
is resumable, so interrupted runs continue where they left off and adding a
language only translates the new one. It preserves source formatting and
content — per-paragraph roles (subtitles, scene breaks), inline italic/bold,
**body images with translatable captions**, **tables with translatable cells**,
and **footnotes** (markers stripped from the prose, bodies extracted and
translatable) — and skips front/back matter (cover pages, synopsis, credits,
index) by default. The `.tbook` format (version 1) is specified in
[`../doc/specs/tbook-format.md`](../doc/specs/tbook-format.md).

## Build

```bash
cd converter
go build -o convert ./cmd/convert
```

## Configure

```bash
cp .env.example .env
# OpenRouter backend: set OPENROUTER_API_KEY, pick MODEL.
# Claude backend: nothing to configure — just a logged-in `claude` CLI.
```

For OpenRouter, get a key at <https://openrouter.ai/keys>; the default model is
`google/gemini-2.5-flash` (fast, cheap, reliable JSON); alternatives are listed
(commented) in `.env.example`. For `--provider claude` the default model is
`claude-haiku-4-5` (override with `CLAUDE_MODEL` / `--model`); `.env`'s `MODEL`
is OpenRouter-only and never leaks into the claude backend, and
`ANTHROPIC_API_KEY` is stripped from the CLI's environment so runs can only
bill to the subscription.
Settings precedence: **command-line flags > shell env > `.env` > defaults**.

## Use

```bash
# Preview parsing/segmentation only — no API calls, no output:
./convert ../robert_shekli-alien_harvest.epub --dry-run

# English → Russian via OpenRouter:
./convert ../robert_shekli-alien_harvest.epub -o sample.tbook

# The same on your Claude subscription (no API key), with the quality loop:
./convert book.epub --provider claude --judge --escalate-model claude-sonnet-4-6 -o book.tbook

# FB2 input works the same way:
./convert book.fb2 --provider claude -o book.tbook

# Multiple target languages in one file (hub-and-spoke; source is the pivot):
./convert book.epub -t ru,de,es -o book.tbook

# Add a language to an EXISTING .tbook — no source EPUB needed. Existing
# translations are kept; only the new target is translated. Overwrites in place:
./convert book.tbook -t en
```

Flags: `-o/--out`, `-t/--target` (comma list), `-s/--source` (default `en`),
`--provider` (`openrouter`|`claude`), `--model`, `--cache-dir` (default
`.tbook_cache`), `--batch-size`, `--concurrency`, `--max-retries`,
`--limit-chapters N`, `--dry-run`, `--force` (re-translate, ignoring cache),
`-v`.

FB2 scope: metadata, cover, sections as chapters (headings, subtitles, scene
breaks) and inline emphasis; FB2 note bodies, poems, epigraphs and images are
skipped (EPUB input supports all of those).

Input may be an `.epub` (fresh conversion) **or** an existing `.tbook` (adds the
`--target` language(s); `--source` and existing translations come from the file,
default output overwrites it in place).

Provider routing (OpenRouter): `--provider-sort throughput|latency|price` and
`--provider-order slug1,slug2` (e.g. `alibaba`). For **bulk** runs prefer the
default routing or `latency` — `throughput` pins the single fastest-tok/s
provider and serializes concurrency (measurably slower for a whole book).

Content flags: `--keep-matter` (don't skip cover/synopsis/credits/index),
`--skip-files pat1,pat2` (extra title/filename patterns to skip), `--no-images`,
`--no-notes`, `--skip-citations` (keep citation-kind footnotes untranslated —
bibliographic references have little learner value and cost tokens).

Quality flags: `--glossary` (one extra call builds a book glossary — recurring
terms + proper nouns — enforced in every translate batch for consistency), the
free static drift check (see below) runs **by default** — `--no-lexcheck`
disables it, `--judge` (semantic verification pass, see below), `--judge-model`,
`--judge-invalidate`, `--escalate-model`. Run `./convert --help` for the full
list.

The run shows a progress bar, retries transient failures (network/5xx/429 with
backoff) and sentences the model drops, then **validates** the result and prints
`sentences / empty / offset_errors`. Sentences left untranslated are written
with an empty translation (no highlight) — re-run to fill them.

A fully-cached run assembles **offline with no API key** (resume / re-assemble).

## How it works

1. **Parse** the EPUB in spine order (`archive/zip` + `goquery`); chapter
   boundaries come from the NCX navigation (all levels flattened — a book's
   parts AND its chapters each become one chapter), falling back to
   `<div class="title1">` splitting; front/back matter is skipped; footnote
   markers are stripped from prose and their bodies resolved from the notes
   document; body images (with captions) and tables are extracted; the cover
   extracted; oversized images downscaled.
2. **Segment** paragraphs into sentences (the `sentencizer` library, after a fix
   that re-inserts spaces missing after `…"`-style breaks) and tokenize source
   **words with rune offsets**.
3. **Translate, then align** via OpenRouter, in **two decoupled passes**: pass 1
   translates (`{id,src}` → text); pass 2 aligns the finished translation
   (`{id,src,words,tr}` → ordered `{tgt, en}` chunks, target fragment → the
   **numbered source word(s)** it translates, echoed as `"index:text"`). Doing
   both in one pass makes the model fall into *positional drift* (target word
   *i* ↔ source word *i*) at batch scale, so they're split — and the align pass
   runs in **small batches** (`--align-batch`, default batch-size/4), which
   measurably curbs drift on cheap models. Pass 1 output containing leaked
   special tokens (`<bos>`, `<|…|>`, U+FFFD) is rejected and retried. The two
   contracts are versioned separately (`TrPromptVersion = v4`,
   `PromptVersion = v6`), so an align-contract change re-aligns a book without
   re-translating it; raw translations cache under a `…|tr|…` namespace.
4. **Resolve + locate**: an `index:text` token resolves to its index when
   the echoed text matches that word (the text is a checksum on the index), else
   falls back to match-by-text (a multi-word string is whitespace-split first).
   The **raw pass-1 translation is the canonical text**: each echoed fragment is
   located inside it (case/punctuation-insensitive, in target order) to compute
   the `[start,end)` highlight spans **deterministically** — the align pass can
   place highlights but can never rewrite the text, so a sloppy echo cannot
   strip punctuation or swap words (pre-v6 it could). A mapped fragment that
   cannot be located means the echo diverged — the sentence is retried, and if
   it never aligns it ships as raw text with no highlights. Punctuation-only
   fragments never make highlights; a list marker (`1.`, `2)`) the model
   dropped from the translation is restored deterministically.
5. **Cache** every sentence on disk (`.tbook_cache/`, keyed by
   `promptVersion|model|source|target|src`) → fully resumable.
6. **Assemble** `manifest.json` + `cover.jpg` + `chapters/chN.json` into the ZIP,
   then validate — reporting structural integrity **and alignment coverage** (a
   `<55%` warning flags gross drift / empty alignment).

## Local embedding alignment (`--align-mode`)

Pass 2 can run **locally for free** instead of through the LLM: a SimAlign-style
aligner (LaBSE token embeddings + mutual argmax, `tools/embalign.py`) matches
words by semantic similarity, so positional drift is structurally impossible.
Benchmarked against the LLM align pass it scores *better* on lexcheck (support
rate 0.710 vs 0.672 en→ru, p≈0.009) at zero token cost — the whole align pass
for a book runs in minutes on CPU.

```bash
tools/embalign-setup.sh      # one-time: creates .venv-embalign (CPU torch);
                             # the first run downloads LaBSE (~1.8 GB)

# Recommended: embedding alignment with an LLM safety net — sentences the free
# gate rejects (lexcheck flag or coverage < --embalign-q, ~7%) are re-aligned
# by the LLM align pass:
./convert book.epub -t ru --align-mode hybrid

# Fully local align pass (no LLM fallback). With a fully translated cache this
# needs no API key at all — e.g. re-aligning after an align-contract bump:
./convert book.epub -t ru --align-mode emb
```

Only en→ru was benchmarked; treat other language pairs as unverified and spot
check with `--judge` before trusting them. Escalation always uses the LLM
align pass regardless of mode.

## Quality & verification

Validation + coverage prove the file is *well-formed and mapped* — they do **not**
prove the alignment is *correct*. A partial positional drift or a wrong-word
mapping keeps coverage ~100%, and a fluent mistranslation is still valid text.
Two verification gates catch those:

**`--lexcheck` — free, offline, instant.** A bilingual dictionary (OPUS
OpenSubtitles word-alignment data; `tools/fetch-lexicons.sh` covers every pair
of de/en/es/fr/it/ru) statically scores each align pair: is the target fragment
a dictionary-plausible rendering of its source word? Isolated misses are
ignored (words are polysemous, dictionaries register-biased); a sentence is
flagged only on aggregate evidence — a low overall support rate, or the
**shift signature** (several fragments that don't fit their own word but fit
its *neighbor* — the fingerprint of an off-by-one cascade, robust to polysemy).
Benchmarked on a real book with synthetic drift injection (`cmd/lexeval`):
~87% recall on full cascades, ~52% on mid-sentence cascades, at ~2.4% false
positives / 97% precision, with 72% of pairs dictionary-covered.

**`--judge` — the semantic gate** (see [spec §10.4](../doc/specs/tbook-format.md)):
an independent LLM reads every sentence's source, translation, and word-mapping
and flags real errors — including mistranslations no dictionary can see. The
judge model defaults to the translate model; `--judge-model` overrides.
Verdicts are cached, so re-runs only judge what changed.

Judge calibration is measured, not guessed (all numbers from a hand-reviewed
page plus synthetic-drift twins, `cmd/driftdemo`, prompt j3): the same-model
cheap judge (flash-lite) catches 12/14 drifted sentences with almost no false
positives; gemini-2.5-flash catches 14/14 but flags 36-53% of clean prose on
mapping pedantry — in a two-chapter loop it even re-flagged **69% of its own
escalated output**, condemning escalation to mass fallback. So: cheap judge +
lexcheck as the standing gate, stronger judge only for one-off audits. Judge
wording is equally delicate: a lenient "flag only reader-visible defects"
phrasing dropped the cheap judge's drift recall to zero — calibrate any prompt
change against drifted twins first.

Flags from both gates feed `--escalate-model` — and escalation is **re-verified**:
the same gates re-check the escalated output, anything still flagged gets one
more attempt, and persistent failures are stripped rather than shipped (a judged
mistranslation ships untranslated, an alignment failure ships as raw text with
no highlights; both are listed in `<out>.unverified.json` and retried on the
next run). A stronger model is not a verified model — without this loop an
escalated hallucination lands in the book unseen.

```bash
# The recommended cheap-model loop — one command:
# translate+align with the cheap model, lexcheck (free) + judge everything,
# then automatically redo ONLY the flagged sentences with a stronger model
# (kept in the same file):
tools/fetch-lexicons.sh es-ru   # once per language pair
convert book.epub -o book.tbook --lexcheck --judge --escalate-model google/gemini-2.5-flash

# Manual variant:
convert book.epub -o book.tbook --judge                  # flags → book.tbook.flagged.json
convert book.epub --invalidate book.tbook.flagged.json   # drop their cached tr + alignment
convert book.epub -o book.tbook                          # redoes only those
# …or in one step: --judge --judge-invalidate, then re-run.
```

Every emitted translation also carries `q` — its alignment-coverage score — so
downstream tools can rank sentences for review.

## Ship it

The reader app loads the bundled book from
`../android/app/src/main/assets/sample.tbook`. To make a freshly converted book
the app's default:

```bash
cp book.tbook ../android/app/src/main/assets/sample.tbook
```

The app also imports any `.tbook` at runtime via its file picker.

## Code layout

```
cmd/convert        CLI entrypoint (flags, orchestration, --dry-run)
cmd/driftdemo      microscope: pipeline + lexcheck + judge on a small passage
cmd/lexeval        lexcheck benchmark (synthetic drift injection on a .tbook)
internal/config    .env + flag resolution
internal/embalign  local embedding word aligner (tools/embalign.py subprocess)
internal/epub      EPUB → chapters of paragraph text
internal/fb2       FB2/FB2.zip → the same parsed-book structure
internal/segment   sentence segmentation + word tokenization (rune offsets)
internal/align     model chunks → highlight spans located in the raw translation
internal/cache     resumable on-disk translation cache (sha256-keyed)
internal/translate LLM clients (OpenRouter HTTP, claude CLI), prompts, batching/retry pipeline
internal/tbook     data model, ZIP assembly, validation
```
