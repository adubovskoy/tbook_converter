package cli

import (
	"context"
	"testing"

	"github.com/dimando/reader/converter/internal/config"
	"github.com/dimando/reader/converter/internal/langcheck"
	"github.com/dimando/reader/converter/internal/tbook"
)

// The verbatim pair from the gonka probe: a well-formed, fully covered, q = 1
// answer in the wrong language.
const (
	gateSrc  = "It is a hobby of mine to have an exact knowledge of London."
	gateBad  = "我的一个爱好是对伦敦了如指掌。"
	gateGood = "Одно из моих увлечений — досконально знать Лондон."
)

func gateSentences() []*tbook.Sentence {
	return []*tbook.Sentence{
		{Src: gateSrc, Words: [][2]int{{0, 2}}, Tr: map[string]tbook.Translation{
			"ru": {Text: gateBad, Align: []tbook.AlignChunk{{T: [2]int{0, 3}, W: []int{0}}}},
		}},
		{Src: "“Wedlock suits you,” he remarked.", Words: [][2]int{{1, 8}}, Tr: map[string]tbook.Translation{
			"ru": {Text: gateGood, Align: []tbook.AlignChunk{{T: [2]int{0, 4}, W: []int{0}}}},
		}},
	}
}

// The verbatim false positive the first B1 run produced: correct German, but
// six of its seven words are proper nouns a German text legitimately keeps, so
// the untranslated signal fires. The gate must NOT delete it.
const (
	addrSrc = "“Is Briony Lodge, Serpentine Avenue, St. John’s Wood.”"
	addrDe  = "„Ist Briony Lodge, Serpentine Avenue, St. John’s Wood.“"
)

func addrSentences() []*tbook.Sentence {
	return []*tbook.Sentence{
		{Src: addrSrc, Words: [][2]int{{1, 3}}, Tr: map[string]tbook.Translation{
			"de": {Text: addrDe, Align: []tbook.AlignChunk{{T: [2]int{0, 3}, W: []int{0}}}},
		}},
	}
}

// TestLangGateKeepsAnUntranslatedOnlyFlag: the kind has no measured precision
// and its known failure shape is a correct translation, so it is reported and
// nothing more — in either format.
func TestLangGateKeepsAnUntranslatedOnlyFlag(t *testing.T) {
	for _, format := range []string{config.FormatV1, config.FormatV2} {
		t.Run(format, func(t *testing.T) {
			cfg := &config.Config{Source: "en", Targets: []string{"de"},
				Format: format, LangcheckGate: true}
			sents := addrSentences()
			if err := runLangGate(context.Background(), cfg, nil, sents, nil); err != nil {
				t.Fatalf("runLangGate: %v", err)
			}
			if got := sents[0].Tr["de"]; got.Text != addrDe || len(got.Align) != 1 {
				t.Errorf("the gate touched a correct translation: %+v", got)
			}
		})
	}
	// …and it is still flagged, so the report and the badge can say so.
	cfg := &config.Config{Source: "en", Targets: []string{"de"}}
	flags := langGateFlags(cfg, addrSentences())
	if kind := flags["de"][addrSrc]; kind != langcheck.KindUntranslated {
		t.Errorf("kind = %q, want %q", kind, langcheck.KindUntranslated)
	}
	if langcheck.Gating(langcheck.KindUntranslated) {
		t.Error("untranslated must not be a gating kind")
	}
}

// With no glossary namespace to re-translate through, the gate has nothing to
// spend and goes straight to its residual handling — which is the half worth
// pinning down: version 1 has no status slot, so a cell it could not fix ships
// with no translation at all rather than shipping fluent Chinese.
func TestLangGateStripsResidualInV1(t *testing.T) {
	cfg := &config.Config{Source: "en", Targets: []string{"ru"},
		Format: config.FormatV1, LangcheckGate: true}
	sents := gateSentences()
	if err := runLangGate(context.Background(), cfg, nil, sents, nil); err != nil {
		t.Fatalf("runLangGate: %v", err)
	}
	if got := sents[0].Tr["ru"]; got.Text != "" || len(got.Align) != 0 {
		t.Errorf("wrong-language cell shipped %+v, want an empty translation", got)
	}
	if got := sents[1].Tr["ru"].Text; got != gateGood {
		t.Errorf("the good cell was touched: %q", got)
	}
}

// Version 2 has a status for exactly this (§6.7): the text stays and the
// assembly badges it wrongLang, so the reader can see what happened.
func TestLangGateKeepsResidualInV2(t *testing.T) {
	cfg := &config.Config{Source: "en", Targets: []string{"ru"},
		Format: config.FormatV2, LangcheckGate: true}
	sents := gateSentences()
	if err := runLangGate(context.Background(), cfg, nil, sents, nil); err != nil {
		t.Fatalf("runLangGate: %v", err)
	}
	if got := sents[0].Tr["ru"].Text; got != gateBad {
		t.Errorf("text = %q, want it kept for the wrongLang badge", got)
	}
}

func TestLangGateOffLeavesEverythingAlone(t *testing.T) {
	cfg := &config.Config{Source: "en", Targets: []string{"ru"},
		Format: config.FormatV1, LangcheckGate: false}
	sents := gateSentences()
	if err := runLangGate(context.Background(), cfg, nil, sents, nil); err != nil {
		t.Fatalf("runLangGate: %v", err)
	}
	if got := sents[0].Tr["ru"].Text; got != gateBad {
		t.Errorf("--langcheck-gate=false still changed the book: %q", got)
	}
}
