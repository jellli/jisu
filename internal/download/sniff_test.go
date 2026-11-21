package download

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestSniffPageDirectLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/landing/index.html", http.StatusFound)
		case "/landing/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, `<!doctype html>
				<base href="../files/"><base href="/ignored/">
				<a href="Release%20Files.ZIP?download=1&amp;key=2#preview">Archive</a>
				<a href="./Release%20Files.ZIP?download=1&amp;key=2#duplicate">Duplicate</a>
				<a href="/setup.msi">Installer</a>
				<video src="trailer.mp4"><source src="manifest.M3U8?download=1"></video>
				<audio><source src="track.ogg"></audio>
				<img src="cover.png">
				<object data="../manual.pdf"></object>
				<a href="//cdn.example.test/package.tar.gz">External archive</a>
				<a href="playlist.mpd">DASH</a><a href="other.html">Another page</a>
				<a href="file:///local.zip">Unsupported scheme</a>
				<a href="https://user:password@example.test/private.zip">Embedded credentials</a>
				<!-- <img src="comment.png"> -->
				<script>const fake = '<video src="script.mp4">';</script>`)
		default:
			t.Errorf("scan fetched a resource: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	q, err := Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()

	got, err := q.SniffPage(context.Background(), " "+server.URL+"/start ")
	if err != nil {
		t.Fatal(err)
	}
	want := []PageResource{
		{server.URL + "/files/Release%20Files.ZIP?download=1&key=2", "Release Files.ZIP", ".zip"},
		{server.URL + "/setup.msi", "setup.msi", ".msi"},
		{server.URL + "/files/trailer.mp4", "trailer.mp4", ".mp4"},
		{server.URL + "/files/track.ogg", "track.ogg", ".ogg"},
		{server.URL + "/files/cover.png", "cover.png", ".png"},
		{server.URL + "/manual.pdf", "manual.pdf", ".pdf"},
		{"http://cdn.example.test/package.tar.gz", "package.tar.gz", ".gz"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
}

func TestSniffPageUsesCustomProxy(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.String() != "http://page.invalid/folder/index.html" {
			t.Errorf("unexpected proxy request: %s %s", r.Method, r.URL)
		}
		io.WriteString(w, `<a href="document.pdf">Download</a>`)
	}))
	defer proxy.Close()
	dir := t.TempDir()
	q, err := Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	if err := q.SetProxy(ProxySettings{Mode: "custom", URL: proxy.URL}); err != nil {
		t.Fatal(err)
	}
	got, err := q.SniffPage(context.Background(), "http://page.invalid/folder/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if want := []PageResource{{"http://page.invalid/folder/document.pdf", "document.pdf", ".pdf"}}; !slices.Equal(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
}

func TestSniffPageDecodesGBKLinks(t *testing.T) {
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(`<a href="资料.zip">下载</a>`))
	if err != nil {
		t.Fatal(err)
	}
	ambiguousGBK, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(`<a href="茅.zip">Download</a>`))
	if err != nil || !utf8.Valid(ambiguousGBK) {
		t.Fatalf("expected GBK fixture to also be valid UTF-8, got %x, error %v", ambiguousGBK, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/folder/header":
			w.Header().Set("Content-Type", "text/html; charset=GBK")
		default:
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, `<meta charset="GBK">`)
		}
		if r.URL.Path == "/folder/meta-utf8-valid" {
			w.Write(ambiguousGBK)
		} else {
			w.Write(gbk)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	q, err := Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	for _, tc := range []struct{ source, name, escaped string }{
		{"header", "资料.zip", "%E8%B5%84%E6%96%99.zip"},
		{"meta", "资料.zip", "%E8%B5%84%E6%96%99.zip"},
		{"meta-utf8-valid", "茅.zip", "%E8%8C%85.zip"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			got, err := q.SniffPage(context.Background(), server.URL+"/folder/"+tc.source)
			if err != nil {
				t.Fatal(err)
			}
			want := []PageResource{{server.URL + "/folder/" + tc.escaped, tc.name, ".zip"}}
			if !slices.Equal(got, want) {
				t.Fatalf("got %+v; want %+v", got, want)
			}
		})
	}
}

func TestSniffPagePreservesUndeclaredUTF8Links(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<!--"+strings.Repeat("a", 2048)+"-->"+`<a href="资料.zip">下载</a>`)
	}))
	defer server.Close()
	dir := t.TempDir()
	q, err := Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()

	got, err := q.SniffPage(context.Background(), server.URL+"/folder/index.html")
	if err != nil {
		t.Fatal(err)
	}
	want := []PageResource{{server.URL + "/folder/%E8%B5%84%E6%96%99.zip", "资料.zip", ".zip"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
}

func TestSniffPageResponseHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/empty":
			io.WriteString(w, "<html><a href='/about'>About</a></html>")
		case "/empty-body":
			w.Header().Set("Content-Type", "text/html")
		case "/pdf":
			w.Header().Set("Content-Type", "application/pdf")
		case "/large":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, strings.Repeat(" ", maxPageBytes+1))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	q, err := Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	for _, tc := range []struct{ path, errorText string }{
		{"/empty", ""}, {"/empty-body", ""}, {"/pdf", "HTML page"}, {"/large", "4 MiB"}, {"/missing", "HTTP 404"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := q.SniffPage(context.Background(), server.URL+tc.path)
			if tc.errorText == "" {
				if err != nil || len(got) != 0 {
					t.Fatalf("got resources %+v, error %v; want empty results", got, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.errorText) {
				t.Fatalf("got error %v; want %q", err, tc.errorText)
			}
		})
	}
}
