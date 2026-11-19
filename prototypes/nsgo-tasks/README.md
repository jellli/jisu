# jisu: NSGo task table prototype

From this directory with Go 1.27.3 or newer:

```sh
go run .
```

The demo starts a loopback file server and queues eight sample downloads in a temporary directory. It removes the temporary data at normal exit. It does not use jisu's production queue or renderer.

- Use Shift-click for a range, or Command-click on macOS / Ctrl-click on Windows or Linux to toggle a row.
- Drag a column title to reorder it, or drag its trailing edge to resize it.
- Right-click a row for Pause, Resume, Remove, Move up, or Move down. Right-clicking a selected row keeps the selection. Moving tasks changes their actual queue order without interrupting downloads already in progress. NSGo v0.2.0's DataTable does not expose row drag reordering, so the context menu is the substitute.
- Each progress cell divides the file into 16 fixed byte bands. Each band fills according to live writes from jisu's dynamic range workers, not in simple left-to-right order and not one band per connection. The Links column shows the number of workers currently active.

The Kite v2 HTML attachment returned HTTP 401 when this prototype was developed. This table follows the task description and ADRs 0004 and 0008; it is not a verified visual match to the attachment.

## Verification

```sh
go tool nsgo check
go tool nsgo test
go tool nsgo test -target macos   # use windows or linux on that host
```

The native test starts the same table over real HTTP transfers, waits for partial progress, and tests Shift-click and queue reordering through the context menu. The demo workflow runs it on macOS and Windows and uploads the live and reordered windows. Screenshots are written under `.nsgo/test/screenshots/`. This small workload is a functionality check, not a Windows scrolling CPU benchmark or a comparison against MyGo.
