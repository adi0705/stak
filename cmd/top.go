package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var topCmd = &cobra.Command{
	Use:     "top",
	Aliases: []string{"t"},
	Short:   "Move to topmost branch in stack",
	Long:    `Navigate to the topmost branch in the current stack by following the parent chain.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runTop(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(topCmd)
}

func runTop() error {
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

	// Follow parent chain to find topmost branch
	topBranch := currentBranch
	for {
		parent, err := stack.GetParent(topBranch)
		if err != nil {
			return fmt.Errorf("failed to get parent: %w", err)
		}

		// If no parent or parent is not tracked, we've reached the top
		if parent == "" {
			break
		}

		// Check if parent has metadata (is tracked)
		parentHasMetadata, err := stack.HasStackMetadata(parent)
		if err != nil {
			return fmt.Errorf("failed to check parent metadata: %w", err)
		}

		if !parentHasMetadata {
			// Parent is not tracked (likely a base branch), stop here
			break
		}

		topBranch = parent
	}

	// If we're already at the top, inform the user
	if topBranch == currentBranch {
		ui.Info("Already at top of stack")
		return nil
	}

	// Switch to top branch
	ui.Info(fmt.Sprintf("Moving from %s to %s", currentBranch, topBranch))
	if err := git.CheckoutBranch(topBranch); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", topBranch, err)
	}

	ui.Success(fmt.Sprintf("Now on branch %s (top of stack)", topBranch))
	return nil
}
