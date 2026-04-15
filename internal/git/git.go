package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Worktree struct {
	Path   string
	Head   string
	Branch string
	Bare   bool
}

func runGit(args []string, cwd string) (string, error) {
	cmd := exec.Command("git", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, fmt.Errorf("git %s failed: %s", strings.Join(args, " "), output)
	}
	return output, nil
}

func ListWorktrees(cwd string) ([]Worktree, error) {
	out, err := runGit([]string{"worktree", "list", "--porcelain"}, cwd)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var worktrees []Worktree
	blocks := strings.Split(out, "\n\n")

	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) == 0 || lines[0] == "" {
			continue
		}

		var wt Worktree
		for _, line := range lines {
			switch {
			case strings.HasPrefix(line, "worktree "):
				wt.Path = strings.TrimPrefix(line, "worktree ")
			case strings.HasPrefix(line, "HEAD "):
				wt.Head = strings.TrimPrefix(line, "HEAD ")
			case strings.HasPrefix(line, "branch "):
				wt.Branch = strings.TrimPrefix(line, "branch ")
			case line == "detached":
				wt.Branch = ""
			case line == "bare":
				wt.Bare = true
			}
		}
		if wt.Path != "" {
			worktrees = append(worktrees, wt)
		}
	}
	return worktrees, nil
}

func IsGitRoot(cwd string) bool {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return false
		}
	}
	out, err := runGit([]string{"rev-parse", "--show-toplevel"}, cwd)
	if err != nil {
		return false
	}
	// Resolve symlinks for comparison (macOS /var -> /private/var)
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return false
	}
	outResolved, err := filepath.EvalSymlinks(out)
	if err != nil {
		return false
	}
	return resolved == outResolved
}

func GetCurrentRef(cwd string) (string, error) {
	out, err := runGit([]string{"symbolic-ref", "--short", "HEAD"}, cwd)
	if err == nil {
		return out, nil
	}
	return runGit([]string{"rev-parse", "HEAD"}, cwd)
}

func GetHeadSha(cwd string) (string, error) {
	return runGit([]string{"rev-parse", "HEAD"}, cwd)
}

func HasUncommittedChanges(cwd string) (bool, error) {
	out, err := runGit([]string{"status", "--porcelain"}, cwd)
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

func StashChanges(cwd string) (bool, error) {
	out, err := runGit([]string{"stash", "-u"}, cwd)
	if err != nil {
		return false, err
	}
	return !strings.Contains(out, "No local changes to save"), nil
}

func PopStash(cwd string) (success bool, conflicted bool, err error) {
	_, err = runGit([]string{"stash", "pop"}, cwd)
	if err == nil {
		return true, false, nil
	}
	if strings.Contains(err.Error(), "CONFLICT") {
		return false, true, nil
	}
	return false, false, err
}

func CreateCheckpoint(cwd string) (string, error) {
	if _, err := runGit([]string{"add", "-A"}, cwd); err != nil {
		return "", err
	}
	if _, err := runGit([]string{"commit", "-m", "spotlight checkpoint"}, cwd); err != nil {
		return "", err
	}
	return runGit([]string{"rev-parse", "HEAD"}, cwd)
}

func AmendCheckpoint(cwd string) (string, error) {
	if _, err := runGit([]string{"add", "-A"}, cwd); err != nil {
		return "", err
	}
	if _, err := runGit([]string{"commit", "--amend", "--no-edit"}, cwd); err != nil {
		return "", err
	}
	return runGit([]string{"rev-parse", "HEAD"}, cwd)
}

func GetTreeSha(commitSha string, cwd string) (string, error) {
	return runGit([]string{"rev-parse", commitSha + "^{tree}"}, cwd)
}

func CheckoutCommit(sha string, cwd string) error {
	_, err := runGit([]string{"checkout", sha}, cwd)
	return err
}

func ForceCheckoutRef(ref string, cwd string) error {
	_, err := runGit([]string{"checkout", "--force", ref}, cwd)
	return err
}

func SoftReset(sha string, cwd string) error {
	_, err := runGit([]string{"reset", "--soft", sha}, cwd)
	return err
}
