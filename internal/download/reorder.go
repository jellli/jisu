package download

import (
	"errors"
	"slices"
)

// MoveJobs moves selected Jobs one place in the given direction, preserving
// their relative order. It changes scheduling order without interrupting transfers.
func (q *Queue) MoveJobs(ids []string, up bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return errors.New("download queue is closed")
	}
	selected := make(map[string]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}
	old := q.config.Jobs
	next := slices.Clone(old)
	move := func(i, j int) {
		if selected[next[i].ID] && !selected[next[j].ID] {
			next[i], next[j] = next[j], next[i]
		}
	}
	if up {
		for i := 1; i < len(next); i++ {
			move(i, i-1)
		}
	} else {
		for i := len(next) - 2; i >= 0; i-- {
			move(i, i+1)
		}
	}
	if slices.Equal(old, next) {
		return nil
	}
	q.config.Jobs = next
	if err := q.saveLocked(); err != nil {
		q.config.Jobs = old
		return err
	}
	q.scheduleLocked()
	return nil
}
