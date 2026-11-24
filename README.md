# mac-cleaner-cli

**A free, open-source Mac cleaner for the terminal.** Scan your Mac, review what is taking space,
choose item by item, then clean or cancel. Removes app and browser caches, logs, old installers,
`node_modules`, Xcode data, device backups, large files and duplicates.

Single static binary (about 6 MB). No install, no runtime, no account, no network access.

### ⬇️ [Download Mac-Cleaner (.zip)](https://github.com/mizanurhasan0/mac-cleaner-cli/raw/main/dist/Mac-Cleaner.zip)

One click, one file. Works on Apple Silicon and Intel Macs.
[All releases](https://github.com/mizanurhasan0/mac-cleaner-cli/releases)

## Quick start (one file)

1. [**Download Mac-Cleaner.zip**](https://github.com/mizanurhasan0/mac-cleaner-cli/raw/main/dist/Mac-Cleaner.zip) (direct download, always the latest version from `main`).
2. Double-click the zip to unpack **`Mac-Cleaner.command`**. Right-click it, choose **Open**, then **Open** again (first time only, the file is unsigned).
3. Pick a mode, review the list, press `a` then `c`, confirm with `y`.

Works on Apple Silicon and Intel Macs. Nothing else needs to be installed.

## Build from source

```bash
go build -o mac-cleaner-cli .
./mac-cleaner-cli              # menu
./mac-cleaner-cli quick        # or: full | deep | dev | dupes | disk
./mac-cleaner-cli full --dry-run
./mac-cleaner-cli dev --plain  # text report, read-only, no TUI
```

Flags work before or after the mode. `./package.sh` builds the one-file `dist/Mac-Cleaner.command`
(universal arm64 + Intel).

## Modes

| Mode | Looks at |
|---|---|
| full | quick + deep + dev in one pass |
| quick | `~/Library/Caches`, browser caches, `~/Library/Logs`, installers in Downloads older than 14 days |
| deep | iPhone/iPad backups, Mail downloads, Finder Trash, files over 500 MB |
| dev | `node_modules`, `target`, `.next`, `dist`, `build`, `venv`, `Pods`; npm, pnpm, Yarn, pip, Homebrew, Go, Gradle, Maven caches; Xcode DerivedData and simulators |
| dupes | identical files (size → 64 KB hash → full SHA-256) in Downloads, Documents, Desktop, Pictures, Movies, Music |
| disk | read-only folder browser with sizes; nothing is removed |

## Keys (review screen)

| Key | Action |
|---|---|
| `↑` `↓` | move |
| `space` | tick / untick |
| `a` / `A` | tick all safe / tick all |
| `n` | untick all |
| `t` | switch trash ⇄ delete |
| `enter` | open / close a group |
| `c` | clean ticked items |
| `q` / `esc` | cancel, touch nothing |

## Safety

- Scans only read. Nothing is selected until you choose.
- Each item shows **safe** or **review**, and **trash** (restorable) or **delete** (permanent).
  You can flip that per item with `t`.
- A review item set to *delete* needs you to type `delete` before it runs.
- Duplicates always keep at least one copy.
- Hard guard on every removal: only inside `$HOME`; refuses `~`, `~/Documents`, `~/Library`, `.ssh`,
  iCloud and Keychains; skips symlinks. See `internal/clean/clean.go` and its tests.
- Refuses to run as root. User-level files only, never `sudo`.
- Every action is logged to `~/.config/mac-cleaner-cli/history.log`.
- `--dry-run` goes through the whole flow and removes nothing.

## Permissions

On first run macOS may ask to let Terminal access Documents, Downloads or Desktop. Click **Allow**.
Grant Full Disk Access to Terminal for complete totals.

## Not in v0.1

App-leftover detection, Time Machine snapshots and system caches (these need admin rights),
Docker, an undo screen.

## Author

**Mizanur Hasan**
Email: [eng.mizanur.hasan@gmail.com](mailto:eng.mizanur.hasan@gmail.com)
LinkedIn: [linkedin.com/in/mizanur-hasan](https://www.linkedin.com/in/mizanur-hasan/)

Bug reports and ideas are welcome via GitHub Issues.

## License

MIT License. Copyright © 2026 Mizanur Hasan. See [LICENSE](LICENSE).

This software is provided as is, without warranty. Review each list before cleaning.
