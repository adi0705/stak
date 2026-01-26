package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var checkoutCmd = &cobra.Command{
	Use:     "checkout [branch]",
	Aliases: []string{"co"},
	Short:   "Smart checkout with branch context",
	Long:    `Switch to a branch with context about its position in the stack. Shows an interactive menu with parent/children information if no branch is specified.`,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := ""
		if len(args) > 0 {
			branchName = args[0]
		}

		if err := runCheckout(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(checkoutCmd)
}

func runCheckout(branchName string) error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Get current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// If branch name provided, checkout directly
	if branchName != "" {
		// Check if branch exists
		exists, err := git.BranchExists(branchName)
		if err != nil {
			return fmt.Errorf("failed to check if branch exists: %w", err)
		}
		if !exists {
			return fmt.Errorf("branch %s does not exist", branchName)
		}

		if branchName == currentBranch {
			ui.Info(fmt.Sprintf("Already on branch %s", branchName))
			return nil
		}

		if err := git.CheckoutBranch(branchName); err != nil {
			return fmt.Errorf("failed to checkout branch %s: %w", branchName, err)
		}

		ui.Success(fmt.Sprintf("Switched to branch %s", branchName))
		return nil
	}

	// No branch specified - show interactive menu
	return selectBranchInteractive(currentBranch)
}

func selectBranchInteractive(currentBranch string) error {
	// Get all local branches
	allBranches, err := git.GetAllLocalBranches()
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}

	// Get all tracked branches
	trackedBranches, err := stack.GetAllStackBranches()
	if err != nil {
		// If error, continue with empty tracked list
		trackedBranches = []string{}
	}

	// Build options with context
	type branchOption struct {
		name    string
		display string
	}

	var options []branchOption

	// 1. Current branch (highlighted)
	hasMetadata, _ := stack.HasStackMetadata(currentBranch)
	if hasMetadata {
		display := fmt.Sprintf("● %s (current)", currentBranch)
		options = append(options, branchOption{name: currentBranch, display: display})
	} else {
		display := fmt.Sprintf("● %s (current, base)", currentBranch)
		options = append(options, branchOption{name: currentBranch, display: display})
	}

	// 2. Tracked branches with context
	for _, branch := range trackedBranches {
		if branch == currentBranch {
			continue // Already added above
		}

		metadata, err := stack.ReadBranchMetadata(branch)
		if err != nil {
			continue
		}

		// Build display with parent and PR info
		parts := []string{branch}
		if metadata.Parent != "" {
			parts = append(parts, fmt.Sprintf("parent: %s", metadata.Parent))
		}
		if metadata.PRNumber > 0 {
			parts = append(parts, fmt.Sprintf("PR #%d", metadata.PRNumber))
		}

		// Add children count
		children, _ := stack.GetChildren(branch)
		if len(children) > 0 {
			parts = append(parts, fmt.Sprintf("%d child(ren)", len(children)))
		}

		display := strings.Join(parts, " • ")
		options = append(options, branchOption{name: branch, display: display})
	}

	// 3. Base branches (main, master, etc.)
	baseBranches := []string{"main", "master", "develop", "development"}
	for _, base := range baseBranches {
		if base == currentBranch {
			continue
		}
		for _, b := range allBranches {
			if b == base && !contains(trackedBranches, base) {
				display := fmt.Sprintf("%s (base)", base)
				options = append(options, branchOption{name: base, display: display})
				break
			}
		}
	}

	// 4. Other untracked branches
	for _, branch := range allBranches {
		if branch == currentBranch {
			continue
		}
		if contains(trackedBranches, branch) {
			continue
		}
		isBase := false
		for _, base := range baseBranches {
			if branch == base {
				isBase = true
				break
			}
		}
		if isBase {
			continue
		}

		options = append(options, branchOption{name: branch, display: branch})
	}

	if len(options) <= 1 {
		return fmt.Errorf("no other branches available")
	}

	// Create display items for promptui
	displayItems := make([]string, len(options))
	for i, opt := range options {
		displayItems[i] = opt.display
	}

	// Prompt user
	prompt := promptui.Select{
		Label: "Select branch to checkout",
		Items: displayItems,
		Size:  15,
		Templates: &promptui.SelectTemplates{
			Active:   "▸ {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "✓ {{ . | green }}",
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return fmt.Errorf("branch selection cancelled")
	}

	targetBranch := options[idx].name

	// Don't checkout if same as current
	if targetBranch == currentBranch {
		ui.Info("Already on this branch")
		return nil
	}

	// Checkout the selected branch
	if err := git.CheckoutBranch(targetBranch); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", targetBranch, err)
	}

	ui.Success(fmt.Sprintf("Switched to branch %s", targetBranch))
	return nil
}
