# Native UI following the kite v2 design

jisu's interface is native, not a webview page, and follows the kite v2 design: no glass or translucency effects, and the same sidebar Categories as v2. Native was chosen over reusing the kite v2 web page because it is what a desktop download manager is expected to feel like, at the cost of building the Job table (multi-select, context menu, drag reordering, aligned columns) and live per-Segment progress ourselves.

The toolkit choice is [ADR 0008](./0008-nsgo-for-the-native-ui.md).
