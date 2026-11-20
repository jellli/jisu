package download

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	DefaultQueue = "default"
	NightQueue   = "night"
)

func (q *Queue) hasQueueLocked(name string) bool {
	return slices.Contains(q.config.Queues, name)
}

func (q *Queue) CreateQueue(name string) error {
	if name == "" || strings.TrimSpace(name) != name || utf8.RuneCountInString(name) > 40 {
		return errors.New("enter a queue name of 1 to 40 characters")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.hasQueueLocked(name) {
		return errors.New("queue already exists or the download manager is closed")
	}
	old := q.config.Queues
	q.config.Queues = append(old, name)
	if err := q.saveLocked(); err != nil {
		q.config.Queues = old
		return err
	}
	return nil
}

func (q *Queue) MoveToQueue(ids []string, name string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || !q.hasQueueLocked(name) {
		return errors.New("queue is unavailable")
	}
	old := q.config.Jobs
	moved := make(map[*record]string)
	next := make([]*record, 0, len(old))
	var tail []*record
	for _, job := range old {
		if job.Queue != name && slices.Contains(ids, job.ID) {
			moved[job] = job.Queue
			job.Queue = name
			tail = append(tail, job)
		} else {
			next = append(next, job)
		}
	}
	if len(tail) == 0 {
		return nil
	}
	q.config.Jobs = append(next, tail...)
	if err := q.saveLocked(); err != nil {
		for job, previous := range moved {
			job.Queue = previous
		}
		q.config.Jobs = old
		return err
	}
	q.scheduleLocked()
	return nil
}

func (q *Queue) connectionTargetLocked(job *record) int {
	if job.ConnectionTarget != 0 {
		return job.ConnectionTarget
	}
	return q.config.Connections
}

// SetJobConnections changes one job without restarting its active transfer.
func (q *Queue) SetJobConnections(id string, n int) error {
	if !validConnections(n) {
		return errors.New("select 4, 8, 16 or 32 connections")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	job := q.findLocked(id)
	if q.closed || job == nil {
		return errors.New("download is unavailable")
	}
	old := job.ConnectionTarget
	job.ConnectionTarget = n
	if err := q.saveLocked(); err != nil {
		job.ConnectionTarget = old
		return err
	}
	if run := q.active[id]; run != nil {
		select {
		case run.changed <- struct{}{}:
		default:
		}
	}
	return nil
}
