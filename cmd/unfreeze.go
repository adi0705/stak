package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var unfreezeCmd = &cobra.Command{
	Use:     "unfreeze [branch]",
	Aliases: []string{"uf"},
	Short:   "Remove protection from a frozen branch",
	Long:    `Unfreeze a branch to allow stack operations to modify it again.`,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := ""
		if len(args) > 0 {
			branchName = args[0]
		}

		if err := runUnfreeze(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(unfreezeCmd)
}

func runUnfreeze(branchName string) error {
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

	// Check if frozen
	isFrozen, err := stack.IsBranchFrozen(branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch is frozen: %w", err)
	}
	if !isFrozen {
		ui.Warning(fmt.Sprintf("Branch %s is not frozen", branchName))
		return nil
	}

	// Unfreeze the branch
	if err := stack.UnfreezeBranch(branchName); err != nil {
		return fmt.Errorf("failed to unfreeze branch: %w", err)
	}

	ui.Success(fmt.Sprintf("Branch %s is now unfrozen", branchName))
	ui.Info("The branch can now be modified by stack operations")

	return nil
}
