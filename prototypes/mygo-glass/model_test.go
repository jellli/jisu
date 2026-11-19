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
	if tasks[0].remaining() != "约 2 分 3 秒" {
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
