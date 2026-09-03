package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dimando/reader/converter/internal/gui/runner"
)

// converter is the slice of runner.Runner the manager needs; an interface so
// tests can fake the child process.
type converter interface {
	Convert(ctx context.Context, spec runner.ConvertSpec, onProgress func(runner.ProgressEvent), onCost func(float64)) (float64, error)
	LogPath(jobID, attempt string) string
}

// SpecResolver builds the full ConvertSpec — including the provider's key —
// for one attempt. Called at launch time so credentials live only in
// settings, never in jobs.json.
type SpecResolver func(j Job, attempt string) (runner.ConvertSpec, error)

// Hooks are the manager's outbound notifications (the service layer forwards
// them as Wails events). All may be nil.
type Hooks struct {
	OnState    func(j Job)
	OnProgress func(jobID string, ev runner.ProgressEvent)
	OnCost     func(jobID string, usd float64)
}

// Manager owns the persistent job list and a single worker goroutine:
// strictly one conversion at a time, FIFO over the queued jobs.
type Manager struct {
	mu      sync.Mutex
	jobs    []*Job
	store   *Store
	conv    converter
	resolve SpecResolver
	hooks   Hooks

	runningID string
	cancel    context.CancelFunc
	wake      chan struct{}
}

func NewManager(store *Store, conv converter, resolve SpecResolver, hooks Hooks) *Manager {
	return &Manager{
		store:   store,
		conv:    conv,
		resolve: resolve,
		hooks:   hooks,
		wake:    make(chan struct{}, 1),
	}
}

// Start loads the persisted list and launches the worker. Jobs found in
// "running" state died with a previous process — they become "interrupted"
// so the UI can offer Resume.
func (m *Manager) Start(ctx context.Context) error {
	jobs, err := m.store.Load()
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.jobs = jobs
	for _, j := range m.jobs {
		if j.Status == StatusRunning {
			j.Status = StatusInterrupted
			j.Error = "the app closed while this conversion was running"
		}
	}
	m.persistLocked()
	m.mu.Unlock()

	go m.worker(ctx)
	m.kick()
	return nil
}

// Jobs returns the list newest-first (queue order preserved among queued).
func (m *Manager) Jobs() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Job, 0, len(m.jobs))
	for i := len(m.jobs) - 1; i >= 0; i-- {
		out = append(out, *m.jobs[i])
	}
	return out
}

func (m *Manager) Get(id string) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.byID(id); j != nil {
		return *j, true
	}
	return Job{}, false
}

// Enqueue adds a new job at the tail of the queue.
func (m *Manager) Enqueue(j Job) (Job, error) {
	if j.InputPath == "" || j.OutputPath == "" || len(j.Targets) == 0 {
		return Job{}, fmt.Errorf("job needs input, output and at least one target")
	}
	j.ID = newID()
	j.CreatedAt = time.Now()
	j.Status = StatusQueued
	j.Error = ""

	rec := j // the list owns its own copy; the worker mutates it under the lock
	m.mu.Lock()
	m.jobs = append(m.jobs, &rec)
	m.persistLocked()
	out := rec
	m.mu.Unlock()

	m.emitState(out)
	m.kick()
	return out, nil
}

// Cancel stops a running job (SIGTERM to the child's process group — the
// cache survives and Retry resumes) or drops a queued one to canceled.
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	j := m.byID(id)
	if j == nil {
		m.mu.Unlock()
		return fmt.Errorf("no such job")
	}
	switch j.Status {
	case StatusRunning:
		cancel := m.cancel
		m.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return nil
	case StatusQueued:
		j.Status = StatusCanceled
		m.persistLocked()
		out := *j
		m.mu.Unlock()
		m.emitState(out)
		return nil
	default:
		m.mu.Unlock()
		return fmt.Errorf("job is not queued or running")
	}
}

// Retry re-queues a finished job with its exact stored parameters (same
// cache identity → resume). force adds --force for a from-scratch redo.
func (m *Manager) Retry(id string, force bool) (Job, error) {
	m.mu.Lock()
	j := m.byID(id)
	if j == nil {
		m.mu.Unlock()
		return Job{}, fmt.Errorf("no such job")
	}
	if j.active() {
		m.mu.Unlock()
		return Job{}, fmt.Errorf("job is already queued or running")
	}
	j.Status = StatusQueued
	j.Error = ""
	j.Force = force
	j.FinishedAt = nil
	m.persistLocked()
	out := *j
	m.mu.Unlock()

	m.emitState(out)
	m.kick()
	return out, nil
}

// Promote moves a queued job to the front of the queue ("Start now").
func (m *Manager) Promote(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := -1
	firstQueued := -1
	for i, j := range m.jobs {
		if j.Status == StatusQueued && firstQueued < 0 {
			firstQueued = i
		}
		if j.ID == id {
			idx = i
		}
	}
	if idx < 0 || m.jobs[idx].Status != StatusQueued {
		return fmt.Errorf("job is not queued")
	}
	if firstQueued < idx {
		j := m.jobs[idx]
		copy(m.jobs[firstQueued+1:idx+1], m.jobs[firstQueued:idx])
		m.jobs[firstQueued] = j
		m.persistLocked()
	}
	return nil
}

// Remove deletes a finished job's record. It never touches the .tbook.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, j := range m.jobs {
		if j.ID != id {
			continue
		}
		if j.active() {
			return fmt.Errorf("cancel the job before removing it")
		}
		m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
		m.persistLocked()
		return nil
	}
	return fmt.Errorf("no such job")
}

func (m *Manager) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
		}
		for {
			j := m.claimNext()
			if j == nil {
				break
			}
			m.runOne(ctx, j)
		}
	}
}

// claimNext marks the oldest queued job running and returns a snapshot.
func (m *Manager) claimNext() *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.Status != StatusQueued {
			continue
		}
		now := time.Now()
		j.Status = StatusRunning
		j.StartedAt = &now
		j.Attempts++
		m.runningID = j.ID
		m.persistLocked()
		return j
	}
	return nil
}

func (m *Manager) runOne(ctx context.Context, j *Job) {
	attempt := strconv.Itoa(j.Attempts)

	runCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.cancel = cancel
	j.LogPath = m.conv.LogPath(j.ID, attempt)
	snapshot := *j
	m.mu.Unlock()
	defer cancel()
	m.emitState(snapshot)

	var cost float64
	var err error
	spec, rerr := m.resolve(snapshot, attempt)
	if rerr != nil {
		err = rerr
	} else {
		cost, err = m.conv.Convert(runCtx, spec,
			func(ev runner.ProgressEvent) {
				if m.hooks.OnProgress != nil {
					m.hooks.OnProgress(snapshot.ID, ev)
				}
			},
			func(usd float64) {
				if m.hooks.OnCost != nil {
					m.hooks.OnCost(snapshot.ID, usd)
				}
			})
	}

	m.mu.Lock()
	now := time.Now()
	j.FinishedAt = &now
	j.CostUSD += cost
	m.runningID = ""
	m.cancel = nil
	switch {
	case err == nil:
		j.Status = StatusDone
		j.Error = ""
	case runCtx.Err() != nil:
		j.Status = StatusCanceled
		j.Error = ""
	default:
		j.Status = StatusFailed
		j.Error = err.Error()
	}
	m.persistLocked()
	out := *j
	m.mu.Unlock()

	m.emitState(out)
}

func (m *Manager) byID(id string) *Job {
	for _, j := range m.jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

func (m *Manager) persistLocked() {
	_ = m.store.Save(m.jobs) // history persistence is best-effort by design
}

func (m *Manager) emitState(j Job) {
	if m.hooks.OnState != nil {
		m.hooks.OnState(j)
	}
}

func (m *Manager) kick() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
