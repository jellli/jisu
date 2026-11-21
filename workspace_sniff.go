package main

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jellli/jisu/internal/download"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

func workspaceSniff(cx *core.Scope, queue *download.Queue, colors func() palette, active func() filter,
	run func(func() error) bool, notify func(string)) (ui.Button, core.Node) {
	open := core.NewSignal(cx, false)
	pageURL := core.NewSignal(cx, "")
	status := core.NewSignal(cx, "")
	busy := core.NewSignal(cx, false)
	resources := core.NewSignalFunc(cx, []download.PageResource(nil), slices.Equal)
	selected := core.NewSignalFunc(cx, map[string]bool(nil), maps.Equal)
	kind := core.NewSignal(cx, "")
	sizes := core.NewSignalFunc(cx, map[string]int64(nil), maps.Equal)
	visible := func() []download.PageResource { return sniffResources(resources.Get(), kind.Get()) }
	parent := cx.Context()
	var cancelScan context.CancelFunc
	cx.Cleanup(func() {
		if cancelScan != nil {
			cancelScan()
		}
	})
	destination := download.DefaultQueue
	closeDialog := func() {
		if cancelScan != nil {
			cancelScan()
			cancelScan = nil
		}
		open.Set(false)
	}
	openDialog := func() {
		destination = download.DefaultQueue
		if view := active(); view.group == "queue" {
			destination = view.key
		}
		notify("")
		pageURL.Set("")
		status.Set("")
		resources.Set(nil)
		selected.Set(nil)
		kind.Set("")
		sizes.Set(nil)
		busy.Set(false)
		open.Set(true)
	}
	scan := func() {
		raw := strings.TrimSpace(pageURL.Get())
		if busy.Get() || raw == "" {
			return
		}
		if cancelScan != nil {
			cancelScan()
		}
		sizes.Set(nil)
		ctx, cancel := context.WithCancel(parent)
		cancelScan = cancel
		resources.Set(nil)
		selected.Set(nil)
		busy.Set(true)
		status.Set("正在嗅探网页…")
		go func() {
			found, err := queue.SniffPage(ctx, raw)
			cx.Post(func() {
				if ctx.Err() != nil {
					return
				}
				cancel()
				cancelScan = nil
				busy.Set(false)
				switch {
				case err != nil:
					status.Set("嗅探失败：" + err.Error())
				case len(found) == 0:
					status.Set("未找到可下载的直链文件。")
				default:
					resources.Set(found)
					status.Set(fmt.Sprintf("找到 %d 个资源，将下载至%s。", len(found), queueLabel(destination)))
					probeCtx, probeCancel := context.WithCancel(parent)
					cancelScan = probeCancel
					go func() {
						for _, item := range found {
							if probeCtx.Err() != nil {
								return
							}
							info, err := queue.Probe(probeCtx, item.URL)
							if err == nil && info.Size != nil {
								size := *info.Size
								cx.Post(func() {
									if probeCtx.Err() == nil {
										next := maps.Clone(sizes.Get())
										if next == nil {
											next = make(map[string]int64)
										}
										next[item.URL] = size
										sizes.Set(next)
									}
								})
							}
						}
					}()
				}
			})
		}()
	}
	add := func() {
		remaining := maps.Clone(selected.Get())
		if len(remaining) == 0 {
			return
		}
		count := len(remaining)
		var addErr error
		ok := run(func() error {
			for _, item := range resources.Get() {
				if !remaining[item.URL] {
					continue
				}
				if addErr = queue.AddToQueue(item.URL, destination); addErr != nil {
					return addErr
				}
				delete(remaining, item.URL)
			}
			return nil
		})
		selected.Set(remaining)
		if ok {
			closeDialog()
			notify(fmt.Sprintf("已添加 %d 个下载任务。", count))
		} else {
			status.Set("无法添加下载任务：" + addErr.Error())
		}
	}
	start := compactButton("sniff-start", "开始嗅探", scan)
	start.Disabled = func() bool { return busy.Get() || strings.TrimSpace(pageURL.Get()) == "" }
	downloadButton := compactButton("sniff-download", "下载所选", add)
	downloadButton.Variant = ui.ButtonPrimary
	downloadButton.Disabled = func() bool { return busy.Get() || len(selected.Get()) == 0 }
	statusText := label("", 12, func() core.Color { return colors().ink })
	statusText.ContentFunc = status.Get
	statusText.MaxLines = 0
	return compactButton("sniff-page", "嗅探网页", openDialog), ui.Dialog{
		Open: open.Get, OnOpenChange: func(value bool, _ base.Reason) {
			if !value {
				closeDialog()
			}
		},
		TestID: "sniff-dialog", Title: "网页嗅探", Description: "粘贴网页网址查找可下载的直链文件，不支持 HLS（m3u8）流。",
		Style: core.Style{Width: core.Pt(620)},
		Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8},
				Children: []core.Node{
					core.Input{TestID: "sniff-url", Value: pageURL.Get, OnChange: pageURL.Set, OnSubmit: func(string) { scan() },
						Placeholder: "https://", AutoFocus: true, Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0),
							Height: core.Pt(34), FontSize: 13, BorderWidth: 1, CornerRadius: 5, Padding: core.Symmetric(0, 8)},
						StyleFunc: func() core.Style {
							p := colors()
							return core.Style{BorderColor: p.line, Background: p.paper, Color: p.ink}
						}},
					start,
				}},
			statusText,
			core.Show{When: func() bool { return len(resources.Get()) > 0 }, Then: func(*core.Scope) core.Node {
				return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
					ui.Checkbox{TestID: "sniff-select-all", Text: "全选当前类型", Checked: func() bool {
						return len(visible()) > 0 && !slices.ContainsFunc(visible(), func(item download.PageResource) bool { return !selected.Get()[item.URL] })
					}, OnChange: func(checked bool) {
						next := maps.Clone(selected.Get())
						if next == nil {
							next = make(map[string]bool)
						}
						for _, item := range visible() {
							if checked {
								next[item.URL] = true
							} else {
								delete(next, item.URL)
							}
						}
						selected.Set(next)
					}},
					ui.Select[string]{TestID: "sniff-type", Label: "资源类型", Options: sniffTypeOptions, Value: kind.Get, OnChange: kind.Set},
				}}
			}},
			core.Show{When: func() bool { return len(resources.Get()) != 0 }, Then: func(*core.Scope) core.Node {
				return core.List{TestID: "sniff-results", Style: core.Style{Height: core.Pt(226), FlexShrink: 1},
					EstimatedSize: 54, Count: func() int { return len(visible()) },
					Row: func(_ *core.Scope, row core.ListRow) core.Node {
						return core.Dynamic(func(*core.Scope) core.Node {
							i, items := row.Index(), visible()
							if i < 0 || i >= len(items) {
								return core.View{}
							}
							item := items[i]
							name := label("", 13, func() core.Color { return colors().ink })
							name.ContentFunc = func() string {
								if size, ok := sizes.Get()[item.URL]; ok {
									return item.Name + " · " + formatBytes(float64(size))
								}
								return item.Name
							}
							address := label(item.URL, 11, func() core.Color { return colors().muted })
							address.MaxLines = 2
							return ui.Checkbox{TestID: fmt.Sprintf("sniff-resource-%d", i), Label: item.Name,
								Style:   core.Style{MinHeight: core.Pt(50)},
								Checked: func() bool { return selected.Get()[item.URL] },
								OnChange: func(checked bool) {
									picked := selected.Get()
									next := make(map[string]bool, len(picked)+1)
									maps.Copy(next, picked)
									if checked {
										next[item.URL] = true
									} else {
										delete(next, item.URL)
									}
									selected.Set(next)
								},
								Children: []core.Node{core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0)},
									Children: []core.Node{name, address}}},
							}
						})
					}}
			}},
		},
		Footer: []core.Node{compactButton("sniff-close", "关闭", closeDialog), downloadButton},
	}
}

var sniffTypeOptions = []ui.Option[string]{
	{Label: "全部类型", Value: ""},
	{Label: "视频", Value: "video"},
	{Label: "音频", Value: "audio"},
	{Label: "压缩包", Value: "archive"},
	{Label: "安装镜像", Value: "iso"},
	{Label: "文档", Value: "document"},
	{Label: "其他文件", Value: "other"},
}

func sniffResources(items []download.PageResource, kind string) []download.PageResource {
	if kind == "" {
		return items
	}
	var result []download.PageResource
	for _, item := range items {
		category := fileCategory(item.Name)
		switch strings.ToLower(filepath.Ext(item.Name)) {
		case ".mp3", ".flac", ".wav", ".m4a", ".ogg":
			category = "audio"
		}
		if category == kind {
			result = append(result, item)
		}
	}
	return result
}
