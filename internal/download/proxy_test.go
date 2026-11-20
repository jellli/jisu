package download

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestProxyValidation(t *testing.T) {
	for _, raw := range []string{"localhost:8080", "http://user:pass@localhost:80", "https://proxy.example:443", "socks5://[::1]:1080", "socks5h://host:1080"} {
		if _, err := normalizeProxy(ProxySettings{Mode: "custom", URL: raw}); err != nil {
			t.Errorf("%q: %v", raw, err)
		}
	}
	for _, raw := range []string{"", "host", "http://host", "http://host:0", "http://host:65536", "http://host:bad", "ftp://host:80", "http://host:80/path", "http://host:80?q=x", "http://host:80/#frag", "http://:80", "http://bad host:80"} {
		if _, err := normalizeProxy(ProxySettings{Mode: "custom", URL: raw}); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	if _, err := normalizeProxy(ProxySettings{Mode: "unknown"}); err == nil {
		t.Fatal("accepted unknown mode")
	}
}

func TestProxyPersistence(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "queue.json")
	if err := os.WriteFile(file, []byte(fmt.Sprintf(`{"directory":%q,"concurrency":2}`, dir)), 0600); err != nil {
		t.Fatal(err)
	}
	q, err := Open(file, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	if q.State().Proxy.Mode != "system" {
		t.Fatal("old config did not default to system")
	}
	for _, p := range []ProxySettings{{Mode: "custom", URL: "http://user:pass@localhost:8888"}, {Mode: "none"}, {Mode: "system"}} {
		if err := q.SetProxy(p); err != nil {
			t.Fatal(err)
		}
		reopened, err := Open(file, dir)
		if err != nil {
			t.Fatal(err)
		}
		got := reopened.State().Proxy
		reopened.Close()
		if got != p {
			t.Fatalf("got %+v want %+v", got, p)
		}
	}
	before, _ := os.ReadFile(file)
	if err := q.SetProxy(ProxySettings{Mode: "custom", URL: "bad"}); err == nil {
		t.Fatal("invalid setting accepted")
	}
	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Fatal("invalid setting persisted")
	}
	q.file = dir
	if err := q.SetProxy(ProxySettings{Mode: "none"}); err == nil {
		t.Fatal("expected save failure")
	}
	if q.State().Proxy.Mode != "system" || q.proxy.Load().(ProxySettings).Mode != "system" {
		t.Fatal("save failure changed proxy")
	}
}

func TestCustomProxyDownload(t *testing.T) {
	var hits atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "proxied payload") }))
	defer origin.Close()
	direct := &http.Transport{Proxy: nil}
	defer direct.CloseIdleConnections()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		r.RequestURI = ""
		resp, err := direct.RoundTrip(r)
		if err != nil {
			http.Error(w, "forward failed", 502)
			return
		}
		defer resp.Body.Close()
		for k, values := range resp.Header {
			w.Header()[k] = values
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}))
	defer proxy.Close()
	q, _, _ := openTestQueue(t)
	if err := q.SetProxy(ProxySettings{Mode: "custom", URL: proxy.URL}); err != nil {
		t.Fatal(err)
	}
	if err := q.Add(origin.URL + "/file.bin"); err != nil {
		t.Fatal(err)
	}
	waitJob(t, q, 0, func(j Job) bool { return j.State == Completed })
	if hits.Load() == 0 {
		t.Fatal("download bypassed custom proxy")
	}
	if err := q.SetProxy(ProxySettings{Mode: "none"}); err != nil {
		t.Fatal(err)
	}
	before := hits.Load()
	if err := q.Add(origin.URL + "/direct.bin"); err != nil {
		t.Fatal(err)
	}
	waitJob(t, q, 1, func(j Job) bool { return j.State == Completed })
	if hits.Load() != before {
		t.Fatal("none still used custom proxy")
	}
}

func TestNoneBypassesEnvironmentProxy(t *testing.T) {
	if os.Getenv("JISU_PROXY_TEST_CHILD") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestNoneBypassesEnvironmentProxy$")
		cmd.Env = append(os.Environ(), "JISU_PROXY_TEST_CHILD=1", "HTTP_PROXY=http://127.0.0.1:1", "http_proxy=", "NO_PROXY=", "no_proxy=", "REQUEST_METHOD=")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	q, _, _ := openTestQueue(t)
	req, _ := http.NewRequest("GET", "http://download.example/file", nil)
	p, err := q.proxyForRequest(req)
	if err != nil || p == nil {
		t.Fatalf("system did not use env: %v %v", p, err)
	}
	if err := q.SetProxy(ProxySettings{Mode: "none"}); err != nil {
		t.Fatal(err)
	}
	p, err = q.client.Transport.(*http.Transport).Proxy(req)
	if err != nil || p != nil {
		t.Fatalf("none used env: %v %v", p, err)
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "direct payload") }))
	defer origin.Close()
	q.client.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "download.example:80" {
			return nil, fmt.Errorf("unexpected proxy dial: %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, origin.Listener.Addr().String())
	}
	if err := q.Add(req.URL.String()); err != nil {
		t.Fatal(err)
	}
	waitJob(t, q, 0, func(j Job) bool { return j.State == Completed })
}
