package main

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"
)

func text(c *ui.Context, value string, size float32, secondary bool) ui.Element {
	p := display.TextProps{FontSize: size, Rows: 1}
	if secondary {
		p.Type = display.TextSecondary
	}
	return display.Text(c, value, p)
}

func (d *demo) view(c *ui.Context) {
	theme.Apply(c)
	tk := theme.TokensOf(c)
	d.tick(c.Now())
	if d.Simulate {
		c.After(time.Second)
	}
	ui.Column(c).Fill().Background(tk.BgContainer).Children(func() {
		ui.Row(c).Shrink(0).Padding(24, 28).Gap(24).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, lucide.Download()).FontSize(24).TextColor(tk.Text)
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				text(c, "极速下载", 20, false).FontWeight(500)
				text(c, "Lobe MyGo 原生组件 · 本地模拟，不会下载文件", 12, true)
			})
			ui.Column(c).AlignItems(ui.End).Gap(4).Children(func() {
				text(c, fmt.Sprintf("%.1f MB/s", d.totalSpeed()), 22, false).FontFeatures("tnum")
				text(c, "总下载速度", 12, true)
			})
		})
		ui.Row(c).Shrink(0).Padding(0, 28, 16, 28).Gap(16).AlignItems(ui.Center).Children(func() {
			action.Segmented(c, &d.Filter, []string{
				fmt.Sprintf("全部任务 %d", len(d.Tasks)),
				fmt.Sprintf("下载中 %d", d.count(downloading)),
				fmt.Sprintf("已暂停 %d", d.count(paused)),
				fmt.Sprintf("已完成 %d", d.count(completed)),
			}, action.SegmentedProps{Size: theme.Small}).Label("任务筛选")
			ui.Box(c).Grow(1)
			text(c, "模拟传输", 12, true)
			choice.Switch(c, &d.Simulate, choice.SwitchProps{Size: theme.Small}).Label("模拟传输")
		})
		layout.Divider(c, layout.DividerProps{})
		ui.Scroll(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().PaddingX(28).Children(func() {
				shown := 0
				for i := range d.Tasks {
					if !d.visible(&d.Tasks[i]) {
						continue
					}
					if shown > 0 {
						layout.Divider(c, layout.DividerProps{})
					}
					d.downloadRow(c.Key(d.Tasks[i].Name), i)
					shown++
				}
				if shown == 0 {
					text(c, "此分类暂无任务", 14, true).PaddingY(40)
				}
			})
		})
		layout.Divider(c, layout.DividerProps{})
		ui.Row(c).Shrink(0).Padding(12, 28).Gap(12).AlignItems(ui.Center).Children(func() {
			text(c, "首行状态", 12, true)
			before := d.Scenario
			field.Select(c, &d.Scenario, field.SelectProps{Size: theme.Small, Options: []field.SelectOption{
				{Value: string(downloading), Label: stateNames[downloading]},
				{Value: string(paused), Label: stateNames[paused]},
				{Value: string(failed), Label: stateNames[failed]},
				{Value: string(completed), Label: stateNames[completed]},
			}}).Width(120).Label("首行状态")
			if d.Scenario != before {
				d.setScenario(d.Scenario)
				c.Invalidate()
			}
			if action.Button(c, "重置样本", action.ButtonProps{Size: theme.Small}).Clicked() {
				d.reset()
				c.Invalidate()
			}
			ui.Box(c).Grow(1)
			text(c, "8 路连接 · HTTP Range · 断点保留", 12, true)
		})
	})
}

func (d *demo) downloadRow(c *ui.Context, index int) {
	t := &d.Tasks[index]
	tk := theme.TokensOf(c)
	ui.Column(c).FillWidth().PaddingY(20).Gap(12).Children(func() {
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, lucide.File()).FontSize(20).TextColor(tk.TextSecondary).Shrink(0)
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
				text(c, t.Name, 14, false).FontWeight(500)
				text(c, t.Source+" · "+bytes(t.SizeMB), 12, true)
			})
			color := display.TagGray
			if t.State == downloading {
				color = display.TagProcessing
			} else if t.State == failed {
				color = display.TagError
			}
			display.Tag(c, stateNames[t.State], display.TagProps{Color: color, Variant: display.TagBorderless, Size: theme.Small})
			label := map[transferState]string{downloading: "暂停", paused: "继续", failed: "重试", completed: "查看详情"}[t.State]
			if action.Button(c, label, action.ButtonProps{Size: theme.Middle}).Label(label + " " + t.Name).Clicked() {
				t.activate()
				if index == 0 {
					d.Scenario = string(t.State)
				}
				c.Invalidate()
			}
		})
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			for n, value := range t.Segments {
				ui.Column(c.Key(n)).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
					ui.Row(c).FillWidth().Justify(ui.SpaceBetween).Children(func() {
						text(c, fmt.Sprintf("%d", n+1), 12, true)
						text(c, fmt.Sprintf("%.0f%%", math.Floor(value*100)), 12, true).FontFeatures("tnum")
					})
					color := tk.TextQuaternary
					if t.State == downloading {
						color = ui.RGB(8, 113, 94)
						if tk.Dark {
							color = ui.RGB(93, 192, 163)
						}
					}
					feedback.Progress(c, float32(value*100), feedback.ProgressProps{Size: theme.Large, Color: color}).Label(fmt.Sprintf("%s 连接 %d", t.Name, n+1))
				})
			}
		})
		ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Center).Children(func() {
			text(c, fmt.Sprintf("%.0f%%", math.Floor(t.progress()*100)), 13, false).FontFeatures("tnum")
			text(c, bytes(t.SizeMB*t.progress())+" / "+bytes(t.SizeMB), 12, true).FontFeatures("tnum")
			ui.Box(c).Grow(1)
			text(c, fmt.Sprintf("%.1f MB/s", t.speed()), 13, false).FontFeatures("tnum")
			text(c, t.remaining(), 12, true).Width(112)
			label := "连接详情"
			if t.Expanded {
				label = "收起连接"
			}
			if action.Button(c, label, action.ButtonProps{Variant: action.Text, Size: theme.Small}).Label(label + " " + t.Name).Clicked() {
				t.Expanded = !t.Expanded
				c.Invalidate()
			}
		})
		if t.Expanded {
			d.connectionDetails(c, t)
		}
	})
}

func (d *demo) connectionDetails(c *ui.Context, t *task) {
	tk := theme.TokensOf(c)
	ui.Column(c).FillWidth().Padding(12, 16).Gap(8).Radius(tk.BorderRadius).Background(tk.BgLayout).Children(func() {
		text(c, "连接       已接收 / 分段大小                 当前速度", 12, true).Font("monospace")
		for n, value := range t.Segments {
			ui.Row(c.Key(n)).FillWidth().Gap(24).Children(func() {
				text(c, fmt.Sprintf("连接 %d", n+1), 12, true).Width(64)
				text(c, bytes(t.SizeMB/8*value)+" / "+bytes(t.SizeMB/8), 12, false).Width(200).FontFeatures("tnum")
				text(c, fmt.Sprintf("%.1f MB/s", t.rate(n)), 12, true).FontFeatures("tnum")
			})
		}
	})
}
