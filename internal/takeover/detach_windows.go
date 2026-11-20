package takeover

import (
	"os/exec"
	"syscall"
)

const createBreakawayFromJob = 0x01000000

func startDetached(cmd *exec.Cmd) error {
	return startWindows(cmd, func() error { return cmd.Start() })
}

func startWindows(cmd *exec.Cmd, start func() error) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008 | 0x00000200 | createBreakawayFromJob,
		HideWindow:    true,
	}
	if err := start(); err == nil {
		return nil
	}
	cmd.SysProcAttr.CreationFlags &^= createBreakawayFromJob
	return start()
}
