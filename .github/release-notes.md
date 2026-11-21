## Highlights

- **Download list:** Sort by column, double-click a completed download to open it, or open its folder from the right-click menu. Failed downloads show what went wrong and can be retried from the list. ([#24](https://github.com/jellli/jisu/pull/24), [#27](https://github.com/jellli/jisu/pull/27))
- **Per-download proxy:** Override the global proxy for an individual download with system settings, a direct connection, or a custom proxy. ([#25](https://github.com/jellli/jisu/pull/25))
- **Workspace:** New icons, grouped settings, clearer download details and connection counts, and queue creation from the sidebar. ([#26](https://github.com/jellli/jisu/pull/26), [#28](https://github.com/jellli/jisu/pull/28))
- **New downloads and page links:** Prefill the new-download dialog from a URL on the clipboard, and filter discovered page links by file type or select all links of the current type. Empty files now report their size correctly. ([#29](https://github.com/jellli/jisu/pull/29))

- **Already on v0.3.0 or v0.3.1?** Open **Settings (设置) > Check for Updates (检查更新)** to install v<version> from within jisu, or wait for the daily background check.
- **New to jisu or still on v0.2.0?** Download and install v<version> manually using the guide below. After that, future updates are available in the app.

## Install

Download **one** file:

| Your computer | File |
| --- | --- |
| Mac with Apple Silicon (M1 or newer) | `jisu-<version>-macos-arm64.dmg` |
| Mac with an Intel processor | `jisu-<version>-macos-x64.dmg` |
| Windows PC | `jisu-<version>-windows-x64-setup.exe` |

Not sure which Mac you have? Open the Apple menu > **About This Mac**. "Chip: Apple M…" means Apple Silicon; "Processor: Intel" means Intel.

This preview is not notarized on macOS or code-signed on Windows, so both systems warn about it the first time. Follow the steps below once.

### macOS: "…dmg is damaged and can’t be opened"

jisu is not damaged. macOS shows this message for any app downloaded from a browser that isn't notarized by Apple, and it gives no "Open Anyway" button for the disk image.

1. Download the DMG into your **Downloads** folder. If the "damaged" message appears, click **Cancel** (not Move to Trash).
2. Open **Terminal** (press Command + Space, type `Terminal`, press Return).
3. Paste this line and press Return. It removes the "downloaded from the internet" mark from the jisu disk image and prints nothing when it works:

   ```sh
   xattr -c ~/Downloads/jisu-*-macos-*.dmg
   ```

4. Double-click the DMG in Downloads, drag **jisu** onto **Applications**, then open jisu from Applications.

If you already copied jisu to Applications and it still won't open, run this line, then open jisu again:

```sh
xattr -cr /Applications/jisu.app
```

### Windows: "Windows protected your PC"

1. Run `jisu-<version>-windows-x64-setup.exe`.
2. If a blue SmartScreen window says "Windows protected your PC", click **More info**.
3. Click **Run anyway**, then follow the installer.
