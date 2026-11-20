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
