# jisu

jisu is a desktop download manager for Windows and macOS, built with Go and [NSGo](https://github.com/nsgo-dev/nsgo).

## Browser Takeover

Open jisu once to register its native messaging host for Chrome and Edge. Keep the app installed at the same path, and open it again after moving it.

Download the extension artifact or use this repository's `extension/` directory. Unzip the artifact first. Open `chrome://extensions` or `edge://extensions`, enable Developer mode, choose **Load unpacked**, and select that directory. No extension build step is needed.

Takeover hands files of at least 5 MB and common archives, video and installers to jisu. Hold Alt while starting a download to skip Takeover. If jisu is unavailable the browser download resumes. Page cookies are used only in memory; authenticated Jobs may need a new Takeover after restarting jisu.

## Install

Get the latest release from [GitHub Releases](https://github.com/jellli/jisu/releases/latest). Under **Assets**, download **one** file (`<version>` is the release number, such as `0.2.0`):

| Your computer | File |
| --- | --- |
| Mac with Apple Silicon (M1 or newer) | `jisu-<version>-macos-arm64.dmg` |
| Mac with an Intel processor | `jisu-<version>-macos-x64.dmg` |
| Windows PC | `jisu-<version>-windows-x64-setup.exe` |

Not sure which Mac you have? Open the Apple menu > **About This Mac**. "Chip: Apple M…" means Apple Silicon; "Processor: Intel" means Intel.

This preview is not notarized on macOS or code-signed on Windows, so both systems warn about it the first time. Follow the steps below once.

### macOS: "…dmg is damaged and can’t be opened"

jisu is not damaged. macOS shows this message for any app downloaded from a browser that isn't notarized by Apple, and it gives no "Open Anyway" button for the disk image.

1. Download the DMG into your **Downloads** folder. If the "damaged" message appears, click **Cancel** (not Move to Trash).
2. Open **Terminal** (press Command + Space, type `Terminal`, press Return).
3. Paste this line and press Return. It removes the "downloaded from the internet" mark from the jisu disk image and prints nothing when it works:

   ```sh
   xattr -c ~/Downloads/jisu-*-macos-*.dmg
   ```

4. Double-click the DMG in Downloads, drag **jisu** onto **Applications**, then open jisu from Applications.

If you already copied jisu to Applications and it still won't open, run this line, then open jisu again:

```sh
xattr -cr /Applications/jisu.app
```

### Windows: "Windows protected your PC"

1. Run `jisu-<version>-windows-x64-setup.exe`.
2. If a blue SmartScreen window says "Windows protected your PC", click **More info**.
3. Click **Run anyway**, then follow the installer.

## Download files

Paste a direct `https://` or `http://` download URL. jisu defaults to eight connections per job for files 4 MiB or larger when the server supports byte ranges and provides a reliable file validator. To change one job's connection target while it runs, select the job, open Details, and choose 4, 8, 16 or 32. Use the sidebar to filter by file type or queue, and right-click a selection to move it up or down within its queue.

Pause jobs or change the number of simultaneous downloads in settings. Interrupted active jobs automatically rejoin the queue when you reopen jisu; downloads you paused stay paused. Use Retry to resume a failed download.

If a server doesn't support safe resume or has changed the file, jisu starts that download from the beginning. Completed files never overwrite existing files. Partial data lives in a hidden `.jisu-*` folder in the destination directory and is removed on completion or when you remove an unfinished job. Save to a local filesystem that supports hard links, such as APFS or NTFS. Removing a completed job only removes its history entry.

Takeover does not carry over an authenticated browser session; for sites that need one, see [Browser Takeover](#browser-takeover). Video playlists are not supported.

### Proxies

jisu uses the macOS or Windows system proxy automatically when no proxy environment variables are set. Changes to the system proxy take effect within a few seconds, including when you toggle tools such as Clash. HTTP, HTTPS and SOCKS proxies and host bypass lists are supported; automatic proxy configuration scripts (PAC) are not.

If any of `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY` or `NO_PROXY` (or their lowercase forms) is set, jisu uses Go's standard environment proxy behavior instead of the system settings. That behavior supports `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY`, but does not use `ALL_PROXY`. Localhost and loopback addresses always connect directly. Other operating systems use environment proxy settings only.

## Develop

- Install [Go 1.27.3 or later](https://go.dev/dl/). Linux development needs GTK 4.12 or later with Pango and Cairo, such as Ubuntu 24.04.
- Run `go tool nsgo dev` to launch the app.
- Run `go vet ./...` and `go test -race ./...` for the headless checks, or `go tool nsgo test -target macos` (use `windows` or `linux` on those systems) to exercise the native workspace and capture screenshots.
- Run `go tool nsgo build` to create installers in `dist/releases` on supported hosts. The Windows installer needs NSIS.
