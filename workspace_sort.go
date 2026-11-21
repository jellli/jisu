package main

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/jellli/jisu/internal/download"
	"github.com/nsgo-dev/nsgo/base/datatable"
	"github.com/nsgo-dev/nsgo/core"
)

func tableColumns(width core.Points) []datatable.Column {
	out := slices.Clone(columns)
	var total core.Points
	for i := range out {
		total += out[i].Width
		out[i].Sortable = i != 0
	}
	out[1].Width += max(0, width-total)
	out[1].MaxWidth = max(1200, out[1].Width)
	return out
}

func jobProgress(job jobView) float64 {
	if job.State == download.Completed {
		return 1
	}
	if job.Total == nil || *job.Total <= 0 {
		return -1
	}
	return float64(job.Received) / float64(*job.Total)
}

func jobETA(job jobView) float64 {
	if job.State == download.Completed {
		return 0
	}
	if job.Total == nil || job.BytesPerSecond <= 0 {
		return math.Inf(1)
	}
	return max(0, float64(*job.Total-job.Received)) / job.BytesPerSecond
}

func sortJobs(jobs []jobView, order datatable.Sort) {
	if order.Key == "" {
		return
	}
	slices.SortStableFunc(jobs, func(a, b jobView) int {
		var result int
		switch order.Key {
		case "name":
			result = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		case "size":
			size := func(j jobView) int64 {
				if j.Total == nil {
					return -1
				}
				return *j.Total
			}
			result = cmp.Compare(size(a), size(b))
		case "progress":
			result = cmp.Compare(jobProgress(a), jobProgress(b))
		case "speed":
			result = cmp.Compare(a.BytesPerSecond, b.BytesPerSecond)
		case "eta":
			result = cmp.Compare(jobETA(a), jobETA(b))
		}
		if order.Descending {
			return -result
		}
		return result
	})
}
