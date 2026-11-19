# Download row exploration

An isolated `/prototype` run for Jisu's download list row and multi-segment progress display. No production renderer or download engine is imported or changed. All transfers are simulated locally; this page makes no network requests.

Open `download-row.html` directly in a browser, or serve it from the repository root:

```sh
python3 -m http.server 4173 --bind 0.0.0.0 --directory prototypes
```

Then open `http://localhost:4173/download-row.html`.

## Directions

| Variant | Axis | When it wins | Cost |
| --- | --- | --- | --- |
| Rail | Compact list with eight inline segment tracks | Scanning many simultaneous downloads | Connection throughput requires expanding details |
| Mosaic | Persistent eight-connection piece map and per-connection rates | Diagnosing uneven parallel transfers | Taller rows, more information to parse |
| Focus | Outcome-first progress with progressive disclosure | Checking completion and time remaining | Segment differences are hidden until expanded |

All three share the same realistic tasks and the original renderer's gray/green tokens. Light mode is fixed to keep the comparison about information structure, not theme. The picker follows the plugin's `PICKER.md` specification without product-specific styling.

## Try it

- Click the bottom picker, press `1`–`3`, or use left/right arrows. Selection persists in `?v=1`, `?v=2`, or `?v=3`.
- Press `R` to reset the current variant. Shortcuts ignore form controls, editable content, and modifier keys.
- Pause or resume a task. Each connection progresses independently; aggregate bytes, progress, throughput, and time remaining are calculated from those connections.
- Open connection details. Completed tasks' action buttons open those same details.
- Change the first row's state to downloading, paused, interrupted, or completed. Retry recovers an interrupted transfer.
- Disable **模拟传输** to freeze the sample for comparisons. Switching variants resets sample progress while preserving the selected scenario and simulation setting.
- Resize the browser: the row layouts adapt to narrow screens, and the static desktop sidebar drops out.

The surrounding sidebar is labeled static context, not functioning app navigation. This is a visual/interaction prototype, not a replacement download engine. No entrance or progress interpolation is added to this high-frequency surface; the picker highlight and pointer press feedback are the only motion.

## Promotion

Stop after comparison and ask for a direction. Integrate only the selected row into the real renderer, connect it to the existing bridge and download snapshots, then remove this prototype surface unless explicitly asked to keep it.
