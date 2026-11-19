# Changelog

All notable changes are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/),
and this project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- `doctor`: diagnose a slow Mac (CPU hogs, memory pressure and swap, disk, throttling, startup agents, services) with safe `--fix` actions.

## [0.1.0]

### Added
- `scan` and `clean` with 30 categories across caches, logs, developer tools, personal files and Docker.
- `stale-deps`: dependency/build folders in projects idle for 30+ days.
- `old-files` and `duplicates`: personal-file categories, moved to Trash.
- `docker`: inspect and prune Docker Desktop's disk image and wait for it to shrink.
- `projects`, `dupes`, `old`, `large`, `top`, `apps`, `leftovers` finder commands.
- Shell completion, including category names for `clean --only`.
- Homebrew install: `brew install iamolayemi/tap/macclean`.
