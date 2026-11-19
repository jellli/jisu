# NSGo for the native UI, MyGo native as the fallback

The native UI ([ADR 0004](./0004-native-ui-following-kite-v2.md)) is built with [NSGo](https://github.com/nsgo-dev/nsgo), the Go port of longbridge's [NSH](https://github.com/longbridge/nsh). NSGo uses AppKit on macOS and draws with Direct2D on Windows, and ships a virtualized DataTable with multi-select, column resizing and reordering, sorting and context menus, which covers most of what the Job table needs. MyGo's native UI has no table, so every one of those would have to be built by hand.

NSH itself was rejected because it is Rust and jisu's backend is Go.

## Consequences

- NSGo was about a month old (v0.2.0) when chosen and its API changes often; pin a version and expect churn.
- Drag reordering of rows is not built in and must be written for Queues ([ADR 0006](./0006-reorderable-queues.md)).
- If NSGo does not run well on real macOS and Windows (table scrolling CPU cost on Windows, live per-Segment progress for dozens of Jobs), fall back to MyGo's native UI.
