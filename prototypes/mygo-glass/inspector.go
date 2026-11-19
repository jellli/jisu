package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
)

var tabNames = []string{"分块与线程", "文件与轨道", "速率曲线", "校验与自动化", "连接日志"}

func (s *prototype) inspector(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Surface).Label("底部任务详情").Children(func() {
		if s.checkedCount() > 1 {
			s.batchInspector(c)
			return
		}
		ui.Row(c).Height(34).Shrink(0).Padding(0, 12).Gap(2).Background(t.Background).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			for i, name := range tabNames {
				b := ui.RadioBase(c.Key(i), &s.tab, i).Height(34).Padding(0, 10).Label(name).Children(func() { ui.Text(c, name).FontSize(12) })
				if s.tab == i {
					b.BorderWidth(0, 0, 2, 0).BorderColor(t.Accent)
				}
			}
			ui.Spacer(c)
			if s.selected >= 0 {
				ui.Text(c, s.tasks[s.selected].name).Grow(1).Basis(0).MinWidth(0).MaxLines(1).TruncateMiddle().TextAlign(ui.End).FontSize(11).TextColor(t.TextMuted)
			}
		})
		if s.selected < 0 || s.selected >= len(s.tasks) {
			ui.Text(c, "请在上方列表选择一个任务").FontSize(12).TextColor(t.TextMuted).Padding(32, 16)
			return
		}
		task := &s.tasks[s.selected]
		switch s.tab {
		case 0:
			s.chunks(c, task)
		case 1:
			s.files(c, task)
		case 2:
			s.speedChart(c, task)
		case 3:
			s.verify(c)
		case 4:
			ui.Scroll(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
				ui.Column(c).FillWidth().Padding(10, 12).Gap(5).Children(func() {
					ui.Text(c, "SIM  HTTP/HTTPS 模拟任务 · "+task.source).FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, fmt.Sprintf("SIM  原始 8 个字节范围 · 已保留 %s", bytes(task.size*task.progress()))).FontSize(11).TextColor(t.TextMuted)
					for _, line := range task.events {
						ui.Text(c, line).FontSize(11).MaxLines(2).TextColor(t.TextMuted)
					}
					if task.state == failed {
						ui.Text(c, task.failure+"；等待重试，已收字节未清零").FontSize(11).TextColor(s.errorColor())
					}
				})
			})
		}
	})
}

func (s *prototype) actionControl(c *ui.Context, task *transfer) {
	label := "暂停"
	switch task.state {
	case paused:
		label = "继续"
	case failed:
		label = "重试"
	case completed:
		label = "在文件夹中显示"
	}
	if glassAction(c, label).Label(label + " " + task.name).Clicked() {
		s.action()
	}
}

func (s *prototype) chunks(c *ui.Context, task *transfer) {
	t := c.Theme()
	if task.state == completed {
		s.files(c, task)
		return
	}
	ranges := task.ranges()
	done := 0
	for _, r := range ranges {
		if r.state == "已完成" {
			done++
		}
	}
	ui.Column(c).Height(62).Shrink(0).Padding(8, 12).Gap(8).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).Gap(10).Children(func() {
			ui.Textf(c, "分块进度 (%d/%d 完成)", done, len(ranges)).FontSize(11).TextColor(t.TextMuted)
			ui.Textf(c, "%s / %s", bytes(task.size*task.progress()), bytes(task.size)).FontSize(11).FontFeatures("tnum")
			ui.Spacer(c)
			s.actionControl(c, task)
			ui.Text(c, "线程数").FontSize(11).TextColor(t.TextMuted)
			ui.Row(c).Gap(0).Children(func() {
				for _, n := range []int{4, 8, 16, 32} {
					b := control(c.Key(n), fmt.Sprintf("%d 路布局预览，真实并发待接入引擎", n), "", fmt.Sprint(n)).Height(22).Padding(0, 7)
					if task.connectionCount() == n {
						b.Background(t.Accent).TextColor(t.Surface)
					}
					if b.Clicked() {
						task.previewCount = n
						s.notice = fmt.Sprintf("%d 路布局预览；原始 8 路模拟数据不变，真实并发未接入", n)
					}
				}
			})
		})
		s.segmentMap(c, task, 10, false)
	})
	ui.ScrollBoth(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
		ui.Column(c).FillWidth().MinWidth(790).Children(func() {
			ui.Row(c).Height(25).Padding(0, 12).Gap(12).Background(t.Background).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, "线程 ID").Width(48).FontSize(10.5).TextColor(t.TextMuted)
				ui.Text(c, "字节范围 (Byte Range)").Grow(1).Basis(0).MinWidth(180).FontSize(10.5).TextColor(t.TextMuted)
				ui.Text(c, "已下载").Width(174).TextAlign(ui.End).FontSize(10.5).TextColor(t.TextMuted)
				ui.Text(c, "瞬时速率").Width(90).TextAlign(ui.End).FontSize(10.5).TextColor(t.TextMuted)
				ui.Text(c, "状态").Width(90).TextAlign(ui.End).FontSize(10.5).TextColor(t.TextMuted)
			})
			for n, r := range ranges {
				ui.Row(c.Key(n)).Height(26).Padding(0, 12).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(t.Border.Alpha(.35)).Children(func() {
					color := t.TextMuted
					if r.state == "请求超时" {
						color = s.errorColor()
					}
					ui.Textf(c, "#%02d", n+1).Width(48).FontSize(11).FontFeatures("tnum").TextColor(color)
					ui.Text(c, grouped(r.start)+" – "+grouped(r.end)).Grow(1).Basis(0).MinWidth(180).FontSize(11).FontFeatures("tnum").TextColor(t.TextMuted)
					ui.Textf(c, "%s / %s", bytes(r.received/1e6), bytes(float64(r.end-r.start+1)/1e6)).Width(174).TextAlign(ui.End).FontSize(11).FontFeatures("tnum")
					rate := "—"
					if r.rate > 0 {
						rate = fmt.Sprintf("%.2f MB/s", r.rate)
					}
					ui.Text(c, rate).Width(90).TextAlign(ui.End).FontSize(11).FontFeatures("tnum").TextColor(t.TextMuted)
					ui.Text(c, r.state).Width(90).TextAlign(ui.End).FontSize(11).TextColor(color)
				})
			}
		})
	})
}

func (s *prototype) files(c *ui.Context, task *transfer) {
	t := c.Theme()
	ui.ScrollBoth(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
		ui.Column(c).FillWidth().MinWidth(650).Gap(10).Padding(10, 12).Children(func() {
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, "包含 1 个 HTTP/HTTPS 文件").FontSize(11).TextColor(t.TextMuted)
				ui.Spacer(c)
				s.actionControl(c, task)
			})
			ui.Row(c).Height(28).Gap(12).Children(func() {
				ui.Icon(c, icons["document"]).FontSize(14).TextColor(t.TextMuted)
				ui.Text(c, task.name).Grow(1).MinWidth(0).FontSize(12).MaxLines(1).TruncateMiddle()
				ui.Text(c, bytes(task.size)).FontSize(11)
				ui.Text(c, string(task.state)).Width(80).TextAlign(ui.End).FontSize(11).TextColor(t.TextMuted)
			})
			ui.Divider(c)
			ui.Text(c, "来源："+task.source).FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, "保存路径：~/Downloads/"+task.name).FontSize(11).TextColor(t.TextMuted)
			ui.Textf(c, "文件大小：%s bytes · 已收 %s", grouped(int64(math.Round(task.size*1e6))), bytes(task.size*task.progress())).FontSize(11).TextColor(t.TextMuted)
		})
	})
}

func (s *prototype) speedChart(c *ui.Context, task *transfer) {
	t := c.Theme()
	ui.Column(c).Grow(1).Basis(0).MinHeight(0).Padding(10, 12).Gap(8).Children(func() {
		ui.Row(c).Gap(20).Children(func() {
			ui.Textf(c, "网络下载速率  %.1f MB/s（模拟）", task.speed()).FontSize(11).TextColor(t.Accent)
			ui.Text(c, "磁盘写入 I/O  —").FontSize(11).TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Text(c, "最近 60 个模拟采样").FontSize(11).TextColor(t.TextMuted)
		})
		ui.Column(c).Grow(1).Basis(0).MinHeight(0).Background(t.Background).Border(1, t.Border).Radius(4).Draw(func(p *ui.Painter, r ui.Rect) {
			for n := 1; n < 5; n++ {
				y := r.Y + r.H*float32(n)/5
				p.Line(r.X, y, r.X+r.W, y, 1, t.Border.Alpha(.5))
			}
			hi := float64(1)
			for _, v := range task.history {
				hi = math.Max(hi, v)
			}
			for n := 1; n < len(task.history); n++ {
				x0 := r.X + r.W*float32(n-1)/59
				x1 := r.X + r.W*float32(n)/59
				y0 := r.Y + r.H - 8 - (r.H-16)*float32(task.history[n-1]/hi)
				y1 := r.Y + r.H - 8 - (r.H-16)*float32(task.history[n]/hi)
				p.Line(x0, y0, x1, y1, 1.5, t.Accent)
			}
		}).Center().Children(func() {
			if len(task.history) < 2 {
				ui.Text(c, "开启模拟传输后记录速率").FontSize(12).TextColor(t.TextMuted)
			}
		})
	})
}

func (s *prototype) verify(c *ui.Context) {
	t := c.Theme()
	hash := ""
	off := false
	ui.Scroll(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
		ui.Row(c).FillWidth().Padding(14).Gap(24).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
				ui.Text(c, "SHA-256 完整性校验").FontSize(12).FontWeight(500)
				ui.Text(c, "无本地下载文件，未计算指纹").FontSize(11).TextColor(t.TextMuted)
				ui.TextInput(c, &hash).Placeholder("官方 SHA-256 哈希值").Disabled(true).Label("SHA-256，未接入本地文件")
				ui.Button(c, "立即校验").Disabled(true).Tooltip("接入真实下载文件后启用")
			})
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
				ui.Text(c, "下载完成后自动化流水线").FontSize(12).FontWeight(500)
				for _, label := range []string{"完成后自动解压", "按文件类型自动归档", "完成后发送系统通知", "完成后执行 Shell 脚本"} {
					ui.Checkbox(c, &off, label).Disabled(true)
				}
			})
		})
	})
}

func (s *prototype) batchInspector(c *ui.Context) {
	t := c.Theme()
	total := float64(0)
	for i, on := range s.checked {
		if on {
			total += s.tasks[i].size
		}
	}
	ui.Column(c).Fill().Padding(16).Gap(16).Background(t.Background).Children(func() {
		ui.Textf(c, "已选择 %d 个任务", s.checkedCount()).FontSize(14).FontWeight(500)
		ui.Text(c, "总大小 "+bytes(total)).FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Gap(8).Children(func() {
			if control(c, "开始选中任务", "play", "开始选中").Clicked() {
				s.setAll(downloading, true)
			}
			if control(c, "暂停选中任务", "pause", "暂停选中").Clicked() {
				s.setAll(paused, true)
			}
			if control(c, "取消多选", "", "取消多选").Clicked() {
				s.choose(s.selected)
			}
		})
	})
}
