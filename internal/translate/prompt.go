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
	base := r.Replace(`You translate {SRC} → {TGT} for a language-learning reader.

You receive a JSON array of sentences, each {id, src}.

For EACH sentence, write a faithful, natural literary {TGT} translation of src:
- Translate the meaning into fluent {TGT}; do not translate word-for-word.
- Output PURE {TGT} — never leave {SRC} words in the translation.

Reply with ONLY a single JSON object mapping each "id" (exact string) to its {TGT}
translation as a STRING. No code fences, no commentary. Translate EVERY sentence.`)
	if len(glossary) == 0 {
		return base
	}
	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString("\n\nGLOSSARY — use these ")
	sb.WriteString(targetName)
	sb.WriteString(" translations consistently wherever the term appears:\n")
	for _, e := range glossary {
		sb.WriteString("- ")
		sb.WriteString(e.Src)
		sb.WriteString(" → ")
		sb.WriteString(e.Tgt)
		sb.WriteString("\n")
	}
	return sb.String()
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
