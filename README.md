# jisu

## Browser Takeover

Open jisu once to register its native messaging host for Chrome and Edge. Keep the app installed at the same path, and open it again after moving it.

Download the extension artifact or use this repository's `extension/` directory. Unzip the artifact first. Open `chrome://extensions` or `edge://extensions`, enable Developer mode, choose **Load unpacked**, and select that directory. No extension build step is needed.

Takeover hands files of at least 5 MB and common archives, video and installers to jisu. Hold Alt while starting a download to skip Takeover. If jisu is unavailable the browser download resumes. Page cookies are used only in memory; authenticated Jobs may need a new Takeover after restarting jisu.

jisu is a desktop download manager for Windows and macOS, built with Go and [NSGo](https://github.com/nsgo-dev/nsgo).

## Install

Get the latest installer from [GitHub Releases](https://github.com/jellli/jisu/releases/latest):

- **Windows:** Run the x64 setup executable. Windows may show a SmartScreen warning for the unsigned preview.
- **macOS:** Choose the Apple Silicon (`arm64`) or Intel (`x64`) DMG and drag jisu into Applications. This preview is not notarized. If macOS blocks it, see [Apple’s guidance for opening a trusted app](https://support.apple.com/102445).

## Download files

Paste a direct `https://` or `http://` download URL. jisu defaults to eight connections per job for files 4 MiB or larger when the server supports byte ranges and provides a reliable file validator. To change one job's connection target while it runs, select the job, open Details, and choose 4, 8, 16 or 32. Use the sidebar to filter by file type or queue, and right-click a selection to move it up or down within its queue.

Pause jobs or change the number of simultaneous downloads in settings. Interrupted active jobs automatically rejoin the queue when you reopen jisu; downloads you paused stay paused. Use Retry to resume a failed download.

If a server doesn't support safe resume or has changed the file, jisu starts that download from the beginning. Completed files never overwrite existing files. Partial data lives in a hidden `.jisu-*` folder in the destination directory and is removed on completion or when you remove an unfinished job. Save to a local filesystem that supports hard links, such as APFS or NTFS. Removing a completed job only removes its history entry.

The first release handles direct URLs, not browser capture, authenticated browser sessions, or video playlists.

## Develop

- Install [Go 1.27.3 or later](https://go.dev/dl/). Linux development needs GTK 4.12 or later with Pango and Cairo, such as Ubuntu 24.04.
- Run `go tool nsgo dev` to launch the app.
- Run `go vet ./...` and `go test -race ./...` for the headless checks, or `go tool nsgo test -target macos` (use `windows` or `linux` on those systems) to exercise the native workspace and capture screenshots.
- Run `go tool nsgo build` to create installers in `dist/releases` on supported hosts. The Windows installer needs NSIS.
