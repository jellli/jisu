// Jisu's isolated native Kite v2 study. No webview or download engine.
package main

import (
	"flag"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type prototype struct {
	tasks                          []transfer
	filter                         queueFilter
	dark, simulate, settings       bool
	notice                         string
	lastTick, lastSample           time.Time
	wasSimulating                  bool
	selected                       int
	detailsOpen                    bool
	category, queue, query, sortBy string
	sortDesc                       bool
	checked                        map[int]bool
	tab                            int
	adding                         bool
	newURLs, addError              string
	policy                         int
	queueShare                     float32
}

func newPrototype() *prototype {
	return &prototype{tasks: fixtures(), detailsOpen: true, checked: map[int]bool{0: true}, queueShare: 66}
}

func (s *prototype) reset() {
	s.tasks, s.filter, s.notice = fixtures(), allTasks, ""
	s.category, s.queue, s.query, s.sortBy = "", "", "", ""
	s.lastTick, s.lastSample, s.wasSimulating = time.Time{}, time.Time{}, false
	s.selected, s.detailsOpen, s.tab = 0, true, 0
	s.checked = map[int]bool{0: true}
	s.policy = 0
}

func (s *prototype) reconcileSelection() {
	if s.selected >= 0 && s.selected < len(s.tasks) && s.visible(s.selected) {
		return
	}
	s.selected = -1
	for _, i := range s.visibleTasks() {
		s.selected = i
		return
	}
}

func (s *prototype) scenario(state transferState) {
	if len(s.tasks) == 0 {
		s.reset()
	}
	s.tasks[0] = fixtures()[0]
	s.tasks[0].state = state
	if state == failed {
		s.tasks[0].failure = "连接 03 请求超时"
	}
	if state == completed {
		s.tasks[0].segments = [8]float64{1, 1, 1, 1, 1, 1, 1, 1}
	}
	s.tasks[0].record("切换模拟状态：" + string(state))
	s.lastTick, s.wasSimulating, s.notice = time.Time{}, false, ""
}

func (s *prototype) tick(now time.Time) {
	if s.simulate && s.wasSimulating && !s.lastTick.IsZero() {
		for i := range s.tasks {
			s.tasks[i].advance(now.Sub(s.lastTick))
		}
	}
	if s.simulate && (s.lastSample.IsZero() || now.Sub(s.lastSample) >= time.Second) {
		for i := range s.tasks {
			t := &s.tasks[i]
			t.history = append(t.history, t.speed())
			if len(t.history) > 60 {
				t.history = t.history[len(t.history)-60:]
			}
		}
		s.lastSample = now
	}
	s.lastTick, s.wasSimulating = now, s.simulate
}

func (s *prototype) count(f queueFilter) int {
	n := 0
	for i := range s.tasks {
		if f.matches(&s.tasks[i]) {
			n++
		}
	}
	return n
}

func theme(dark bool) ui.Theme {
	t := *ui.LightTheme()
	if dark {
		t = *ui.DarkTheme()
		t.Background, t.Surface = ui.Hex("#202421"), ui.Hex("#191d1a")
		t.Text, t.TextMuted = ui.Hex("#e6eae7"), ui.Hex("#a9b3ac")
		t.Border, t.Accent = ui.Hex("#39423c"), ui.Hex("#7cc69d")
	} else {
		t.Background, t.Surface = ui.Hex("#f3f4f2"), ui.Hex("#fcfdfb")
		t.Text, t.TextMuted = ui.Hex("#202722"), ui.Hex("#58645c")
		t.Border, t.Accent = ui.Hex("#d9ded9"), ui.Hex("#226b46")
	}
	t.FontSize, t.Radius, t.Focus = 13, 5, t.Accent
	return t
}

func (s *prototype) errorColor() ui.Color {
	if s.dark {
		return ui.Hex("#f1a39b")
	}
	return ui.Hex("#a13028")
}

func main() {
	dark := flag.Bool("dark", false, "Start in dark appearance")
	flag.Parse()
	s := newPrototype()
	s.dark = *dark
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{Title: "Jisu", Width: 1180, Height: 780,
			MinWidth: 720, MinHeight: 500, Content: ui.View(s.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
