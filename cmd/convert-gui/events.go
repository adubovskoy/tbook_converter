package main

import (
	"github.com/dimando/reader/converter/internal/gui/jobs"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Event names the frontend subscribes to. Registered with payload types so
// the bindings generator emits a typed JS API.
const (
	eventJobState     = "job:state"     // full jobs.Job on every transition
	eventJobProgress  = "job:progress"  // ProgressPayload, throttled by the converter itself
	eventJobCost      = "job:cost"      // CostPayload every ~5s while running
	eventFilesDropped = "files:dropped" // []string of absolute paths from a native drop
)

// ProgressPayload is one progress tick, with the overall percent already
// folded in Go (single source for the weight math).
type ProgressPayload struct {
	ID      string  `json:"id"`
	Phase   string  `json:"phase"`
	Target  string  `json:"target,omitempty"`
	Done    int     `json:"done"`
	Total   int     `json:"total"`
	Percent float64 `json:"percent"`
	Label   string  `json:"label"`
}

type CostPayload struct {
	ID      string  `json:"id"`
	CostUSD float64 `json:"costUSD"`
}

func init() {
	application.RegisterEvent[jobs.Job](eventJobState)
	application.RegisterEvent[ProgressPayload](eventJobProgress)
	application.RegisterEvent[CostPayload](eventJobCost)
	application.RegisterEvent[[]string](eventFilesDropped)
}
