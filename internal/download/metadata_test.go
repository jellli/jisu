package download

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-0" {
			t.Error("probe must request only one byte")
		}
		w.Header().Set("Content-Disposition", `attachment; filename="release.iso"`)
		switch r.URL.Path {
		case "/range":
			w.Header().Set("Content-Range", "bytes 0-0/12345")
			w.WriteHeader(http.StatusPartialContent)
			fmt.Fprint(w, "x")
		case "/empty":
			w.Header().Set("Content-Length", "0")
		case "/empty-range":
			w.Header().Set("Content-Range", "bytes */0")
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		case "/invalid-range":
			w.Header().Set("Content-Range", "bytes */123")
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		case "/missing-range":
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		case "/unknown":
			w.(http.Flusher).Flush()
		case "/error":
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	q, err := Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	for _, tc := range []struct {
		path          string
		size          int64
		known, failed bool
	}{
		{"/range", 12345, true, false}, {"/empty", 0, true, false},
		{"/empty-range", 0, true, false}, {"/invalid-range", 0, false, true}, {"/missing-range", 0, false, true},
		{"/unknown", 0, false, false}, {"/error", 0, false, true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := q.Probe(context.Background(), server.URL+tc.path)
			if (err != nil) != tc.failed {
				t.Fatalf("error = %v", err)
			}
			if tc.failed {
				return
			}
			if got.Name != "release.iso" || (got.Size != nil) != tc.known {
				t.Fatalf("metadata = %+v", got)
			}
			if got.Size != nil && *got.Size != tc.size {
				t.Fatalf("size = %d", *got.Size)
			}
		})
	}
	if _, err := q.Probe(context.Background(), "file:///etc/passwd"); err == nil {
		t.Fatal("accepted non-HTTP URL")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := q.Probe(ctx, server.URL); err == nil {
		t.Fatal("ignored cancellation")
	}
}
