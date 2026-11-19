## What & why

## How I tested
- [ ] `make test` passes
- [ ] Ran `dustit scan -v` / `clean --only <id> -n` on macOS ___

## Safety
- [ ] All removals go through `fsx.Remove`
- [ ] New categories holding user data are `Review` (and `ToTrash` for personal files)
