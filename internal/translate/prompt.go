package translate

import "strings"

// LangNames maps language codes to human names for the translation prompt.
var LangNames = map[string]string{
	"ru": "Russian", "en": "English", "es": "Spanish", "fr": "French",
	"de": "German", "it": "Italian", "pt": "Portuguese", "uk": "Ukrainian",
	"pl": "Polish", "ja": "Japanese", "zh": "Chinese", "ko": "Korean",
	"tr": "Turkish", "nl": "Dutch", "ar": "Arabic",
}

// LangName returns the human name for a code, or the code itself if unknown.
func LangName(code string) string {
	if n, ok := LangNames[code]; ok {
		return n
	}
	return code
}

// translateSystemPrompt is pass 1 — translation only (no alignment). Keeping
// this separate from alignment is the core fix: asking a model to translate AND
// emit a by-meaning reverse word-alignment in one shot collapses into positional
// drift at batch scale. Translation alone is reliable even in large batches.
// A non-empty glossary is appended as enforced terminology so recurring terms
// and proper nouns stay consistent across every batch of the book.
func translateSystemPrompt(sourceName, targetName string, glossary []GlossEntry) string {
	r := strings.NewReplacer("{SRC}", sourceName, "{TGT}", targetName)
	base := r.Replace(`You translate {SRC} → {TGT} for a language-learning reader: the app shows your
translation beside the original and highlights matching word pairs, so the reader
constantly compares the two texts word by word.

You receive a JSON array of sentences, each {id, src}.

For EACH sentence, write a faithful, natural literary {TGT} translation of src:
- Each sentence STANDS ALONE: translate exactly the content of its own src — never
  borrow, merge, or shift words or meaning from a neighboring sentence in the batch.
- COMPLETE and EXACT: every meaning element of src appears in the translation — no
  dropped clause, modifier, or negation; nothing invented; numbers, names, and
  quoted speech preserved.
- Natural {TGT} comes first — but when two renderings are equally natural, prefer
  the one that MIRRORS the source: give each content word an explicit {TGT}
  counterpart, keep the source clause order, keep metaphors as images. Do not
  paraphrase freely; split or merge clauses only where {TGT} grammar demands it.
- Match the source register and tone: slang stays slang, formal stays formal.
- Output PURE {TGT} — never leave {SRC} words in the translation.

Reply with ONLY a single JSON object mapping each "id" (exact string) to its {TGT}
translation as a STRING. No code fences, no commentary. Translate EVERY sentence.`)
	return base + glossaryBlock(targetName, glossary)
}

// glossaryBlock renders the enforced-terminology section shared by the translate
// and proofread prompts. Entries carrying a gender get a [male] / [female] tag
// and the block gains one line explaining it.
//
// The tag is what tells the model a fact the sentence cannot: which gender the
// words around a character must agree with. Measured en→ru on text the model
// cannot recognise, agreement went from 45% to 98% (fixed 32, broke 1,
// p=7.9e-09; replicated on a second book: 41% → 99%, fixed 150, broke 0). It
// costs ~4 tokens per tagged entry and changes nothing else — adherence to the
// glossary is identical with and without it. On a book the model KNOWS the tag
// is redundant (a cast list identifies the novel and it recalls the genders
// itself), which is why this was measured on de-identified text. See
// bench-quality/reports/glossary-scale-and-gender.md §4.
func glossaryBlock(targetName string, glossary []GlossEntry) string {
	if len(glossary) == 0 {
		return ""
	}
	gendered := glossaryHasGender(glossary)
	var sb strings.Builder
	sb.WriteString("\n\nGLOSSARY — use these ")
	sb.WriteString(targetName)
	sb.WriteString(" translations consistently wherever the term appears:\n")
	if gendered {
		sb.WriteString("A [male] / [female] tag gives the gender of the person that term refers to. " +
			"Every " + targetName + " word that agrees with that person — past-tense verb, adjective, " +
			"participle, pronoun — must take that gender, even where the source does not mark it.\n")
	}
	for _, e := range glossary {
		sb.WriteString("- ")
		sb.WriteString(e.Src)
		sb.WriteString(" → ")
		sb.WriteString(e.Tgt)
		switch e.Gender {
		case "m":
			sb.WriteString("  [male]")
		case "f":
			sb.WriteString("  [female]")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// repairSystemPrompt is pass 1.5 — proofread an existing translation and fix
// only what is WRONG, leaving everything else byte-identical. It never
// re-translates: the align pass locates fragments inside this text, and a
// rewritten-to-taste sentence would be a second variable in every comparison.
//
// Measured on gonka/Kimi-K2.6 (issue #8): it changes ~4% of a book's sentences
// and wins 87% of the decisive blind pairwise judgements on exactly those
// sentences (p=3.4e-05), at no cost to alignment coverage. The two rules that
// make it safe rather than harmful are both here and both were learned the hard
// way: the glossary must be visible (without it the pass "corrects" the book's
// own enforced terms into literal ones — «оболочка», a body, into «рукав»), and
// context-free guessing must be forbidden (without that rule it flips a
// character's gender wherever the source is genderless).
//
// ctxN > 0 (--context) replaces that second rule with its opposite: each item
// then carries the N preceding sentences with their translations, so a guess
// becomes a lookup. Measured full-book at N=2: 672 sentences changed (4.8%)
// against 544 (3.9%) context-free, at 84.4% precision against 87.1% — a net
// +59 sentences per 14k — for three times the phase's wall time. N=1 is
// measurably WORSE than no context at all (80.0% precision): one sentence back
// gives the model the confidence to touch a character's gender without the
// information to get it right. Use 0 or 2, never 1.
func repairSystemPrompt(sourceName, targetName string, glossary []GlossEntry, ctxN int) string {
	r := strings.NewReplacer("{SRC}", sourceName, "{TGT}", targetName)
	base := r.Replace(`You proofread {TGT} translations of {SRC} sentences for a language-learning reader: the app
shows the translation beside the original and highlights matching word pairs, so every
{TGT} word must be correct {TGT} that a learner can safely imitate.

You receive a JSON array of items {id, src, tr}: src is the {SRC} original, tr its {TGT} translation.

For EACH item, return tr FIXED — but ONLY where it is genuinely wrong:
- GRAMMAR: gender/number/case agreement, verb conjugation, verb and preposition government.
  A malformed or non-existent {TGT} word is always an error.
- CALQUES: a word-by-word rendering of a {SRC} idiom, phrasal verb, slang or fixed expression
  that is not real {TGT} — replace it with the natural {TGT} expression of the same meaning
  and the same register.
- REGISTER: slang stays slang, formal stays formal.
- FIDELITY: restore a dropped meaning element of src, delete anything invented, and never
  leave {SRC} words in the translation.
- PHRASING: where tr is grammatical but reads as translated-from-{SRC} rather than written
  in {TGT} — unidiomatic collocations, clumsy clause order, a stiff literal choice where
  {TGT} has an ordinary word — rewrite that part into what a {TGT} author would write.
  Keep the same meaning, the same register, and the same sentence boundaries.
Change NOTHING else: keep tr's wording, word order and style wherever it is already correct.
Do not merge or split sentences, do not alter names, numbers or quoted speech.`)
	if ctxN > 0 {
		base += r.Replace(`
Each item also carries "prev": the sentence(s) immediately before it, with their FINISHED {TGT}
translations. They are CONTEXT ONLY — already translated, already shipped. Never translate,
quote, merge or modify prev, and never return anything for it.
Use prev to settle what this sentence alone cannot: who is being spoken about and with which
gender, what a pronoun refers to, which {TGT} wording a recurring thing already got. Where prev
settles it and tr contradicts it, FIX tr. Where neither src, tr nor prev settles it, leave tr
exactly as it is — an unsupported change there is a guess, and guessing breaks the book.`)
	} else {
		base += r.Replace(`
You see ONE sentence at a time with no surrounding context: never "correct" the gender of a
person, the referent of a pronoun, or a recurring term just because it looks unexpected —
without the context that decided it, such a change is a guess, and guessing here breaks the
book. The one exception is a person the GLOSSARY below tags with a gender: that is context,
supplied for the whole book, and tr must agree with it.`)
	}
	base += ` MOST ITEMS NEED NO CHANGE — then return tr exactly as given.`
	if len(glossary) > 0 {
		var sb strings.Builder
		sb.WriteString(base)
		sb.WriteString("\n\nGLOSSARY — these ")
		sb.WriteString(targetName)
		sb.WriteString(" renderings are enforced across the whole book. They are CORRECT by\n")
		sb.WriteString("decision, even where a different word looks more literal — never change them:\n")
		if glossaryHasGender(glossary) {
			sb.WriteString("A [male] / [female] tag gives the gender of the person that term refers to. It is\n")
			sb.WriteString("a FACT about the book, not a guess: where tr makes a verb, adjective, participle or\n")
			sb.WriteString("pronoun agree with that person in the other gender, FIX it.\n")
		}
		for _, e := range glossary {
			sb.WriteString("- ")
			sb.WriteString(e.Src)
			sb.WriteString(" → ")
			sb.WriteString(e.Tgt)
			switch e.Gender {
			case "m":
				sb.WriteString("  [male]")
			case "f":
				sb.WriteString("  [female]")
			}
			sb.WriteString("\n")
		}
		base = strings.TrimRight(sb.String(), "\n")
	}
	return base + r.Replace(`

Reply with ONLY a single JSON object mapping each "id" (exact string) to the final {TGT}
sentence as a STRING. No code fences, no commentary. Output EVERY item.`)
}

// glossaryHasGender reports whether any entry states a gender, i.e. whether the
// prompt needs the line that explains the tag.
func glossaryHasGender(glossary []GlossEntry) bool {
	for _, e := range glossary {
		if e.Gender == "m" || e.Gender == "f" {
			return true
		}
	}
	return false
}

// alignSystemPrompt is pass 2 — align only, given the finished translation. The
// model receives NUMBERED source words and echoes "index:text" tokens; the
// producer trusts an index only when its echoed text matches, else falls back
// to match-by-text (the v5 "numbered echo" contract — measurably harder for a
// cheap model to drift positionally than text-only echoing, because it must
// look the word up to number it). sourceName/targetName are human language names.
func alignSystemPrompt(sourceName, targetName string) string {
	r := strings.NewReplacer("{SRC}", sourceName, "{TGT}", targetName)
	return r.Replace(`You align {SRC} sentences to their GIVEN {TGT} translations, word by word, BY MEANING.

You receive a JSON array of items {id, src, words, tr}: src is the {SRC} sentence, words its
numbered source words ("0:The 1:deepest 2:layer …"), tr the FINISHED {TGT} translation.
Do NOT change tr.

For EACH item, break tr into chunks at WORD GRANULARITY — ONE chunk per {TGT} word:
  {"tgt":"<{TGT} word, attached punctuation>","en":["<index:word>", …]}
- Concatenating all chunk.tgt in order (normal {TGT} spacing) must reproduce tr exactly.
- "en" lists the numbered {SRC} word(s) with the SAME MEANING as this {TGT} word, copied from
  words as "index:text" (e.g. "3:includes"). Find the word by MEANING, wherever it sits —
  {SRC} and {TGT} word order OFTEN DIFFER, and a correct alignment often crosses.
- INSERTED WORDS: a {TGT} word with no {SRC} counterpart (an added pronoun, particle, or
  copula) takes "en":[]. NEVER attach an inserted word to some {SRC} word — that steals it
  and shifts every later pair (the #1 defect).
- A multi-word {SRC} unit is several entries: {"tgt":"гостиную","en":["4:living","5:room"]}.
- IDIOMS, PHRASAL VERBS, FIXED EXPRESSIONS: when a {SRC} expression is rendered by fewer
  {TGT} words (often one), map EVERY word of the expression to that {TGT} word — e.g.
  src "Piss off, John.", tr "Отвали, Джон.":
  [{"tgt":"Отвали,","en":["0:Piss","1:off"]},{"tgt":"Джон.","en":["2:John"]}].
  This includes split verb+particle pairs ("gave it up": the {TGT} verb takes both
  "gave" and "up"). Never leave a particle of a translated expression unmapped or
  attached to a neighboring word it does not translate.
- The same {SRC} word may appear in several chunks; some {SRC} words (articles, function
  words absorbed by {TGT} grammar) may appear in none.
- When a {SRC} word occurs MORE THAN ONCE, pick the occurrence from the SAME clause as the
  {TGT} word — never a duplicate from elsewhere in the sentence.

Example — src "The deepest layer includes your identity.",
words "0:The 1:deepest 2:layer 3:includes 4:your 5:identity",
tr "Самый глубокий слой включает вашу идентичность.":
  CORRECT: [{"tgt":"Самый","en":["1:deepest"]},{"tgt":"глубокий","en":["1:deepest"]},
            {"tgt":"слой","en":["2:layer"]},{"tgt":"включает","en":["3:includes"]},
            {"tgt":"вашу","en":["4:your"]},{"tgt":"идентичность.","en":["5:identity"]}]
  WRONG (positional — never do this): {"tgt":"слой","en":["3:includes"]} just because both
  are third in their sentence.

Reply with ONLY a single JSON object mapping each "id" (exact string) to its chunk array.
No code fences, no commentary. Align EVERY item.`)
}
