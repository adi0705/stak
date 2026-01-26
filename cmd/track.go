package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var (
	trackParent    string
	trackAuto      bool
	trackForce     bool
	trackRecursive bool
)

var trackCmd = &cobra.Command{
	Use:     "track [branch]",
	Aliases: []string{"tr"},
	Short:   "Add existing branch to stack",
	Long: `Track an existing branch by designating its parent branch.
This allows you to incorporate branches not created with stak create into the stack system.`,
	Run: func(cmd *cobra.Command, args []string) {
		branchName := ""
		if len(args) > 0 {
			branchName = args[0]
		}

		if err := runTrack(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	trackCmd.Flags().StringVar(&trackParent, "parent", "", "Specify parent branch explicitly")
	trackCmd.Flags().BoolVar(&trackAuto, "auto", false, "Auto-detect parent from PR base")
	trackCmd.Flags().BoolVar(&trackForce, "force", false, "Use most recent tracked ancestor as parent")
	trackCmd.Flags().BoolVar(&trackRecursive, "recursive", false, "Recursively track untracked parents")
	rootCmd.AddCommand(trackCmd)
}

func runTrack(branchName string) error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// 1. Determine target branch (argument or current)
	if branchName == "" {
		var err error
		branchName, err = git.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
	}

	// 2. Validate branch exists
	exists, err := git.BranchExists(branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("branch %s does not exist", branchName)
	}

	// 3. Check if already tracked
	hasMetadata, err := stack.HasStackMetadata(branchName)
	if err != nil {
		return fmt.Errorf("failed to check stack metadata: %w", err)
	}
	if hasMetadata {
		ui.Warning(fmt.Sprintf("Branch %s is already tracked", branchName))
		return offerUpdateParent(branchName)
	}

	// 4. Determine parent based on flags
	parent, err := determineParent(branchName)
	if err != nil {
		return err
	}

	// 5. Validate parent and handle recursive tracking
	if err := validateAndTrackParent(parent, branchName); err != nil {
		return err
	}

	// 6. Check for cycles
	wouldCycle, err := stack.WouldCreateCycle(branchName, parent)
	if err != nil {
		return fmt.Errorf("failed to check for cycles: %w", err)
	}
	if wouldCycle {
		return fmt.Errorf("cannot set parent: would create circular dependency")
	}

	// 7. Get PR number if exists
	prNumber := 0
	pr, base, err := github.GetPRForBranch(branchName)
	if err == nil {
		prNumber = pr
		ui.Info(fmt.Sprintf("Found PR #%d", prNumber))
		// If auto mode and base doesn't match parent, warn user
		if trackAuto && base != parent {
			ui.Warning(fmt.Sprintf("PR base is %s but tracking with parent %s", base, parent))
		}
	}

	// 8. Write metadata
	if err := stack.WriteBranchMetadata(branchName, parent, prNumber); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// 9. Show success with visualization
	parentInfo := parent
	if parentInfo == "" {
		parentInfo = "(root of stack)"
	}
	ui.Success(fmt.Sprintf("Tracked %s with parent %s", branchName, parentInfo))

	if prNumber > 0 {
		ui.Info(fmt.Sprintf("PR: #%d", prNumber))
	}

	return nil
}

func determineParent(branch string) (string, error) {
	// Strategy 1: --parent flag (explicit)
	if trackParent != "" {
		return trackParent, nil
	}

	// Strategy 2: --auto flag (PR-based detection)
	if trackAuto {
		prNumber, baseRef, err := github.GetPRForBranch(branch)
		if err == nil && baseRef != "" {
			ui.Info(fmt.Sprintf("Detected PR #%d with base: %s", prNumber, baseRef))
			return baseRef, nil
		}
		ui.Warning("No PR found for auto-detection, falling back to interactive")
	}

	// Strategy 3: --force flag (find most recent tracked ancestor)
	if trackForce {
		return findMostRecentTrackedAncestor(branch)
	}

	// Strategy 4: Interactive selection (default)
	return selectParentInteractive(branch)
}

func findMostRecentTrackedAncestor(branch string) (string, error) {
	ui.Info("Finding most recent tracked ancestor...")

	// Get commit history
	ancestors, err := git.GetCommitAncestors(branch)
	if err != nil {
		return "", fmt.Errorf("failed to get commit ancestors: %w", err)
	}

	// Get all tracked branches
	trackedBranches, err := stack.GetAllStackBranches()
	if err != nil {
		return "", fmt.Errorf("failed to get tracked branches: %w", err)
	}

	// Find first commit that exists in a tracked branch
	for _, commit := range ancestors {
		for _, tracked := range trackedBranches {
			if git.BranchContainsCommit(tracked, commit) {
				ui.Success(fmt.Sprintf("Found tracked ancestor: %s", tracked))
				return tracked, nil
			}
		}
	}

	// Fallback to base branch
	return getBaseBranch()
}

func getBaseBranch() (string, error) {
	// Try common base branches in order
	baseBranches := []string{"main", "master", "develop", "development"}
	for _, base := range baseBranches {
		exists, err := git.BranchExists(base)
		if err == nil && exists {
			return base, nil
		}
	}
	return "", fmt.Errorf("no base branch found (tried: main, master, develop, development)")
}

func selectParentInteractive(branch string) (string, error) {
	// Get all local branches except current
	allBranches, err := git.GetAllLocalBranches()
	if err != nil {
		return "", fmt.Errorf("failed to list branches: %w", err)
	}

	// Build options with categories
	var options []string

	// 1. Base branches (main, master, develop)
	baseBranches := []string{"main", "master", "develop", "development"}
	for _, base := range baseBranches {
		if contains(allBranches, base) && base != branch {
			options = append(options, fmt.Sprintf("%s (base branch)", base))
		}
	}

	// 2. Tracked branches (already in stack)
	trackedBranches, err := stack.GetAllStackBranches()
	if err == nil {
		for _, tracked := range trackedBranches {
			if tracked != branch {
				metadata, err := stack.ReadBranchMetadata(tracked)
				prInfo := ""
				if err == nil && metadata.PRNumber > 0 {
					prInfo = fmt.Sprintf(", PR #%d", metadata.PRNumber)
				}
				options = append(options, fmt.Sprintf("%s (tracked%s)", tracked, prInfo))
			}
		}
	}

	// 3. Other branches
	for _, b := range allBranches {
		if b != branch && !contains(trackedBranches, b) && !contains(baseBranches, b) {
			options = append(options, b)
		}
	}

	// 4. No parent option (root)
	options = append(options, "[No parent - root of stack]")

	if len(options) == 0 {
		return "", fmt.Errorf("no available parent branches found")
	}

	// Prompt user
	prompt := promptui.Select{
		Label: fmt.Sprintf("Select parent branch for %s", branch),
		Items: options,
		Size:  10,
	}

	_, result, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("parent selection cancelled")
	}

	// Parse selection (remove annotations)
	if strings.Contains(result, "[No parent") {
		return "", nil // No parent = root branch
	}

	// Extract branch name from "branch (annotation)"
	parent := strings.Split(result, " ")[0]
	return parent, nil
}

func validateAndTrackParent(parent, childBranch string) error {
	if parent == "" {
		return nil // Root branch, no parent validation needed
	}

	// Check if parent exists
	exists, err := git.BranchExists(parent)
	if err != nil {
		return fmt.Errorf("failed to check if parent exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("parent branch %s does not exist locally", parent)
	}

	// Prevent setting parent to same branch
	if parent == childBranch {
		return fmt.Errorf("cannot set branch as its own parent")
	}

	// Check if parent is tracked
	hasMetadata, err := stack.HasStackMetadata(parent)
	if err != nil {
		return fmt.Errorf("failed to check parent metadata: %w", err)
	}

	// Check if parent is a base branch
	isBase := stack.IsBaseBranch(parent)

	if !hasMetadata && !isBase {
		ui.Warning(fmt.Sprintf("Parent %s is not tracked in stack", parent))

		if trackRecursive {
			// Auto-track parent recursively
			ui.Info(fmt.Sprintf("Recursively tracking %s", parent))
			return runTrack(parent)
		}

		// Prompt user
		prompt := promptui.Select{
			Label: "What would you like to do?",
			Items: []string{
				"Track parent recursively",
				"Continue anyway (parent treated as base)",
				"Cancel",
			},
		}

		_, result, err := prompt.Run()
		if err != nil || result == "Cancel" {
			return fmt.Errorf("tracking cancelled")
		}

		if result == "Track parent recursively" {
			return runTrack(parent)
		}
	}

	return nil
}

func offerUpdateParent(branch string) error {
	// Get current parent
	currentParent, err := stack.GetParent(branch)
	if err != nil {
		return fmt.Errorf("failed to get current parent: %w", err)
	}

	parentInfo := currentParent
	if currentParent == "" {
		parentInfo = "(root of stack)"
	}

	ui.Info(fmt.Sprintf("Current parent: %s", parentInfo))

	// Get current PR number
	metadata, err := stack.ReadBranchMetadata(branch)
	if err == nil && metadata.PRNumber > 0 {
		ui.Info(fmt.Sprintf("Current PR: #%d", metadata.PRNumber))
	}

	// Ask if user wants to update
	prompt := promptui.Select{
		Label: "Branch is already tracked. What would you like to do?",
		Items: []string{
			"Update parent",
			"Keep existing",
			"Cancel",
		},
	}

	_, result, err := prompt.Run()
	if err != nil || result == "Cancel" || result == "Keep existing" {
		return nil
	}

	// Update parent
	newParent, err := selectParentInteractive(branch)
	if err != nil {
		return err
	}

	// Validate and track parent
	if err := validateAndTrackParent(newParent, branch); err != nil {
		return err
	}

	// Check for cycles
	wouldCycle, err := stack.WouldCreateCycle(branch, newParent)
	if err != nil {
		return fmt.Errorf("failed to check for cycles: %w", err)
	}
	if wouldCycle {
		return fmt.Errorf("cannot update parent: would create circular dependency")
	}

	// Update metadata (keep existing PR number)
	prNumber := 0
	if metadata != nil {
		prNumber = metadata.PRNumber
	}

	if err := stack.WriteBranchMetadata(branch, newParent, prNumber); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	newParentInfo := newParent
	if newParent == "" {
		newParentInfo = "(root of stack)"
	}

	ui.Success(fmt.Sprintf("Updated parent to: %s", newParentInfo))
	return nil
}

// contains checks if a string slice contains a value
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
