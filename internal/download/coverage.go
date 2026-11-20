package download

// ByteSpan is a half-open interval of bytes already written to a Job's file.
type ByteSpan struct {
	Start, End int64
}

// StateWithCoverage takes a consistent snapshot of Jobs and their written byte
// spans, keyed by Job ID. Active ranges reflect live writes, not disk checkpoints.
func (q *Queue) StateWithCoverage() (Snapshot, map[string][]ByteSpan) {
	q.mu.Lock()
	defer q.mu.Unlock()
	state := q.stateLocked()
	coverage := make(map[string][]ByteSpan)
	for _, job := range q.config.Jobs {
		if job.Received == 0 {
			continue
		}
		if job.State == Completed || job.Plan == nil {
			coverage[job.ID] = []ByteSpan{{End: job.Received}}
			continue
		}
		if run := q.active[job.ID]; run != nil && run.ranges != nil {
			for _, r := range run.ranges {
				if r.Start < r.Next {
					coverage[job.ID] = append(coverage[job.ID], ByteSpan{r.Start, r.Next})
				}
			}
		} else {
			for _, r := range job.Plan.Ranges {
				if r.Start < r.Next {
					coverage[job.ID] = append(coverage[job.ID], ByteSpan{r.Start, r.Next})
				}
			}
		}
	}
	return state, coverage
}
