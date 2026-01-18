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

var restoreCmd = &cobra.Command{
	Use:   "restore [pr-number]",
	Short: "Restore stack metadata from GitHub PR comments",
	Long: `Restores local stack metadata by reading it from GitHub PR comments.
This is useful when:
- You clone a repo and want to work on an existing stack
- You lost local metadata
- A teammate created a stack and you want to work on it`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var prNumber int
		if len(args) > 0 {
			fmt.Sscanf(args[0], "%d", &prNumber)
		}

		if err := runRestore(prNumber); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(prNumber int) error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Check if gh CLI is authenticated
	if !github.IsGHAuthenticated() {
		return fmt.Errorf("gh CLI not authenticated. Run: gh auth login")
	}

	// If no PR number provided, try to get it from current branch
	if prNumber == 0 {
		currentBranch, err := git.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}

		// Check if current branch has PR metadata
		existingPR, err := git.GetBranchPRNumber(currentBranch)
		if err != nil {
			return fmt.Errorf("failed to get PR number: %w", err)
		}

		if existingPR == 0 {
			return fmt.Errorf("no PR number provided and current branch has no associated PR. Usage: stak restore <pr-number>")
		}

		prNumber = existingPR
	}

	ui.Info(fmt.Sprintf("Fetching metadata from PR #%d", prNumber))

	// Get all comments from the PR
	comments, err := github.GetPRComments(prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR comments: %w", err)
	}

	// Find and parse the stack metadata comment
	var stackMetadata map[string]*struct {
		Name     string
		Parent   string
		PRNumber int
	}

	for _, comment := range comments {
		metadata, err := stack.ParseStackMetadata(comment)
		if err == nil {
			stackMetadata = make(map[string]*struct {
				Name     string
				Parent   string
				PRNumber int
			})
			for name, branch := range metadata {
				stackMetadata[name] = &struct {
					Name     string
					Parent   string
					PRNumber int
				}{
					Name:     branch.Name,
					Parent:   branch.Parent,
					PRNumber: branch.PRNumber,
				}
			}
			break
		}
	}

	if stackMetadata == nil {
		return fmt.Errorf("no stack metadata found in PR #%d comments. The stack comment may not have been created yet", prNumber)
	}

	// Write metadata to git config
	ui.Info(fmt.Sprintf("Restoring metadata for %d branch(es)", len(stackMetadata)))

	for _, branchInfo := range stackMetadata {
		if err := stack.WriteBranchMetadata(branchInfo.Name, branchInfo.Parent, branchInfo.PRNumber); err != nil {
			ui.Warning(fmt.Sprintf("Failed to write metadata for %s: %v", branchInfo.Name, err))
			continue
		}
		ui.Info(fmt.Sprintf("✓ Restored %s (parent: %s, PR: #%d)", branchInfo.Name, branchInfo.Parent, branchInfo.PRNumber))
	}

	ui.Success("Stack metadata restored successfully")
	ui.Info("You can now use stak up/down to navigate and stak sync to sync the stack")

	return nil
}
