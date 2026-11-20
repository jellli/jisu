package takeover

import (
	"errors"
	"os/exec"
	"testing"
)

func TestWindowsBreakawayRetry(t *testing.T) {
	for _, fail := range []bool{false, true} {
		cmd := exec.Command("jisu.exe")
		calls := 0
		err := startWindows(cmd, func() error {
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
	if err := startWindows(exec.Command("jisu.exe"), func() error { return want }); err != want {
		t.Fatalf("lost launch error: %v", err)
	}
}
