package main

import (
	"fmt"
	"math"
	"time"
)

type transferState string

const (
	downloading transferState = "downloading"
	paused      transferState = "paused"
	failed      transferState = "failed"
	completed   transferState = "completed"
)

var stateNames = map[transferState]string{
	downloading: "下载中", paused: "已暂停", failed: "连接中断", completed: "已完成",
}

var initialSegments = [8]float64{1, .78, .36, .90, .55, .48, .67, .30}
var connectionRates = [8]float64{2.4, 2.8, 2.3, 3.2, 2.7, 2.1, 2.5, 2.8}

type task struct {
	Name, Source string
	SizeMB       float64
	State        transferState
	Segments     [8]float64
	Expanded     bool
}

func sampleTasks() []task {
	return []task{
		{Name: "ubuntu-24.04.1-desktop-amd64.iso", Source: "releases.ubuntu.com", SizeMB: 6100, State: downloading, Segments: initialSegments},
		{Name: "Blender-4.3.2-macos-arm64.dmg", Source: "download.blender.org", SizeMB: 384, State: paused, Segments: [8]float64{.92, .6, .27, .51, .38, .7, .29, .41}},
		{Name: "Figma-Design-Assets-2026.zip", Source: "design-assets.example.com", SizeMB: 128, State: completed, Segments: [8]float64{1, 1, 1, 1, 1, 1, 1, 1}},
	}
}

func (t *task) progress() float64 {
	var sum float64
	for _, v := range t.Segments {
		sum += v
	}
	return sum / float64(len(t.Segments))
}

func (t *task) rate(n int) float64 {
	if t.State != downloading || t.Segments[n] >= 1 {
		return 0
	}
	return connectionRates[n]
}

func (t *task) speed() float64 {
	var total float64
	for n := range t.Segments {
		total += t.rate(n)
	}
	return total
}

func (t *task) advance(elapsed time.Duration) {
	if t.State != downloading || elapsed <= 0 || t.SizeMB <= 0 {
		return
	}
	for n, v := range t.Segments {
		t.Segments[n] = min(1, v+t.rate(n)*elapsed.Seconds()/(t.SizeMB/8))
	}
	if t.progress() >= 1 {
		t.State = completed
	}
}

func (t *task) activate() {
	switch t.State {
	case downloading:
		t.State = paused
	case paused, failed:
		t.State = downloading
	case completed:
		t.Expanded = !t.Expanded
	}
}

func (t *task) remaining() string {
	switch t.State {
	case completed:
		return "传输完成"
	case paused:
		return "等待继续"
	case failed:
		return "等待重试"
	}
	if t.speed() == 0 {
		return "等待连接"
	}
	s := int(math.Ceil(t.SizeMB * (1 - t.progress()) / t.speed()))
	if s >= 60 {
		return fmt.Sprintf("约 %d 分 %d 秒", s/60, s%60)
	}
	return fmt.Sprintf("约 %d 秒", s)
}

func bytes(mb float64) string {
	if mb >= 1000 {
		return fmt.Sprintf("%.2f GB", mb/1000)
	}
	return fmt.Sprintf("%.0f MB", mb)
}

type demo struct {
	Tasks         []task
	Filter        int
	Simulate      bool
	Scenario      string
	lastTick      time.Time
	wasSimulating bool
}

func newDemo() *demo {
	return &demo{Tasks: sampleTasks(), Scenario: string(downloading)}
}

func (d *demo) reset() {
	d.Tasks = sampleTasks()
	d.Filter = 0
	d.Scenario = string(downloading)
	d.lastTick = time.Time{}
}

func (d *demo) setScenario(value string) {
	s := transferState(value)
	if _, ok := stateNames[s]; !ok {
		return
	}
	d.Scenario = value
	d.Tasks[0].State = s
	d.Tasks[0].Segments = initialSegments
	if s == completed {
		d.Tasks[0].Segments = [8]float64{1, 1, 1, 1, 1, 1, 1, 1}
	}
	d.lastTick = time.Time{}
}

func (d *demo) tick(now time.Time) {
	if d.Simulate != d.wasSimulating {
		d.lastTick = now
		d.wasSimulating = d.Simulate
	}
	if d.Simulate && !d.lastTick.IsZero() && now.Sub(d.lastTick) >= time.Second {
		for i := range d.Tasks {
			d.Tasks[i].advance(now.Sub(d.lastTick))
		}
		d.Scenario = string(d.Tasks[0].State)
		d.lastTick = now
	}
	if !d.Simulate || d.lastTick.IsZero() {
		d.lastTick = now
	}
}

func (d *demo) count(s transferState) int {
	var count int
	for _, t := range d.Tasks {
		if t.State == s {
			count++
		}
	}
	return count
}

func (d *demo) totalSpeed() float64 {
	var total float64
	for i := range d.Tasks {
		total += d.Tasks[i].speed()
	}
	return total
}

func (d *demo) visible(t *task) bool {
	return d.Filter == 0 || d.Filter == 1 && t.State == downloading ||
		d.Filter == 2 && t.State == paused || d.Filter == 3 && t.State == completed
}
