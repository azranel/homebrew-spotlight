# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`spotlight` is a single-binary Go CLI that mirrors a git worktree's working state into the main repository so you can run/test those changes from the main repo (hot reload, app server, etc.) without switching branches there. It is a standalone replacement for Conductor's Spotlight feature.

## Commands

```bash
go build -o spotlight .   # build the binary
go test ./...             # run tests (NOTE: no test files exist yet)
go vet ./...              # vet/lint
go run . list             # run a subcommand without building
go run . sync <worktree>  # run sync from the main repo root
```

There is currently no test suite — `go test ./...` passes vacuously. If you add tests, follow standard `*_test.go` placement next to the package under test.

## Architecture

Three layers, all small:

- **`main.go`** — calls `cmd.Execute()`, prints errors to stderr, exits non-zero on failure.
- **`cmd/`** — cobra command definitions. `root.go` holds the root command and the hardcoded `version` string. `list.go` and `sync.go` register themselves onto `rootCmd` via `init()`. `sync.go` contains essentially all the orchestration logic and the fsnotify watch loop.
- **`internal/git/`** — every git operation, implemented by shelling out to the `git` binary via `exec.Command` (no git library). **`internal/lock/`** — PID-based lockfile plus signal-driven cleanup registration.

### The `cwd` convention (most important pattern)

Every function in `internal/git` takes a `cwd string` as its last argument. This is what makes the whole tool work, because spotlight operates on **two repos at once**:

- `cwd == ""` → operate on the current directory = **the main repo** (where the user runs `spotlight sync`).
- `cwd == worktreePath` → operate on **the worktree** being synced.

When editing sync logic, always be deliberate about which repo a git call targets. Passing the wrong `cwd` is the easiest way to corrupt state (e.g. checkpointing the main repo instead of the worktree).

### How `sync` works (cmd/sync.go)

1. Verify the cwd is a git root (`git.IsGitRoot`), resolve the worktree name to a path via `git.ListWorktrees` (matches on the last path segment).
2. Acquire the lock (`lock.Acquire`) — refuses if another live PID holds it; reclaims a stale lock if the recorded PID is dead.
3. Record restore points: main repo's current ref (`GetCurrentRef`) and the worktree's original HEAD (`GetHeadSha`).
4. Stash the main repo (`git stash -u`), remembering whether anything was actually stashed.
5. Initial sync: if the worktree has uncommitted changes, create a "spotlight checkpoint" commit there; otherwise use its HEAD. Then check that commit out in the main repo (detached HEAD).
6. Watch loop: `fsnotify` recursively watches the worktree (`.git` and `node_modules` are skipped, see `shouldIgnore`/`addWatchRecursive`). Changes are debounced 300ms, then the checkpoint is amended (`--amend --no-edit`) and re-checked-out in the main repo — **but only if the tree SHA actually changed** (`GetTreeSha` comparison), to avoid pointless checkouts.
7. Cleanup (on Ctrl+C / SIGTERM, via `lock.RegisterCleanupHandlers`): force-checkout the original ref in the main repo, pop the stash, soft-reset the worktree back to its original HEAD to drop the checkpoint, release the lock.

### Cleanup is duplicated by design — keep both paths correct

There are two cleanup routes and both must leave the repos clean:

- **Early-failure path**: before the watch loop is established, each error return calls `lock.Release(cwd)` manually before returning. If you add steps here, release the lock (and undo anything already done) on every error branch.
- **Signal path**: once watching, the `cleanup` closure (guarded by `cleaningUp`/`cleanupMu` so it runs once) is invoked from the signal handler. It is the only thing that restores the stash and worktree after a normal session.

When you add a new mutation to the startup sequence, make sure its inverse exists in the `cleanup` closure, and that early-error returns before the watcher don't leave the stash/checkpoint dangling.

## Releasing

The version lives **only** in `cmd/root.go` (`var version = "..."`). The release flow reads it by grepping that file, so bump it there first.

```bash
# 1. bump `version` in cmd/root.go, commit it
# 2.
./scripts/release.sh
```

`scripts/release.sh` cross-compiles darwin/arm64, darwin/amd64, linux/amd64 with `-ldflags "-s -w -X .../cmd.version=$VERSION"`, tarballs each (binary renamed to `spotlight` inside), tags + pushes, creates a GitHub release (`gh`), and rewrites `Formula/spotlight.rb` with the new URLs and SHA256s. After it finishes you must commit and push the updated formula yourself. The script refuses to run with a dirty tree, an existing tag, version still `"dev"`, or no `gh` auth.
