package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var freezeCmd = &cobra.Command{
	Use:     "freeze [branch]",
	Aliases: []string{"fr"},
	Short:   "Protect a branch from modifications",
	Long:    `Mark a branch as frozen to prevent stack operations from modifying it. This is useful for protecting stable branches while working on dependent branches.`,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := ""
		if len(args) > 0 {
			branchName = args[0]
		}

		if err := runFreeze(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(freezeCmd)
}

func runFreeze(branchName string) error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Determine target branch
	if branchName == "" {
		var err error
		branchName, err = git.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
	}

	// Validate branch exists
	exists, err := git.BranchExists(branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("branch %s does not exist", branchName)
	}

	// Check if branch is tracked
	hasMetadata, err := stack.HasStackMetadata(branchName)
	if err != nil {
		return fmt.Errorf("failed to check stack metadata: %w", err)
	}
	if !hasMetadata {
		return fmt.Errorf("branch %s is not tracked", branchName)
	}

	// Check if already frozen
	isFrozen, err := stack.IsBranchFrozen(branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch is frozen: %w", err)
	}
	if isFrozen {
		ui.Warning(fmt.Sprintf("Branch %s is already frozen", branchName))
		return nil
	}

	// Freeze the branch
	if err := stack.FreezeBranch(branchName); err != nil {
		return fmt.Errorf("failed to freeze branch: %w", err)
	}

	ui.Success(fmt.Sprintf("Branch %s is now frozen", branchName))
	ui.Info("This branch will be protected from:")
	ui.Info("  - Modifications by stak modify")
	ui.Info("  - Rebases by stak sync")
	ui.Info("  - Parent changes by stak move")
	ui.Info("")
	ui.Info("Use 'stak unfreeze " + branchName + "' to allow modifications again")

	return nil
}
