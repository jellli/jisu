# Native download row — Lobe MyGo

An isolated native UI comparison for Jisu. Uses Lobe MyGo components on MyGo's Go UI renderer: no HTML, JavaScript, CSS, webview, glass plugin, or production download engine.

## Run

Requires Go 1.27.4 and a graphical desktop. On Linux, a working X11 display and OpenGL/Mesa runtime are needed; install a CJK font such as `fonts-noto-cjk` for Chinese labels.

```sh
cd prototypes/native-lobe
go run .
```

In the configured Devin environment, use `"$JISU_GO" run .` instead. This directory is a standalone nested Go module, so its dependencies and entry point do not change the production app. It creates no download, configuration, or persistence files.

## Compare

- The same three fixtures as the web prototype: Ubuntu ISO (6100 MB), paused Blender DMG (384 MB), completed Figma ZIP (128 MB).
- Eight independently valued, accessible native progress tracks remain visible on every row. Connection numbers stay stable when pausing, resuming, and expanding details.
- Lobe MyGo provides buttons, segmented filters, switches, status tags, progress bars, the scenario select, typography, dividers, and theme tokens. MyGo supplies native windowing and layout. One restrained green identifies active transfer tracks; stopped and completed transfers use neutral tracks.
- Pause/resume and retry preserve partial segment progress. The completed row opens connection details rather than pretending to open a real file. Details report per-connection bytes and speed.
- The scenario select previews downloading, paused, interrupted, and completed states on the first row. Filters use live counts. Reset restores the original rows, filters, connection values, and collapsed details.
- Simulation starts off for a repeatable visual comparison. Enable **模拟传输** to advance active tasks at each connection's mock rate; disable it to freeze values. Enabling it never catches up time spent frozen. All speed and ETA values are illustrative, not measurements.
- The native window supports system light/dark appearance, keyboard navigation, and scrolling at the 880 × 600 minimum size.

No variant is selected for production and no production file is changed.

## Check

Run from this directory, not the repository root:

```sh
go test -race ./...
go vet ./...
go build -o /tmp/jisu-native-lobe .
```

Tests cover fixture independence, totals, paused/failed/completed behavior, retry continuity, completion clamping, frozen time, scenarios, filters, reset, native progress accessibility, and native button interactions through MyGo's headless tester.

## Linux/X11 rendering caveat

On the Ubuntu test VM, MyGo rendered very pale, color-fringed text with the desktop's `Xft.rgba: rgb`, using either CPU rendering or forced OpenGL. For readable screenshots, testing temporarily changed that X resource to grayscale antialiasing, restarted the app, and restored the original setting afterward:

```sh
xrdb -query | grep Xft.rgba
printf 'Xft.rgba: none\n' | xrdb -merge
# Start the prototype, then restore the original value after closing it.
```

This is an environment workaround, not a product fix. It changes the current X session's font setting, so only use it when the symptom occurs; do not make it a global desktop default. Linux/X11 native interactions were verified with this workaround. macOS, Windows, Wayland, live appearance switching, and screen-reader integration were not verified.
