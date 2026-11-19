package main

import (
	"slices"
	"testing"
	"time"

	"github.com/jellli/jisu/internal/download"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestSegmentFillShowsNoncontiguousBytes(t *testing.T) {
	got := segmentFill(160, []download.ByteSpan{{Start: 0, End: 14}, {Start: 80, End: 87}, {Start: 87, End: 94}})
	want := [segmentCount]float32{0: 1, 1: 0.4, 8: 1, 9: 0.4}
	if got != want {
		t.Fatalf("segment coverage = %v, want %v", got, want)
	}
}

func TestTaskTable(t *testing.T) {
	q, closeDemo, err := demoQueue(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDemo)
	app := nsgotest.Start(t, taskTable(q), nsgotest.Size(1040, 570), nsgotest.Idiom(core.IdiomDesktop),
		nsgotest.Environment(core.Environment{OS: core.OSLinux}), nsgotest.Measure())
	second := "text:Design Assets.zip"
	third := "text:Workshop Recording.mp4"
	app.WaitFor(second, 5*time.Second)
	if nsgotest.Target() != "" {
		app.WaitFor("id:tasks >> text:/^[1-9][0-9]%$/ >> nth=0", 12*time.Second)
		app.Screenshot("download-tasks-live")
	}

	app.Press(second)
	app.PressWith(third, core.Modifiers{Shift: true})
	app.WaitFor("text:2 selected", time.Second)
	app.RightClick(second)
	app.ChoosePopup("Move up")

	state := q.State()
	got := []string{state.Jobs[0].Name, state.Jobs[1].Name, state.Jobs[2].Name}
	want := []string{samples[1].name, samples[2].name, samples[0].name}
	if !slices.Equal(got, want) {
		t.Fatalf("queue order after move up: %v, want %v", got, want)
	}
	app.WaitFor("text:2 selected", time.Second)

	if nsgotest.Target() != "" {
		app.Screenshot("download-tasks-reordered")
		size := "role:columnheader Size"
		before := app.Find(size).Bounds()
		app.Drag("id:resize:size", core.Point{X: 12}, core.Point{X: 35})
		if after := app.Find(size).Bounds(); after.Size.Width <= before.Size.Width {
			t.Fatalf("column width did not increase: before %v, after %v", before, after)
		}
		app.Drag(size, core.Point{X: -20}, core.Point{X: -230})
		if after := app.Find(size).Bounds(); after.Origin.X >= app.Find("role:columnheader File name").Bounds().Origin.X {
			t.Fatalf("Size column did not move ahead of File name: %v", after)
		}
		app.Screenshot("download-tasks-columns")
	}
}
