package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func press(t *testing.T, tt *ui.Tester, role ui.Role, name string) {
	t.Helper()
	b, ok := tt.AccessByRole(role, name)
	if !ok {
		t.Fatalf("missing control %q; texts: %q", name, tt.Texts())
	}
	tt.ClickAt(b.Bounds.X+b.Bounds.W/2, b.Bounds.Y+b.Bounds.H/2)
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
}

func TestNativeViewExposesAllEightConnectionsPerTask(t *testing.T) {
	d := newDemo()
	tt := ui.NewTester(d.view, 1120, 820)
	for _, task := range d.Tasks {
		if !tt.HasText(task.Name) || !tt.HasText(task.Source+" · "+bytes(task.SizeMB)) {
			t.Fatalf("missing file metadata for %s", task.Name)
		}
		for n := range task.Segments {
			name := fmt.Sprintf("%s 连接 %d", task.Name, n+1)
			if _, ok := tt.AccessByRole(ui.RoleProgress, name); !ok {
				t.Fatalf("missing native progress role %q", name)
			}
		}
	}
}

func TestNativePauseResumeAndDetails(t *testing.T) {
	d := newDemo()
	tt := ui.NewTester(d.view, 1120, 820)
	name := d.Tasks[0].Name
	press(t, tt, ui.RoleButton, "暂停 "+name)
	if d.Tasks[0].State != paused || !tt.HasText("0.0 MB/s") {
		t.Fatal("pause did not update the native row and summary")
	}
	press(t, tt, ui.RoleButton, "继续 "+name)
	if d.Tasks[0].State != downloading || !tt.HasText("18.4 MB/s") {
		t.Fatal("resume did not restore the native row and summary")
	}
	press(t, tt, ui.RoleButton, "连接详情 "+name)
	if !d.Tasks[0].Expanded || !tt.HasText("连接 8") {
		t.Fatal("connection details did not expand")
	}
	press(t, tt, ui.RoleButton, "收起连接 "+name)
	if d.Tasks[0].Expanded {
		t.Fatal("connection details did not collapse")
	}
}

func TestNativeResetRestoresRowsAndHeader(t *testing.T) {
	d := newDemo()
	d.setScenario(string(failed))
	tt := ui.NewTester(d.view, 1120, 820)
	press(t, tt, ui.RoleButton, "重试 "+d.Tasks[0].Name)
	if d.Tasks[0].State != downloading {
		t.Fatal("native retry did not resume")
	}
	d.Filter = 3
	tt.Frame()
	if tt.HasText(d.Tasks[0].Name) || !tt.HasText(d.Tasks[2].Name) {
		t.Fatal("native completed filter shows incorrect rows")
	}
	press(t, tt, ui.RoleButton, "重置样本")
	if d.Filter != 0 || !tt.HasText(d.Tasks[0].Name) || !tt.HasText("18.4 MB/s") {
		t.Fatal("reset left stale native rows or summary")
	}
}
