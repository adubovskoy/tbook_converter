package jobs

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dimando/reader/converter/internal/gui/runner"
)

// fakeConv scripts the child process: per-input behavior, recorded specs.
type fakeConv struct {
	mu    sync.Mutex
	specs []runner.ConvertSpec
	// run decides the outcome; nil = instant success with cost 0.1.
	run func(ctx context.Context, spec runner.ConvertSpec) (float64, error)
}

func (f *fakeConv) Convert(ctx context.Context, spec runner.ConvertSpec, onProgress func(runner.ProgressEvent), onCost func(float64)) (float64, error) {
	f.mu.Lock()
	f.specs = append(f.specs, spec)
	f.mu.Unlock()
	if f.run != nil {
		return f.run(ctx, spec)
	}
	return 0.1, nil
}

func (f *fakeConv) LogPath(jobID, attempt string) string {
	return filepath.Join(os.TempDir(), jobID+"-"+attempt+".log")
}

func (f *fakeConv) recorded() []runner.ConvertSpec {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]runner.ConvertSpec(nil), f.specs...)
}

type stateLog struct {
	mu     sync.Mutex
	states []Job
}

func (l *stateLog) add(j Job) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.states = append(l.states, j)
}

// wait polls until the job with id reaches status (states arrive async).
func (l *stateLog) wait(t *testing.T, id string, status Status) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		l.mu.Lock()
		for i := len(l.states) - 1; i >= 0; i-- {
			if l.states[i].ID == id && l.states[i].Status == status {
				j := l.states[i]
				l.mu.Unlock()
				return j
			}
		}
		l.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never reached %s", id, status)
	return Job{}
}

func resolveEcho(j Job, attempt string) (runner.ConvertSpec, error) {
	return runner.ConvertSpec{
		JobID: j.ID, Attempt: attempt,
		InputPath: j.InputPath, OutputPath: j.OutputPath,
		Source: j.Source, Targets: j.Targets,
		Provider: j.Provider, Model: j.Model,
		Force: j.Force,
	}, nil
}

func newTestManager(t *testing.T, conv *fakeConv) (*Manager, *stateLog) {
	t.Helper()
	log := &stateLog{}
	m := NewManager(NewStore(filepath.Join(t.TempDir(), "jobs.json")), conv, resolveEcho,
		Hooks{OnState: log.add})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	return m, log
}

// waitStatus polls the manager until the job reaches status.
func waitStatus(t *testing.T, m *Manager, id string, status Status) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got, _ := m.Get(id); got.Status == status {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never reached %s", id, status)
}

func testJob(input string) Job {
	return Job{
		InputPath:  input,
		OutputPath: input + ".tbook",
		Source:     "en",
		Targets:    []string{"ru"},
		Provider:   "openrouter",
	}
}

func TestEnqueueRunsFIFO(t *testing.T) {
	order := make(chan string, 2)
	conv := &fakeConv{run: func(ctx context.Context, spec runner.ConvertSpec) (float64, error) {
		order <- spec.InputPath
		return 0.25, nil
	}}
	m, log := newTestManager(t, conv)

	a, _ := m.Enqueue(testJob("a.epub"))
	b, _ := m.Enqueue(testJob("b.epub"))
	first := log.wait(t, a.ID, StatusDone)
	log.wait(t, b.ID, StatusDone)

	if got := []string{<-order, <-order}; got[0] != "a.epub" || got[1] != "b.epub" {
		t.Fatalf("run order = %v, want [a.epub b.epub]", got)
	}
	if first.CostUSD != 0.25 {
		t.Fatalf("cost = %v, want 0.25", first.CostUSD)
	}
	if first.Attempts != 1 || first.LogPath == "" || first.StartedAt == nil || first.FinishedAt == nil {
		t.Fatalf("bookkeeping missing: %+v", first)
	}
}

func TestCancelRunning(t *testing.T) {
	started := make(chan struct{})
	conv := &fakeConv{run: func(ctx context.Context, spec runner.ConvertSpec) (float64, error) {
		close(started)
		<-ctx.Done()
		return 0, ctx.Err()
	}}
	m, log := newTestManager(t, conv)

	j, _ := m.Enqueue(testJob("a.epub"))
	<-started
	if err := m.Cancel(j.ID); err != nil {
		t.Fatal(err)
	}
	got := log.wait(t, j.ID, StatusCanceled)
	if got.Error != "" {
		t.Fatalf("canceled job carries error %q", got.Error)
	}
}

func TestCancelQueued(t *testing.T) {
	block := make(chan struct{})
	conv := &fakeConv{run: func(ctx context.Context, spec runner.ConvertSpec) (float64, error) {
		<-block
		return 0, nil
	}}
	m, log := newTestManager(t, conv)

	a, _ := m.Enqueue(testJob("a.epub"))
	b, _ := m.Enqueue(testJob("b.epub"))
	if err := m.Cancel(b.ID); err != nil {
		t.Fatal(err)
	}
	log.wait(t, b.ID, StatusCanceled)
	close(block)
	log.wait(t, a.ID, StatusDone)
	if len(conv.recorded()) != 1 {
		t.Fatalf("canceled queued job still ran")
	}
}

func TestRetryFailedResumesWithSameParams(t *testing.T) {
	fail := true
	conv := &fakeConv{run: func(ctx context.Context, spec runner.ConvertSpec) (float64, error) {
		if fail {
			fail = false
			return 0.1, context.DeadlineExceeded
		}
		return 0.2, nil
	}}
	m, log := newTestManager(t, conv)

	j, _ := m.Enqueue(testJob("a.epub"))
	failed := log.wait(t, j.ID, StatusFailed)
	if failed.Error == "" {
		t.Fatal("failed job carries no error")
	}

	if _, err := m.Retry(j.ID, false); err != nil {
		t.Fatal(err)
	}
	done := log.wait(t, j.ID, StatusDone)
	if done.Attempts != 2 {
		t.Fatalf("attempts = %d, want 2", done.Attempts)
	}
	if diff := done.CostUSD - 0.3; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("cost = %v, want accumulated 0.3", done.CostUSD)
	}
	specs := conv.recorded()
	if specs[1].Attempt != "2" || specs[1].InputPath != "a.epub" || specs[1].Force {
		t.Fatalf("retry spec wrong: %+v", specs[1])
	}
}

func TestRetryForce(t *testing.T) {
	conv := &fakeConv{run: func(ctx context.Context, spec runner.ConvertSpec) (float64, error) {
		if !spec.Force {
			return 0, context.DeadlineExceeded
		}
		return 0, nil
	}}
	m, log := newTestManager(t, conv)
	j, _ := m.Enqueue(testJob("a.epub"))
	log.wait(t, j.ID, StatusFailed)
	if _, err := m.Retry(j.ID, true); err != nil {
		t.Fatal(err)
	}
	log.wait(t, j.ID, StatusDone)
}

func TestStartMarksStaleRunningInterrupted(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "jobs.json"))
	stale := testJob("a.epub")
	stale.ID = "stale"
	stale.Status = StatusRunning
	if err := store.Save([]*Job{&stale}); err != nil {
		t.Fatal(err)
	}

	m := NewManager(store, &fakeConv{}, resolveEcho, Hooks{})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	got, ok := m.Get("stale")
	if !ok || got.Status != StatusInterrupted {
		t.Fatalf("stale job = %+v, want interrupted", got)
	}
}

func TestRemoveGuardsActive(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	conv := &fakeConv{run: func(ctx context.Context, spec runner.ConvertSpec) (float64, error) {
		<-block
		return 0, nil
	}}
	m, _ := newTestManager(t, conv)
	j, _ := m.Enqueue(testJob("a.epub"))
	waitStatus(t, m, j.ID, StatusRunning)
	if err := m.Remove(j.ID); err == nil {
		t.Fatal("Remove allowed on a running job")
	}
}

func TestPromote(t *testing.T) {
	block := make(chan struct{})
	conv := &fakeConv{run: func(ctx context.Context, spec runner.ConvertSpec) (float64, error) {
		<-block
		return 0, nil
	}}
	m, log := newTestManager(t, conv)
	r, _ := m.Enqueue(testJob("running.epub"))
	waitStatus(t, m, r.ID, StatusRunning)
	b, _ := m.Enqueue(testJob("b.epub"))
	c, _ := m.Enqueue(testJob("c.epub"))
	if err := m.Promote(c.ID); err != nil {
		t.Fatal(err)
	}
	close(block)
	log.wait(t, c.ID, StatusDone)
	log.wait(t, b.ID, StatusDone)
	specs := conv.recorded()
	if specs[1].InputPath != "c.epub" || specs[2].InputPath != "b.epub" {
		t.Fatalf("promote order wrong: %v then %v", specs[1].InputPath, specs[2].InputPath)
	}
}

func TestStoreCorruptRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	jobs, err := NewStore(path).Load()
	if err != nil || jobs != nil {
		t.Fatalf("Load = %v, %v; want nil, nil", jobs, err)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatal("corrupt file not preserved as .bak")
	}
}

func TestResolverErrorFailsJob(t *testing.T) {
	log := &stateLog{}
	m := NewManager(NewStore(filepath.Join(t.TempDir(), "jobs.json")), &fakeConv{},
		func(j Job, attempt string) (runner.ConvertSpec, error) {
			return runner.ConvertSpec{}, context.DeadlineExceeded
		},
		Hooks{OnState: log.add})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	j, _ := m.Enqueue(testJob("a.epub"))
	failed := log.wait(t, j.ID, StatusFailed)
	if failed.Error == "" {
		t.Fatal("resolver failure produced no error message")
	}
}
