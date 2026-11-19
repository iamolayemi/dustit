# Changelog

All notable changes are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/),
and this project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- `doctor`: diagnose a slow Mac (CPU hogs, memory pressure and swap, disk, throttling, battery health,
  pending updates, Docker memory, startup agents, services) with safe `--fix` actions and `--watch`.
- `history` and `undo`: every removal is logged; items moved to the Trash can be put back.
- `old-versions`: old Node, Python, Ruby, Rust, Go, Terraform, Java, Flutter, Gradle, Android SDK,
  asdf/mise tools, unused Homebrew versioned formulae and extra Xcode copies. Keeps the default, the
  newest and anything a project pins.
- `ai-models`, `macos-installers` and `pnpm-old-stores` categories.
- Config file (`~/.config/dustit/config.yaml`) with excludes, project folders, thresholds, disabled
  and custom categories; `config edit`, `config check`, `config init`, `config path`.
- `schedule`: clean safe categories daily or weekly with a notification.
- `ui`: interactive full-screen picker.
- `--json` for `scan`, `doctor`, `list` and `history`.
- Man pages and shell completions in releases and the Homebrew cask.
- `report`: self-contained HTML report with a treemap of the home folder, categories and doctor findings.
- `alert` and `schedule alert`: notification when free space runs low.
- `git-gc`, `dev-vms`, `icloud-evict`, `maven-repo`, `minikube-cache`, `cocoapods-specs`, `android-cache`
  and `gradle-daemon-logs` categories.
- `doctor` checks Spotlight indexing and browser memory.
- DustIt: a menu bar app (`make menubar`).
- End-to-end tests against a fake home folder and fuzzing of the delete guard in CI.
- `storage`: disk usage in plain groups with app owners; the report leads with it.
- `startup`: launch agents and daemons with owners and state, orphans first; disable, enable, remove orphans.
- `privacy`: read-only security and privacy check with links to System Settings.
- `installers`, `screenshots`, `large-videos`, `editor-orphans`, `ai-tool-data`, `agent-worktrees`
  and `git-branches` categories; `duplicate_folders` setting.
- Caches of apps that are open are left alone.

- `here`: everything one project uses (dependencies, build output, git, Docker Compose), with cleanup.
- `explore`: full-screen folder browser.
- `growth` and `snapshot`: a disk timeline showing what grew; `storage` and `report` record snapshots too.
- `watch` and `schedule watch`: alerts for runaway processes, sudden free-space drops and fast-growing folders.
- Regrowth awareness: scan and clean say when a cache refills within days.
- `similar`: near-duplicate photos by perceptual hash (JPEG, PNG, GIF, HEIC, TIFF, WebP).
- `attachments`: read-only report of Messages and Mail attachments by year.
- `mcp`: Model Context Protocol server for AI agents.
- Raycast extension, `dustit://` links in the menu bar app, and an app icon.

### Changed
- DustIt redesigned to look and behave like a native macOS menu; its actions no longer need
  the Automation permission.
- The HTML report has a storage bar, a calmer map and the exact command for every category.

### Fixed
- Other apps' private data is only read with Full Disk Access, so macOS no longer shows
  "tried to access data from other apps" warnings.

### Fixed
- `dot-cache` no longer removes corepack's pnpm/yarn binaries.
- Categories whose tool command fails show "failed" instead of 0 B.
- `dupes` keeps the copy that was created first.

## [0.1.0]

### Added
- `scan` and `clean` with 30 categories across caches, logs, developer tools, personal files and Docker.
- `stale-deps`: dependency/build folders in projects idle for 30+ days.
- `old-files` and `duplicates`: personal-file categories, moved to Trash.
- `docker`: inspect and prune Docker Desktop's disk image and wait for it to shrink.
- `projects`, `dupes`, `old`, `large`, `top`, `apps`, `leftovers` finder commands.
- Shell completion, including category names for `clean --only`.
- Homebrew install: `brew install iamolayemi/tap/dustit`.
