package takeover

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"github.com/jellli/jisu/internal/download"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFraming(t *testing.T) {
	var b bytes.Buffer
	if err := WriteFrame(&b, Message{URL: "https://example.com/"}); err != nil {
		t.Fatal(err)
	}
	var m Message
	if err := ReadFrame(&b, &m); err != nil || m.URL != "https://example.com/" {
		t.Fatalf("round trip: %v", err)
	}
	binary.Write(&b, binary.NativeEndian, uint32(1<<20+1))
	if ReadFrame(&b, &m) == nil {
		t.Fatal("oversize accepted")
	}
	if ReadFrame(strings.NewReader(""), &m) != io.EOF {
		t.Fatal("empty frame")
	}
}
func TestEndpointAndChain(t *testing.T) {
	config := t.TempDir()
	q, err := download.Open(filepath.Join(config, "queue.json"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	e, err := Start(config, q)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	data, _ := os.ReadFile(e.file)
	var d discovery
	json.Unmarshal(data, &d)
	h := handler(d.Token, q)
	for _, tc := range []struct {
		method, token, body string
		code                int
	}{{"POST", "", "{}", 401}, {"POST", "wrong", "{}", 401}, {"GET", d.Token, "", 405}, {"POST", d.Token, `{"url":"` + strings.Repeat("a", 65<<10) + `"}`, 413}, {"POST", d.Token, "{", 400}} {
		r := httptest.NewRequest(tc.method, "/", strings.NewReader(tc.body))
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("status %d want %d", w.Code, tc.code)
		}
	}
	payload := bytes.Repeat([]byte("Takeover"), 1<<20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session=ok" {
			w.WriteHeader(403)
			return
		}
		http.ServeContent(w, r, "file.zip", time.Unix(100, 0), bytes.NewReader(payload))
	}))
	defer server.Close()
	var input, output bytes.Buffer
	WriteFrame(&input, Message{URL: server.URL + "/file.zip", Cookies: "session=ok"})
	if err := RunHost(&input, &output, config, func() error { t.Fatal("unexpected launch"); return nil }); err != nil {
		t.Fatal(err)
	}
	var reply Reply
	if err := ReadFrame(&output, &reply); err != nil || !reply.OK {
		t.Fatalf("host reply %+v %v", reply, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		jobs := q.State().Jobs
		if len(jobs) > 0 {
			j := jobs[0]
			if j.State == download.Failed {
				t.Fatal(j.Error)
			}
			if j.State == download.Completed {
				path, err := q.CompletedPath(j.ID)
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatal("wrong bytes")
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Job did not complete")
}
func TestManifest(t *testing.T) {
	for platform, count := range map[string]int{"darwin": 2, "windows": 1, "linux": 3} {
		paths := ManifestPaths(platform, t.TempDir(), t.TempDir())
		if len(paths) != count {
			t.Fatal(paths)
		}
		for _, path := range paths {
			if !filepath.IsAbs(path) || !strings.HasSuffix(path, HostName+".json") {
				t.Fatal(path)
			}
		}
	}
	home, config := t.TempDir(), t.TempDir()
	if err := register(home, config, func(path string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var m map[string]any
		if err = json.Unmarshal(b, &m); err != nil {
			return err
		}
		if m["name"] != HostName || m["type"] != "stdio" || m["allowed_origins"].([]any)[0] != "chrome-extension://"+ExtensionID+"/" {
			t.Fatal("invalid manifest")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
