# EPUB → `.tbook` converter (Go)

A single self-contained Go binary that turns a standard `.epub` into a
**`.tbook`** archive for the [Reader app](../android): every sentence gets a
translation **and word-level alignment**, so the app shows the full-sentence
translation with the tapped word highlighted — fully offline.

It does the whole job — **parse → translate → assemble → validate** — calling
**OpenRouter** for translation (any model; no Anthropic). The translation cache
is resumable, so interrupted runs continue where they left off and adding a
language only translates the new one. It also preserves source formatting —
per-paragraph roles (subtitles, scene breaks) and inline italic/bold. The
`.tbook` format (version 1) is specified in
[`../doc/specs/tbook-format.md`](../doc/specs/tbook-format.md).

## Build

```bash
cd converter
go build -o convert ./cmd/convert
```

## Configure

```bash
cp .env.example .env
# edit .env: set OPENROUTER_API_KEY, pick MODEL, etc.
```

Get a key at <https://openrouter.ai/keys>. The default model is
`google/gemini-2.5-flash` (fast, cheap, reliable JSON); alternatives are listed
(commented) in `.env.example`.
Settings precedence: **command-line flags > shell env > `.env` > defaults**.

## Use

```bash
# Preview parsing/segmentation only — no API calls, no output:
./convert ../robert_shekli-alien_harvest.epub --dry-run

# English → Russian:
./convert ../robert_shekli-alien_harvest.epub -o sample.tbook

# Multiple target languages in one file (hub-and-spoke; source is the pivot):
./convert book.epub -t ru,de,es -o book.tbook
```

Flags: `-o/--out`, `-t/--target` (comma list), `-s/--source` (default `en`),
`--model`, `--cache-dir` (default `.tbook_cache`), `--batch-size`,
`--concurrency`, `--max-retries`, `--limit-chapters N`, `--dry-run`,
`--force` (re-translate, ignoring cache), `-v`.
Run `./convert --help` for the full list.

The run shows a progress bar, retries transient failures (network/5xx/429 with
backoff) and sentences the model drops, then **validates** the result and prints
`sentences / empty / offset_errors`. Sentences left untranslated are written
with an empty translation (no highlight) — re-run to fill them.

A fully-cached run assembles **offline with no API key** (resume / re-assemble).

## How it works

1. **Parse** the EPUB in spine order (`archive/zip` + `goquery`); chapters split
   on `<div class="title1">`; front matter (`title`/`epigraph` divs) skipped;
   the cover extracted.
2. **Segment** paragraphs into sentences (the `sentencizer` library, after a fix
   that re-inserts spaces missing after `…"`-style breaks) and tokenize source
   **words with rune offsets**.
3. **Translate + align** via OpenRouter: per batch, the model returns ordered
   `{tgt, en}` chunks (target fragment → the **source word(s) as text** it
   translates).
4. **Resolve + smart-join**: match each chunk's source text back to word indices
   and concatenate fragments, computing the `[start,end)` highlight spans
   **deterministically** (the model never emits offsets).
5. **Cache** every sentence on disk (`.tbook_cache/`, keyed by
   `promptVersion|model|source|target|src`) → fully resumable.
6. **Assemble** `manifest.json` + `cover.jpg` + `chapters/chN.json` into the ZIP,
   then validate.

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
internal/config    .env + flag resolution
internal/epub      EPUB → chapters of paragraph text
internal/segment   sentence segmentation + word tokenization (rune offsets)
internal/align     model chunks → translation text + highlight spans
internal/cache     resumable on-disk translation cache (sha256-keyed)
internal/translate OpenRouter client, prompt, batching/retry pipeline
internal/tbook     data model, ZIP assembly, validation
```
