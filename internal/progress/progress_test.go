package progress

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// newTestSink returns a sink over a buffer with a manually advanced clock.
func newTestSink() (*Sink, *bytes.Buffer, *time.Time) {
	var buf bytes.Buffer
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	s := New(&buf)
	s.now = func() time.Time { return now }
	return s, &buf, &now
}

func lines(buf *bytes.Buffer) []map[string]any {
	var out []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if ln == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			panic("bad NDJSON line: " + ln)
		}
		out = append(out, m)
	}
	return out
}

func TestNilSinkIsSafe(t *testing.T) {
	var s *Sink
	s.Update("translate", "ru", 1, 10)
	s.Final("translate", "ru", 10, 10)
	s.Done(nil)
	if err := s.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}

func TestThrottle(t *testing.T) {
	s, buf, now := newTestSink()
	s.Update("translate", "ru", 1, 100) // first line: always written
	s.Update("translate", "ru", 2, 100) // within interval: dropped
	s.Update("translate", "ru", 3, 100) // dropped
	*now = now.Add(minInterval)
	s.Update("translate", "ru", 4, 100) // interval elapsed: written
	got := lines(buf)
	if len(got) != 2 {
		t.Fatalf("want 2 lines, got %d: %s", len(got), buf.String())
	}
	if got[0]["done"].(float64) != 1 || got[1]["done"].(float64) != 4 {
		t.Fatalf("wrong lines: %s", buf.String())
	}
}

func TestThrottleIsPerPhase(t *testing.T) {
	s, buf, _ := newTestSink()
	s.Update("translate", "ru", 1, 100)
	s.Update("embalign", "ru", 1, 100)  // different phase: own throttle window
	s.Update("translate", "de", 2, 100) // different target: own throttle window
	if n := len(lines(buf)); n != 3 {
		t.Fatalf("want 3 lines, got %d: %s", n, buf.String())
	}
}

func TestNaturalFinalBypassesThrottle(t *testing.T) {
	s, buf, _ := newTestSink()
	s.Update("align", "ru", 1, 10)
	s.Update("align", "ru", 10, 10) // done == total: written despite throttle
	got := lines(buf)
	if len(got) != 2 || got[1]["done"].(float64) != 10 {
		t.Fatalf("final line missing: %s", buf.String())
	}
}

func TestFinalBypassesThrottleAndDedupes(t *testing.T) {
	s, buf, _ := newTestSink()
	s.Update("align", "ru", 1, 10)
	s.Final("align", "ru", 7, 10) // short phase (retries exhausted): still written
	s.Final("align", "ru", 7, 10) // identical repeat: suppressed
	got := lines(buf)
	if len(got) != 2 || got[1]["done"].(float64) != 7 {
		t.Fatalf("want final 7/10 exactly once: %s", buf.String())
	}
}

func TestUpdateThenFinalNoDuplicate(t *testing.T) {
	s, buf, _ := newTestSink()
	s.Update("judge", "ru", 5, 5) // natural final
	s.Final("judge", "ru", 5, 5)  // phase-end guarantee: no duplicate line
	if n := len(lines(buf)); n != 1 {
		t.Fatalf("want 1 line, got %d: %s", n, buf.String())
	}
}

func TestEventShape(t *testing.T) {
	s, buf, _ := newTestSink()
	s.Update("translate", "ru", 128, 9391)
	s.Update("assemble", "", 1, 1)
	s.Done(nil)
	got := lines(buf)
	if len(got) != 3 {
		t.Fatalf("want 3 lines: %s", buf.String())
	}
	tr := got[0]
	if tr["ts"] != "2026-07-12T12:00:00Z" || tr["phase"] != "translate" ||
		tr["target"] != "ru" || tr["done"].(float64) != 128 || tr["total"].(float64) != 9391 {
		t.Fatalf("bad translate event: %v", tr)
	}
	if _, hasTarget := got[1]["target"]; hasTarget {
		t.Fatalf("assemble event must omit empty target: %v", got[1])
	}
	if got[2]["phase"] != "done" || got[2]["ok"] != true {
		t.Fatalf("bad done event: %v", got[2])
	}
	if _, hasErr := got[2]["error"]; hasErr {
		t.Fatalf("ok done event must omit error: %v", got[2])
	}
}

func TestDoneWithError(t *testing.T) {
	s, buf, _ := newTestSink()
	s.Done(errors.New("parse epub: boom"))
	got := lines(buf)
	if len(got) != 1 || got[0]["ok"] != false || got[0]["error"] != "parse epub: boom" {
		t.Fatalf("bad failed done event: %s", buf.String())
	}
}
