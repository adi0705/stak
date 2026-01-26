package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var (
	squashMessage string
)

var squashCmd = &cobra.Command{
	Use:     "squash [branch]",
	Aliases: []string{"sq"},
	Short:   "Squash all commits in a branch",
	Long:    `Consolidate all commits in a branch into a single commit. Useful for cleaning up commit history before merging.`,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := ""
		if len(args) > 0 {
			branchName = args[0]
		}

		if err := runSquash(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	squashCmd.Flags().StringVarP(&squashMessage, "message", "m", "", "Commit message for squashed commit")
	rootCmd.AddCommand(squashCmd)
}

func runSquash(branchName string) error {
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

	// Get metadata
	metadata, err := stack.ReadBranchMetadata(branchName)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	parent := metadata.Parent
	if parent == "" {
		return fmt.Errorf("branch %s has no parent (is a root branch)", branchName)
	}

	// Checkout the branch
	currentBranch, _ := git.GetCurrentBranch()
	if currentBranch != branchName {
		ui.Info(fmt.Sprintf("Checking out %s", branchName))
		if err := git.CheckoutBranch(branchName); err != nil {
			return fmt.Errorf("failed to checkout branch: %w", err)
		}
	}

	// Count commits
	commitCount, err := getCommitCount(branchName, parent)
	if err != nil {
		return fmt.Errorf("failed to count commits: %w", err)
	}

	if commitCount <= 1 {
		ui.Info(fmt.Sprintf("Branch %s has only %d commit. Nothing to squash.", branchName, commitCount))
		return nil
	}

	ui.Info(fmt.Sprintf("Squashing %d commits on %s", commitCount, branchName))

	// Reset to parent (soft reset keeps changes staged)
	ui.Info(fmt.Sprintf("Resetting to %s (keeping changes)", parent))
	cmd := exec.Command("git", "reset", "--soft", parent)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to reset: %s", string(output))
	}

	// Create commit message
	var commitMsg string
	if squashMessage != "" {
		commitMsg = squashMessage
	} else {
		// Use interactive editor for commit message
		ui.Info("Opening editor for commit message")
		commitCmd := exec.Command("git", "commit")
		commitCmd.Stdin = os.Stdin
		commitCmd.Stdout = os.Stdout
		commitCmd.Stderr = os.Stderr
		if err := commitCmd.Run(); err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}
	}

	// If message was provided via flag, commit with it
	if squashMessage != "" {
		if err := git.Commit(commitMsg); err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}
	}

	ui.Success(fmt.Sprintf("Squashed %d commits into 1", commitCount))

	// Force push
	ui.Info(fmt.Sprintf("Force pushing %s", branchName))
	if err := git.Push(branchName, false, true); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// Get children
	children, err := stack.GetChildren(branchName)
	if err != nil {
		return fmt.Errorf("failed to get children: %w", err)
	}

	// Rebase children
	if len(children) > 0 {
		ui.Info(fmt.Sprintf("Syncing %d child branch(es)", len(children)))
		for _, child := range children {
			if err := syncBranchRecursive(child); err != nil {
				return fmt.Errorf("failed to sync child %s: %w", child, err)
			}
		}

		// Return to original branch
		if err := git.CheckoutBranch(branchName); err != nil {
			return fmt.Errorf("failed to return to branch: %w", err)
		}
	}

	ui.Success(fmt.Sprintf("Squashed commits on %s", branchName))
	return nil
}
