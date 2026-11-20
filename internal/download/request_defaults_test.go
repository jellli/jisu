package download

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPlainJobRequestDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, present := r.Header["Cookie"]; present {
			t.Error("plain Job sent a Cookie header")
		}
		if r.UserAgent() != "Go-http-client/1.1" {
			t.Errorf("User-Agent = %q, want Go default", r.UserAgent())
		}
		http.ServeContent(w, r, "file.bin", time.Unix(100, 0), bytes.NewReader([]byte("plain Job")))
	}))
	defer server.Close()
	q, _, _ := openTestQueue(t)
	if err := q.Add(server.URL); err != nil {
		t.Fatal(err)
	}
	waitForState(t, q, 0, Completed)
}
