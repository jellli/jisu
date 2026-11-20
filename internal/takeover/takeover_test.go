package takeover

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jellli/jisu/internal/download"
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
	WriteFrame(&input, Message{URL: server.URL + "/file.zip", Cookies: "session=ok", Filename: `C:\Downloads\browser-name.zip`})
	if err := RunHost(&input, &output, config, func() error {
		t.Fatal("unexpected launch")
		return nil
	}); err != nil {
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
				if j.Name != "browser-name.zip" {
					t.Fatalf("browser filename was not forwarded: %q", j.Name)
				}
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
		home, config := t.TempDir(), t.TempDir()
		paths := ManifestPaths(platform, home, config)
		if platform != "windows" && len(paths) != 0 {
			t.Fatal("manifests for missing browsers", paths)
		}
		for _, root := range browserTestRoots(platform, home) {
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
		}
		paths = ManifestPaths(platform, home, config)
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
	for _, root := range browserTestRoots(runtime.GOOS, home) {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
	}
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

func browserTestRoots(platform, home string) []string {
	switch platform {
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Application Support", "Google", "Chrome"),
			filepath.Join(home, "Library", "Application Support", "Microsoft Edge"),
		}
	case "windows":
		return nil
	default:
		return []string{
			filepath.Join(home, ".config", "google-chrome"),
			filepath.Join(home, ".config", "chromium"),
			filepath.Join(home, ".config", "microsoft-edge"),
		}
	}
}

func TestEndpointCloseOwnership(t *testing.T) {
	for _, replacement := range []string{"", "second-instance", "invalid-json", "missing"} {
		t.Run(replacement, func(t *testing.T) {
			config := t.TempDir()
			e, err := Start(config, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			switch replacement {
			case "second-instance":
				err = atomicJSON(e.file, discovery{Port: 1234, Token: replacement})
			case "invalid-json":
				err = os.WriteFile(e.file, []byte("{"), 0600)
			case "missing":
				err = os.Remove(e.file)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := e.Close(); err != nil {
				t.Fatal(err)
			}
			_, err = os.Stat(e.file)
			wantRemoved := replacement == "" || replacement == "missing"
			if wantRemoved != os.IsNotExist(err) {
				t.Fatalf("discovery file state: %v", err)
			}
		})
	}
}

func TestLaunchCommand(t *testing.T) {
	for _, tc := range []struct {
		platform, executable, command string
		args                          []string
	}{
		{"darwin", "/Applications/jisu.app/Contents/MacOS/jisu", "/usr/bin/open", []string{"/Applications/jisu.app"}},
		{"darwin", "/Applications/My jisu.app/Contents/MacOS/jisu", "/usr/bin/open", []string{"/Applications/My jisu.app"}},
		{"darwin", "/usr/local/bin/jisu", "/usr/local/bin/jisu", nil},
		{"linux", "/opt/jisu.app/Contents/MacOS/jisu", "/opt/jisu.app/Contents/MacOS/jisu", nil},
	} {
		tc.executable = filepath.FromSlash(tc.executable)
		if tc.command != "/usr/bin/open" {
			tc.command = filepath.FromSlash(tc.command)
		}
		for i := range tc.args {
			tc.args[i] = filepath.FromSlash(tc.args[i])
		}
		cmd := launchCommand(tc.platform, tc.executable)
		if cmd.Path != tc.command || strings.Join(cmd.Args[1:], "|") != strings.Join(tc.args, "|") {
			t.Fatalf("launch command: %v", cmd.Args)
		}
	}
}

func TestManifestSkipsMissingBrowserRoots(t *testing.T) {
	for _, platform := range []string{"darwin", "linux"} {
		home := t.TempDir()
		roots := browserTestRoots(platform, home)
		if err := os.MkdirAll(roots[0], 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(roots[1]), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(roots[1], nil, 0600); err != nil {
			t.Fatal(err)
		}
		paths := ManifestPaths(platform, home, t.TempDir())
		if len(paths) != 1 || paths[0] != filepath.Join(roots[0], "NativeMessagingHosts", HostName+".json") {
			t.Fatal("unexpected browser manifests", paths)
		}
	}
}
