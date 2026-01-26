package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var upCmd = &cobra.Command{
	Use:     "up [steps]",
	Aliases: []string{"u"},
	Short:   "Move to parent branch",
	Long:    `Switch to the parent branch of the current branch in the stack. Optionally specify number of steps to traverse up.`,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		steps := 1
		if len(args) > 0 {
			var err error
			steps, err = strconv.Atoi(args[0])
			if err != nil || steps < 1 {
				ui.Error("steps must be a positive integer")
				os.Exit(1)
			}
		}
		if err := runUp(steps); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(steps int) error {
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

	// Traverse up the specified number of steps
	targetBranch := currentBranch
	for i := 0; i < steps; i++ {
		// Get parent branch
		parent, err := stack.GetParent(targetBranch)
		if err != nil {
			return fmt.Errorf("failed to get parent: %w", err)
		}

		if parent == "" {
			if i == 0 {
				return fmt.Errorf("branch %s has no parent", targetBranch)
			}
			ui.Warning(fmt.Sprintf("Reached top of stack after %d step(s)", i))
			break
		}

		// Check if parent branch exists
		parentExists, err := git.BranchExists(parent)
		if err != nil {
			return fmt.Errorf("failed to check if parent exists: %w", err)
		}
		if !parentExists {
			return fmt.Errorf("parent branch %s does not exist locally", parent)
		}

		// Check if parent is part of the stack (has metadata)
		parentHasMetadata, err := stack.HasStackMetadata(parent)
		if err != nil {
			return fmt.Errorf("failed to check parent metadata: %w", err)
		}

		// Warn if parent is not tracked, but allow navigation
		if !parentHasMetadata {
			ui.Warning(fmt.Sprintf("Parent %s is not tracked in stack", parent))
			ui.Info(fmt.Sprintf("Consider running: stak track %s", parent))
		}

		targetBranch = parent
	}

	// Switch to target branch
	if targetBranch != currentBranch {
		ui.Info(fmt.Sprintf("Moving from %s to %s", currentBranch, targetBranch))
		if err := git.CheckoutBranch(targetBranch); err != nil {
			return fmt.Errorf("failed to checkout branch %s: %w", targetBranch, err)
		}
		ui.Success(fmt.Sprintf("Now on branch %s", targetBranch))
	}

	return nil
}
