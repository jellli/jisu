// Jisu's isolated native download-row study. No webview or download engine.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

type prototype struct {
	tasks    []transfer
	dark     bool
	simulate bool
	lastTick time.Time
}

func newPrototype() *prototype { return &prototype{tasks: fixtures()} }

func svg(path string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
}

var (
	fileIcon     = svg(`<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8zM14 2v6h6"/>`)
	downloadIcon = svg(`<path d="M12 3v12m-5-5 5 5 5-5M5 17v4h14v-4"/>`)
	pauseIcon    = svg(`<path d="M9 5v14m6-14v14"/>`)
	playIcon     = svg(`<path d="m8 5 11 7-11 7z"/>`)
	checkIcon    = svg(`<path d="m5 12 4 4L19 6"/>`)
	chevronIcon  = svg(`<path d="m9 5 7 7-7 7"/>`)
	resetIcon    = svg(`<path d="M20 10a8 8 0 1 0-1 7M20 5v5h-5"/>`)
	moonIcon     = svg(`<path d="M20.9 13A9 9 0 0 1 11 3.1 9 9 0 1 0 20.9 13Z"/>`)
)

func theme(dark bool) ui.Theme {
	t := *ui.LightTheme()
	if dark {
		t = *ui.DarkTheme()
		t.Background = ui.Hex("#17191a")
		t.Text = ui.Hex("#e9ebea")
		t.TextMuted = ui.Hex("#a4aaa7")
		t.Border = ui.Hex("#353a37")
		t.Accent = ui.Hex("#67bd91")
	} else {
		t.Background = ui.Hex("#f2f4f2")
		t.Text = ui.Hex("#202722")
		t.TextMuted = ui.Hex("#626c65")
		t.Border = ui.Hex("#dbe0dc")
		t.Accent = ui.Hex("#277b52")
	}
	t.FontSize = 14
	t.Radius = 8
	t.Focus = t.Accent
	return t
}

func iconButton(c *ui.Context, label string, icon *ui.SVG) ui.Element {
	return ui.ButtonBase(c).Size(36, 36).Radius(18).Label(label).Tooltip(label).
		Material(glass.Glass{Interactive: true}).Center().Children(func() {
		ui.Icon(c, icon).FontSize(16)
	})
}

func (s *prototype) view(c *ui.Context) {
	t := theme(s.dark)
	c.SetTheme(&t)
	if s.simulate {
		now := c.Now()
		if !s.lastTick.IsZero() {
			for i := range s.tasks {
				s.tasks[i].advance(now.Sub(s.lastTick))
			}
		}
		s.lastTick = now
		c.After(100 * time.Millisecond)
	} else {
		s.lastTick = time.Time{}
	}
	w, _ := c.Size()
	padding := float32(40)
	if w < 800 {
		padding = 24
	}
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(64).Padding(0, padding).Gap(12).Children(func() {
			ui.Icon(c, downloadIcon).FontSize(20)
			ui.Text(c, "极速").FontSize(18).FontWeight(600)
			ui.Text(c, "Jisu").FontSize(14).TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Text(c, "下载行原型 · 仅模拟数据").FontSize(12).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		ui.Row(c).Padding(24, padding, 20, padding).AlignItems(ui.End).Children(func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "全部下载").FontSize(28).FontWeight(500).Role(ui.RoleHeading).Level(1)
				ui.Text(c, "3 个任务 · 8 路并行连接").FontSize(13).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.Column(c).AlignItems(ui.End).Gap(4).Children(func() {
				var total float64
				for i := range s.tasks {
					total += s.tasks[i].speed()
				}
				ui.Row(c).Gap(6).AlignItems(ui.Baseline).Children(func() {
					ui.Textf(c, "%.1f", total).FontSize(24).FontWeight(500).FontFeatures("tnum")
					ui.Text(c, "MB/s").FontSize(12).TextColor(t.TextMuted)
				})
				ui.Text(c, "总下载速度").FontSize(12).TextColor(t.TextMuted)
			})
		})
		// One glass toolbar, not glass on every row: the data keeps contrast.
		ui.Row(c).Margin(0, padding, 12, padding).Padding(6, 8, 6, 16).Radius(24).
			Material(glass.Glass{}).Gap(8).Children(func() {
			ui.Text(c, "任务").FontSize(13).FontWeight(500)
			ui.Text(c, "3").FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Checkbox(c, &s.simulate, "模拟传输").Changed() {
				s.lastTick = time.Time{}
			}
			if iconButton(c.Key("reset"), "重置模拟数据", resetIcon).Clicked() {
				s.tasks = fixtures()
				s.lastTick = time.Time{}
			}
			if iconButton(c.Key("appearance"), "切换深浅外观", moonIcon).Clicked() {
				s.dark = !s.dark
			}
		})
		ui.Scroll(c).Grow(1).Padding(0, padding, 24, padding).Children(func() {
			ui.Column(c).Radius(12).Background(t.Surface).Border(1, t.Border).Dividers(1, t.Border).Children(func() {
				for i := range s.tasks {
					s.row(c.Key(i), &s.tasks[i])
				}
			})
			ui.Row(c).Padding(16, 0).Children(func() {
				ui.Text(c, "保存到 ~/Downloads").FontSize(12).TextColor(t.TextMuted)
				ui.Spacer(c)
				ui.Text(c, "模拟数据 · 不会下载文件").FontSize(12).TextColor(t.TextMuted)
			})
		})
	})
}

func (s *prototype) row(c *ui.Context, task *transfer) {
	t := c.Theme()
	ui.Column(c).Padding(20, 24).Gap(12).Label(task.name).Children(func() {
		ui.Row(c).Gap(12).Children(func() {
			ui.Icon(c, fileIcon).FontSize(24).TextColor(t.TextMuted)
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, task.name).FontSize(15).FontWeight(500).SingleLine().Tooltip(task.name)
				ui.Textf(c, "%s · %s · %s", task.source, bytes(task.size), task.ext).FontSize(12).TextColor(t.TextMuted).SingleLine()
			})
			ui.Column(c).AlignItems(ui.End).Gap(4).Children(func() {
				color := t.TextMuted
				if task.state == downloading {
					color = t.Accent
				}
				ui.Text(c, string(task.state)).FontSize(12).TextColor(color)
				ui.Textf(c, "%d%%", int(math.Floor(task.progress()*100))).FontSize(16).FontWeight(500).FontFeatures("tnum")
			})
			if task.state == completed {
				ui.Box(c).Size(36, 36).Center().Children(func() { ui.Icon(c, checkIcon).FontSize(18).TextColor(t.TextMuted) })
			} else {
				label, icon := "暂停 "+task.name, pauseIcon
				if task.state == paused {
					label, icon = "继续 "+task.name, playIcon
				}
				if iconButton(c.Key("toggle"), label, icon).Clicked() {
					task.toggle()
				}
			}
		})
		// All eight equal byte ranges stay visible. A quiet completed lane
		// and colored in-flight lanes distinguish work left from work received.
		ui.Row(c).Gap(8).Children(func() {
			for n, value := range task.segments {
				ui.Column(c.Key(n)).Grow(1).Basis(0).MinWidth(0).Gap(5).Children(func() {
					ui.Textf(c, "%02d", n+1).FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
					color := t.TextMuted
					if task.state == downloading && value < 1 {
						color = t.Accent
					}
					ui.Progress(c, value).Height(5).Radius(2).TextColor(color).
						Label(fmt.Sprintf("%s 连接 %d", task.name, n+1))
				})
			}
		})
		ui.Row(c).Gap(12).Children(func() {
			ui.Textf(c, "%s / %s", bytes(task.size*task.progress()), bytes(task.size)).FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
			ui.Text(c, task.remaining()).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			if task.state == downloading {
				ui.Textf(c, "%.1f MB/s", task.speed()).FontSize(13).FontFeatures("tnum").TextColor(t.Accent)
			}
			label := "展开连接详情 " + task.name
			if task.expanded {
				label = "收起连接详情 " + task.name
			}
			if ui.ButtonBase(c.Key("details")).Gap(4).Padding(4).Radius(4).Label(label).Expanded(task.expanded).Children(func() {
				ui.Text(c, "连接详情").FontSize(12).TextColor(t.TextMuted)
				icon := ui.Icon(c, chevronIcon).FontSize(12).TextColor(t.TextMuted)
				if task.expanded {
					icon.Rotate(90)
				}
			}).Clicked() {
				task.expanded = !task.expanded
			}
		})
		if task.expanded {
			ui.Divider(c)
			ui.Column(c).Gap(8).Children(func() {
				for n, value := range task.segments {
					ui.Row(c.Key(n)).Gap(16).Children(func() {
						ui.Textf(c, "连接 %02d", n+1).Width(72).FontSize(12).TextColor(t.TextMuted)
						ui.Progress(c, value).Grow(1).Height(3).TextColor(t.TextMuted).Label(fmt.Sprintf("%s 连接 %d 详情", task.name, n+1))
						ui.Textf(c, "%d%%", int(math.Floor(value*100))).Width(44).TextAlign(ui.End).FontSize(12).FontFeatures("tnum")
						ui.Textf(c, "%.1f MB/s", task.rate(n)).Width(76).TextAlign(ui.End).FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
					})
				}
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
			Title: "Jisu · Native Glass", Width: 1040, Height: 800,
			MinWidth: 720, MinHeight: 560, Content: ui.View(s.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
