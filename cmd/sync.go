package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/azranel/spotlight/internal/git"
	"github.com/azranel/spotlight/internal/lock"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

const (
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	bold   = "\033[1m"
	reset  = "\033[0m"
)

var syncCmd = &cobra.Command{
	Use:   "sync <worktree>",
	Short: "Sync a worktree's changes to the main repository",
	Args:  cobra.ExactArgs(1),
	RunE:  runSync,
}

func init() {
	syncCmd.Flags().BoolP("detach", "d", false, "Run the watcher in the background and return immediately")
	syncCmd.Flags().Bool("once", false, "Sync the worktree's current state once and exit (no watcher)")
	syncCmd.Flags().Bool("daemon", false, "Internal: run as the backgrounded watcher process")
	syncCmd.Flags().MarkHidden("daemon")
	syncCmd.MarkFlagsMutuallyExclusive("detach", "once")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	worktreeName := args[0]
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	detach, _ := cmd.Flags().GetBool("detach")
	daemon, _ := cmd.Flags().GetBool("daemon")
	once, _ := cmd.Flags().GetBool("once")

	if !git.IsGitRoot("") {
		fmt.Fprintf(os.Stderr, "%sError:%s Not in a git repository root directory.\n", red, reset)
		os.Exit(1)
	}

	worktreePath := resolveWorktreePath(worktreeName)

	if once {
		return runOnce(cwd, worktreeName, worktreePath)
	}

	// Detach: validate, fail fast if a sync is already active, then spawn a
	// backgrounded copy of ourselves and return immediately.
	if detach && !daemon {
		if ld, active := lock.Active(cwd); active {
			return fmt.Errorf("spotlight is already syncing %q (PID %d, started %s). Run 'spotlight stop' first", ld.Worktree, ld.PID, ld.StartedAt)
		}
		logPath, err := logFilePath(cwd, worktreeName)
		if err != nil {
			return err
		}
		return spawnDaemon(cwd, worktreeName, logPath)
	}

	return runWatch(cwd, worktreeName, worktreePath, daemon)
}

// resolveWorktreePath maps a worktree name (last path segment) to its absolute
// path, printing available worktrees and exiting if it cannot be found.
func resolveWorktreePath(worktreeName string) string {
	worktrees, err := git.ListWorktrees("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError:%s %s\n", red, reset, err)
		os.Exit(1)
	}

	for _, wt := range worktrees {
		if filepath.Base(wt.Path) == worktreeName {
			return wt.Path
		}
	}

	fmt.Fprintf(os.Stderr, "%sError:%s Worktree %q not found.\n", red, reset, worktreeName)
	var linked []string
	for i, wt := range worktrees {
		if i == 0 || wt.Bare {
			continue
		}
		linked = append(linked, filepath.Base(wt.Path))
	}
	if len(linked) > 0 {
		fmt.Fprintln(os.Stderr, "Available worktrees:")
		for _, name := range linked {
			fmt.Fprintf(os.Stderr, "  - %s\n", name)
		}
	} else {
		fmt.Fprintln(os.Stderr, "No linked worktrees available.")
	}
	os.Exit(1)
	return ""
}

// logFilePath returns the per-repo, per-worktree log file under ~/.spotlight,
// creating the directory if needed.
func logFilePath(repoRoot, worktreeName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".spotlight")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%s.log", filepath.Base(repoRoot), worktreeName)
	return filepath.Join(dir, name), nil
}

// spawnDaemon re-execs this binary as a detached background watcher whose
// stdout/stderr stream to a log file, then prints a handle and returns.
func spawnDaemon(repoRoot, worktreeName, logPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}
	defer logFile.Close()

	c := exec.Command(exe, "sync", worktreeName, "--daemon")
	c.Dir = repoRoot
	c.Stdout = logFile
	c.Stderr = logFile
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := c.Start(); err != nil {
		return fmt.Errorf("failed to start background watcher: %w", err)
	}

	fmt.Printf("%s✓%s Spotlight watching %s%s%s in the background (PID %d)\n", green, reset, bold, worktreeName, reset, c.Process.Pid)
	fmt.Printf("  logs:   %s\n", logPath)
	fmt.Printf("  status: spotlight status\n")
	fmt.Printf("  stop:   spotlight stop\n")
	return nil
}

// syncState captures everything the watch loop and cleanup need after the
// initial sync has run.
type syncState struct {
	originalRef          string
	worktreeOriginalHead string
	didStash             bool
	hasCheckpoint        bool
	checkpointSha        string
}

// performInitialSync acquires the lock, records restore points, stashes the
// main repo, creates (or reuses) the worktree's checkpoint, and checks it out
// in the main repo. The restore info is written to the lock so 'spotlight stop'
// can recover even if this process later dies. On error it releases the lock.
func performInitialSync(cwd, worktreeName, worktreePath, mode string, isDaemon bool) (*syncState, error) {
	if err := lock.Acquire(cwd, worktreeName, mode); err != nil {
		return nil, err
	}

	originalRef, err := git.GetCurrentRef("")
	if err != nil {
		lock.Release(cwd)
		return nil, fmt.Errorf("failed to get current ref: %w", err)
	}

	worktreeOriginalHead, err := git.GetHeadSha(worktreePath)
	if err != nil {
		lock.Release(cwd)
		return nil, fmt.Errorf("failed to get worktree HEAD: %w", err)
	}

	didStash, err := git.StashChanges("")
	if err != nil {
		lock.Release(cwd)
		return nil, fmt.Errorf("failed to stash changes: %w", err)
	}

	var checkpointSha string
	hasCheckpoint := false

	hasChanges, err := git.HasUncommittedChanges(worktreePath)
	if err != nil {
		lock.Release(cwd)
		return nil, err
	}

	if hasChanges {
		checkpointSha, err = git.CreateCheckpoint(worktreePath)
		if err != nil {
			lock.Release(cwd)
			return nil, fmt.Errorf("failed to create checkpoint: %w", err)
		}
		hasCheckpoint = true
	} else {
		checkpointSha, err = git.GetHeadSha(worktreePath)
		if err != nil {
			lock.Release(cwd)
			return nil, err
		}
	}

	if err := git.CheckoutCommit(checkpointSha, ""); err != nil {
		lock.Release(cwd)
		return nil, fmt.Errorf("failed to checkout: %w", err)
	}

	fmt.Printf("%s✓%s Synced %s → main repo\n", green, reset, worktreeName)

	logPath := ""
	if isDaemon {
		logPath, _ = logFilePath(cwd, worktreeName)
	}
	if err := lock.SetRestoreInfo(cwd, originalRef, worktreePath, worktreeOriginalHead, logPath, didStash); err != nil {
		fmt.Fprintf(os.Stderr, "%sWarning:%s could not record restore info: %s\n", yellow, reset, err)
	}

	return &syncState{
		originalRef:          originalRef,
		worktreeOriginalHead: worktreeOriginalHead,
		didStash:             didStash,
		hasCheckpoint:        hasCheckpoint,
		checkpointSha:        checkpointSha,
	}, nil
}

// runOnce syncs the worktree's current state into the main repo and exits
// without watching. The main repo stays on the checkpoint (detached HEAD) and
// the once-mode lock keeps other syncs out until 'spotlight stop' restores it.
func runOnce(cwd, worktreeName, worktreePath string) error {
	if _, err := performInitialSync(cwd, worktreeName, worktreePath, lock.ModeOnce, false); err != nil {
		return err
	}
	fmt.Printf("  Main repo now reflects %s%s%s (detached HEAD, no watcher).\n", bold, worktreeName, reset)
	fmt.Printf("  Run %sspotlight stop%s to restore your branch when done.\n", bold, reset)
	return nil
}

// runWatch performs the initial sync, then watches the worktree until
// interrupted. It blocks; in daemon mode it is the long-lived background process.
func runWatch(cwd, worktreeName, worktreePath string, isDaemon bool) error {
	state, err := performInitialSync(cwd, worktreeName, worktreePath, lock.ModeWatch, isDaemon)
	if err != nil {
		return err
	}

	originalRef := state.originalRef
	worktreeOriginalHead := state.worktreeOriginalHead
	didStash := state.didStash
	hasCheckpoint := state.hasCheckpoint
	currentCheckoutSha := state.checkpointSha

	// --- WATCH LOOP ---

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		lock.Release(cwd)
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Add worktree directory recursively
	if err := addWatchRecursive(watcher, worktreePath); err != nil {
		watcher.Close()
		lock.Release(cwd)
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	var debounceTimer *time.Timer
	var debounceMu sync.Mutex

	// --- CLEANUP ---

	cleaningUp := false
	var cleanupMu sync.Mutex

	cleanup := func() {
		cleanupMu.Lock()
		if cleaningUp {
			cleanupMu.Unlock()
			return
		}
		cleaningUp = true
		cleanupMu.Unlock()

		// Stop pending debounced syncs so they don't race the restore for the
		// git index lock. Do NOT close the watcher first: closing it unblocks
		// the event loop to return from main(), which would terminate the
		// process before the restore below finishes. The handler's os.Exit(0)
		// tears everything down once restore is complete.
		debounceMu.Lock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceMu.Unlock()

		if err := git.ForceCheckoutRef(originalRef, ""); err != nil {
			fmt.Fprintf(os.Stderr, "%sWarning:%s Failed to restore ref: %s\n", yellow, reset, err)
		}

		if didStash {
			success, conflicted, err := git.PopStash("")
			if !success && conflicted {
				fmt.Fprintf(os.Stderr, "%sWarning:%s Stash pop had conflicts. Run 'git stash pop' manually to resolve.\n", yellow, reset)
			} else if err != nil {
				fmt.Fprintf(os.Stderr, "%sWarning:%s Failed to pop stash: %s\n", yellow, reset, err)
			}
		}

		if hasCheckpoint {
			if err := git.SoftReset(worktreeOriginalHead, worktreePath); err != nil {
				fmt.Fprintf(os.Stderr, "%sWarning:%s Failed to reset worktree: %s\n", yellow, reset, err)
			}
		}

		lock.Release(cwd)
		fmt.Printf("%s✓%s Restored to %s\n", green, reset, originalRef)
	}

	lock.RegisterCleanupHandlers(cleanup)

	fmt.Printf("%sWatching for changes...%s (Ctrl+C to stop)\n", bold, reset)

	// Event loop
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if shouldIgnore(event.Name, worktreePath) {
				continue
			}

			// Add new directories to watcher
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					addWatchRecursive(watcher, event.Name)
				}
			}

			debounceMu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(300*time.Millisecond, func() {
				cleanupMu.Lock()
				stopping := cleaningUp
				cleanupMu.Unlock()
				if stopping {
					return
				}

				fmt.Printf("%s⟳%s Change detected, syncing...\n", yellow, reset)

				var newSha string
				var syncErr error
				if hasCheckpoint {
					newSha, syncErr = git.AmendCheckpoint(worktreePath)
				} else {
					newSha, syncErr = git.CreateCheckpoint(worktreePath)
					if syncErr == nil {
						hasCheckpoint = true
					}
				}

				if syncErr != nil {
					fmt.Fprintf(os.Stderr, "%sError during sync:%s %s\n", red, reset, syncErr)
					return
				}

				newTree, err := git.GetTreeSha(newSha, worktreePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%sError:%s %s\n", red, reset, err)
					return
				}

				currentTree, err := git.GetTreeSha(currentCheckoutSha, "")
				if err != nil {
					fmt.Fprintf(os.Stderr, "%sError:%s %s\n", red, reset, err)
					return
				}

				if newTree != currentTree {
					if err := git.CheckoutCommit(newSha, ""); err != nil {
						fmt.Fprintf(os.Stderr, "%sError:%s %s\n", red, reset, err)
						return
					}
					currentCheckoutSha = newSha
					now := time.Now().Format("15:04:05")
					fmt.Printf("%s✓%s Synced %s → main repo [%s]\n", green, reset, worktreeName, now)
				}
			})
			debounceMu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "%sWatcher error:%s %s\n", yellow, reset, err)
		}
	}
}

func shouldIgnore(path string, worktreePath string) bool {
	rel, err := filepath.Rel(worktreePath, path)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		if part == ".git" || part == "node_modules" {
			return true
		}
	}
	return false
}

func addWatchRecursive(watcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
}
