# Jisu — MyGo native UI + Glass

An isolated download-row comparison built entirely with MyGo's native `ui`
toolkit and its official `plugins/glass` package. No HTML, CSS, JavaScript,
webview, third-party component library, or real downloads. Production UI is
untouched. The nested Go module can coexist with the production app and the
other prototype branches.

## Run

Requires [Go 1.27.3+](https://go.dev/dl/). From the repository root:

```sh
cd prototypes/mygo-glass
go run .
# Optional: begin in the other appearance
go run . -dark
```

MyGo is pinned to the stable `v0.10.0` release. The desktop binary builds with
`CGO_ENABLED=0`. On Linux, MyGo needs the system GTK/WebKitGTK 4.1 runtime even
though this window uses no webview, and an OpenGL-capable display. On Ubuntu:

```sh
sudo apt-get install libwebkit2gtk-4.1-0 fonts-noto-cjk
```

The default is a frozen comparison. Check **模拟传输** to advance the mock
transfers; pause/resume any incomplete task, expand **连接详情** to see all
eight connection rates, reset to the original fixtures, or switch appearance.
All controls support keyboard focus; icon buttons have accessible names and
tooltips. The progress bars expose their connection values to assistive
technology. Glass follows MyGo's Reduce Transparency and Increase Contrast
support. No perpetual animation runs while the simulation is off.

## Comparison data

Fixtures are copied from [PR #4](https://github.com/jellli/jisu/pull/4),
`prototypes/download-row.html` at `f4cd316eafa505303760da1d948b4f8041b8c2a4`:

| File | State | MB | Connection fractions |
| --- | --- | ---: | --- |
| ubuntu-24.04.1-desktop-amd64.iso | Downloading | 6100 | 1, .78, .36, .90, .55, .48, .67, .30 |
| Blender-4.3.2-macos-arm64.dmg | Paused | 384 | .92, .6, .27, .51, .38, .7, .29, .41 |
| Figma-Design-Assets-2026.zip | Completed | 128 | 1, 1, 1, 1, 1, 1, 1, 1 |

Rates are `[0, 2.8, 2.3, 3.2, 2.7, 2.1, 2.5, 2.8]` MB/s, zero for inactive
or complete connections; the same 2.4 MB/s fallback applies to connection 1
if it is ever incomplete. Sizes, percentages, and ETA use the same decimal
units and rounding. Initial totals are **63%, 18.4 MB/s**, **51%**, **100%**.

## Design choices

- **Product:** a desktop download manager for monitoring parallel transfers.
- **Character:** precise, quiet, dependable.
- **World:** a file list, not a dashboard or promotional landing page.
- **Moment:** eight numbered byte-range tracks, green only for active work.
- **Restraint:** a single glass toolbar; opaque transfer rows keep contrast.

The earlier web directions were already restrained after their unslop pass.
This is a native material/layout comparison, not a new visual brand: preserve
their line icons, calm hierarchy, metadata, and numbered lanes. Avoid nested
cards, ornamental extension tiles, giant percentage counters, all-over glass,
and animations on transfer statistics. Row actions stay visible. Details
expand immediately rather than making a frequent utility action wait.

The toolbar and round action buttons use the official `glass.Glass` material.
This is MyGo's cross-platform shader, **not** AppKit's `NSGlassEffectView`:
`Native: true` is specifically for glass over a webview on macOS, which this
prototype does not have. Windows, Linux, and macOS use the same material code.

Design method: [unslop](https://github.com/heydamir/unslop) by Damir Sokolovsky
([@heydamir](https://x.com/heydamir)), CC BY 4.0; interaction restraint informed
by [Emil Kowalski's design-engineering skills](https://github.com/emilkowalski/skills).

## Checks

```sh
go test -race ./...
go vet ./...
CGO_ENABLED=0 go build ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/jisu-glass.exe .
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o /tmp/jisu-glass-macos .
```

Screenshots and runtime verification are attached to the PR. Cross-compiling
is not runtime verification on macOS or Windows.
