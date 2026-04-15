package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/azranel/spotlight/internal/git"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all git worktrees and their names",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !git.IsGitRoot("") {
			fmt.Fprintln(os.Stderr, "Error: Not in a git repository root directory.")
			os.Exit(1)
		}

		worktrees, err := git.ListWorktrees("")
		if err != nil {
			return err
		}

		// Filter non-bare worktrees
		var linked []git.Worktree
		for _, wt := range worktrees {
			if !wt.Bare {
				linked = append(linked, wt)
			}
		}

		if len(linked) <= 1 {
			fmt.Println("No linked worktrees found.")
			fmt.Println(`Use "git worktree add <path> <branch>" to create a worktree.`)
			return nil
		}

		// Calculate column widths
		nameWidth := 4
		branchWidth := 6
		for i, wt := range linked {
			if i == 0 {
				continue // skip main
			}
			name := filepath.Base(wt.Path)
			if len(name) > nameWidth {
				nameWidth = len(name)
			}
		}
		for _, wt := range linked {
			branch := branchDisplay(wt.Branch)
			if len(branch) > branchWidth {
				branchWidth = len(branch)
			}
		}

		fmt.Printf("%-*s  %-*s  %s\n", nameWidth, "NAME", branchWidth, "BRANCH", "PATH")

		for i, wt := range linked {
			name := filepath.Base(wt.Path)
			if i == 0 {
				name = "(main)"
			}
			branch := branchDisplay(wt.Branch)
			fmt.Printf("%-*s  %-*s  %s\n", nameWidth, name, branchWidth, branch, wt.Path)
		}

		return nil
	},
}

func branchDisplay(branch string) string {
	if branch == "" {
		return "detached"
	}
	return strings.TrimPrefix(branch, "refs/heads/")
}

func init() {
	rootCmd.AddCommand(listCmd)
}
