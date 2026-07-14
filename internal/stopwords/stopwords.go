// Package stopwords ships small, high-precision closed-class word lists —
// articles, adpositions, conjunctions, pronouns, copula/auxiliary forms,
// negation and core particles — for the alignment-coverage gate. Function
// words routinely and harmlessly ship unaligned (an article belongs to its
// noun's chunk or to none), so counting them lets their noise drown the
// signal the gate exists for: an unaligned CONTENT word.
//
// The lists are deliberately conservative: a word that can carry content on
// its own (place/time adverbs, verbs of saying, quantities) stays OUT, since
// a stopword here means "its absence from the alignment proves nothing".
// Languages match the converter's supported targets; Set returns nil for
// anything else and callers fall back to counting every word.
package stopwords

import "strings"

// lists holds one whitespace-separated lowercase word list per language code.
var lists = map[string]string{
	"en": `
		a an the
		and or but nor so yet both either neither
		of in on at to for from by with without as into onto over under
		between through after before above below against about along
		i you he she it we they me him her us them my your his its our their
		mine yours hers ours theirs this that these those who whom whose which what
		myself yourself himself herself itself ourselves themselves
		is are was were be been being am
		do does did done have has had having
		will would shall should can could may might must
		not no nor never none
		if then than because while when where why how whether
		there here too very just also only even still already yet again
	`,
	"ru": `
		в во на с со к ко у за по от ото до из изо без под подо над при про
		о об обо для через между перед передо около возле среди сквозь ради
		и а но да или либо ни что чтобы чтоб если хотя хоть как будто словно
		тоже также зато однако причём причем притом
		не нет же ли бы б ведь вот вон уж уже ещё еще лишь только пусть пускай
		даже именно разве неужели ну
		я ты он она оно мы вы они меня тебя его её ее нас вас их мне тебе ему
		ей нам вам им мной мною тобой тобою ею ими нём нем ней нею них него
		неё нее себя себе собой собою
		мой моя моё мое мои твой твоя твоё твое твои наш наша наше наши ваш
		ваша ваше ваши свой своя своё свое свои
		этот эта это эти тот та то те такой такая такое такие
		кто что какой какая какое какие чей чья чьё чье чьи который которая
		которое которые весь вся всё все всего всех всем всеми
		быть был была было были есть будет будут буду будешь будем будете
		очень
	`,
	"uk": `
		в у на з із зі до від за по при про для без під над через між перед
		біля коло серед крізь
		і й та а але або ані ні що щоб як якщо хоча хоч мов наче ніби
		також теж проте однак зате
		не нема немає же ж би б лише тільки навіть саме хіба невже вже ще ось он
		я ти він вона воно ми ви вони мене тебе його її нас вас їх мені тобі
		йому їй нам вам їм мною тобою ним нею ними ньому ній них нього неї
		себе собі собою
		мій моя моє мої твій твоя твоє твої наш наша наше наші ваш ваша ваше
		ваші свій своя своє свої
		цей ця це ці той та те ті такий така таке такі
		хто що який яка яке які чий чия чиє чиї котрий котра котре котрі
		весь вся все всі
		бути був була було були є буде будуть буду будеш будемо будете
		дуже
	`,
	"es": `
		el la los las un una unos unas lo
		de del a al en con por para sin sobre entre hacia hasta desde tras
		y e o u ni que si aunque porque cuando donde como cual cuales quien
		quienes cuyo cuya cuyos cuyas pero sino
		no me te se le les nos os mi mis tu tus su sus
		yo tú él ella ello usted nosotros nosotras vosotros vosotras ellos
		ellas ustedes mí ti sí conmigo contigo
		mío mía míos mías tuyo tuya tuyos tuyas suyo suya suyos suyas nuestro
		nuestra nuestros nuestras vuestro vuestra vuestros vuestras
		este esta esto estos estas ese esa eso esos esas aquel aquella aquello
		aquellos aquellas
		es son era eran fue fueron ser sea sean siendo sido soy eres somos sois
		está están estaba estaban estar estoy estás estamos estáis estado
		he ha han has hemos habéis hay había habían hubo haber habiendo
		ya también muy más aún todavía
	`,
	"de": `
		der die das den dem des ein eine einen einem einer eines
		und oder aber sondern denn doch dass daß ob wenn als weil obwohl
		während bevor nachdem damit
		nicht kein keine keinen keinem keiner keines nein
		zu in an auf mit von bei nach aus für über unter vor hinter neben
		zwischen durch gegen ohne um bis seit ab
		ich du er sie es wir ihr mich dich sich uns euch mir dir ihm ihn ihnen
		mein meine meinen meinem meiner meines dein deine sein seine seinen
		seinem seiner seines unser unsere euer eure ihre ihren ihrem ihrer ihres
		dies diese dieser dieses diesen diesem jene jener jenes welche welcher
		welches wer wen wem wessen was
		ist sind war waren bin bist seid sein gewesen werde wird werden wurde
		wurden worden hat haben habe hast hatte hatten hätte hätten
		kann können konnte könnte muss müssen musste soll sollen sollte will
		wollen wollte mag mögen darf dürfen
		ja auch noch schon nur so wie da man sehr
	`,
	"fr": `
		le la les un une des du de au aux
		à en dans sur sous avec sans pour par entre vers chez depuis pendant
		contre malgré parmi
		et ou mais ni car que qui quoi dont où si quand comme parce puisque
		lorsque quoique
		ne pas non plus point
		je tu il elle on nous vous ils elles me te se lui leur y moi toi soi eux
		mon ma mes ton ta tes son sa ses notre nos votre vos leurs
		ce cet cette ces cela ceci ça celui celle ceux celles
		est sont était étaient être suis es êtes sommes fut furent sera seront
		serait seraient été étant
		a ont ai as avons avez avait avaient eu eût ayant aura auront aurait
		déjà aussi très encore toujours
	`,
	"it": `
		il lo la i gli le un uno una
		di del dello della dei degli delle a al allo alla ai agli alle da dal
		dallo dalla dai dagli dalle in nel nello nella nei negli nelle con su
		sul sullo sulla sui sugli sulle per tra fra senza contro verso presso
		e ed o od ma né che chi cui se anche perché quando dove come sebbene
		poiché mentre
		non mi ti si ci vi li ne
		io tu lui lei noi voi loro esso essa essi esse me te sé
		mio mia miei mie tuo tua tuoi tue suo sua suoi sue nostro nostra nostri
		nostre vostro vostra vostri vostre
		questo questa questi queste quello quella quelli quelle ciò
		è sono era erano essere sia siano fu furono sarà saranno sarebbe stato
		stata stati state essendo
		ho ha hanno hai abbiamo avete aveva avevano ebbe ebbero avuto avendo
		già più molto ancora sempre
	`,
	"pt": `
		o a os as um uma uns umas
		de do da dos das em no na nos nas por pelo pela pelos pelas para com
		sem sobre entre até desde contra após sob
		e ou nem que se não como quando onde porque embora pois mas porém
		me te lhe lhes vos lho lha
		eu tu ele ela nós vós eles elas você vocês mim ti si comigo contigo
		meu minha meus minhas teu tua teus tuas seu sua seus suas nosso nossa
		nossos nossas vosso vossa vossos vossas
		este esta estes estas esse essa esses essas aquele aquela aqueles
		aquelas isto isso aquilo
		é são era eram foi foram ser seja sejam sendo sido sou és somos sois
		está estão estava estavam estar estou estás estamos estado
		há havia houve haver tem têm tinha tinham ter tendo tido
		já também muito mais ainda
	`,
	"pl": `
		i w we z ze na do od o u za po przy przez dla bez pod nad między przed
		obok wśród mimo około
		a ale lub albo ani że żeby aby bo czy gdy kiedy jeśli jeżeli choć
		chociaż jednak także też oraz lecz
		nie tylko już jeszcze nawet właśnie niech tak
		ja ty on ona ono my wy oni one mnie ciebie cię jego go jej ją nas was
		ich je mi ci mu nam wam im mną tobą nim nią nami wami nimi sobie się
		siebie sobą
		mój moja moje moi twój twoja twoje twoi nasz nasza nasze nasi wasz
		wasza wasze wasi swój swoja swoje swoi
		ten ta to te tamten tamta tamto który która które którzy kto co jaki
		jaka jakie taki taka takie wszystko wszyscy każdy
		jest są był była było byli będzie będą być jestem jesteś jesteśmy
		jesteście będę będziesz
		mam ma mają masz mamy macie miał miała mieli
		bardzo
	`,
	"nl": `
		de het een
		en of maar want dus noch
		niet geen nee
		in op aan met van bij naar uit voor over onder tussen door tegen
		zonder om tot sinds achter naast
		dat die deze dit wie wat welke als toen omdat terwijl hoewel wanneer
		ik jij je hij zij ze wij we jullie u mij me jou hem haar ons hun het
		zich zichzelf
		mijn jouw zijn haar onze uw hun
		is zijn was waren ben bent geweest wordt worden werd werden word
		heeft hebben heb hebt had hadden gehad hebbende
		zal zullen zou zouden kan kunnen kon konden moet moeten mocht mogen
		wil willen wilde
		ook nog al zo wel er hier daar men heel erg
	`,
	"tr": `
		bir bu şu o ve ile veya ya da de ki ne eğer ama fakat ancak çünkü
		hem yani ise
		değil mi mı mu mü
		ben sen biz siz onlar bana sana ona bize size onlara beni seni onu
		bizi sizi onları bende sende onda bizde sizde onlarda benden senden
		ondan bizden sizden onlardan benim senin onun bizim sizin onların
		kendi kendine kendini
		için gibi kadar göre sonra önce üzere karşı doğru rağmen beri dolayı
		var yok idi imiş olan olarak oldu olur olacak olduğu
		çok daha en pek hiç
	`,
}

// sets is lists compiled into lookup sets once at init.
var sets = func() map[string]map[string]bool {
	m := make(map[string]map[string]bool, len(lists))
	for lang, words := range lists {
		set := map[string]bool{}
		for w := range strings.FieldsSeq(words) {
			set[w] = true
		}
		m[lang] = set
	}
	return m
}()

// Set returns the stopword set for a language code (lowercase words), or nil
// when no list ships for it — callers then fall back to counting every word.
func Set(lang string) map[string]bool {
	return sets[strings.ToLower(strings.TrimSpace(lang))]
}
