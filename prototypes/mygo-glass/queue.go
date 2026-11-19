package main

import (
	"fmt"
	"math"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

func categoryFor(ext string) string {
	switch strings.ToUpper(ext) {
	case "ZIP", "GZ", "XZ", "RAR", "7Z":
		return "archive"
	case "ISO", "DMG", "EXE", "PKG":
		return "iso"
	case "MP4", "MKV", "MOV", "WEBM":
		return "video"
	default:
		return "document"
	}
}

func (s *prototype) visible(i int) bool {
	t := &s.tasks[i]
	return s.filter.matches(t) && (s.category == "" || categoryFor(t.ext) == s.category) &&
		s.queue != "night" && (s.query == "" || strings.Contains(strings.ToLower(t.name+" "+t.source+" "+t.protocol()), strings.ToLower(s.query)))
}

func (s *prototype) visibleTasks() []int {
	indices := []int{}
	for i := range s.tasks {
		if s.visible(i) {
			indices = append(indices, i)
		}
	}
	if s.sortBy != "" {
		sort.SliceStable(indices, func(i, j int) bool {
			a, b := &s.tasks[indices[i]], &s.tasks[indices[j]]
			cmp := 0
			x, y := float64(0), float64(0)
			switch s.sortBy {
			case "size":
				x, y = a.size, b.size
			case "progress":
				x, y = a.progress(), b.progress()
			case "speed":
				x, y = a.speed(), b.speed()
			case "eta":
				x, y = a.remainingSeconds(), b.remainingSeconds()
			default:
				cmp = strings.Compare(a.name, b.name)
			}
			if s.sortBy != "name" {
				if x < y {
					cmp = -1
				}
				if x > y {
					cmp = 1
				}
			}
			if s.sortDesc {
				return cmp > 0
			}
			return cmp < 0
		})
	}
	return indices
}

func (s *prototype) choose(i int) {
	s.selected, s.detailsOpen = i, true
	s.checked = map[int]bool{i: true}
}

func (s *prototype) checkedCount() int {
	n := 0
	for i, v := range s.checked {
		if v && i >= 0 && i < len(s.tasks) {
			n++
		}
	}
	return n
}

func (s *prototype) setAll(state transferState, selectedOnly bool) {
	for i := range s.tasks {
		if selectedOnly && !s.checked[i] {
			continue
		}
		t := &s.tasks[i]
		if t.state == completed {
			continue
		}
		if state == downloading && t.state != downloading || state == paused && t.state == downloading {
			t.toggle()
			t.record(string(t.state) + "（模拟）")
		}
	}
}

func (s *prototype) action() {
	if s.selected < 0 || s.selected >= len(s.tasks) {
		return
	}
	t := &s.tasks[s.selected]
	if t.state == completed {
		s.notice = "模拟文件位置：~/Downloads/" + t.name
		s.tab = 1
		return
	}
	wasFailed := t.state == failed
	t.toggle()
	t.record(string(t.state) + "（模拟，保留已收字节）")
	if wasFailed {
		s.notice = "已从保留的分段继续：" + t.name
	}
}

func (s *prototype) removeChecked() {
	kept := make([]transfer, 0, len(s.tasks))
	for i, t := range s.tasks {
		if !s.checked[i] {
			kept = append(kept, t)
		}
	}
	s.tasks = kept
	s.checked = map[int]bool{}
	s.selected = -1
	s.reconcileSelection()
	s.notice = "仅移除了模拟任务；重置样本可恢复"
}

// Only model tasks are added: URLs are never requested by this prototype.
func (s *prototype) addTasks() bool {
	lines := strings.Fields(s.newURLs)
	if len(lines) == 0 {
		s.addError = "请输入 HTTP 或 HTTPS 链接"
		return false
	}
	added := []transfer{}
	for _, line := range lines {
		u, err := url.Parse(line)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
			s.addError = "只接受 HTTP/HTTPS 链接；未添加任何任务"
			return false
		}
		name := path.Base(u.Path)
		if name == "." || name == "/" || name == "" {
			name = "download.bin"
		}
		t := transfer{name: name, source: u.Hostname(), ext: strings.ToUpper(strings.TrimPrefix(path.Ext(name), ".")), size: 128, state: paused, scheme: u.Scheme}
		t.record("添加 HTTP/HTTPS 模拟任务，样本大小 128 MB；未请求网络")
		added = append(added, t)
	}
	first := len(s.tasks)
	s.tasks = append(s.tasks, added...)
	s.filter, s.category, s.queue, s.query = allTasks, "", "", ""
	s.choose(first)
	s.newURLs, s.addError, s.notice = "", "", "已添加模拟任务，未发起网络请求"
	return true
}

func (t *transfer) record(message string) {
	t.events = append(t.events, time.Now().Format("15:04:05")+"  "+message)
}

func (t *transfer) protocol() string {
	if t.scheme == "http" {
		return "HTTP"
	}
	return "HTTPS"
}

func (t *transfer) remainingSeconds() float64 {
	if t.state != downloading {
		return math.Inf(1)
	}
	seconds := float64(0)
	for n, v := range t.segments {
		if r := t.rate(n); r > 0 {
			seconds = math.Max(seconds, t.size/8*(1-v)/r)
		}
	}
	return seconds
}

type rangeView struct {
	start, end               int64
	fraction, received, rate float64
	state                    string
}

func (t *transfer) connectionCount() int {
	if t.previewCount == 4 || t.previewCount == 16 || t.previewCount == 32 {
		return t.previewCount
	}
	return 8
}

// Rebin the original eight ranges for layout previews. Their exact received
// coverage is preserved; this does not change the simulated network workers.
func (t *transfer) ranges() []rangeView {
	count := t.connectionCount()
	total := t.size * 1e6
	rows := make([]rangeView, count)
	for n := range rows {
		lo, hi := float64(n)*total/float64(count), float64(n+1)*total/float64(count)
		r := &rows[n]
		r.start, r.end = int64(math.Round(lo)), int64(math.Round(hi))-1
		for k, v := range t.segments {
			start := float64(k) * total / 8
			stop := start + total/8
			r.received += math.Max(0, math.Min(hi, start+v*total/8)-math.Max(lo, start))
			r.rate += t.rate(k) * math.Max(0, math.Min(hi, stop)-math.Max(lo, start)) / (total / 8)
		}
		r.fraction = r.received / (hi - lo)
		r.state = string(t.state)
		if r.fraction >= 1-1e-9 {
			r.state = "已完成"
			r.rate = 0
		} else if t.state == downloading {
			r.state = "传输中"
		} else if t.state == failed {
			r.state = "等待重试"
			if lo < 3*total/8 && hi > 2*total/8 {
				r.state = "请求超时"
			}
		}
	}
	return rows
}

func grouped(n int64) string {
	s := fmt.Sprintf("%d", n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
