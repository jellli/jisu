package download

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	Queued      = "queued"
	Downloading = "downloading"
	Paused      = "paused"
	Failed      = "failed"
	Completed   = "completed"
)

type Job struct {
	ID             string  `json:"id"`
	URL            string  `json:"url"`
	Name           string  `json:"name"`
	Directory      string  `json:"directory"`
	State          string  `json:"state"`
	Error          string  `json:"error"`
	Total          *int64  `json:"total"`
	Received       int64   `json:"received"`
	Connections    int     `json:"connections"`
	BytesPerSecond float64 `json:"bytesPerSecond"`
}

type Snapshot struct {
	Jobs              []Job  `json:"jobs"`
	DownloadDirectory string `json:"downloadDirectory"`
	Concurrency       int    `json:"concurrency"`
}

type record struct {
	Job
	Plan      *plan  `json:"plan,omitempty"`
	FinalPath string `json:"finalPath,omitempty"`
}

type config struct {
	Jobs        []*record `json:"jobs"`
	Directory   string    `json:"directory"`
	Concurrency int       `json:"concurrency"`
}

type attempt struct {
	cancel  context.CancelFunc
	started time.Time
	base    int64
}

type Queue struct {
	mu      sync.Mutex
	workers sync.WaitGroup
	config  config
	active  map[string]*attempt
	file    string
	client  *http.Client
	closed  bool
}

func Open(stateFile, directory string) (*Queue, error) {
	q := &Queue{
		config: config{Directory: directory, Concurrency: 2},
		file:   stateFile, active: make(map[string]*attempt), client: newClient(),
	}
	body, err := os.ReadFile(stateFile)
	if err == nil {
		if err := json.Unmarshal(body, &q.config); err != nil {
			return nil, fmt.Errorf("read download queue: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if !filepath.IsAbs(q.config.Directory) || q.config.Concurrency < 1 || q.config.Concurrency > 5 {
		return nil, errors.New("invalid saved download settings")
	}
	for _, job := range q.config.Jobs {
		if job == nil || len(job.ID) != 32 || strings.Trim(job.ID, "0123456789abcdef") != "" || !filepath.IsAbs(job.Directory) {
			return nil, errors.New("invalid saved download")
		}
		job.Connections, job.BytesPerSecond = 0, 0
		if job.State == Downloading {
			job.State = Queued
		}
		if job.State == Completed {
			_ = os.RemoveAll(partialDirectory(job))
		}
	}
	q.mu.Lock()
	q.scheduleLocked()
	q.mu.Unlock()
	return q, nil
}

func (q *Queue) Close() {
	q.mu.Lock()
	q.closed = true
	for _, run := range q.active {
		run.cancel()
	}
	q.mu.Unlock()
	q.workers.Wait()
	q.client.CloseIdleConnections()
}

func (q *Queue) State() Snapshot {
	q.mu.Lock()
	defer q.mu.Unlock()
	state := Snapshot{Jobs: make([]Job, 0, len(q.config.Jobs)), DownloadDirectory: q.config.Directory, Concurrency: q.config.Concurrency}
	for _, job := range q.config.Jobs {
		view := job.Job
		if run := q.active[job.ID]; run != nil && view.State == Downloading {
			view.BytesPerSecond = float64(max(view.Received-run.base, 0)) / max(time.Since(run.started).Seconds(), 1)
		}
		state.Jobs = append(state.Jobs, view)
	}
	return state
}

func (q *Queue) Add(rawURL string) error {
	parsed, err := parseURL(strings.TrimSpace(rawURL))
	if err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return errors.New("download queue is closed")
	}
	var id [16]byte
	rand.Read(id[:])
	q.config.Jobs = append(q.config.Jobs, &record{Job: Job{
		ID: fmt.Sprintf("%x", id), URL: parsed.String(), Name: safeFilename(path.Base(parsed.Path)),
		Directory: q.config.Directory, State: Queued,
	}})
	if err := q.saveLocked(); err != nil {
		q.config.Jobs = q.config.Jobs[:len(q.config.Jobs)-1]
		return err
	}
	q.scheduleLocked()
	return nil
}

func (q *Queue) Pause(id string) error  { return q.changeState(id, Paused) }
func (q *Queue) Resume(id string) error { return q.changeState(id, Queued) }

func (q *Queue) changeState(id, state string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	job := q.findLocked(id)
	if job == nil || q.closed {
		return errors.New("download is unavailable")
	}
	if state == Paused && job.State != Downloading && job.State != Queued {
		return errors.New("only active or queued downloads can be paused")
	}
	if state == Queued && job.State != Paused && job.State != Failed {
		return errors.New("only paused or failed downloads can be resumed")
	}
	previous := job.Job
	job.State, job.Error, job.Connections = state, "", 0
	if err := q.saveLocked(); err != nil {
		job.Job = previous
		return err
	}
	if run := q.active[id]; run != nil && state == Paused {
		run.cancel()
	}
	q.scheduleLocked()
	return nil
}

func (q *Queue) Remove(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, job := range q.config.Jobs {
		if job.ID != id {
			continue
		}
		old := q.config.Jobs
		q.config.Jobs = append(append([]*record(nil), old[:i]...), old[i+1:]...)
		if err := q.saveLocked(); err != nil {
			q.config.Jobs = old
			return err
		}
		if run := q.active[id]; run != nil {
			run.cancel()
			return nil
		}
		return os.RemoveAll(partialDirectory(job))
	}
	return errors.New("download not found")
}

func (q *Queue) CompletedPath(id string) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	job := q.findLocked(id)
	if job == nil || job.State != Completed {
		return "", errors.New("download is not complete")
	}
	return job.FinalPath, nil
}

func (q *Queue) SetDirectory(directory string) error {
	if !filepath.IsAbs(directory) {
		return errors.New("choose an absolute download folder")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	old := q.config.Directory
	q.config.Directory = directory
	if err := q.saveLocked(); err != nil {
		q.config.Directory = old
		return err
	}
	return nil
}

func (q *Queue) SetConcurrency(value int) error {
	if value < 1 || value > 5 {
		return errors.New("select between 1 and 5 simultaneous downloads")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	old := q.config.Concurrency
	q.config.Concurrency = value
	if err := q.saveLocked(); err != nil {
		q.config.Concurrency = old
		return err
	}
	q.scheduleLocked()
	return nil
}

func (q *Queue) findLocked(id string) *record {
	for _, job := range q.config.Jobs {
		if job.ID == id {
			return job
		}
	}
	return nil
}

func (q *Queue) scheduleLocked() {
	if q.closed {
		return
	}
	for _, job := range q.config.Jobs {
		if len(q.active) >= q.config.Concurrency {
			return
		}
		if job.State != Queued || q.active[job.ID] != nil {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		run := &attempt{cancel: cancel, started: time.Now(), base: job.Received}
		job.State = Downloading
		q.active[job.ID] = run
		q.workers.Go(func() { q.run(ctx, run, job) })
	}
}

func (q *Queue) run(ctx context.Context, run *attempt, job *record) {
	defer run.cancel()
	err := q.transfer(ctx, run, job)
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.active, job.ID)
	if q.findLocked(job.ID) == nil {
		_ = os.RemoveAll(partialDirectory(job))
	} else if job.State == Downloading && !q.closed {
		job.Connections = 0
		if err != nil {
			job.State, job.Error = Failed, err.Error()
		}
		if err := q.saveLocked(); err != nil {
			job.State, job.Error = Failed, err.Error()
		}
	}
	q.scheduleLocked()
}

func (q *Queue) saveLocked() error {
	dir := filepath.Dir(q.file)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("save download queue: %w", err)
	}
	f, err := os.CreateTemp(dir, ".queue-*")
	if err != nil {
		return fmt.Errorf("save download queue: %w", err)
	}
	defer os.Remove(f.Name())
	if err := json.NewEncoder(f).Encode(q.config); err != nil {
		_ = f.Close()
		return fmt.Errorf("save download queue: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("save download queue: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("save download queue: %w", err)
	}
	if err := os.Rename(f.Name(), q.file); err != nil {
		return fmt.Errorf("save download queue: %w", err)
	}
	return nil
}

func parseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("enter a valid HTTP or HTTPS link without embedded credentials")
	}
	u.Fragment = ""
	return u, nil
}
