package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func appendFile(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

func TestTailerMissingFile(t *testing.T) {
	var events []ProgressEvent
	tl := newProgressTailer(filepath.Join(t.TempDir(), "nope.ndjson"),
		func(ev ProgressEvent) { events = append(events, ev) })
	tl.drain()
	if len(events) != 0 || tl.finalError() != "" {
		t.Errorf("missing file produced events=%v err=%q", events, tl.finalError())
	}
}

func TestTailerPartialAndMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.ndjson")
	var events []ProgressEvent
	tl := newProgressTailer(path, func(ev ProgressEvent) { events = append(events, ev) })

	appendFile(t, path, `{"ts":"t","phase":"translate","target":"ru","done":1`)
	tl.drain()
	if len(events) != 0 {
		t.Fatalf("partial line must not emit: %v", events)
	}

	appendFile(t, path, ",\"total\":10}\nnot json\n")
	tl.drain()
	if len(events) != 1 || events[0].Done != 1 || events[0].Total != 10 {
		t.Fatalf("completed line: got %v", events)
	}

	appendFile(t, path, `{"ts":"t","phase":"translate","target":"de","done":2,"total":10}`+"\n")
	tl.drain()
	if len(events) != 2 || events[1].Target != "de" {
		t.Fatalf("interleaved target: got %v", events)
	}
}

func TestTailerCapturesDoneError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.ndjson")
	tl := newProgressTailer(path, nil)

	appendFile(t, path, `{"ts":"t","phase":"done","ok":false,"error":"boom"}`+"\n")
	tl.drain()
	if tl.finalError() != "boom" {
		t.Errorf("finalError = %q, want boom", tl.finalError())
	}
}

func TestTailerDoneErrorFallbackMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.ndjson")
	tl := newProgressTailer(path, nil)

	appendFile(t, path, `{"ts":"t","phase":"done","ok":false}`+"\n")
	tl.drain()
	if tl.finalError() != "conversion failed" {
		t.Errorf("finalError = %q, want fallback", tl.finalError())
	}
}

func TestTailerDoneOKIsNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.ndjson")
	tl := newProgressTailer(path, nil)

	appendFile(t, path, `{"ts":"t","phase":"done","ok":true}`+"\n")
	tl.drain()
	if tl.finalError() != "" {
		t.Errorf("finalError = %q, want empty", tl.finalError())
	}
}

func TestTailerStartStopFinalDrain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.ndjson")
	ch := make(chan ProgressEvent, 16)
	tl := newProgressTailer(path, func(ev ProgressEvent) { ch <- ev })
	stop := tl.start()

	appendFile(t, path, `{"ts":"t","phase":"translate","target":"ru","done":5,"total":10}`+"\n")
	select {
	case ev := <-ch:
		if ev.Done != 5 {
			t.Errorf("event = %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("tailer never picked up the line")
	}

	// A line written just before stop must be delivered by the final drain.
	appendFile(t, path, `{"ts":"t","phase":"done","ok":true}`+"\n")
	stop()
	stop() // idempotent
	select {
	case ev := <-ch:
		if ev.Phase != "done" {
			t.Errorf("final drain event = %+v", ev)
		}
	default:
		t.Fatal("final drain missed the last line")
	}
}
