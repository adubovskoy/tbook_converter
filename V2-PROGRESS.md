# `.tbook` version 2 — producer progress

Spec: `doc/specs/tbook-format-v2.md` (status **Candidate**, ratified by the author
on 2026-09-26). Version 2 is the **default output** (`--format v1` still writes
version 1); converter_web ships with v2 together with the v2-capable readers, with
no v1 pin. Ratification outcomes: 21 of the 22 open decisions accepted as drafted
(including a single `v` object per translation), derived run/split labelling
(§8.3) lowered to MAY, and the website copy of the spec deferred until Final.

## Done

- **`internal/tbook/v2`** — types (manifest, skeleton, overlay, notes), `Link`
  marshalling that keeps `null` and `[]` distinct, tokenizer `tbook-w2`,
  chapter and paragraph id recipes, `alignDigest`, ZIP writer (`mimetype`
  first and STORED, `manifest.json` last, per-entry digests, embedded JSON
  Schema), shape + semantic validator (§11.1), reader, tap algorithm (§8.2).
- **`internal/tbook/v2/migrate`** — version 1 → version 2 per Annex A:
  paragraph join, containment remap of links, `x` escapes, digit
  pass-through, `s: "unaligned"` only, ids, digests, `alignDigest`.
- **`cmd/tbook-migrate`**, **`cmd/tbook-validate`**.
- **`--format v2`** on the converter (`internal/cli/v2out.go`): the pipeline
  still builds the version-1 model; the version-2 archive is derived at write
  time and annotated with real gate state — statuses, verdicts, per-overlay
  `gates`, `meta.runs[].options.tokenizer`, and the `source` identity block.
- **Version guard** — `internal/tbook.Read` refuses `formatVersion != 1` with
  a message naming the version and the v2 path.
- **Append a language** (2026-09-28, spec §2.4) — `convert book.tbook -t xx`
  accepts a version-2 input. `migrate.ToBook` rebuilds the version-1-shaped
  model (one v1 paragraph per block, sentences = `sents` ranges, words = the
  block's `tbook-w2` inventory, every linked token and `x` escape one chunk);
  the new language runs through the unchanged pipeline; `migrate.Overlays`
  maps it onto the **existing** skeleton blocks (nothing re-derived); and
  `v2.Append` truncates at the old manifest's local header, appends the
  overlays and a manifest edited in place (key order and unknown fields kept;
  `targets`, `spine[].gloss`, `notes.gloss`, `digests`, one new `meta.runs[]`
  record), then a central directory reusing the old records verbatim.
  `--format v1` with a version-2 input is refused (a language the book
  already has was refused too until replacement landed, below). A version-1 input keeps the old whole-book path.
  `cmd/score`, `cmd/tbookdiff`, `cmd/typofix` read either version through
  `migrate.ReadBook`. Measured: `ToBook` → `Convert` reproduces every skeleton
  and overlay byte for byte on the migrated `sample.tbook` and
  `full-kimi.tbook`; appending the sample's eighth language to a
  seven-language migration leaves every byte before the old manifest intact,
  validates with 0 violations, and yields overlays byte-identical to a
  whole-book migration (`migrate/append_test.go`, `cli/v2append_test.go`).
  For the pipeline the CLI re-tokenizes sentences with `segment.Tokenize`
  (`pipelineWords`): the aligned cache is keyed by sentence text only, so an
  entry made by an EPUB run holds indices into that inventory, and
  `Overlays` maps them onto `tbook-w2` tokens by containment exactly as the
  EPUB path does. Checked end to end: appending the sample's eighth language
  through the CLI from a cache of its version-1 alignments gives 6,808 of
  6,808 cells identical (text and links) to a whole-book migration.
- **Replace a language** (2026-09-28, spec §2.5, G9) — `convert book.tbook
  -t xx` where `xx` is already a target re-translates it and calls
  `v2.Replace`, which rewrites the archive: every entry that is not an `xx`
  overlay is copied verbatim (local header, data, descriptor) in its original
  order with central records re-offset, the new overlays take the old ones'
  places, the manifest (last, once) drops the old digests and adds the new;
  chapters the run does not cover keep their `xx` overlay verbatim (digest
  unchanged; author's decision 2026-09-28). Append and replace now record
  every run (no collapsing), set only `updatedAt` and cap `runs` at 20 (G8),
  and refuse a non-conformant input naming the entry after the manifest (G10).
  Tested on the fixture (full and partial) and the sample: other entries
  byte-identical with unchanged digests, 0 violations, sample through the CLI
  6,808 of 6,808 cells identical (`migrate/replace_test.go`,
  `cli/v2append_test.go`).
- README documents both tools and the flag.

## Measured

| | version 1 | version 2 | taps equal |
|---|---|---|---|
| `sample.tbook` (8 languages) | 6,963,017 B | 3,554,093 B | 836,496 / 836,600 |
| `full-kimi.tbook` (1 language) | 2,188,722 B | 1,299,559 B | 158,421 / 158,421 |

All 104 sample differences lie in the five paragraphs version 1 cut mid-word;
the version-2 file renders those pages correctly instead. Both outputs pass
`tbook-validate` with no warnings. `go build ./... && go vet ./... && go test
./...` are green; the equivalence tests skip when the reference books are
absent.

## Decisions worth knowing

- **A sampled judge run does not claim the `judge` gate.** With
  `--judge-scope flagged` the pass covers a risk-scoped subset, so the overlay
  lists `x-judge-scoped`. Claiming `judge` would make a missing verdict mean
  "judged and passed" for cells the judge never saw (§6.8).
- **The translation cache key is unchanged.** The tokenizer affects only the
  write-time projection of char-range chunks into token links, which is
  recomputed on every write; the cached artefacts (translation text, aligner
  chunks in rune offsets) are tokenizer-independent. A tokenizer change
  therefore needs a re-write, not a re-translation or re-alignment.
- **`source` carries** kind, file SHA-256, identifier, title and language.
  `textSha256` and `docs[]` are omitted: the EPUB text stream is not retained
  by the parser at write time. Adding them means keeping per-document text
  hashes from `internal/epub` through to assembly.

## Not done

1. ~~Consumers~~ — done 2026-09-19. Android (`data/book/BookSource.kt`,
   `TbookV2Reader.kt`, `TbookW2.kt`, `TbookV2Tap.kt`, `V2BookSource.kt`) and
   desktop (`src-tauri/src/{models,tbook}.rs`,
   `src/lib/{tokenize,alignV2,renderV2}.ts`) open both migrated books and pass
   the Annex B.2 taps.
   ~~Consumer leftovers~~ — done 2026-09-26, both apps:
   - **Locators** (§3.5.2): positions stored as
     `chapterId/paragraphId/sentence/word`, resolved with the cross-chapter
     paragraph-id fallback; unknown paragraph → chapter start → book start; a
     missing sentence/word is dropped, never moved. Version 1 keeps its
     indices. Android saves the top paragraph's first word (its surfaces anchor
     by paragraph + pixel offset, kept when the locator resolves exactly);
     desktop saves and restores the first word on screen.
   - **Integrity** (§9): a missing referenced entry refuses the file on open;
     every entry is digest-checked on first read (damaged skeleton → chapter
     refused, damaged overlay → rejected like a mis-bound one); the whole book is
     verified on import; each overlay's `alignDigest` (and each notes overlay's)
     is recomputed before use and a mismatch rejects it. All 104 + 13 overlays of
     the two migrated books reproduce their digests in Kotlin and TypeScript.
   - **Gates** (§6.8): a flagged cell's note/badge names the gates that ran
     ("checked by …" / "no quality checks ran"); clean cells show nothing.
     Android shows alignment-only flags (`raw`, `unaligned`, `skipped`, `free`)
     as a quiet caption and the rest on the error tint.
   - **Desktop footnotes** (§4.12, §6.12): markers spliced into v2 paragraphs,
     `NoteSheetV2.svelte`, taps against the footnote overlay, which both apps
     now bind per note id (§6.12) — Android did not check that binding before.
   - **Unit mode** (§8.4) is a consumer MAY — not implemented. Run/split
     labelling (§8.3) was already in both.
   Tests: Android 200 JVM tests; desktop 49 Rust tests, 51 TS checks
   (`npm test`), `npm run check` clean.
2. **Aligner shortcut.** `internal/embalign` produces token pairs and projects
   them to rune ranges (`WordStrings`, `embalign.go:348`), which migration then
   maps back to token indices. A later step can have the aligner emit
   `tbook-w2` token indices directly and skip both conversions.
3. **Learner slots.** `units`, `groups` and `parts` are representable and
   validated but nothing fills them; they need a source-side expression pass
   and a morphology pass.
4. ~~`--estimate` and `--progress-file`~~ — re-checked 2026-09-26 with no
   network (`unshare -rn`, a local stub answering the provider): `--estimate`
   prints the same single JSON object on stdout for `--format v1` and `v2`
   (byte-identical); `--dry-run` creates no progress file in either; a real run
   writes the same NDJSON events (`translate` per target, `assemble` 0/1 → 1/1,
   terminal `done`) in both formats. The converter_web contract is unchanged.
