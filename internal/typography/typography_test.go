package typography

import (
	"strings"
	"testing"
	"unicode"
)

type tc struct {
	name, lang, prev, src, in, want string
}

const n = "\u202F"

var cases = []tc{
	// ASCII quotes
	{"ru ascii full", "ru", "", "“How often?”", `"Как часто?"`, "«Как часто?»"},
	{"ru ascii comma", "ru", "", "“Quite so,” he answered.", `"Совершенно верно," — ответил он.`, "«Совершенно верно», — ответил он."},
	{"ru ascii close-only", "ru", "", "Here it is, and you can read for yourself.”", `Вот он, можете прочитать сами."`, "Вот он, можете прочитать сами»."},
	{"de ascii", "de", "", "“How often?”", `"Wie oft?"`, "„Wie oft?“"},
	{"de ascii mixed", "de", "", "“Why serious?”", `„Warum ernst?"`, "„Warum ernst?“"},
	{"fr ascii", "fr", "", "“How often?”", `"Combien de fois ?"`, "«" + n + "Combien de fois" + n + "?" + n + "»"},
	{"inch mark stays", "ru", "", "He was six feet 2\" tall.", `Его рост был шесть футов 2" дюйма.`, `Его рост был шесть футов 2" дюйма.`},
	{"code stays", "de", "", "Type `\"x\"` here.", "Tippen Sie `\"x\"` hier.", "Tippen Sie `\"x\"` hier."},

	// doubled close and nesting
	{"ru nest inner", "ru", "", "Have the goodness to say ‘sir’ and ‘please.’”", "Будьте любезны говорить «сэр» и «пожалуйста»».", "Будьте любезны говорить „сэр“ и „пожалуйста“»."},
	{"ru double close no evidence", "ru", "", "It is over.”", "Всё кончено»».", "Всё кончено»."},
	{"fr nest inner", "fr", "", "Always say ‘sir.’”", "Dites toujours « monsieur ». »", "Dites toujours “monsieur”." + n + "»"},
	{"de nest inner", "de", "", "“He said ‘no’ to me.”", "„Er sagte „nein“ zu mir.“", "„Er sagte ‚nein‘ zu mir.“"},

	// source mirroring
	{"ru add close", "ru", "", "It would be a pity to miss it.”", "Было бы жаль это пропустить.", "Было бы жаль это пропустить»."},
	{"ru add both", "ru", "", "“Not in the least.”", "Нисколько.", "«Нисколько»."},
	{"ru add close after ?", "ru", "", "When?”", "Когда?", "Когда?»"},
	{"ru stray close", "ru", "", "Her jacket was black.", "Её жакет был чёрным».", "Её жакет был чёрным."},
	{"ru stray close wrapped open-only", "ru", "", "“Your Majesty must pay.", "«Ваше Величество должно заплатить.»", "«Ваше Величество должно заплатить."},
	{"ru stray open wrapped close-only", "ru", "", "When?”", "«Когда?»", "Когда?»"},
	{"ru unwrap", "ru", "", "Born in New Jersey in the year 1858.", "«Родилась в Нью-Джерси в 1858 году.»", "Родилась в Нью-Джерси в 1858 году."},
	{"ru dialogue dash counts as open", "ru", "", "“Wedlock suits you,” he remarked.", "— Брак вам к лицу, — заметил он.", "— Брак вам к лицу, — заметил он."},
	{"ru dialogue dash no close", "ru", "", "“I think,” said he, “that you are right.”", "— Я думаю, — сказал он, — что вы правы.", "— Я думаю, — сказал он, — что вы правы."},
	{"ru title kept", "ru", "", "I advertised in the Chronicle.", "Я дала объявление в «Кроникл».", "Я дала объявление в «Кроникл»."},
	{"de stray close", "de", "", "The driver looked twice at such a shabby fare.", "Der Fahrer sah zweimal hin.“", "Der Fahrer sah zweimal hin."},
	{"de partial kept", "de", "", "It was a pretty expensive joke for them.”", "Es war ein teurer Scherz für sie.“", "Es war ein teurer Scherz für sie.“"},
	{"de add close after inner", "de", "", "Have you a family?’", "Haben Sie eine Familie?‘", "Haben Sie eine Familie?‘"},
	{"de self-contained balanced", "de", "", "“‘L’homme c’est rien,’ as Flaubert wrote.”", "„‚L’homme c’est rien‘, wie Flaubert schrieb.“", "„‚L’homme c’est rien‘, wie Flaubert schrieb.“"},
	{"fr incise kept", "fr", "", "“Never mind,” said Holmes, laughing; “it is my business to know things.", "« Peu importe, dit Holmes en riant ; c'est mon métier de savoir les choses.", "«" + n + "Peu importe, dit Holmes en riant" + n + "; c'est mon métier de savoir les choses."},
	{"punctuation-only cell untouched", "ru", "", "’", "»", "»"},
	{"interior quotes block removal", "ru", "", "By Jove!” he continued, “it is not my duty to—”", "«Ей-богу! — продолжал он, — это не мой долг —»", "«Ей-богу! — продолжал он, — это не мой долг —»"},

	// quotation mark followed by a dialogue dash
	{"ru quote dash", "ru", "", "“‘Oh, never mind about that, Mr. Wilson!", "«— Ах, не беспокойтесь об этом, мистер Уилсон!»", "«Ах, не беспокойтесь об этом, мистер Уилсон!"},
	{"ru quote dash open-only", "ru", "", "“‘What would be the hours?", "«— А каковы были бы часы работы?", "«А каковы были бы часы работы?"},
	{"ru quote dash single src", "ru", "", "‘I should be able to look after that for you.’", "«— Я смогу сам об этом позаботиться».", "«Я смогу сам об этом позаботиться»."},
	{"ru quote dash tight", "ru", "", "“Ten to two.", "«—С десяти до двух.", "«С десяти до двух."},
	{"ru quote dash mid incise kept", "ru", "", "“‘That would suit me,’ said I. ‘And the pay?’", "«— Это мне подошло бы, — сказал я. — А плата?»", "«Это мне подошло бы, — сказал я. — А плата?»"},
	{"ru quote dash needs quoted src", "ru", "", "He said: — Yes.", "«— Да, — сказал он.", "— Да, — сказал он."},
	{"ru plain dialogue dash kept", "ru", "", "“Yes,” said he.", "— Да, — сказал он.", "— Да, — сказал он."},
	{"fr quote dash", "fr", "", "“‘What would be the hours?", "« — Quelles seraient les heures ?", "«" + n + "Quelles seraient les heures" + n + "?"},
	{"fr quote dash nnbsp", "fr", "", "“‘Ten to two.", "«" + n + "— De dix à deux heures.", "«" + n + "De dix à deux heures."},
	{"de no quote dash rule", "de", "", "“‘Ten to two.", "„– Von zehn bis zwei.", "„– Von zehn bis zwei."},

	// English “ ” used as outer marks
	{"fr foreign close", "fr", "", "Dig.”", "Fouiller.”", "Fouiller." + n + "»"},
	{"fr foreign close ?", "fr", "", "Is that understood?”", "Est-ce compris ?”", "Est-ce compris" + n + "?" + n + "»"},
	{"fr foreign pair", "fr", "", "“But what about people, sir?”", "“Mais qu'en est-il des gens, monsieur ?”", "«" + n + "Mais qu'en est-il des gens, monsieur" + n + "?" + n + "»"},
	{"fr foreign open", "fr", "", "“You’re to do the following.", "“Vous devez faire ce qui suit.", "«" + n + "Vous devez faire ce qui suit."},
	{"fr inner quote kept", "fr", "", "You may remember the saying, ‘There is danger for him.’", "Vous connaissez l'adage : “Il y a un danger pour lui.”", "Vous connaissez l'adage" + n + ": “Il y a un danger pour lui.”"},
	{"ru foreign close", "ru", "", "It must be bought.”", "Её нужно купить.”", "Её нужно купить»."},
	{"ru inner close kept", "ru", "", "He said ‘no.’", "Он сказал „нет“", "Он сказал „нет“"},
	{"de foreign close", "de", "", "What a queen she would have made!”", "Was für eine Königin sie abgegeben hätte!”", "Was für eine Königin sie abgegeben hätte!“"},
	{"de close at cell start kept", "de", "", "’ I asked.", "“, fragte ich.", "“, fragte ich."},
	{"de foreign open", "de", "", "“How often?”", "“Wie oft?”", "„Wie oft?“"},

	// Russian placement
	{"ru period out", "ru", "", "“She will not sell.”", "«Она не продаст.»", "«Она не продаст»."},
	{"ru period out mid", "ru", "", "“Go.” Then he left.", "«Иди.» Потом он ушёл.", "«Иди». Потом он ушёл."},
	{"ru ellipsis stays", "ru", "", "“Well…”", "«Ну...»", "«Ну...»"},
	{"ru abbreviation stays", "ru", "", "“And so on.”", "«И т. д.»", "«И т. д.»"},
	{"ru comma out", "ru", "", "“Yes,” said he.", "«Да,» — сказал он.", "«Да», — сказал он."},
	{"ru tag dash", "ru", "“Well, have you solved it?”", "I asked as I entered.", "спросил я, входя.", "— спросил я, входя."},
	{"ru tag dash needs quoted prev", "ru", "About five ft. seven in.", "in height; strongly built.", "крепкого сложения.", "крепкого сложения."},
	{"ru tag dash not for capital", "ru", "“Seven!”", "I answered.", "Я ответил.", "Я ответил."},

	// dashes
	{"ru unspaced em", "ru", "", "“It—it’s not actionable,” he stammered.", "«Это—это не подсудно»,—заикаясь, проговорил он.", "«Это — это не подсудно», — заикаясь, проговорил он."},
	{"ru double hyphen", "ru", "", "Wait--now.", "Подождите--сейчас.", "Подождите — сейчас."},
	{"ru spaced hyphen", "ru", "", "Wait - now.", "Подождите - сейчас.", "Подождите — сейчас."},
	{"ru hyphen compound stays", "ru", "", "Baker Street.", "Бейкер-стрит.", "Бейкер-стрит."},
	{"ru year range stays", "ru", "", "1888–1890.", "1888—1890.", "1888—1890."},
	{"ru leading hyphen dialogue", "ru", "", "“Yes.”", "- Да.", "— Да."},
	{"ru interrupted", "ru", "", "But how—”", "Но как—", "Но как —»"},
	{"de em to spaced en", "de", "", "“I was mad—insane.”", "„Ich war verrückt—wahnsinnig.“", "„Ich war verrückt – wahnsinnig.“"},
	{"de spaced em", "de", "", "One night — it was March.", "Eines Nachts — es war März.", "Eines Nachts – es war März."},
	{"de en compound stays", "de", "", "Berlin to Paris.", "Die Strecke Berlin–Paris.", "Die Strecke Berlin–Paris."},
	{"de interrupted", "de", "", "“But your client—”", "„Aber Ihr Klient—“", "„Aber Ihr Klient –“"},
	{"fr unspaced em", "fr", "", "Warsaw—yes!", "Varsovie—oui !", "Varsovie — oui" + n + "!"},

	// French
	{"fr comma out", "fr", "", "“It was only a joke at first,” groaned our visitor.", "« Ce n'était qu'une plaisanterie au début, » gémit notre visiteur.", "«" + n + "Ce n'était qu'une plaisanterie au début" + n + "», gémit notre visiteur."},
	{"fr nnbsp marks", "fr", "", "What?", "Quoi ? Vraiment!", "Quoi" + n + "? Vraiment" + n + "!"},
	{"fr time kept", "fr", "", "At 10:30.", "À 10:30.", "À 10:30."},
	{"fr url kept", "fr", "", "See http://x.", "Voir http://x.", "Voir http://x."},
	{"fr ?! once", "fr", "", "What?!", "Quoi ?!", "Quoi" + n + "?!"},

	// spacing
	{"space before comma", "ru", "", "Yes, sir.", "Да , сэр .", "Да, сэр."},
	{"ru space inside guillemets", "ru", "", "“Yes.”", "« Да ».", "«Да»."},
	{"de space before close", "de", "", "“Yes.”", "„ Ja. “", "„Ja.“"},
	{"collapse spaces", "de", "", "Yes sir.", "Ja   mein Herr.", "Ja mein Herr."},

	// unsupported language
	{"unsupported", "es", "", "“Yes.”", `"Sí."`, `"Sí."`},
}

func TestApply(t *testing.T) {
	for _, c := range cases {
		got := Apply(Input{PrevSrc: c.prev, Src: c.src, Tr: c.in, Lang: c.lang}).Text
		if got != c.want {
			t.Errorf("%s:\n in   %q\n got  %q\n want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestIdempotent(t *testing.T) {
	for _, c := range cases {
		in := Input{PrevSrc: c.prev, Src: c.src, Tr: c.in, Lang: c.lang}
		once := Apply(in).Text
		in.Tr = once
		twice := Apply(in)
		if twice.Text != once {
			t.Errorf("%s: not idempotent:\n once  %q\n twice %q (%v)", c.name, once, twice.Text, twice.Rules)
		}
	}
}

func letters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestNoLettersChanged(t *testing.T) {
	for _, c := range cases {
		got := Apply(Input{PrevSrc: c.prev, Src: c.src, Tr: c.in, Lang: c.lang}).Text
		if letters(got) != letters(c.in) {
			t.Errorf("%s: letters changed: %q -> %q", c.name, c.in, got)
		}
	}
}

func TestRulesReported(t *testing.T) {
	r := Apply(Input{Src: "“Not in the least.”", Tr: "Нисколько.", Lang: "ru"})
	want := []string{RuleAddOpen, RuleAddClose}
	if strings.Join(r.Rules, ",") != strings.Join(want, ",") {
		t.Errorf("rules %v, want %v", r.Rules, want)
	}
	r = Apply(Input{Src: "“‘Ten to two.", Tr: "«— С десяти до двух.", Lang: "ru"})
	if strings.Join(r.Rules, ",") != RuleQuoteDash {
		t.Errorf("rules %v, want %v", r.Rules, RuleQuoteDash)
	}
	if r := Apply(Input{Src: "Yes.", Tr: "Да.", Lang: "ru"}); len(r.Rules) != 0 {
		t.Errorf("unchanged cell reports rules %v", r.Rules)
	}
}

func TestLangVariants(t *testing.T) {
	if !Supported("ru-RU") || !Supported("FR") || Supported("es") {
		t.Fatal("Supported")
	}
	if got := Fix("“Yes.”", `"Да."`, "ru_RU"); got != "«Да»." {
		t.Errorf("region suffix: %q", got)
	}
}

func TestInnerQuoteAtEnd(t *testing.T) {
	for _, c := range []tc{
		{"de mid quote at end", "de", "", "I tossed my rocket with a cry of “Fire!”", "Ich warf die Rakete mit einem Schrei „Feuer!“ in den Raum.", "Ich warf die Rakete mit einem Schrei „Feuer!“ in den Raum."},
		{"de inner single at end", "de", "", "Now for the ‘Eg.’", "Nun zu dem ‚Eg‘.", "Nun zu dem ‚Eg‘."},
		{"ru mid quote stray close kept out", "ru", "", "He cried “Fire!”", "Он крикнул «Пожар!»", "Он крикнул «Пожар!»"},
		{"ru close-only with apostrophe", "ru", "", "I can’t sleep.”", "Я не могу спать.", "Я не могу спать»."},
	} {
		got := Apply(Input{Src: c.src, Tr: c.in, Lang: c.lang}).Text
		if got != c.want {
			t.Errorf("%s:\n got  %q\n want %q", c.name, got, c.want)
		}
	}
}

func TestInnerCloseMirrored(t *testing.T) {
	c := tc{"ru inner close kept", "ru", "", "“‘Well,’ said I, ‘the gentleman at No. 4.’", "«Ну, — сказал я, — джентльмен из № 4».", "«Ну, — сказал я, — джентльмен из № 4»."}
	if got := Apply(Input{Src: c.src, Tr: c.in, Lang: c.lang}).Text; got != c.want {
		t.Errorf("%s: got %q", c.name, got)
	}
}
