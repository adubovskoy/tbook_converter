package translate

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Stats appends one JSONL record per LLM request attempt (including retries
// and parse failures) for offline latency/cost analysis. Enabled by --stats.
// Safe for concurrent use; a nil *Stats discards everything.
type Stats struct {
	mu sync.Mutex
	f  *os.File
	enc *json.Encoder
}

// OpenStats opens (appending) the JSONL sink at path.
func OpenStats(path string) (*Stats, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Stats{f: f, enc: json.NewEncoder(f)}, nil
}

// statRec is one request attempt. Status/Provider/tokens come from the HTTP
// response when available; Err carries the transport/API/parse failure.
type statRec struct {
	TS           string  `json:"ts"`
	Model        string  `json:"model"`
	Phase        string  `json:"phase,omitempty"`
	Attempt      int     `json:"attempt"`
	LatencyMS    int64   `json:"latency_ms"`
	Status       int     `json:"status,omitempty"`
	Provider     string  `json:"provider,omitempty"`
	PromptTok    int     `json:"prompt_tokens,omitempty"`
	OutputTok    int     `json:"completion_tokens,omitempty"`
	Cost         float64 `json:"cost,omitempty"`
	FinishReason string  `json:"finish_reason,omitempty"`
	ReqBytes     int     `json:"req_bytes,omitempty"`
	RespBytes    int     `json:"resp_bytes,omitempty"`
	Err          string  `json:"err,omitempty"`
}

func (s *Stats) log(rec *statRec) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.enc.Encode(rec)
}

// Close flushes and closes the sink.
func (s *Stats) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.Close()
}

// Phase labels stats records with the pass that issued the request
// (translate/align/judge/glossary); carried via context so the shared client
// needs no per-pass state.
type phaseCtxKey struct{}

// WithPhase returns ctx labeled with a pass name for stats records.
func WithPhase(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, phaseCtxKey{}, name)
}

func phaseOf(ctx context.Context) string {
	if v, ok := ctx.Value(phaseCtxKey{}).(string); ok {
		return v
	}
	return ""
}

func nowTS() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }
