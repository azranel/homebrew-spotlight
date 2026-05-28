# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `spotlight sync <worktree> --detach` (`-d`) — run the watcher in the background and return immediately, printing the PID and log path
- `spotlight sync <worktree> --once` — sync the worktree's current state once and exit without watching
- `spotlight status [--json]` — report the active sync: worktree, PID, running/stale/one-shot state, pre-sync branch, and log path
- `spotlight stop` — terminate the running watcher (or recover a crashed/one-shot session) and restore the main repo to its pre-sync branch
- Background watcher logs streamed to `~/.spotlight/<repo>-<worktree>.log`
- Restore info (pre-sync branch, stash state, worktree HEAD) persisted in the lock so `spotlight stop` can recover after an unclean shutdown

### Changed

- Lock file moved out of the repo to `~/.spotlight/locks/` (keyed by repo path); previously it lived inside the working tree and could be swept away by the `git stash -u` performed during sync, silently disabling the single-instance guard
- `--detach` and `--once` are mutually exclusive

### Fixed

- Cleanup race where closing the file watcher first let the process exit before the git restore and lock release completed, leaving the repo on a detached HEAD with the changes stashed

## [0.2.0] - 2026-04-15

### Changed

- Rewritten from TypeScript/Bun to Go for smaller binaries (~4MB vs ~22MB), faster startup, no runtime dependencies, and no macOS code signing issues
- File watching now uses fsnotify (Go native) instead of chokidar
- CLI framework changed from Commander.js to Cobra

## [0.1.1] - 2026-04-15

### Added

- CHANGELOG.md for tracking changes between versions
- README.md with install, usage, and update instructions
- Local release script (`scripts/release.sh`) for building and publishing

### Changed

- Version and description now read from package.json instead of hardcoded
- Homebrew formula uses self-contained compiled binaries (no bun runtime dependency)

## [0.1.0] - 2026-04-15

### Added

- `spotlight sync <worktree>` command — syncs worktree changes to the main repo via checkpoint commits
- `spotlight list` command — shows all git worktrees with name, branch, and path
- File watching with automatic re-sync on changes (300ms debounce)
- Full state restoration on Ctrl+C (original branch, stash, worktree cleanup)
- PID-based lock file to prevent concurrent syncs on the same repo
- Stale lock detection with user warning after unclean shutdown
- Tree SHA comparison to skip unnecessary checkouts
- Homebrew formula for installation
- Local release script for building and publishing

[Unreleased]: https://github.com/azranel/spotlight/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/azranel/spotlight/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/azranel/spotlight/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/azranel/spotlight/releases/tag/v0.1.0
