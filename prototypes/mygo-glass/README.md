# Jisu — native Kite v2 study

An isolated MyGo `ui` prototype following the user's Kite v2 reference rather
than reinterpreting its layout. Only Jisu's name/colors replace Kite's identity.
There is no HTML, CSS, JavaScript, webview or third-party component library.
Production UI and the download engine are untouched.

## Run

Requires Go 1.27.3+; MyGo is pinned to stable v0.10.0.

```sh
cd prototypes/mygo-glass
go run .
# Optional: start dark
go run . -dark
```

Desktop builds use `CGO_ENABLED=0`. On Ubuntu, install the system runtime/fonts:

```sh
sudo apt-get install libwebkit2gtk-4.1-0 fonts-noto-cjk
```

The native window uses the GPU when available and MyGo's official CPU renderer
otherwise. The Linux VM captures use the CPU fallback. OS window chrome and
font metrics remain native; fake macOS traffic lights are not drawn on Linux.

## Reference fidelity and scope

- 44px toolbar with Jisu identity, new-download action, global start/pause,
  remove-selection, search, appearance and inspector toggles.
- 200px sidebar with download status counts, file categories and queues.
- 44px table rows with selection checkboxes, protocol/name/source, size,
  in-row micro-segment progress, speed/state and remaining-time columns.
  Clickable headers sort; search and sidebar filters reconcile selection.
- Resizable split between queue and bottom inspector. Five reference tabs:
  chunks/threads, files/tracks, speed, verification/automation and logs.
- The chunk pane has one full-width segment map, 4/8/16/32 layout controls,
  and an aligned byte-range/received/total/rate/state table. Completed tasks
  instead show the file, source and mock saved path.
- Multi-selection has batch start/pause actions. New downloads accept only
  HTTP/HTTPS URLs and add paused 128 MB mock files, without network requests.
- Failed transfers stay in the active queue and needs-attention filter.
  Connection 03 times out without losing bytes; Retry retains progress.
  Labeled pause/resume/retry/show-in-folder actions live in the inspector.

**This is still a simulated design prototype.** The 4/8/16/32 controls rebin
the original eight ranges' exact received coverage for layout previews; they
do not create or change network workers. Row maps show at most 16 visual slots.
Actual concurrency switching, high-frequency engine updates, disk I/O and
performance work belong to engine integration, not this prototype.

Only HTTP/HTTPS badges are present. Unsupported BT/HLS/SFTP/DASH protocol
badges and media-sniffing navigation are omitted. Verification and automation
keep their reference layout but are visibly disabled: there are no local
download files to hash or act on. No success hashes or I/O measurements are
fabricated. Speed plots contain actual simulator samples, not random curves;
connection logs describe simulation, not fabricated server responses.

Most Glass decoration goes away to follow v2. The official
`github.com/egoist/mygo/plugins/glass` material is retained only for the small
labeled inspector action, never a toolbar or table surface.

## Comparison fixtures

The initial three fixtures still match [PR #4](https://github.com/jellli/jisu/pull/4):

| File | State | Decimal MB | Eight range fractions |
| --- | --- | ---: | --- |
| ubuntu-24.04.1-desktop-amd64.iso | Downloading | 6100 | 1, .78, .36, .90, .55, .48, .67, .30 |
| Blender-4.3.2-macos-arm64.dmg | Paused | 384 | .92, .6, .27, .51, .38, .7, .29, .41 |
| Figma-Design-Assets-2026.zip | Completed | 128 | 1, 1, 1, 1, 1, 1, 1, 1 |

Rates are `[0, 2.8, 2.3, 3.2, 2.7, 2.1, 2.5, 2.8]` MB/s (2.4 fallback for
an incomplete first range). Initial progress is 63%, 51%, 100%; Ubuntu speed
18.4 MB/s and received bytes 3.84/6.10 GB. ETA follows the slowest remaining
fixed-rate range: about 212 seconds, conservatively displayed as 约 4 分钟.

Simulation starts **off**. Footer **演示设置** enables simulation, reset and
the first-row failure scenario. Freeze/resume never catches up frozen time.
The folder action only shows `~/Downloads/<filename>`; it opens nothing.

## Checks

```sh
go test -race ./...
go vet ./...
CGO_ENABLED=0 go build ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/jisu-glass.exe .
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o /tmp/jisu-glass-macos .
```

Runtime screenshots and verification are in the PR. Cross-compilation is not
macOS/Windows runtime verification. Design restraint informed by
[unslop](https://github.com/heydamir/unslop), CC BY 4.0, and
[Emil Kowalski's skills](https://github.com/emilkowalski/skills).
