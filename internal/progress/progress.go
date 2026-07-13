// Package progress appends machine-readable NDJSON progress events (one JSON
// object per line, flushed per write) for an external supervisor to tail —
// enabled by --progress-file. The human progress bars are unaffected.
//
// Event stream, per phase (translate, embalign, align, judge, assemble):
//
//	{"ts":"2026-07-12T12:00:00Z","phase":"translate","target":"ru","done":128,"total":9391}
//	{"ts":"…","phase":"assemble","done":1,"total":1}
//	{"ts":"…","phase":"done","ok":true}
//
// Updates are throttled to at most ~2 lines/sec per phase+target; the final
// line of a phase is always written. A nil *Sink discards everything, so
// callers never need to check whether the flag is set.
package progress

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// minInterval is the per-phase throttle: at most one line per interval
// (~2 lines/sec), except final lines, which are always written.
const minInterval = 500 * time.Millisecond

// Sink writes the NDJSON event stream. Safe for concurrent use.
type Sink struct {
	mu   sync.Mutex
	w    io.Writer
	c    io.Closer // nil for a plain writer (tests)
	now  func() time.Time
	last map[string]entry // "phase|target" → last written line
}

// entry remembers the last written line per phase+target, for throttling and
// duplicate-final suppression.
type entry struct {
	t           time.Time
	done, total int
}

// Open opens (appending) the NDJSON sink at path.
func Open(path string) (*Sink, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	s := New(f)
	s.c = f
	return s, nil
}

// New wraps a writer (used by tests; Open is the production constructor).
func New(w io.Writer) *Sink {
	return &Sink{w: w, now: time.Now, last: map[string]entry{}}
}

// updateEvent is one phase-progress line.
type updateEvent struct {
	TS     string `json:"ts"`
	Phase  string `json:"phase"`
	Target string `json:"target,omitempty"`
	Done   int    `json:"done"`
	Total  int    `json:"total"`
}

// doneEvent is the terminal line of a run.
type doneEvent struct {
	TS    string `json:"ts"`
	Phase string `json:"phase"` // always "done"
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Update reports phase progress (throttled). A line with done ≥ total counts
// as the phase's natural final line and bypasses the throttle; use Final at
// the end of a phase that may finish short (retries exhausted, aligner died)
// to guarantee the last state is written regardless.
func (s *Sink) Update(phase, target string, done, total int) {
	s.write(phase, target, done, total, total > 0 && done >= total)
}

// Final writes the phase's closing line, bypassing the throttle. A line
// identical to the last one written for the phase is suppressed, so a phase
// that already emitted its natural final via Update produces no duplicate.
func (s *Sink) Final(phase, target string, done, total int) {
	s.write(phase, target, done, total, true)
}

func (s *Sink) write(phase, target string, done, total int, force bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := phase + "|" + target
	e, seen := s.last[key]
	if seen && e.done == done && e.total == total {
		return // duplicate of the last written line (e.g. Final after a natural final)
	}
	now := s.now()
	if seen && !force && now.Sub(e.t) < minInterval {
		return
	}
	s.encode(updateEvent{TS: ts(now), Phase: phase, Target: target, Done: done, Total: total})
	s.last[key] = entry{t: now, done: done, total: total}
}

// Done writes the terminal event: {"phase":"done","ok":true} on success,
// {"phase":"done","ok":false,"error":"…"} for a failed run. Never throttled.
func (s *Sink) Done(runErr error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ev := doneEvent{TS: ts(s.now()), Phase: "done", OK: runErr == nil}
	if runErr != nil {
		ev.Error = runErr.Error()
	}
	s.encode(ev)
}

// encode writes one line; errors are dropped (progress is best-effort and
// must never fail the conversion). Callers hold s.mu.
func (s *Sink) encode(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	_, _ = s.w.Write(append(b, '\n'))
}

// Close closes the underlying file, if any.
func (s *Sink) Close() error {
	if s == nil || s.c == nil {
		return nil
	}
	return s.c.Close()
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }
