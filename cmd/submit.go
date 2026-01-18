package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Push changes and update PR",
	Long: `Push the current branch changes to remote and update the PR.

If no PR exists yet, creates one. If a PR already exists, pushes the latest changes and updates the stack visualization.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSubmit(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
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

	// If no metadata, branch was not created with stak create
	if !hasMetadata {
		return createPRForBranch(currentBranch)
		return fmt.Errorf("branch %s is not part of a stack. Use 'stak create' to create stacked branches", currentBranch)
	}

	// Check if PR already exists
	metadata, err := stack.ReadBranchMetadata(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to read metadata for %s: %w", currentBranch, err)
	}

	// If no PR yet, create it
	if metadata.PRNumber == 0 {
		return createPRForBranch(currentBranch)
	}

	// Check if any parent branches have been merged on GitHub
	// If so, user needs to sync first
	if err := checkForMergedAncestors(currentBranch); err != nil {
		return err
	}

	// PR exists, ensure single commit before pushing
	// Count commits on this branch compared to parent
	commitCount, err := git.CountCommits(metadata.Parent)
	if err != nil {
		return fmt.Errorf("failed to count commits: %w", err)
	}

	// If more than one commit, squash them into one
	if commitCount > 1 {
		ui.Info(fmt.Sprintf("Found %d commits, squashing into one", commitCount))
		if err := git.SquashCommits(metadata.Parent); err != nil {
			return fmt.Errorf("failed to squash commits: %w", err)
		}
		ui.Success("Commits squashed into one")
	}

	// Push changes
	ui.Info(fmt.Sprintf("Pushing %s to origin", currentBranch))
	if err := git.Push(currentBranch, false, true); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	ui.Success(fmt.Sprintf("Pushed %s", currentBranch))

	// Update stack comments on GitHub
	ui.Info("Updating stack comments on GitHub")
	if err := updateStackComments(currentBranch); err != nil {
		ui.Warning(fmt.Sprintf("Failed to update stack comments: %v", err))
		// Don't fail the whole operation if comments fail
	}

	ui.Success("Submit completed successfully")
	ui.Info(fmt.Sprintf("PR #%d has been updated with your changes", metadata.PRNumber))
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

	// Prompt for PR title
	fmt.Print("Enter PR title: ")
	reader := bufio.NewReader(os.Stdin)
	prTitle, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read PR title: %w", err)
	}
	prTitle = strings.TrimSpace(prTitle)

	if prTitle == "" {
		return fmt.Errorf("PR title cannot be empty")
	}

	// Ensure single commit before creating PR
	commitCount, err := git.CountCommits(parentBranch)
	if err != nil {
		return fmt.Errorf("failed to count commits: %w", err)
	}

	// If more than one commit, squash them into one
	if commitCount > 1 {
		ui.Info(fmt.Sprintf("Found %d commits, squashing into one", commitCount))
		if err := git.SquashCommits(parentBranch); err != nil {
			return fmt.Errorf("failed to squash commits: %w", err)
		}
		ui.Success("Commits squashed into one")
	}

	// Push branch to remote
	ui.Info(fmt.Sprintf("Pushing branch %s to origin", branchName))
	if err := git.Push(branchName, true, false); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	// Create PR with the provided title and auto-filled body from commits
	ui.Info(fmt.Sprintf("Creating PR: %s → %s", branchName, parentBranch))

	// Pass title but empty body - body will be auto-filled from commits
	prNumber, err := github.CreatePR(parentBranch, branchName, prTitle, "", false)
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

	// Post stack visualization to all PRs in the stack
	if err := updateStackComments(branchName); err != nil {
		ui.Warning(fmt.Sprintf("Failed to update stack comments: %v", err))
		// Don't fail the whole operation if comments fail
	}

	return nil
}

// checkForMergedAncestors checks if any parent branches have merged PRs
// and warns the user to sync first
func checkForMergedAncestors(branch string) error {
	// Get all ancestors
	ancestors, err := stack.GetAncestors(branch)
	if err != nil {
		// If we can't get ancestors, just continue - don't fail
		return nil
	}

	// Check each ancestor for merged PR
	var mergedBranches []string
	for _, ancestor := range ancestors {
		// Check if branch still exists locally
		exists, err := git.BranchExists(ancestor)
		if err != nil || !exists {
			continue
		}

		// Get metadata
		metadata, err := stack.ReadBranchMetadata(ancestor)
		if err != nil || metadata.PRNumber == 0 {
			continue
		}

		// Check PR status on GitHub
		status, err := github.GetPRStatus(metadata.PRNumber)
		if err != nil {
			// If we can't get status, skip this branch
			continue
		}

		// If PR is merged, add to list
		if status.IsMerged() {
			mergedBranches = append(mergedBranches, fmt.Sprintf("%s (PR #%d)", ancestor, metadata.PRNumber))
		}
	}

	// If any ancestors are merged, warn user
	if len(mergedBranches) > 0 {
		ui.Warning("⚠️  Stack is out of sync!")
		fmt.Println("\nThe following parent branches have been merged on GitHub:")
		for _, branch := range mergedBranches {
			fmt.Printf("  • %s\n", branch)
		}
		fmt.Println("\nYou need to sync first to update your stack:")
		fmt.Println("  stak sync")
		fmt.Println("\nThis will:")
		fmt.Println("  • Delete merged branches locally")
		fmt.Println("  • Update your branch to point to the new parent")
		fmt.Println("  • Rebase your changes onto the latest code")
		return fmt.Errorf("stack out of sync - run 'stak sync' first")
	}

	return nil
}

