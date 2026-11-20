package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jellli/jisu/internal/download"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
	"github.com/nsgo-dev/nsgo/updater"
)

func workspaceControls(cx *core.Scope, queue *download.Queue, colors func() palette, directory func() string, concurrency func() int,
	queues func() []string, active func() filter, search *core.Signal[string], inspector, dark *core.Signal[bool],
	message *core.Signal[string], remove *core.Signal[bool], selected, all func() []jobView, run func(func() error) bool,
	apply func([]jobView, func(jobView) bool, func(string) error)) (core.Node, []core.Node) {
	newJob, settings, newQueue := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
	url, destination, name := core.NewSignal(cx, ""), core.NewSignal(cx, download.DefaultQueue), core.NewSignal(cx, "")
	proxyMode, proxyURL := core.NewSignal(cx, "system"), core.NewSignal(cx, "")
	applyProxy := func() {
		run(func() error {
			return queue.SetProxy(download.ProxySettings{Mode: proxyMode.Get(), URL: proxyURL.Get()})
		})
	}
	openNew := func() {
		message.Set("")
		url.Set("")
		view := active()
		destination.Set(download.DefaultQueue)
		if view.group == "queue" {
			destination.Set(view.key)
		}
		newJob.Set(true)
	}
	add := func() {
		if run(func() error { return queue.AddToQueue(url.Get(), destination.Get()) }) {
			newJob.Set(false)
		}
	}
	create := func() {
		if run(func() error { return queue.CreateQueue(strings.TrimSpace(name.Get())) }) {
			newQueue.Set(false)
		}
	}
	cx.OnKey("mod+n", openNew)
	newButton := compactButton("new-download", "＋ 新建下载", openNew)
	newButton.Variant = ui.ButtonPrimary
	resume := compactButton("resume-all", "▶ 开始全部", func() { apply(all(), canResume, queue.Resume) })
	resume.Disabled = func() bool { return !slices.ContainsFunc(all(), canResume) }
	pause := compactButton("pause-all", "Ⅱ 暂停全部", func() { apply(all(), canPause, queue.Pause) })
	pause.Disabled = func() bool { return !slices.ContainsFunc(all(), canPause) }
	trash := compactButton("delete", "删除", func() { remove.Set(true) })
	trash.Disabled = func() bool { return len(selected()) == 0 }
	brand := label("jisu", 15, func() core.Color { return colors().ink })
	brand.Style.FontWeight = 700
	brand.Style.Margin = core.Edges{Right: 4}
	searchField := core.Input{TestID: "search", Placeholder: "搜索任务、域名或协议...", Value: search.Get, OnChange: search.Set,
		Style: core.Style{Width: core.Pt(216), MinWidth: core.Pt(115), FlexShrink: 1, Height: core.Pt(26), Padding: core.Symmetric(0, 8),
			FontSize: 12, BorderWidth: 1, CornerRadius: 5},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors().paper, Color: colors().ink, BorderColor: colors().line}
		}}
	toolbar := core.View{Style: core.Style{Height: core.Pt(44), Direction: core.Row, AlignItems: core.AlignCenter,
		Padding: core.Symmetric(0, 12), Borders: core.Edges{Bottom: 1}, Gap: 6},
		StyleFunc: func() core.Style { return core.Style{Background: colors().chrome, BorderColor: colors().line} },
		Children: []core.Node{
			brand, newButton, resume, pause, trash,
			core.View{Style: core.Style{FlexGrow: 1}}, searchField,
			compactButton("settings", "设置", func() {
				message.Set("")
				p := queue.State().Proxy
				proxyMode.Set(p.Mode)
				proxyURL.Set(p.URL)
				settings.Set(true)
			}),
			compactButton("theme", "主题", func() {
				dark.Set(!dark.Get())
				if dark.Get() {
					cx.SetAppearance(core.AppearanceDark)
				} else {
					cx.SetAppearance(core.AppearanceLight)
				}
			}),
			compactButton("toggle-inspector", "详情", func() { inspector.Set(!inspector.Get()) }),
		}}

	changeOpen := func(open *core.Signal[bool]) func(bool, base.Reason) {
		return func(value bool, _ base.Reason) { open.Set(value) }
	}
	errorLabel := label("", 12, func() core.Color { return core.RGB(0xdc2626) })
	errorLabel.ContentFunc = message.Get
	settingsError := errorLabel
	settingsError.MaxLines = 0
	settingsError.TestID = "settings-error"
	input := func(id, placeholder string, signal *core.Signal[string], submit func()) core.Node {
		return core.Input{TestID: id, Value: signal.Get, OnChange: signal.Set, Placeholder: placeholder, AutoFocus: true,
			OnSubmit: func(string) { submit() }, Style: core.Style{Height: core.Pt(34), FontSize: 13,
				BorderWidth: 1, CornerRadius: 5, Padding: core.Symmetric(0, 8)},
			StyleFunc: func() core.Style {
				return core.Style{BorderColor: colors().line, Background: colors().paper, Color: colors().ink}
			}}
	}
	confirm := func(id, caption string, action func()) ui.Button {
		b := compactButton(id, caption, action)
		b.Variant = ui.ButtonPrimary
		return b
	}
	cancel := func(open *core.Signal[bool]) ui.Button {
		return compactButton("", "取消", func() { open.Set(false) })
	}
	var choices []ui.Option[int]
	for i := 1; i <= 5; i++ {
		choices = append(choices, ui.Option[int]{Value: i, Label: fmt.Sprint(i)})
	}
	folder := label("", 12, func() core.Color { return colors().muted })
	folder.ContentFunc = directory
	return toolbar, []core.Node{
		ui.Dialog{Open: newJob.Get, OnOpenChange: changeOpen(newJob), Title: "新建下载", TestID: "add-dialog",
			Description: "输入 HTTP 或 HTTPS 下载链接",
			Children: []core.Node{input("download-url", "https://", url, add), core.Dynamic(func(*core.Scope) core.Node {
				names := queues()
				options := make([]ui.Option[string], len(names))
				for i, key := range names {
					options[i] = ui.Option[string]{Label: queueLabel(key), Value: key}
				}
				return ui.Select[string]{Label: "队列", Options: options, Value: destination.Get, OnChange: destination.Set, TestID: "download-queue"}
			}), errorLabel},
			Footer: []core.Node{cancel(newJob), confirm("add-download", "开始下载", add)}},
		ui.Dialog{Open: settings.Get, OnOpenChange: changeOpen(settings), Title: "设置", TestID: "settings-dialog",
			Children: []core.Node{
				label(versionCaption(), 12, func() core.Color { return colors().muted }),
				compactButton("check-update", "检查更新", func() { updater.Check(cx) }),
				label("下载目录", 12, func() core.Color { return colors().ink }), folder,
				compactButton("choose-folder", "选择文件夹", func() {
					cx.PickDirectories(core.PickDirectoriesOptions{Title: "选择下载目录", Start: directory()}, func(dirs []core.Directory, err error) {
						if err != nil {
							message.Set(err.Error())
							return
						}
						if len(dirs) > 0 {
							run(func() error { return queue.SetDirectory(dirs[0].Path) })
						}
					})
				}),
				label("同时下载任务数", 12, func() core.Color { return colors().ink }),
				ui.Select[int]{Label: "同时下载任务数", Options: choices, Value: concurrency, OnChange: func(n int) {
					run(func() error { return queue.SetConcurrency(n) })
				}},
				compactButton("new-queue", "新建队列", func() { settings.Set(false); message.Set(""); name.Set(""); newQueue.Set(true) }),
				label("代理服务器", 12, func() core.Color { return colors().ink }),
				ui.Select[string]{TestID: "proxy-mode", Label: "Proxy", Value: proxyMode.Get,
					Options: []ui.Option[string]{{Value: "system", Label: "System"}, {Value: "none", Label: "None"}, {Value: "custom", Label: "Custom"}},
					OnChange: func(mode string) {
						proxyMode.Set(mode)
						message.Set("")
					}},
				core.Show{When: func() bool { return proxyMode.Get() == "custom" }, Then: func(*core.Scope) core.Node {
					field := input("proxy-url", "http://host:port", proxyURL, applyProxy).(core.Input)
					field.Masked = true
					return field
				}},
				settingsError,
			}, Footer: []core.Node{confirm("apply-proxy", "应用代理", applyProxy), compactButton("", "关闭", func() { settings.Set(false) })}},
		ui.Dialog{Open: newQueue.Get, OnOpenChange: changeOpen(newQueue), Title: "新建队列",
			Children: []core.Node{input("queue-name", "队列名称", name, create), errorLabel},
			Footer:   []core.Node{cancel(newQueue), confirm("create-queue", "创建", create)}},
		ui.Dialog{Open: remove.Get, OnOpenChange: changeOpen(remove), Title: "删除所选任务？",
			Description: "未完成的下载数据将被删除。已完成的文件不会被删除。",
			Footer: []core.Node{cancel(remove), confirm("confirm-delete", "删除", func() {
				remove.Set(false)
				apply(selected(), anyJob, queue.Remove)
			})}},
	}
}

func versionCaption() string {
	if version := core.App().Version; version != "" {
		return "当前版本 v" + version
	}
	return "当前版本（开发版）"
}

func detailPanel(cx *core.Scope, queue *download.Queue, colors func() palette, selected func() []jobView, run func(func() error) bool) core.Node {
	tab := core.NewSignal(cx, false)
	header := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(34), Gap: 6,
		Padding: core.Symmetric(0, 10), Borders: core.Edges{Bottom: 1}},
		StyleFunc: func() core.Style { return core.Style{Background: colors().chrome, BorderColor: colors().line} },
		Children: []core.Node{
			compactButton("segments-tab", "分块与线程", func() { tab.Set(false) }),
			compactButton("file-tab", "文件", func() { tab.Set(true) }),
		}}
	first := func() jobView {
		jobs := selected()
		if len(jobs) == 1 {
			return jobs[0]
		}
		return jobView{}
	}
	summary := label("", 12, func() core.Color { return colors().muted })
	summary.ContentFunc = func() string {
		if n := len(selected()); n != 0 {
			return fmt.Sprintf("已选 %d 项任务", n)
		}
		return "请在上方列表选择一个任务以检视详情"
	}
	name := label("", 12, func() core.Color { return colors().ink })
	name.ContentFunc = func() string { return first().Name }
	connections := label("", 11, func() core.Color { return colors().muted })
	connections.TestID = "live-connections"
	connections.ContentFunc = func() string {
		job := first()
		return fmt.Sprintf("活跃连接 %d / %d", job.Connections, job.target)
	}
	progress := label("", 12, func() core.Color { return colors().muted })
	progress.ContentFunc = func() string {
		j := first()
		return "已下载 " + formatBytes(float64(j.Received)) + " / " + cellText(j, "size")
	}
	failure := label("", 12, func() core.Color { return core.RGB(0xdc2626) })
	failure.ContentFunc = func() string { return first().Error }
	var buttons []core.Node
	for _, n := range []int{4, 8, 16, 32} {
		button := compactButton(fmt.Sprintf("connections-%d", n), fmt.Sprint(n), func() {
			run(func() error { return queue.SetJobConnections(first().ID, n) })
		})
		button.Disabled = func() bool { return first().State == download.Completed }
		buttons = append(buttons, button)
	}
	path := label("", 12, func() core.Color { return colors().muted })
	path.ContentFunc = func() string {
		job := first()
		if job.State == download.Completed {
			if final, err := queue.CompletedPath(job.ID); err == nil {
				return final
			}
		}
		return job.Directory
	}
	return core.View{TestID: "inspector", Style: core.Style{Height: core.Pt(240), Borders: core.Edges{Top: 1}},
		StyleFunc: func() core.Style { return core.Style{BorderColor: colors().line} },
		Children: []core.Node{header, core.View{Style: core.Style{Gap: 12, Padding: core.All(12)}, Children: []core.Node{
			core.Show{When: func() bool { return len(selected()) == 1 }, Else: func(*core.Scope) core.Node { return summary },
				Then: func(*core.Scope) core.Node {
					return core.View{Style: core.Style{Gap: 11}, Children: []core.Node{name,
						core.Show{When: tab.Get,
							Then: func(*core.Scope) core.Node {
								reveal := compactButton("reveal", "在文件夹中显示", func() {
									final, err := queue.CompletedPath(first().ID)
									if err != nil {
										run(func() error { return err })
										return
									}
									cx.RevealFile(final, func(err error) {
										if err != nil {
											run(func() error { return err })
										}
									})
								})
								reveal.Disabled = func() bool { return first().State != download.Completed }
								return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{path, reveal}}
							},
							Else: func(*core.Scope) core.Node {
								return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
									core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6},
										Children: append([]core.Node{progress, core.View{Style: core.Style{FlexGrow: 1}}, connections}, buttons...)},
									progressBars(colors, first, 10), failure,
								}}
							}},
					}}
				}},
		}}}}
}
