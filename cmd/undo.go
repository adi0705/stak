package cmd

import (
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/history"
	"stacking/internal/ui"
)

var undoForce bool

var undoCmd = &cobra.Command{
	Use:     "undo",
	Aliases: []string{"un"},
	Short:   "View and undo recent stack operations",
	Long:    `Display recent stack operations and their details. Note: Actual undo functionality is limited - this command shows operation history and provides guidance for manual reversal.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUndo(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	undoCmd.Flags().BoolVarP(&undoForce, "force", "f", false, "Skip confirmation")
	rootCmd.AddCommand(undoCmd)
}

func runUndo() error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Get last operation
	lastOp, err := history.GetLastOperation()
	if err != nil {
		return fmt.Errorf("failed to get operation history: %w", err)
	}

	// Display operation details
	ui.Info("Last operation:")
	fmt.Printf("  Command:     %s\n", lastOp.Command)
	fmt.Printf("  Branch:      %s\n", lastOp.Branch)
	fmt.Printf("  Description: %s\n", lastOp.Description)
	fmt.Printf("  Timestamp:   %s\n", lastOp.Timestamp.Format("2006-01-02 15:04:05"))

	if len(lastOp.Metadata) > 0 {
		ui.Info("\nOperation metadata:")
		for key, value := range lastOp.Metadata {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	ui.Info("")
	ui.Warning("Note: Automatic undo is not yet fully implemented.")
	ui.Info("To manually undo this operation:")

	// Provide guidance based on operation type
	switch lastOp.Command {
	case "create":
		ui.Info("  1. Delete the branch: git branch -D " + lastOp.Branch)
		ui.Info("  2. Untrack: stak untrack " + lastOp.Branch)

	case "move":
		if oldParent, ok := lastOp.Metadata["old_parent"].(string); ok {
			ui.Info("  1. Move back to original parent: stak move " + lastOp.Branch + " --parent " + oldParent)
		}

	case "fold":
		ui.Info("  This operation cannot be automatically undone.")
		ui.Info("  You may need to restore from git reflog or a backup.")

	case "squash":
		ui.Info("  1. Find the original commits in reflog: git reflog " + lastOp.Branch)
		ui.Info("  2. Reset to the pre-squash state: git reset --hard <commit-hash>")
		ui.Info("  3. Force push: git push --force-with-lease")

	case "split":
		if newBranch, ok := lastOp.Metadata["new_branch"].(string); ok {
			ui.Info("  1. Delete the new branch: git branch -D " + newBranch)
			ui.Info("  2. Cherry-pick commits back to original: git cherry-pick <commits>")
		}

	case "reorder":
		ui.Info("  Run 'stak reorder' again to restore the original order.")

	case "submit":
		if prNumber, ok := lastOp.Metadata["pr_number"]; ok {
			ui.Info(fmt.Sprintf("  1. Close the PR: gh pr close %v", prNumber))
		}

	case "merge":
		ui.Info("  This operation cannot be undone (PR already merged).")

	default:
		ui.Info("  No specific undo guidance available for this operation.")
	}

	ui.Info("")

	// Confirm removal from history
	if !undoForce {
		prompt := promptui.Select{
			Label: "Remove this operation from history?",
			Items: []string{"Yes", "No"},
		}

		_, result, err := prompt.Run()
		if err != nil || result == "No" {
			ui.Info("Operation kept in history")
			return nil
		}
	}

	// Remove the operation from log
	if err := history.RemoveLastOperation(); err != nil {
		return fmt.Errorf("failed to remove operation from history: %w", err)
	}

	ui.Success("Operation removed from history")
	ui.Info("Use 'stak undo' again to see the previous operation")

	return nil
}
