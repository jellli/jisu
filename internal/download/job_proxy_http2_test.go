package download

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func connectTestProxy(t *testing.T, connects *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}
		upstream, err := net.Dial("tcp", r.Host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		client, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			upstream.Close()
			return
		}
		connects.Add(1)
		buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		buffered.Flush()
		go func() {
			defer upstream.Close()
			defer client.Close()
			io.Copy(upstream, buffered)
		}()
		go func() {
			defer upstream.Close()
			defer client.Close()
			io.Copy(client, upstream)
		}()
	}))
}

func TestJobProxyHTTP2RouteIsolation(t *testing.T) {
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 2 {
			t.Errorf("request protocol = %s, want HTTP/2", r.Proto)
		}
		w.Write([]byte("payload"))
	}))
	origin.EnableHTTP2 = true
	origin.StartTLS()
	defer origin.Close()
	var globalConnects, jobConnects atomic.Int32
	global := connectTestProxy(t, &globalConnects)
	defer global.Close()
	override := connectTestProxy(t, &jobConnects)
	defer override.Close()
	q, id := openJobProxyQueue(t)
	defer q.Close()
	q.client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	if err := q.SetProxy(ProxySettings{Mode: "custom", URL: global.URL}); err != nil {
		t.Fatal(err)
	}
	if err := q.SetJobProxy(id, ProxySettings{Mode: "custom", URL: override.URL}); err != nil {
		t.Fatal(err)
	}
	request := func(ctx context.Context) {
		t.Helper()
		resp, err := q.requestOnce(ctx, origin.URL, "", "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	request(context.Background())
	ctx := context.WithValue(context.Background(), jobProxyKey{}, id)
	request(ctx)
	if globalConnects.Load() != 1 || jobConnects.Load() != 1 {
		t.Fatalf("CONNECT counts: global=%d job=%d, want one connection per route", globalConnects.Load(), jobConnects.Load())
	}
	request(ctx)
	request(context.Background())
	if globalConnects.Load() != 1 || jobConnects.Load() != 1 {
		t.Fatalf("route pools not reused: global=%d job=%d", globalConnects.Load(), jobConnects.Load())
	}
	if err := q.SetJobProxy(id, ProxySettings{Mode: "none"}); err != nil {
		t.Fatal(err)
	}
	q.mu.Lock()
	remaining := len(q.jobTransports)
	q.mu.Unlock()
	if remaining != 0 {
		t.Fatal("unused override transport was not retired")
	}
	request(ctx)
	if globalConnects.Load() != 1 || jobConnects.Load() != 1 {
		t.Fatal("direct override used a proxy connection")
	}
	if err := q.SetJobProxy(id, ProxySettings{Mode: "custom", URL: override.URL}); err != nil {
		t.Fatal(err)
	}
	request(ctx)
	if jobConnects.Load() != 2 {
		t.Fatalf("retired connection reused: job CONNECTs=%d", jobConnects.Load())
	}
	if err := q.Remove(id); err != nil {
		t.Fatal(err)
	}
	q.mu.Lock()
	remaining = len(q.jobTransports)
	q.mu.Unlock()
	if remaining != 0 {
		t.Fatal("removing job retained its transport")
	}
}
