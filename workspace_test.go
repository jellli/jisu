package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jellli/jisu/internal/download"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestWorkspaceSniffPageDownloadsSelectionThroughProxy(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.String() {
		case "http://page.invalid/folder/index.html":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<a href="archive.zip">Archive</a><video src="clip.mp4"></video><a href="stream.m3u8">Stream</a>`)
		case "http://page.invalid/folder/clip.mp4":
			fmt.Fprint(w, "video content")
		default:
			t.Errorf("unexpected proxy request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer proxy.Close()
	dir := t.TempDir()
	queue, err := download.Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer queue.Close()
	if err := queue.SetProxy(download.ProxySettings{Mode: "custom", URL: proxy.URL}); err != nil {
		t.Fatal(err)
	}

	app := nsgotest.Start(t, workspace(queue), nsgotest.Size(1100, 680), nsgotest.Idiom(core.IdiomDesktop), nsgotest.Measure())
	app.Press("id:sniff-page")
	app.Type("id:sniff-url", "http://page.invalid/folder/index.html")
	app.Press("id:sniff-start")
	app.WaitFor("id:sniff-dialog >> text:archive.zip (.zip)", 3*time.Second)
	app.WaitFor("id:sniff-dialog >> text:http://page.invalid/folder/clip.mp4", 3*time.Second)
	if got := len(queue.State().Jobs); got != 0 {
		t.Fatalf("sniffing started %d downloads before selection", got)
	}
	app.PressAt("id:sniff-resource-1", core.Point{X: 8, Y: 8})
	app.Press("id:sniff-download")
	app.WaitForGone("id:sniff-dialog", 3*time.Second)
	app.WaitUntil(3*time.Second, func() bool {
		jobs := queue.State().Jobs
		return len(jobs) == 1 && jobs[0].State == download.Completed
	})
	job := queue.State().Jobs[0]
	if job.URL != "http://page.invalid/folder/clip.mp4" || job.Name != "clip.mp4" || job.Queue != download.DefaultQueue {
		t.Fatalf("unexpected selected download: %+v", job)
	}
}

func TestProxySettingsDialog(t *testing.T) {
	dir := t.TempDir()
	queue, err := download.Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(queue.Close)
	if err := queue.SetProxy(download.ProxySettings{Mode: "custom", URL: "http://user:secret@localhost:8080"}); err != nil {
		t.Fatal(err)
	}
	app := nsgotest.Start(t, workspace(queue), nsgotest.Size(1100, 680), nsgotest.Idiom(core.IdiomDesktop), nsgotest.Measure())
	app.Press("id:settings")
	app.WaitFor("id:proxy-url", time.Second)
	app.Type("id:proxy-url", "bad")
	app.Press("id:apply-proxy")
	app.WaitFor("id:settings-dialog >> text:/proxy URL must use/", time.Second)
	if bounds := app.Find("id:settings-error").Bounds(); bounds.Size.Height < 24 {
		t.Fatalf("validation error did not wrap: %v", bounds)
	}
	if queue.State().Proxy.URL != "http://user:secret@localhost:8080" {
		t.Fatal("invalid UI input changed saved proxy")
	}
	if nsgotest.Target() != "" {
		app.Screenshot("proxy-settings")
	}
}

func TestSelectingCustomProxyDoesNotApply(t *testing.T) {
	dir := t.TempDir()
	queue, err := download.Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(queue.Close)
	app := nsgotest.Start(t, workspace(queue), nsgotest.Size(1100, 680), nsgotest.Idiom(core.IdiomDesktop), nsgotest.Measure())
	app.Press("id:settings")
	app.Press("id:proxy-mode")
	app.WaitFor("id:proxy-mode-2", time.Second)
	row := app.Find("id:proxy-mode-2").Bounds()
	app.Press("id:proxy-mode-2")
	app.WaitFor("id:proxy-url", time.Second)
	button := app.Find("id:apply-proxy").Bounds()
	if button.Origin.X < row.Origin.X+row.Size.Width && row.Origin.X < button.Origin.X+button.Size.Width && button.Origin.Y < row.Origin.Y+row.Size.Height && row.Origin.Y < button.Origin.Y+button.Size.Height {
		t.Fatalf("Apply overlaps dismissed Custom row: row=%v button=%v", row, button)
	}
	if queue.State().Proxy.Mode != "system" {
		t.Fatal("selecting Custom applied settings")
	}
	app.WaitForGone("id:settings-dialog >> text:/proxy URL must use/", time.Second)
	app.Press("id:apply-proxy")
	app.WaitFor("id:settings-dialog >> text:/proxy URL must use/", time.Second)
}

func TestWorkspaceUpdateCheck(t *testing.T) {
	if nsgotest.Target() != "" {
		t.Skip("update responses are simulated only by the headless driver")
	}
	for _, tc := range []struct {
		name   string
		found  core.UpdateCheck
		alerts int
	}{
		{"newer", core.UpdateCheck{App: "jisu", Current: "0.2.0", Version: "0.3.0", URL: "https://example.com/release"}, 1},
		{"current", core.UpdateCheck{App: "jisu", Current: "0.2.0"}, 1},
		{"offline", core.UpdateCheck{Err: errors.New("offline")}, 1},
		{"native dialog", core.UpdateCheck{Shown: true}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			queue, err := download.Open(filepath.Join(dir, "queue.json"), dir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(queue.Close)
			app := nsgotest.Start(t, workspace(queue), nsgotest.Size(1100, 680))
			app.Update = func(bool) core.UpdateCheck { return tc.found }
			app.Press("id:settings")
			if len(app.Query("text:"+versionCaption())) != 1 {
				t.Fatal("current version is missing")
			}
			app.Press("id:check-update")
			if !slices.Equal(app.UpdateChecks, []bool{false}) {
				t.Fatalf("checks %v", app.UpdateChecks)
			}
			if alerts := app.Alerts(); len(alerts) != tc.alerts {
				t.Fatalf("alerts %+v", alerts)
			}
		})
	}
}

func TestSegmentFillShowsNoncontiguousBytes(t *testing.T) {
	got := segmentFill(160, []download.ByteSpan{{Start: 0, End: 14}, {Start: 80, End: 87}, {Start: 87, End: 94}})
	want := [segmentCount]float32{0: 1, 1: 0.4, 8: 1, 9: 0.4}
	if got != want {
		t.Fatalf("segment coverage = %v, want %v", got, want)
	}
}

func TestKiteWorkspace(t *testing.T) {
	endpoint := httptest.NewServer(http.HandlerFunc(serveTestDownload))
	t.Cleanup(endpoint.Close)
	dir := t.TempDir()
	queue, err := download.Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(queue.Close)
	names := []string{"Northern-Lights.mov", "Design-Assets.zip", "Workshop-Recording.mp4", "Field-Notes.pdf"}
	for i, name := range names {
		target := download.DefaultQueue
		if i == len(names)-1 {
			target = download.NightQueue
		}
		if err := queue.AddToQueue(endpoint.URL+"/"+name, target); err != nil {
			t.Fatal(err)
		}
	}
	options := []nsgotest.Option{nsgotest.Size(1100, 680), nsgotest.Idiom(core.IdiomDesktop), nsgotest.Measure()}
	if nsgotest.Target() == "" {
		options = append(options, nsgotest.Environment(core.Environment{OS: core.OSLinux}))
	}
	app := nsgotest.Start(t, workspace(queue), options...)
	const timeout = 7 * time.Second
	waitLive := func(selector string) {
		app.WaitUntil(timeout, func() bool {
			if nsgotest.Target() == "" {
				app.Advance(250 * time.Millisecond)
			}
			return len(app.Query(selector)) == 1
		})
	}
	first, second, third, fourth := "text:"+names[0], "text:"+names[1], "text:"+names[2], "text:"+names[3]
	app.WaitFor(second, timeout)
	waitLive("id:tasks >> text:/^[1-9][0-9.]*%$/ >> nth=0")
	app.WaitForGone("id:inspector", timeout)
	screenshot := func(name string) {
		if nsgotest.Target() != "" {
			app.Screenshot(name)
		}
	}
	screenshot("01-downloads")

	app.Press("id:category-archive")
	app.WaitFor(second, timeout)
	app.WaitForGone(first, timeout)
	screenshot("02-category")
	app.Press("id:queue-night")
	app.WaitFor(fourth, timeout)
	app.WaitForGone(second, timeout)
	screenshot("03-night-queue")
	app.Press("id:queue-default")
	app.WaitFor(first, timeout)
	app.WaitForGone(fourth, timeout)
	app.Press(second)
	checkbox := func(index int) string { return "id:select-" + queue.State().Jobs[index].ID }
	clickCheckbox := func(index int) { app.PressAt(checkbox(index), core.Point{X: 8, Y: 8}) }
	clickCheckbox(0)
	app.WaitFor("text:已选 2 项", timeout)
	clickCheckbox(1)
	app.WaitFor("text:已选 1 项", timeout)
	clickCheckbox(0)
	app.WaitFor("text:已选 0 项", timeout)
	app.Press("id:select-all")
	app.WaitFor("text:已选 3 项", timeout)
	clickCheckbox(0)
	app.WaitFor("text:已选 2 项", timeout)
	app.Press(second)
	app.PressWith(third, core.Modifiers{Shift: true})
	app.WaitFor("text:已选 2 项", timeout)
	screenshot("04-multiple-selection")

	app.RightClick(second)
	app.ChoosePopup("上移")
	assertJobOrder(t, queue, []string{names[1], names[2], names[0], names[3]})
	app.WaitFor("text:已选 2 项", timeout)
	screenshot("05-move-up")
	app.RightClick(second)
	app.ChoosePopup("下移")
	assertJobOrder(t, queue, names)
	screenshot("06-move-down")

	app.Press(first)
	app.WaitFor("id:inspector", timeout)
	waitLive("text:活跃连接 8 / 8")
	if nsgotest.Target() == "" {
		app.Advance(500 * time.Millisecond)
	} else {
		time.Sleep(500 * time.Millisecond)
	}
	app.Press(first)
	app.WaitForGone("id:inspector", timeout)
	app.Press("id:toggle-inspector")
	app.WaitFor("id:inspector", timeout)
	waitLive("text:活跃连接 8 / 8")
	screenshot("07-expanded-inspector")
	for _, n := range []int{4, 16, 32, 8} {
		app.Press(fmt.Sprintf("id:connections-%d", n))
		waitLive(fmt.Sprintf("text:活跃连接 %d / %d", n, n))
		if n == 32 {
			screenshot("08-live-32-connections")
		}
	}

	if nsgotest.Target() != "" {
		size := "role:columnheader 大小"
		before := app.Find(size).Bounds()
		app.Drag("id:resize:size", core.Point{X: 12}, core.Point{X: 35})
		if after := app.Find(size).Bounds(); after.Size.Width <= before.Size.Width {
			t.Fatalf("column width did not increase: before %v, after %v", before, after)
		}
		app.Drag(size, core.Point{X: -20}, core.Point{X: -277})
		if after := app.Find(size).Bounds(); after.Origin.X >= app.Find("role:columnheader 文件名 / 协议").Bounds().Origin.X {
			t.Fatalf("大小 did not move ahead of 文件名 / 协议: %v", after)
		}
		screenshot("09-column-layout")
	}
}

func assertJobOrder(t *testing.T, queue *download.Queue, want []string) {
	t.Helper()
	var got []string
	for _, job := range queue.State().Jobs {
		got = append(got, job.Name)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("queue order = %v, want %v", got, want)
	}
}

func serveTestDownload(w http.ResponseWriter, r *http.Request) {
	const size = 128 << 20
	w.Header().Set("ETag", `"kite-workspace-test"`)
	w.Header().Set("Accept-Ranges", "bytes")
	var start, end int
	if parsed, _ := fmt.Sscanf(strings.TrimSpace(r.Header.Get("Range")), "bytes=%d-%d", &start, &end); parsed == 2 && start >= 0 && start <= end && end < size {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		w.Header().Set("Content-Length", fmt.Sprint(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		count := min(end-start+1, 512<<10)
		_, _ = w.Write(make([]byte, count))
		if count < end-start+1 {
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}
		return
	}
	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.WriteHeader(http.StatusOK)
}
