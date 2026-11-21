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

## Updates

In **设置**, **检查更新** checks for a new version; the dialog also shows the current version. Release builds check daily in the background using Sparkle on macOS and WinSparkle on Windows. Their native dialogs handle confirmation, download progress, installation and restart. Updates download full packages, not deltas, and verify their EdDSA signatures before installing.

**v0.2.0 has no updater. Install the next release manually once**, using the steps above. After that, updates are available in the app. Initial downloads still have the unsigned-build warnings described above. macOS packages are ad hoc signed, without Developer ID or notarization; NSGo documents successful Sparkle updates with ad hoc signing.

Release maintainers must set the GitHub Actions secret `NSGO_UPDATES_KEY` to the secret matching `updates.key` in `nsgo.yaml`. Keep a safe backup: changing the public key prevents existing installations from trusting new updates. Every release must include `appcast-windows_x64.xml`, `appcast-macos_arm64.xml` and `appcast-macos_x64.xml` alongside the installers and `SHA256SUMS`. The default feed reads the latest release, so a missing appcast breaks checks for that target. Pull requests without the signing secret can still build unsigned test artifacts; tagged releases require all signed appcasts.

## Download files

Paste a direct `https://` or `http://` download URL. jisu defaults to eight connections per job for files 4 MiB or larger when the server supports byte ranges and provides a reliable file validator. To change one job's connection target while it runs, select the job, open Details, and choose 4, 8, 16 or 32. Use the sidebar to filter by file type or queue, and right-click a selection to move it up or down within its queue.

Pause jobs or change the number of simultaneous downloads in settings. Interrupted active jobs automatically rejoin the queue when you reopen jisu; downloads you paused stay paused. Use Retry to resume a failed download.

In Settings → Network, choose System (跟随系统, the default, using environment or operating system proxy settings), None (不使用, direct), or Custom (自定义). System and None take effect immediately. Custom accepts `http://`, `https://`, `socks5://`, or `socks5h://` URLs with a host and port, optionally including `user:password@`. A bare `host:port` uses HTTP. The address is masked to hide inline credentials. A valid address takes effect when you press Enter, leave the field, or close Settings; an invalid address leaves the previous proxy in place. Changes affect new requests without restarting; active transfers are not interrupted. Proxy settings, including credentials, are stored in your user configuration directory's `jisu/queue.json`, so protect that file. Browser Takeover's loopback connection always stays direct.

Each Job also has a proxy section in its inspector: 跟随全局 (follow global, the default), 系统 (System), 不使用 (direct), or 自定义 (Custom). Select one Job, choose a mode, enter a Custom URL if needed, and press 应用代理. Overrides are saved with the Job and affect its subsequent requests, including redirects and Segments, without restarting the transfer. Choosing 跟随全局 clears the override. Page sniffing continues to use the global proxy.

If a server doesn't support safe resume or has changed the file, jisu starts that download from the beginning. Completed files never overwrite existing files. Partial data lives in a hidden `.jisu-*` folder in the destination directory and is removed on completion or when you remove an unfinished job. Save to a local filesystem that supports hard links, such as APFS or NTFS. Removing a completed job only removes its history entry.

Takeover does not carry over an authenticated browser session; for sites that need one, see [Browser Takeover](#browser-takeover). Video playlists are not supported.

### Proxies

In System mode, jisu uses the macOS or Windows system proxy automatically when no non-empty HTTP or HTTPS proxy environment variables are configured. Changes to the system proxy take effect within a few seconds, including when you toggle tools such as Clash. HTTP, HTTPS and SOCKS proxies and host bypass lists are supported; automatic proxy configuration scripts (PAC) are not.

If `HTTP_PROXY` or `HTTPS_PROXY` (or their lowercase forms) has a non-empty value, System mode uses Go's standard environment proxy behavior instead of the system settings, including `NO_PROXY` bypasses. Otherwise, `NO_PROXY` (or `no_proxy`) adds bypasses to the system proxy's own exceptions. Empty proxy variables and `ALL_PROXY` do not disable the system proxy; `ALL_PROXY` is ignored. In System mode, localhost and loopback addresses always connect directly. Other operating systems use environment proxy settings only.

## Develop

- Install [Go 1.27.3 or later](https://go.dev/dl/). Linux development needs GTK 4.12 or later with Pango and Cairo, such as Ubuntu 24.04.
- Run `go tool nsgo dev` to launch the app.
- Run `go vet ./...` and `go test -race ./...` for the headless checks, or `go tool nsgo test -target macos` (use `windows` or `linux` on those systems) to exercise the native workspace and capture screenshots.
- Run `go tool nsgo build` to create installers in `dist/releases` on supported hosts. The Windows installer needs NSIS.
