package main

import (
	"math"
	"testing"
	"time"
)

func TestComparisonFixturesAndPause(t *testing.T) {
	tasks := fixtures()
	for n, want := range []float64{.63, .51, 1} {
		if math.Abs(tasks[n].progress()-want) > 1e-9 {
			t.Fatalf("row %d progress = %f, want %f", n, tasks[n].progress(), want)
		}
	}
	if math.Abs(tasks[0].speed()-18.4) > 1e-9 || tasks[1].speed() != 0 || tasks[2].speed() != 0 {
		t.Fatal("fixture transfer rates differ from PR #4")
	}
	if tasks[0].remaining() != "约 4 分钟" {
		t.Fatalf("ETA = %q", tasks[0].remaining())
	}
	tasks[0].toggle()
	before := tasks[0].segments
	tasks[0].advance(time.Hour)
	if tasks[0].segments != before || tasks[0].speed() != 0 {
		t.Fatal("paused transfer advanced")
	}
	tasks[0].toggle()
	tasks[0].advance(time.Second)
	if tasks[0].segments[1] <= before[1] || tasks[0].segments[0] != 1 {
		t.Fatal("resume did not advance incomplete connections independently")
	}
	tasks[0].advance(time.Hour)
	if tasks[0].state != completed || tasks[0].progress() != 1 || tasks[0].speed() != 0 {
		t.Fatal("completed transfer not clamped or still has active rates")
	}
	if fixtures()[0].segments != before {
		t.Fatal("simulation mutated the reset fixture")
	}
}

func TestSlowestConnectionETAAndCompletion(t *testing.T) {
	task := fixtures()[0]
	// Range 03 needs 212.17 seconds; aggregate throughput predicts only 123.
	task.advance(123 * time.Second)
	if task.state == completed || task.remaining() != "约 2 分钟" {
		t.Fatalf("aggregate ETA incorrectly used: %s, %s", task.state, task.remaining())
	}
	task.advance(89 * time.Second)
	if task.state == completed || task.remaining() != "约 10 秒" {
		t.Fatal("last incomplete connection must still determine ETA")
	}
	task.advance(time.Second)
	if task.state != completed {
		t.Fatal("all byte ranges finished without marking completion")
	}
}

func TestFailureStaysVisibleAndRetryKeepsBytes(t *testing.T) {
	s := newPrototype()
	s.filter = activeTasks
	s.scenario(failed)
	task := &s.tasks[0]
	before := task.segments
	if !s.filter.matches(task) || s.count(attentionTasks) != 1 || s.count(activeTasks) != 1 {
		t.Fatal("failed active transfer disappeared from queue or attention count")
	}
	if task.failure == "" || task.connectionState(2) != "请求超时" || task.connectionState(0) != "已完成" {
		t.Fatal("failure does not identify the timed-out connection")
	}
	task.advance(time.Hour)
	if task.segments != before || task.speed() != 0 {
		t.Fatal("failed transfer continued receiving")
	}
	task.toggle()
	if task.segments != before || task.state != downloading || task.failure != "" || s.count(attentionTasks) != 0 {
		t.Fatal("retry lost partial bytes or left a stale error")
	}
	task.advance(time.Second)
	if task.segments[2] <= before[2] || task.segments[0] != 1 {
		t.Fatal("retry did not resume only incomplete ranges")
	}
}

func TestFiltersResetAndSimulationDoNotCatchUpFrozenTime(t *testing.T) {
	s := newPrototype()
	for i, want := range []int{3, 1, 1, 0, 1} {
		if got := s.count(queueFilter(i)); got != want {
			t.Fatalf("filter %d count %d, want %d", i, got, want)
		}
	}
	now := time.Unix(1, 0)
	s.tick(now)
	s.simulate = true
	s.tick(now.Add(time.Hour))
	before := s.tasks[0].segments
	if before != fixtures()[0].segments {
		t.Fatal("enabling simulation caught up frozen time")
	}
	s.tick(now.Add(time.Hour + time.Second))
	if s.tasks[0].segments[2] <= before[2] || s.tasks[1].segments != fixtures()[1].segments {
		t.Fatal("simulation did not respect active and paused states")
	}
	s.simulate = false
	s.tick(now.Add(2 * time.Hour))
	before = s.tasks[0].segments
	s.simulate = true
	s.tick(now.Add(3 * time.Hour))
	if s.tasks[0].segments != before {
		t.Fatal("simulation caught up after a frozen interval")
	}
	s.filter, s.dark, s.settings = attentionTasks, true, true
	s.reset()
	if s.filter != allTasks || s.tasks[0].segments != fixtures()[0].segments || !s.simulate || !s.dark || !s.settings {
		t.Fatal("reset did not restore fixtures while preserving demo preferences")
	}
}

func TestInspectorSelectionFollowsVisibleQueue(t *testing.T) {
	s := newPrototype()
	if s.selected != 0 || !s.detailsOpen {
		t.Fatal("fresh comparison must inspect the first task")
	}
	s.filter = pausedTasks
	s.reconcileSelection()
	if s.selected != 1 {
		t.Fatal("inspector still shows a task hidden by the paused filter")
	}
	s.filter = attentionTasks
	s.reconcileSelection()
	if s.selected != -1 {
		t.Fatal("empty filtered queue retained a stale inspector")
	}
	s.scenario(failed)
	s.reconcileSelection()
	if s.selected != 0 {
		t.Fatal("failure did not become selectable in needs attention")
	}
	s.tasks[0].toggle()
	s.reconcileSelection()
	if s.selected != -1 {
		t.Fatal("retried task remains selected after leaving the attention filter")
	}
	s.detailsOpen = false
	s.reset()
	if s.selected != 0 || !s.detailsOpen {
		t.Fatal("reset did not restore the initial inspector")
	}
}
