package translate

import (
	"testing"
)

// The verbatim reply DeepSeek-V4-Flash-0731 returned on the gonka gateway: one
// JSON object per line instead of one object. The translations are correct and
// in Russian; the strict parse rejected the reply with
// `parse: invalid character '{' after top-level value`, extractObject spanned
// every object at once and failed too, and the whole batch of 8 was discarded
// (bench-quality/reports/gonka-models-2026-09-19.md).
const ndjsonReply = `{"1":"«Он всё ещё с вами, я полагаю?»","2":"«Дай Бог, мистер Уилсон, чтобы я был рыжеволосым."}
{"3":"Видите ли, мистер Холмс, я очень домоседливый человек, и так как моё дело само приходило ко мне, а не мне приходилось ходить по нему, я часто неделями не выходил за порог."}
{"4":"«О, всего лишь пара сотен в год, но работа лёгкая, и она не должна слишком мешать другим занятиям.»"}`

func TestParseTextsAcceptsNDJSON(t *testing.T) {
	m, err := parseTexts(ndjsonReply)
	if err != nil {
		t.Fatalf("parseTexts: %v", err)
	}
	if len(m) != 4 {
		t.Fatalf("got %d ids, want 4: %v", len(m), m)
	}
	if got := m["1"]; got != "«Он всё ещё с вами, я полагаю?»" {
		t.Errorf("id 1 = %q", got)
	}
	if got := m["4"]; got == "" {
		t.Error("id 4 lost: the last line must merge like the rest")
	}
}

// The second malformed shape the B1 run met: a complete object followed by
// commentary, 15 of 100 translate requests, reported as
// `parse: invalid character '#' after top-level value`. Three rounds of retries
// returned the same shape and 8 Russian sentences shipped untranslated.
const hashTailReply = `{"1":"«Он всё ещё с вами, я полагаю?»","2":"«Дай Бог, мистер Уилсон."}
# Note: ids 3-8 were ambiguous, so I left them out.`

func TestParseTextsAcceptsATrailingComment(t *testing.T) {
	m, err := parseTexts(hashTailReply)
	if err != nil {
		t.Fatalf("parseTexts: %v", err)
	}
	if len(m) != 2 || m["1"] == "" {
		t.Fatalf("got %v, want the two ids the model did answer", m)
	}
}

func TestMergeNDJSONIsNarrow(t *testing.T) {
	cases := []struct {
		name, in string
		ok       bool
		ids      int
	}{
		{"one object", `{"1":"a"}`, true, 1},
		{"two objects merge", "{\"1\":\"a\"}\n{\"2\":\"b\"}", true, 2},
		{"trailing prose keeps what parsed", "{\"1\":\"a\"}\n{\"2\":\"b\"}\nDone.", true, 2},
		{"prose first is not this shape", "Here:\n{\"1\":\"a\"}", false, 0},
		{"truncated reply", `{"1":"a","2":"b"`, false, 0},
		{"repeated id is ambiguous", "{\"1\":\"a\"}\n{\"1\":\"b\"}", false, 0},
		// A value the map cannot hold is junk like any other: what already
		// parsed stands, and the ids it did not carry are retried.
		{"non-string values after a good object", "{\"1\":\"a\"}\n{\"2\":[1,2]}", true, 1},
		{"non-string values first", `{"1":[1,2]}`, false, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, ok := mergeNDJSON(c.in)
			if ok != c.ok {
				t.Fatalf("mergeNDJSON(%q) ok = %v, want %v", c.in, ok, c.ok)
			}
			if ok && len(m) != c.ids {
				t.Errorf("got %d ids, want %d", len(m), c.ids)
			}
		})
	}
}

// The align reply is the same model over the same gateway and fails the same
// way: a complete object with commentary after it, or one object per line.
// Before B1b the whole batch was discarded and re-requested.
func TestParseChunksAcceptsAStream(t *testing.T) {
	cases := []struct {
		name, in string
		ids      int
	}{
		{"object then comment", `{"1":[{"tgt":"Он","en":["0:He"]}]}
# ids 2-8 omitted`, 1},
		{"one object per line", `{"1":[{"tgt":"Он","en":["0:He"]}]}
{"2":[{"tgt":"шёл","en":["1:walked"]}]}`, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := parseChunks(c.in)
			if err != nil {
				t.Fatalf("parseChunks: %v", err)
			}
			if len(m) != c.ids {
				t.Fatalf("got %d ids, want %d: %v", len(m), c.ids, m)
			}
			if len(m["1"]) != 1 || m["1"][0].Tgt != "Он" {
				t.Errorf("id 1 = %+v", m["1"])
			}
		})
	}
	// A truncated align reply is still a failure: nothing complete arrived.
	if _, err := parseChunks(`{"1":[{"tgt":"Он"`); err == nil {
		t.Error("a truncated reply parsed")
	}
}
