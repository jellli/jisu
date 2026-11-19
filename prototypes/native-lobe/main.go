package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	state := newDemo()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title: "极速 · Lobe MyGo 原生下载行",
			Width: 1120, Height: 820, MinWidth: 880, MinHeight: 600,
			Content: ui.View(state.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
