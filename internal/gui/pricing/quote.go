package pricing

import (
	"fmt"
	"math"
)

// GonkaTokenPrice is gonka's flat price in USD per token (~$0.000117 per 1M,
// measured on proxy.gonka.gg). Gonka publishes no price endpoint, so the
// figure is pinned (copied from converter_web backend/internal/llm).
const GonkaTokenPrice = 0.000117 / 1e6

// QuoteResult is the display estimate for one target language.
type QuoteResult struct {
	USD     float64 `json:"usd"`     // raw model estimate; 0 for claude/local
	Kind    string  `json:"kind"`    // "usd" | "included" (claude) | "free" (local)
	Display string  `json:"display"` // "≈ $1.28" | "Included in your plan" | "Free"
}

// Quote computes the display estimate for one target language.
// provider: openrouter|gonka|claude|ollama|llamacpp (anything else is quoted
// as metered). repairOn is the effective proofread state; gonka ignores it —
// its auto-repair is already priced in.
func Quote(chars, sentences int, src, tgt, provider string, promptUSD, completionUSD float64, repairOn bool) QuoteResult {
	switch provider {
	case "claude":
		return QuoteResult{Kind: "included", Display: "Included in your plan"}
	case "ollama", "llamacpp":
		return QuoteResult{Kind: "free", Display: "Free"}
	case "gonka":
		// Flat token price both sides; ×2 covers the always-on repair pass.
		usd := 2 * RawCost(int64(chars), sentences, src, tgt, GonkaTokenPrice, GonkaTokenPrice)
		return QuoteResult{USD: usd, Kind: "usd", Display: display(usd)}
	}
	usd := RawCost(int64(chars), sentences, src, tgt, promptUSD, completionUSD)
	if repairOn {
		usd += repairCost(int64(chars), sentences, src, tgt, promptUSD, completionUSD)
	}
	return QuoteResult{USD: usd, Kind: "usd", Display: display(usd)}
}

// Display renders a summed (multi-target) quote the way Quote renders a
// single one.
func Display(usd float64, kind string) string {
	switch kind {
	case "included":
		return "Included in your plan"
	case "free":
		return "Free"
	}
	return display(usd)
}

// display renders "≈ $X.YZ": the raw estimate plus a 15% buffer, rounded up
// to whole cents (epsilon guards float noise), floored at one cent so a
// near-free gonka quote never shows $0.00.
func display(usd float64) string {
	v := math.Ceil(usd*1.15*100-1e-9) / 100
	if v < 0.01 {
		v = 0.01
	}
	return fmt.Sprintf("≈ $%.2f", v)
}
