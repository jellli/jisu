package main

import (
	"math"
	"testing"
)

func TestRangeLayoutPreviewsPreserveExactRetainedBytes(t *testing.T) {
	task := fixtures()[0]
	before := task.segments
	for _, count := range []int{4, 8, 16, 32} {
		task.previewCount = count
		ranges := task.ranges()
		if len(ranges) != count {
			t.Fatalf("range count %d, want %d", len(ranges), count)
		}
		var received float64
		for n, r := range ranges {
			if r.fraction < 0 || r.fraction > 1 {
				t.Fatal("invalid fraction")
			}
			if n > 0 && ranges[n-1].end+1 != r.start {
				t.Fatal("byte ranges overlap or leave gaps")
			}
			received += r.received
		}
		if math.Abs(received-task.size*1e6*task.progress()) > .01 {
			t.Fatal("layout preview lost retained bytes")
		}
		if ranges[0].start != 0 || ranges[len(ranges)-1].end != 6099999999 {
			t.Fatal("incorrect file boundaries")
		}
		if task.segments != before {
			t.Fatal("layout preview changed original workers")
		}
	}
}

func TestHTTPOnlyBatchAddIsAtomic(t *testing.T) {
	s := newPrototype()
	s.newURLs = "https://example.com/a.zip sftp://example.com/b.zip"
	if s.addTasks() || len(s.tasks) != 3 {
		t.Fatal("unsupported protocol partially added a batch")
	}
	s.newURLs = "http://example.com/a.zip https://example.com/b.iso"
	if !s.addTasks() || len(s.tasks) != 5 || s.selected != 3 {
		t.Fatal("valid HTTP batch not added/selected")
	}
	if s.tasks[3].protocol() != "HTTP" || s.tasks[4].protocol() != "HTTPS" {
		t.Fatal("protocol badges misrepresent links")
	}
	if s.tasks[3].progress() != 0 || s.tasks[3].state != paused {
		t.Fatal("new mock unexpectedly started")
	}
}

func TestCategorySearchSortAndBatchActions(t *testing.T) {
	s := newPrototype()
	s.category = "iso"
	if len(s.visibleTasks()) != 2 {
		t.Fatal("installation image category mismatch")
	}
	s.category = ""
	s.query = "blender"
	s.reconcileSelection()
	if s.selected != 1 {
		t.Fatal("search left hidden inspector selection")
	}
	s.query = ""
	s.sortBy = "size"
	if got := s.visibleTasks(); got[0] != 2 || got[2] != 0 {
		t.Fatal("size sort mismatch")
	}
	s.checked = map[int]bool{0: true, 1: true}
	s.setAll(paused, true)
	if s.tasks[0].state != paused || s.tasks[2].state != completed {
		t.Fatal("batch pause changed completed task")
	}
	s.setAll(downloading, true)
	if s.tasks[0].state != downloading || s.tasks[1].state != downloading {
		t.Fatal("batch resume failed")
	}
	s.removeChecked()
	s.reconcileSelection()
	if len(s.tasks) != 1 || s.selected != 0 || s.tasks[0].state != completed {
		t.Fatal("mock removal left stale selection")
	}
	s.reset()
	if len(s.tasks) != 3 || s.query != "" || s.category != "" {
		t.Fatal("reset did not restore queue")
	}
}
