package download

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func openJobProxyQueue(t *testing.T) (*Queue, string) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "queue.json")
	id := strings.Repeat("a", 32)
	body := fmt.Sprintf(`{"directory":%q,"concurrency":2,"jobs":[{"id":%q,"url":"http://download.invalid/start","name":"file.bin","directory":%q,"state":"paused"}]}`, dir, id, dir)
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	q, err := Open(file, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	return q, id
}

func TestJobProxyPersistence(t *testing.T) {
	q, id := openJobProxyQueue(t)
	if got := q.State().Jobs[0].Proxy; got != (ProxySettings{}) {
		t.Fatalf("old job proxy = %+v, want inherit", got)
	}
	for _, p := range []ProxySettings{{Mode: "custom", URL: "http://user:secret@localhost:8888"}, {Mode: "none"}, {Mode: "system"}, {}} {
		if err := q.SetJobProxy(id, p); err != nil {
			t.Fatal(err)
		}
		reopened, err := Open(q.file, q.config.Directory)
		if err != nil {
			t.Fatal(err)
		}
		got := reopened.State().Jobs[0].Proxy
		reopened.Close()
		if got != p {
			t.Fatalf("proxy = %+v, want %+v", got, p)
		}
	}
	if err := q.SetJobProxy(id, ProxySettings{Mode: "custom", URL: "localhost:8080"}); err != nil {
		t.Fatal(err)
	}
	if got := q.State().Jobs[0].Proxy.URL; got != "http://localhost:8080" {
		t.Fatalf("normalized URL = %q", got)
	}
	before, _ := os.ReadFile(q.file)
	for _, p := range []ProxySettings{{Mode: "unknown"}, {Mode: "custom", URL: "bad"}, {Mode: "custom", URL: "ftp://host:80"}} {
		if err := q.SetJobProxy(id, p); err == nil {
			t.Fatalf("accepted %+v", p)
		}
	}
	if err := q.SetJobProxy("missing", ProxySettings{}); err == nil {
		t.Fatal("accepted missing job")
	}
	after, _ := os.ReadFile(q.file)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid update changed saved state")
	}
	previous := q.State().Jobs[0].Proxy
	q.file = q.config.Directory
	if err := q.SetJobProxy(id, ProxySettings{Mode: "none"}); err == nil {
		t.Fatal("expected save error")
	}
	if got := q.State().Jobs[0].Proxy; got != previous {
		t.Fatalf("save failure changed proxy: %+v", got)
	}
	q.Close()
	if err := q.SetJobProxy(id, ProxySettings{}); err == nil {
		t.Fatal("accepted update after close")
	}
}

func TestInvalidSavedJobProxy(t *testing.T) {
	q, _ := openJobProxyQueue(t)
	q.Close()
	q.config.Jobs[0].Proxy = ProxySettings{Mode: "custom", URL: "bad"}
	body, err := json.Marshal(q.config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(q.file, body, 0600); err != nil {
		t.Fatal(err)
	}
	if reopened, err := Open(q.file, q.config.Directory); err == nil {
		reopened.Close()
		t.Fatal("accepted invalid saved job proxy")
	}
}

func TestJobProxyDownloads(t *testing.T) {
	payload := bytes.Repeat([]byte("proxy payload"), 256*1024)
	var globalHits, jobHits, globalRanges, jobRanges atomic.Int32
	proxy := func(hits, ranges *atomic.Int32) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if r.URL.Path == "/start" {
				http.Redirect(w, r, "http://redirect.invalid/file.bin", http.StatusFound)
				return
			}
			if r.Header.Get("Range") != "" && r.Header.Get("Range") != "bytes=0-0" {
				ranges.Add(1)
			}
			w.Header().Set("ETag", `"payload"`)
			http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(payload))
		}))
	}
	global := proxy(&globalHits, &globalRanges)
	defer global.Close()
	override := proxy(&jobHits, &jobRanges)
	defer override.Close()
	q, id := openJobProxyQueue(t)
	if err := q.SetProxy(ProxySettings{Mode: "custom", URL: global.URL}); err != nil {
		t.Fatal(err)
	}
	if err := q.SetJobProxy(id, ProxySettings{Mode: "custom", URL: override.URL}); err != nil {
		t.Fatal(err)
	}
	if err := q.Resume(id); err != nil {
		t.Fatal(err)
	}
	waitJob(t, q, 0, func(j Job) bool { return j.State == Completed })
	assertOutput(t, q, id, payload)
	if globalHits.Load() != 0 || jobHits.Load() < 3 || jobRanges.Load() == 0 {
		t.Fatalf("override routing: global=%d job=%d ranges=%d", globalHits.Load(), jobHits.Load(), jobRanges.Load())
	}
	before := jobHits.Load()
	if err := q.Add("http://download.invalid/start"); err != nil {
		t.Fatal(err)
	}
	j := waitJob(t, q, 1, func(j Job) bool { return j.State == Completed })
	assertOutput(t, q, j.ID, payload)
	if globalHits.Load() < 3 || globalRanges.Load() == 0 || jobHits.Load() != before {
		t.Fatalf("global routing: global=%d ranges=%d job=%d", globalHits.Load(), globalRanges.Load(), jobHits.Load())
	}
}

func TestJobProxyUpdatesSubsequentRequests(t *testing.T) {
	q, id := openJobProxyQueue(t)
	global := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "global") }))
	defer global.Close()
	override := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "job") }))
	defer override.Close()
	if err := q.SetProxy(ProxySettings{Mode: "custom", URL: global.URL}); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), jobProxyKey{}, id)
	for _, p := range []ProxySettings{{Mode: "custom", URL: override.URL}, {Mode: "none"}, {Mode: "system"}, {}} {
		if err := q.SetJobProxy(id, p); err != nil {
			t.Fatal(err)
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost/file", nil)
		u, err := q.proxyForRequest(req)
		want := ""
		if p.Mode == "custom" {
			want = override.URL
		} else if p.Mode == "" {
			want = global.URL
		}
		if err != nil || proxyString(u) != want {
			t.Fatalf("mode %q: proxy=%v err=%v want=%q", p.Mode, u, err, want)
		}
		if p.Mode == "custom" || p.Mode == "" {
			resp, err := q.requestOnce(ctx, "http://download.invalid/file", "", "")
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			expected := "global"
			if p.Mode == "custom" {
				expected = "job"
			}
			if err != nil || string(body) != expected {
				t.Fatalf("request after update: body=%q err=%v", body, err)
			}
		}
	}
	if err := q.SetJobProxy(id, ProxySettings{Mode: "custom", URL: override.URL}); err != nil {
		t.Fatal(err)
	}
	resp, err := q.requestOnce(context.Background(), "http://page.invalid/", "", "")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || string(body) != "global" {
		t.Fatalf("jobless request: body=%q err=%v", body, err)
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "direct") }))
	defer origin.Close()
	if err := q.SetJobProxy(id, ProxySettings{Mode: "none"}); err != nil {
		t.Fatal(err)
	}
	resp, err = q.requestOnce(ctx, origin.URL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || string(body) != "direct" {
		t.Fatalf("direct request: body=%q err=%v", body, err)
	}
}
