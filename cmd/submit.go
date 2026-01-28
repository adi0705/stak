package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var (
	submitStack      bool
	submitUpdateOnly bool
	submitDraft      bool
)

var submitCmd = &cobra.Command{
	Use:     "submit",
	Aliases: []string{"s"},
	Short:   "Create or update PRs in the stack",
	Long: `Push branches and create or update pull requests for the current branch or entire stack.
Does NOT merge PRs - use 'stak merge' to merge approved PRs.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSubmit(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	submitCmd.Flags().BoolVarP(&submitStack, "stack", "s", false, "Submit entire stack from current branch")
	submitCmd.Flags().BoolVarP(&submitUpdateOnly, "update-only", "u", false, "Only update existing PRs, don't create new")
	submitCmd.Flags().BoolVar(&submitDraft, "draft", false, "Create PRs as drafts")
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

	// If no metadata, branch was not created with stak create
	if !hasMetadata {
		return fmt.Errorf("branch %s is not part of a stack. Use 'stak create' to create stacked branches", currentBranch)
	}

	// Build list of branches to submit
	var branchesToSubmit []string
	if submitStack {
		// Submit entire chain: ancestors + current
		ancestors, err := stack.GetAncestors(currentBranch)
		if err != nil {
			return fmt.Errorf("failed to get ancestors: %w", err)
		}
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

	ui.Success("All PRs created/updated successfully")
	ui.Info("To merge approved PRs, run: stak merge")
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

	// Get the last commit message as PR title
	prTitle, err := getLastCommitMessage()
	if err != nil {
		return fmt.Errorf("failed to get commit message: %w", err)
	}

	ui.Info(fmt.Sprintf("Using commit message as PR title: %s", prTitle))

	// Push branch to remote
	ui.Info(fmt.Sprintf("Pushing branch %s to origin", branchName))
	if err := git.Push(branchName, true, false); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	// Create PR with the provided title and auto-filled body from commits
	ui.Info(fmt.Sprintf("Creating PR: %s → %s", branchName, parentBranch))

	// Pass title but empty body - body will be auto-filled from commits
	prNumber, err := github.CreatePR(parentBranch, branchName, prTitle, "", submitDraft)
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

func updateStackComments(branchName string) error {
	// Get all ancestors
	ancestors, err := stack.GetAncestors(branchName)
	if err != nil {
		return fmt.Errorf("failed to get ancestors: %w", err)
	}

	// Get all descendants
	descendants, err := stack.GetDescendants(branchName)
	if err != nil {
		return fmt.Errorf("failed to get descendants: %w", err)
	}

	// Build the full stack
	fullStack := append(ancestors, branchName)
	fullStack = append(fullStack, descendants...)

	// Update comment on each PR in the stack
	for _, branch := range fullStack {
		metadata, err := stack.ReadBranchMetadata(branch)
		if err != nil {
			continue
		}

		if metadata.PRNumber == 0 {
			continue
		}

		// Generate visualization for this branch
		visualization, err := stack.GenerateStackVisualization(branch)
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to generate visualization for %s: %v", branch, err))
			continue
		}

		// Post comment
		if err := github.CommentOnPR(metadata.PRNumber, visualization); err != nil {
			ui.Warning(fmt.Sprintf("Failed to comment on PR #%d: %v", metadata.PRNumber, err))
			continue
		}

		ui.Info(fmt.Sprintf("Updated stack comment on PR #%d", metadata.PRNumber))
	}

	return nil
}

// getLastCommitMessage returns the subject line of the last commit
func getLastCommitMessage() (string, error) {
	cmd := exec.Command("git", "log", "-1", "--pretty=%s")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit message: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func submitBranch(branch string) error {
	ui.Info(fmt.Sprintf("Processing branch %s", branch))

	// Get branch metadata
	metadata, err := stack.ReadBranchMetadata(branch)
	if err != nil {
		return fmt.Errorf("failed to read metadata for %s: %w", branch, err)
	}

	// If PR doesn't exist and --update-only flag is set, skip
	if metadata.PRNumber == 0 && submitUpdateOnly {
		ui.Warning(fmt.Sprintf("Skipping %s (no PR exists, --update-only specified)", branch))
		return nil
	}

	// If no PR exists, create it
	if metadata.PRNumber == 0 {
		// Checkout the branch first
		currentBranch, _ := git.GetCurrentBranch()
		if currentBranch != branch {
			if err := git.CheckoutBranch(branch); err != nil {
				return fmt.Errorf("failed to checkout %s: %w", branch, err)
			}
		}
		return createPRForBranch(branch)
	}

	// PR exists - push updates
	prNumber := metadata.PRNumber
	ui.Info(fmt.Sprintf("Updating PR #%d for branch %s", prNumber, branch))

	// Checkout the branch
	currentBranch, _ := git.GetCurrentBranch()
	if currentBranch != branch {
		if err := git.CheckoutBranch(branch); err != nil {
			return fmt.Errorf("failed to checkout %s: %w", branch, err)
		}
	}

	// Push latest changes (force push for existing PRs since commits may have been amended)
	ui.Info(fmt.Sprintf("Pushing %s to origin (force push)", branch))
	if err := git.Push(branch, false, true); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	ui.Success(fmt.Sprintf("Updated PR #%d", prNumber))

	// Update stack comments
	if err := updateStackComments(branch); err != nil {
		ui.Warning(fmt.Sprintf("Failed to update stack comments: %v", err))
	}

	return nil
}
