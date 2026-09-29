package langcheck

import "testing"

// The verbatim pair from the gonka probe: DeepSeek-V4-Flash-0731 returned a
// whole en→ru batch in fluent Chinese, in well-formed JSON, with every id
// present (bench-quality/reports/gonka-models-2026-09-19.md).
const (
	chineseSrc = "It is a hobby of mine to have an exact knowledge of London."
	chineseGot = "我的一个爱好是对伦敦了如指掌。"
	russianGot = "Одно из моих увлечений — досконально знать Лондон."
)

func TestWrongScript(t *testing.T) {
	cases := []struct {
		name, src, text, source, target string
		want                            bool
		lang                            string
	}{
		{"chinese for russian", chineseSrc, chineseGot, "en", "ru", true, "zh"},
		{"russian for russian", chineseSrc, russianGot, "en", "ru", false, ""},
		{"chinese for german", chineseSrc, chineseGot, "en", "de", true, "zh"},
		{"russian for german", "Uncle Henry and Aunt Em had a big bed in one corner.",
			"Дядя Генри и тётя Эм имели большую кровать в одном углу.", "en", "de", true, ""},
		{"german for german", "Uncle Henry and Aunt Em had a big bed in one corner.",
			"Onkel Henry und Tante Em hatten ein großes Bett in einer Ecke.", "en", "de", false, ""},
		// The source's own script is NOT allowed: Latin inside Russian is the
		// leak the report signal deliberately tolerates as a quotation.
		{"english for russian", "He had left his enormous fortune in the hands of trustees.",
			"Somebody bequeathed everything to a committee.", "en", "ru", true, ""},
		// …but words the source itself carries are exempt, so a kept title or
		// name never fails the guard.
		{"quoted title kept", `He read "The Times" every morning.`,
			`Он читал «The Times» каждое утро.`, "en", "ru", false, ""},
		// A minority of foreign letters is a rendering choice, not a leak.
		{"latin brand inside russian", "The iPhone rang.", "Зазвонил iPhone.", "en", "ru", false, ""},
		{"unknown target: no opinion", chineseSrc, chineseGot, "en", "xx", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, lang := WrongScript(c.src, c.text, c.source, c.target)
			if got != c.want {
				t.Errorf("WrongScript(%s→%s) = %v, want %v", c.source, c.target, got, c.want)
			}
			if got && lang != c.lang {
				t.Errorf("lang = %q, want %q", lang, c.lang)
			}
		})
	}
}

func TestOutsideScript(t *testing.T) {
	cases := []struct {
		text, target string
		want         bool
	}{
		// The glossary entries that taught the leak into the Latin targets.
		{"Айрин Адлер", "de", true},
		{"Irene Adler", "de", false},
		{"JacSol", "ru", true},
		{"Пространство Откровения", "ru", false},
		// Mixed renderings stay: a brand or a formula may keep its Latin.
		{"СИСС (SISS)", "ru", false},
		{"", "ru", false},
	}
	for _, c := range cases {
		if got := OutsideScript(c.text, c.target); got != c.want {
			t.Errorf("OutsideScript(%q, %s) = %v, want %v", c.text, c.target, got, c.want)
		}
	}
}

func TestDominantLangNamesOnlyUnambiguousScripts(t *testing.T) {
	cases := []struct {
		words []string
		want  string
	}{
		{[]string{"伦敦"}, "zh"},
		{[]string{"こんにちは"}, "ja"},
		{[]string{"東京", "です"}, "ja"}, // kana present: kanji alone would read as Han
		{[]string{"Лондон"}, ""},     // Cyrillic is ru, uk, bg, … — naming one is a guess
		{[]string{"London"}, ""},
	}
	for _, c := range cases {
		if got := dominantLang(c.words); got != c.want {
			t.Errorf("dominantLang(%v) = %q, want %q", c.words, got, c.want)
		}
	}
}
