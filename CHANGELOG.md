# Changelog

All notable changes are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/),
and this project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- DustIt menu bar app: "Open at Login" option (a standard macOS login item).

## [0.1.0] - 2026-11-19

The first release of DustIt: a careful, open-source disk cleaner and slowdown doctor for macOS,
made for developers.

### Cleaning
- `scan` and `clean` across 49 categories: app, browser and tool caches, logs, the Trash, Xcode and
  simulators, package-manager caches (npm, pnpm, Yarn, Bun, Cargo, Gradle, Maven, Composer, CocoaPods,
  Homebrew), dependency folders in idle projects, old language and SDK versions, old JetBrains IDEs,
  local AI models and AI coding-tool sessions, agent worktrees, merged git branches, big git repos,
  dev VMs, iCloud downloads, installers, screenshots, large videos, old files and duplicates.
- Safe and review categories: safe ones are rebuilt automatically; review ones are only cleaned when
  named, and personal files always go to the Trash.
- Caches of apps that are open are left alone; old-version cleanup keeps your default, the newest and
  anything a project pins.
- `ui` (interactive picker), `here` (one project's footprint), `explore` (folder browser), `similar`
  (near-duplicate photos), `docker` (prune Docker Desktop and shrink its disk image), `projects`,
  `dupes`, `old`, `large`, `top`, `apps --uninstall`, `leftovers`.
- `history` and `undo`: every removal is logged; anything moved to the Trash can be put back.

### Understanding the disk
- `storage`: disk usage in plain groups, with the app that owns each data folder.
- `report`: a self-contained HTML report with a storage bar and a map of the home folder.
- `growth` and `snapshot`: a timeline that shows what grew and what shrank.
- `attachments`: Messages and Mail attachments by year (read-only).
- Regrowth awareness: scan and clean say when a cache refills within days.

### Health and security
- `doctor`: CPU hogs, memory pressure, disk, throttling, battery, updates, Docker, Spotlight, browsers,
  startup items and services, with safe `--fix` actions and a `--watch` live view.
- `watch` and `schedule watch`: notifications for runaway processes, sudden free-space drops and
  fast-growing folders.
- `startup`: every background item with its owner and state, orphans first.
- `privacy`: a read-only check of FileVault, firewall, Gatekeeper, SIP, updates, screen lock, sharing,
  AirDrop, Find My and login settings.

### Automation and integrations
- `schedule` (weekly or daily clean-ups), `alert` and `schedule alert` (low disk space).
- `--json` output; `mcp`, a Model Context Protocol server for AI agents.
- Config file with excludes, project folders, thresholds and custom categories; `config edit` and
  `config check`.
- DustIt menu bar app with `dustit://` links for Shortcuts, and a Raycast extension.
- Man pages and shell completions; Homebrew install with `brew install iamolayemi/tap/dustit`.

### Safety
- One delete guard for every removal: nothing outside your home folder, no top-level or protected
  folders (Keychains, Preferences, Mail, Messages, `~/.ssh`), never through symlinks, and never what
  you exclude. Fuzzed and covered by end-to-end tests against a fake home folder.
- Other apps' private data is only read with Full Disk Access, so macOS shows no access warnings.

[Unreleased]: https://github.com/iamolayemi/dustit/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/iamolayemi/dustit/releases/tag/v0.1.0
