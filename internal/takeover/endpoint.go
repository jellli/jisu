package takeover

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jellli/jisu/internal/download"
)

type Message struct {
	URL       string `json:"url"`
	Referrer  string `json:"referrer"`
	Cookies   string `json:"cookies"`
	UserAgent string `json:"userAgent"`
	Filename  string `json:"filename,omitempty"`
}

type Reply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type discovery struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

type Endpoint struct {
	server *http.Server
	file   string
	token  string
}

func atomicJSON(file string, v any) error {
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(file), ".takeover-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		err = json.NewEncoder(f).Encode(v)
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), file)
}

func Start(config string, q *download.Queue) (*Endpoint, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	var secret [32]byte
	if _, err = rand.Read(secret[:]); err != nil {
		listener.Close()
		return nil, err
	}
	d := discovery{listener.Addr().(*net.TCPAddr).Port, hex.EncodeToString(secret[:])}
	e := &Endpoint{file: filepath.Join(config, "jisu", "takeover.json"), token: d.Token}
	e.server = &http.Server{
		Handler:           handler(d.Token, q),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	if err = atomicJSON(e.file, d); err != nil {
		listener.Close()
		return nil, err
	}
	go e.server.Serve(listener)
	return e, nil
}

func (e *Endpoint) Close() error {
	err := e.server.Close()
	data, readErr := os.ReadFile(e.file)
	var current discovery
	if readErr == nil && json.Unmarshal(data, &current) == nil && current.Token == e.token {
		os.Remove(e.file)
	}
	return err
}

func handler(token string, q *download.Queue) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(Reply{Error: "unauthorized"})
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var m Message
		body, err := io.ReadAll(r.Body)
		if err == nil {
			err = json.Unmarshal(body, &m)
		}
		if err != nil {
			var large *http.MaxBytesError
			if errors.As(err, &large) {
				w.WriteHeader(413)
			} else {
				w.WriteHeader(400)
			}
			json.NewEncoder(w).Encode(Reply{Error: "invalid Takeover message"})
			return
		}
		err = q.AddWithOptions(m.URL, download.DefaultQueue, download.RequestOptions{
			Cookie:    m.Cookies,
			Referer:   m.Referrer,
			UserAgent: m.UserAgent,
			Filename:  m.Filename,
		})
		reply := Reply{OK: err == nil}
		if err != nil {
			reply.Error = "could not add Takeover Job"
		}
		json.NewEncoder(w).Encode(reply)
	})
}
