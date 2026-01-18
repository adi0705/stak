package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Move to the parent branch in the stack",
	Long:  `Checkout the parent branch (move up one level in the stack).`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUp(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp() error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Get current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if branch has stack metadata
	hasMetadata, err := stack.HasStackMetadata(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to check stack metadata: %w", err)
	}
	if !hasMetadata {
		return fmt.Errorf("branch %s is not part of a stack", currentBranch)
	}

	// Get parent branch
	parent, err := stack.GetParent(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to get parent: %w", err)
	}

	if parent == "" {
		ui.Info(fmt.Sprintf("Branch %s is at the top of the stack", currentBranch))
		return nil
	}

	// Check if parent is also part of the stack
	parentHasMetadata, err := stack.HasStackMetadata(parent)
	if err != nil {
		return fmt.Errorf("failed to check parent metadata: %w", err)
	}

	if !parentHasMetadata {
		ui.Info(fmt.Sprintf("Branch %s is at the top of the stack (parent %s is not in stack)", currentBranch, parent))
		return nil
	}

	// Checkout parent branch
	ui.Info(fmt.Sprintf("Moving up: %s → %s", currentBranch, parent))
	if err := git.CheckoutBranch(parent); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", parent, err)
	}

	ui.Success(fmt.Sprintf("Now on branch %s", parent))
	return nil
}
