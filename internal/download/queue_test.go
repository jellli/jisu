package download

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const prefixSize = 64 << 10

func TestResumeSegmentsAfterRestart(t *testing.T) {
	t.Parallel()
	fixture := newRangeFixture(t)
	fixture.stall.Store(true)
	q, state, dir := openTestQueue(t)
	if err := q.Add(fixture.URL + "/archive.bin"); err != nil {
		t.Fatal(err)
	}
	job := waitJob(t, q, 0, func(j Job) bool {
		return j.State == Downloading && j.Connections == 4 && j.Received == 4*prefixSize
	})
	q.Close() // Simulates exiting during a download without explicitly pausing it.
	fixture.stall.Store(false)
	fixture.resetStarts()
	q, err := Open(state, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	complete := waitForState(t, q, 0, Completed)
	if complete.ID != job.ID {
		t.Fatal("job ID changed across restart")
	}
	assertOutput(t, q, complete.ID, fixture.original)
	starts := fixture.resetStarts()
	for i := range 4 {
		want := int64(i*len(fixture.original)/4 + prefixSize)
		if !starts[want] {
			t.Errorf("missing resumed range at %d; got %v", want, starts)
		}
	}
}

func TestQueuePauseAndChangedFile(t *testing.T) {
	t.Parallel()
	fixture := newRangeFixture(t)
	fixture.stall.Store(true)
	q, state, dir := openTestQueue(t)
	if err := q.SetConcurrency(1); err != nil {
		t.Fatal(err)
	}
	if err := q.Add(fixture.URL + "/first.bin"); err != nil {
		t.Fatal(err)
	}
	first := waitJob(t, q, 0, func(j Job) bool { return j.Received == 4*prefixSize })
	if err := q.Add(fixture.URL + "/second.bin"); err != nil {
		t.Fatal(err)
	}
	if state := q.State().Jobs[1].State; state != Queued {
		t.Fatalf("second job should wait for capacity; got %s", state)
	}
	if err := q.Pause(first.ID); err != nil {
		t.Fatal(err)
	}
	second := waitForState(t, q, 1, Downloading)
	if err := q.Pause(second.ID); err != nil {
		t.Fatal(err)
	}
	q.Close()
	fixture.stall.Store(false)
	fixture.replaced.Store(true)
	fixture.resetStarts()
	q, err := Open(state, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	for _, job := range q.State().Jobs {
		if job.State != Paused {
			t.Fatalf("explicit pause did not persist: %+v", job)
		}
	}
	if err := q.Resume(first.ID); err != nil {
		t.Fatal(err)
	}
	waitForState(t, q, 0, Completed)
	assertOutput(t, q, first.ID, fixture.replacement)
	starts := fixture.resetStarts()
	for i := range 4 {
		want := int64(i * len(fixture.original) / 4)
		if !starts[want] {
			t.Errorf("changed file should restart range at %d; got %v", want, starts)
		}
	}
	if err := q.Remove(second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".jisu-"+second.ID)); !os.IsNotExist(err) {
		t.Fatalf("unfinished parts not removed, stat error: %v", err)
	}
}

func TestStreamFallbackAndNoOverwrite(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Disposition", `attachment; filename="../notes.txt"`)
		fmt.Fprint(w, "downloaded") // Ignore the range and send the entire stream.
	}))
	t.Cleanup(server.Close)
	q, _, dir := openTestQueue(t)
	original := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(original, []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := q.Add(server.URL + "/ignored"); err != nil {
		t.Fatal(err)
	}
	done := waitForState(t, q, 0, Completed)
	if done.Name != "notes (1).txt" || requests.Load() != 1 {
		t.Fatalf("fallback did not use a single request or honor the available name: %+v (%d requests)", done, requests.Load())
	}
	assertOutput(t, q, done.ID, []byte("downloaded"))
	if err := q.Remove(done.ID); err != nil {
		t.Fatal(err)
	}
	assertFile(t, original, []byte("keep me"))
	assertFile(t, filepath.Join(dir, done.Name), []byte("downloaded"))
	if err := q.Add("file:///tmp/notes.txt"); err == nil {
		t.Fatal("accepted a non-HTTP URL")
	}
}

func TestStreamFallbackWhenServerOnlyHonorsProbe(t *testing.T) {
	t.Parallel()
	const upstream = "an entire small file"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"stable"`)
		if r.Header.Get("Range") == "bytes=0-0" {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", len(upstream)))
			w.WriteHeader(http.StatusPartialContent)
			fmt.Fprint(w, upstream[:1])
			return
		}
		fmt.Fprint(w, upstream)
	}))
	t.Cleanup(server.Close)
	q, _, _ := openTestQueue(t)
	if err := q.Add(server.URL + "/small.bin"); err != nil {
		t.Fatal(err)
	}
	done := waitForState(t, q, 0, Completed)
	assertOutput(t, q, done.ID, []byte(upstream))
}

type rangeFixture struct {
	*httptest.Server
	stall, replaced       atomic.Bool
	original, replacement []byte
	mu                    sync.Mutex
	starts                map[int64]bool
}

func newRangeFixture(t *testing.T) *rangeFixture {
	t.Helper()
	fixture := &rangeFixture{
		original: make([]byte, 4<<20), replacement: make([]byte, 4<<20), starts: make(map[int64]bool),
	}
	for i := range fixture.original {
		fixture.original[i] = byte(i*31 + i/4096)
		fixture.replacement[i] = byte(i*11 - i/4096)
	}
	fixture.Server = httptest.NewServer(http.HandlerFunc(fixture.serve))
	t.Cleanup(fixture.Close)
	return fixture
}

func (f *rangeFixture) serve(w http.ResponseWriter, r *http.Request) {
	body, tag := f.original, `"rev-1"`
	if f.replaced.Load() {
		body, tag = f.replacement, `"rev-2"`
	}
	w.Header().Set("ETag", tag)
	if raw := r.Header.Get("Range"); raw != "bytes=0-0" && f.stall.Load() {
		var start, end int
		if n, _ := fmt.Sscanf(raw, "bytes=%d-%d", &start, &end); n != 2 || end >= len(body) || end-start+1 < prefixSize {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(body)))
		w.Header().Set("Content-Length", fmt.Sprint(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(body[start : start+prefixSize])
		_ = http.NewResponseController(w).Flush()
		<-r.Context().Done()
		return
	} else if raw != "bytes=0-0" {
		var start, end int64
		if n, _ := fmt.Sscanf(raw, "bytes=%d-%d", &start, &end); n == 2 {
			f.mu.Lock()
			f.starts[start] = true
			f.mu.Unlock()
		}
	}
	http.ServeContent(w, r, "file.bin", time.Unix(1_700_000_000, 0), bytes.NewReader(body))
}

func (f *rangeFixture) resetStarts() map[int64]bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	starts := f.starts
	f.starts = make(map[int64]bool)
	return starts
}

func openTestQueue(t *testing.T) (*Queue, string, string) {
	t.Helper()
	state, dir := filepath.Join(t.TempDir(), "queue.json"), t.TempDir()
	q, err := Open(state, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	return q, state, dir
}

func waitForState(t *testing.T, q *Queue, index int, state string) Job {
	t.Helper()
	return waitJob(t, q, index, func(j Job) bool { return j.State == state })
}

func waitJob(t *testing.T, q *Queue, index int, ready func(Job) bool) Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if jobs := q.State().Jobs; len(jobs) > index {
			if j := jobs[index]; ready(j) || j.State == Failed {
				if j.State == Failed {
					t.Fatalf("job failed: %s", j.Error)
				}
				return j
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for download; got %+v", q.State().Jobs)
	return Job{}
}

func assertOutput(t *testing.T, q *Queue, id string, want []byte) {
	t.Helper()
	filename, err := q.CompletedPath(id)
	if err != nil {
		t.Fatal(err)
	}
	assertFile(t, filename, want)
}

func assertFile(t *testing.T, filename string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(want, got) {
		t.Fatalf("download content mismatch: got %d bytes vs %d expected (%v)", len(got), len(want), err)
	}
}
