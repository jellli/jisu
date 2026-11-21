package main

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jellli/jisu/internal/download"
	"github.com/nsgo-dev/nsgo/base/datatable"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

const segmentCount = 16

var columns = []datatable.Column{
	{Key: "selection", Title: " ", Width: 37, Fixed: true, NoResize: true, NoMove: true, NoSelect: true, Align: datatable.AlignCenterColumn},
	{Key: "name", Title: "文件名", Width: 266, MinWidth: 130},
	{Key: "size", Title: "大小", Width: 96, MinWidth: 67, Align: datatable.AlignEndColumn},
	{Key: "progress", Title: "进度与分块", Width: 210, MinWidth: 135},
	{Key: "speed", Title: "速度", Width: 105, MinWidth: 88, Align: datatable.AlignEndColumn},
	{Key: "eta", Title: "剩余时间", Width: 104, MinWidth: 77, Align: datatable.AlignEndColumn},
}

func workspaceTable(cx *core.Scope, queue *download.Queue, colors func() palette, rows func() []jobView,
	queues func() []string, empty core.Node, selection *core.Signal[[]string], inspector, remove *core.Signal[bool],
	sortOrder *core.Signal[datatable.Sort],
	run func(func() error) bool, apply func([]jobView, func(jobView) bool, func(string) error)) core.Node {
	width := core.NewSignal(cx, core.Points(0))
	openFile := func(job jobView) {
		if job.State != download.Completed {
			return
		}
		path, err := queue.CompletedPath(job.ID)
		if err != nil {
			run(func() error { return err })
			return
		}
		cx.OpenFile(core.SharedFile{Path: path}, func(_ bool, err error) {
			if err != nil {
				run(func() error { return err })
			}
		})
	}
	rowAt := func(i int) jobView {
		visible := rows()
		if i >= 0 && i < len(visible) {
			return visible[i]
		}
		return jobView{}
	}
	isSelected := func(id string) bool { return slices.Contains(selection.Get(), id) }
	selectedRows := func() (indices []int) {
		for i, job := range rows() {
			if isSelected(job.ID) {
				indices = append(indices, i)
			}
		}
		return indices
	}
	var beforeRowPress *[]string
	selectRows := func(indices []int) {
		previous := slices.Clone(selection.Get())
		beforeRowPress = &previous
		cx.Post(func() {
			if beforeRowPress == &previous {
				beforeRowPress = nil
			}
		})
		ids := make([]string, 0, len(indices))
		for _, i := range indices {
			ids = append(ids, rowAt(i).ID)
		}
		selection.Set(ids)
	}
	selectAll := func(checked bool) {
		var ids []string
		if checked {
			for _, job := range rows() {
				ids = append(ids, job.ID)
			}
		}
		selection.Set(ids)
	}
	contextMenu := func(event datatable.ContextMenu) {
		clicked := rowAt(event.Row)
		if clicked.ID == "" {
			return
		}
		if !isSelected(clicked.ID) {
			selection.Set([]string{clicked.ID})
		}
		ids := make([]string, 0)
		targets := make([]jobView, 0)
		for _, job := range rows() {
			if isSelected(job.ID) {
				ids = append(ids, job.ID)
				targets = append(targets, job)
			}
		}
		action := func(label string, allowed func(jobView) bool, fn func(string) error) desktop.MenuItem {
			return desktop.MenuItem{Label: label, Disabled: !slices.ContainsFunc(targets, allowed),
				OnSelect: func() { apply(targets, allowed, fn) }}
		}
		var destinations []desktop.MenuItem
		for _, name := range queues() {
			destinations = append(destinations, desktop.MenuItem{Label: queueLabel(name),
				Disabled: !slices.ContainsFunc(targets, func(job jobView) bool { return job.Queue != name }),
				OnSelect: func() { run(func() error { return queue.MoveToQueue(ids, name) }) }})
		}
		desktop.PopupMenu(cx, []desktop.MenuItem{
			action("暂停", canPause, queue.Pause), action("继续 / 重试", canResume, queue.Resume),
			{Label: "打开文件", Disabled: clicked.State != download.Completed, OnSelect: func() { openFile(clicked) }},
			{Label: "打开文件夹", OnSelect: func() {
				if clicked.State == download.Completed {
					path, err := queue.CompletedPath(clicked.ID)
					if err != nil {
						run(func() error { return err })
						return
					}
					cx.RevealFile(path, func(err error) {
						if err != nil {
							run(func() error { return err })
						}
					})
				} else {
					cx.OpenFile(core.SharedFile{Path: clicked.Directory}, func(_ bool, err error) {
						if err != nil {
							run(func() error { return err })
						}
					})
				}
			}},
			{Label: "显示详情", OnSelect: func() { inspector.Set(true) }},
			{Separator: true},
			{Label: "上移", OnSelect: func() { run(func() error { return queue.MoveJobs(ids, true) }) }},
			{Label: "下移", OnSelect: func() { run(func() error { return queue.MoveJobs(ids, false) }) }},
			{Label: "移至队列", Items: destinations},
			{Separator: true},
			{Label: "删除", OnSelect: func() { remove.Set(true) }},
		}, event.At, nil)
	}
	return datatable.DataTable{
		Columns: func() []datatable.Column { return tableColumns(width.Get()) }, RowCount: func() int { return len(rows()) },
		OnLayout: func(bounds core.Bounds) { width.Set(bounds.Size.Width) },
		Sort:     sortOrder.Get,
		OnSortChange: func(next datatable.Sort) {
			if next.Key == "" {
				next = sortOrder.Get()
				next.Descending = false
			}
			sortOrder.Set(next)
		},
		OnDoublePress: func(row, _ int) { openFile(rowAt(row)) },
		Text:          func(i, j int) string { return cellText(rowAt(i), columns[j].Key) },
		RenderHeaderCell: func(_ *core.Scope, cell func() datatable.HeaderCell) core.Node {
			return core.Show{
				When: func() bool { return cell().Key == "selection" },
				Then: func(*core.Scope) core.Node {
					return ui.Checkbox{Label: "选择全部任务", TestID: "select-all",
						Checked:       func() bool { return len(rows()) > 0 && len(selectedRows()) == len(rows()) },
						Indeterminate: func() bool { return len(selectedRows()) > 0 && len(selectedRows()) < len(rows()) },
						Disabled:      func() bool { return len(rows()) == 0 }, OnChange: selectAll}
				},
				Else: func(*core.Scope) core.Node {
					title := label("", 11, func() core.Color { return colors().muted })
					title.ContentFunc = func() string { return sortHeaderTitle(cell().Title, cell().Key, sortOrder.Get()) }
					title.StyleFunc = func() core.Style {
						return core.Style{Color: colors().muted, TextAlign: columnAlignment(cell().Column)}
					}
					return core.View{Role: core.RoleButton, LabelFunc: func() string { return cell().Title },
						Style: core.Style{FlexGrow: 1}, OnPress: func(core.PressEvent) {
							current := sortOrder.Get()
							sortOrder.Set(datatable.Sort{Key: cell().Key, Descending: current.Key == cell().Key && !current.Descending})
						}, Children: []core.Node{title}}
				},
			}
		},
		RenderCell: func(_ *core.Scope, cell func() datatable.Cell) core.Node {
			get := func() jobView { return rowAt(cell().Row) }
			text := label("", 12, func() core.Color {
				if cell().Key == "progress" {
					return stateColor(colors(), get())
				}
				return colors().ink
			})
			color := text.StyleFunc
			text.StyleFunc = func() core.Style {
				style := color()
				style.TextAlign = columnAlignment(cell().Column)
				return style
			}
			text.ContentFunc = func() string { return cell().Text }
			return core.Show{When: func() bool { return cell().Key == "selection" }, Then: func(*core.Scope) core.Node {
				return ui.Checkbox{Label: "选择任务", TestID: "select-" + get().ID, Checked: func() bool { return isSelected(get().ID) }, OnChange: func(checked bool) {
					id := get().ID
					ids := slices.Clone(selection.Get())
					if checked && !slices.Contains(ids, id) {
						ids = append(ids, id)
					} else if !checked {
						ids = slices.DeleteFunc(ids, func(s string) bool { return s == id })
					}
					selection.Set(ids)
				}}
			}, Else: func(*core.Scope) core.Node {
				if cell().Key == "eta" {
					return core.Show{When: func() bool { return get().State == download.Failed }, Then: func(*core.Scope) core.Node {
						return compactButton("retry-"+get().ID, "重试", func() { run(func() error { return queue.Resume(get().ID) }) })
					}, Else: func(*core.Scope) core.Node { return text }}
				}
				return core.Show{When: func() bool { return cell().Key == "progress" }, Then: func(*core.Scope) core.Node {
					return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, FlexGrow: 1, Gap: 8},
						Children: []core.Node{progressBars(colors, get, 5), text}}
				}, Else: func(*core.Scope) core.Node {
					return core.Show{When: func() bool { return cell().Key == "name" }, Then: func(*core.Scope) core.Node {
						return nameCell(colors, get, text)
					}, Else: func(*core.Scope) core.Node { return text }}
				}}
			}}
		},
		RowHeight: 44, HeaderHeight: 32, FontSize: 12, MultiSelect: true, NoLoopSelection: true,
		SelectedRows: selectedRows, OnSelectedRowsChange: selectRows, OnContextMenu: contextMenu,
		OnRowPress: func(press datatable.Press) {
			previous := selection.Get()
			if beforeRowPress != nil {
				previous = *beforeRowPress
			}
			beforeRowPress = nil
			// The table's hit layer takes pointer presses over rendered checkboxes.
			if press.Column == 0 {
				id := rowAt(press.Row).ID
				ids := slices.Clone(previous)
				if slices.Contains(ids, id) {
					ids = slices.DeleteFunc(ids, func(value string) bool { return value == id })
				} else {
					ids = append(ids, id)
				}
				selection.Set(ids)
			} else if press.Clicks == 1 {
				if press.Column >= 0 && press.Column < len(columns) && columns[press.Column].Key == "eta" && rowAt(press.Row).State == download.Failed {
					run(func() error { return queue.Resume(rowAt(press.Row).ID) })
					return
				}
				id := rowAt(press.Row).ID
				inspector.Set(!(inspector.Get() && len(previous) == 1 && previous[0] == id))
			}
		},
		Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)},
		RowStyle: func(i int) core.Style {
			if rowAt(i).State == download.Failed {
				return core.Style{Background: colors().failed}
			}
			return core.Style{}
		},
		StyleFunc: func() core.Style { return core.Style{Background: colors().paper} },
		StylesFunc: func() datatable.Styles {
			p := colors()
			return datatable.Styles{
				Header:       core.Style{Background: p.chrome, Color: p.muted, Borders: core.Edges{Bottom: 1}, BorderColor: p.line},
				HeaderCell:   core.Style{Padding: core.Symmetric(0, 8)},
				ResizeHandle: core.Style{Background: p.line}, MoveIndicator: core.Style{Background: p.accent},
				Row:         core.Style{Borders: core.Edges{Bottom: 1}, BorderColor: p.line},
				RowSelected: core.Style{Background: p.picked}, Cell: core.Style{Padding: core.Symmetric(0, 8)},
			}
		},
		Label: "下载任务", TestID: "tasks",
		Empty: empty,
	}
}

func columnAlignment(i int) core.TextAlign {
	if i >= 0 && i < len(columns) && columns[i].Align == datatable.AlignEndColumn {
		return core.TextAlignEnd
	}
	return core.TextAlignStart
}

func nameCell(colors func() palette, get func() jobView, name core.Text) core.Node {
	icon := ui.IconOf(func() *core.SVG {
		if get().State == download.Failed {
			return lucide.CircleAlert()
		}
		return categoryIcons[fileCategory(get().Name)]()
	}, 16, func() core.Color {
		if get().State == download.Failed {
			return core.RGB(0xdc2626)
		}
		return colors().muted
	})
	host := label("", 10, func() core.Color { return colors().muted })
	host.MaxLines = 2
	host.ContentFunc = func() string {
		if get().State == download.Failed {
			return failureReason(get().Error)
		}
		_, h := protocol(get())
		return h
	}
	host.StyleFunc = func() core.Style {
		if get().State == download.Failed {
			return core.Style{Color: stateColor(colors(), get())}
		}
		return core.Style{Color: colors().muted}
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 7, MinWidth: core.Pt(0), FlexShrink: 1},
		Children: []core.Node{icon, core.View{Style: core.Style{Gap: 2, MinWidth: core.Pt(0), FlexShrink: 1}, Children: []core.Node{name, host}}}}
}

func cellText(job jobView, key string) string {
	if job.ID == "" {
		return ""
	}
	switch key {
	case "name":
		return job.Name
	case "size":
		if job.Total != nil {
			return formatBytes(float64(*job.Total))
		}
	case "progress":
		percent := "0%"
		if job.State == download.Completed {
			percent = "100%"
		} else if job.Total != nil && *job.Total > 0 {
			percent = fmt.Sprintf("%.1f%%", min(99.9, 100*float64(job.Received)/float64(*job.Total)))
		}
		switch job.State {
		case download.Completed:
			return percent + " · 已完成"
		case download.Paused:
			return percent + " · 已暂停"
		case download.Queued:
			return percent + " · 排队中"
		case download.Failed:
			return percent + " · 下载失败"
		}
		return percent
	case "speed":
		if job.State == download.Downloading {
			return formatBytes(job.BytesPerSecond) + "/s"
		}
		return ""
	case "eta":
		if job.State == download.Downloading && job.Total != nil && job.BytesPerSecond > 0 {
			seconds := math.Ceil(float64(max(0, *job.Total-job.Received)) / job.BytesPerSecond)
			if seconds < 24*60*60 {
				return (time.Duration(seconds) * time.Second).String()
			}
		}
	case "selection":
		return ""
	}
	return "—"
}

var httpFailure = regexp.MustCompile(`(?i)\bHTTP ([45][0-9]{2})\b`)

func failureReason(raw string) string {
	if match := httpFailure.FindStringSubmatch(raw); match != nil {
		switch match[1] {
		case "404":
			return "文件不存在 (404)"
		case "403":
			return "无权访问 (403)"
		case "429":
			return "请求过于频繁，被限流 (429)"
		default:
			return "服务器错误 (" + match[1] + ")"
		}
	}
	lower := strings.ToLower(raw)
	for _, marker := range []string{"no such host", "dns", "connection refused", "timeout", "timed out", "deadline exceeded"} {
		if strings.Contains(lower, marker) {
			return "连不上服务器"
		}
	}
	return strings.TrimSpace(raw)
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

func stateColor(p palette, job jobView) core.Color {
	switch job.State {
	case download.Paused:
		return core.RGB(0xd97706)
	case download.Failed:
		return core.RGB(0xdc2626)
	case download.Completed:
		return core.RGB(0x16a34a)
	case download.Queued:
		return p.muted
	default:
		return p.accent
	}
}

func progressBars(colors func() palette, get func() jobView, height core.Points) core.Node {
	bars := make([]core.Node, segmentCount)
	for i := range bars {
		bars[i] = core.View{
			Style:     core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), Height: core.Pt(height), Overflow: core.Hidden, CornerRadius: 1},
			StyleFunc: func() core.Style { return core.Style{Background: colors().track} },
			Children: []core.Node{core.View{Style: core.Style{Height: core.Pt(height)}, StyleFunc: func() core.Style {
				job := get()
				fill := job.segments[i]
				color := stateColor(colors(), job)
				if job.State == download.Completed {
					fill = 1
				}
				if fill >= 1 {
					color = core.RGB(0x16a34a)
				}
				return core.Style{Background: color, Width: core.Pct(100 * fill)}
			}}},
		}
	}
	return core.View{Style: core.Style{Direction: core.Row, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 1}, Children: bars}
}
