package cmd

import (
	"github.com/spf13/cobra"
)

var version = "0.3.0"

var rootCmd = &cobra.Command{
	Use:     "spotlight",
	Short:   "Sync git worktree changes to the main repository as checkpoints",
	Version: version,
}

func Execute() error {
	return rootCmd.Execute()
}
