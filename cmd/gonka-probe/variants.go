package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dimando/reader/converter/internal/jsonx"
	"github.com/dimando/reader/converter/internal/translate"
)

// The reasoning-off variant matrix.
//
// Gonka's own switch ("thinking":{"type":"disabled"}) is ignored by both served
// models, so this sweep tries the other request-body shapes OpenAI-compatible
// gateways accept for these model families, one field at a time, and reads what
// the production client throws away: the `reasoning`/`reasoning_content`
// message fields, the reasoning-token breakdown in `usage`, and the raw
// content. That is why these requests go over plain HTTP instead of
// translate.Client — everything else (endpoint, auth, prompt, sampling, JSON
// mode, cache-busting user tag) is kept identical to production.

// variantResult is one (model, variant) request.
type variantResult struct {
	Model        string `json:"model"`
	Variant      string `json:"variant"`
	Fields       string `json:"fields"` // the variant's own body fields, as sent
	Sentences    int    `json:"sentences"`
	OK           bool   `json:"ok"`
	Status       int    `json:"status,omitempty"`
	LatencyMS    int64  `json:"latency_ms"`
	PromptTok    int    `json:"prompt_tokens,omitempty"`
	OutputTok    int    `json:"completion_tokens,omitempty"`
	ReasoningTok int    `json:"reasoning_tokens,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
	ThinkTag     bool   `json:"think_tag"`          // <think> inside message.content
	ReasoningOut bool   `json:"reasoning_field"`    // message.reasoning / reasoning_content non-empty
	ReasoningLen int    `json:"reasoning_chars"`    // its length
	ContentLen   int    `json:"content_chars"`      //
	Parsed       int    `json:"parsed_sentences"`   // ids recovered from the content
	Rejected     bool   `json:"rejected,omitempty"` // 4xx naming the variant field
	Err          string `json:"err,omitempty"`
}

// ReasoningOff reports whether this variant actually silenced the reasoning:
// no <think> block, no reasoning field, no reasoning tokens — and the batch
// still came back parseable.
func (v variantResult) ReasoningOff() bool {
	return v.OK && v.Parsed > 0 && !v.ThinkTag && !v.ReasoningOut && v.ReasoningTok == 0
}

// bodyVariant names one request-body shape and the fields it adds to the
// production body. tokens is the sentence count, for the budget variant.
type bodyVariant struct {
	Name  string
	Apply func(body map[string]any, sentences int)
}

// bodyVariants is the matrix, in the order the report lists it. "control" is
// the production body with no reasoning field at all — the yardstick every
// other row is read against.
var bodyVariants = []bodyVariant{
	{"control (no field)", func(map[string]any, int) {}},
	{`thinking:{"type":"disabled"}`, func(b map[string]any, _ int) {
		b["thinking"] = map[string]any{"type": "disabled"}
	}},
	{"enable_thinking:false", func(b map[string]any, _ int) {
		b["enable_thinking"] = false
	}},
	{`chat_template_kwargs:{"enable_thinking":false}`, func(b map[string]any, _ int) {
		b["chat_template_kwargs"] = map[string]any{"enable_thinking": false}
	}},
	{`reasoning:{"enabled":false}`, func(b map[string]any, _ int) {
		b["reasoning"] = map[string]any{"enabled": false}
	}},
	{`reasoning:{"effort":"none"}`, func(b map[string]any, _ int) {
		b["reasoning"] = map[string]any{"effort": "none"}
	}},
	{`reasoning_effort:"none"`, func(b map[string]any, _ int) {
		b["reasoning_effort"] = "none"
	}},
	{"max_completion_tokens:40/sentence", func(b map[string]any, n int) {
		b["max_completion_tokens"] = 40 * n
	}},
}

// variantSweep sends one batch per (model, variant) and returns the rows.
// match, when non-empty, keeps only the variants whose name contains it — for
// re-running a single cell without paying for the whole matrix again.
func (p *prober) variantSweep(ctx context.Context, models []string, nSent int, match string) []variantResult {
	var out []variantResult
	for _, model := range models {
		for i, v := range bodyVariants {
			if match != "" && !strings.Contains(strings.ToLower(v.Name), strings.ToLower(match)) {
				continue
			}
			r := p.runVariant(ctx, model, i, v, p.sentences[:min(nSent, len(p.sentences))])
			out = append(out, r)
			fmt.Printf("  %-44s %s\n", v.Name, fmtVariant(r))
		}
	}
	return out
}

// runVariant builds the production translate body, applies one variant's
// fields, and reads the full reply. idx only feeds the cache-busting user tag:
// it must be unique per variant, and variant names are not (punctuation-only
// differences collapse), which would make one row the gateway's cached copy of
// another.
func (p *prober) runVariant(ctx context.Context, model string, idx int, v bodyVariant, sents []string) variantResult {
	sys := translate.TranslateSystemPrompt(translate.LangName(p.source), translate.LangName(p.target), nil)
	inputs := make([]translateInput, len(sents))
	for i, s := range sents {
		inputs[i] = translateInput{ID: strconv.Itoa(i + 1), Src: s}
	}
	userJSON, _ := jsonx.Marshal(inputs)

	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": string(userJSON)},
		},
		"temperature":     p.temperature,
		"response_format": map[string]any{"type": "json_object"},
		"user":            fmt.Sprintf("probe-%s-%s-v%02d-%s", p.run, shortID(model), idx, slug(v.Name)),
	}
	base := len(body)
	v.Apply(body, len(sents))
	res := variantResult{Model: model, Variant: v.Name, Sentences: len(sents), Fields: variantFields(body, base)}

	payload, err := json.Marshal(body)
	if err != nil {
		res.Err = clip(err.Error(), 200)
		return res
	}
	reqCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		res.Err = clip(err.Error(), 200)
		return res
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := (&http.Client{Timeout: p.timeout}).Do(req)
	res.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		res.Err = clip(err.Error(), 200)
		return res
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	res.Status = resp.StatusCode

	var cr variantResponse
	_ = json.Unmarshal(raw, &cr)
	if resp.StatusCode/100 != 2 {
		msg := strings.TrimSpace(string(raw))
		if cr.Error != nil && cr.Error.Message != "" {
			msg = cr.Error.Message
		}
		res.Err = clip(msg, 300)
		res.Rejected = rejectsField(msg, body, base)
		return res
	}
	if cr.Usage != nil {
		res.PromptTok, res.OutputTok = cr.Usage.PromptTokens, cr.Usage.CompletionTokens
		if cr.Usage.Details != nil {
			res.ReasoningTok = cr.Usage.Details.ReasoningTokens
		}
	}
	if len(cr.Choices) == 0 {
		res.Err = "no choices in response"
		return res
	}
	ch := cr.Choices[0]
	res.FinishReason = ch.FinishReason
	reasoning := ch.Message.Reasoning
	if reasoning == "" {
		reasoning = ch.Message.ReasoningContent
	}
	res.ReasoningOut = strings.TrimSpace(reasoning) != ""
	res.ReasoningLen = len(reasoning)
	res.ContentLen = len(ch.Message.Content)
	res.ThinkTag = hasThink(ch.Message.Content)
	res.OK = true
	res.Parsed = countParsed(ch.Message.Content)
	if ch.FinishReason == "length" {
		res.Err = "truncated (finish_reason=length)"
	}
	return res
}

// variantResponse is the reply with the reasoning fields the production
// chatResponse drops.
type variantResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			Reasoning        string `json:"reasoning"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		Details          *struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

var thinkBlockRE = regexp.MustCompile(`(?s)<think>.*?</think>`)

// countParsed reports how many id→translation pairs the reply yields, so a
// variant that silences reasoning by breaking the output is not mistaken for a
// win.
func countParsed(content string) int {
	s := thinkBlockRE.ReplaceAllString(content, "")
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i > 0 {
		s = s[i:]
	}
	if j := strings.LastIndex(s, "}"); j >= 0 {
		s = s[:j+1]
	}
	var m map[string]string
	if json.Unmarshal([]byte(s), &m) != nil {
		return 0
	}
	n := 0
	for _, v := range m {
		if strings.TrimSpace(v) != "" {
			n++
		}
	}
	return n
}

// variantFields renders the fields the variant added, for the artifact and the
// report table. The body carries no credential — the key travels in the
// Authorization header only.
func variantFields(body map[string]any, base int) string {
	if len(body) == base {
		return "—"
	}
	var parts []string
	for k, v := range body {
		if baseField(k) {
			continue
		}
		b, _ := json.Marshal(v)
		parts = append(parts, fmt.Sprintf("%s=%s", k, b))
	}
	return strings.Join(parts, " ")
}

func baseField(k string) bool {
	switch k {
	case "model", "messages", "temperature", "response_format", "user":
		return true
	}
	return false
}

// rejectsField reports whether a 4xx names one of the variant's own fields —
// "unknown field reasoning_effort" is a different answer from "the model
// ignored it".
func rejectsField(msg string, body map[string]any, base int) bool {
	low := strings.ToLower(msg)
	for k := range body {
		if baseField(k) {
			continue
		}
		if strings.Contains(low, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func slug(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		}
		return -1
	}, s)
}

func fmtVariant(r variantResult) string {
	if !r.OK {
		return fmt.Sprintf("FAIL status=%d %s", r.Status, clip(r.Err, 90))
	}
	return fmt.Sprintf("%5d tok (%d reasoning) %6.1fs think=%v reasoning_field=%v(%d ch) parsed=%d/%d finish=%s",
		r.OutputTok, r.ReasoningTok, float64(r.LatencyMS)/1000, r.ThinkTag,
		r.ReasoningOut, r.ReasoningLen, r.Parsed, r.Sentences, r.FinishReason)
}

func renderVariantTable(rows []variantResult) string {
	var sb strings.Builder
	sb.WriteString("\n| model | variant | status | out tok | reasoning tok | latency s | `<think>` | reasoning field | parsed | reasoning off? |\n")
	sb.WriteString("|---|---|--:|--:|--:|--:|:-:|:-:|--:|:-:|\n")
	for _, r := range rows {
		status := strconv.Itoa(r.Status)
		if r.Status == 0 {
			status = "—"
		}
		off := "no"
		if r.ReasoningOff() {
			off = "**yes**"
		}
		if !r.OK {
			off = "n/a"
		}
		fmt.Fprintf(&sb, "| `%s` | `%s` | %s | %d | %d | %.1f | %s | %s | %d/%d | %s |\n",
			r.Model, r.Variant, status, r.OutputTok, r.ReasoningTok, float64(r.LatencyMS)/1000,
			yn(r.ThinkTag), yn(r.ReasoningOut), r.Parsed, r.Sentences, off)
	}
	return sb.String()
}

func yn(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
