# jisu

jisu is a desktop download manager for Windows and macOS that downloads HTTP and HTTPS files over parallel Connections.

## Language

**Job**:
One file the user asked jisu to download, from its URL to the finished file on disk.
_Avoid_: Task, download item

**Queue**:
An ordered list of Jobs that jisu works through; the user can have several and reorder Jobs within one.
_Avoid_: List, batch

**Category**:
A sidebar grouping of Jobs by file kind or state, as in the kite v2 design.
_Avoid_: Folder, filter

**Segment**:
A contiguous byte range of a Job's file that one Connection downloads; Segments are split dynamically while the Job runs.
_Avoid_: Part, chunk, block

**Connection**:
One HTTP request stream downloading a Segment; a Job runs several at once.
_Avoid_: Thread, worker

**Takeover**:
The browser extension handing a browser download to jisu instead of letting the browser save it.
_Avoid_: Capture, interception

**System proxy**:
A proxy configured in the operating system's network settings, shared by applications that use those settings.

**Job proxy override**:
A saved proxy choice for one Job: System, direct, or Custom. Without an override, the Job follows the global proxy setting.
