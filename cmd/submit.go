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
	submitAll        bool
	submitMergeMethod string
	submitSkipChecks bool
)

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit and merge PRs in the stack",
	Long: `Submit PRs in the correct order, merging from bottom to top.
After each merge, updates dependent PRs to point to the new base.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSubmit(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	submitCmd.Flags().BoolVar(&submitAll, "all", false, "Submit entire stack from current branch")
	submitCmd.Flags().StringVar(&submitMergeMethod, "method", "squash", "Merge method: squash, merge, or rebase")
	submitCmd.Flags().BoolVar(&submitSkipChecks, "skip-checks", false, "Skip approval and CI checks")
	rootCmd.AddCommand(submitCmd)
}

func runSubmit() error {
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

	// If no metadata, this is a new branch - create the PR first
	if !hasMetadata {
		return createPRForBranch(currentBranch)
	}

	// Build ancestor chain
	ancestors, err := stack.GetAncestors(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to get ancestors: %w", err)
	}

	// Build list of branches to submit
	var branchesToSubmit []string
	if submitAll {
		// Submit entire chain: ancestors + current
		branchesToSubmit = append(ancestors, currentBranch)
	} else {
		// Submit only current branch
		branchesToSubmit = []string{currentBranch}
	}

	ui.Info(fmt.Sprintf("Submitting %d branch(es)", len(branchesToSubmit)))

	// Fetch latest
	if err := git.Fetch(); err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	// Submit each branch in order
	for _, branch := range branchesToSubmit {
		if err := submitBranch(branch); err != nil {
			return err
		}
	}

	ui.Success("All PRs submitted successfully")
	return nil
}

func createPRForBranch(branchName string) error {
	// Read metadata to get parent branch
	metadata, err := stack.ReadBranchMetadata(branchName)
	if err != nil {
		return fmt.Errorf("failed to read metadata for %s: %w", branchName, err)
	}

	parentBranch := metadata.Parent
	if parentBranch == "" {
		return fmt.Errorf("no parent branch found in metadata for %s", branchName)
	}

	// Check if there are any commits on this branch
	hasCommits, err := git.HasCommits()
	if err != nil {
		return fmt.Errorf("failed to check for commits: %w", err)
	}

	if !hasCommits {
		return fmt.Errorf("no commits on branch %s. Make some commits first", branchName)
	}

	// Push branch to remote
	ui.Info(fmt.Sprintf("Pushing branch %s to origin", branchName))
	if err := git.Push(branchName, true, false); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	// Create PR with auto-filled title and body from commits
	ui.Info(fmt.Sprintf("Creating PR: %s → %s", branchName, parentBranch))
	ui.Info("PR title and description will be generated from your commits...")

	// Use empty title and body - CreatePR will use --fill-first
	prNumber, err := github.CreatePR(parentBranch, branchName, "", "", false)
	if err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}

	// Update metadata with PR number
	if err := stack.WriteBranchMetadata(branchName, parentBranch, prNumber); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	// Get PR URL
	prURL, err := github.GetPRURL(prNumber)
	if err != nil {
		// Don't fail, just show PR number
		ui.Success(fmt.Sprintf("Created PR #%d", prNumber))
	} else {
		ui.Success(fmt.Sprintf("Created PR #%d: %s", prNumber, prURL))
	}

	return nil
}

func submitBranch(branch string) error {
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
	if !submitSkipChecks {
		if !status.IsApproved() {
			return fmt.Errorf("PR #%d is not approved", prNumber)
		}

		if !status.IsCIPassing() {
			return fmt.Errorf("PR #%d has failing CI checks", prNumber)
		}
	}

	// Merge the PR
	ui.Info(fmt.Sprintf("Merging PR #%d", prNumber))
	if err := github.MergePR(prNumber, submitMergeMethod); err != nil {
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
