package cmd

import (
	"fmt"
	"strings"

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

		// Check if a stack comment already exists
		comments, err := github.GetPRCommentsWithIDs(metadata.PRNumber)
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to get comments for PR #%d: %v", metadata.PRNumber, err))
			continue
		}

		// Look for existing stack comment (contains stak-metadata marker)
		var existingCommentID string
		for _, comment := range comments {
			if containsStackMetadata(comment.Body) {
				existingCommentID = comment.ID
				break
			}
		}

		// Update existing comment or create new one
		if existingCommentID != "" {
			if err := github.UpdateComment(existingCommentID, visualization); err != nil {
				ui.Warning(fmt.Sprintf("Failed to update comment on PR #%d: %v", metadata.PRNumber, err))
				continue
			}
			ui.Info(fmt.Sprintf("Updated stack comment on PR #%d", metadata.PRNumber))
		} else {
			if err := github.CommentOnPR(metadata.PRNumber, visualization); err != nil {
				ui.Warning(fmt.Sprintf("Failed to create comment on PR #%d: %v", metadata.PRNumber, err))
				continue
			}
			ui.Info(fmt.Sprintf("Created stack comment on PR #%d", metadata.PRNumber))
		}
	}

	return nil
}

// containsStackMetadata checks if a comment body contains stack metadata or is a stack comment
func containsStackMetadata(body string) bool {
	// Check for the new format with metadata
	if strings.Contains(body, "<!-- stak-metadata") {
		return true
	}
	// Check for stack comments (old or new format)
	return strings.Contains(body, "## 📚 Stack") && strings.Contains(body, "This stack is managed by")
}
