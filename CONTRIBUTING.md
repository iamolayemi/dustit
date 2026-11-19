# Contributing to DustIt

Thanks for helping! DustIt deletes files, so changes are held to a high bar for safety.

## Getting started

```bash
git clone https://github.com/iamolayemi/dustit && cd dustit
make test     # go vet + unit and end-to-end tests (e2e/ runs the real binary on a fake home)
make lint     # golangci-lint
make build    # ./dustit
make docs     # man pages + completions into generated/
```

You need Go 1.24+ and macOS: the code uses Darwin-specific file metadata.

## Project layout

| Path | What lives there |
|---|---|
| `cmd/` | Cobra commands: flags, prompts and output only |
| `internal/fsx` | Disk usage, timestamps, and **the only code that removes files** (`Remove`, `CheckDeletable`) |
| `internal/finder` | Read-only scanners: stale dependencies, duplicates, old files, apps, leftovers |
| `internal/targets` | The cleanup categories (`All()`), scanning and cleaning them |
| `internal/docker` | Docker Desktop disk image and prune steps |
| `internal/doctor` | Read-only performance checks and their safe fixes |
| `internal/history` | The removal log behind `history` and `undo` |
| `internal/config` | `~/.config/dustit/config.yaml`: loading and `config check` |
| `internal/schedule` | The launchd agents for scheduled runs and low-disk alerts |
| `internal/report` | The HTML report and its treemap |
| `internal/privacy` | The read-only privacy & security check |
| `e2e/` | End-to-end tests: the real binary against a throwaway home folder |
| `macos/DustIt` | The SwiftUI menu bar app (`make menubar`) |
| `internal/ui`, `internal/sh` | Styling, prompts, running external commands |

## Adding a cleanup category

1. Add a `Target` to `All()` in `internal/targets/targets.go`.
2. Use `Dirs` when a folder's contents can go, `Find` for anything smarter (put the finder in
   `internal/finder`), or `CleanCmd` when a tool has its own cleanup command.
3. Choose `Risk: Safe` only if the data is regenerated automatically with no user action
   beyond a reinstall. Anything personal is `Review`, and if it's a user's own file, `ToTrash: true`.
4. Add a test (an `e2e/` case if it deletes files), and run `dustit scan -v` and
   `dustit clean --only <id> -n` on your machine.
5. Changing `fsx.CheckDeletable`? Run the fuzzer for a while:
   `go test ./internal/fsx -run '^$' -fuzz FuzzCheckDeletable -fuzztime 2m`

## Safety rules (please don't break these)

- Never remove files outside `fsx.Remove`; never weaken `CheckDeletable` without discussion.
- Finders must not follow symlinks and must stay read-only.
- When unsure whether something is safe, make it `Review` or leave it out.

## Pull requests

- Keep PRs focused; describe what you tested and on which macOS version.
- `make test` and `make lint` must pass. CI runs both on macOS.
- Commit messages: short imperative summary ("Add Rust target cleanup").

By contributing you agree that your contributions are licensed under the MIT License and that you
will follow the [Code of Conduct](CODE_OF_CONDUCT.md).
