//go:build unix

package runner

import (
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// setupCmd runs the child in its own process group so cancellation also kills
// its descendants (the embalign python subprocess). Cancel sends SIGTERM to
// the group — the converter's NotifyContext flushes the cache and writes
// done{ok:false} — escalating to SIGKILL after 10s via a timer (WaitDelay only
// signals the leader). The returned cleanup stops the escalation timer.
func setupCmd(cmd *exec.Cmd) (cleanup func()) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 15 * time.Second

	var mu sync.Mutex
	var kill *time.Timer
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		pid := cmd.Process.Pid
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		mu.Lock()
		kill = time.AfterFunc(10*time.Second, func() {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		})
		mu.Unlock()
		return nil
	}
	return func() {
		mu.Lock()
		if kill != nil {
			kill.Stop()
		}
		mu.Unlock()
	}
}
