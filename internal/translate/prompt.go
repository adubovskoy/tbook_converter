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

// systemPrompt builds the word-level alignment instruction (the "v3"
// match-by-text contract: the model echoes source words as TEXT under "en";
// offsets are computed locally). sourceName/targetName are human language names.
func systemPrompt(sourceName, targetName string) string {
	r := strings.NewReplacer("{SRC}", sourceName, "{TGT}", targetName)
	return r.Replace(`You translate {SRC} → {TGT} for a language-learning reader, with FINE WORD-LEVEL alignment.

You receive a JSON array of sentences, each {id, src, words}, where words[i] is the [start,end)
character offset of source word i (word text = src.substring(start,end)).

For EACH sentence: write a faithful, natural literary {TGT} translation, then break it into
chunks at WORD GRANULARITY:
- ONE chunk per {TGT} word (keep attached punctuation with its word).
- Concatenating all chunk.tgt in order (normal {TGT} spacing) must reproduce the translation exactly.
- For each {TGT} word, "en" = the {SRC} source word(s) it specifically translates, BY MEANING
  (not position), copied VERBATIM from src. Usually one word; an array only for a multi-word
  source unit. Use [] for an inserted {TGT} word with no source. The same source word may appear
  in several chunks.
- Do NOT lump words together; align by meaning, even across reordering. Copy the source word TEXT
  exactly as written in src — do NOT emit indices; we locate each word for you.

Example — src "Stan went to the living room." ; "Стэн прошёл в гостиную":
  [{"tgt":"Стэн","en":"Stan"},{"tgt":"прошёл","en":"went"},{"tgt":"в","en":"to"},{"tgt":"гостиную","en":["living","room"]}]

Reply with ONLY a single JSON object mapping each "id" (exact string) to its chunk array.
No code fences, no commentary. Translate EVERY sentence.`)
}
