//go:build !windows

package takeover

import (
	"os/exec"
	"syscall"
)

func startDetached(command func() *exec.Cmd) (*exec.Cmd, error) {
	cmd := command()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd, cmd.Start()
}
