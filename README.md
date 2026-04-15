# Spotlight

Sync git worktree changes to the main repository as checkpoints. A standalone replacement for [Conductor's Spotlight](https://docs.conductor.build/guides/spotlight-testing) feature.

## How it works

Spotlight creates a temporary checkpoint commit in your worktree, then checks it out in the main repo so you can test the changes there (hot reloading, running the app, etc.). It watches for file changes and re-syncs automatically. When you stop with Ctrl+C, the main repo is restored to its original state and the checkpoint commit is removed from the worktree.

## Install

### Homebrew (recommended)

Installs a self-contained binary — no runtime dependencies needed.

```bash
brew tap azranel/spotlight
brew install spotlight
```

### From source

Requires [Go](https://go.dev) 1.26+.

```bash
git clone https://github.com/azranel/spotlight.git
cd spotlight
go build -o spotlight .
cp spotlight ~/.local/bin/  # or anywhere on your PATH
```

## Usage

### List worktrees

```bash
spotlight list
```

Shows all git worktrees with their name, branch, and path. The name (last directory segment) is what you pass to `sync`.

### Sync a worktree

```bash
spotlight sync <worktree-name>
```

Run this from the **main repository root**. It will:

1. Stash any uncommitted changes in the main repo
2. Create a checkpoint commit in the worktree and check it out in the main repo
3. Watch for file changes and re-sync automatically
4. On Ctrl+C: restore the main repo to its original branch, pop the stash, and clean up the checkpoint

Only one sync operation per repository is allowed at a time.

### Example workflow

```bash
# Create a worktree for your feature branch
git worktree add ../my-feature feature-branch

# In another terminal, make changes in ../my-feature/
# ...

# From the main repo, start syncing
spotlight sync my-feature
# ✓ Synced my-feature → main repo
# Watching for changes... (Ctrl+C to stop)

# Changes in ../my-feature/ are now visible in the main repo
# Run your app, tests, etc. from the main repo directory

# When done, Ctrl+C
# ✓ Restored to main
```

## Updating

### Homebrew

```bash
brew update
brew upgrade spotlight
```

### From source

```bash
git pull
go build -o spotlight .
```

### Checking your version

```bash
spotlight --version
```

## Development

```bash
go build -o spotlight .   # build
go test ./...             # run tests
go vet ./...              # lint
```

## Releasing

Requires [GitHub CLI](https://cli.github.com/) (`gh`).

1. Bump the version in `cmd/root.go`
2. Commit the version bump
3. Run the release script:

```bash
./scripts/release.sh
```

This will:
- Cross-compile binaries for macOS (arm64, amd64) and Linux (amd64)
- Strip debug symbols (`-s -w`) for smaller binaries (~4MB)
- Create tarballs and compute SHA256 checksums
- Tag the commit and push the tag
- Create a GitHub release with the binaries attached
- Update `Formula/spotlight.rb` with the real checksums

After the script finishes, commit and push the updated formula.

## How syncing works internally

1. Records the main repo's current branch and worktree's HEAD SHA
2. Stashes uncommitted changes in the main repo (`git stash -u`)
3. Stages and commits all worktree changes as a "spotlight checkpoint"
4. Checks out the checkpoint commit in the main repo (detached HEAD)
5. Watches the worktree with fsnotify (300ms debounce)
6. On changes: amends the checkpoint, compares tree SHAs, only checks out if the tree actually changed
7. On cleanup: force-checkouts the original branch, pops stash, soft-resets the worktree to remove the checkpoint

A `.spotlight-sync.lock` file prevents concurrent syncs on the same repo.

## License

MIT
