package cmd

import (
	"fmt"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

// updateStackComments updates the stack visualization comment on all PRs in the stack
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
