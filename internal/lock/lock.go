package lock

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const lockFilename = ".spotlight-sync.lock"

type LockData struct {
	PID       int    `json:"pid"`
	Worktree  string `json:"worktree"`
	StartedAt string `json:"startedAt"`
}

func lockPath(repoRoot string) string {
	return filepath.Join(repoRoot, lockFilename)
}

func isPidAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func Acquire(repoRoot string, worktreeName string) error {
	path := lockPath(repoRoot)

	data, err := os.ReadFile(path)
	if err == nil {
		var existing LockData
		if jsonErr := json.Unmarshal(data, &existing); jsonErr == nil {
			if isPidAlive(existing.PID) {
				return fmt.Errorf("another sync is already running (PID %d, worktree %q, started %s). Only one sync per repository is allowed", existing.PID, existing.Worktree, existing.StartedAt)
			}
			fmt.Fprintf(os.Stderr, "Warning: Previous sync (PID %d, worktree %q) did not shut down cleanly. Your repository may be in a dirty state. Cleaning up stale lock.\n", existing.PID, existing.Worktree)
			os.Remove(path)
		}
	}

	lockData := LockData{
		PID:       os.Getpid(),
		Worktree:  worktreeName,
		StartedAt: time.Now().Format(time.RFC3339),
	}
	content, err := json.MarshalIndent(lockData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
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
