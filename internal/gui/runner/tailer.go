package runner

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// progressTailer polls a possibly-not-yet-existing NDJSON file, emitting each
// complete new line as a ProgressEvent. Poll + seek (no inotify): the file may
// appear only a few seconds into the run.
type progressTailer struct {
	path    string
	cb      func(ProgressEvent)
	mu      sync.Mutex
	offset  int64
	partial []byte
	lastErr string
	done    chan struct{}
	wg      sync.WaitGroup
}

func newProgressTailer(path string, cb func(ProgressEvent)) *progressTailer {
	return &progressTailer{path: path, cb: cb, done: make(chan struct{})}
}

// start launches the polling goroutine and returns a stop function that
// performs one final drain and blocks until the goroutine exits.
func (t *progressTailer) start() (stop func()) {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-t.done:
				t.drain()
				return
			case <-ticker.C:
				t.drain()
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(t.done)
			t.wg.Wait()
		})
	}
}

// drain reads any bytes appended since the previous call.
func (t *progressTailer) drain() {
	f, err := os.Open(t.path)
	if err != nil {
		return // not created yet — fine
	}
	defer f.Close()
	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return
	}
	data, err := io.ReadAll(f)
	if err != nil || len(data) == 0 {
		return
	}
	t.offset += int64(len(data))

	buf := append(t.partial, data...)
	for {
		nl := bytes.IndexByte(buf, '\n')
		if nl < 0 {
			break
		}
		line := bytes.TrimSpace(buf[:nl])
		buf = buf[nl+1:]
		if len(line) == 0 {
			continue
		}
		var ev ProgressEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // tolerate malformed lines
		}
		if ev.Phase == "done" && ev.OK != nil && !*ev.OK {
			t.mu.Lock()
			t.lastErr = ev.Error
			if t.lastErr == "" {
				t.lastErr = "conversion failed"
			}
			t.mu.Unlock()
		}
		if t.cb != nil {
			t.cb(ev)
		}
	}
	t.partial = append([]byte(nil), buf...)
}

// finalError returns the error of a {"phase":"done","ok":false} event, if any.
func (t *progressTailer) finalError() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastErr
}
