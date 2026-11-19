# macclean

[![CI](https://github.com/iamolayemi/macclean/actions/workflows/ci.yml/badge.svg)](https://github.com/iamolayemi/macclean/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/iamolayemi/macclean)](https://github.com/iamolayemi/macclean/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/iamolayemi/macclean)](https://goreportcard.com/report/github.com/iamolayemi/macclean)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**A careful, open-source disk cleaner and slowdown doctor for macOS, made for developers.**

macclean finds what is eating your disk: caches, `node_modules` in projects you abandoned months
ago, old Node/Python/Rust/Java versions, old IDE versions, duplicate files, leftovers from deleted
apps and Docker's disk image. It removes them only when you say so, keeps a history, and can
undo anything it moved to the Trash. `macclean doctor` tells you why your Mac is slow.

```text
$ macclean scan
╭────────────────────────┬──────────┬───────┬────────┬───────────────────────────────────╮
│ CATEGORY               │ SIZE     │ ITEMS │ TYPE   │ WHAT                              │
├────────────────────────┼──────────┼───────┼────────┼───────────────────────────────────┤
│ user-caches            │   8.9 GB │    94 │ safe   │ App caches                        │
│ npm-cache              │  11.1 GB │     3 │ safe   │ npm cache                         │
│ stale-deps             │  16.8 GB │   101 │ safe   │ Dependencies in idle projects     │
│ jetbrains-old          │  12.1 GB │    11 │ review │ Old JetBrains IDE versions        │
│ old-versions           │   2.4 GB │     6 │ review │ Old language & SDK versions       │
│ …                      │          │       │        │                                   │
╰────────────────────────┴──────────┴───────┴────────┴───────────────────────────────────╯

  Safe to reclaim: 56.3 GB    Needs review: 33.5 GB
```

## Why macclean

- **Read-only until you ask.** `scan` never touches anything; `clean` asks before each category;
  `--dry-run` shows exactly what would go.
- **Undo.** Every removal is logged. `macclean undo` puts back what a run moved to the Trash.
- **Knows developer machines.** `node_modules`, `vendor`, `Pods`, `target`, `.venv`, `.next`, Xcode,
  nvm/pyenv/rustup/asdf/mise/SDKMAN…, npm, pnpm, Yarn, Bun, Cargo, Gradle, Composer, Homebrew,
  JetBrains, Electron apps, Docker Desktop and local AI models.
- **Keeps what you use.** Old-version cleanup keeps your default version, the newest, and every
  version a project pins in `.nvmrc`, `.python-version`, `.tool-versions`, `composer.json`, …
- **Hard to misuse.** Every removal goes through one guard that refuses anything outside your home
  folder, top-level folders like `~/Documents`, Keychains, Preferences, Mail, `~/.ssh`, and anything
  you exclude in your config. Symlinks are never followed. Personal files go to the Trash.
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

## Usage

```bash
macclean scan          # what could be freed (-v for the biggest items, --json for scripts)
macclean ui            # pick what to clean in a full-screen list
macclean clean -n      # dry run of the safe categories
macclean clean         # clean safe categories, asking before each
macclean clean --only stale-deps,old-versions
macclean undo          # put back what the last run moved to the Trash
macclean doctor        # why is my Mac slow?
```

### Commands

| Command | What it does |
|---|---|
| `scan [-v] [--json]` | Read-only report of every category, plus Docker, Time Machine snapshots and big app data |
| `clean` | Cleans the **safe** categories, asking before each. `--only a,b` picks categories; `--all` adds **review** ones; `-n` dry run; `-y` no prompts; `--trash` moves instead of deleting; `--notify` shows a notification |
| `ui` | Full-screen picker: tick categories, look inside them, then clean |
| `history [run-id]` / `undo [run-id]` | What was removed and when; put trashed items back |
| `doctor [--fix] [--watch] [--json]` | Why the Mac is slow: CPU hogs, memory pressure and swap, disk, throttling, battery health, pending updates, Docker memory, startup agents, services. `--fix` offers the safe fixes; `--watch` is a live CPU/memory view |
| `schedule on\|off` | Clean safe categories daily or weekly with a notification (`--daily`, `--day fri`, `--at 21:30`) |
| `config [edit\|check\|init\|path]` | Your settings: excludes, project folders, thresholds, custom categories. `check` finds syntax errors and typos |
| `docker [--clean] [--volumes]` | What's inside Docker Desktop's disk image; prune it and wait for the file to shrink |
| `projects [path] [--days 30] [--clean]` | Dependency/build folders in idle projects |
| `dupes [path] [--min 1MB] [--delete]` | Duplicate files by content; you pick which copy to keep |
| `old [path] [--days 180]`, `large [path] [--min 500MB]`, `top [path]` | Old items, big files, biggest folders |
| `apps [--uninstall]`, `leftovers [--clean]` | Apps by last use; data from apps you've deleted |
| `list [--json]` | Every category and what it removes |

Man pages ship with releases (`man macclean-doctor`), and shell completion (`macclean completion zsh`)
suggests category names for `--only`.

### Categories

**Safe:** rebuilt or re-downloaded automatically. Cleaned by `macclean clean`.

| ID | Removes |
|---|---|
| `user-caches`, `user-logs`, `trash` | `~/Library/Caches`, logs and crash reports, the Trash |
| `electron-caches` | Cache folders of VS Code, Slack, Postman, Figma, Chrome… (quit them first) |
| `dot-cache` | `~/.cache` (AI model caches and corepack's pnpm/yarn are kept) |
| `stale-deps` | `node_modules`, `vendor`, `Pods`, `target`, `.venv`, `.next`… in projects idle for 30+ days |
| `xcode-derived`, `xcode-device-support`, `simulator-caches`, `simulators-unavailable` | Xcode build data and simulators |
| `npm-cache`, `yarn-cache`, `pnpm-store`, `bun-cache`, `gradle-cache`, `composer-cache`, `cargo-cache`, `homebrew` | Package manager caches |
| `ios-updates`, `mail-downloads` | iOS firmware downloads, Mail attachment copies |

**Review:** may hold things you want. Only cleaned with `--only <id>` or `--all`.

| ID | Removes |
|---|---|
| `old-versions` | Old Node (nvm, fnm, Volta), Python (pyenv), Ruby (rbenv, rvm), Rust (rustup), Go, Terraform, Java (JDKs, SDKMAN), Flutter (fvm), Gradle, Android SDK, asdf/mise tools, unused Homebrew `python@`/`php@`/… and extra Xcode copies |
| `old-files`, `duplicates`, `old-downloads` | Big files untouched for a year, extra copies of identical files, old downloads (all to Trash) |
| `jetbrains-old`, `pnpm-old-stores`, `iac-plugins`, `go-modcache` | Old IDE versions, old pnpm stores, sst/Pulumi/Terraform plugins, Go module cache |
| `ai-models`, `macos-installers` | Ollama / LM Studio / Hugging Face models, "Install macOS" apps |
| `docker`, `xcode-archives`, `simulator-devices`, `ios-backups` | Docker prune, Xcode archives, simulator devices, device backups |

### How macclean decides

- **Project folders** are `~/Projects`, `~/Developer`, `~/code`, `~/src`, `~/dev`, `~/workspace`,
  `~/Sites`, `~/Documents` and `~/Desktop`, plus any `project_roots` in your config.
- **A dependency folder** counts only when its project file sits next to it: `node_modules` next to
  `package.json`, `vendor` next to `composer.json`, `target` next to `Cargo.toml`, … (Go's `vendor/`
  is never touched). A project is **idle** when neither git nor any source file changed for 30 days.
- **Old versions:** for each tool, macclean keeps your default/current version, the newest one, and
  any version pinned by a project (`.nvmrc`, `.node-version`, `.python-version`, `.ruby-version`,
  `.tool-versions`, `mise.toml`, `rust-toolchain`, `.go-version`, `.terraform-version`, `.java-version`,
  `.sdkmanrc`, `.fvmrc`, `gradle-wrapper.properties`, `composer.json`'s `php`, Volta's `package.json`).
  Homebrew versioned formulae are only suggested when a newer one is installed, nothing depends on
  them, and they aren't running as a service.
- **Age** is the later of modified and changed time. Access time is ignored: backups and Spotlight
  read files constantly.
- **Personal-file scans** skip app-managed folders (Music and Photos libraries, caches, app bundles).

## Configuration

`macclean config edit` opens `~/.config/macclean/config.yaml` (created from a commented example) and
checks it when you save. `macclean config check` reports syntax errors, misspelled keys and bad
values with line numbers.

```yaml
exclude:                      # never removed, by any command
  - ~/Projects/client-x/node_modules
  - ~/Downloads/*.dmg
project_roots:                # searched in addition to the defaults
  - ~/work
stale_days: 30
old_file_days: 365
old_file_min: 100MB
duplicate_min: 5MB
download_days: 90
disabled: [electron-caches]   # categories to turn off
categories:                   # your own: the contents of each path are removed
  - id: unity-cache
    name: Unity cache
    paths: [~/Library/Unity/cache]
    risk: safe                # safe | review
```

## Why is my Mac slow?

```text
$ macclean doctor
✗ CPU load: 22.8 tasks waiting on 8 cores (5-min average) — 3× more work than the CPU can run
      177%  WhatsApp                     open for 10h 56m
       76%  mysqld                       open for 2d 17h
    → mysqld is using 76% CPU and has been running 2d 17h. A service this busy is often stuck
      on a long query or leftover job. Restart it: brew services restart mysql@9.7

✗ Battery: check battery condition, 69% of original capacity, 807 charge cycles
! Memory: 17.2 GB RAM, 13.7 GB swap in use — memory pressure is elevated
✓ Temperature: no throttling
```

macclean doesn't "clean RAM" or kill apps behind your back. macOS manages memory itself, and purging
it only makes apps slower while they reload. `doctor` tells you which process is actually responsible
and what to do about it.

## History and undo

Every removal is recorded in `~/Library/Application Support/macclean/history.jsonl`.

```bash
macclean history               # recent runs: what, how much, trashed or deleted
macclean history <run-id>      # every item of one run
macclean undo                  # put back what the last run moved to the Trash
```

Caches and dependency folders are deleted permanently (their tools rebuild them); personal files go
to the Trash and can be undone until you empty it.

## Full Disk Access

macOS hides some folders (Trash, Mail, iOS backups) from terminal apps. To include them, add your
terminal under **System Settings → Privacy & Security → Full Disk Access**. macclean works without it
and tells you when something was skipped.

## Contributing

Bug reports, new cleanup categories and fixes are welcome; see [CONTRIBUTING.md](CONTRIBUTING.md).
`make docs` builds the man pages and completions; `make demo` records a demo GIF with
[vhs](https://github.com/charmbracelet/vhs). If you find a way macclean could delete something it
shouldn't, please report it privately as described in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
