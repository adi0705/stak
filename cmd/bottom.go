package cmd

import (
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var bottomCmd = &cobra.Command{
	Use:     "bottom",
	Aliases: []string{"b"},
	Short:   "Move to bottommost branch in stack",
	Long:    `Navigate to the bottommost branch in the current stack by following the child chain. If multiple children exist at any level, shows a menu to select one.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runBottom(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(bottomCmd)
}

func runBottom() error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Get current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Follow children to find bottommost branch
	bottomBranch := currentBranch
	for {
		children, err := stack.GetChildren(bottomBranch)
		if err != nil {
			return fmt.Errorf("failed to get children: %w", err)
		}

		// If no children, we've reached the bottom
		if len(children) == 0 {
			break
		}

		// If only one child, follow it automatically
		if len(children) == 1 {
			bottomBranch = children[0]
		} else {
			// Multiple children - show selection menu
			prompt := promptui.Select{
				Label: "Multiple child branches found. Select path to bottom",
				Items: children,
			}

			_, result, err := prompt.Run()
			if err != nil {
				return fmt.Errorf("branch selection cancelled: %w", err)
			}
			bottomBranch = result
		}
	}

	// If we're already at the bottom, inform the user
	if bottomBranch == currentBranch {
		ui.Info("Already at bottom of stack")
		return nil
	}

	// Switch to bottom branch
	ui.Info(fmt.Sprintf("Moving from %s to %s", currentBranch, bottomBranch))
	if err := git.CheckoutBranch(bottomBranch); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", bottomBranch, err)
	}

	ui.Success(fmt.Sprintf("Now on branch %s (bottom of stack)", bottomBranch))
	return nil
}
