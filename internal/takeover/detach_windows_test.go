package takeover

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestWindowsRetryAfterConsumedStart(t *testing.T) {
	if os.Getenv("JISU_START_HELPER") == "1" {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := func() *exec.Cmd {
		cmd := exec.Command(executable, "-test.run=^TestWindowsRetryAfterConsumedStart$")
		cmd.Env = append(os.Environ(), "JISU_START_HELPER=1")
		return cmd
	}
	calls := 0
	cmd, err := startWindows(command, func(cmd *exec.Cmd) error {
		calls++
		if calls == 1 {
			cmd.Err = errors.New("breakaway denied")
			err := cmd.Start()
			cmd.Err = nil
			return err
		}
		return cmd.Start()
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsBreakawayRetry(t *testing.T) {
	for _, fail := range []bool{false, true} {
		calls := 0
		_, err := startWindows(func() *exec.Cmd { return exec.Command("jisu.exe") }, func(cmd *exec.Cmd) error {
			calls++
			wantBreakaway := calls == 1
			if (cmd.SysProcAttr.CreationFlags&createBreakawayFromJob != 0) != wantBreakaway {
				t.Fatal("incorrect breakaway flag")
			}
			if cmd.SysProcAttr.CreationFlags&0x208 != 0x208 || !cmd.SysProcAttr.HideWindow {
				t.Fatal("lost detached process flags")
			}
			if fail && calls == 1 {
				return errors.New("breakaway denied")
			}
			return nil
		})
		if err != nil || calls != map[bool]int{false: 1, true: 2}[fail] {
			t.Fatalf("start: calls=%d err=%v", calls, err)
		}
	}
	want := errors.New("start failed")
	if _, err := startWindows(func() *exec.Cmd { return exec.Command("jisu.exe") }, func(*exec.Cmd) error { return want }); err != want {
		t.Fatalf("lost launch error: %v", err)
	}
}
