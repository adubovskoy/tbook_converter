package main

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestClassifyErr(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{errors.New("gonka 429: too many concurrent requests"), "429"},
		{errors.New("gonka 502: redundancy: no currently-available executor"), "502"},
		{errors.New("gonka 404: model \"moonshotai/Kimi-K2.6\" is not available"), "404"},
		{errors.New("completion truncated (finish_reason=length) — batch may exceed the cap"), "truncated"},
		{errors.New("parse response: unexpected end of JSON input"), "parse"},
		{errors.New(`Post "https://proxy.gonka.gg/v1/chat/completions": context deadline exceeded`), "timeout"},
		{errors.New(`Post "https://proxy.gonka.gg/v1/chat/completions": context canceled`), "canceled"},
		{errors.New(`Post "https://x/v1": dial tcp: connection refused`), "network"},
		{errors.New("something else entirely"), "other"},
	}
	for _, c := range cases {
		if got := classifyErr(c.err); got != c.want {
			t.Errorf("classifyErr(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestClassifyRec(t *testing.T) {
	cases := []struct {
		status int
		finish string
		err    string
		want   string
	}{
		{200, "stop", "", ""},
		{200, "length", "completion truncated (finish_reason=length)", "truncated"},
		{429, "", "gonka 429: too many concurrent requests", "429"},
		{0, "", "context deadline exceeded", "timeout"},
		{200, "", "parse: unexpected end of JSON input", "parse"},
	}
	for _, c := range cases {
		if got := classifyRec(c.status, c.finish, c.err); got != c.want {
			t.Errorf("classifyRec(%d,%q,%q) = %q, want %q", c.status, c.finish, c.err, got, c.want)
		}
	}
}

// TestSummarize checks the aggregation a report cell is read from: rates over
// attempted (not canceled) requests, percentiles over successful ones only,
// and throughput over the cell's wall time.
func TestSummarize(t *testing.T) {
	reqs := []reqRow{
		{LatencyMS: 1000, Status: 200, FinishReason: "stop", OutputTok: 300},
		{LatencyMS: 2000, Status: 200, FinishReason: "stop", OutputTok: 500},
		{LatencyMS: 9000, Status: 200, FinishReason: "stop", OutputTok: 400},
		{LatencyMS: 120, Status: 429, Err: "gonka 429: busy", ErrClass: "429"},
		{LatencyMS: 130, Status: 502, Err: "gonka 502: redundancy", ErrClass: "502"},
		{LatencyMS: 5000, Status: 200, FinishReason: "length", Err: "completion truncated", ErrClass: "truncated"},
		{LatencyMS: 140, Err: "context canceled", ErrClass: "canceled"},
	}
	batches := []batchRow{
		{Sent: 8, Got: 8, OK: true, CyrMajority: 8, LatinLeak: 1},
		{Sent: 8, Got: 8, OK: true, CyrMajority: 6, LatinLeak: 0},
		{Sent: 8, Got: 7, OK: true, CyrMajority: 7, LatinLeak: 2},
		{Sent: 8, ErrClass: "429"},
		{Sent: 8, ErrClass: "502"},
		{Sent: 8, ErrClass: "truncated"},
		{Sent: 8, ErrClass: "canceled"}, // excluded everywhere
	}
	c := summarize(cellInput{
		Model: "m", Conc: 8, Run: "r1", Reqs: reqs, Batches: batches,
		Wall: 10 * time.Second, Replies: 3, ThinkReplies: 1,
	})

	if c.Requests != 6 || c.Canceled != 1 {
		t.Fatalf("requests=%d canceled=%d, want 6/1", c.Requests, c.Canceled)
	}
	near := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 0.05 {
			t.Errorf("%s = %.2f, want %.2f", name, got, want)
		}
	}
	near("success", c.SuccessPct, 50)
	near("429", c.Pct429, 100.0/6)
	near("502", c.Pct502, 100.0/6)
	near("trunc", c.PctTrunc, 100.0/6)
	near("other", c.PctOther, 0)
	if c.P50MS != 2000 || c.P90MS != 9000 {
		t.Errorf("p50/p90 = %d/%d, want 2000/9000", c.P50MS, c.P90MS)
	}
	if c.MeanOutTok != 400 {
		t.Errorf("mean output tokens = %d, want 400", c.MeanOutTok)
	}
	if c.SentAsked != 48 || c.SentGot != 23 {
		t.Errorf("sentences = %d/%d, want 23/48", c.SentGot, c.SentAsked)
	}
	near("sent/s", c.SentPerSec, 2.3)
	near("think", c.ThinkPct, 100.0/3)
	near("cyr", c.CyrPct, 100*21.0/23)
	near("latin", c.LatinPct, 100*3.0/23)
	if c.Unservable {
		t.Error("cell marked unservable without an abort")
	}
}

func TestSummarizeEmptyCell(t *testing.T) {
	c := summarize(cellInput{Model: "m", Conc: 4, Wall: time.Second, Unservable: true})
	if c.Requests != 0 || c.SuccessPct != 0 || c.SentPerSec != 0 || !c.Unservable {
		t.Errorf("empty cell summarized wrong: %+v", c)
	}
}

func TestScriptSanity(t *testing.T) {
	if !cyrillicMajority("Утро было ярким и холодным.") {
		t.Error("Russian sentence not majority Cyrillic")
	}
	if cyrillicMajority("The morning was bright and cold.") {
		t.Error("English sentence counted as majority Cyrillic")
	}
	if !cyrillicMajority("Шерлок Холмс, лондонский сыщик, the detective.") {
		t.Error("mostly-Cyrillic sentence with a Latin tail should still pass")
	}

	src := "To Sherlock Holmes she is always the woman."
	if got := latinLeaks(src, "Для Шерлока Холмса она всегда была the woman."); len(got) != 0 {
		t.Errorf("latin words present in the source must not be flagged, got %v", got)
	}
	got := latinLeaks(src, "Für Шерлока Холмса она immer была женщиной.")
	if len(got) != 2 || got[0] != "Für" || got[1] != "immer" {
		t.Errorf("latinLeaks = %v, want [Für immer]", got)
	}
	if got := latinLeaks(src, "Для Шерлока Холмса она всегда женщина."); got != nil {
		t.Errorf("clean translation flagged: %v", got)
	}
}

func TestHasThinkAndChunk(t *testing.T) {
	if !hasThink("<think>reasoning…</think>{\"1\":\"…\"}") {
		t.Error("think block not detected")
	}
	if hasThink(`{"1":"Утро было ярким."}`) {
		t.Error("clean reply reported as carrying a think block")
	}
	got := chunk([]string{"a", "b", "c", "d", "e"}, 2)
	if len(got) != 3 || len(got[2]) != 1 {
		t.Errorf("chunk = %v, want 3 groups ending in 1", got)
	}
}
