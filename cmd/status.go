package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/azranel/spotlight/internal/lock"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current spotlight sync status for this repository",
	Args:  cobra.NoArgs,
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().Bool("json", false, "Output machine-parseable JSON")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	jsonOut, _ := cmd.Flags().GetBool("json")

	ld, err := lock.Read(cwd)
	if err != nil {
		if jsonOut {
			return printJSON(map[string]any{"active": false})
		}
		fmt.Println("No active spotlight sync in this repository.")
		return nil
	}

	alive := lock.IsPidAlive(ld.PID)
	once := ld.Mode == lock.ModeOnce

	if jsonOut {
		return printJSON(map[string]any{
			"active":       once || alive,
			"stale":        !once && !alive,
			"mode":         ld.Mode,
			"pid":          ld.PID,
			"worktree":     ld.Worktree,
			"worktreePath": ld.WorktreePath,
			"startedAt":    ld.StartedAt,
			"originalRef":  ld.OriginalRef,
			"logPath":      ld.LogPath,
		})
	}

	var state string
	switch {
	case once:
		state = yellow + "synced (one-shot, no watcher)" + reset
	case alive:
		state = green + "running" + reset
	default:
		state = red + "stale (process not running)" + reset
	}

	fmt.Printf("%sSpotlight sync%s\n", bold, reset)
	fmt.Printf("  state:      %s\n", state)
	fmt.Printf("  worktree:   %s\n", ld.Worktree)
	if !once {
		fmt.Printf("  pid:        %d\n", ld.PID)
	}
	fmt.Printf("  started:    %s\n", ld.StartedAt)
	if ld.OriginalRef != "" {
		fmt.Printf("  pre-sync:   %s\n", ld.OriginalRef)
	}
	if ld.LogPath != "" {
		fmt.Printf("  logs:       %s\n", ld.LogPath)
	}
	if once {
		fmt.Printf("\nRun %sspotlight stop%s to restore your branch.\n", bold, reset)
	} else if !alive {
		fmt.Printf("\nRun %sspotlight stop%s to clean up and restore the repository.\n", bold, reset)
	}
	return nil
}

func printJSON(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
