# macclean

[![CI](https://github.com/iamolayemi/macclean/actions/workflows/ci.yml/badge.svg)](https://github.com/iamolayemi/macclean/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/iamolayemi/macclean)](https://github.com/iamolayemi/macclean/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/iamolayemi/macclean)](https://goreportcard.com/report/github.com/iamolayemi/macclean)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**A careful, open-source disk cleaner for macOS, made for developers.**

macclean finds what is eating your disk (caches, logs, `node_modules` in projects you
abandoned months ago, old IDE versions, duplicate files, leftovers from deleted apps,
Docker's disk image) and removes it only when you say so.

```text
$ macclean scan
╭────────────────────────┬──────────┬───────┬────────┬───────────────────────────────────╮
│ CATEGORY               │ SIZE     │ ITEMS │ TYPE   │ WHAT                              │
├────────────────────────┼──────────┼───────┼────────┼───────────────────────────────────┤
│ user-caches            │   8.9 GB │    94 │ safe   │ App caches                        │
│ npm-cache              │  11.1 GB │     3 │ safe   │ npm cache                         │
│ stale-deps             │  16.8 GB │   101 │ safe   │ Dependencies in idle projects     │
│ jetbrains-old          │  12.1 GB │    11 │ review │ Old JetBrains IDE versions        │
│ go-modcache            │  11.1 GB │       │ review │ Go module cache                   │
│ …                      │          │       │        │                                   │
╰────────────────────────┴──────────┴───────┴────────┴───────────────────────────────────╯

  Safe to reclaim: 56.3 GB    Needs review: 33.5 GB
```

## Why macclean

- **Read-only until you ask.** `scan` never touches anything. `clean` asks before each
  category, and `--dry-run` shows exactly what would go.
- **Knows developer machines.** It understands `node_modules`, `vendor`, `Pods`, `target`,
  `.venv`, `.next`, Xcode DerivedData, Go, npm, pnpm, Yarn, Bun, Cargo, Gradle, Composer,
  Homebrew, JetBrains, Electron apps, sst/Pulumi/Terraform plugins and Docker Desktop.
- **Hard to misuse.** Every removal goes through one guard that refuses anything outside
  your home folder, top-level folders like `~/Documents`, and areas such as Keychains,
  Preferences, Mail and `~/.ssh`. Symlinks are never followed.
- **Personal files go to the Trash.** Old files, duplicates and downloads are moved to the
  Trash, never deleted outright.
- **Single binary, no daemon, no telemetry.**

## Install

**Homebrew:**

```bash
brew install iamolayemi/tap/macclean
```

**Download a release.** Grab the universal (Intel + Apple Silicon) binary from
[Releases](https://github.com/iamolayemi/macclean/releases):

```bash
curl -L https://github.com/iamolayemi/macclean/releases/latest/download/macclean_Darwin_all.tar.gz | tar xz
```
```bash
sudo mv macclean /usr/local/bin/
```

**With Go (1.24+):**

```bash
go install github.com/iamolayemi/macclean@latest
```

**From source:**

```bash
git clone https://github.com/iamolayemi/macclean && cd macclean && make install
```

## Usage

Start with a scan, then clean what you're happy to lose:

```bash
macclean scan          # what could be freed (add -v for the biggest items)
macclean clean -n      # dry run of the safe categories
macclean clean         # clean safe categories, asking before each
macclean clean --only stale-deps,old-files
```

### Commands

| Command | What it does |
|---|---|
| `scan [-v]` | Read-only report of every category, plus Docker, Time Machine snapshots and big app data |
| `clean` | Cleans the **safe** categories, asking before each. `--only a,b` picks categories; `--all` adds **review** ones; `-n` is a dry run; `-y` skips prompts; `--trash` moves instead of deleting |
| `list` | Every category and what it removes |
| `docker [--clean] [--volumes]` | What's inside Docker Desktop's disk image. `--clean` prunes containers, images, build cache and networks, then waits for the image file to shrink. Volumes are only offered with `--volumes` |
| `projects [path] [--days 30] [--clean]` | Dependency/build folders in projects untouched for N days |
| `dupes [path] [--min 1MB] [--delete]` | Duplicate files by content; you pick which copy to keep |
| `old [path] [--days 180] [--delete]` | Old items in a folder (default `~/Downloads`) |
| `large [path] [--min 500MB] [--delete]` | Largest files |
| `top [path]` | Which folders use the most space |
| `apps [--days 180] [--uninstall]` | Apps by last use and size; uninstall moves the app and its data to the Trash |
| `leftovers [--clean]` | Data folders of apps you've already deleted |
| `completion zsh\|bash\|fish` | Shell completion, including category names for `--only` |

### Categories

**Safe:** rebuilt or re-downloaded automatically. Cleaned by `macclean clean`.

| ID | Removes |
|---|---|
| `user-caches` | `~/Library/Caches` |
| `user-logs` | `~/Library/Logs`, crash reports |
| `trash` | The Trash |
| `electron-caches` | Cache folders of VS Code, Slack, Postman, Figma, Chrome… (quit them first) |
| `dot-cache` | `~/.cache` (AI model caches are kept) |
| `stale-deps` | `node_modules`, `vendor`, `Pods`, `target`, `.venv`, `.next`… in projects idle for 30+ days |
| `xcode-derived`, `xcode-device-support`, `simulator-caches`, `simulators-unavailable` | Xcode build data and simulators |
| `npm-cache`, `yarn-cache`, `pnpm-store`, `bun-cache`, `gradle-cache`, `composer-cache`, `cargo-cache`, `homebrew` | Package manager caches |
| `ios-updates`, `mail-downloads` | iOS firmware downloads, Mail attachment copies |

**Review:** may hold things you want. Only cleaned with `--only <id>` or `--all`.

| ID | Removes |
|---|---|
| `old-files` | Files ≥ 100 MB not changed in a year (to Trash) |
| `duplicates` | Extra copies of identical files ≥ 5 MB; the oldest is kept (to Trash) |
| `old-downloads` | Downloads older than 90 days (to Trash) |
| `jetbrains-old` | JetBrains IDE versions older than your newest one |
| `iac-plugins` | sst / Pulumi / Terraform provider plugins |
| `go-modcache` | Go module cache |
| `docker` | `docker system prune` |
| `xcode-archives`, `simulator-devices`, `ios-backups` | Xcode archives, simulator devices, device backups |

### How "idle project" and "old file" are decided

- A dependency folder counts only when its project file sits next to it: `node_modules` next
  to `package.json`, `vendor` next to `composer.json`, `target` next to `Cargo.toml`, and so on.
  A Go `vendor/` folder is never touched.
- A project is idle when neither git nor any source file has changed for `--days` days.
- Age is the later of modified and changed time. Access time is ignored, because backups and
  Spotlight read files constantly.
- Personal-file scans skip app-managed folders (the Music and Photos libraries, caches and app
  bundles), so cleaning can't break an app's library.

## Full Disk Access

macOS hides some folders (Trash, Mail, iOS backups) from terminal apps. To include them, add your
terminal under **System Settings → Privacy & Security → Full Disk Access**. macclean works without
it and tells you when something was skipped.

## Contributing

Bug reports, new cleanup categories and fixes are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).
If you find a way macclean could delete something it shouldn't, please report it privately as
described in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
