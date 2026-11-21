package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jellli/jisu/internal/download"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
	"github.com/nsgo-dev/nsgo/updater"
)

func workspaceControls(cx *core.Scope, queue *download.Queue, colors func() palette, directory func() string, concurrency func() int,
	queues func() []string, active func() filter, search *core.Signal[string], inspector, dark *core.Signal[bool],
	message *core.Signal[string], remove *core.Signal[bool], selected, all func() []jobView, run func(func() error) bool,
	apply func([]jobView, func(jobView) bool, func(string) error)) (core.Node, []core.Node, core.Node, func()) {
	newJob, newQueue := core.NewSignal(cx, false), core.NewSignal(cx, false)
	url, destination, name := core.NewSignal(cx, ""), core.NewSignal(cx, download.DefaultQueue), core.NewSignal(cx, "")
	setDark := func(value bool) {
		dark.Set(value)
		if value {
			cx.SetAppearance(core.AppearanceDark)
		} else {
			cx.SetAppearance(core.AppearanceLight)
		}
	}
	openSettings, settingsDialog := workspaceSettings(cx, queue, colors, directory, concurrency, dark.Get, setDark, message, run)
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
	openQueue := func() {
		message.Set("")
		name.Set("")
		newQueue.Set(true)
	}
	cx.OnKey("mod+n", openNew)
	newButton := compactButton("new-download", "新建下载", openNew)
	newButton.Icon = lucide.Plus
	newButton.Variant = ui.ButtonPrimary
	sniffButton, sniffDialog := workspaceSniff(cx, queue, colors, active, run, message.Set)
	sniffButton.Icon = lucide.Radar
	entry := func(id, caption string, action func()) core.Node {
		button := compactButton(id, caption, action)
		button.Size = ui.SizeLarge
		if id == "empty-new" {
			button.Variant = ui.ButtonPrimary
		}
		button.Style.Width = core.Pt(260)
		return button
	}
	title := label("还没有下载任务", 18, func() core.Color { return colors().ink })
	title.Style.FontWeight = 600
	explanation := label("粘贴一个下载链接，或让 jisu 从网页里找出可下载的文件", 12, func() core.Color { return colors().muted })
	empty := core.View{TestID: "empty-state", Style: core.Style{Gap: 14, AlignItems: core.AlignCenter}, Children: []core.Node{
		title, explanation,
		entry("empty-new", "粘贴链接", openNew),
		entry("empty-sniff", "嗅探网页", sniffButton.OnPress),
		entry("empty-extension", "安装浏览器插件", func() {
			cx.OpenURL("https://github.com/jellli/jisu#browser-takeover", func(ok bool) {
				if !ok {
					message.Set("无法打开浏览器插件安装说明")
				}
			})
		}),
	}}
	resume := compactButton("resume-all", "开始全部", func() { apply(all(), canResume, queue.Resume) })
	resume.Icon = lucide.Play
	resume.Disabled = func() bool { return !slices.ContainsFunc(all(), canResume) }
	pause := compactButton("pause-all", "暂停全部", func() { apply(all(), canPause, queue.Pause) })
	pause.Icon = lucide.Pause
	pause.Disabled = func() bool { return !slices.ContainsFunc(all(), canPause) }
	trash := ui.IconButton{TestID: "delete", Label: "删除", Icon: lucide.Trash, OnPress: func() { remove.Set(true) }}
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
			brand, newButton, sniffButton, resume, pause, trash,
			core.View{Style: core.Style{FlexGrow: 1}}, searchField,
			ui.IconButton{TestID: "settings", Label: "设置", Icon: lucide.Settings, OnPress: openSettings},
			ui.IconButton{TestID: "theme", Label: "主题", Icon: func() *core.SVG {
				if dark.Get() {
					return lucide.Sun()
				}
				return lucide.Moon()
			}, OnPress: func() { setDark(!dark.Get()) }},
			ui.IconButton{TestID: "toggle-inspector", Label: "详情", Icon: lucide.PanelRight, OnPress: func() { inspector.Set(!inspector.Get()) }},
		}}

	changeOpen := func(open *core.Signal[bool]) func(bool, base.Reason) {
		return func(value bool, _ base.Reason) { open.Set(value) }
	}
	errorLabel := label("", 12, func() core.Color { return core.RGB(0xdc2626) })
	errorLabel.ContentFunc = message.Get
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
	return toolbar, []core.Node{
		sniffDialog,
		settingsDialog,
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
		ui.Dialog{Open: newQueue.Get, OnOpenChange: changeOpen(newQueue), Title: "新建队列",
			Children: []core.Node{input("queue-name", "队列名称", name, create), errorLabel},
			Footer:   []core.Node{cancel(newQueue), confirm("create-queue", "创建", create)}},
		ui.Dialog{Open: remove.Get, OnOpenChange: changeOpen(remove), Title: "删除所选任务？",
			Description: "未完成的下载数据将被删除。已完成的文件不会被删除。",
			Footer: []core.Node{cancel(remove), confirm("confirm-delete", "删除", func() {
				remove.Set(false)
				apply(selected(), anyJob, queue.Remove)
			})}},
	}, empty, openQueue
}

func workspaceSettings(cx *core.Scope, queue *download.Queue, colors func() palette, directory func() string, concurrency func() int,
	dark func() bool, setDark func(bool), message *core.Signal[string], run func(func() error) bool) (func(), core.Node) {
	open := core.NewSignal(cx, false)
	proxyMode, proxyURL := core.NewSignal(cx, "system"), core.NewSignal(cx, "")
	proxyError := core.NewSignal(cx, "")
	var savedProxy download.ProxySettings
	show := func() {
		message.Set("")
		proxyError.Set("")
		savedProxy = queue.State().Proxy
		proxyMode.Set(savedProxy.Mode)
		proxyURL.Set(savedProxy.URL)
		open.Set(true)
	}
	saveProxy := func(next download.ProxySettings) bool {
		proxyError.Set("")
		if next != savedProxy {
			if err := queue.SetProxy(next); err != nil {
				proxyError.Set(err.Error())
				return false
			}
			savedProxy = queue.State().Proxy
		}
		proxyMode.Set(savedProxy.Mode)
		proxyURL.Set(savedProxy.URL)
		return true
	}
	commitCustom := func() bool {
		return !open.Get() || proxyMode.Get() != "custom" ||
			saveProxy(download.ProxySettings{Mode: "custom", URL: proxyURL.Get()})
	}
	selectMode := func(mode string) {
		if mode == "custom" {
			proxyError.Set("")
			proxyMode.Set(mode)
			return
		}
		saveProxy(download.ProxySettings{Mode: mode})
	}
	dismiss := compactButton("close-settings", "关闭", func() {
		if commitCustom() {
			open.Set(false)
		}
	})
	chooseFolder := compactButton("choose-folder", "选择文件夹", func() {
		cx.PickDirectories(core.PickDirectoriesOptions{Title: "选择下载目录", Start: directory()}, func(dirs []core.Directory, err error) {
			if err != nil {
				message.Set(err.Error())
			} else if len(dirs) > 0 {
				run(func() error { return queue.SetDirectory(dirs[0].Path) })
			}
		})
	})
	chooseFolder.Icon = lucide.FolderOpen
	folder := label("", 12, func() core.Color { return colors().muted })
	folder.ContentFunc = directory
	folder.EllipsizeMode = core.EllipsizeMiddle
	folder.Style.FlexGrow, folder.Style.FlexShrink, folder.Style.MinWidth = 1, 1, core.Pt(0)
	folderControl := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter,
		Width: core.Pt(305), Gap: 10}, Children: []core.Node{folder, chooseFolder}}
	var choices []ui.Option[int]
	for i := 1; i <= 5; i++ {
		choices = append(choices, ui.Option[int]{Value: i, Label: fmt.Sprint(i)})
	}
	errorText := func(id string, content func() string) core.Node {
		return core.Show{When: func() bool { return content() != "" }, Then: func(*core.Scope) core.Node {
			text := label("", 12, func() core.Color {
				if dark() {
					return core.RGB(0xfca5a5)
				}
				return core.RGB(0xb91c1c)
			})
			text.TestID, text.ContentFunc, text.MaxLines = id, content, 0
			text.Style.Padding = core.Symmetric(0, 12)
			return text
		}}
	}
	pickerStyle := core.Style{Width: core.Pt(146), Height: core.Pt(26), FontSize: 12}
	return show, ui.Dialog{Open: open.Get, Title: "设置", TestID: "settings-dialog", NoCloseOnBackdropPress: true,
		OnOpenChange: func(value bool, _ base.Reason) { open.Set(value) },
		Style:        core.Style{Width: core.Pt(520)},
		StyleFunc:    func() core.Style { return core.Style{Background: colors().paper} },
		Children: []core.Node{core.Scroll{Style: core.Style{FlexShrink: 1, MinHeight: core.Pt(0)},
			Children: []core.Node{core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
				ui.FormSection{Title: "常规", Children: []core.Node{
					ui.FormRow{Title: "深色模式", Control: ui.Switch{TestID: "settings-theme", Label: "深色模式", Checked: dark, OnChange: setDark, Size: ui.SizeSmall}},
				}},
				ui.FormSection{Title: "下载", Children: []core.Node{
					ui.FormRow{Title: "下载目录", Control: folderControl},
					ui.FormRow{Title: "同时下载任务数", Control: ui.Select[int]{TestID: "download-concurrency", Label: "同时下载任务数",
						Style: pickerStyle, Options: choices, Value: concurrency, OnChange: func(n int) {
							run(func() error { return queue.SetConcurrency(n) })
						}}},
				}},
				ui.FormSection{Title: "网络", Children: []core.Node{core.View{Children: []core.Node{
					ui.FormRow{Title: "代理服务器", Control: ui.Select[string]{TestID: "proxy-mode", Label: "代理服务器",
						Style: pickerStyle, Value: proxyMode.Get, OnChange: selectMode,
						Options: []ui.Option[string]{{Value: "system", Label: "跟随系统"}, {Value: "none", Label: "不使用"}, {Value: "custom", Label: "自定义"}}}},
					core.Show{When: func() bool { return proxyMode.Get() == "custom" }, Then: func(*core.Scope) core.Node {
						return ui.FormRow{Title: "代理地址", Detail: "回车或离开输入框后生效",
							Control: core.Input{TestID: "proxy-url", Label: "代理地址", Placeholder: "http://host:port", Masked: true,
								Value: proxyURL.Get, Invalid: func() bool { return proxyError.Get() != "" },
								OnChange: func(value string) { proxyURL.Set(value); proxyError.Set("") },
								OnSubmit: func(string) { commitCustom() }, OnBlur: func() { commitCustom() },
								Style: core.Style{Width: core.Pt(230), Height: core.Pt(28), Padding: core.Symmetric(0, 8),
									FontSize: 12, BorderWidth: 1, CornerRadius: 5},
								StyleFunc: func() core.Style {
									border := colors().line
									if proxyError.Get() != "" {
										border = core.RGB(0xdc2626)
									}
									return core.Style{BorderColor: border, Background: colors().paper, Color: colors().ink}
								}}}
					}},
					errorText("settings-proxy-error", proxyError.Get),
				}}}},
				ui.FormSection{Title: "更新", Children: []core.Node{
					ui.FormRow{Title: versionCaption(), Control: compactButton("check-update", "检查更新", func() { updater.Check(cx) })},
				}},
				errorText("settings-error", message.Get),
			}}},
		}}, Footer: []core.Node{dismiss}}
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
		Children: []core.Node{ui.Tabs[bool]{Variant: ui.TabsUnderline, Size: ui.SizeSmall, Label: "任务详情", TestID: "detail-tabs",
			Value: tab.Get, OnChange: tab.Set, Options: []ui.Option[bool]{
				{Value: false, Label: "分块与线程", TestID: "segments-tab"},
				{Value: true, Label: "文件", TestID: "file-tab"},
			}}}}
	first := func() jobView {
		jobs := selected()
		if len(jobs) == 1 {
			return jobs[0]
		}
		return jobView{}
	}
	proxyMode, proxyURL := core.NewSignal(cx, ""), core.NewSignal(cx, "")
	proxyError := core.NewSignal(cx, "")
	jobID := core.NewMemo(cx, func() string { return first().ID })
	cx.Effect(func() {
		id := jobID.Get()
		p := download.ProxySettings{}
		for _, job := range queue.State().Jobs {
			if job.ID == id {
				p = job.Proxy
				break
			}
		}
		proxyMode.Set(p.Mode)
		proxyURL.Set(p.URL)
		proxyError.Set("")
	})
	applyProxy := func() {
		err := queue.SetJobProxy(first().ID, download.ProxySettings{Mode: proxyMode.Get(), URL: proxyURL.Get()})
		proxyError.Set("")
		if err != nil {
			proxyError.Set(err.Error())
			return
		}
		run(func() error { return nil })
	}
	proxyFailure := label("", 12, func() core.Color { return core.RGB(0xdc2626) })
	proxyFailure.TestID = "job-proxy-error"
	proxyFailure.ContentFunc = proxyError.Get
	proxyFailure.MaxLines = 0
	proxySection := core.View{Style: core.Style{Gap: 6}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			label("任务代理", 12, func() core.Color { return colors().ink }),
			ui.Select[string]{TestID: "job-proxy-mode", Label: "任务代理", Value: proxyMode.Get,
				Options:  []ui.Option[string]{{Value: "", Label: "跟随全局"}, {Value: "system", Label: "系统"}, {Value: "none", Label: "不使用"}, {Value: "custom", Label: "自定义"}},
				OnChange: func(mode string) { proxyMode.Set(mode); proxyError.Set("") }},
			compactButton("apply-job-proxy", "应用代理", applyProxy),
		}},
		core.Show{When: func() bool { return proxyMode.Get() == "custom" }, Then: func(*core.Scope) core.Node {
			return core.Input{TestID: "job-proxy-url", Placeholder: "http://host:port", Value: proxyURL.Get, OnChange: proxyURL.Set,
				Masked: true, OnSubmit: func(string) { applyProxy() },
				Style: core.Style{Height: core.Pt(30), FontSize: 13, BorderWidth: 1, CornerRadius: 5, Padding: core.Symmetric(0, 8)},
				StyleFunc: func() core.Style {
					return core.Style{BorderColor: colors().line, Background: colors().paper, Color: colors().ink}
				}}
		}},
		proxyFailure,
	}}
	summary := label("", 12, func() core.Color { return colors().muted })
	summary.ContentFunc = func() string {
		if n := len(selected()); n != 0 {
			return fmt.Sprintf("多选任务：%d", n)
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
	progress.Style.MinWidth, progress.Style.FlexShrink = core.Pt(0), 1
	progress.ContentFunc = func() string {
		return downloadSummary(first())
	}
	failure := label("", 12, func() core.Color { return core.RGB(0xdc2626) })
	failure.ContentFunc = func() string { return failureReason(first().Error) }
	failure.MaxLines = 0
	var targets []ui.Option[int]
	for _, n := range []int{4, 8, 16, 32} {
		caption := fmt.Sprint(n)
		text := core.Text{Content: caption, MaxLines: 1, StyleFunc: func() core.Style {
			if first().target == n && first().State != download.Completed {
				p := colors()
				return core.Style{Color: ui.Mix(p.accent, p.ink, 0.35)}
			}
			return core.Style{}
		}}
		targets = append(targets, ui.Option[int]{Value: n, Label: caption, TestID: fmt.Sprintf("connections-%d", n), Content: text,
			Disabled: func() bool { return first().State == download.Completed }})
	}
	connectionControl := ui.Tabs[int]{Variant: ui.TabsSegmented, Size: ui.SizeSmall, Label: "连接数", TestID: "connection-target",
		Value: func() int { return first().target }, Options: targets, OnChange: func(n int) {
			run(func() error { return queue.SetJobConnections(first().ID, n) })
		}}
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
	url := label("", 11, func() core.Color { return colors().muted })
	url.ContentFunc = func() string { return first().URL }
	return core.View{TestID: "inspector", Style: core.Style{Height: core.Pt(320), Borders: core.Edges{Top: 1}},
		StyleFunc: func() core.Style { return core.Style{BorderColor: colors().line} },
		Children: []core.Node{header, core.View{Style: core.Style{Gap: 12, Padding: core.All(12)}, Children: []core.Node{
			core.Show{When: func() bool { return len(selected()) == 1 }, Else: func(*core.Scope) core.Node { return summary },
				Then: func(*core.Scope) core.Node {
					return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{name, url, path, failure,
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
								reveal.Icon = lucide.FolderOpen
								reveal.Disabled = func() bool { return first().State != download.Completed }
								return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{reveal}}
							},
							Else: func(*core.Scope) core.Node {
								return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
									core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6},
										Children: []core.Node{progress, core.View{Style: core.Style{FlexGrow: 1}}, connections, connectionControl}},
									progressBars(colors, first, 10),
								}}
							}}, proxySection,
					}}
				}},
		}}}}
}

func downloadSummary(job jobView) string {
	if job.ID == "" {
		return ""
	}
	parts := []string{"已下载 " + formatBytes(float64(job.Received)) + " / " + cellText(job, "size"), cellText(job, "progress"), cellText(job, "speed")}
	parts = slices.DeleteFunc(parts, func(part string) bool { return part == "" })
	return strings.Join(parts, " · ")
}
