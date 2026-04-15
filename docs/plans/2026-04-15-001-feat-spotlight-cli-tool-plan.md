---
title: "feat: Build Spotlight CLI tool for git worktree-to-main-repo syncing"
type: feat
status: active
date: 2026-04-15
deepened: 2026-04-15
---

# feat: Build Spotlight CLI tool for git worktree-to-main-repo syncing

## Overview

Build a TypeScript CLI tool called `spotlight` that replicates Conductor's experimental Spotlight feature — syncing changes from a git worktree into the main repository as checkpoint commits. The tool runs inside the main git repository directory and provides two commands: `sync` (watch and sync a worktree) and `list` (show available worktrees). Distributable via Homebrew as a single compiled binary using Bun.

## Problem Frame

Conductor's Spotlight feature is broken in the newest version. The user needs a standalone CLI replacement that:
- Takes changes made in a git worktree and mirrors them into the main repo as checkpoints
- Watches for ongoing changes and continues syncing
- Restores the main repo to its original state on CTRL+C
- Prevents multiple concurrent sync operations on the same repo

## Requirements Trace

- R1. `spotlight sync <WORKTREE_NAME>` syncs worktree changes to main repo via checkpoint commits
- R2. After initial sync, the tool continues watching for changes and re-syncing automatically
- R3. The tool blocks the terminal; CTRL+C stops syncing and restores the main repo to its pre-sync state
- R4. `spotlight list` shows all git worktrees and their names
- R5. Only one sync operation per repo at any time (enforced via lock file)
- R6. Installable via Homebrew
- R7. Uses TypeScript with oxlint and oxfmt for code quality

## Scope Boundaries

- One-way sync only: worktree -> main repo (not bidirectional)
- Only git-tracked files are synced (respects .gitignore automatically via git commits)
- No GUI, no web UI — CLI only
- No support for syncing multiple worktrees simultaneously to the same repo

### Deferred to Separate Tasks

- Publishing to npm registry
- CI/CD pipeline setup
- Automated release workflow

## Context & Research

### Relevant Patterns

- Conductor Spotlight creates checkpoint commits in the workspace, then checks them out in the main repo directory
- Git worktrees share the same `.git` object store and refs — commits made in a worktree are immediately accessible from the main repo
- `git worktree list --porcelain` provides machine-parseable worktree information

### External References

- Chokidar v4/v5 for reliable cross-platform file watching (used by Vite, Webpack)
- `bun build --compile` for single native binary output (no runtime dependency)
- Oxlint 1.0+ (production-ready, 650+ rules, 50-100x faster than ESLint)
- Oxfmt (alpha but functional, 30x faster than Prettier)

## Key Technical Decisions

- **Bun runtime**: Fast TypeScript execution, built-in bundler, `bun build --compile` produces standalone binaries for Homebrew distribution
- **Checkpoint commits for sync**: Create a temporary commit in the worktree (staging all changes), then `git checkout` that commit in the main repo. This is git-native, atomic, and automatically respects .gitignore — no need for manual file copying or gitignore parsing. On subsequent changes, always `git commit --amend` the existing checkpoint so there is exactly one checkpoint commit on top of the worktree's original HEAD. Record the worktree's original HEAD SHA at startup for reliable cleanup
- **Chokidar for file watching**: Node's native `fs.watch` is unreliable across platforms. Chokidar is battle-tested and handles macOS FSEvents properly
- **PID-based lock file**: Write `.spotlight-sync.lock` in the repo root containing the PID. Check on startup, clean up on exit (including SIGINT/SIGTERM). Simple and sufficient for single-process enforcement
- **Commander.js for CLI parsing**: Well-established, good TypeScript support, minimal overhead
- **State restoration via git ref**: Record the main repo's current HEAD ref and the worktree's HEAD SHA before syncing. On cleanup, force-checkout (`git checkout --force`) the original ref in the main repo (force required because the working tree may have become dirty during sync from editors/LSPs), pop stash, and `git reset --soft <recorded-worktree-sha>` in the worktree to undo the checkpoint commit

## Open Questions

### Resolved During Planning

- **How to handle uncommitted changes in the main repo before sync?**: Stash them automatically using `git stash -u` (include untracked files) before sync, pop on restore. If stash pop fails on restore (conflicts), print a warning with recovery instructions (`git stash pop` manually) but continue cleanup (release lock, restore ref) — do not abort cleanup mid-way
- **What if the worktree has uncommitted changes?**: That's the point — we stage and commit everything in the worktree as a checkpoint, then sync that commit to main
- **How to debounce rapid file changes?**: Use a 300ms debounce window in chokidar before triggering a sync cycle

- **How to handle "nothing to commit" on initial sync?**: If the worktree has no uncommitted changes, skip checkpoint creation and checkout the worktree's current HEAD directly in the main repo. The watcher still starts — future changes will create checkpoints as normal
- **How to handle stale lock files after crash?**: When a stale lock is detected (dead PID), warn the user that the previous sync may have left the repo in a dirty state, then offer to clean up the lock and proceed. Do not silently proceed — the user needs to know their repo may be on a detached HEAD
- **How to avoid unnecessary re-syncs?**: Before checking out in the main repo, compare the new checkpoint's tree SHA with the currently checked-out tree SHA. Skip checkout if they match (handles cases where amend produces a new commit SHA but identical tree)

### Deferred to Implementation

- Exact chokidar ignore patterns beyond `.git` — will depend on testing with real worktrees

## High-Level Technical Design

> *This illustrates the intended approach and is directional guidance for review, not implementation specification. The implementing agent should treat it as context, not code to reproduce.*

```
Sync Lifecycle:

1. STARTUP
   - Verify CWD is a git repo root
   - Verify worktree name exists (via `git worktree list`)
   - Acquire lock file (.spotlight-sync.lock with PID)
     - If stale lock found: warn user about potentially dirty repo state, ask to proceed
   - Record main repo's current HEAD ref (branch name or commit SHA)
   - Record worktree's current HEAD SHA (the "original worktree head")
   - Stash uncommitted changes in main repo (`git stash -u` to include untracked files)

2. INITIAL SYNC
   - In worktree: check if there are uncommitted changes
     - If yes: `git add -A && git commit -m "spotlight checkpoint"`
     - If no: use worktree's current HEAD as the sync target
   - In main repo: checkout the target commit SHA (detached HEAD)
   - Report sync complete

3. WATCH LOOP
   - Watch worktree directory with chokidar (ignore .git, node_modules/)
     - Note: .git in a worktree is a FILE, not a directory — use pattern that matches both
   - On change (debounced 300ms):
     - In worktree: `git add -A && git commit --amend --no-edit`
       (always amend — keeps exactly one checkpoint commit on top of original HEAD)
     - Compare new commit's tree SHA with currently checked-out tree SHA
     - If trees differ: checkout updated checkpoint in main repo
     - If trees match: skip checkout (no effective change)
   - Print status on each sync

4. CLEANUP (SIGINT/SIGTERM/exit — must be idempotent)
   - In main repo: `git checkout --force <original-ref>`
     (force required: editors/LSPs may have dirtied the working tree during sync)
   - In main repo: `git stash pop` if stash was created
     - If pop fails (conflicts): warn with recovery instructions, continue cleanup
   - In worktree: `git reset --soft <recorded-worktree-head-sha>`
     (undoes checkpoint commit, preserves all working directory changes)
   - Remove lock file
   - Print restore confirmation
```

## Output Structure

```
spotlight/
  src/
    index.ts              # CLI entry point (commander setup)
    commands/
      sync.ts             # sync command (includes watcher setup and colored output inline)
      list.ts             # list command implementation
    lib/
      git.ts              # git operations (worktree list, commit, checkout, stash)
      lock.ts             # lock file management
  tests/
    commands/
      sync.test.ts
      list.test.ts
    lib/
      git.test.ts
      lock.test.ts
  package.json
  tsconfig.json
  oxlintrc.json
  bunfig.toml
```

## Implementation Units

- [x] **Unit 1: Project scaffolding and tooling**

  **Goal:** Set up the Bun + TypeScript project with oxlint, oxfmt, and basic CLI structure

  **Requirements:** R7

  **Dependencies:** None

  **Files:**
  - Create: `package.json`
  - Create: `tsconfig.json`
  - Create: `oxlintrc.json`
  - Create: `bunfig.toml`
  - Create: `src/index.ts`
  - Create: `.gitignore`

  **Approach:**
  - Initialize with `bun init`
  - Add dependencies: `commander`, `chokidar`
  - Add dev dependencies: `oxlint`, `@biomejs/biome` (fallback formatter if oxfmt gaps), `bun-types`
  - Configure oxlint with TypeScript rules enabled
  - Set up `src/index.ts` with commander, registering `list` and `sync` commands as stubs
  - Add scripts: `dev`, `build`, `lint`, `fmt`, `test`

  **Patterns to follow:**
  - Standard Bun project structure

  **Test expectation:** none — scaffolding only, verified by successful `bun run lint` and `bun build`

  **Verification:**
  - `bun run src/index.ts --help` shows the two commands
  - `bun run lint` passes
  - `bun build --compile --outfile=spotlight src/index.ts` produces a binary

- [x] **Unit 2: Git operations module**

  **Goal:** Implement core git operations: worktree listing, checkpoint commit creation, checkout, stash, and state restoration

  **Requirements:** R1, R3, R4

  **Dependencies:** Unit 1

  **Files:**
  - Create: `src/lib/git.ts`
  - Create: `tests/lib/git.test.ts`

  **Approach:**
  - Use `Bun.spawn` / `Bun.spawnSync` to run git commands
  - `listWorktrees()`: parse `git worktree list --porcelain` output into structured data (path, HEAD, branch name)
  - `isGitRoot()`: verify CWD is a git repo root
  - `getCurrentRef()`: get current branch name or detached HEAD commit
  - `stashChanges()` / `popStash()`: save and restore uncommitted work. `stashChanges()` must use `git stash -u` to include untracked files
  - `getHeadSha(cwd)`: get the current HEAD SHA for any directory (used to record worktree's original HEAD)
  - `hasUncommittedChanges(cwd)`: check if there are staged or unstaged changes
  - `createCheckpoint(worktreePath)`: in the worktree, `git add -A && git commit -m "spotlight checkpoint"`. Returns the new commit SHA
  - `amendCheckpoint(worktreePath)`: in the worktree, `git add -A && git commit --amend --no-edit`. Returns the new commit SHA
  - `getTreeSha(sha, cwd)`: get the tree SHA for a given commit (for comparing whether a re-sync is needed)
  - `checkoutCommit(sha)`: in main repo, `git checkout <sha>` (detached HEAD)
  - `forceCheckoutRef(ref)`: in main repo, `git checkout --force <ref>` (for cleanup when working tree may be dirty)
  - `softReset(sha, cwd)`: `git reset --soft <sha>` in the given directory (for undoing checkpoint in worktree)
  - All git commands should throw typed errors on failure

  **Patterns to follow:**
  - Wrap each git operation in a function that handles stderr parsing for clear error messages

  **Test scenarios:**
  - Happy path: `listWorktrees` parses porcelain output with multiple worktrees correctly (mock git output)
  - Happy path: `getCurrentRef` returns branch name when on a branch
  - Happy path: `stashChanges` uses `-u` flag to include untracked files
  - Happy path: `createCheckpoint` returns the new commit SHA
  - Happy path: `getTreeSha` returns tree hash for a given commit
  - Edge case: `getCurrentRef` returns commit SHA when in detached HEAD state
  - Edge case: `listWorktrees` returns empty array when no linked worktrees exist (only main)
  - Edge case: `hasUncommittedChanges` returns false when worktree is clean
  - Error path: `createCheckpoint` throws when worktree path doesn't exist
  - Error path: `isGitRoot` returns false when not in a git repo
  - Error path: `forceCheckoutRef` succeeds even when working tree is dirty

  **Verification:**
  - All tests pass with `bun test`
  - Functions correctly parse real `git worktree list --porcelain` output format

- [x] **Unit 3: Lock file management**

  **Goal:** Implement lock file creation, checking, and cleanup to enforce single-sync-per-repo

  **Requirements:** R5

  **Dependencies:** Unit 1

  **Files:**
  - Create: `src/lib/lock.ts`
  - Create: `tests/lib/lock.test.ts`

  **Approach:**
  - Lock file at `.spotlight-sync.lock` in the repo root
  - Contents: JSON with `pid`, `worktree`, `startedAt` timestamp
  - `acquireLock(worktreeName)`: check if lock exists, if so check if PID is alive (via `process.kill(pid, 0)`). If stale (dead PID): print warning about potentially dirty repo state, remove lock and re-acquire. If active: throw error with details (PID, worktree name, start time)
  - `releaseLock()`: remove lock file
  - Register cleanup handlers for SIGINT, SIGTERM, and `beforeExit`

  **Patterns to follow:**
  - Use `Bun.write` / `Bun.file` for file operations

  **Test scenarios:**
  - Happy path: acquiring lock when none exists creates the lock file with correct contents
  - Happy path: releasing lock removes the file
  - Edge case: acquiring lock when a stale lock exists (dead PID) succeeds after cleanup
  - Error path: acquiring lock when an active lock exists throws with the active PID and worktree name
  - Integration: lock file is removed on process signal (SIGTERM handler)

  **Verification:**
  - All tests pass
  - Lock file is properly cleaned up even on forced exit

- [x] **Unit 4: List command**

  **Goal:** Implement `spotlight list` to display all git worktrees and their names for use with the sync command

  **Requirements:** R4

  **Dependencies:** Unit 2

  **Files:**
  - Create: `src/commands/list.ts`
  - Create: `tests/commands/list.test.ts`

  **Approach:**
  - Call `listWorktrees()` from the git module
  - Display in a formatted table: name, path, current branch/commit
  - Mark the main worktree distinctly (it's not a valid sync target)
  - Exit with error if not in a git repo

  **Patterns to follow:**
  - Use console.log with aligned columns (no heavy table library needed)

  **Test scenarios:**
  - Happy path: lists multiple worktrees with correct name, path, and branch
  - Edge case: shows meaningful message when no linked worktrees exist
  - Error path: exits with error when not in a git repo

  **Verification:**
  - `spotlight list` in a repo with worktrees shows them formatted correctly
  - `spotlight list` in a non-git directory shows a clear error

- [x] **Unit 5: Sync command**

  **Goal:** Implement `spotlight sync <worktree>` — the core sync lifecycle with watch, checkpoint, checkout, and cleanup

  **Requirements:** R1, R2, R3, R5

  **Dependencies:** Units 2, 3

  **Files:**
  - Create: `src/commands/sync.ts`
  - Create: `tests/commands/sync.test.ts`

  **Approach:**
  - Implement the full sync lifecycle from the technical design:
    1. Validate: ensure git root, worktree exists, acquire lock
    2. Record main repo HEAD ref and worktree HEAD SHA, stash uncommitted changes (`git stash -u`)
    3. Initial sync: if worktree has uncommitted changes, create checkpoint commit; otherwise use worktree HEAD directly. Checkout target in main repo
    4. Start chokidar watcher inline (no separate module — ignore `.git` as both file and directory, `node_modules/`, `ignoreInitial: true`, 300ms debounce)
    5. On each change: amend checkpoint, compare tree SHAs, checkout only if tree changed
    6. On SIGINT/SIGTERM: force-checkout original ref, pop stash (warn on conflict), soft-reset worktree to recorded SHA, release lock
  - Colored output inline using ANSI template literals (no separate logger module)
  - Display clear status messages: "Syncing...", "Watching for changes...", "Change detected, syncing...", "Restored to <ref>"
  - Handle edge cases: worktree not found, lock already held, stash conflicts on restore

  **Patterns to follow:**
  - Use `AbortController` pattern for clean shutdown coordination
  - Signal handlers must be idempotent (guard against double-cleanup with a boolean flag)

  **Test scenarios:**
  - Happy path: sync command creates checkpoint and checks it out in main repo
  - Happy path: file change triggers re-sync with new checkpoint
  - Happy path: CTRL+C restores original branch and pops stash
  - Happy path: sync when worktree has no uncommitted changes — checks out worktree HEAD directly, watcher still starts
  - Edge case: cleanup is idempotent (double SIGINT doesn't corrupt state)
  - Edge case: change that doesn't affect tracked files (e.g., gitignored file) skips checkout (tree SHA unchanged)
  - Error path: sync fails fast when lock is already held, showing the active sync details
  - Error path: sync aborts with clear message when worktree name not found
  - Error path: stash pop conflict on restore prints warning with recovery instructions but continues cleanup
  - Error path: main repo working tree dirty during cleanup — force-checkout succeeds
  - Integration: full lifecycle — start sync, detect change, re-sync, stop, verify restore

  **Verification:**
  - Full manual test: start sync, make changes in worktree, verify they appear in main repo, CTRL+C, verify main repo is restored
  - All tests pass

- [x] **Unit 6: Build and Homebrew formula template**

  **Goal:** Configure build pipeline and create Homebrew tap formula template for distribution

  **Requirements:** R6

  **Dependencies:** Units 4, 5

  **Files:**
  - Modify: `package.json` (build scripts)
  - Create: `Formula/spotlight.rb` (Homebrew formula template with placeholder URLs)

  **Approach:**
  - Add `build` script: `bun build --compile --outfile=spotlight src/index.ts`
  - Add platform-specific build targets for macOS (arm64, x64) and Linux (x64)
  - Create Homebrew formula template with placeholder release URLs and SHA256 checksums — the formula cannot be fully functional until a GitHub release workflow exists (deferred)
  - Note: `bun build --compile` binaries may require code signing on macOS (`codesign --sign -`). Document this in the build script comments
  - Document the intended tap installation: `brew tap <user>/spotlight && brew install spotlight`

  **Patterns to follow:**
  - Standard Homebrew formula structure for binary distributions
  - Use `hardware.arm?` in formula for Apple Silicon vs Intel

  **Test scenarios:**
  - Happy path: `bun build --compile` produces a working binary
  - Happy path: compiled binary runs `spotlight --help`, `spotlight list`, `spotlight sync` correctly
  - Edge case: binary works without Bun installed on the system

  **Verification:**
  - Compiled binary runs all commands correctly on macOS
  - Formula template has correct structure (but won't pass `brew audit` until real URLs are provided)

## System-Wide Impact

- **Interaction graph:** The tool interacts with git state (HEAD, index, stash) in both the main repo and the worktree. Careful sequencing is critical to avoid corruption
- **Error propagation:** Git command failures during a sync cycle should log a warning and skip that cycle, not crash the watcher. Cleanup failures (stash pop conflicts) should warn but continue cleanup (release lock, restore ref) — never leave cleanup half-done
- **State lifecycle risks:** Power loss or `kill -9` during sync leaves the main repo on a detached checkpoint commit with a stale lock file. On next run, the tool detects the stale lock (dead PID), warns the user that the repo may be in a dirty state, then cleans up the lock and proceeds. The user can inspect and fix the repo state before starting a new sync
- **Unchanged invariants:** The worktree's actual working changes are never lost — checkpoint commits are soft-reset on cleanup, preserving the working directory

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Unclean shutdown leaves repo on detached HEAD | Lock file contains enough info to detect and guide recovery. Consider a `spotlight recover` command in the future |
| Chokidar misses events on some filesystems | Use polling fallback option as a CLI flag if needed |
| Oxfmt alpha status may have formatting gaps | Use oxfmt where it works, fall back to manual formatting for edge cases |
| Large repos may have slow checkpoint commits | The debounce window helps batch rapid changes. Could add `--debounce` flag later |
| `bun build --compile` binaries may need code signing on macOS | Add ad-hoc signing (`codesign --sign -`) to the build script. Document for Homebrew formula |

## Sources & References

- Conductor Spotlight docs: https://docs.conductor.build/guides/spotlight-testing
- Chokidar: https://github.com/paulmillr/chokidar
- Bun compile docs: https://bun.sh/docs/bundler/executables
- Oxlint: https://oxc.rs/docs/guide/usage/linter.html
