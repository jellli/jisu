package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"time"

	"github.com/jellli/jisu/internal/download"
)

var samples = []struct {
	name string
	size int64
}{
	{"Northern Lights.mov", 96 << 20},
	{"Design Assets.zip", 72 << 20},
	{"Workshop Recording.mp4", 48 << 20},
	{"Field Notes.pdf", 12 << 20},
	{"Sample Library.tar", 64 << 20},
	{"Kite Reference.png", 8 << 20},
	{"Archive 2025.zip", 32 << 20},
	{"Ambient Sessions.flac", 24 << 20},
}

func demoQueue(dir string) (*download.Queue, func(), error) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, sample := range samples {
			if r.URL.Path == "/"+sample.name {
				w.Header().Set("ETag", `"jisu-demo-v1"`)
				http.ServeContent(w, r, sample.name, time.Time{}, &sampleFile{size: sample.size})
				return
			}
		}
		http.NotFound(w, r)
	}))
	q, err := download.Open(filepath.Join(dir, "queue.json"), dir)
	if err != nil {
		server.Close()
		return nil, nil, err
	}
	closeDemo := func() { q.Close(); server.Close() }
	for _, sample := range samples {
		if err := q.Add(server.URL + "/" + url.PathEscape(sample.name)); err != nil {
			closeDemo()
			return nil, nil, err
		}
	}
	return q, closeDemo, nil
}

// sampleFile produces bytes on demand. Slower regions give the range
// workers time to finish at different rates and steal each other's work.
type sampleFile struct {
	pos, size int64
}

func (f *sampleFile) Read(p []byte) (int, error) {
	n := int(min(int64(len(p)), f.size-f.pos))
	if n <= 0 {
		return 0, io.EOF
	}
	if n > 1 {
		time.Sleep((3 + time.Duration((f.pos/(4<<20))%7)*3) * time.Millisecond)
	}
	for i := range p[:n] {
		p[i] = byte((f.pos + int64(i)) % 251)
	}
	f.pos += int64(n)
	return n, nil
}

func (f *sampleFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		offset += f.pos
	case io.SeekEnd:
		offset += f.size
	case io.SeekStart:
	default:
		return 0, fmt.Errorf("invalid seek origin: %d", whence)
	}
	if offset < 0 {
		return 0, fmt.Errorf("negative offset: %d", offset)
	}
	f.pos = offset
	return offset, nil
}
