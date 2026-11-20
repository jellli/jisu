package takeover

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func ReadFrame(r io.Reader, v any) error {
	var n uint32
	if err := binary.Read(r, binary.NativeEndian, &n); err != nil {
		return err
	}
	if n > 1<<20 {
		return errors.New("native message too large")
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func WriteFrame(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err = binary.Write(w, binary.NativeEndian, uint32(len(b))); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func RunHost(r io.Reader, w io.Writer, config string, start func() error) error {
	for {
		var m Message
		if err := ReadFrame(r, &m); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		reply := forward(config, m, start)
		if err := WriteFrame(w, reply); err != nil {
			return err
		}
	}
}

func forward(config string, m Message, start func() error) Reply {
	deadline := time.Now().Add(10 * time.Second)
	started := false
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer client.CloseIdleConnections()
	for {
		data, err := os.ReadFile(filepath.Join(config, "jisu", "takeover.json"))
		var d discovery
		if err == nil {
			err = json.Unmarshal(data, &d)
		}
		if err == nil && d.Port > 0 && d.Port <= 65535 && len(d.Token) == 64 {
			body, _ := json.Marshal(m)
			req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/", d.Port), bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+d.Token)
			resp, postErr := client.Do(req)
			if postErr == nil {
				var reply Reply
				err = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&reply)
				resp.Body.Close()
				if err != nil {
					return Reply{Error: "invalid app response"}
				}
				return reply
			}
		}
		if !started {
			if start() == nil {
				started = true
			} else {
				return Reply{Error: "could not start jisu"}
			}
		}
		if time.Now().After(deadline) {
			return Reply{Error: "jisu is unavailable"}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func launchCommand(platform, executable string) *exec.Cmd {
	if platform == "darwin" {
		macOS := filepath.Dir(executable)
		contents := filepath.Dir(macOS)
		bundle := filepath.Dir(contents)
		if filepath.Base(macOS) == "MacOS" && filepath.Base(contents) == "Contents" && strings.HasSuffix(bundle, ".app") {
			return exec.Command("/usr/bin/open", bundle)
		}
	}
	return exec.Command(executable)
}

func StartApp() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := launchCommand(runtime.GOOS, executable)
	if err = startDetached(cmd); err != nil {
		return err
	}
	return cmd.Process.Release()
}
