//go:build windows

package runner

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

const createNoWindow = 0x08000000 // CREATE_NO_WINDOW

// setupCmd hides the child's console window and installs a cancel handler
// that kills the whole process tree via taskkill (no process groups to signal
// on Windows).
func setupCmd(cmd *exec.Cmd) (cleanup func()) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	cmd.WaitDelay = 15 * time.Second

	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: createNoWindow,
		}
		_ = kill.Run()
		return nil
	}
	return func() {}
}
