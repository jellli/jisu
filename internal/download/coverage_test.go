package download

import (
	"slices"
	"testing"
)

func TestStateWithCoverageUsesLiveRanges(t *testing.T) {
	total := int64(100)
	job := &record{
		Job:  Job{ID: "a", Total: &total, Received: 35},
		Plan: &plan{Ranges: []byteRange{{0, 10, 49}, {50, 50, 99}}},
	}
	q := &Queue{
		config: config{Jobs: []*record{job}},
		active: map[string]*attempt{"a": {ranges: []*liveRange{
			{byteRange: byteRange{0, 20, 49}},
			{byteRange: byteRange{50, 65, 74}},
			{byteRange: byteRange{75, 75, 99}},
		}}},
	}
	state, coverage := q.StateWithCoverage()
	if state.Jobs[0].Received != 35 || !slices.Equal(coverage["a"], []ByteSpan{{0, 20}, {50, 65}}) {
		t.Fatalf("unexpected live coverage: %+v, %+v", state, coverage)
	}
	delete(q.active, "a")
	_, coverage = q.StateWithCoverage()
	if !slices.Equal(coverage["a"], []ByteSpan{{0, 10}}) {
		t.Fatalf("unexpected checkpoint coverage: %+v", coverage)
	}
}

func TestMoveJobsPersistsRelativeOrder(t *testing.T) {
	q, state, dir := openTestQueue(t)
	for _, id := range []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "cccccccccccccccccccccccccccccccc", "dddddddddddddddddddddddddddddddd"} {
		q.config.Jobs = append(q.config.Jobs, &record{Job: Job{ID: id, Directory: dir, Name: id[:1], State: Paused, Queue: DefaultQueue}})
	}
	ids := []string{q.config.Jobs[1].ID, q.config.Jobs[2].ID}
	names := func(q *Queue) string {
		var s string
		for _, job := range q.State().Jobs {
			s += job.Name
		}
		return s
	}
	if err := q.MoveJobs(ids, true); err != nil || names(q) != "bcad" {
		t.Fatalf("move up: %s, %v", names(q), err)
	}
	if err := q.MoveJobs(ids, false); err != nil || names(q) != "abcd" {
		t.Fatalf("move down: %s, %v", names(q), err)
	}
	if err := q.MoveJobs(ids, false); err != nil || names(q) != "adbc" {
		t.Fatalf("move down again: %s, %v", names(q), err)
	}
	reopened, err := Open(state, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := names(reopened); got != "adbc" {
		t.Fatalf("reopened order: %s", got)
	}
}
