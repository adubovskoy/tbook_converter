package tbook

import (
	"encoding/json"
	"reflect"

	"github.com/dimando/reader/converter/internal/jsonx"
)

// This file implements manifest.meta — spec §3.4. The object is provenance
// ABOUT the file (who produced it, with which model, prompt, and settings),
// never content: no consumer needs it to render, and nothing else in the
// archive may depend on it.
//
// Two spec rules shape the types below:
//
//   - Unknown keys are legal by construction, so Meta round-trips the keys this
//     producer does not understand (Meta.Extra) instead of dropping them — a
//     re-assembly, e.g. adding a target language, must not silently discard
//     another tool's bookkeeping.
//   - Nothing here may carry credentials or machine identity. Callers build
//     RunRecord.Options from a curated allowlist of settings; paths, hosts, and
//     keys stay out.

// MetaRunsMax caps the run history: the oldest records are dropped so meta
// stays within the few kilobytes §3.4 asks for, however many times a book is
// re-assembled (a resumable conversion writes on every run).
const MetaRunsMax = 20

// Producer identifies the tool that wrote a run's output.
type Producer struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Commit  string `json:"commit,omitempty"`
	URL     string `json:"url,omitempty"`
}

// RunRecord is one producer run that contributed to the file. Models and
// Prompts are keyed by pass name ("translate", "align", "repair", "judge",
// "escalate", "embed"); Options records the settings the run was invoked with.
// Targets lists only the languages this run produced or updated — a run that
// adds one language lists that one language.
type RunRecord struct {
	At       string            `json:"at,omitempty"`
	Producer *Producer         `json:"producer,omitempty"`
	Targets  []string          `json:"targets,omitempty"`
	Provider string            `json:"provider,omitempty"`
	Models   map[string]string `json:"models,omitempty"`
	Prompts  map[string]string `json:"prompts,omitempty"`
	Options  map[string]any    `json:"options,omitempty"`
}

// Meta is manifest.meta. CreatedAt stamps the first assembly, UpdatedAt the
// most recent one (absent until the file is rewritten). Extra holds top-level
// keys this producer does not know, preserved verbatim across a rewrite.
type Meta struct {
	CreatedAt string
	UpdatedAt string
	Runs      []RunRecord
	Extra     map[string]json.RawMessage
}

// metaFields is the registered-key view of Meta, split out so Meta's own
// (Un)MarshalJSON can encode/decode it without recursing.
type metaFields struct {
	CreatedAt string      `json:"createdAt,omitempty"`
	UpdatedAt string      `json:"updatedAt,omitempty"`
	Runs      []RunRecord `json:"runs,omitempty"`
}

// MarshalJSON emits the registered keys plus any preserved unknown ones. A
// registered key wins over an Extra entry of the same name, so a stale
// carried-over value can never shadow what this run recorded.
func (m Meta) MarshalJSON() ([]byte, error) {
	known, err := jsonx.Marshal(metaFields{CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, Runs: m.Runs})
	if err != nil {
		return nil, err
	}
	if len(m.Extra) == 0 {
		return known, nil
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(known, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]json.RawMessage{}
	}
	for k, v := range m.Extra {
		if _, taken := out[k]; !taken {
			out[k] = v
		}
	}
	return jsonx.Marshal(out) // map keys marshal sorted — deterministic output
}

// UnmarshalJSON decodes the registered keys and keeps every other key in Extra.
func (m *Meta) UnmarshalJSON(b []byte) error {
	var f metaFields
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*m = Meta{CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt, Runs: f.Runs}
	for _, k := range []string{"createdAt", "updatedAt", "runs"} {
		delete(raw, k)
	}
	if len(raw) > 0 {
		m.Extra = raw
	}
	return nil
}

// AppendRun records r in the run history and stamps the timestamps from r.At:
// the first assembly sets CreatedAt, every later one sets UpdatedAt. A record
// identical to the previous one but for its timestamp is collapsed (§3.4 allows
// it) so repeated re-assemblies of the same book with the same settings — the
// normal resume path — leave one record, not one per run.
func (m *Meta) AppendRun(r RunRecord) {
	if m.CreatedAt == "" {
		m.CreatedAt = r.At
	} else {
		m.UpdatedAt = r.At
	}
	if n := len(m.Runs); n > 0 {
		prev, cur := m.Runs[n-1], r
		prev.At, cur.At = "", ""
		if reflect.DeepEqual(prev, cur) {
			return
		}
	}
	m.Runs = append(m.Runs, r)
	if len(m.Runs) > MetaRunsMax {
		m.Runs = m.Runs[len(m.Runs)-MetaRunsMax:]
	}
}
