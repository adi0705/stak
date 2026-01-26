package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var reorderCmd = &cobra.Command{
	Use:     "reorder",
	Aliases: []string{"ro"},
	Short:   "Reorder branches in the stack",
	Long:    `Interactively reorder the branches in a stack by changing their parent relationships.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runReorder(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(reorderCmd)
}

func runReorder() error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Get current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if branch is tracked
	hasMetadata, err := stack.HasStackMetadata(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to check stack metadata: %w", err)
	}
	if !hasMetadata {
		return fmt.Errorf("branch %s is not tracked", currentBranch)
	}

	// Get the full stack path from root to current
	ancestors, err := stack.GetAncestors(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to get ancestors: %w", err)
	}

	// Build the stack: ancestors + current
	stackBranches := append(ancestors, currentBranch)

	// Get descendants
	descendants, err := stack.GetDescendants(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to get descendants: %w", err)
	}
	stackBranches = append(stackBranches, descendants...)

	if len(stackBranches) <= 2 {
		return fmt.Errorf("stack has only %d branch(es), nothing to reorder", len(stackBranches))
	}

	// Display current order
	ui.Info("Current stack order:")
	for i, branch := range stackBranches {
		metadata, _ := stack.ReadBranchMetadata(branch)
		parentInfo := ""
		if metadata != nil && metadata.Parent != "" {
			parentInfo = fmt.Sprintf(" (parent: %s)", metadata.Parent)
		}
		fmt.Printf("  %d. %s%s\n", i+1, branch, parentInfo)
	}

	// Prompt for new order
	ui.Info("")
	ui.Info("Enter new order as comma-separated numbers (e.g., 1,3,2,4)")
	ui.Info("Press Ctrl+C to cancel")
	fmt.Print("New order: ")

	var input string
	_, err = fmt.Scanln(&input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Parse new order
	parts := strings.Split(input, ",")
	if len(parts) != len(stackBranches) {
		return fmt.Errorf("invalid order: expected %d numbers, got %d", len(stackBranches), len(parts))
	}

	newOrder := make([]int, len(parts))
	for i, part := range parts {
		num, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || num < 1 || num > len(stackBranches) {
			return fmt.Errorf("invalid number: %s", part)
		}
		newOrder[i] = num - 1 // Convert to 0-indexed
	}

	// Check for duplicates
	seen := make(map[int]bool)
	for _, idx := range newOrder {
		if seen[idx] {
			return fmt.Errorf("duplicate number in order")
		}
		seen[idx] = true
	}

	// Build new branch order
	newStackBranches := make([]string, len(stackBranches))
	for i, idx := range newOrder {
		newStackBranches[i] = stackBranches[idx]
	}

	// Display new order for confirmation
	ui.Info("")
	ui.Info("New stack order:")
	for i, branch := range newStackBranches {
		var newParent string
		if i == 0 {
			// First branch keeps its current parent (base)
			metadata, _ := stack.ReadBranchMetadata(branch)
			if metadata != nil {
				newParent = metadata.Parent
			}
		} else {
			newParent = newStackBranches[i-1]
		}
		fmt.Printf("  %d. %s (parent: %s)\n", i+1, branch, newParent)
	}

	// Confirm reorder
	prompt := promptui.Select{
		Label: "Apply this reorder?",
		Items: []string{"Yes", "No"},
	}

	_, result, err := prompt.Run()
	if err != nil || result == "No" {
		ui.Info("Reorder cancelled")
		return nil
	}

	// Apply the reorder
	ui.Info("Applying reorder...")

	// For each branch in new order, update its parent
	for i, branch := range newStackBranches {
		var newParent string
		if i == 0 {
			// First branch keeps its original parent
			metadata, err := stack.ReadBranchMetadata(branch)
			if err != nil {
				return fmt.Errorf("failed to read metadata for %s: %w", branch, err)
			}
			newParent = metadata.Parent
		} else {
			newParent = newStackBranches[i-1]
		}

		metadata, err := stack.ReadBranchMetadata(branch)
		if err != nil {
			return fmt.Errorf("failed to read metadata for %s: %w", branch, err)
		}

		currentParent := metadata.Parent
		if currentParent != newParent {
			ui.Info(fmt.Sprintf("Moving %s: %s → %s", branch, currentParent, newParent))

			// Checkout branch
			if err := git.CheckoutBranch(branch); err != nil {
				return fmt.Errorf("failed to checkout %s: %w", branch, err)
			}

			// Rebase onto new parent
			if err := git.RebaseOnto(newParent); err != nil {
				ui.Error(fmt.Sprintf("Failed to rebase %s onto %s", branch, newParent))
				ui.Info("You may need to resolve conflicts manually")
				return fmt.Errorf("rebase failed")
			}

			// Update metadata
			if err := stack.WriteBranchMetadata(branch, newParent, metadata.PRNumber); err != nil {
				return fmt.Errorf("failed to update metadata: %w", err)
			}

			// Force push
			if err := git.Push(branch, false, true); err != nil {
				return fmt.Errorf("failed to push %s: %w", branch, err)
			}
		}
	}

	// Return to original branch
	if err := git.CheckoutBranch(currentBranch); err != nil {
		ui.Warning(fmt.Sprintf("Could not return to %s", currentBranch))
	}

	ui.Success("Reorder completed successfully")
	ui.Info("Use 'stak log' to view the new stack structure")

	return nil
}
