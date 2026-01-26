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

var (
	untrackForce     bool
	untrackRecursive bool
)

var untrackCmd = &cobra.Command{
	Use:     "untrack [branch]",
	Aliases: []string{"ut"},
	Short:   "Stop tracking a branch",
	Long:    `Remove a branch from stack tracking. This removes the branch's metadata but does not delete the branch or PR.`,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := ""
		if len(args) > 0 {
			branchName = args[0]
		}

		if err := runUntrack(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	untrackCmd.Flags().BoolVarP(&untrackForce, "force", "f", false, "Skip confirmation prompts")
	untrackCmd.Flags().BoolVarP(&untrackRecursive, "recursive", "r", false, "Recursively untrack all children")
	rootCmd.AddCommand(untrackCmd)
}

func runUntrack(branchName string) error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Determine target branch (argument or current)
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

	// Get branch info
	metadata, err := stack.ReadBranchMetadata(branchName)
	if err != nil {
		return fmt.Errorf("failed to read branch metadata: %w", err)
	}

	// Check for children
	children, err := stack.GetChildren(branchName)
	if err != nil {
		return fmt.Errorf("failed to get children: %w", err)
	}

	// If has children, warn and offer options
	if len(children) > 0 && !untrackRecursive {
		ui.Warning(fmt.Sprintf("Branch %s has %d child branch(es):", branchName, len(children)))
		for _, child := range children {
			fmt.Printf("  - %s\n", child)
		}

		if !untrackForce {
			prompt := promptui.Select{
				Label: "What would you like to do?",
				Items: []string{
					"Untrack only this branch (children will become orphaned)",
					"Untrack recursively (untrack this branch and all children)",
					"Cancel",
				},
			}

			_, result, err := prompt.Run()
			if err != nil || result == "Cancel" {
				ui.Info("Untrack cancelled")
				return nil
			}

			if result == "Untrack recursively (untrack this branch and all children)" {
				untrackRecursive = true
			}
		}
	}

	// Confirm untracking if not forced
	if !untrackForce && !untrackRecursive {
		ui.Info(fmt.Sprintf("Branch: %s", branchName))
		if metadata.Parent != "" {
			ui.Info(fmt.Sprintf("Parent: %s", metadata.Parent))
		}
		if metadata.PRNumber > 0 {
			ui.Info(fmt.Sprintf("PR: #%d", metadata.PRNumber))
		}

		prompt := promptui.Select{
			Label: "Untrack this branch?",
			Items: []string{"Yes", "No"},
		}

		_, result, err := prompt.Run()
		if err != nil || result == "No" {
			ui.Info("Untrack cancelled")
			return nil
		}
	}

	// Untrack recursively if requested
	if untrackRecursive && len(children) > 0 {
		ui.Info(fmt.Sprintf("Recursively untracking %d child branch(es)", len(children)))
		for _, child := range children {
			if err := untrackBranch(child); err != nil {
				ui.Warning(fmt.Sprintf("Failed to untrack %s: %v", child, err))
			} else {
				ui.Success(fmt.Sprintf("Untracked %s", child))
			}
		}
	}

	// Untrack the branch itself
	if err := untrackBranch(branchName); err != nil {
		return err
	}

	ui.Success(fmt.Sprintf("Untracked %s", branchName))

	// Show note about children if they weren't recursively untracked
	if len(children) > 0 && !untrackRecursive {
		ui.Info("Note: Child branches are no longer tracked in the stack")
		ui.Info("You can re-track them with: stak track <branch>")
	}

	return nil
}

func untrackBranch(branch string) error {
	return stack.DeleteBranchMetadata(branch)
}
