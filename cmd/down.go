package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var downCmd = &cobra.Command{
	Use:     "down [steps]",
	Aliases: []string{"d"},
	Short:   "Move to child branch",
	Long:    `Switch to a child branch of the current branch in the stack. If multiple children exist, shows a menu to select one. Optionally specify number of steps to traverse down.`,
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
		if err := runDown(steps); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown(steps int) error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Get current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Traverse down the specified number of steps
	targetBranch := currentBranch
	for i := 0; i < steps; i++ {
		// Get children branches
		children, err := stack.GetChildren(targetBranch)
		if err != nil {
			return fmt.Errorf("failed to get children: %w", err)
		}

		if len(children) == 0 {
			if i == 0 {
				return fmt.Errorf("branch %s has no child branches", targetBranch)
			}
			ui.Warning(fmt.Sprintf("Reached bottom of stack after %d step(s)", i))
			break
		}

		// If only one child, use it directly
		if len(children) == 1 {
			targetBranch = children[0]
		} else {
			// Multiple children - show selection menu
			prompt := promptui.Select{
				Label: fmt.Sprintf("Select child branch (step %d of %d)", i+1, steps),
				Items: children,
			}

			_, result, err := prompt.Run()
			if err != nil {
				return fmt.Errorf("branch selection cancelled: %w", err)
			}
			targetBranch = result
		}
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
