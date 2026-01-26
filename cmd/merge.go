package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var (
	mergeAll        bool
	mergeMethod     string
	mergeSkipChecks bool
)

var mergeCmd = &cobra.Command{
	Use:     "merge",
	Aliases: []string{"mg"},
	Short:   "Merge PRs in the stack",
	Long: `Merge approved PRs in the correct order (bottom to top).
After each merge, updates dependent PRs to point to the new base and rebases children.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runMerge(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	mergeCmd.Flags().BoolVar(&mergeAll, "all", false, "Merge entire stack from current branch")
	mergeCmd.Flags().StringVar(&mergeMethod, "method", "squash", "Merge method: squash, merge, or rebase")
	mergeCmd.Flags().BoolVar(&mergeSkipChecks, "skip-checks", false, "Skip approval and CI checks")
	rootCmd.AddCommand(mergeCmd)
}

func runMerge() error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Check if gh CLI is authenticated
	if !github.IsGHAuthenticated() {
		return fmt.Errorf("gh CLI not authenticated. Run: gh auth login")
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

	// Get branch metadata
	metadata, err := stack.ReadBranchMetadata(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	if metadata.PRNumber == 0 {
		return fmt.Errorf("branch %s has no associated PR", currentBranch)
	}

	// Build ancestor chain
	ancestors, err := stack.GetAncestors(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to get ancestors: %w", err)
	}

	// Build list of branches to merge
	var branchesToMerge []string
	if mergeAll {
		// Merge entire chain: ancestors + current
		branchesToMerge = append(ancestors, currentBranch)
	} else {
		// Merge only current branch
		branchesToMerge = []string{currentBranch}
	}

	ui.Info(fmt.Sprintf("Merging %d PR(s)", len(branchesToMerge)))

	// Fetch latest
	if err := git.Fetch(); err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	// Merge each branch in order
	for _, branch := range branchesToMerge {
		if err := mergeBranch(branch); err != nil {
			return err
		}
	}

	ui.Success("All PRs merged successfully")
	return nil
}

func mergeBranch(branch string) error {
	ui.Info(fmt.Sprintf("Processing branch %s", branch))

	// Get branch metadata
	metadata, err := stack.ReadBranchMetadata(branch)
	if err != nil {
		return fmt.Errorf("failed to read metadata for %s: %w", branch, err)
	}

	if metadata.PRNumber == 0 {
		return fmt.Errorf("branch %s has no associated PR", branch)
	}

	prNumber := metadata.PRNumber

	// Check PR status
	ui.Info(fmt.Sprintf("Checking status of PR #%d", prNumber))
	status, err := github.GetPRStatus(prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR status: %w", err)
	}

	// Check if already merged
	if status.IsMerged() {
		ui.Warning(fmt.Sprintf("PR #%d is already merged", prNumber))
		return nil
	}

	// Check if open
	if !status.IsOpen() {
		return fmt.Errorf("PR #%d is not open (state: %s)", prNumber, status.State)
	}

	// Verify approval and CI unless skipping checks
	if !mergeSkipChecks {
		if !status.IsApproved() {
			return fmt.Errorf("PR #%d is not approved", prNumber)
		}

		if !status.IsCIPassing() {
			return fmt.Errorf("PR #%d has failing CI checks", prNumber)
		}
	}

	// Merge the PR
	ui.Info(fmt.Sprintf("Merging PR #%d", prNumber))
	if err := github.MergePR(prNumber, mergeMethod); err != nil {
		return fmt.Errorf("failed to merge PR #%d: %w", prNumber, err)
	}

	ui.Success(fmt.Sprintf("Merged PR #%d", prNumber))

	// Get the parent branch (which is now the new base for children)
	newBase := metadata.Parent

	// Get children of this branch
	children, err := stack.GetChildren(branch)
	if err != nil {
		return fmt.Errorf("failed to get children of %s: %w", branch, err)
	}

	// Update each child
	for _, child := range children {
		if err := updateChildAfterMerge(child, branch, newBase); err != nil {
			return err
		}
	}

	// Delete local branch
	ui.Info(fmt.Sprintf("Deleting local branch %s", branch))
	currentBranch, _ := git.GetCurrentBranch()
	if currentBranch == branch {
		// Switch to parent branch first
		if newBase != "" {
			if err := git.CheckoutBranch(newBase); err != nil {
				ui.Warning(fmt.Sprintf("Could not checkout %s: %v", newBase, err))
			}
		}
	}

	if err := git.DeleteBranch(branch, false); err != nil {
		ui.Warning(fmt.Sprintf("Could not delete branch %s: %v", branch, err))
	}

	// Delete metadata
	if err := stack.DeleteBranchMetadata(branch); err != nil {
		ui.Warning(fmt.Sprintf("Could not delete metadata for %s: %v", branch, err))
	}

	return nil
}

func updateChildAfterMerge(child, oldParent, newParent string) error {
	ui.Info(fmt.Sprintf("Updating child branch %s (parent: %s → %s)", child, oldParent, newParent))

	// Get child metadata
	childMetadata, err := stack.ReadBranchMetadata(child)
	if err != nil {
		return fmt.Errorf("failed to read metadata for %s: %w", child, err)
	}

	// Checkout child branch
	if err := git.CheckoutBranch(child); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", child, err)
	}

	// Rebase onto new parent
	ui.Info(fmt.Sprintf("Rebasing %s onto origin/%s", child, newParent))
	onto := fmt.Sprintf("origin/%s", newParent)
	if err := git.RebaseOnto(onto); err != nil {
		if conflictErr, ok := err.(*git.RebaseConflictError); ok {
			return handleRebaseConflict(child, conflictErr)
		}
		return fmt.Errorf("failed to rebase %s: %w", child, err)
	}

	// Force push
	ui.Info(fmt.Sprintf("Force pushing %s", child))
	if err := git.Push(child, false, true); err != nil {
		return fmt.Errorf("failed to push %s: %w", child, err)
	}

	// Update PR base on GitHub
	if childMetadata.PRNumber > 0 {
		ui.Info(fmt.Sprintf("Updating PR #%d base to %s", childMetadata.PRNumber, newParent))
		if err := github.UpdatePRBase(childMetadata.PRNumber, newParent); err != nil {
			return fmt.Errorf("failed to update PR base: %w", err)
		}
	}

	// Update metadata
	if err := stack.WriteBranchMetadata(child, newParent, childMetadata.PRNumber); err != nil {
		return fmt.Errorf("failed to update metadata for %s: %w", child, err)
	}

	ui.Success(fmt.Sprintf("Updated child branch %s", child))
	return nil
}
