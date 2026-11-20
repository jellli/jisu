# Browser extension takes over large and common downloads

A Chrome and Edge extension hands downloads to jisu instead of letting the browser save them. It takes over a download when the file is 5 MB or larger or has a common download type, and passes the page's cookies and referring page along so authenticated downloads work. Holding Alt while starting a download skips jisu and lets the browser handle it.

## Consequences

- The extension talks to the desktop app through native messaging, which has to be registered separately for Chrome and for Edge.
- [ADR 0009](0009-takeover-cookies.md) describes cookie lifetime and redirect rules.
