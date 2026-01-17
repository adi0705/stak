package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var (
	modifyAmend      bool
	modifyRebaseNum  int
	modifyEditPR     bool
	modifyTitle      string
	modifyBody       string
	modifyPushOnly   bool
)

var modifyCmd = &cobra.Command{
	Use:   "modify",
	Short: "Modify current branch and sync children",
	Long: `Modify the current branch by amending commits or rebasing, then push changes
and sync all child branches. Optionally update the PR details.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runModify(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	modifyCmd.Flags().BoolVar(&modifyAmend, "amend", false, "Amend the last commit")
	modifyCmd.Flags().IntVar(&modifyRebaseNum, "rebase", 0, "Interactive rebase last N commits")
	modifyCmd.Flags().BoolVar(&modifyEditPR, "edit", false, "Edit PR title/body")
	modifyCmd.Flags().StringVar(&modifyTitle, "title", "", "New PR title")
	modifyCmd.Flags().StringVar(&modifyBody, "body", "", "New PR body")
	modifyCmd.Flags().BoolVar(&modifyPushOnly, "push-only", false, "Only push changes, skip syncing children")
	rootCmd.AddCommand(modifyCmd)
}

func runModify() error {
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

	// Handle amend
	if modifyAmend {
		ui.Info("Opening editor to amend last commit")
		cmd := exec.Command("git", "commit", "--amend")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to amend commit: %w", err)
		}
	}

	// Handle interactive rebase
	if modifyRebaseNum > 0 {
		ui.Info(fmt.Sprintf("Starting interactive rebase for last %d commits", modifyRebaseNum))
		cmd := exec.Command("git", "rebase", "-i", fmt.Sprintf("HEAD~%d", modifyRebaseNum))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to rebase: %w", err)
		}
	}

	// If no modification flags were provided, just push and sync
	if !modifyAmend && modifyRebaseNum == 0 && !modifyEditPR {
		ui.Info("No modification flags provided. Pushing current changes and syncing children.")
	}

	// Force push changes
	ui.Info(fmt.Sprintf("Force pushing %s", currentBranch))
	if err := git.Push(currentBranch, false, true); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	ui.Success(fmt.Sprintf("Pushed %s", currentBranch))

	// Edit PR if requested
	if modifyEditPR || modifyTitle != "" || modifyBody != "" {
		metadata, err := stack.ReadBranchMetadata(currentBranch)
		if err != nil {
			return fmt.Errorf("failed to read branch metadata: %w", err)
		}

		if metadata.PRNumber == 0 {
			ui.Warning("No PR associated with this branch")
		} else {
			ui.Info(fmt.Sprintf("Updating PR #%d", metadata.PRNumber))
			if err := github.EditPR(metadata.PRNumber, modifyTitle, modifyBody); err != nil {
				return fmt.Errorf("failed to edit PR: %w", err)
			}
			ui.Success(fmt.Sprintf("Updated PR #%d", metadata.PRNumber))
		}
	}

	// Sync children unless push-only flag is set
	if !modifyPushOnly {
		children, err := stack.GetChildren(currentBranch)
		if err != nil {
			return fmt.Errorf("failed to get children: %w", err)
		}

		if len(children) > 0 {
			ui.Info(fmt.Sprintf("Syncing %d child branch(es)", len(children)))

			// Fetch first
			if err := git.Fetch(); err != nil {
				return fmt.Errorf("failed to fetch: %w", err)
			}

			// Sync each child recursively
			for _, child := range children {
				if err := syncBranchRecursive(child); err != nil {
					return err
				}
			}

			// Return to original branch
			if err := git.CheckoutBranch(currentBranch); err != nil {
				return fmt.Errorf("failed to return to branch %s: %w", currentBranch, err)
			}
		}
	}

	ui.Success("Modify completed successfully")
	return nil
}
