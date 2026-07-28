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
**~15 minutes for ~$1.30** on the default `google/gemini-3.1-flash-lite`.
Runs are resumable: interrupt and re-run to continue from the cache; a
fully-cached run assembles offline without an API key.

A plain run does, in order:

1. **Parse + segment** — chapters, images, tables, footnotes, emphasis
   preserved; front/back matter skipped; sentences tokenized with rune offsets.
2. **Glossary** (1 extra call) — recurring terms + proper nouns, enforced in
   every batch so names stay consistent. `--no-glossary` skips it (also needed
   to reuse caches made before glossary became the default). The glossary is
   written to `<out>.glossary.<src>-<tgt>.json` next to the output and reused
   verbatim on every later run of the same book and language pair — edit it by
   hand (or run `--only-glossary` first, see below) to control how specific
   terms/names are rendered before translating. Editing it moves the cache
   namespace, so the affected sentences are re-translated.
3. **Translate** — batches of 16, 32 requests in parallel.
4. **Align** — free local LaBSE word alignment (`hybrid` mode); the ~4% of
   sentences the quality gate rejects are re-aligned by the LLM. Without the
   embalign setup the run falls back to full LLM alignment with a notice.
5. **Lexcheck** — free offline dictionary drift check; flags go to
   `<out>.lexflagged.json`.
6. **Assemble + validate** — structure, offsets, and alignment coverage.

Near-free alternative: `./convert book.epub --provider gonka` sends batches to
the [Gonka](https://gonka.ai) decentralized compute network via the
[proxy.gonka.gg](https://proxy.gonka.gg) gateway (OpenAI-compatible; set
`GONKA_API_KEY`). Pricing is ~$0.0001 per 1M tokens — a whole book costs a
fraction of a cent. Default model `moonshotai/Kimi-K2.6` (reasoning is switched
off automatically); `MiniMaxAI/MiniMax-M2.7` is also served but always reasons,
paying reasoning latency on every batch (its inline `<think>` output is
stripped at parse time).

If you can't/won't use an API key: `./convert book.epub --provider claude`
runs every batch on your logged-in `claude` CLI subscription (default model
`claude-haiku-4-5`; `MODEL` in `.env` is OpenRouter-only and never leaks into
the claude backend).

Fully local and free — two backends:

- `--provider ollama` sends batches to an [Ollama](https://ollama.com) server
  (default `http://localhost:11434/v1`, override with `OLLAMA_BASE_URL`).
  Default model `translategemma:12b` — pull it first
  (`ollama pull translategemma:12b`) or set `OLLAMA_MODEL`.
- `--provider llamacpp` sends them to a
  [llama.cpp](https://github.com/ggml-org/llama.cpp) `llama-server`
  (default `http://localhost:8080/v1`, override with `LLAMACPP_BASE_URL`), e.g.
  `llama-server -hf ggml-org/gemma-3-4b-it-GGUF -np 2 -c 8192 --jinja`.
  llama-server serves one model and ignores the requested id, so the served
  model's name is adopted automatically (set `LLAMACPP_MODEL` to pin cache
  keys); `LLAMACPP_API_KEY` matches a server started with `--api-key`.

For both, defaults adapt (unless set by flag/env): batch size 4 (small local
models silently answer only the first few items of a 16-sentence batch),
concurrency 2 (the server queues requests past its parallel slots), request
timeout 300 s. Expect a local run to be far slower than an API model unless
the model fits your GPU.

Settings precedence: **flags > shell env > `.env` > defaults**.
`./convert --help` lists everything.

## Flags

Core: `-o/--out`, `-t/--target` (comma list), `-s/--source` (default `en`),
`--provider` (`openrouter`|`gonka`|`claude`|`ollama`|`llamacpp`), `--model`, `--cache-dir` (default
`.tbook_cache`), `--limit-chapters N`, `--dry-run`, `--force` (ignore cache),
`--stats file.jsonl` (per-request latency/provider/tokens/cost log), `-v`.

`--repair` / `--no-repair` — the proofread pass (see below). **On by default for
`--provider gonka`, off everywhere else**; either flag (or `REPAIR` / `NO_REPAIR`)
overrides that.

Machine integration (for scripts and services driving the converter):

- `--estimate` — parse + segment only (no API calls, no key, no python) and
  print exactly one JSON object to stdout, then exit 0:
  `{title, author, detectedLanguage, chapters, sentences, noteSentences,
  words, chars, warnings}`. `detectedLanguage` comes from the EPUB
  `dc:language` / FB2 `<lang>` metadata, normalized to a lowercase two-letter
  code (`en-US` → `en`); `null` plus a warning when the book carries none.
  Nothing else is written to stdout; parse failures exit non-zero with
  `error: …` on stderr.
- `--only-glossary` — build (or load) the book glossary, write it to
  `<out>.glossary.<src>-<tgt>.json`, open it in your system's default app for
  `.json` files, then exit — no translation runs. Edit the `tgt` of any entry
  under `terms` (or delete an entry to stop enforcing it) and re-run normally;
  the edited file is picked up verbatim instead of rebuilding the glossary, so
  the same edits apply to the real translation. Re-running `--only-glossary`
  just re-opens the file — free, no API key needed. To throw the edits away
  and go back to the model's glossary, delete the file or add `--force`. This
  is an interactive flag: it writes no `--progress-file` events, because it
  produces no `.tbook`.

  The `source`/`target`/`title`/`author`/`sentences` fields at the top of the
  file scope it to one book and one language pair — a file that doesn't match
  the current run (a second target language, or a glossary built under
  `--limit-chapters`) is reported and rebuilt rather than enforced on a book
  it was never built for. Don't hand-edit them.
- `--progress-file file.ndjson` (env `PROGRESS_FILE`) — during a real
  conversion, append NDJSON progress events (one JSON per line, flushed per
  write, throttled to ≤ ~2 lines/sec per phase, final line of a phase always
  written): `{"ts":…,"phase":"translate|embalign|align|judge","target":"ru",
  "done":128,"total":9391}`, then `{"phase":"assemble","done":1,"total":1}`
  and finally `{"phase":"done","ok":true}` (`ok:false` + `error` on a fatal
  failure). The human progress bars are unchanged; `--dry-run`/`--estimate`
  never create the file.

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

Quality: `--no-glossary`, `--only-glossary` (build/edit the glossary, see
above, then exit; `--force` rebuilds it), `--no-lexcheck`, `--judge` (semantic verification
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

Between the two, an optional **pass 1.5 proofreads** the translation
(`{id,src,tr}` → fixed text) so pass 2 aligns the text that actually ships. It
fixes what a learner would copy — agreement, verb/preposition government,
non-words, calqued idioms, register — and is forbidden to paraphrase, to guess a
character's gender or referent without context, or to touch the enforced
glossary. Measured on gonka/Kimi-K2.6: it rewrites ~4% of a book's sentences and
wins 87% of the decisive blind pairwise judgements on exactly those sentences
(p=3.4e-05), for ~5 extra minutes and no change in alignment coverage. That is
why it is default-on where tokens are free and off where they are metered.
Proofread text lives in its own cache namespace and the final aligned entry
carries a `+rp` marker, so toggling the pass never re-translates a book and never
serves an alignment built from the other variant.

Idioms and phrasal verbs map as units: "piss off" → "отвали" claims both
source words (tapping either highlights the pair). The LLM align prompt has an
explicit fixed-expression rule, and the local aligner glues uncovered
expression words to a neighbor's target when embeddings support it
(`EMBALIGN_GLUE_MIN`, default 0.3; 0 disables) — measured +17pp source-word
tap coverage on a real novel with no drift-detection regression.

Full details: [the speed report](https://github.com/adubovskoy/tbook_converter/issues/2) (measurements & tuning),
[`../doc/specs/tbook-format.md`](../doc/specs/tbook-format.md) (format),
[`../doc/specs/article.md`](../doc/specs/article.md) (design history).

## Provenance stamped into the file

Every assembled `.tbook` carries a run record in `manifest.meta`
([spec §3.4](../doc/specs/tbook-format.md#34-meta--provenance-and-service-metadata)):
converter version and commit, the provider, the model of each pass
(translate / align / repair / judge / escalate / embed), each pass's prompt-contract
version, and the run's settings — so months later you can tell what produced a
book. Reading a `.tbook` back (adding a language) appends a record instead of
replacing the history; a re-run with unchanged settings only moves `updatedAt`.

No credentials, hosts, or paths go in there — the spec forbids it and a `.tbook`
gets shared. Inspect it with:

```bash
unzip -p book.tbook manifest.json | jq .meta
```

The version comes from `internal/buildinfo`, `"dev"` unless stamped at build time:

```bash
go build -ldflags "-X github.com/dimando/reader/converter/internal/buildinfo.Version=1.5.1" \
  -o convert ./cmd/convert
```

The commit needs no flag — Go embeds the VCS revision automatically.

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
internal/buildinfo producer identity (version, VCS commit) for manifest.meta
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
