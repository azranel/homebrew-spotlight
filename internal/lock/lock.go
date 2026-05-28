package lock

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// Sync modes recorded in the lock. A watch lock is held by a live process; a
// once lock has no owning process and stays active until 'spotlight stop'.
const (
	ModeWatch = "watch"
	ModeOnce  = "once"
)

type LockData struct {
	PID                  int    `json:"pid"`
	Worktree             string `json:"worktree"`
	WorktreePath         string `json:"worktreePath,omitempty"`
	StartedAt            string `json:"startedAt"`
	Mode                 string `json:"mode,omitempty"`
	OriginalRef          string `json:"originalRef,omitempty"`
	WorktreeOriginalHead string `json:"worktreeOriginalHead,omitempty"`
	DidStash             bool   `json:"didStash"`
	LogPath              string `json:"logPath,omitempty"`
}

// lockPath returns the lockfile location for a repo. The lock lives OUTSIDE the
// repo (under ~/.spotlight/locks), keyed by the repo's canonical path, so that
// the `git stash -u` performed during sync can never sweep it away.
func lockPath(repoRoot string) string {
	canon := repoRoot
	if resolved, err := filepath.EvalSymlinks(repoRoot); err == nil {
		canon = resolved
	}
	sum := sha256.Sum256([]byte(canon))
	key := fmt.Sprintf("%s-%s", filepath.Base(canon), hex.EncodeToString(sum[:])[:8])

	home, err := os.UserHomeDir()
	if err != nil {
		// Fall back to a temp dir rather than the repo itself.
		return filepath.Join(os.TempDir(), "spotlight-"+key+".lock")
	}
	dir := filepath.Join(home, ".spotlight", "locks")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, key+".lock")
}

// IsPidAlive reports whether a process with the given PID is currently running.
func IsPidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Read returns the lock data for the repo, or an error if no lock exists.
func Read(repoRoot string) (*LockData, error) {
	data, err := os.ReadFile(lockPath(repoRoot))
	if err != nil {
		return nil, err
	}
	var ld LockData
	if err := json.Unmarshal(data, &ld); err != nil {
		return nil, err
	}
	return &ld, nil
}

// Active returns the lock data and whether it should block a new sync. A
// once-mode lock blocks until the repo is restored via 'spotlight stop', even
// though no process holds it; a watch-mode lock blocks only while its process
// is alive (a dead watcher is a crash we can reclaim). A nil result means no
// lock file exists at all.
func Active(repoRoot string) (*LockData, bool) {
	ld, err := Read(repoRoot)
	if err != nil {
		return nil, false
	}
	return ld, ld.Mode == ModeOnce || IsPidAlive(ld.PID)
}

func write(repoRoot string, ld *LockData) error {
	content, err := json.MarshalIndent(ld, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(lockPath(repoRoot), content, 0644)
}

// Acquire claims the single-instance lock for this repo. It refuses if another
// sync is active (a live watcher, or any un-restored once-mode sync), and
// reclaims the lock if it belonged to a watcher whose process has since died.
func Acquire(repoRoot, worktreeName, mode string) error {
	if ld, active := Active(repoRoot); active {
		return fmt.Errorf("spotlight is already syncing %q (PID %d, started %s). Run 'spotlight stop' first", ld.Worktree, ld.PID, ld.StartedAt)
	} else if ld != nil {
		fmt.Fprintf(os.Stderr, "Warning: previous sync (PID %d, worktree %q) did not shut down cleanly. Your repository may be in a dirty state. Cleaning up stale lock.\n", ld.PID, ld.Worktree)
		os.Remove(lockPath(repoRoot))
	}

	lockData := LockData{
		PID:       os.Getpid(),
		Worktree:  worktreeName,
		Mode:      mode,
		StartedAt: time.Now().Format(time.RFC3339),
	}
	return write(repoRoot, &lockData)
}

// SetRestoreInfo records the data needed to restore the repo if the watcher is
// stopped or dies, without disturbing the PID/worktree/startedAt fields.
func SetRestoreInfo(repoRoot, originalRef, worktreePath, worktreeOriginalHead, logPath string, didStash bool) error {
	ld, err := Read(repoRoot)
	if err != nil {
		return err
	}
	ld.OriginalRef = originalRef
	ld.WorktreePath = worktreePath
	ld.WorktreeOriginalHead = worktreeOriginalHead
	ld.DidStash = didStash
	if logPath != "" {
		ld.LogPath = logPath
	}
	return write(repoRoot, ld)
}

func Release(repoRoot string) {
	path := lockPath(repoRoot)
	os.Remove(path)
}

func RegisterCleanupHandlers(cleanup func()) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cleanup()
		os.Exit(0)
	}()
}
