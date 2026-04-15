package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	worktreeName := args[0]
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// --- STARTUP ---

	if !git.IsGitRoot("") {
		fmt.Fprintf(os.Stderr, "%sError:%s Not in a git repository root directory.\n", red, reset)
		os.Exit(1)
	}

	worktrees, err := git.ListWorktrees("")
	if err != nil {
		return err
	}

	var worktreePath string
	for _, wt := range worktrees {
		if filepath.Base(wt.Path) == worktreeName {
			worktreePath = wt.Path
			break
		}
	}

	if worktreePath == "" {
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
	}

	if err := lock.Acquire(cwd, worktreeName); err != nil {
		return err
	}

	originalRef, err := git.GetCurrentRef("")
	if err != nil {
		lock.Release(cwd)
		return fmt.Errorf("failed to get current ref: %w", err)
	}

	worktreeOriginalHead, err := git.GetHeadSha(worktreePath)
	if err != nil {
		lock.Release(cwd)
		return fmt.Errorf("failed to get worktree HEAD: %w", err)
	}

	didStash, err := git.StashChanges("")
	if err != nil {
		lock.Release(cwd)
		return fmt.Errorf("failed to stash changes: %w", err)
	}

	// --- INITIAL SYNC ---

	var currentCheckoutSha string
	hasCheckpoint := false

	hasChanges, err := git.HasUncommittedChanges(worktreePath)
	if err != nil {
		lock.Release(cwd)
		return err
	}

	if hasChanges {
		currentCheckoutSha, err = git.CreateCheckpoint(worktreePath)
		if err != nil {
			lock.Release(cwd)
			return fmt.Errorf("failed to create checkpoint: %w", err)
		}
		hasCheckpoint = true
	} else {
		currentCheckoutSha, err = git.GetHeadSha(worktreePath)
		if err != nil {
			lock.Release(cwd)
			return err
		}
	}

	if err := git.CheckoutCommit(currentCheckoutSha, ""); err != nil {
		lock.Release(cwd)
		return fmt.Errorf("failed to checkout: %w", err)
	}

	fmt.Printf("%s✓%s Synced %s → main repo\n", green, reset, worktreeName)

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

		watcher.Close()

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
