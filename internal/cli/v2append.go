package cli

import (
	"fmt"
	"os"
	"slices"

	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/segment"
	"github.com/dimando/reader/converter/internal/tbook"
	v2 "github.com/dimando/reader/converter/internal/tbook/v2"
	"github.com/dimando/reader/converter/internal/tbook/v2/migrate"
)

// readTbookInput reads a .tbook input of either version into the version-1
// model the pipeline works on. For a version-2 file it also returns the
// archive: the new languages are then appended to it (spec §2.4), and a
// language it already has is replaced (§2.5), rather than the book being
// re-converted.
func readTbookInput(cfg *config.Config) (*tbook.Book, *v2.Archive, error) {
	ver, err := v2.FormatVersionOf(cfg.Input)
	if err != nil {
		return nil, nil, fmt.Errorf("read tbook: %w", err)
	}
	if ver != v2.FormatVersion {
		b, err := tbook.Read(cfg.Input)
		if err != nil {
			return nil, nil, fmt.Errorf("read tbook: %w", err)
		}
		return b, nil, nil
	}
	if cfg.Format == config.FormatV1 {
		return nil, nil, fmt.Errorf("%s is .tbook formatVersion 2 and --format v1 was requested: "+
			"a language is added to a version-2 book by appending to it, which keeps it version 2 — "+
			"drop --format v1, or re-convert the source book with --format v1", cfg.Input)
	}
	arc, err := v2.ReadArchive(cfg.Input)
	if err != nil {
		return nil, nil, fmt.Errorf("read tbook: %w", err)
	}
	for _, t := range cfg.Targets {
		if t == arc.Manifest.Languages.Source {
			return nil, nil, fmt.Errorf("%s is in %s, its source language: it cannot be a target", cfg.Input, t)
		}
	}
	b, err := migrate.ToBook(arc)
	if err != nil {
		return nil, nil, fmt.Errorf("read tbook: %w", err)
	}
	pipelineWords(b)
	return b, arc, nil
}

// pipelineWords gives every sentence the word inventory a fresh conversion
// uses (segment.Tokenize) instead of the file's tbook-w2 words. The aligned
// cache is keyed by sentence text alone, so an entry made by an EPUB run holds
// indices into that inventory, and the write path maps them onto tbook-w2
// tokens by containment either way. The carried-over translations keep their
// text for the language check; their alignments, now meaningless, are
// dropped — their overlays are appended from the file's bytes, not from here.
func pipelineWords(b *tbook.Book) {
	prose, notes, cites := b.Sentences()
	for _, group := range [][]*tbook.Sentence{prose, notes, cites} {
		for _, s := range group {
			s.Words = segment.Tokenize(s.Src)
			for lang, tr := range s.Tr {
				tr.Align = nil
				s.Tr[lang] = tr
			}
		}
	}
}

// skippedCitations is the set of citation sentences left untranslated on
// purpose (--skip-citations); they are written as {s: skipped}.
func skippedCitations(cfg *config.Config, citeSents []*tbook.Sentence) map[*tbook.Sentence]bool {
	citations := map[*tbook.Sentence]bool{}
	if cfg.SkipCitations {
		for _, s := range citeSents {
			citations[s] = true
		}
	}
	return citations
}

// appendV2 appends this run's languages to the version-2 input (spec §2.4):
// every existing entry keeps its bytes, the new overlays follow, and a
// rewritten manifest closes the archive. A language the input already has is
// replaced instead (§2.5): the archive is rewritten with every other entry
// copied byte for byte. The result is validated like any other version-2
// output.
func appendV2(cfg *config.Config, b *tbook.Book, arc *v2.Archive, run tbook.RunRecord, g *gateState) error {
	adds, stats, err := migrate.Overlays(b, arc, cfg.Targets, migrate.Options{
		Annotate: g.annotate,
		Skipped:  g.skipped,
		Gates:    g.gatesFor,
	})
	if err != nil {
		return fmt.Errorf("append v2: %w", err)
	}
	var added, replaced []string
	for _, t := range cfg.Targets {
		if slices.Contains(arc.Manifest.Languages.Targets, t) {
			replaced = append(replaced, t)
		} else {
			added = append(added, t)
		}
	}
	for _, add := range adds {
		if !slices.Contains(replaced, add.Lang) {
			continue
		}
		n, kept := 0, 0
		for i, ref := range arc.Manifest.Spine {
			if ov := add.Overlays[i]; ov != nil {
				n++
			} else if ref.Gloss[add.Lang] != "" {
				kept++
			}
		}
		fmt.Printf("  %s: replaced in %d chapters, existing overlay kept in %d (spec §2.5)\n", add.Lang, n, kept)
	}
	if err := v2.Replace(cfg.Input, cfg.Out, adds, run); err != nil {
		return fmt.Errorf("append v2: %w", err)
	}
	if fi, err := os.Stat(cfg.Out); err == nil {
		how := ""
		if len(added) > 0 {
			how = fmt.Sprintf(", appended %v", added)
		}
		if len(replaced) > 0 {
			how += fmt.Sprintf(", replaced %v", replaced)
		}
		fmt.Printf("Done. %s (%d KB, formatVersion 2%s)\n", cfg.Out, fi.Size()/1024, how)
	}
	fmt.Printf("  %d sentences, %d translations, %d links, %d escapes; gates %s\n",
		stats.Sentences, stats.Translations, stats.TokenLinks, stats.Escapes, gatesSummary(g, cfg.Targets))
	view := &v2.Archive{NotesOverlays: map[string]*v2.NotesOverlay{}}
	for _, add := range adds {
		for _, ov := range add.Overlays {
			view.Overlays = append(view.Overlays, map[string]*v2.Overlay{add.Lang: ov})
		}
		if add.Notes != nil {
			view.NotesOverlays[add.Lang] = add.Notes
		}
	}
	fmt.Println(statusSummary(view))
	return validateV2(cfg.Out)
}
