package main

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// reqRow is one HTTP attempt, as the translate client's own stats record saw
// it, plus the cell it belongs to. With --max-retries 0 a batch is exactly one
// attempt, so len(requests) == len(batches) inside a cell — but the two are
// not row-paired: the client logs a record from its own goroutine.
type reqRow struct {
	Model        string `json:"model"`
	Concurrency  int    `json:"concurrency"`
	Run          string `json:"run"`
	TS           string `json:"ts"`
	LatencyMS    int64  `json:"latency_ms"`
	Status       int    `json:"status,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
	PromptTok    int    `json:"prompt_tokens,omitempty"`
	OutputTok    int    `json:"completion_tokens,omitempty"`
	Err          string `json:"err,omitempty"`
	ErrClass     string `json:"err_class,omitempty"`
}

// batchRow is one probe-side batch: what the production translate call gave
// back, and what the returned target text looks like.
type batchRow struct {
	Model       string   `json:"model"`
	Concurrency int      `json:"concurrency"`
	Run         string   `json:"run"`
	Batch       int      `json:"batch"`
	Sent        int      `json:"sentences_sent"`
	Got         int      `json:"sentences_got"`
	LatencyMS   int64    `json:"latency_ms"`
	OK          bool     `json:"ok"`
	ErrClass    string   `json:"err_class,omitempty"`
	Err         string   `json:"err,omitempty"`
	CyrMajority int      `json:"cyrillic_majority"`
	LatinLeak   int      `json:"latin_leak"`
	Leaks       []string `json:"leaks,omitempty"`
}

// cellSummary is one (model, concurrency) cell of the report table.
type cellSummary struct {
	Model       string  `json:"model"`
	BatchSize   int     `json:"batch_size"`
	Concurrency int     `json:"concurrency"`
	Run         string  `json:"run"`
	WallS       float64 `json:"wall_s"`
	Requests    int     `json:"requests"`
	Canceled    int     `json:"canceled"`
	SuccessPct  float64 `json:"success_pct"`
	Pct429      float64 `json:"pct_429"`
	Pct502      float64 `json:"pct_502"`
	PctTrunc    float64 `json:"pct_truncated"`
	PctParse    float64 `json:"pct_parse"`
	PctOther    float64 `json:"pct_other"`
	P50MS       int64   `json:"p50_ms"`
	P90MS       int64   `json:"p90_ms"`
	SentAsked   int     `json:"sentences_asked"`
	SentGot     int     `json:"sentences_got"`
	SentPerSec  float64 `json:"sentences_per_s"`
	MeanOutTok  int     `json:"mean_output_tokens"`
	ThinkPct    float64 `json:"think_pct"`
	CyrPct      float64 `json:"cyrillic_majority_pct"`
	LatinPct    float64 `json:"latin_leak_pct"`
	Unservable  bool    `json:"unservable"`
}

// apiErrRE matches the client's apiError wording, "<provider> <status>: <msg>".
var apiErrRE = regexp.MustCompile(`\b(\d{3}): `)

// classifyErr buckets a failed request the way the report counts them. The
// HTTP status is read back out of the client's error text (the client wraps
// non-2xx as "<provider> <status>: <msg>") so the probe needs no second copy
// of the transport.
func classifyErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "context canceled"):
		return "canceled"
	case strings.Contains(low, "deadline exceeded"), strings.Contains(low, "timeout"),
		strings.Contains(low, "timed out"):
		return "timeout"
	case strings.Contains(low, "finish_reason=length"):
		return "truncated"
	case strings.HasPrefix(low, "parse response"), strings.Contains(low, "parse:"):
		return "parse"
	}
	if m := apiErrRE.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	if strings.Contains(low, "no such host") || strings.Contains(low, "connection refused") ||
		strings.Contains(low, "eof") || strings.Contains(low, "reset by peer") {
		return "network"
	}
	return "other"
}

// classifyRec buckets a stats record the same way, from its status and error
// text (a record carries the status directly when a response arrived).
func classifyRec(status int, finish, errText string) string {
	if errText == "" {
		return ""
	}
	low := strings.ToLower(errText)
	switch {
	case strings.Contains(low, "context canceled"):
		return "canceled"
	case finish == "length" || strings.Contains(low, "finish_reason=length"):
		return "truncated"
	case strings.HasPrefix(low, "parse:"):
		return "parse"
	case strings.Contains(low, "deadline exceeded"), strings.Contains(low, "timeout"),
		strings.Contains(low, "timed out"):
		return "timeout"
	}
	if status > 0 {
		return strconv.Itoa(status)
	}
	if strings.Contains(low, "no such host") || strings.Contains(low, "connection refused") ||
		strings.Contains(low, "eof") || strings.Contains(low, "reset by peer") {
		return "network"
	}
	return "other"
}

// cellInput is everything one cell produced. Replies/ThinkReplies are counted
// at cell level, not per batch: the client hands raw replies to a single
// callback shared by every concurrent request, and one client per cell is what
// keeps the throughput number honest (a client per batch would pay a fresh TLS
// handshake per request).
type cellInput struct {
	Model        string
	Conc         int
	Batch        int
	Run          string
	Reqs         []reqRow
	Batches      []batchRow
	Wall         time.Duration
	Unservable   bool
	Replies      int
	ThinkReplies int
}

// summarize aggregates one cell. Rates are shares of the requests that were
// actually attempted and not canceled by the early stop, so an unservable
// cell's abort does not dilute its own failure rate. Latency percentiles are
// over successful requests only — a 0.2 s "model not available" 404 is not a
// latency datum.
func summarize(in cellInput) cellSummary {
	model, conc, run, reqs, batches, wall := in.Model, in.Conc, in.Run, in.Reqs, in.Batches, in.Wall
	c := cellSummary{Model: model, BatchSize: in.Batch, Concurrency: conc, Run: run, WallS: wall.Seconds(), Unservable: in.Unservable}

	var lat []int64
	var okTok, okTokN int
	counts := map[string]int{}
	for _, r := range reqs {
		if r.ErrClass == "canceled" {
			c.Canceled++
			continue
		}
		c.Requests++
		if r.ErrClass == "" {
			lat = append(lat, r.LatencyMS)
			if r.OutputTok > 0 {
				okTok += r.OutputTok
				okTokN++
			}
			continue
		}
		counts[r.ErrClass]++
	}
	ok := c.Requests - sumOf(counts)
	if c.Requests > 0 {
		n := float64(c.Requests)
		c.SuccessPct = 100 * float64(ok) / n
		c.Pct429 = 100 * float64(counts["429"]) / n
		c.Pct502 = 100 * float64(counts["502"]) / n
		c.PctTrunc = 100 * float64(counts["truncated"]) / n
		c.PctParse = 100 * float64(counts["parse"]) / n
		other := 0
		for k, v := range counts {
			switch k {
			case "429", "502", "truncated", "parse":
			default:
				other += v
			}
		}
		c.PctOther = 100 * float64(other) / n
	}
	if okTokN > 0 {
		c.MeanOutTok = okTok / okTokN
	}
	c.P50MS, c.P90MS = percentiles(lat)

	var outputs int
	for _, b := range batches {
		if b.ErrClass == "canceled" {
			continue
		}
		c.SentAsked += b.Sent
		c.SentGot += b.Got
		if b.OK {
			outputs += b.Got
			c.CyrPct += float64(b.CyrMajority)
			c.LatinPct += float64(b.LatinLeak)
		}
	}
	if in.Replies > 0 {
		c.ThinkPct = 100 * float64(in.ThinkReplies) / float64(in.Replies)
	}
	if outputs > 0 {
		c.CyrPct = 100 * c.CyrPct / float64(outputs)
		c.LatinPct = 100 * c.LatinPct / float64(outputs)
	} else {
		c.CyrPct, c.LatinPct = 0, 0
	}
	if wall > 0 {
		c.SentPerSec = float64(c.SentGot) / wall.Seconds()
	}
	return c
}

func sumOf(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// percentiles returns the p50 and p90 of a latency sample (nearest-rank).
func percentiles(v []int64) (int64, int64) {
	if len(v) == 0 {
		return 0, 0
	}
	s := append([]int64(nil), v...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	at := func(p float64) int64 {
		i := int(p * float64(len(s)))
		if i >= len(s) {
			i = len(s) - 1
		}
		return s[i]
	}
	return at(0.50), at(0.90)
}

// cyrillicMajority reports whether a translation is mostly Cyrillic — the
// cheap test for "did this model answer in the target language at all".
func cyrillicMajority(s string) bool {
	cyr, lat := 0, 0
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Cyrillic, r):
			cyr++
		case unicode.Is(unicode.Latin, r):
			lat++
		}
	}
	return cyr > lat
}

// latinWordRE matches Latin-script words of three letters or more, diacritics
// included — an untranslated «immer» and a «Für» must both count.
var latinWordRE = regexp.MustCompile(`\p{Latin}{3,}`)

// latinLeaks returns the Latin-script words of the translation that do not
// occur in its source sentence — untranslated source, or another language
// entirely. Words shared with the source (names, quoted titles) are expected
// and not reported.
func latinLeaks(src, out string) []string {
	if !latinWordRE.MatchString(out) {
		return nil
	}
	inSrc := map[string]bool{}
	for _, w := range latinWordRE.FindAllString(src, -1) {
		inSrc[strings.ToLower(w)] = true
	}
	var leaks []string
	seen := map[string]bool{}
	for _, w := range latinWordRE.FindAllString(out, -1) {
		l := strings.ToLower(w)
		if inSrc[l] || seen[l] {
			continue
		}
		seen[l] = true
		leaks = append(leaks, w)
	}
	return leaks
}

// hasThink reports whether a raw reply carries a <think> block — the reasoning
// some models emit inside message content, which stripThink removes before the
// pipeline ever sees it.
func hasThink(content string) bool {
	return strings.Contains(content, "<think>")
}
