package takeover

import (
	"os/exec"
	"syscall"
)

const createBreakawayFromJob = 0x01000000

func startDetached(command func() *exec.Cmd) (*exec.Cmd, error) {
	return startWindows(command, (*exec.Cmd).Start)
}

func startWindows(command func() *exec.Cmd, start func(*exec.Cmd) error) (*exec.Cmd, error) {
	var err error
	for _, flags := range []uint32{0x00000208 | createBreakawayFromJob, 0x00000208} {
		cmd := command()
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags}
		if err = start(cmd); err == nil {
			return cmd, nil
		}
	}
	return nil, err
}
