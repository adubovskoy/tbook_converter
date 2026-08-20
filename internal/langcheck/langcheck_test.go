package langcheck

import (
	"testing"

	"github.com/dimando/reader/converter/internal/tbook"
)

func sent(src string, tr map[string]string) *tbook.Sentence {
	s := &tbook.Sentence{Src: src, Tr: map[string]tbook.Translation{}}
	for t, text := range tr {
		s.Tr[t] = tbook.Translation{Text: text}
	}
	return s
}

// The en→de sentences of the broken multi-language book: German prose carrying
// the Russian terms of an en→ru glossary, and whole sentences that gave up and
// continued in Russian. Both must be flagged; the clean German sentence must
// not be.
func TestForeignScriptCatchesTheGlossaryLeak(t *testing.T) {
	sentences := []*tbook.Sentence{
		sent("Uncle Henry and Aunt Em had a big bed in one corner, and Dorothy a little bed in another corner.",
			map[string]string{"de": "Дядя Генри und тётя Эм hatten ein großes Bett in einer Ecke, und Дороти ein kleines Bett in einer anderen Ecke."}),
		sent("There was a razorstorm coming in.",
			map[string]string{"de": "Ein бритвенный шторм aufziehend."}),
		sent("Dorothy lived in the midst of the great Kansas prairies.",
			map[string]string{"de": "Удивительный волшебник из страны Оз war ein Buch."}),
		sent("She wore a greatcoat and closed the collar.",
			map[string]string{"de": "Sie trug einen Mantel und schloss den Kragen."}),
	}
	flags := Check(sentences, "en", []string{"de"}, Options{})
	if got := len(flags); got != 3 {
		t.Fatalf("flags = %d, want 3 (three leaking sentences, one clean): %+v", got, flags)
	}
	for _, f := range flags {
		if f.Kind != KindForeignScript {
			t.Errorf("kind = %q, want %q", f.Kind, KindForeignScript)
		}
		if len(f.Words) == 0 || f.Share <= 0 {
			t.Errorf("flag carries no evidence: %+v", f)
		}
	}
}

// The other measured case: a batch of an en→ru book answered in Chinese, which
// validation passed as OK and lexcheck could not see at all.
func TestForeignScriptCatchesAWrongLanguageBatch(t *testing.T) {
	sentences := []*tbook.Sentence{
		sent("“No,” Sylveste said, through clenched teeth.",
			map[string]string{"ru": "“不，”西尔维斯特咬着牙说。"}),
		sent("“No,” Sylveste said, through clenched teeth.",
			map[string]string{"ru": "— Нет, — сказал Сильвест сквозь стиснутые зубы."}),
	}
	flags := Check(sentences, "en", []string{"ru"}, Options{})
	if len(flags) != 1 || flags[0].Kind != KindForeignScript {
		t.Fatalf("want exactly one foreign-script flag, got %+v", flags)
	}
	if flags[0].Share < 0.9 {
		t.Errorf("a fully Chinese sentence should be ~all foreign letters, share = %v", flags[0].Share)
	}
}

// Legitimate mixed script must stay clean: Latin names inside a Russian
// translation are the source's own script, and a Cyrillic line quoted by an
// English original is carried over on purpose.
func TestForeignScriptDoesNotFlagLegitimateMixing(t *testing.T) {
	sentences := []*tbook.Sentence{
		sent("Dorothy lived in the midst of the great Kansas prairies with Uncle Henry.",
			map[string]string{"ru": "Дороти жила среди огромных прерий Kansas вместе с дядей Генри."}),
		sent("He read the sign: «Осторожно, злая собака» and walked past it slowly.",
			map[string]string{"de": "Er las das Schild: «Осторожно, злая собака» und ging langsam daran vorbei."}),
		// short sentences survive translation unchanged all the time (names,
		// dates, interjections) — the untranslated signal must not judge them
		sent("Kansas, 1899.", map[string]string{"de": "Kansas, 1899."}),
		sent("Hallo, Toto!", map[string]string{"de": "Hallo, Toto!"}),
	}
	if flags := Check(sentences, "en", []string{"ru", "de"}, Options{}); len(flags) != 0 {
		t.Errorf("no flags expected, got %+v", flags)
	}
}

// A pivot copied through instead of translated.
func TestUntranslated(t *testing.T) {
	sentences := []*tbook.Sentence{
		sent("The dig was in disarray, with floods and gravitometers toppled and broken.",
			map[string]string{"de": "The dig was in disarray, with floods and gravitometers toppled and broken."}),
		sent("The dig was in disarray, with floods and gravitometers toppled and broken.",
			map[string]string{"es": "La excavación estaba en desorden, con reflectores y gravitómetros derribados y roto."}),
	}
	flags := Check(sentences, "en", []string{"de", "es"}, Options{})
	if len(flags) != 1 || flags[0].Kind != KindUntranslated || flags[0].Target != "de" {
		t.Fatalf("want one untranslated flag on de, got %+v", flags)
	}
}

// One answer served for two languages — the same-script leak the script signal
// cannot see (here ru/uk, as in the broken book).
func TestDuplicateTarget(t *testing.T) {
	same := "Дороти жила среди огромных прерий вместе с дядей Генри и тётей Эм."
	sentences := []*tbook.Sentence{
		sent("Dorothy lived in the midst of the great Kansas prairies with Uncle Henry and Aunt Em.",
			map[string]string{"ru": same, "uk": same}),
		sent("Toto barked.", map[string]string{"ru": "Тотошка залаял.", "uk": "Тотошка загавкав."}),
		// short enough to be legitimately identical in two languages
		sent("Oz.", map[string]string{"ru": "Оз.", "uk": "Оз."}),
	}
	flags := Check(sentences, "en", []string{"ru", "uk"}, Options{})
	if len(flags) != 1 {
		t.Fatalf("want exactly one duplicate flag, got %+v", flags)
	}
	f := flags[0]
	if f.Kind != KindDuplicateTarget || f.Target != "uk" || f.Other != "ru" {
		t.Errorf("unexpected flag: %+v", f)
	}
}

// An unknown language code must not produce script flags — guessing would
// condemn a whole correct book.
func TestUnknownLanguageIsNotJudged(t *testing.T) {
	if Known("en", "xx") {
		t.Error("xx should be unknown")
	}
	if !Known("en", "de") {
		t.Error("en→de should be known")
	}
	sentences := []*tbook.Sentence{
		sent("Dorothy lived in the midst of the great Kansas prairies.",
			map[string]string{"xx": "Дороти жила среди огромных прерий Канзаса."}),
	}
	if flags := Check(sentences, "en", []string{"xx"}, Options{}); len(flags) != 0 {
		t.Errorf("unknown target must not be script-judged, got %+v", flags)
	}
}

func TestSummarizeAndSrcsByTarget(t *testing.T) {
	flags := []Flag{
		{Target: "de", Kind: KindForeignScript, Src: "a"},
		{Target: "de", Kind: KindForeignScript, Src: "a"}, // duplicate src, one entry
		{Target: "de", Kind: KindUntranslated, Src: "b"},
		{Target: "fr", Kind: KindDuplicateTarget, Src: "c"},
	}
	sums := Summarize(flags, []string{"de", "fr", "es"})
	if len(sums) != 2 || sums[0].Target != "de" || sums[0].Total != 3 ||
		sums[0].Kinds[KindForeignScript] != 2 {
		t.Fatalf("unexpected summary: %+v", sums)
	}
	byTarget := SrcsByTarget(flags)
	if len(byTarget["de"]) != 2 || len(byTarget["fr"]) != 1 {
		t.Fatalf("unexpected srcs: %+v", byTarget)
	}
}
