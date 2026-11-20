package download

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBrowserFilenameHint(t *testing.T) {
	for _, tc := range []struct{ hint, disposition, want string }{
		{"/home/user/Downloads/browser.zip", "", "browser.zip"},
		{`C:\Users\user\Downloads\browser.zip`, "", "browser.zip"},
		{`C:\Downloads/mixed/bad?.zip`, "", "bad_.zip"},
		{"browser.zip", `attachment; filename="server.zip"`, "server.zip"},
		{"", "", "url-name.zip"},
	} {
		t.Run(tc.hint+tc.disposition, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.disposition != "" {
					w.Header().Set("Content-Disposition", tc.disposition)
				}
				http.ServeContent(w, r, "url-name.zip", time.Unix(100, 0), bytes.NewReader([]byte("hinted file")))
			}))
			defer server.Close()
			q, _, _ := openTestQueue(t)
			if err := q.AddWithOptions(server.URL+"/url-name.zip", DefaultQueue, RequestOptions{Filename: tc.hint}); err != nil {
				t.Fatal(err)
			}
			job := waitForState(t, q, 0, Completed)
			if job.Name != tc.want {
				t.Fatalf("name = %q, want %q", job.Name, tc.want)
			}
		})
	}
}
