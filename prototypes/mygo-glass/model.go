package main

import (
	"fmt"
	"math"
	"time"
)

type transferState string

const (
	downloading transferState = "下载中"
	paused      transferState = "已暂停"
	failed      transferState = "连接中断"
	completed   transferState = "已完成"
)

type transfer struct {
	name, source, ext string
	size              float64 // Decimal MB, matching the web prototype.
	state             transferState
	segments          [8]float64
	failure           string
}

// Fixtures and per-connection rates match PR #4's resetTasks(), baseSegments,
// and rates. Simulation is opt-in so comparisons start at identical values.
func fixtures() []transfer {
	return []transfer{
		{name: "ubuntu-24.04.1-desktop-amd64.iso", source: "releases.ubuntu.com", ext: "ISO", size: 6100, state: downloading, segments: [8]float64{1, .78, .36, .90, .55, .48, .67, .30}},
		{name: "Blender-4.3.2-macos-arm64.dmg", source: "download.blender.org", ext: "DMG", size: 384, state: paused, segments: [8]float64{.92, .6, .27, .51, .38, .7, .29, .41}},
		{name: "Figma-Design-Assets-2026.zip", source: "design-assets.example.com", ext: "ZIP", size: 128, state: completed, segments: [8]float64{1, 1, 1, 1, 1, 1, 1, 1}},
	}
}

var rates = [8]float64{0, 2.8, 2.3, 3.2, 2.7, 2.1, 2.5, 2.8}

func (t *transfer) progress() float64 {
	var total float64
	for _, value := range t.segments {
		total += value
	}
	return total / 8
}

func (t *transfer) rate(n int) float64 {
	if t.state != downloading || t.segments[n] >= 1 {
		return 0
	}
	if rates[n] == 0 {
		return 2.4
	}
	return rates[n]
}

func (t *transfer) speed() float64 {
	var total float64
	for n := range t.segments {
		total += t.rate(n)
	}
	return total
}

func (t *transfer) remaining() string {
	switch t.state {
	case completed:
		return "传输完成"
	case paused:
		return "等待继续"
	case failed:
		return "等待重试"
	}
	if t.speed() == 0 {
		return "正在连接"
	}
	// Connections keep their own fixed rates in the simulator. The slowest
	// remaining byte range, not aggregate throughput, determines completion.
	var seconds float64
	for n, value := range t.segments {
		if rate := t.rate(n); rate > 0 {
			seconds = math.Max(seconds, t.size/8*(1-value)/rate)
		}
	}
	if seconds >= 60 {
		return fmt.Sprintf("约 %d 分钟", int(math.Ceil(seconds/60)))
	}
	return fmt.Sprintf("约 %d 秒", max(10, int(math.Ceil(seconds/10))*10))
}

func (t *transfer) toggle() {
	switch t.state {
	case downloading:
		t.state = paused
	case paused, failed:
		t.state = downloading
		t.failure = ""
	}
}

func (t *transfer) advance(elapsed time.Duration) {
	if t.state != downloading || elapsed <= 0 {
		return
	}
	for n := range t.segments {
		t.segments[n] = math.Min(1, t.segments[n]+t.rate(n)*elapsed.Seconds()/(t.size/8))
	}
	if t.progress() >= 1 {
		t.state = completed
	}
}

func (t *transfer) connectionState(n int) string {
	if t.segments[n] >= 1 {
		return "已完成"
	}
	switch t.state {
	case downloading:
		return "传输中"
	case failed:
		if n == 2 {
			return "请求超时"
		}
		return "等待重试"
	default:
		return "已暂停"
	}
}

type queueFilter int

const (
	allTasks queueFilter = iota
	activeTasks
	pausedTasks
	attentionTasks
	completeTasks
)

var filterNames = []string{"全部", "进行中", "已暂停", "需处理", "已完成"}

func (f queueFilter) matches(t *transfer) bool {
	switch f {
	case activeTasks:
		// An interrupted active transfer must not disappear from its queue.
		return t.state == downloading || t.state == failed
	case pausedTasks:
		return t.state == paused
	case attentionTasks:
		return t.state == failed
	case completeTasks:
		return t.state == completed
	default:
		return true
	}
}

func bytes(value float64) string {
	if value >= 1000 {
		return fmt.Sprintf("%.2f GB", value/1000)
	}
	return fmt.Sprintf("%.0f MB", value)
}
