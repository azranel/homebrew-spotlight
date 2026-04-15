# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/azranel/spotlight/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/azranel/spotlight/releases/tag/v0.1.0
