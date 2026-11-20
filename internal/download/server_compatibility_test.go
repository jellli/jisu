package download

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDownloadUserAgent(t *testing.T) {
	for _, agent := range []string{"", "Browser/1.0"} {
		t.Run(agent, func(t *testing.T) {
			want := agent
			if want == "" {
				want = "jisu"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.UserAgent() != want {
					t.Errorf("User-Agent = %q, want %q", r.UserAgent(), want)
				}
			}))
			defer server.Close()
			q, _, _ := openTestQueue(t)
			resp, err := q.request(context.Background(), server.URL, "0-0", "", RequestOptions{UserAgent: agent})
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
		})
	}
}

func TestSegmentHTTPError(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Range") == "bytes=0-0" {
					w.Header().Set("ETag", `"file"`)
					w.Header().Set("Content-Range", "bytes 0-0/8388608")
					w.WriteHeader(http.StatusPartialContent)
					w.Write([]byte{0})
					return
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			q, _, _ := openTestQueue(t)
			if err := q.Add(server.URL); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(10 * time.Second)
			job := q.State().Jobs[0]
			for job.State != Failed && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
				job = q.State().Jobs[0]
			}
			if job.State != Failed {
				t.Fatalf("job did not fail: %+v", job)
			}
			if want := fmt.Sprintf("server returned HTTP %d", status); job.Error != want {
				t.Fatalf("error = %q, want %q", job.Error, want)
			}
		})
	}
}
