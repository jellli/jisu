package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

func svg(path string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
}

var icons = map[string]*ui.SVG{
	"all":      svg(`<path d="M4 5h16v4H4zM4 13h16M4 17h16M4 21h16"/>`),
	"download": svg(`<path d="M12 3v13m-5-5 5 5 5-5M4 16v5h16v-5"/>`),
	"play":     svg(`<path d="m8 4 12 8-12 8z"/>`),
	"pause":    svg(`<path d="M8 5v14M16 5v14"/>`),
	"done":     svg(`<circle cx="12" cy="12" r="9"/><path d="m8 12 3 3 5-6"/>`),
	"error":    svg(`<circle cx="12" cy="12" r="9"/><path d="M12 7v6m0 4h.01"/>`),
	"archive":  svg(`<rect x="3" y="4" width="18" height="4" rx="1"/><path d="m5 8 1 12h12l1-12M10 12h4"/>`),
	"video":    svg(`<rect x="3" y="6" width="13" height="13" rx="2"/><path d="m16 10 5-4v13l-5-4"/>`),
	"iso":      svg(`<circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="3"/>`),
	"document": svg(`<path d="M5 3h9l5 5v13H5zM14 3v6h5M8 13h8M8 17h5"/>`),
	"queue":    svg(`<path d="M4 6h16M4 12h16M4 18h10"/>`),
	"delete":   svg(`<path d="M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7M14 10v7"/>`),
	"theme":    svg(`<path d="M20 15a9 9 0 0 1-11-11 9 9 0 1 0 11 11z"/>`),
	"pane":     svg(`<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 15h18"/>`),
	"plus":     svg(`<path d="M12 4v16M4 12h16"/>`),
}

func control(c *ui.Context, label, icon, text string) ui.Element {
	t := c.Theme()
	return ui.ButtonBase(c).Height(26).Padding(0, 8).Radius(5).Border(1, t.Border).
		Background(t.Surface).Label(label).Tooltip(label).Gap(5).Children(func() {
		if icon != "" {
			ui.Icon(c, icons[icon]).FontSize(14).TextColor(t.TextMuted)
		}
		if text != "" {
			ui.Text(c, text).FontSize(12)
		}
	})
}

func (s *prototype) view(c *ui.Context) {
	t := theme(s.dark)
	c.SetTheme(&t)
	s.tick(c.Now())
	if s.simulate {
		c.After(250 * time.Millisecond)
	}
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		s.toolbar(c)
		ui.Divider(c)
		s.reconcileSelection()
		ui.Row(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
			s.sidebar(c)
			ui.Column(c).Width(1).FillHeight().Background(t.Border)
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).FillHeight().Children(func() {
				if s.detailsOpen {
					ui.SplitVerticalPercent(c, &s.queueShare, func() { s.taskTable(c) }, func() { s.inspector(c) }).Fill()
				} else {
					s.taskTable(c)
				}
			})
		})
		ui.Divider(c)
		s.statusBar(c)
		if s.settings {
			s.demoSettings(c)
		}
	})
	ui.Modal(c, &s.adding, func() {
		ui.Column(c).Width(480).Gap(14).Children(func() {
			ui.Text(c, "新建下载").FontSize(16).FontWeight(500)
			ui.Text(c, "HTTP / HTTPS 链接，每行一个").FontSize(12).TextColor(t.TextMuted)
			ui.TextArea(c, &s.newURLs).Height(120).Placeholder("https://example.com/archive.zip").Label("下载链接")
			ui.Text(c, "仅添加 128 MB 模拟任务，不请求网络").FontSize(12).TextColor(t.TextMuted)
			if s.addError != "" {
				ui.Text(c, s.addError).FontSize(12).TextColor(s.errorColor())
			}
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					s.adding = false
				}
				if ui.PrimaryButton(c, "添加模拟任务").Clicked() && s.addTasks() {
					s.adding = false
				}
			})
		})
	})
}

func (s *prototype) toolbar(c *ui.Context) {
	t := c.Theme()
	width, _ := c.Size()
	ui.Row(c).Height(44).Shrink(0).Padding(0, 12).Gap(8).Background(t.Background).Children(func() {
		ui.Icon(c, icons["download"]).FontSize(16).TextColor(t.Accent)
		ui.Text(c, "Jisu").FontSize(14).FontWeight(500).Margin(0, 8, 0, 0)
		if ui.ButtonBase(c).Height(26).Padding(0, 10).Radius(5).Background(t.Accent).Label("新建下载").Gap(5).Children(func() {
			ui.Icon(c, icons["plus"]).FontSize(14).TextColor(t.Surface)
			ui.Text(c, "新建下载").FontSize(12).TextColor(t.Surface)
		}).Clicked() {
			s.adding = true
			s.addError = ""
		}
		start, pause := "全部开始", "全部暂停"
		if width < 1000 {
			start, pause = "", ""
		}
		ui.Row(c).Gap(0).Children(func() {
			if control(c, "全部开始", "play", start).Clicked() {
				s.setAll(downloading, false)
			}
			if control(c, "全部暂停", "pause", pause).Clicked() {
				s.setAll(paused, false)
			}
			if control(c, "移除选中模拟任务", "delete", "").Disabled(s.checkedCount() == 0).Clicked() {
				s.removeChecked()
			}
		})
		ui.Spacer(c)
		searchWidth := float32(240)
		if width < 1000 {
			searchWidth = 160
		}
		ui.TextInput(c, &s.query).Width(searchWidth).Height(26).FontSize(12).
			Placeholder("搜索任务、域名或协议…").Label("搜索任务、域名或协议").Search()
		if control(c, "切换浅色 / 深色外观", "theme", "").Clicked() {
			s.dark = !s.dark
		}
		if control(c, "展开 / 收起底部详情", "pane", "").Expanded(s.detailsOpen).Clicked() {
			s.detailsOpen = !s.detailsOpen
		}
	})
}

func (s *prototype) navItem(c *ui.Context, label, icon string, count int, active bool) bool {
	t := c.Theme()
	color := t.Text
	if active {
		color = t.Accent
	}
	b := ui.ButtonBase(c).Height(28).FillWidth().Padding(0, 8).Radius(5).Gap(8).Label(label).Children(func() {
		ui.Icon(c, icons[icon]).FontSize(14).TextColor(color)
		ui.Text(c, label).FontSize(12).TextColor(color)
		ui.Spacer(c)
		ui.Textf(c, "%d", count).FontSize(11).FontFeatures("tnum").TextColor(t.TextMuted)
	})
	if active {
		b.Background(t.Accent.Alpha(.10))
	}
	return b.Clicked()
}

func (s *prototype) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Width(200).Shrink(0).FillHeight().Background(t.Background).Children(func() {
		ui.Column(c).FillWidth().Padding(12, 8).Gap(2).Children(func() {
			ui.Text(c, "下载状态").FontSize(11).TextColor(t.TextMuted).Padding(0, 8, 6, 8)
			for _, f := range []queueFilter{allTasks, activeTasks, completeTasks, pausedTasks, attentionTasks} {
				label := map[queueFilter]string{allTasks: "全部任务", activeTasks: "正在下载", completeTasks: "已完成", pausedTasks: "已暂停", attentionTasks: "需处理"}[f]
				icon := map[queueFilter]string{allTasks: "all", activeTasks: "download", completeTasks: "done", pausedTasks: "pause", attentionTasks: "error"}[f]
				if s.navItem(c.Key(f), label, icon, s.count(f), s.filter == f && s.category == "" && s.queue == "") {
					s.filter, s.category, s.queue = f, "", ""
				}
			}
			ui.Text(c, "分类筛选").FontSize(11).TextColor(t.TextMuted).Padding(18, 8, 6, 8)
			for _, cat := range []struct{ key, label string }{{"video", "视频媒体"}, {"archive", "压缩包"}, {"iso", "安装镜像"}, {"document", "文档"}} {
				n := 0
				for _, task := range s.tasks {
					if categoryFor(task.ext) == cat.key {
						n++
					}
				}
				if s.navItem(c.Key(cat.key), cat.label, cat.key, n, s.category == cat.key) {
					s.filter, s.category, s.queue = allTasks, cat.key, ""
				}
			}
			ui.Text(c, "队列").FontSize(11).TextColor(t.TextMuted).Padding(18, 8, 6, 8)
			if s.navItem(c, "默认队列", "queue", len(s.tasks), s.queue == "default") {
				s.filter, s.category, s.queue = allTasks, "", "default"
			}
			if s.navItem(c, "夜间队列", "queue", 0, s.queue == "night") {
				s.filter, s.category, s.queue = allTasks, "", "night"
			}
		})
	})
}

func (s *prototype) taskTable(c *ui.Context) {
	t := c.Theme()
	ui.ScrollBoth(c).Fill().Children(func() {
		ui.Column(c).FillWidth().MinWidth(850).Children(func() {
			ui.Row(c).Height(28).Padding(0, 12).Background(t.Background).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				all := len(s.visibleTasks()) > 0
				for _, i := range s.visibleTasks() {
					if !s.checked[i] {
						all = false
					}
				}
				if ui.Checkbox(c, &all, "").Width(26).Label("选择全部可见任务").Changed() {
					for _, i := range s.visibleTasks() {
						s.checked[i] = all
					}
				}
				for _, col := range []struct {
					key, label string
					width      float32
				}{{"name", "文件名 / 协议", 0}, {"size", "大小", 92}, {"progress", "分块进度", 195}, {"speed", "速度", 104}, {"eta", "剩余时间", 90}} {
					label := col.label
					if s.sortBy == col.key {
						if s.sortDesc {
							label += " ↓"
						} else {
							label += " ↑"
						}
					}
					b := ui.ButtonBase(c.Key(col.key)).Height(28).Padding(0, 8).Label("按" + col.label + "排序").Children(func() {
						align := ui.Start
						if col.width > 0 && col.key != "progress" {
							align = ui.End
						}
						ui.Text(c, label).FillWidth().TextAlign(align).FontSize(11).TextColor(t.TextMuted)
					})
					if col.width == 0 {
						b.Grow(1).Basis(0).MinWidth(230)
					} else {
						b.Width(col.width)
					}
					if b.Clicked() {
						if s.sortBy == col.key {
							s.sortDesc = !s.sortDesc
						} else {
							s.sortBy = col.key
							s.sortDesc = false
						}
					}
				}
			})
			for _, i := range s.visibleTasks() {
				s.tableRow(c.Key(i), i)
			}
			if len(s.visibleTasks()) == 0 {
				ui.Text(c, "当前视图没有匹配的下载任务").FontSize(12).TextColor(t.TextMuted).Padding(60, 24)
			}
		})
	})
}

func (s *prototype) tableRow(c *ui.Context, i int) {
	t := c.Theme()
	task := &s.tasks[i]
	row := ui.Row(c).Height(44).Shrink(0).Padding(0, 12).BorderWidth(0, 0, 1, 0).BorderColor(t.Border.Alpha(.6))
	if s.selected == i || s.checked[i] {
		row.Background(t.Accent.Alpha(.07))
	}
	row.Children(func() {
		checked := s.checked[i]
		if ui.Checkbox(c, &checked, "").Width(26).Label("选择 " + task.name).Changed() {
			s.checked[i] = checked
			if checked {
				s.selected = i
				s.detailsOpen = true
			}
		}
		if ui.ButtonBase(c).Grow(1).Basis(0).MinWidth(0).Height(44).Padding(0, 8).Label("查看详情 " + task.name).Gap(8).Children(func() {
			ui.Text(c, task.protocol()).FontSize(9.5).TextColor(t.TextMuted).Border(1, t.Border).Radius(3).Padding(1, 4)
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, task.name).FontSize(12.5).FontWeight(500).MaxLines(1).TruncateMiddle().Tooltip(task.name)
				meta := fmt.Sprintf("%s · %dT", task.source, task.connectionCount())
				color := t.TextMuted
				if task.state == failed {
					meta = task.failure + " · 已保留 " + bytes(task.size*task.progress())
					color = s.errorColor()
				}
				ui.Text(c, meta).FontSize(11).TextColor(color).MaxLines(1).Tooltip(meta)
			})
		}).Clicked() {
			s.choose(i)
		}
		ui.Text(c, bytes(task.size)).Width(92).Padding(0, 8).TextAlign(ui.End).FontSize(11.5).FontFeatures("tnum").TextColor(t.TextMuted)
		ui.Row(c).Width(195).Padding(0, 12).Gap(8).Children(func() {
			if task.state == completed {
				ui.Text(c, "已完成").FontSize(11).TextColor(t.Accent).Grow(1)
			} else {
				s.segmentMap(c, task, 5, true)
			}
			if task.state != completed {
				ui.Textf(c, "%.1f%%", task.progress()*100).Width(40).TextAlign(ui.End).FontSize(11).FontFeatures("tnum").TextColor(t.TextMuted)
			}
		})
		state := string(task.state)
		color := t.TextMuted
		if task.state == downloading {
			state = fmt.Sprintf("%.1f MB/s", task.speed())
			color = t.Text
		}
		if task.state == completed {
			state = "—"
		}
		if task.state == failed {
			color = s.errorColor()
		}
		ui.Text(c, state).Width(104).Padding(0, 8).TextAlign(ui.End).FontSize(11.5).FontFeatures("tnum").TextColor(color)
		eta := "—"
		if task.state == downloading {
			eta = task.remaining()
		}
		ui.Text(c, eta).Width(90).Padding(0, 8).TextAlign(ui.End).FontSize(11.5).TextColor(t.TextMuted)
	})
}

func (s *prototype) segmentMap(c *ui.Context, task *transfer, height float32, micro bool) {
	t := c.Theme()
	ranges := task.ranges()
	step := 1
	if micro && len(ranges) > 16 {
		step = 2
	}
	ui.Row(c).Height(height).Grow(1).Basis(0).Gap(1).Radius(2).Clip().Label("分块进度 " + task.name).Children(func() {
		for n := 0; n < len(ranges); n += step {
			value := ranges[n].fraction
			if step == 2 {
				value = (value + ranges[n+1].fraction) / 2
			}
			color := t.Accent
			if task.state == paused || value >= 1 {
				color = t.TextMuted
			}
			if ranges[n].state == "请求超时" {
				color = s.errorColor()
			}
			ui.Progress(c.Key(n), value).Height(height).Radius(0).Grow(1).Basis(0).TextColor(color).Label(fmt.Sprintf("分块 %02d", n+1))
		}
	})
}

func (s *prototype) statusBar(c *ui.Context) {
	t := c.Theme()
	width, _ := c.Size()
	var speed float64
	for i := range s.tasks {
		speed += s.tasks[i].speed()
	}
	ui.Row(c).Height(28).Shrink(0).Padding(0, 12).Gap(12).Background(t.Background).Children(func() {
		ui.Text(c, "本地模拟 · HTTP/HTTPS").FontSize(11).TextColor(t.TextMuted)
		if s.notice != "" {
			ui.Text(c, s.notice).Grow(1).MinWidth(0).MaxLines(1).FontSize(11).TextColor(t.Accent).Tooltip(s.notice)
		} else {
			ui.Spacer(c)
		}
		if control(c, "演示设置", "", "演示设置").Height(22).Clicked() {
			s.settings = !s.settings
		}
		ui.Textf(c, "↓ %.1f MB/s", speed).FontSize(11).FontFeatures("tnum")
		if width > 900 {
			ui.Textf(c, "%d 项 · %d 个活动中", len(s.tasks), s.count(activeTasks)).FontSize(11).TextColor(t.TextMuted)
		}
	})
}

func (s *prototype) demoSettings(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Shrink(0).Height(44).Padding(0, 12).Gap(12).Background(t.Background).Children(func() {
		if ui.Checkbox(c, &s.simulate, "模拟传输").Changed() {
			s.wasSimulating = false
		}
		if control(c, "重置样本", "", "重置样本").Clicked() {
			s.reset()
		}
		ui.Text(c, "首行状态").FontSize(11).TextColor(t.TextMuted)
		for _, state := range []transferState{downloading, paused, failed, completed} {
			if control(c.Key(state), string(state), "", string(state)).Clicked() {
				s.scenario(state)
			}
		}
	})
}

func glassAction(c *ui.Context, label string) ui.Element {
	return ui.ButtonBase(c).Height(24).Padding(0, 10).Radius(4).Label(label).
		Material(glass.Glass{Interactive: true}).Children(func() { ui.Text(c, label).FontSize(11.5) })
}
