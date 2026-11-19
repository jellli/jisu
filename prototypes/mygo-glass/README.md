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

The default is a frozen comparison. Open **演示设置** in the footer to enable
**模拟传输**, change appearance, reset samples, or inject a first-row state.
These demonstration controls stay out of the queue's main visual hierarchy.

- **暂停 / 继续:** stop or resume incomplete ranges without losing bytes.
- **重试:** resume a timed-out transfer from retained data, not from zero.
- **在文件夹中显示:** show the mock file's path inline. It never opens a real
  folder or file; the prototype has no downloaded files.
- **Select a row / details chevron:** a single inspector below the queue follows
  the selected visible task. Its eight adjacent byte ranges form one track over
  an aligned connection / received bytes / rate / state table. Completed files
  show their source and saved path instead of eight redundant 100% readings.
  Filtering moves selection to a visible task; an empty result hides the inspector.
- **Filters:** live counts for all, in progress, paused, needs attention, and
  completed. In progress includes interrupted active transfers so a failure
  does not disappear from the queue. Needs attention isolates the retry action.

All controls support keyboard focus; icon buttons have accessible names and
tooltips. The adjacent progress ranges expose their values to assistive
technology. Glass follows MyGo's Reduce Transparency and Increase Contrast
support. No perpetual animation runs while the simulation is off, and enabling
simulation never catches up time spent frozen.

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
if it is ever incomplete. Initial totals remain **63%, 18.4 MB/s**, **51%**,
**100%**. ETA intentionally corrects the original web study: fixed independent
rates mean the *slowest remaining range* determines completion, not remaining
file bytes divided by aggregate speed. The Ubuntu fixture needs about 212
seconds, displayed conservatively as **约 4 分钟**, without false precision.

## Design choices

- **Product:** a desktop download manager for monitoring parallel transfers.
- **Character:** precise, quiet, dependable.
- **World:** a file list, not a dashboard or promotional landing page.
- **Moment:** one continuous eight-range track, green only for active work.
- **Restraint:** no glass toolbar; glass is confined to labeled row actions.

The revised row follows both native-design critiques and Kite's queue/inspector
structure, not Kite's faux-macOS skin. Each task stays in a fixed 64px row with
overall progress; its detailed eight-range track lives in the shared bottom pane.
The queue and inspector scroll independently at the 720×500 minimum size. Quiet
completed rows with a file action, and failure context beside retained bytes
and retry replace oversized cards. The error demo identifies connection 03 as timed out; details
distinguish it from completed and waiting ranges. The original three fixtures
are preserved; failure is an opt-in scenario, not a fourth invented download.

Filename, percentage and rounded ETA share the same group. Source, received bytes
and per-connection data move into details. Completed rows have no bar,
repeated percentage or zero-speed display. Paused rows report no zero speed.
Use one green for active work and semantic red only for errors. Small text
uses contrast of at least 4.5:1 against the chosen light/dark surfaces.
Details and keyboard actions respond immediately; transfer statistics do not
animate decoratively. No third-party Lobe dependency is introduced: only its
useful behavior is ported using MyGo's own widgets.

The labeled row action buttons use the official `glass.Glass` material.
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
