# EPUB/FB2 → `.tbook` converter (Go)

Turns a standard `.epub` (or `.fb2` / `.fb2.zip`) into a **`.tbook`** archive
for the Reader app: every sentence gets a translation **and word-level
alignment**, so the app shows the full-sentence translation with the tapped
word highlighted — fully offline.

## Quick start

```bash
cd converter
go build -o convert ./cmd/convert

# One-time setup
cp .env.example .env            # put your OPENROUTER_API_KEY in it
tools/embalign-setup.sh         # free local word aligner (LaBSE, CPU)
tools/fetch-lexicons.sh en-ru   # dictionary for the free drift check

# Convert (English → Russian by default)
./convert book.epub -o book.tbook

# Other languages; add a language to an existing .tbook; preview only
./convert book.epub -s de -t ru -o book.tbook
./convert book.tbook -t en
./convert book.epub --dry-run
```

No further flags needed — **the defaults are the measured optimum**
(see [the speed report](https://github.com/adubovskoy/tbook_converter/issues/2)): a 200k-word novel converts in
**~15 minutes for ~$2** on the default `google/gemini-2.5-flash`.
Runs are resumable: interrupt and re-run to continue from the cache; a
fully-cached run assembles offline without an API key.

A plain run does, in order:

1. **Parse + segment** — chapters, images, tables, footnotes, emphasis
   preserved; front/back matter skipped; sentences tokenized with rune offsets.
2. **Glossary** (1 extra call) — recurring terms + proper nouns, enforced in
   every batch so names stay consistent. `--no-glossary` skips it (also needed
   to reuse caches made before glossary became the default).
3. **Translate** — batches of 16, 32 requests in parallel.
4. **Align** — free local LaBSE word alignment (`hybrid` mode); the ~4% of
   sentences the quality gate rejects are re-aligned by the LLM. Without the
   embalign setup the run falls back to full LLM alignment with a notice.
5. **Lexcheck** — free offline dictionary drift check; flags go to
   `<out>.lexflagged.json`.
6. **Assemble + validate** — structure, offsets, and alignment coverage.

If you can't/won't use an API key: `./convert book.epub --provider claude`
runs every batch on your logged-in `claude` CLI subscription (default model
`claude-haiku-4-5`; `MODEL` in `.env` is OpenRouter-only and never leaks into
the claude backend).

Settings precedence: **flags > shell env > `.env` > defaults**.
`./convert --help` lists everything.

## Flags

Core: `-o/--out`, `-t/--target` (comma list), `-s/--source` (default `en`),
`--provider` (`openrouter`|`claude`), `--model`, `--cache-dir` (default
`.tbook_cache`), `--limit-chapters N`, `--dry-run`, `--force` (ignore cache),
`--stats file.jsonl` (per-request latency/provider/tokens/cost log), `-v`.

Performance (defaults are measured optima — change only with reason):
`--batch-size` (16; bigger is *slower* — generation is output-bound),
`--concurrency` (32; gemini took 48 with zero 429s — lower for `:free` models),
`--align-mode` (`hybrid` default | `emb` | `llm`), `--align-batch`,
`--max-retries`, `--embalign-*` (local aligner: python/script/model/layer/
method/threshold), `--provider-sort`/`--provider-order` (OpenRouter routing;
for whole books keep the default — `throughput` sort pins one provider and
serializes concurrency).

Content: `--keep-matter`, `--skip-files pat1,pat2`, `--no-images`, `--no-notes`,
`--skip-citations` (leave bibliographic footnotes untranslated).

Quality: `--no-glossary`, `--no-lexcheck`, `--judge` (semantic verification
report, see below), `--judge-scope` (`flagged` default: suspects + a 5%
calibration sample — seconds per book | `all`), `--judge-model`,
`--judge-invalidate`, `--escalate-model` (redo flagged sentences with a
stronger model — see the warning below), `--invalidate file.json` (clear cached
translations for listed sentences, then exit).

## Quality & verification

Validation proves the file is *well-formed*; two gates check it is *correct*:

- **Lexcheck** (free, offline, on by default): a bilingual dictionary scores
  every alignment pair and flags sentences on aggregate evidence — low support
  or the off-by-one *shift signature*. Measured ~87% recall on drift cascades
  at ~97% precision. Lexicons: `tools/fetch-lexicons.sh` (all pairs of
  de/en/es/fr/it/ru).
- **`--judge`**: an independent LLM reads source, translation, and word mapping,
  and writes flagged sentences to `<out>.flagged.json`. With the default
  `hybrid` alignment treat it as a **report**, not an automatic gate — the
  judge over-flags the embedding aligner's per-word style (measured 53–57%
  false flags on correct de→ru alignments — see the speed report, issue #2).

To redo sentences a report (or your own reading) flagged:

```bash
./convert book.epub --invalidate book.tbook.lexflagged.json
./convert book.epub -o book.tbook        # re-translates only those
```

**Escalation warning** (measured — speed report, issue #2): with the default
`hybrid` alignment, leave `--escalate-model` off. The hybrid gate already
LLM-re-aligns every suspicious sentence inside the pipeline; the post-run
lexcheck flags that remain are almost entirely dictionary-coverage artifacts
(foreign quotes, macaronic passages) with correct alignments. Escalating them
burns tokens and can degrade good sentences to no-highlight fallback — and
never use a reasoning-tier model (`*-pro`) as the escalator (p95 90 s/request).

## How it works (short version)

Translation and alignment are **two decoupled passes** — a single combined
pass collapses into positional drift at batch scale. Pass 1 translates
(`{id,src}` → text). Pass 2 aligns the finished translation: locally via
SimAlign-style LaBSE embeddings (mutual argmax — structurally immune to
positional drift, and it *beats* the LLM align pass on lexcheck for en→ru and
de→ru), with the LLM numbered-echo contract (`"index:text"`) as fallback for
gated sentences. The raw pass-1 text is canonical: alignment can place
highlights but can never rewrite the text. Everything is cached per sentence
(`promptVersion|model|source|target|src`), so runs resume and contract bumps
re-align without re-translating.

Full details: [the speed report](https://github.com/adubovskoy/tbook_converter/issues/2) (measurements & tuning),
[`../doc/specs/tbook-format.md`](../doc/specs/tbook-format.md) (format),
[`../doc/specs/article.md`](../doc/specs/article.md) (design history).

## Ship it

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
internal/translate LLM clients (OpenRouter HTTP, claude CLI), prompts, batching/retry
internal/tbook     data model, ZIP assembly, validation
```
