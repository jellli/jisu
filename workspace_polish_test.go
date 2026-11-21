package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jellli/jisu/internal/download"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestClipboardURL(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{" https://example.com/a.zip\n", "https://example.com/a.zip"},
		{"http://localhost/a", "http://localhost/a"}, {"", ""},
		{"some text", ""}, {"file:///etc/passwd", ""}, {"https://", ""},
		{"https://example.com/a b", ""},
	} {
		if got := clipboardURL(tc.raw); got != tc.want {
			t.Errorf("clipboardURL(%q) = %q", tc.raw, got)
		}
	}
}

func TestNewDownloadMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="release.iso"`)
		w.Header().Set("Content-Range", "bytes 0-0/1024")
		w.WriteHeader(http.StatusPartialContent)
		fmt.Fprint(w, "x")
	}))
	defer server.Close()
	dir := t.TempDir()
	queue, err := download.Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer queue.Close()
	app := nsgotest.Start(t, workspace(queue), nsgotest.Size(1100, 680), nsgotest.Idiom(core.IdiomDesktop), nsgotest.Measure())
	app.Press("id:new-download")
	app.WaitFor("text:保存位置："+dir, time.Second)
	app.Type("id:download-url", server.URL+"/release")
	app.WaitFor("text:release.iso · 1.0 KiB", 3*time.Second)
	app.Clear("id:download-url")
	app.Type("id:download-url", server.URL+"/missing")
	app.WaitFor("text:暂时无法识别文件信息，可直接开始下载。", 3*time.Second)
	if len(queue.State().Jobs) != 0 {
		t.Fatal("metadata preview added a download")
	}
}

func TestSniffResources(t *testing.T) {
	items := []download.PageResource{{Name: "movie.mp4"}, {Name: "release.ISO"}, {Name: "notes.pdf"}, {Name: "song.FLAC"}}
	for _, tc := range []struct {
		kind  string
		count int
	}{{"", 4}, {"video", 1}, {"audio", 1}, {"iso", 1}, {"archive", 0}} {
		if got := len(sniffResources(items, tc.kind)); got != tc.count {
			t.Errorf("%q: %d", tc.kind, got)
		}
	}
	if len(sniffResources(nil, "video")) != 0 {
		t.Fatal("empty resources")
	}
}

func TestStatusActivityAndLowProgress(t *testing.T) {
	jobs := []jobView{{Job: download.Job{State: download.Downloading}}, {Job: download.Job{State: download.Queued}}, {Job: download.Job{State: download.Paused}}}
	colors := func() palette { return palette{} }
	all := func() []jobView { return jobs }
	bar := statusBar(colors, all, all, all, func() string { return "" }).(core.View)
	speed := bar.Children[2].(core.View).Children[1].(core.Text)
	if got := speed.ContentFunc(); !strings.Contains(got, "下载中 1 · 排队 1") {
		t.Fatal(got)
	}
	jobs = nil
	if got := speed.ContentFunc(); !strings.Contains(got, "下载中 0 · 排队 0") {
		t.Fatal(got)
	}
	job := jobView{}
	job.segments[0] = .003
	bars := progressBars(colors, func() jobView { return job }, 5).(core.View)
	fill := bars.Children[0].(core.View).Children[0].(core.View)
	if fill.StyleFunc().MinWidth != core.Pt(2) {
		t.Fatal("low progress has no visible start")
	}
	job.segments[0] = 0
	if fill.StyleFunc().MinWidth == core.Pt(2) {
		t.Fatal("empty segment has a blue start")
	}
}

func TestDarkErrorContrast(t *testing.T) {
	dark := palette{paper: core.RGB(0x1c1c1e)}
	if got := stateColor(dark, jobView{Job: download.Job{State: download.Failed}}); got != core.RGB(0xff8585) {
		t.Fatalf("dark error color = %v", got)
	}
	if got := errorColor(palette{}); got != core.RGB(0xdc2626) {
		t.Fatalf("light error color = %v", got)
	}
}
