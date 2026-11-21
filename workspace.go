package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jellli/jisu/internal/download"
	"github.com/nsgo-dev/nsgo/base/datatable"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

type jobView struct {
	download.Job
	segments [segmentCount]float32
	target   int
}

type filter struct {
	group, key, label string
}

var statusFilters = []filter{
	{"status", "", "全部任务"}, {"status", download.Downloading, "正在下载"},
	{"status", download.Completed, "已完成"}, {"status", download.Paused, "已暂停"},
}

var categoryFilters = []filter{
	{"category", "video", "视频媒体"}, {"category", "archive", "压缩包"},
	{"category", "iso", "安装镜像"}, {"category", "document", "文档"},
	{"category", "other", "其他文件"},
}

var categoryIcons = map[string]func() *core.SVG{
	"video": lucide.Film, "archive": lucide.Archive, "iso": lucide.Disc,
	"document": lucide.FileText, "other": lucide.File,
}

func (f filter) icon() func() *core.SVG {
	switch f.group {
	case "category":
		return categoryIcons[f.key]
	case "queue":
		return lucide.ListOrdered
	default:
		return map[string]func() *core.SVG{
			"": lucide.Inbox, download.Downloading: lucide.ArrowDownToLine,
			download.Completed: lucide.CircleCheck, download.Paused: lucide.CirclePause,
		}[f.key]
	}
}

func fileCategory(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".m4v", ".mkv", ".mov", ".webm", ".avi", ".mp3", ".flac", ".wav", ".m4a", ".ogg":
		return "video"
	case ".zip", ".7z", ".rar", ".tar", ".gz", ".bz2", ".xz", ".zst", ".tgz":
		return "archive"
	case ".iso", ".img", ".dmg":
		return "iso"
	case ".pdf", ".doc", ".docx", ".odt", ".txt", ".xls", ".xlsx", ".csv", ".ppt", ".pptx", ".epub", ".md":
		return "document"
	default:
		return "other"
	}
}

func queueLabel(key string) string {
	switch key {
	case download.DefaultQueue:
		return "默认队列"
	case download.NightQueue:
		return "夜间队列"
	default:
		return key
	}
}

func (f filter) matches(job jobView) bool {
	switch f.group {
	case "category":
		return fileCategory(job.Name) == f.key
	case "queue":
		return job.Queue == f.key
	default:
		return f.key == "" || job.State == f.key
	}
}

type palette struct {
	ink, muted, line, paper, chrome, sidebar, accent, picked, track, failed core.Color
}

func errorColor(p palette) core.Color {
	if p.paper == core.RGB(0x1c1c1e) {
		return core.RGB(0xff8585)
	}
	return core.RGB(0xdc2626)
}

func workspace(queue *download.Queue) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		cx.SetTitle("jisu")
		jobs := core.NewSignalFunc(cx, []jobView(nil), slices.Equal)
		queues := core.NewSignalFunc(cx, []string(nil), slices.Equal)
		directory := core.NewSignal(cx, "")
		concurrency := core.NewSignal(cx, 0)
		refresh := func() {
			state, spans := queue.StateWithCoverage()
			rows := make([]jobView, len(state.Jobs))
			for i, job := range state.Jobs {
				rows[i] = jobView{Job: job, target: job.ConnectionTarget}
				if rows[i].target == 0 {
					rows[i].target = state.Connections
				}
				if job.Total != nil {
					rows[i].segments = segmentFill(*job.Total, spans[job.ID])
				}
			}
			jobs.Set(rows)
			queues.Set(state.Queues)
			directory.Set(state.DownloadDirectory)
			concurrency.Set(state.Concurrency)
		}
		refresh()
		var tick func()
		tick = func() {
			refresh()
			cx.AfterFunc(250*time.Millisecond, tick)
		}
		cx.AfterFunc(250*time.Millisecond, tick)

		active := core.NewSignal(cx, statusFilters[0])
		search := core.NewSignal(cx, "")
		message := core.NewSignal(cx, "")
		selection := core.NewSignalFunc(cx, []string(nil), slices.Equal)
		inspector := core.NewSignal(cx, false)
		sortOrder := core.NewSignal(cx, datatable.Sort{})
		dark := core.NewSignal(cx, false)
		colors := func() palette {
			if dark.Get() {
				return palette{core.RGB(0xf4f4f5), core.RGB(0xa1a1aa), core.RGB(0x38383a), core.RGB(0x1c1c1e),
					core.RGB(0x242426), core.RGB(0x202022), core.RGB(0x0a84ff), core.RGB(0x303c4a), core.RGB(0x38383a), core.RGB(0x352326)}
			}
			return palette{core.RGB(0x18181b), core.RGB(0x71717a), core.RGB(0xe4e4e7), core.RGB(0xffffff),
				core.RGB(0xfafafa), core.RGB(0xf4f4f5), core.RGB(0x007aff), core.RGB(0xeff6ff), core.RGB(0xe4e4e7), core.RGB(0xfff1f2)}
		}
		rows := core.NewMemoFunc(cx, func() []jobView {
			f, query := active.Get(), strings.ToLower(strings.TrimSpace(search.Get()))
			var visible []jobView
			for _, job := range jobs.Get() {
				if f.matches(job) && (query == "" || strings.Contains(strings.ToLower(job.Name+" "+job.URL), query)) {
					visible = append(visible, job)
				}
			}
			sortJobs(visible, sortOrder.Get())
			return visible
		}, slices.Equal).Get
		selectView := func(f filter) {
			active.Set(f)
			selection.Set(nil)
		}
		selected := func() []jobView {
			ids := selection.Get()
			var out []jobView
			for _, job := range rows() {
				if slices.Contains(ids, job.ID) {
					out = append(out, job)
				}
			}
			return out
		}
		run := func(f func() error) bool {
			message.Set("")
			err := f()
			refresh()
			if err != nil {
				message.Set(err.Error())
			}
			return err == nil
		}
		apply := func(jobs []jobView, allowed func(jobView) bool, action func(string) error) {
			run(func() error {
				for _, job := range jobs {
					if allowed(job) {
						if err := action(job.ID); err != nil {
							return err
						}
					}
				}
				return nil
			})
		}
		remove := core.NewSignal(cx, false)
		toolbar, dialogs, empty, openQueue := workspaceControls(cx, queue, colors, directory.Get, concurrency.Get, queues.Get, active.Get,
			search, inspector, dark, message, remove, selected, jobs.Get, run, apply)
		table := workspaceTable(cx, queue, colors, rows, queues.Get, empty, selection, inspector, remove, sortOrder, run, apply)
		return core.View{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, StyleFunc: func() core.Style {
			return core.Style{Background: colors().paper}
		}, Children: append([]core.Node{
			toolbar,
			core.View{Style: core.Style{Direction: core.Row, FlexGrow: 1, MinHeight: core.Pt(0)}, Children: []core.Node{
				sidebar(colors, jobs.Get, queues.Get, active.Get, selectView, openQueue),
				core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0)}, Children: []core.Node{
					table,
					core.Show{When: inspector.Get, Then: func(scope *core.Scope) core.Node {
						return detailPanel(scope, queue, colors, selected, run)
					}},
				}},
			}},
			statusBar(colors, rows, selected, jobs.Get, message.Get),
		}, dialogs...)}
	}
}

func label(text string, size core.Points, color func() core.Color) core.Text {
	return core.Text{Content: text, MaxLines: 1, Style: core.Style{FontSize: size}, StyleFunc: func() core.Style {
		return core.Style{Color: color()}
	}}
}

func sidebar(colors func() palette, jobs func() []jobView, queues func() []string, active func() filter, selectView func(filter), openQueue func()) core.Node {
	item := func(f filter) core.Node {
		title := label(f.label, 12, func() core.Color { return colors().ink })
		title.Style.FlexGrow = 1
		count := label("", 11, func() core.Color { return colors().muted })
		count.ContentFunc = func() string {
			n := 0
			for _, job := range jobs() {
				if f.matches(job) {
					n++
				}
			}
			if n == 0 {
				return ""
			}
			return fmt.Sprint(n)
		}
		return core.View{TestID: f.group + "-" + f.key, Role: core.RoleButton, Label: f.label,
			OnPress: func(core.PressEvent) { selectView(f) },
			Style: core.Style{Height: core.Pt(27), Direction: core.Row, AlignItems: core.AlignCenter,
				Padding: core.Symmetric(0, 8), CornerRadius: 4, Gap: 7},
			StyleFunc: func() core.Style {
				if active() == f {
					return core.Style{Background: colors().picked}
				}
				return core.Style{}
			}, Children: []core.Node{ui.Icon(f.icon(), 14, func() core.Color { return colors().muted }), title, count}}
	}
	section := func(title string, children []core.Node, actions ...core.Node) core.View {
		head := label(title, 10, func() core.Color { return colors().muted })
		head.Style.FlexGrow = 1
		header := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 8),
			Margin: core.Edges{Top: 8, Bottom: 3}}, Children: append([]core.Node{head}, actions...)}
		return core.View{Style: core.Style{Gap: 1}, Children: append([]core.Node{header}, children...)}
	}
	static := func(group []filter) []core.Node {
		items := make([]core.Node, 0, len(group))
		for _, f := range group {
			items = append(items, item(f))
		}
		return items
	}
	return core.Scroll{Style: core.Style{Width: core.Pt(200), FlexShrink: 0, Borders: core.Edges{Right: 1}},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors().sidebar, BorderColor: colors().line}
		}, Children: []core.Node{core.View{Style: core.Style{Padding: core.All(8), Gap: 13}, Children: []core.Node{
			section("下载状态", static(statusFilters)),
			section("分类筛选", static(categoryFilters)),
			section("队列", []core.Node{core.Each(queues, func(key string) string { return key }, func(_ *core.Scope, key func() string, _ func() int) core.Node {
				return item(filter{"queue", key(), queueLabel(key())})
			})}, ui.IconButton{TestID: "new-queue", Label: "新建队列", Icon: lucide.Plus, Size: 22, OnPress: openQueue}),
		}}}}
}

func canPause(job jobView) bool {
	return job.State == download.Downloading || job.State == download.Queued
}
func canResume(job jobView) bool { return job.State == download.Paused || job.State == download.Failed }
func anyJob(jobView) bool        { return true }

func statusBar(colors func() palette, rows, selected, all func() []jobView, message func() string) core.Node {
	count := label("", 11, func() core.Color { return colors().muted })
	count.TestID = "selection-count"
	count.ContentFunc = func() string { return fmt.Sprintf("%d 项任务 · 已选 %d 项", len(rows()), len(selected())) }
	err := label("", 11, func() core.Color { return errorColor(colors()) })
	err.ContentFunc = message
	err.Style.FlexShrink = 1
	speed := label("", 11, func() core.Color { return colors().ink })
	speed.ContentFunc = func() string {
		var bytes float64
		var downloading, queued int
		for _, job := range all() {
			bytes += job.BytesPerSecond
			if job.State == download.Downloading {
				downloading++
			} else if job.State == download.Queued {
				queued++
			}
		}
		return fmt.Sprintf("%s/s · 下载中 %d · 排队 %d", formatBytes(bytes), downloading, queued)
	}
	return core.View{Style: core.Style{Height: core.Pt(28), Direction: core.Row, AlignItems: core.AlignCenter,
		JustifyContent: core.JustifySpaceBetween, Gap: 10, Padding: core.Symmetric(0, 12), Borders: core.Edges{Top: 1}},
		StyleFunc: func() core.Style { return core.Style{Background: colors().chrome, BorderColor: colors().line} },
		Children: []core.Node{count, err, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4},
			Children: []core.Node{ui.Icon(lucide.ArrowDown, 14, func() core.Color { return colors().ink }), speed}}}}
}

func protocol(job jobView) (scheme, host string) {
	u, err := url.Parse(job.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ""
	}
	return strings.ToUpper(u.Scheme), u.Hostname()
}

func formatBytes(bytes float64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	for i, unit := range units {
		if bytes < 1024 || i == len(units)-1 {
			if i == 0 {
				return fmt.Sprintf("%.0f %s", bytes, unit)
			}
			return fmt.Sprintf("%.1f %s", bytes, unit)
		}
		bytes /= 1024
	}
	return ""
}

func compactButton(id, text string, action func()) ui.Button {
	return ui.Button{TestID: id, Text: text, Size: ui.SizeSmall, Variant: ui.ButtonOutline, OnPress: action}
}
