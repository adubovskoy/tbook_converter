// Package jobs is the GUI's conversion queue: persistent job records and a
// FIFO worker that runs one conversion at a time. Credentials are never
// stored in a job — the manager asks a resolver for the full ConvertSpec at
// launch time, so jobs.json stays key-free.
package jobs

import (
	"time"

	"github.com/dimando/reader/converter/internal/gui/runner"
)

type Status string

const (
	StatusQueued      Status = "queued"
	StatusRunning     Status = "running"
	StatusDone        Status = "done"
	StatusFailed      Status = "failed"
	StatusCanceled    Status = "canceled"
	StatusInterrupted Status = "interrupted" // app died while the job ran
)

// Job is one conversion, persisted across restarts. Retry reuses the record
// (same ID, next attempt) so its cache identity — provider, model, repair
// settings — stays exactly what makes resume-from-cache work.
type Job struct {
	ID         string     `json:"id"`
	CreatedAt  time.Time  `json:"createdAt"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`

	InputPath  string   `json:"inputPath"`
	OutputPath string   `json:"outputPath"`
	Source     string   `json:"source"`
	Targets    []string `json:"targets"`

	Provider string `json:"provider"`
	Model    string `json:"model,omitempty"`

	LimitChapters int   `json:"limitChapters,omitempty"`
	Repair        *bool `json:"repair,omitempty"`
	RepairContext int   `json:"repairContext,omitempty"`
	Judge         bool  `json:"judge,omitempty"`
	Force         bool  `json:"force,omitempty"`

	Status   Status  `json:"status"`
	Error    string  `json:"error,omitempty"`
	CostUSD  float64 `json:"costUSD"`
	Attempts int     `json:"attempts"`

	Estimate *runner.Estimate `json:"estimate,omitempty"`
	LogPath  string           `json:"logPath,omitempty"`
}

func (j *Job) active() bool {
	return j.Status == StatusQueued || j.Status == StatusRunning
}
