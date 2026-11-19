package main

import (
	"log"
	"os"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dir, err := os.MkdirTemp("", "jisu-nsgo-tasks-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	q, closeDemo, err := demoQueue(dir)
	if err != nil {
		return err
	}
	defer closeDemo()
	nsgo.Run(taskTable(q), nsgo.WindowSize(1040, 570), nsgo.Appearance(core.AppearanceLight))
	return nil
}
