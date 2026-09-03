// Package pricing estimates the LLM cost of a conversion for the GUI. The
// token model and the OpenRouter price lookup are ported from converter_web
// (backend/internal/pricing), where the model was calibrated against real
// historical conversion runs.
package pricing

import "math"

// Pinned fallback prices (USD per token) used when OpenRouter is unreachable
// and no earlier fetch succeeded. They match google/gemini-3.1-flash-lite.
const (
	DefaultPromptPrice     = 0.00000025
	DefaultCompletionPrice = 0.0000015
)

var latinLangs = map[string]bool{
	"en": true, "de": true, "fr": true, "es": true, "it": true, "pt": true,
	"pl": true, "nl": true, "tr": true, "cs": true, "sk": true, "sl": true,
	"hr": true, "ro": true, "hu": true, "sv": true, "da": true, "no": true,
	"fi": true, "et": true, "lv": true, "lt": true, "sq": true, "ca": true,
	"gl": true, "eu": true, "id": true, "ms": true, "vi": true, "af": true,
}

var cyrillicLangs = map[string]bool{
	"ru": true, "uk": true, "be": true, "bg": true, "sr": true, "mk": true,
	"kk": true, "ky": true, "mn": true,
}

// charsPerToken approximates how many source characters one LLM token covers.
func charsPerToken(lang string) float64 {
	switch {
	case latinLangs[lang]:
		return 4.0
	case cyrillicLangs[lang]:
		return 3.2
	case lang == "el":
		return 3.0
	case lang == "zh" || lang == "ja":
		return 1.8
	case lang == "ko":
		return 2.5
	case lang == "ar" || lang == "he":
		return 3.5
	default:
		return 3.5
	}
}

// pairExpand holds explicit source→target character expansion overrides.
var pairExpand = map[[2]string]float64{
	{"en", "ru"}: 1.05,
}

// expandRatio estimates the target/source character ratio for a language pair.
func expandRatio(src, tgt string) float64 {
	if r, ok := pairExpand[[2]string{src, tgt}]; ok {
		return r
	}
	switch tgt {
	case "zh":
		return 0.45
	case "ja":
		return 0.55
	case "ko":
		return 0.75
	}
	if src == "de" && latinLangs[tgt] {
		return 0.95
	}
	if latinLangs[src] && tgt == "de" {
		return 1.15
	}
	return 1.10
}

// The converter's proofread pass (phase "repair") is deliberately NOT priced
// here: converter_web runs it only where tokens are effectively free (gonka).
// Enable repair on a metered provider and this model must grow a term of
// roughly one more translate pass — that is what repairCost adds for the GUI.
//
// RawCost returns the estimated raw LLM cost in USD for converting a book of
// `chars` source characters / `sentences` sentences from src to tgt, given
// prompt/completion prices in USD per token.
func RawCost(chars int64, sentences int, src, tgt string, promptPrice, completionPrice float64) float64 {
	if chars <= 0 || sentences <= 0 {
		return 0
	}
	c := float64(chars)
	s := float64(sentences)

	srcTok := c / charsPerToken(src)
	trTok := c * expandRatio(src, tgt) / charsPerToken(tgt)
	perSentSrc := srcTok / s
	perSentTr := trTok / s

	// Translate pass: batches of 16 sentences, ~550 prompt-overhead tokens each.
	inTok := math.Ceil(s/16)*550 + srcTok*1.08
	outTok := trTok * 1.06

	// Align pass: ~7% of sentences go through the LLM aligner in batches of 4.
	n := 0.07 * s
	inTok += math.Ceil(n/4)*750 + n*(2*perSentSrc+perSentTr)
	outTok += n * perSentTr * 8

	// Judge buffer: ~6% of sentences, batches of 8.
	n = 0.06 * s
	inTok += math.Ceil(n/8)*400 + n*(perSentSrc+2*perSentTr)
	outTok += n * 8

	// Glossary pass.
	inTok += 5000
	outTok += 500

	return inTok*promptPrice + outTok*completionPrice
}

// repairCost approximates the proofread pass on a metered provider as one
// more translate pass (the approximation converter_web's model comment calls
// for). Repair rereads and rewrites roughly what translate did; the context
// sentences it also reads are absorbed by the display buffer.
func repairCost(chars int64, sentences int, src, tgt string, promptPrice, completionPrice float64) float64 {
	if chars <= 0 || sentences <= 0 {
		return 0
	}
	c := float64(chars)
	s := float64(sentences)
	srcTok := c / charsPerToken(src)
	trTok := c * expandRatio(src, tgt) / charsPerToken(tgt)
	inTok := math.Ceil(s/16)*550 + srcTok*1.08
	outTok := trTok * 1.06
	return inTok*promptPrice + outTok*completionPrice
}
