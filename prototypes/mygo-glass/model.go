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
	completed   transferState = "已完成"
)

type transfer struct {
	name, source, ext string
	size              float64 // Decimal MB, matching the web prototype.
	state             transferState
	segments          [8]float64
	expanded          bool
}

// Fixtures and per-connection rates match PR #4's resetTasks(), baseSegments,
// and rates. Simulation is opt-in so comparisons start at identical values.
func fixtures() []transfer {
	return []transfer{
		{"ubuntu-24.04.1-desktop-amd64.iso", "releases.ubuntu.com", "ISO", 6100, downloading, [8]float64{1, .78, .36, .90, .55, .48, .67, .30}, false},
		{"Blender-4.3.2-macos-arm64.dmg", "download.blender.org", "DMG", 384, paused, [8]float64{.92, .6, .27, .51, .38, .7, .29, .41}, false},
		{"Figma-Design-Assets-2026.zip", "design-assets.example.com", "ZIP", 128, completed, [8]float64{1, 1, 1, 1, 1, 1, 1, 1}, false},
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
	}
	if t.speed() == 0 {
		return "正在连接"
	}
	seconds := int(math.Ceil(t.size * (1 - t.progress()) / t.speed()))
	if seconds >= 60 {
		return fmt.Sprintf("约 %d 分 %d 秒", seconds/60, seconds%60)
	}
	return fmt.Sprintf("约 %d 秒", seconds)
}

func (t *transfer) toggle() {
	switch t.state {
	case downloading:
		t.state = paused
	case paused:
		t.state = downloading
	}
}

func (t *transfer) advance(elapsed time.Duration) {
	if t.state != downloading {
		return
	}
	for n := range t.segments {
		t.segments[n] = math.Min(1, t.segments[n]+t.rate(n)*elapsed.Seconds()/(t.size/8))
	}
	if t.progress() >= 1 {
		t.state = completed
	}
}

func bytes(value float64) string {
	if value >= 1000 {
		return fmt.Sprintf("%.2f GB", value/1000)
	}
	return fmt.Sprintf("%.0f MB", value)
}
