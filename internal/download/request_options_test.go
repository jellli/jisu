package download

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestTakeoverHeaders(t *testing.T) {
	f := newRangeFixture(t)
	original := f.Config.Handler
	f.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session=private" || r.UserAgent() != "Takeover" || r.Referer() != "http://example.com/" {
			w.WriteHeader(403)
			return
		}
		original.ServeHTTP(w, r)
	})
	q, state, _ := openTestQueue(t)
	if err := q.AddWithOptions(f.URL, DefaultQueue, RequestOptions{Cookie: "session=private", UserAgent: "Takeover", Referer: "http://example.com/"}); err != nil {
		t.Fatal(err)
	}
	j := waitForState(t, q, 0, Completed)
	assertOutput(t, q, j.ID, f.original)
	data, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("session=private")) {
		t.Fatal("cookie persisted")
	}
	var saved config
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Jobs[0].Options.Cookie != "" || saved.Jobs[0].Options.Referer != "http://example.com/" {
		t.Fatal("incorrect restored options")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(f.resetStarts()) < 2 {
		t.Fatal("expected segmented requests")
	}
}

func TestInvalidRequestOptions(t *testing.T) {
	for _, v := range []string{"secret\r\nx", "x\x00", "x\x7f", strings.Repeat("x", 16385)} {
		o := RequestOptions{Cookie: v}
		if o.validate() == nil {
			t.Fatal("accepted invalid cookie")
		}
	}
	for _, v := range []string{"https://user:pass@example.com/", "file:///tmp/a", "invalid"} {
		o := RequestOptions{Referer: v}
		if o.validate() != nil || o.Referer != "" {
			t.Fatal("unsafe referrer")
		}
	}
	o := RequestOptions{}
	if o.validate() != nil {
		t.Fatal("empty options rejected")
	}
}

func TestRedirectCredentials(t *testing.T) {
	for _, downgrade := range []bool{false, true} {
		t.Run(map[bool]string{false: "cross-host", true: "downgrade"}[downgrade], func(t *testing.T) {
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Cookie") != "" {
					t.Error("cookie leaked")
				}
				if downgrade && r.Referer() != "" {
					t.Error("referrer leaked")
				}
			}))
			defer target.Close()
			destination := target.URL
			if !downgrade {
				destination = strings.Replace(destination, "127.0.0.1", "localhost", 1)
			}
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination, 302) })
			source := httptest.NewUnstartedServer(h)
			if downgrade {
				source.StartTLS()
			} else {
				source.Start()
			}
			defer source.Close()
			q, _, _ := openTestQueue(t)
			if downgrade {
				q.client.Transport = source.Client().Transport
			}
			resp, err := q.request(context.Background(), source.URL, "", "", RequestOptions{Cookie: "secret=x", Referer: "https://example.com/"})
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
		})
	}
}
