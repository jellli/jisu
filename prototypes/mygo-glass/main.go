// Jisu's isolated native download-row study. No webview or download engine.
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

type prototype struct {
	tasks                    []transfer
	filter                   queueFilter
	dark, simulate, settings bool
	notice                   string
	lastTick                 time.Time
	wasSimulating            bool
	selected                 int
	detailsOpen              bool
}

func newPrototype() *prototype { return &prototype{tasks: fixtures(), detailsOpen: true} }

func (s *prototype) reset() {
	s.tasks, s.filter, s.notice = fixtures(), allTasks, ""
	s.lastTick, s.wasSimulating = time.Time{}, false
	s.selected, s.detailsOpen = 0, true
}

// Selection follows the visible queue; never inspect a task hidden by a filter.
func (s *prototype) reconcileSelection() {
	if s.selected >= 0 && s.selected < len(s.tasks) && s.filter.matches(&s.tasks[s.selected]) {
		return
	}
	s.selected = -1
	for i := range s.tasks {
		if s.filter.matches(&s.tasks[i]) {
			s.selected = i
			return
		}
	}
}

func (s *prototype) scenario(state transferState) {
	s.tasks[0] = fixtures()[0]
	s.tasks[0].state = state
	if state == failed {
		s.tasks[0].failure = "连接 03 请求超时"
	}
	if state == completed {
		s.tasks[0].segments = [8]float64{1, 1, 1, 1, 1, 1, 1, 1}
	}
	s.lastTick, s.wasSimulating, s.notice = time.Time{}, false, ""
}

func (s *prototype) tick(now time.Time) {
	if s.simulate && s.wasSimulating && !s.lastTick.IsZero() {
		for i := range s.tasks {
			s.tasks[i].advance(now.Sub(s.lastTick))
		}
	}
	s.lastTick, s.wasSimulating = now, s.simulate
}

func (s *prototype) count(f queueFilter) int {
	count := 0
	for i := range s.tasks {
		if f.matches(&s.tasks[i]) {
			count++
		}
	}
	return count
}

var chevronIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="m9 5 7 7-7 7"/></svg>`))

func theme(dark bool) ui.Theme {
	t := *ui.LightTheme()
	if dark {
		t = *ui.DarkTheme()
		t.Background = ui.Hex("#161917")
		t.Surface = ui.Hex("#1d211e")
		t.Text = ui.Hex("#e6eae7")
		t.TextMuted = ui.Hex("#a9b3ac")
		t.Border = ui.Hex("#39423c")
		t.Accent = ui.Hex("#7cc69d")
	} else {
		t.Background = ui.Hex("#f3f4f2")
		t.Surface = ui.Hex("#fcfdfb")
		t.Text = ui.Hex("#202722")
		t.TextMuted = ui.Hex("#58645c")
		t.Border = ui.Hex("#d9ded9")
		t.Accent = ui.Hex("#226b46")
	}
	t.FontSize, t.Radius, t.Focus = 14, 6, t.Accent
	return t
}

func (s *prototype) errorColor() ui.Color {
	if s.dark {
		return ui.Hex("#f1a39b")
	}
	return ui.Hex("#a13028")
}

// Glass belongs to small, useful controls, not a second toolbar around data.
func glassAction(c *ui.Context, text, label string) ui.Element {
	e := ui.ButtonBase(c).Height(32).Padding(0, 12).Radius(6).Label(label).
		Material(glass.Glass{Interactive: true}).Center().Children(func() {
		ui.Text(c, text).FontSize(13)
	})
	if e.Pressed() {
		e.Scale(.97, .97)
	}
	return e
}

func (s *prototype) view(c *ui.Context) {
	t := theme(s.dark)
	c.SetTheme(&t)
	s.tick(c.Now())
	if s.simulate {
		c.After(250 * time.Millisecond)
	}
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Height(64).Shrink(0).Padding(0, 24).Gap(12).Children(func() {
			ui.Text(c, "下载").FontSize(20).FontWeight(500).Role(ui.RoleHeading).Level(1)
			ui.Spacer(c)
			var speed float64
			for i := range s.tasks {
				speed += s.tasks[i].speed()
			}
			if speed > 0 {
				ui.Textf(c, "%.1f MB/s", speed).FontSize(13).FontFeatures("tnum").TextColor(t.TextMuted).Label("总下载速度")
			}
		})
		ui.Row(c).Height(44).Shrink(0).Padding(0, 16).Gap(4).Label("任务筛选").Children(func() {
			for i, name := range filterNames {
				f := queueFilter(i)
				color := t.TextMuted
				if f == attentionTasks && s.count(f) > 0 {
					color = s.errorColor()
				}
				b := ui.RadioBase(c.Key(i), &s.filter, f).Label(name).Padding(7, 10).Radius(5).Gap(6).Children(func() {
					ui.Text(c, name).FontSize(13).TextColor(color)
					ui.Textf(c, "%d", s.count(f)).FontSize(12).FontFeatures("tnum").TextColor(color)
				})
				if s.filter == f {
					b.Background(t.Border.Alpha(.45))
				}
			}
		})
		ui.Divider(c)
		s.reconcileSelection()
		ui.Scroll(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Background(t.Surface).Dividers(1, t.Border).Children(func() {
				for i := range s.tasks {
					if s.filter.matches(&s.tasks[i]) {
						s.row(c.Key(i), i, &s.tasks[i])
					}
				}
			})
			if s.count(s.filter) == 0 {
				ui.Column(c).Padding(48, 24).Gap(10).Children(func() {
					message := "此分类暂无任务"
					if s.filter == attentionTasks {
						message = "暂无需处理的任务"
					}
					ui.Text(c, message).FontSize(15)
					ui.Text(c, "切换到全部查看下载队列").FontSize(13).TextColor(t.TextMuted)
				})
			}
		})
		if s.detailsOpen && s.selected >= 0 {
			s.inspector(c)
		}
		if s.notice != "" {
			ui.Text(c, s.notice).FontSize(12).TextColor(t.TextMuted).Padding(10, 24).MaxLines(2)
		}
		ui.Divider(c)
		ui.Row(c).Height(44).Shrink(0).Padding(0, 24).Gap(12).Children(func() {
			ui.Text(c, "模拟数据 · 不下载或打开真实文件").FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			label := "演示设置"
			if s.settings {
				label = "收起设置"
			}
			if ui.ButtonBase(c).Height(30).Padding(0, 8).Radius(4).Label(label).Expanded(s.settings).Children(func() {
				ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
			}).Clicked() {
				s.settings = !s.settings
			}
		})
		if s.settings {
			s.demoSettings(c)
		}
	})
}

func (s *prototype) demoSettings(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Shrink(0).Padding(4, 24, 16, 24).Gap(10).Children(func() {
		ui.Row(c).Gap(16).Children(func() {
			if ui.Checkbox(c, &s.simulate, "模拟传输").Changed() {
				s.wasSimulating = false
			}
			ui.Checkbox(c, &s.dark, "深色外观")
			ui.Spacer(c)
			if ui.Button(c, "重置样本").FontSize(12).Clicked() {
				s.reset()
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, "首行状态").FontSize(12).TextColor(t.TextMuted)
			for _, state := range []transferState{downloading, paused, failed, completed} {
				if ui.Button(c.Key(state), string(state)).FontSize(12).Clicked() {
					s.scenario(state)
				}
			}
		})
	})
}

func (s *prototype) row(c *ui.Context, index int, task *transfer) {
	t := c.Theme()
	selected := s.detailsOpen && s.selected == index
	row := ui.Row(c).Height(64).Shrink(0).Padding(8, 24).Gap(12)
	if selected {
		row.Background(t.Border.Alpha(.25))
	}
	row.Children(func() {
		if ui.ButtonBase(c).Grow(1).Basis(0).MinWidth(0).Height(48).Radius(3).
			Label("查看详情 " + task.name).Expanded(selected).Children(func() {
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Row(c).Gap(12).Children(func() {
					ui.Text(c, task.name).FontSize(14).FontWeight(500).Grow(1).MinWidth(0).
						MaxLines(1).TruncateMiddle().Tooltip(task.name)
					if task.state != completed {
						ui.Textf(c, "%.0f%%", task.progress()*100).Width(40).TextAlign(ui.End).FontSize(13).FontFeatures("tnum")
					}
					if task.state == downloading {
						ui.Text(c, task.remaining()).FontSize(12).TextColor(t.TextMuted).Label("预计剩余 " + task.remaining())
					}
				})
				ui.Row(c).Height(18).Gap(16).Children(func() {
					if task.state == downloading || task.state == paused {
						color := t.Accent
						status := fmt.Sprintf("%.1f MB/s", task.speed())
						if task.state == paused {
							color, status = t.TextMuted, "已暂停"
						}
						ui.Progress(c, task.progress()).Height(3).Grow(1).Basis(0).TextColor(color).Label(task.name + " 总进度")
						ui.Text(c, status).Width(88).TextAlign(ui.End).FontSize(12).FontFeatures("tnum").TextColor(color)
						return
					}
					meta := fmt.Sprintf("已完成 · %s · ~/Downloads", bytes(task.size))
					color := t.TextMuted
					if task.state == failed {
						meta = fmt.Sprintf("%s · 已保留 %s", task.failure, bytes(task.size*task.progress()))
						color = s.errorColor()
					}
					ui.Text(c, meta).FontSize(12).TextColor(color).Grow(1).MinWidth(0).MaxLines(1).Tooltip(meta).FontFeatures("tnum")
				})
			})
		}).Clicked() {
			s.selected, s.detailsOpen = index, true
		}
		ui.Row(c).Width(140).Justify(ui.End).Children(func() {
			label := "暂停"
			switch task.state {
			case paused:
				label = "继续"
			case failed:
				label = "重试"
			case completed:
				label = "在文件夹中显示"
			}
			if glassAction(c, label, label+" "+task.name).Clicked() {
				s.selected, s.detailsOpen = index, true
				if task.state == completed {
					s.notice = "模拟文件位置：~/Downloads/" + task.name
				} else {
					wasFailed := task.state == failed
					task.toggle()
					if wasFailed {
						s.notice = "已从保留的分段继续：" + task.name
					}
				}
			}
		})
		label := "展开详情 " + task.name
		if selected {
			label = "收起详情 " + task.name
		}
		if ui.ButtonBase(c).Size(28, 32).Radius(4).Center().Label(label).Tooltip(label).Expanded(selected).Children(func() {
			icon := ui.Icon(c, chevronIcon).FontSize(14).TextColor(t.TextMuted)
			if selected {
				icon.Rotate(90)
			}
		}).Clicked() {
			s.selected, s.detailsOpen = index, !selected
		}
	})
}

func (s *prototype) inspector(c *ui.Context) {
	t := c.Theme()
	task := &s.tasks[s.selected]
	_, height := c.Size()
	paneHeight := float32(340)
	if height < 700 {
		paneHeight = 280
	}
	if height < 600 || s.settings {
		paneHeight = 180
	}
	ui.Column(c).Height(paneHeight).Shrink(0).Background(t.Surface).Label("所选任务详情").Children(func() {
		ui.Divider(c)
		ui.Row(c).Height(40).Shrink(0).Padding(0, 24).Gap(16).Children(func() {
			ui.Text(c, "任务详情").FontSize(13).FontWeight(500)
			ui.Text(c, task.name).Grow(1).MinWidth(0).MaxLines(1).TruncateMiddle().FontSize(12).TextColor(t.TextMuted)
			if ui.ButtonBase(c).Height(28).Padding(0, 6).Radius(4).Label("收起底部详情").Children(func() {
				ui.Text(c, "收起").FontSize(12).TextColor(t.TextMuted)
			}).Clicked() {
				s.detailsOpen = false
			}
		})
		ui.Scroll(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Padding(4, 24, 12, 24).Children(func() {
				s.details(c.Key(s.selected), task)
			})
		})
	})
}

func (s *prototype) details(c *ui.Context, task *transfer) {
	t := c.Theme()
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, task.source+" · 保存到 ~/Downloads").FontSize(12).TextColor(t.TextMuted)
		if task.state == completed {
			ui.Text(c, "~/Downloads/"+task.name).FontSize(12).TextColor(t.TextMuted).MaxLines(2)
			return
		}
		ui.Row(c).Gap(12).Children(func() {
			ui.Textf(c, "已收 %s / %s", bytes(task.size*task.progress()), bytes(task.size)).FontSize(12).FontFeatures("tnum")
			ui.Spacer(c)
			ui.Text(c, "8 路连接").FontSize(12).TextColor(t.TextMuted)
		})
		// One byte-range track in the inspector, not eight independent row cards.
		ui.Row(c).Height(8).Radius(2).Clip().Label("八路分段进度").Children(func() {
			for n, value := range task.segments {
				color := t.TextMuted
				if task.state == downloading && value < 1 {
					color = t.Accent
				} else if task.state == failed && n == 2 {
					color = s.errorColor()
				}
				ui.Progress(c.Key(n), value).Height(8).Radius(0).Grow(1).Basis(0).
					TextColor(color).Label(fmt.Sprintf("%s 连接 %02d", task.name, n+1))
			}
		})
		ui.Row(c).Gap(16).Children(func() {
			ui.Text(c, "连接").Width(64).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "已收 / 总量").Grow(1).TextAlign(ui.End).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "速率").Width(104).TextAlign(ui.End).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "状态").Width(88).TextAlign(ui.End).FontSize(12).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		for n, value := range task.segments {
			ui.Row(c.Key(n)).Gap(16).Children(func() {
				color := t.TextMuted
				if task.state == failed && n == 2 {
					color = s.errorColor()
				}
				ui.Textf(c, "%02d", n+1).Width(64).FontSize(12).FontFeatures("tnum").TextColor(color)
				ui.Textf(c, "%s / %s", bytes(task.size/8*value), bytes(task.size/8)).Grow(1).
					TextAlign(ui.End).FontSize(12).FontFeatures("tnum")
				rate := "—"
				if task.rate(n) > 0 {
					rate = fmt.Sprintf("%.1f MB/s", task.rate(n))
				}
				ui.Text(c, rate).Width(104).TextAlign(ui.End).FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
				ui.Text(c, task.connectionState(n)).Width(88).TextAlign(ui.End).FontSize(12).TextColor(color)
			})
		}
	})
}

func main() {
	dark := flag.Bool("dark", false, "Start in dark appearance")
	flag.Parse()
	s := newPrototype()
	s.dark = *dark
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title: "Jisu · Native Glass", Width: 1040, Height: 760,
			MinWidth: 720, MinHeight: 500, Content: ui.View(s.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
