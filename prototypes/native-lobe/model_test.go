package main

import (
	"math"
	"testing"
	"time"
)

func closeTo(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}

func TestSampleDataAndAggregateSpeed(t *testing.T) {
	d := newDemo()
	closeTo(t, d.Tasks[0].progress(), .63)
	closeTo(t, d.Tasks[1].progress(), .51)
	closeTo(t, d.Tasks[2].progress(), 1)
	closeTo(t, d.totalSpeed(), 18.4)
	if d.count(downloading) != 1 || d.count(paused) != 1 || d.count(completed) != 1 {
		t.Fatal("initial state counts differ from the web prototype")
	}
	if got := d.Tasks[0].remaining(); got != "约 2 分 3 秒" {
		t.Fatalf("remaining = %q", got)
	}
	other := sampleTasks()
	d.Tasks[0].Segments[0] = 0
	if other[0].Segments[0] != 1 {
		t.Fatal("sample segment arrays are shared")
	}
}

func TestPauseResumeRetryPreservePartialBytes(t *testing.T) {
	task := sampleTasks()[0]
	initial := task.Segments
	task.activate()
	task.advance(30 * time.Second)
	if task.State != paused || task.Segments != initial || task.speed() != 0 {
		t.Fatal("paused transfer changed")
	}
	task.activate()
	task.advance(2 * time.Second)
	closeTo(t, task.Segments[1], .78+2.8*2/(6100.0/8))
	if task.Segments[0] != 1 {
		t.Fatal("completed connection changed")
	}
	task.State = failed
	before := task.Segments
	task.advance(time.Hour)
	if task.speed() != 0 || task.Segments != before {
		t.Fatal("failed transfer continued downloading")
	}
	task.activate()
	if task.State != downloading || task.Segments != before {
		t.Fatal("retry discarded partial bytes")
	}
}

func TestCompletionClampsAllSegments(t *testing.T) {
	task := sampleTasks()[0]
	task.advance(time.Hour)
	if task.State != completed || task.progress() != 1 || task.speed() != 0 {
		t.Fatalf("completion state: %+v", task)
	}
	for _, value := range task.Segments {
		if value != 1 {
			t.Fatal("completion overflowed or left an unfinished segment")
		}
	}
	task.activate()
	if !task.Expanded || task.State != completed {
		t.Fatal("completed row action did not open details")
	}
	before := task.Segments
	task.advance(-time.Second)
	if task.Segments != before {
		t.Fatal("negative duration changed progress")
	}
}

func TestSimulationDoesNotCatchUpFrozenTime(t *testing.T) {
	d := newDemo()
	now := time.Unix(1000, 0)
	d.tick(now)
	d.tick(now.Add(time.Hour))
	if d.Tasks[0].Segments != initialSegments {
		t.Fatal("frozen fixture advanced")
	}
	d.Simulate = true
	d.tick(now.Add(2 * time.Hour))
	if d.Tasks[0].Segments != initialSegments {
		t.Fatal("enabling simulation caught up frozen time")
	}
	d.tick(now.Add(2*time.Hour + time.Second))
	if d.Tasks[0].Segments == initialSegments {
		t.Fatal("simulation did not advance")
	}
	if d.Tasks[1].State != paused || d.Tasks[1].progress() != .51 {
		t.Fatal("simulation changed a paused task")
	}
}

func TestScenariosResetAndFilter(t *testing.T) {
	d := newDemo()
	for _, state := range []transferState{paused, failed, completed, downloading} {
		d.setScenario(string(state))
		if d.Tasks[0].State != state || d.Scenario != string(state) {
			t.Fatalf("scenario %s not applied", state)
		}
	}
	d.setScenario("unknown")
	if d.Tasks[0].State != downloading {
		t.Fatal("invalid scenario was applied")
	}
	d.Filter = 2
	if d.visible(&d.Tasks[0]) || !d.visible(&d.Tasks[1]) {
		t.Fatal("paused filter shows the wrong task")
	}
	d.Tasks[0].Expanded = true
	d.setScenario(string(completed))
	d.reset()
	if d.Filter != 0 || d.Tasks[0].Expanded || d.Tasks[0].Segments != initialSegments {
		t.Fatal("reset did not restore the deterministic initial sample")
	}
	closeTo(t, d.totalSpeed(), 18.4)
}
