package cmd

import (
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var (
	moveParent string
)

var moveCmd = &cobra.Command{
	Use:     "move [branch]",
	Aliases: []string{"mv"},
	Short:   "Change a branch's parent",
	Long:    `Move a branch to a different parent in the stack. This rebases the branch onto the new parent and updates all metadata and PR bases.`,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := ""
		if len(args) > 0 {
			branchName = args[0]
		}

		if err := runMove(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	moveCmd.Flags().StringVar(&moveParent, "parent", "", "New parent branch")
	rootCmd.AddCommand(moveCmd)
}

func runMove(branchName string) error {
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
		return fmt.Errorf("branch %s is not tracked. Use 'stak track' first", branchName)
	}

	// Get current metadata
	metadata, err := stack.ReadBranchMetadata(branchName)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	currentParent := metadata.Parent
	ui.Info(fmt.Sprintf("Current parent: %s", currentParent))

	// Determine new parent
	var newParent string
	if moveParent != "" {
		newParent = moveParent
	} else {
		// Interactive selection
		newParent, err = selectNewParent(branchName, currentParent)
		if err != nil {
			return err
		}
	}

	if newParent == currentParent {
		ui.Info("New parent is the same as current parent. Nothing to do.")
		return nil
	}

	// Validate new parent exists
	if newParent != "" {
		exists, err := git.BranchExists(newParent)
		if err != nil {
			return fmt.Errorf("failed to check if new parent exists: %w", err)
		}
		if !exists {
			return fmt.Errorf("new parent branch %s does not exist", newParent)
		}
	}

	// Prevent setting branch as its own parent
	if newParent == branchName {
		return fmt.Errorf("cannot set branch as its own parent")
	}

	// Check for cycles
	wouldCycle, err := stack.WouldCreateCycle(branchName, newParent)
	if err != nil {
		return fmt.Errorf("failed to check for cycles: %w", err)
	}
	if wouldCycle {
		return fmt.Errorf("cannot move: would create circular dependency")
	}

	// Checkout the branch
	currentBranch, _ := git.GetCurrentBranch()
	if currentBranch != branchName {
		ui.Info(fmt.Sprintf("Checking out %s", branchName))
		if err := git.CheckoutBranch(branchName); err != nil {
			return fmt.Errorf("failed to checkout branch: %w", err)
		}
	}

	// Rebase onto new parent
	ui.Info(fmt.Sprintf("Rebasing %s onto %s", branchName, newParent))
	if err := git.RebaseOnto(newParent); err != nil {
		return fmt.Errorf("failed to rebase: %w", err)
	}

	// Update metadata
	if err := stack.WriteBranchMetadata(branchName, newParent, metadata.PRNumber); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	// Push changes
	ui.Info(fmt.Sprintf("Force pushing %s", branchName))
	if err := git.Push(branchName, false, true); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// Update PR base if PR exists
	if metadata.PRNumber > 0 {
		ui.Info(fmt.Sprintf("Updating PR #%d base to %s", metadata.PRNumber, newParent))
		if err := github.UpdatePRBase(metadata.PRNumber, newParent); err != nil {
			return fmt.Errorf("failed to update PR base: %w", err)
		}
	}

	// Rebase all children onto the moved branch
	children, err := stack.GetChildren(branchName)
	if err != nil {
		return fmt.Errorf("failed to get children: %w", err)
	}

	if len(children) > 0 {
		ui.Info(fmt.Sprintf("Syncing %d child branch(es)", len(children)))
		for _, child := range children {
			if err := syncBranchRecursive(child); err != nil {
				return fmt.Errorf("failed to sync child %s: %w", child, err)
			}
		}

		// Return to the branch we moved
		if err := git.CheckoutBranch(branchName); err != nil {
			return fmt.Errorf("failed to return to branch: %w", err)
		}
	}

	ui.Success(fmt.Sprintf("Moved %s from %s to %s", branchName, currentParent, newParent))
	return nil
}

func selectNewParent(branch, currentParent string) (string, error) {
	// Get all local branches except current
	allBranches, err := git.GetAllLocalBranches()
	if err != nil {
		return "", fmt.Errorf("failed to list branches: %w", err)
	}

	// Get all tracked branches
	trackedBranches, err := stack.GetAllStackBranches()
	if err != nil {
		trackedBranches = []string{}
	}

	// Build options
	var options []string

	// Base branches (main, master, develop)
	baseBranches := []string{"main", "master", "develop", "development"}
	for _, base := range baseBranches {
		for _, b := range allBranches {
			if b == base && b != branch {
				if base == currentParent {
					options = append(options, fmt.Sprintf("%s (current parent)", base))
				} else {
					options = append(options, fmt.Sprintf("%s (base branch)", base))
				}
				break
			}
		}
	}

	// Tracked branches
	for _, tracked := range trackedBranches {
		if tracked != branch {
			metadata, _ := stack.ReadBranchMetadata(tracked)
			prInfo := ""
			if metadata != nil && metadata.PRNumber > 0 {
				prInfo = fmt.Sprintf(", PR #%d", metadata.PRNumber)
			}
			if tracked == currentParent {
				options = append(options, fmt.Sprintf("%s (current parent%s)", tracked, prInfo))
			} else {
				options = append(options, fmt.Sprintf("%s (tracked%s)", tracked, prInfo))
			}
		}
	}

	// Other branches
	for _, b := range allBranches {
		if b == branch {
			continue
		}
		isBase := false
		for _, base := range baseBranches {
			if b == base {
				isBase = true
				break
			}
		}
		if isBase {
			continue
		}
		isTracked := false
		for _, tracked := range trackedBranches {
			if b == tracked {
				isTracked = true
				break
			}
		}
		if !isTracked {
			options = append(options, b)
		}
	}

	// No parent option
	options = append(options, "[No parent - root of stack]")

	if len(options) == 0 {
		return "", fmt.Errorf("no available parent branches found")
	}

	// Prompt user
	prompt := promptui.Select{
		Label: fmt.Sprintf("Select new parent for %s", branch),
		Items: options,
		Size:  10,
	}

	_, result, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("parent selection cancelled")
	}

	// Parse selection
	if result == "[No parent - root of stack]" {
		return "", nil
	}

	// Extract branch name from "branch (annotation)"
	parts := []string{}
	for i, c := range result {
		if c == ' ' && i < len(result)-1 && result[i+1] == '(' {
			break
		}
		parts = append(parts, string(c))
	}
	parent := ""
	for _, p := range parts {
		parent += p
	}

	return parent, nil
}
