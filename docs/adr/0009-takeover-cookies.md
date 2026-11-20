# Takeover request credentials

The browser scopes cookies to the final download URL before handing them to jisu. Partitioned cookies are excluded rather than mixing page partitions. The native host forwards credentials only to the authenticated loopback endpoint advertised by the running app.

Cookies live only on the in-memory Job record, not in queue.json, snapshots, UI state or errors. A resumed Job after restart has no cookies and may need a new Takeover. Referrer and User-Agent may persist. Header values are bounded and control characters are rejected. Referrers must be HTTP or HTTPS URLs without userinfo.

All probe and Segment Connections use these options. Cookies are dropped for cross-host redirects and HTTPS to HTTP downgrades, including subsequent Segment Connections to a resolved URL. HTTPS referrers are never sent to HTTP targets.
