# jisu

jisu is a desktop download manager for Windows and macOS, built with Go and [MyGo](https://github.com/egoist/mygo).

## Install

Get the latest installer from [GitHub Releases](https://github.com/jellli/jisu/releases/latest):

- **Windows:** Run the x64 setup executable. Windows may show a SmartScreen warning for the unsigned preview.
- **macOS:** Choose the Apple Silicon (`arm64`) or Intel (`amd64`) DMG and drag jisu into Applications. This preview is not notarized. If macOS blocks it, see [Apple’s guidance for opening a trusted app](https://support.apple.com/102445).

## Download files

Paste a direct `https://` or `http://` download URL. jisu queues the file and downloads up to four parts in parallel for files 4 MiB or larger when the server supports byte ranges and provides a reliable file validator. Pause jobs or change the number of simultaneous downloads in settings. Interrupted active jobs automatically rejoin the queue when you reopen jisu; downloads you paused stay paused. Use Retry to resume a failed download.

If a server doesn't support safe resume or has changed the file, jisu starts that download from the beginning. Completed files never overwrite existing files. Partial data lives in a hidden `.jisu-*` folder in the destination directory and is removed on completion or when you remove an unfinished job. Save to a local filesystem that supports hard links, such as APFS or NTFS. Removing a completed job only removes its history entry.

The first release handles direct URLs, not browser capture, authenticated browser sessions, or video playlists.

## Develop

- Install [Go 1.27.3 or later](https://go.dev/dl/). The UI uses the system webview: WKWebView on macOS, WebView2 on Windows, or WebKitGTK for development on Linux.
- Run `go run .` to launch the desktop app. The frontend is embedded in the Go binary and does not need Node.js.
- Run `go test ./...` to test the queue; `go tool mygo build` creates distributable packages on supported build hosts.
