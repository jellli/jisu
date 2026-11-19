package main

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/jellli/jisu/internal/download"
	"github.com/nsgo-dev/nsgo/base/datatable"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
)

const segmentCount = 16

type row struct {
	download.Job
	segments [segmentCount]float32
}

func segmentFill(total int64, spans []download.ByteSpan) (fill [segmentCount]float32) {
	if total <= 0 {
		return fill
	}
	boundary := func(i int64) int64 { return total/segmentCount*i + total%segmentCount*i/segmentCount }
	for i := range fill {
		start, end := boundary(int64(i)), boundary(int64(i+1))
		var written int64
		for _, span := range spans {
			written += max(0, min(end, span.End)-max(start, span.Start))
		}
		if end > start {
			fill[i] = float32(written) / float32(end-start)
		}
	}
	return fill
}

var (
	ink    = core.RGB(0x172331)
	muted  = core.RGB(0x526478)
	line   = core.RGB(0xdfe6ee)
	paper  = core.RGB(0xffffff)
	accent = core.RGB(0x06756a)
)

var columns = []datatable.Column{
	{Key: "name", Title: "File name", Width: 243, MinWidth: 114},
	{Key: "size", Title: "Size", Width: 92, MinWidth: 70},
	{Key: "progress", Title: "Progress", Width: 236, MinWidth: 160},
	{Key: "speed", Title: "Speed", Width: 115, MinWidth: 98},
	{Key: "status", Title: "Status", Width: 123, MinWidth: 98},
	{Key: "eta", Title: "ETA", Width: 93, MinWidth: 61},
	{Key: "connections", Title: "Links", Width: 76, MinWidth: 57},
}

func taskTable(q *download.Queue) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		cx.SetTitle("jisu | Download tasks")
		rows := core.NewSignalFunc(cx, []row(nil), slices.Equal)
		selected := core.NewSignalFunc(cx, []string(nil), slices.Equal)
		message := core.NewSignal(cx, "")
		refresh := func() {
			state, coverage := q.StateWithCoverage()
			next := make([]row, len(state.Jobs))
			for i, job := range state.Jobs {
				next[i].Job = job
				if job.Total != nil {
					next[i].segments = segmentFill(*job.Total, coverage[job.ID])
				}
			}
			rows.Set(next)
		}
		refresh()
		// One timer on the UI scope, automatically stopped when the window is disposed.
		var tick func()
		tick = func() {
			refresh()
			cx.AfterFunc(250*time.Millisecond, tick)
		}
		cx.AfterFunc(250*time.Millisecond, tick)

		rowAt := func(i int) row {
			rs := rows.Get()
			if i < 0 || i >= len(rs) {
				return row{}
			}
			return rs[i]
		}
		selectedIndices := func() (indices []int) {
			ids := selected.Get()
			for i, r := range rows.Get() {
				if slices.Contains(ids, r.ID) {
					indices = append(indices, i)
				}
			}
			return indices
		}
		selectIndices := func(indices []int) {
			ids := make([]string, 0, len(indices))
			for _, i := range indices {
				ids = append(ids, rowAt(i).ID)
			}
			selected.Set(ids)
		}
		runAction := func(action func() error) {
			message.Set("")
			if err := action(); err != nil {
				message.Set(err.Error())
			}
			refresh()
		}
		contextMenu := func(e datatable.ContextMenu) {
			clicked := rowAt(e.Row)
			if clicked.ID == "" {
				return
			}
			if !slices.Contains(selected.Get(), clicked.ID) {
				selected.Set([]string{clicked.ID})
			}
			ids := slices.Clone(selected.Get())
			action := func(label string, allowed func(row) bool, fn func(string) error) desktop.MenuItem {
				var targets []string
				for _, r := range rows.Get() {
					if slices.Contains(ids, r.ID) && allowed(r) {
						targets = append(targets, r.ID)
					}
				}
				return desktop.MenuItem{Label: label, Disabled: len(targets) == 0, OnSelect: func() {
					runAction(func() error {
						for _, id := range targets {
							if err := fn(id); err != nil {
								return err
							}
						}
						return nil
					})
				}}
			}
			move := func(label string, up bool) desktop.MenuItem {
				return desktop.MenuItem{Label: label, OnSelect: func() {
					runAction(func() error { return q.MoveJobs(ids, up) })
				}}
			}
			desktop.PopupMenu(cx, []desktop.MenuItem{
				action("Pause", func(r row) bool { return r.State == download.Downloading || r.State == download.Queued }, q.Pause),
				action("Resume", func(r row) bool { return r.State == download.Paused || r.State == download.Failed }, q.Resume),
				{Separator: true},
				move("Move up", true), move("Move down", false),
				{Separator: true},
				action("Remove", func(row) bool { return true }, func(id string) error {
					if err := q.Remove(id); err != nil {
						return err
					}
					selected.Set(slices.DeleteFunc(slices.Clone(selected.Get()), func(s string) bool { return s == id }))
					return nil
				}),
			}, e.At, nil)
		}

		table := datatable.DataTable{
			Columns: func() []datatable.Column { return columns }, RowCount: func() int { return len(rows.Get()) },
			Text: func(i, j int) string { return cellText(rowAt(i), columns[j].Key) },
			RenderCell: func(_ *core.Scope, cell func() datatable.Cell) core.Node {
				return core.Show{
					When: func() bool { return cell().Key == "progress" },
					Then: func(*core.Scope) core.Node {
						return progressCell(func() row { return rowAt(cell().Row) }, func() string { return cell().Text })
					},
					Else: func(*core.Scope) core.Node {
						return core.Text{ContentFunc: func() string { return cell().Text }, MaxLines: 1, Style: core.Style{FontSize: 13},
							StyleFunc: func() core.Style {
								c := cell()
								color := ink
								if c.Key == "speed" || c.Key == "eta" || c.Key == "connections" {
									color = muted
								}
								if c.Key == "status" && (c.Text == "Downloading" || c.Text == "Complete") {
									color = accent
								}
								return core.Style{Color: color}
							}}
					},
				}
			},
			RowHeight: 44, HeaderHeight: 36, FontSize: 13, MultiSelect: true, NoLoopSelection: true,
			SelectedRows: selectedIndices, OnSelectedRowsChange: selectIndices, OnContextMenu: contextMenu,
			Stripe: func() bool { return true }, Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0), Background: paper, BorderWidth: 1, BorderColor: line},
			Styles: datatable.Styles{
				Header:        core.Style{Background: core.RGB(0xeef3f8), Color: muted, Borders: core.Edges{Bottom: 1}, BorderColor: line},
				HeaderCell:    core.Style{Padding: core.Symmetric(0, 12), FontSize: 12},
				ResizeHandle:  core.Style{Background: line},
				MoveIndicator: core.Style{Background: accent},
				Row:           core.Style{Borders: core.Edges{Bottom: 1}, BorderColor: line},
				RowOdd:        core.Style{Background: core.RGB(0xf8fafc)},
				RowSelected:   core.Style{Background: core.RGB(0xdaf0f9)},
				Cell:          core.Style{Padding: core.Symmetric(0, 12)},
			},
			Label: "Download tasks", TestID: "tasks",
			Empty: core.Text{Content: "No tasks", Style: core.Style{Color: muted}},
		}

		return core.View{Style: core.Style{FlexGrow: 1, Background: core.RGB(0xf3f6fa), Padding: core.All(22), Gap: 14}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
				core.View{Style: core.Style{Gap: 3}, Children: []core.Node{
					core.Text{Content: "JISU / KITE V2", Style: core.Style{Color: accent, FontSize: 11, FontWeight: 700}},
					core.Text{Content: "Download tasks", Style: core.Style{Color: ink, FontSize: 26, FontWeight: 650}},
				}},
				core.Text{Content: "NSGo prototype  /  Local HTTP transfers", Style: core.Style{Color: muted, FontSize: 12}},
			}},
			table,
			core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
				core.Text{TestID: "selection-count", ContentFunc: func() string {
					return fmt.Sprintf("%d tasks   /   %d selected", len(rows.Get()), len(selectedIndices()))
				}, Style: core.Style{Color: muted, FontSize: 12}},
				core.Text{ContentFunc: func() string {
					if err := message.Get(); err != "" {
						return err
					}
					return "Drag headers to arrange columns  ·  Right-click tasks for queue actions"
				}, MaxLines: 1, Style: core.Style{Color: muted, FontSize: 12, FlexShrink: 1}},
			}},
		}}
	}
}

func progressCell(get func() row, percent func() string) core.Node {
	bars := make([]core.Node, segmentCount)
	for i := range bars {
		bars[i] = core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0),
			Height: core.Pt(10), Background: core.RGB(0xdde6ed), CornerRadius: 2, Overflow: core.Hidden}, Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(10)}, StyleFunc: func() core.Style {
				r := get()
				color := accent
				if r.State == download.Paused {
					color = core.RGB(0x95630c)
				}
				return core.Style{Background: color, Width: core.Pct(100 * r.segments[i])}
			}},
		}}
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: bars},
		core.Text{ContentFunc: percent, Style: core.Style{Color: muted, FontSize: 12, Width: core.Pt(36), TextAlign: core.TextAlignEnd}},
	}}
}

func cellText(r row, key string) string {
	if r.ID == "" {
		return ""
	}
	switch key {
	case "name":
		return r.Name
	case "size":
		if r.Total != nil {
			return fmt.Sprintf("%.1f MiB", float64(*r.Total)/(1<<20))
		}
	case "progress":
		if r.State == download.Completed {
			return "100%"
		}
		if r.Total != nil && *r.Total > 0 {
			return fmt.Sprintf("%d%%", min(99, int(100*float64(r.Received)/float64(*r.Total))))
		}
		return "0%"
	case "speed":
		if r.BytesPerSecond > 0 {
			return fmt.Sprintf("%.1f MiB/s", r.BytesPerSecond/(1<<20))
		}
	case "status":
		switch r.State {
		case download.Downloading:
			return "Downloading"
		case download.Queued:
			return "Queued"
		case download.Paused:
			return "Paused"
		case download.Failed:
			return "Failed"
		case download.Completed:
			return "Complete"
		}
	case "eta":
		if r.Total != nil && r.BytesPerSecond > 0 {
			seconds := math.Ceil(float64(max(0, *r.Total-r.Received)) / r.BytesPerSecond)
			if seconds < 24*60*60 {
				return (time.Duration(seconds) * time.Second).String()
			}
		}
	case "connections":
		if r.Connections > 0 {
			return fmt.Sprint(r.Connections)
		}
	}
	return "–"
}
