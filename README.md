# DustIt

[![CI](https://github.com/iamolayemi/dustit/actions/workflows/ci.yml/badge.svg)](https://github.com/iamolayemi/dustit/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/iamolayemi/dustit)](https://github.com/iamolayemi/dustit/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/iamolayemi/dustit)](https://goreportcard.com/report/github.com/iamolayemi/dustit)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**A careful, open-source disk cleaner and slowdown doctor for macOS, made for developers.**

DustIt finds what is eating your disk: caches, `node_modules` in projects you abandoned months
ago, old Node/Python/Rust/Java versions, old IDE versions, duplicate files, leftovers from deleted
apps and Docker's disk image. It removes them only when you say so, keeps a history, and can
undo anything it moved to the Trash. `dustit doctor` tells you why your Mac is slow.

```text
$ dustit scan
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

## Why DustIt

- **Read-only until you ask.** `scan` never touches anything; `clean` asks before each category;
  `--dry-run` shows exactly what would go.
- **Undo.** Every removal is logged. `dustit undo` puts back what a run moved to the Trash.
- **Knows developer machines.** `node_modules`, `vendor`, `Pods`, `target`, `.venv`, `.next`, Xcode,
  nvm/pyenv/rustup/asdf/mise/SDKMAN…, npm, pnpm, Yarn, Bun, Cargo, Gradle, Composer, Homebrew,
  JetBrains, Electron apps, Docker Desktop and local AI models.
- **Keeps what you use.** Old-version cleanup keeps your default version, the newest, and every
  version a project pins in `.nvmrc`, `.python-version`, `.tool-versions`, `composer.json`, …
- **Hard to misuse.** Every removal goes through one guard that refuses anything outside your home
  folder, top-level folders like `~/Documents`, Keychains, Preferences, Mail, `~/.ssh`, and anything
  you exclude in your config. Symlinks are never followed. Personal files go to the Trash.
- **Learns over time.** It tracks how your disk changes, warns about runaway processes and sudden growth,
  and tells you when a cache comes back within days of cleaning it.
- **Single binary, no telemetry.** Background checks are opt-in.

## Menu bar app

`DustIt` lives in the menu bar and looks like macOS's own menus: free space and a capacity
bar, the biggest cleanable items, anything slowing the Mac down, and "Clean Up…", "Review…",
"Storage Report" and "Undo" one click away. It's a small SwiftUI app on top of the CLI (`--json`),
so all the safety rules stay in one place. Build it from source (needs the Xcode
Command Line Tools):

```bash
make menubar && mv "dist/DustIt.app" /Applications/
```

## Use it from AI agents, Raycast and Shortcuts

- **AI agents:** `dustit mcp` is a Model Context Protocol server. Agents get `scan`, `storage`, `doctor`,
  `list_items`, `clean`, `project`, `history` and `undo` tools. Clean-ups are dry runs unless the agent
  passes `dry_run=false`, review categories need `allow_review=true`, and everything goes through the same
  guard and undo log as the CLI.
  ```bash
  claude mcp add dustit -- dustit mcp
  ```
- **Raycast:** the extension in [`integrations/raycast`](integrations/raycast) adds Disk Space, Clean Up
  Safely, Health Check and What Grew This Week.
- **Shortcuts and links:** the DustIt menu bar app handles `dustit://clean`, `dustit://report`,
  `dustit://scan`, `dustit://choose`, `dustit://doctor` and `dustit://explore`. Use them with Shortcuts'
  "Open URL" action, or call the CLI with "Run Shell Script".

## Install

**Homebrew:**

```bash
brew install iamolayemi/tap/dustit
```

**Download a release.** Grab the universal (Intel + Apple Silicon) binary from
[Releases](https://github.com/iamolayemi/dustit/releases):

```bash
curl -L https://github.com/iamolayemi/dustit/releases/latest/download/dustit_Darwin_all.tar.gz | tar xz
```
```bash
sudo mv dustit /usr/local/bin/
```

**With Go (1.24+):**

```bash
go install github.com/iamolayemi/dustit@latest
```

## Usage

```bash
dustit scan          # what could be freed (-v for the biggest items, --json for scripts)
dustit ui            # pick what to clean in a full-screen list
dustit clean -n      # dry run of the safe categories
dustit clean         # clean safe categories, asking before each
dustit clean --only stale-deps,old-versions
dustit undo          # put back what the last run moved to the Trash
dustit doctor        # why is my Mac slow?
```

### Commands

| Command | What it does |
|---|---|
| `scan [-v] [--json]` | Read-only report of every category, plus Docker, Time Machine snapshots and big app data |
| `clean` | Cleans the **safe** categories, asking before each. `--only a,b` picks categories; `--all` adds **review** ones; `-n` dry run; `-y` no prompts; `--trash` moves instead of deleting; `--notify` shows a notification |
| `ui` | Full-screen picker: tick categories, look inside them, then clean |
| `here [path] [--clean]` | Everything the current project uses: dependency and build folders, `.git`, merged branches, worktrees, and its Docker Compose containers, images and volumes |
| `explore [path]` | Full-screen folder browser sorted by size (like ncdu): drill in, reveal in Finder, move to Trash |
| `growth [--since 7d]` / `snapshot` | What grew and what shrank since a past snapshot ("Docker +12 GB, ~/Projects/x +4 GB") |
| `similar [path] [--delete]` | Similar-looking photos (resized copies, edits, bursts), not just exact duplicates; HEIC included |
| `attachments [--open]` | How much Messages and Mail attachments take, by year (read-only; points to Apple's tools to free it) |
| `storage [--apps]` | Where your space went in plain groups (Applications, Downloads, Photos, Mail, backups, app data, developer files, macOS…); `--apps` shows which app owns each big data folder |
| `report [--open]` | One self-contained HTML page: the storage breakdown, a map of your home folder, every category and the health check |
| `alert [--below 10%]` | Is free space below a threshold? Exit status 1 when it is; `--notify` for a notification |
| `startup [--disable label] [--remove-orphans]` | Every background item and launch agent, which app it belongs to and whether it runs; ones left behind by removed apps first |
| `privacy [--open id]` | Read-only check of FileVault, firewall, Gatekeeper, SIP, updates, screen lock, sharing, AirDrop, Find My, login settings and app permissions, with a link to each System Settings pane |
| `history [run-id]` / `undo [run-id]` | What was removed and when; put trashed items back |
| `doctor [--fix] [--watch] [--json]` | Why the Mac is slow: CPU hogs, memory pressure and swap, disk, throttling, battery health, pending updates, Docker memory, Spotlight, browser memory, startup agents, services. `--fix` offers the safe fixes; `--watch` is a live CPU/memory view |
| `schedule on\|off` | Clean safe categories daily or weekly with a notification (`--daily`, `--day fri`, `--at 21:30`) |
| `watch` / `schedule watch on\|off` | Every 15 minutes: processes stuck above 80% CPU for 30+ minutes, free space dropping 10 GB in an hour, folders growing 5 GB in a day, plus a daily timeline snapshot. Notifies you |
| `mcp` | MCP server so AI agents (Claude Code, Cursor…) can scan and clean through DustIt's safety guard: `claude mcp add dustit -- dustit mcp` |
| `schedule alert on\|off` | Notify when free space drops below a threshold (`--below 10%` or `20GB`, `--every 1h`); never deletes anything |
| `config [edit\|check\|init\|path]` | Your settings: excludes, project folders, thresholds, custom categories. `check` finds syntax errors and typos |
| `docker [--clean] [--volumes]` | What's inside Docker Desktop's disk image; prune it and wait for the file to shrink |
| `projects [path] [--days 30] [--clean]` | Dependency/build folders in idle projects |
| `dupes [path] [--min 1MB] [--delete]` | Duplicate files by content; you pick which copy to keep |
| `old [path] [--days 180]`, `large [path] [--min 500MB]`, `top [path]` | Old items, big files, biggest folders |
| `apps [--uninstall]`, `leftovers [--clean]` | Apps by last use; data from apps you've deleted |
| `list [--json]` | Every category and what it removes |

Man pages ship with releases (`man dustit-doctor`), and shell completion (`dustit completion zsh`)
suggests category names for `--only`.

### Categories

**Safe:** rebuilt or re-downloaded automatically. Cleaned by `dustit clean`.

| ID | Removes |
|---|---|
| `user-caches`, `user-logs`, `trash` | `~/Library/Caches`, logs and crash reports, the Trash (caches of apps that are open are left alone) |
| `electron-caches` | Cache folders of VS Code, Slack, Postman, Figma, Chrome… (quit them first) |
| `dot-cache` | `~/.cache` (AI model caches and corepack's pnpm/yarn are kept) |
| `stale-deps` | `node_modules`, `vendor`, `Pods`, `target`, `.venv`, `.next`… in projects idle for 30+ days |
| `xcode-derived`, `xcode-device-support`, `simulator-caches`, `simulators-unavailable` | Xcode build data and simulators |
| `npm-cache`, `yarn-cache`, `pnpm-store`, `bun-cache`, `gradle-cache`, `composer-cache`, `cargo-cache`, `homebrew` | Package manager caches |
| `minikube-cache`, `cocoapods-specs`, `android-cache`, `gradle-daemon-logs` | minikube images, CocoaPods spec repos, Android build cache, Gradle daemon logs |
| `editor-orphans` | VS Code, Cursor and Windsurf state for folders that no longer exist |
| `ios-updates`, `mail-downloads` | iOS firmware downloads, Mail attachment copies |

**Review:** may hold things you want. Only cleaned with `--only <id>` or `--all`.

| ID | Removes |
|---|---|
| `old-versions` | Old Node (nvm, fnm, Volta), Python (pyenv), Ruby (rbenv, rvm), Rust (rustup), Go, Terraform, Java (JDKs, SDKMAN), Flutter (fvm), Gradle, Android SDK, asdf/mise tools, unused Homebrew `python@`/`php@`/… and extra Xcode copies |
| `old-files`, `duplicates`, `old-downloads` | Big files untouched for a year, extra copies of identical files, old downloads (all to Trash) |
| `installers`, `screenshots`, `large-videos` | Old `.dmg`/`.pkg` files, screenshots and recordings older than 30 days, videos over 500 MB untouched for 90 days (all to Trash) |
| `ai-tool-data`, `agent-worktrees` | Claude Code / Codex session logs older than 60 days (settings and memory are never touched); clean, idle git worktrees made by AI agents (`git worktree remove`, branch kept) |
| `git-branches` | Local branches merged into the main branch and untouched for 90 days (`git branch -d`; long-lived branches like `develop` are kept) |
| `jetbrains-old`, `pnpm-old-stores`, `iac-plugins`, `go-modcache` | Old IDE versions, old pnpm stores, sst/Pulumi/Terraform plugins, Go module cache |
| `ai-models`, `macos-installers` | Ollama / LM Studio / Hugging Face models, "Install macOS" apps |
| `git-gc` | Repos whose `.git` is over 500 MB: `git worktree prune` + `git gc` (history and reflog are kept) |
| `dev-vms` | minikube, kind, k3d, Colima and Lima clusters/VMs (deleting one removes what's inside) |
| `icloud-evict` | iCloud Drive files ≥ 10 MB unchanged for 90 days: the local copy goes, the file stays in iCloud |
| `maven-repo` | `~/.m2/repository` |
| `docker`, `xcode-archives`, `simulator-devices`, `ios-backups` | Docker prune, Xcode archives, simulator devices, device backups |

### How dustit decides

- **Project folders** are `~/Projects`, `~/Developer`, `~/code`, `~/src`, `~/dev`, `~/workspace`,
  `~/Sites`, `~/Documents` and `~/Desktop`, plus any `project_roots` in your config.
- **A dependency folder** counts only when its project file sits next to it: `node_modules` next to
  `package.json`, `vendor` next to `composer.json`, `target` next to `Cargo.toml`, … (Go's `vendor/`
  is never touched). A project is **idle** when neither git nor any source file changed for 30 days.
- **Old versions:** for each tool, DustIt keeps your default/current version, the newest one, and
  any version pinned by a project (`.nvmrc`, `.node-version`, `.python-version`, `.ruby-version`,
  `.tool-versions`, `mise.toml`, `rust-toolchain`, `.go-version`, `.terraform-version`, `.java-version`,
  `.sdkmanrc`, `.fvmrc`, `gradle-wrapper.properties`, `composer.json`'s `php`, Volta's `package.json`).
  Homebrew versioned formulae are only suggested when a newer one is installed, nothing depends on
  them, and they aren't running as a service.
- **Age** is the later of modified and changed time. Access time is ignored: backups and Spotlight
  read files constantly.
- **Personal-file scans** skip app-managed folders (Music and Photos libraries, caches, app bundles).

## Configuration

`dustit config edit` opens `~/.config/dustit/config.yaml` (created from a commented example) and
checks it when you save. `dustit config check` reports syntax errors, misspelled keys and bad
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
$ dustit doctor
✗ CPU load: 22.8 tasks waiting on 8 cores (5-min average) — 3× more work than the CPU can run
      177%  WhatsApp                     open for 10h 56m
       76%  mysqld                       open for 2d 17h
    → mysqld is using 76% CPU and has been running 2d 17h. A service this busy is often stuck
      on a long query or leftover job. Restart it: brew services restart mysql@9.7

✗ Battery: check battery condition, 69% of original capacity, 807 charge cycles
! Memory: 17.2 GB RAM, 13.7 GB swap in use — memory pressure is elevated
✓ Temperature: no throttling
```

DustIt doesn't "clean RAM" or kill apps behind your back. macOS manages memory itself, and purging
it only makes apps slower while they reload. `doctor` tells you which process is actually responsible
and what to do about it.

## History and undo

Every removal is recorded in `~/Library/Application Support/dustit/history.jsonl`.

```bash
dustit history               # recent runs: what, how much, trashed or deleted
dustit history <run-id>      # every item of one run
dustit undo                  # put back what the last run moved to the Trash
```

Caches and dependency folders are deleted permanently (their tools rebuild them); personal files go
to the Trash and can be undone until you empty it.

## Full Disk Access

macOS hides some folders (Trash, Mail, iOS backups, other apps' private data) from terminal apps.
Without Full Disk Access DustIt never reads inside other apps' containers, so macOS won't show
"tried to access data from other apps" warnings; it tells you what it skipped instead. To include
those folders, add your terminal (or DustIt) under **System Settings → Privacy & Security →
Full Disk Access**.

## Contributing

Bug reports, new cleanup categories and fixes are welcome; see [CONTRIBUTING.md](CONTRIBUTING.md).
`make docs` builds the man pages and completions; `make demo` records a demo GIF with
[vhs](https://github.com/charmbracelet/vhs). If you find a way dustit could delete something it
shouldn't, please report it privately as described in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
