package main

import (
	"slices"
	"testing"

	"github.com/jellli/jisu/internal/download"
	"github.com/nsgo-dev/nsgo/base/datatable"
	"github.com/nsgo-dev/nsgo/core"
)

func TestSortHeaderTitle(t *testing.T) {
	for _, tt := range []struct {
		order datatable.Sort
		want  string
	}{
		{datatable.Sort{}, "大小"},
		{datatable.Sort{Key: "name"}, "大小"},
		{datatable.Sort{Key: "name", Descending: true}, "大小"},
		{datatable.Sort{Key: "size"}, "大小 ↑"},
		{datatable.Sort{Key: "size", Descending: true}, "大小 ↓"},
	} {
		if got := sortHeaderTitle("大小", "size", tt.order); got != tt.want {
			t.Errorf("order %+v: got %q, want %q", tt.order, got, tt.want)
		}
	}
}

func TestSortJobs(t *testing.T) {
	small, large := int64(100), int64(1000)
	a := jobView{Job: download.Job{ID: "a", Name: "Alpha", Total: &small, Received: 10, BytesPerSecond: 1}}
	b := jobView{Job: download.Job{ID: "b", Name: "beta", Total: &large, Received: 500, BytesPerSecond: 2}}
	for _, key := range []string{"name", "size", "progress", "speed", "eta"} {
		for _, descending := range []bool{false, true} {
			jobs := []jobView{b, a}
			sortJobs(jobs, datatable.Sort{Key: key, Descending: descending})
			want := "a"
			if descending {
				want = "b"
			}
			if jobs[0].ID != want {
				t.Errorf("%s descending=%v: got %s", key, descending, jobs[0].ID)
			}
			sortJobs(jobs, datatable.Sort{Key: key, Descending: descending})
			if jobs[0].ID != want {
				t.Errorf("sort reset on refresh: %s", key)
			}
		}
	}
	sortJobs(nil, datatable.Sort{Key: "size"})
	unknown := jobView{Job: download.Job{ID: "unknown"}}
	for _, key := range []string{"size", "progress", "eta"} {
		jobs := []jobView{a, unknown}
		sortJobs(jobs, datatable.Sort{Key: key})
		want := "unknown"
		if key == "eta" {
			want = "a"
		}
		if jobs[0].ID != want {
			t.Errorf("unknown %s: %s", key, jobs[0].ID)
		}
	}
	jobs := []jobView{b, a}
	before := slices.Clone(jobs)
	sortJobs(jobs, datatable.Sort{})
	if !slices.Equal(jobs, before) {
		t.Fatal("unsorted order changed")
	}
}

func TestTableColumnsFillWidth(t *testing.T) {
	for _, width := range []float32{0, 400, 1200, 2400} {
		cols := tableColumns(core.Points(width))
		var total core.Points
		for _, col := range cols {
			total += col.Width
		}
		if total < core.Points(width) {
			t.Fatalf("width %v: columns total %v", width, total)
		}
		if cols[1].Width < columns[1].Width {
			t.Fatal("name column shrank below base width")
		}
	}
}
