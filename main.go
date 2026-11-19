package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/egoist/mygo"
	"github.com/jellli/jisu/internal/download"
)

//go:embed all:src/renderer
var renderer embed.FS

var stateChanged = mygo.NewEvent[download.Snapshot]("jisu:state")

type service struct{ queue *download.Queue }

func (s service) GetState() download.Snapshot    { return s.queue.State() }
func (s service) Add(url string) error           { return s.queue.Add(url) }
func (s service) Pause(id string) error          { return s.queue.Pause(id) }
func (s service) Resume(id string) error         { return s.queue.Resume(id) }
func (s service) Remove(id string) error         { return s.queue.Remove(id) }
func (s service) SetConnections(value int) error { return s.queue.SetConnections(value) }

func (s service) SetConcurrency(value int) error { return s.queue.SetConcurrency(value) }

func (s service) ChooseDirectory(ctx context.Context) (*string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: mygo.CallerWindow(ctx), Title: "Choose download folder", Directory: true,
		CreateDirectories: true, DefaultPath: s.queue.State().DownloadDirectory,
	})
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	if err := s.queue.SetDirectory(paths[0]); err != nil {
		return nil, err
	}
	return &paths[0], nil
}

func (s service) Reveal(id string) error {
	path, err := s.queue.CompletedPath(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	mygo.Shell.ShowItemInFolder(path)
	return nil
}

func publishState(ctx context.Context, win *mygo.Window, q *download.Queue) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var previous download.Snapshot
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			next := q.State()
			if reflect.DeepEqual(previous, next) {
				continue
			}
			if err := stateChanged.Emit(win, next); err != nil {
				return
			}
			previous = next
		}
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	mygo.App.SetName("jisu")
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	web, err := fs.Sub(renderer, "src/renderer")
	if err != nil {
		return err
	}
	mygo.SetFrontend(web)
	var window *mygo.Window
	var queue *download.Queue
	var startupErr error
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		if queue != nil {
			queue.Close()
		}
	}()
	mygo.App.OnSecondInstance(func([]string, string) {
		if window != nil {
			window.Restore()
			window.Focus()
		}
	})
	mygo.App.OnWindowAllClosed(func() { mygo.App.Quit() })
	mygo.App.WhenReady(func() {
		queue, startupErr = openQueue()
		if startupErr != nil {
			go func() {
				mygo.Dialog.Error("Could not start jisu", startupErr.Error())
				mygo.App.Quit()
			}()
			return
		}
		mygo.BindAs("Jisu", service{queue})
		window = mygo.NewWindow(mygo.WindowOptions{
			Title: "jisu", Width: 1000, Height: 660, MinWidth: 720, MinHeight: 500,
		})
		page := window.Page()
		page.OnWillNavigate(func(e *mygo.NavigateEvent) { e.PreventDefault() })
		page.SetWindowOpenHandler(func(mygo.WindowOpenRequest) *mygo.WindowOptions { return nil })
		page.OnWillDownload(func(e *mygo.DownloadEvent) { e.PreventDefault() })
		if err := page.LoadURL("/"); err != nil {
			startupErr = err
			mygo.App.Quit()
			return
		}
		go publishState(ctx, window, queue)
	})
	if err := mygo.App.Run(); err != nil {
		return err
	}
	return startupErr
}

func openQueue() (*download.Queue, error) {
	data, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	folder, err := mygo.App.Path(mygo.PathDownloads)
	if err != nil || folder == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		folder = filepath.Join(home, "Downloads")
	}
	return download.Open(filepath.Join(data, "queue.json"), folder)
}
