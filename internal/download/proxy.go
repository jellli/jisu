package download

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type ProxySettings struct {
	Mode string `json:"mode"`
	URL  string `json:"url,omitempty"`
}

func normalizeProxy(p ProxySettings) (ProxySettings, error) {
	if p.Mode == "" {
		p.Mode = "system"
	}
	switch p.Mode {
	case "system", "none":
		p.URL = ""
		return p, nil
	case "custom":
	default:
		return ProxySettings{}, errors.New("select System, None or Custom proxy mode")
	}
	raw := strings.TrimSpace(p.URL)
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	invalid := errors.New("proxy URL must use http, https, socks5 or socks5h with a host and port (1-65535)")
	if err != nil {
		return ProxySettings{}, invalid
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return ProxySettings{}, invalid
	}
	port, err := strconv.Atoi(u.Port())
	_, _, hostErr := net.SplitHostPort(u.Host)
	if err != nil || hostErr != nil || port < 1 || port > 65535 || u.Hostname() == "" || strings.ContainsAny(u.Hostname(), " \t\r\n") || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return ProxySettings{}, invalid
	}
	p.URL = u.String()
	return p, nil
}

func (q *Queue) proxyForRequest(req *http.Request) (*url.URL, error) {
	p := q.proxy.Load().(ProxySettings)
	if id, ok := req.Context().Value(jobProxyKey{}).(string); ok {
		q.mu.Lock()
		if job := q.findLocked(id); job != nil && job.Proxy.Mode != "" {
			p = job.Proxy
		}
		q.mu.Unlock()
	}
	return proxyForSettings(req, p)
}

func proxyForSettings(req *http.Request, p ProxySettings) (*url.URL, error) {
	switch p.Mode {
	case "none":
		return nil, nil
	case "custom":
		return url.Parse(p.URL)
	default:
		return systemProxy(req)
	}
}

type jobProxyKey struct{}

func normalizeJobProxy(p ProxySettings) (ProxySettings, error) {
	if p.Mode == "" {
		return ProxySettings{}, nil
	}
	return normalizeProxy(p)
}

func (q *Queue) SetJobProxy(id string, p ProxySettings) error {
	p, err := normalizeJobProxy(p)
	if err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	job := q.findLocked(id)
	if q.closed || job == nil {
		return errors.New("download is unavailable")
	}
	old := job.Proxy
	job.Proxy = p
	if err := q.saveLocked(); err != nil {
		job.Proxy = old
		return err
	}
	q.client.CloseIdleConnections()
	q.pruneJobTransportsLocked()
	return nil
}

type jobProxyTransport struct {
	queue    *Queue
	fallback http.RoundTripper
}

func (t jobProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	q := t.queue
	id, _ := req.Context().Value(jobProxyKey{}).(string)
	q.mu.Lock()
	job := q.findLocked(id)
	base, ok := t.fallback.(*http.Transport)
	if job == nil || job.Proxy.Mode == "" || !ok || q.closed {
		q.mu.Unlock()
		return t.fallback.RoundTrip(req)
	}
	p := job.Proxy
	transport := q.jobTransports[p]
	if transport == nil {
		transport = base.Clone()
		transport.Proxy = func(req *http.Request) (*url.URL, error) { return proxyForSettings(req, p) }
		if q.jobTransports == nil {
			q.jobTransports = make(map[ProxySettings]*http.Transport)
		}
		q.jobTransports[p] = transport
	}
	q.mu.Unlock()
	cleanup := func() {
		q.mu.Lock()
		retired := q.jobTransports[p] != transport
		q.mu.Unlock()
		if retired {
			transport.CloseIdleConnections()
		}
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		cleanup()
		return nil, err
	}
	resp.Body = &jobProxyBody{ReadCloser: resp.Body, cleanup: cleanup}
	return resp, nil
}

type jobProxyBody struct {
	io.ReadCloser
	cleanup func()
}

func (b *jobProxyBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.cleanup()
	}
	return n, err
}

func (b *jobProxyBody) Close() error {
	err := b.ReadCloser.Close()
	b.cleanup()
	return err
}

func (q *Queue) pruneJobTransportsLocked() {
	for p, transport := range q.jobTransports {
		used := false
		if !q.closed {
			for _, job := range q.config.Jobs {
				if job.Proxy == p {
					used = true
					break
				}
			}
		}
		if !used {
			delete(q.jobTransports, p)
			transport.CloseIdleConnections()
		}
	}
}

func systemProxy(req *http.Request) (*url.URL, error) {
	return ProxyForRequest(req)
}

func (q *Queue) SetProxy(p ProxySettings) error {
	p, err := normalizeProxy(p)
	if err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	old := q.config.Proxy
	q.config.Proxy = p
	if err := q.saveLocked(); err != nil {
		q.config.Proxy = old
		return err
	}
	q.proxy.Store(p)
	q.client.CloseIdleConnections()
	return nil
}
