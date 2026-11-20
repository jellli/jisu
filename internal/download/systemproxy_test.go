package download

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func proxyString(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.String()
}

func TestSCUtilProxy(t *testing.T) {
	p := parseSCUtilProxy(`<dictionary> {
  ExceptionsList : <array> {
    0 : *.example.com
    1 : 169.254/16
    2 : printer
  }
  ExcludeSimpleHostnames : 1
  HTTPEnable : 1
  HTTPProxy : 127.0.0.1
  HTTPPort : 7890
  HTTPSEnable : 1
  HTTPSProxy : ::1
  HTTPSPort : 7891
  SOCKSEnable : 1
  SOCKSProxy : 127.0.0.1
  SOCKSPort : 7892
}`)
	for raw, want := range map[string]string{
		"http://github.com/file":     "http://127.0.0.1:7890",
		"https://github.com/file":    "http://[::1]:7891",
		"https://a.example.com/file": "",
		"https://169.254.2.3/file":   "",
		"http://printer/file":        "",
		"http://intranet/file":       "",
	} {
		u, _ := url.Parse(raw)
		if got := proxyString(p.forURL(u)); got != want {
			t.Errorf("%s: got %q, want %q", raw, got, want)
		}
	}
	p.https = nil
	u, _ := url.Parse("https://github.com")
	if got := proxyString(p.forURL(u)); got != "socks5://127.0.0.1:7892" {
		t.Fatal(got)
	}
	p.socks = nil
	if p.forURL(u) != nil {
		t.Fatal("HTTPS must not fall back to HTTP")
	}
	for _, sample := range []string{"", "garbage", "HTTPEnable : 0\nHTTPProxy : localhost\nHTTPPort : 80", "HTTPEnable : 1\nHTTPProxy : localhost\nHTTPPort : invalid", "HTTPEnable : 1\nHTTPProxy : localhost\nHTTPPort : 99999"} {
		p := parseSCUtilProxy(sample)
		if p.http != nil || p.https != nil || p.socks != nil {
			t.Fatalf("invalid/disabled configuration: %+v", p)
		}
	}
}

func TestSCUtilProxyIgnoresScopedSettings(t *testing.T) {
	p := parseSCUtilProxy(`<dictionary> {
  HTTPEnable : 1
  HTTPProxy : global.test
  HTTPPort : 7890
  HTTPSEnable : 0
  ExcludeSimpleHostnames : 0
  ExceptionsList : <array> {
    0 : global-bypass.test
  }
  __SCOPED__ : <dictionary> {
    en0 : <dictionary> {
      HTTPEnable : 1
      HTTPProxy : scoped.test
      HTTPPort : 9999
      HTTPSEnable : 1
      HTTPSProxy : scoped.test
      HTTPSPort : 9999
      ExcludeSimpleHostnames : 1
      ExceptionsList : <array> {
        0 : scoped-bypass.test
      }
    }
  }
}`)
	for raw, want := range map[string]string{
		"http://remote.test":        "http://global.test:7890",
		"http://global-bypass.test": "",
		"http://scoped-bypass.test": "http://global.test:7890",
		"http://printer":            "http://global.test:7890",
		"https://remote.test":       "",
	} {
		u, _ := url.Parse(raw)
		if got := proxyString(p.forURL(u)); got != want {
			t.Errorf("%s: got %q, want %q", raw, got, want)
		}
	}
}

func TestWindowsProxy(t *testing.T) {
	for _, tc := range []struct{ server, http, https, socks string }{
		{"proxy.test:8080", "http://proxy.test:8080", "http://proxy.test:8080", ""},
		{"http=proxy.test:80;https=secure.test:81;socks=sock.test:1080", "http://proxy.test:80", "http://secure.test:81", "socks5://sock.test:1080"},
		{" https = proxy.test:8080 ; socks = [::1]:1080 ", "", "http://proxy.test:8080", "socks5://[::1]:1080"},
		{"", "", "", ""},
		{"http=;https=bad:invalid;socks=bad:99999;garbage", "", "", ""},
	} {
		p := parseWindowsProxy(true, tc.server, "<local>;*.example.com;printer")
		if proxyString(p.http) != tc.http || proxyString(p.https) != tc.https || proxyString(p.socks) != tc.socks {
			t.Errorf("%q: %+v", tc.server, p)
		}
		if !proxyBypassed("intranet", p.exceptions, p.excludeSimple) || !proxyBypassed("a.example.com", p.exceptions, false) {
			t.Fatal("missing Windows override")
		}
		if p := parseWindowsProxy(false, tc.server, ""); p.http != nil || p.https != nil || p.socks != nil {
			t.Fatal("disabled proxy used")
		}
	}
	p := parseWindowsProxy(true, "http=proxy.test:80;socks=sock.test:1080", "")
	u, _ := url.Parse("https://github.com")
	if got := proxyString(p.forURL(u)); got != "socks5://sock.test:1080" {
		t.Fatal(got)
	}
}

func TestProxyBypass(t *testing.T) {
	for _, tc := range []struct {
		host, rule   string
		simple, want bool
	}{
		{"localhost", "", false, true}, {"LOCALHOST.", "", false, true}, {"a.localhost", "", false, true},
		{"127.2.3.4", "", false, true}, {"::1", "", false, true},
		{"a.example.com", "*.example.com", false, true}, {"example.com", "*.example.com", false, false},
		{"EXAMPLE.COM.", "example.com", false, true}, {"otherexample.com", "example.com", false, false},
		{"printer", "printer", false, true}, {"printer", "", true, true}, {"printer.test", "", true, false},
		{"192.168.1.2", "", true, false}, {"printer", "<local>", false, true},
		{"169.254.3.4", "169.254/16", false, true}, {"169.253.3.4", "169.254/16", false, false},
		{"10.2.3.4", "10.0.0.0/8", false, true}, {"fe80::1234", "fe80::/10", false, true},
		{"remote.test", "bad[/bad", false, false}, {"remote.test", "", false, false},
	} {
		if got := proxyBypassed(tc.host, []string{tc.rule}, tc.simple); got != tc.want {
			t.Errorf("%+v: got %v", tc, got)
		}
	}
}

func TestProxyEnvironmentPrecedence(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://github.com", nil)
	sentinel := errors.New("environment result")
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		for _, result := range []error{nil, sentinel} {
			value := "configured"
			lookup := func(k string) (string, bool) {
				if k == key {
					return value, true
				}
				return "", false
			}
			_, err := resolveProxy(req, lookup, func(r *http.Request) (*url.URL, error) {
				if r != req {
					t.Fatal("request changed")
				}
				return nil, result
			}, func() systemProxyConfig { t.Fatal("system lookup with env set"); return systemProxyConfig{} })
			if err != result {
				t.Fatalf("%s=%q: %v", key, value, err)
			}
		}
	}
	absent := func(string) (string, bool) { return "", false }
	got, err := resolveProxy(req, absent, nil, func() systemProxyConfig { return parseWindowsProxy(true, "proxy.test:80", "") })
	if err != nil || proxyString(got) != "http://proxy.test:80" {
		t.Fatalf("system: %v %v", got, err)
	}
	got, err = resolveProxy(req, absent, nil, func() systemProxyConfig { return systemProxyConfig{} })
	if err != nil || got != nil {
		t.Fatalf("direct: %v %v", got, err)
	}
	for _, host := range []string{"localhost", "127.0.0.1", "[::1]"} {
		r, _ := http.NewRequest("GET", "http://"+host, nil)
		got, err := resolveProxy(r, nil, nil, nil)
		if got != nil || err != nil {
			t.Fatalf("loopback proxied: %s", host)
		}
	}
}

func TestSystemProxyEnvironmentBypass(t *testing.T) {
	for _, tc := range []struct {
		name, rawURL string
		env          map[string]string
		want         string
	}{
		{"empty proxies", "https://github.com", map[string]string{"HTTP_PROXY": "", "HTTPS_PROXY": "", "http_proxy": "", "https_proxy": ""}, "http://proxy.test:80"},
		{"ALL_PROXY ignored", "https://github.com", map[string]string{"ALL_PROXY": "http://other.test:80", "all_proxy": "http://other.test:80"}, "http://proxy.test:80"},
		{"unmatched NO_PROXY", "https://github.com", map[string]string{"NO_PROXY": "dev.test"}, "http://proxy.test:80"},
		{"matched NO_PROXY", "https://github.com", map[string]string{"NO_PROXY": "github.com"}, ""},
		{"lowercase bypass", "https://github.com", map[string]string{"no_proxy": "github.com"}, ""},
		{"empty uppercase fallback", "https://github.com", map[string]string{"NO_PROXY": "", "no_proxy": "github.com"}, ""},
		{"uppercase precedence", "https://github.com", map[string]string{"NO_PROXY": "dev.test", "no_proxy": "github.com"}, "http://proxy.test:80"},
		{"domain bypass", "https://assets.example.com", map[string]string{"NO_PROXY": ".example.com"}, ""},
		{"CIDR bypass", "https://10.2.3.4", map[string]string{"NO_PROXY": "10.0.0.0/8"}, ""},
		{"port bypass", "https://github.com:443", map[string]string{"NO_PROXY": "github.com:443"}, ""},
		{"different port", "https://github.com:8443", map[string]string{"NO_PROXY": "github.com:443"}, "http://proxy.test:80"},
		{"wildcard bypass", "https://github.com", map[string]string{"NO_PROXY": "*"}, ""},
		{"OS bypass retained", "https://internal.test", map[string]string{"NO_PROXY": "github.com"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tc.rawURL, nil)
			lookup := func(k string) (string, bool) { v, ok := tc.env[k]; return v, ok }
			got, err := resolveProxy(req, lookup, func(*http.Request) (*url.URL, error) { t.Fatal("environment proxy selected"); return nil, nil }, func() systemProxyConfig { return parseWindowsProxy(true, "proxy.test:80", "internal.test") })
			if err != nil || proxyString(got) != tc.want {
				t.Fatalf("got %v, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestSystemProxyCache(t *testing.T) {
	now := time.Unix(100, 0)
	var calls atomic.Int32
	cache := newSystemProxyCache(func() systemProxyConfig { calls.Add(1); return systemProxyConfig{} }, func() time.Time { return now })
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); cache() }()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("lookups = %d", calls.Load())
	}
	now = now.Add(8 * time.Second)
	cache()
	if calls.Load() != 2 {
		t.Fatalf("lookups after expiry = %d", calls.Load())
	}
}

func TestDownloadThroughSystemProxy(t *testing.T) {
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	var requests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.String() != "http://download.invalid/file.zip" {
			t.Errorf("proxy target = %s", r.URL)
		}
		w.Header().Set("Content-Disposition", "attachment; filename=file.zip")
		io.WriteString(w, "download through system proxy")
	}))
	defer proxy.Close()
	previous := cachedSystemProxy
	cachedSystemProxy = func() systemProxyConfig { return parseWindowsProxy(true, strings.TrimPrefix(proxy.URL, "http://"), "") }
	t.Cleanup(func() { cachedSystemProxy = previous })
	q, _, _ := openTestQueue(t)
	if err := q.Add("http://download.invalid/file.zip"); err != nil {
		t.Fatal(err)
	}
	job := waitForState(t, q, 0, Completed)
	assertOutput(t, q, job.ID, []byte("download through system proxy"))
	if requests.Load() == 0 {
		t.Fatal("proxy was not used")
	}
}

type failingProxyTransport struct{}

func (failingProxyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host == "github.com" {
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://release-assets.githubusercontent.com/file?signature=private"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	}
	return nil, io.EOF
}

func TestRequestErrorNamesRedirectHost(t *testing.T) {
	q, _, _ := openTestQueue(t)
	q.client.Transport = failingProxyTransport{}
	_, err := q.request(context.Background(), "https://github.com/file", "0-0", "")
	if err == nil || err.Error() != "request to release-assets.githubusercontent.com failed: EOF" {
		t.Fatalf("error = %v", err)
	}
	if !errors.Is(err, io.EOF) {
		t.Fatal("error cause lost")
	}
}
