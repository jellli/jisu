# Dynamic segmentation with 8 Connections by default

The engine splits a Job into Segments dynamically, the way IDM does: when a Connection finishes its range, the largest remaining Segment is split and the free Connection takes the second half. A Job starts with 8 Connections, and the user can change the count while it runs.

This replaces the original fixed split into equal parts, each written to its own part file. Dynamic splitting is what keeps every Connection busy until the end of the file, which is the main reason IDM-style managers are fast.

## Consequences

- The on-disk resume format changes, because Segment boundaries are no longer fixed. Unfinished Jobs saved in the old fixed-part format must still resume after an upgrade.
