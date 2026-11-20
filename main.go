package main

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jellli/jisu/internal/download"
	"github.com/jellli/jisu/internal/takeover"
	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
)

func main() {
	config, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "chrome-extension://") {
		if err := takeover.RunHost(os.Stdin, os.Stdout, config, takeover.StartApp); err != nil {
			log.Print(err)
		}
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	var queue *download.Queue
	var endpoint *takeover.Endpoint
	nsgo.Run(func(app *core.Scope) {
		var openErr error
		queue, openErr = download.Open(filepath.Join(config, "jisu", "queue.json"), downloadsDir(config, home))
		if openErr != nil {
			nsgo.SetMainView(app, func(cx *core.Scope) core.Node {
				cx.SetTitle("jisu")
				return core.Text{Content: "无法打开下载队列：" + openErr.Error(), Style: core.Style{Padding: core.All(16)}}
			})
			return
		}
		if err := takeover.Register(home, config); err != nil {
			log.Print(err)
		}
		endpoint, openErr = takeover.Start(config, queue)
		if openErr != nil {
			log.Print(openErr)
		}
		nsgo.SetMainView(app, workspace(queue))
	}, nsgo.WindowSize(1100, 680), nsgo.WindowConstraints(core.WindowConstraints{MinWidth: 740, MinHeight: 500}),
		nsgo.Appearance(core.AppearanceLight))
	if queue != nil {
		if endpoint != nil {
			endpoint.Close()
		}
		queue.Close()
	}
}

func downloadsDir(config, home string) string {
	if runtime.GOOS == "linux" {
		if f, err := os.Open(filepath.Join(config, "user-dirs.dirs")); err == nil {
			defer f.Close()
			s := bufio.NewScanner(f)
			for s.Scan() {
				key, value, ok := strings.Cut(strings.TrimSpace(s.Text()), "=")
				if ok && key == "XDG_DOWNLOAD_DIR" {
					path := strings.ReplaceAll(strings.Trim(value, `"`), "$HOME", home)
					if filepath.IsAbs(path) {
						return path
					}
				}
			}
		}
	}
	return filepath.Join(home, "Downloads")
}
