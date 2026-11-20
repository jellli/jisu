package download

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"", 0}, {"garbage", 0}, {"-1", 0}, {"0", 0}, {" 2 ", 2 * time.Second},
		{"9999999999", maxRetryDelay},
		{now.Add(4 * time.Second).Format(http.TimeFormat), 4 * time.Second},
		{now.Add(time.Hour).Format(http.TimeFormat), maxRetryDelay},
		{now.Add(-time.Hour).Format(http.TimeFormat), 0},
	} {
		t.Run(tc.value, func(t *testing.T) {
			if got := retryAfter(tc.value, now); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestThrottleRequests(t *testing.T) {
	for _, status := range []int{429, 503, 404, 416} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) <= 2 {
					w.WriteHeader(status)
					return
				}
				w.WriteHeader(http.StatusPartialContent)
			}))
			defer server.Close()
			q, _, _ := openTestQueue(t)
			q.retryDelay = func(int) time.Duration { return time.Millisecond }
			ctx := context.WithValue(context.Background(), throttleKey{}, newRequestThrottle())
			resp, err := q.request(ctx, server.URL, "0-0", "")
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			wantCalls, wantStatus := int32(1), status
			if status == 429 || status == 503 {
				wantCalls, wantStatus = 3, 206
			}
			if calls.Load() != wantCalls || resp.StatusCode != wantStatus {
				t.Fatalf("calls=%d status=%d", calls.Load(), resp.StatusCode)
			}
		})
	}
}

func TestThrottleCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		cancel()
	}))
	defer server.Close()
	q, _, _ := openTestQueue(t)
	ctx = context.WithValue(ctx, throttleKey{}, newRequestThrottle())
	if _, err := q.request(ctx, server.URL, "0-0", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

type generatedFile struct{ size int64 }

func (g generatedFile) ReadAt(p []byte, off int64) (int, error) {
	if off >= g.size {
		return 0, io.EOF
	}
	n := int(min(int64(len(p)), g.size-off))
	for i := range p[:n] {
		p[i] = byte((off+int64(i))*31 + 17)
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func TestThrottledRanges(t *testing.T) { checkThrottledFile(t, 8<<20) }

func TestThrottledGiB(t *testing.T) {
	if os.Getenv("JISU_THROTTLE_GIB") != "1" {
		t.Skip("set JISU_THROTTLE_GIB=1 for the 1 GiB integrity check")
	}
	checkThrottledFile(t, 1<<30)
}

func checkThrottledFile(t *testing.T, size int64) {
	t.Helper()
	source := generatedFile{size}
	var active, throttles atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-0" {
			n := active.Add(1)
			defer active.Add(-1)
			if n > 1 {
				throttles.Add(1)
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(429)
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		w.Header().Set("ETag", `"stable"`)
		http.ServeContent(w, r, "generated.bin", time.Time{}, io.NewSectionReader(source, 0, size))
	}))
	defer server.Close()
	q, _, _ := openTestQueue(t)
	q.retryDelay = func(int) time.Duration { return time.Millisecond }
	if err := q.Add(server.URL + "/generated.bin"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Minute)
	var job Job
	for time.Now().Before(deadline) {
		job = q.State().Jobs[0]
		if job.State == Completed || job.State == Failed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job.State != Completed {
		t.Fatalf("download did not complete: %+v", job)
	}
	filename, err := q.CompletedPath(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	want, got := sha256.New(), sha256.New()
	if _, err := io.Copy(want, io.NewSectionReader(source, 0, size)); err != nil {
		t.Fatal(err)
	}
	n, err := io.Copy(got, f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("bytes=%d source SHA-256=%x", size, want.Sum(nil))
	t.Logf("bytes=%d output SHA-256=%x", n, got.Sum(nil))
	t.Logf("HTTP 429 responses=%d", throttles.Load())
	if n != size || string(want.Sum(nil)) != string(got.Sum(nil)) || throttles.Load() == 0 {
		t.Fatal("integrity or throttling check failed")
	}
}
