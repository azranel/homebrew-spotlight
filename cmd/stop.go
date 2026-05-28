package cmd

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/azranel/spotlight/internal/git"
	"github.com/azranel/spotlight/internal/lock"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running spotlight sync and restore the repository",
	Args:  cobra.NoArgs,
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	ld, err := lock.Read(cwd)
	if err != nil {
		fmt.Println("No active spotlight sync to stop.")
		return nil
	}

	once := ld.Mode == lock.ModeOnce

	if !once && lock.IsPidAlive(ld.PID) {
		fmt.Printf("Stopping spotlight sync (PID %d)...\n", ld.PID)
		if proc, err := os.FindProcess(ld.PID); err == nil {
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				fmt.Fprintf(os.Stderr, "%sWarning:%s failed to signal watcher: %s\n", yellow, reset, err)
			}
		}

		// The watcher restores the repo and removes the lock during its own
		// cleanup. Wait for that to happen.
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := lock.Read(cwd); err != nil {
				fmt.Printf("%s✓%s Stopped. Restored to %s\n", green, reset, refOrUnknown(ld.OriginalRef))
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}

		if lock.IsPidAlive(ld.PID) {
			return fmt.Errorf("watcher (PID %d) did not shut down within 10s; it may be busy. Try again, or kill it manually", ld.PID)
		}
		// Process died without finishing cleanup; fall through to manual restore.
		fmt.Fprintf(os.Stderr, "%sWarning:%s watcher exited without cleaning up; restoring manually.\n", yellow, reset)
	} else if once {
		fmt.Printf("Restoring after one-shot sync of %q...\n", ld.Worktree)
	} else {
		fmt.Printf("Watcher (PID %d) is not running. Cleaning up stale lock and restoring...\n", ld.PID)
	}

	return manualRestore(ld)
}

// manualRestore recovers the main repo from stored lock data when the watcher
// is no longer alive to do it itself.
func manualRestore(ld *lock.LockData) error {
	if ld.OriginalRef != "" {
		if err := git.ForceCheckoutRef(ld.OriginalRef, ""); err != nil {
			fmt.Fprintf(os.Stderr, "%sWarning:%s failed to restore ref %s: %s\n", yellow, reset, ld.OriginalRef, err)
		}
	}

	if ld.DidStash {
		success, conflicted, err := git.PopStash("")
		if !success && conflicted {
			fmt.Fprintf(os.Stderr, "%sWarning:%s stash pop had conflicts. Run 'git stash pop' manually to resolve.\n", yellow, reset)
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "%sWarning:%s failed to pop stash: %s\n", yellow, reset, err)
		}
	}

	if ld.WorktreePath != "" && ld.WorktreeOriginalHead != "" {
		if err := git.SoftReset(ld.WorktreeOriginalHead, ld.WorktreePath); err != nil {
			fmt.Fprintf(os.Stderr, "%sWarning:%s failed to reset worktree: %s\n", yellow, reset, err)
		}
	}

	cwd, _ := os.Getwd()
	lock.Release(cwd)
	fmt.Printf("%s✓%s Cleaned up. Restored to %s\n", green, reset, refOrUnknown(ld.OriginalRef))
	return nil
}

func refOrUnknown(ref string) string {
	if ref == "" {
		return "previous state"
	}
	return ref
}
