package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var (
	splitAt   string
	splitName string
)

var splitCmd = &cobra.Command{
	Use:     "split [branch]",
	Aliases: []string{"sp"},
	Short:   "Split a branch into two branches",
	Long:    `Split a branch at a specific commit, creating a new branch with commits after the split point.`,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := ""
		if len(args) > 0 {
			branchName = args[0]
		}

		if err := runSplit(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	splitCmd.Flags().StringVar(&splitAt, "at", "", "Commit hash to split at")
	splitCmd.Flags().StringVar(&splitName, "name", "", "Name for the new branch")
	rootCmd.AddCommand(splitCmd)
}

func runSplit(branchName string) error {
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
		return fmt.Errorf("branch %s is not tracked", branchName)
	}

	// Get metadata
	metadata, err := stack.ReadBranchMetadata(branchName)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	parent := metadata.Parent
	if parent == "" {
		return fmt.Errorf("branch %s has no parent (is a root branch)", branchName)
	}

	// Checkout the branch
	currentBranch, _ := git.GetCurrentBranch()
	if currentBranch != branchName {
		ui.Info(fmt.Sprintf("Checking out %s", branchName))
		if err := git.CheckoutBranch(branchName); err != nil {
			return fmt.Errorf("failed to checkout branch: %w", err)
		}
	}

	// Get commit list
	commits, err := getCommitList(branchName, parent)
	if err != nil {
		return fmt.Errorf("failed to get commit list: %w", err)
	}

	if len(commits) <= 1 {
		return fmt.Errorf("branch has only %d commit(s), cannot split", len(commits))
	}

	// Determine split point
	var splitCommit string
	if splitAt != "" {
		splitCommit = splitAt
	} else {
		// Interactive selection
		splitCommit, err = selectSplitPoint(commits)
		if err != nil {
			return err
		}
	}

	// Validate split commit exists
	cmd := exec.Command("git", "rev-parse", "--verify", splitCommit)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("invalid commit: %s", splitCommit)
	}

	// Determine new branch name
	newBranchName := splitName
	if newBranchName == "" {
		newBranchName = fmt.Sprintf("%s-2", branchName)
		ui.Info(fmt.Sprintf("New branch name: %s", newBranchName))
	}

	// Check if new branch name already exists
	exists, err = git.BranchExists(newBranchName)
	if err != nil {
		return fmt.Errorf("failed to check if new branch exists: %w", err)
	}
	if exists {
		return fmt.Errorf("branch %s already exists", newBranchName)
	}

	ui.Info(fmt.Sprintf("Splitting %s at commit %s", branchName, splitCommit[:8]))

	// Create new branch at split point
	ui.Info(fmt.Sprintf("Creating %s at %s", newBranchName, splitCommit))
	cmd = exec.Command("git", "branch", newBranchName, splitCommit)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// Reset original branch to split point (hard reset)
	ui.Info(fmt.Sprintf("Resetting %s to %s", branchName, splitCommit))
	cmd = exec.Command("git", "reset", "--hard", splitCommit)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to reset: %s", string(output))
	}

	// Force push original branch
	ui.Info(fmt.Sprintf("Force pushing %s", branchName))
	if err := git.Push(branchName, false, true); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// Checkout new branch and cherry-pick remaining commits
	ui.Info(fmt.Sprintf("Checking out %s", newBranchName))
	if err := git.CheckoutBranch(newBranchName); err != nil {
		return fmt.Errorf("failed to checkout new branch: %w", err)
	}

	// Get children of original branch
	children, err := stack.GetChildren(branchName)
	if err != nil {
		return fmt.Errorf("failed to get children: %w", err)
	}

	// Track new branch with original branch as parent
	if err := stack.WriteBranchMetadata(newBranchName, branchName, 0); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Update children to point to new branch
	for _, child := range children {
		ui.Info(fmt.Sprintf("Updating %s parent: %s → %s", child, branchName, newBranchName))

		childMetadata, err := stack.ReadBranchMetadata(child)
		if err != nil {
			ui.Warning(fmt.Sprintf("Could not read metadata for %s: %v", child, err))
			continue
		}

		// Update metadata
		if err := stack.WriteBranchMetadata(child, newBranchName, childMetadata.PRNumber); err != nil {
			ui.Warning(fmt.Sprintf("Could not update metadata for %s: %v", child, err))
			continue
		}

		// Update PR base if PR exists
		if childMetadata.PRNumber > 0 {
			if err := github.UpdatePRBase(childMetadata.PRNumber, newBranchName); err != nil {
				ui.Warning(fmt.Sprintf("Could not update PR #%d base: %v", childMetadata.PRNumber, err))
			} else {
				ui.Success(fmt.Sprintf("Updated PR #%d base to %s", childMetadata.PRNumber, newBranchName))
			}
		}
	}

	// Push new branch
	ui.Info(fmt.Sprintf("Pushing %s", newBranchName))
	if err := git.Push(newBranchName, true, false); err != nil {
		return fmt.Errorf("failed to push new branch: %w", err)
	}

	ui.Success(fmt.Sprintf("Split %s into %s and %s", branchName, branchName, newBranchName))
	ui.Info(fmt.Sprintf("Create PR for %s with: stak submit", newBranchName))

	return nil
}

func getCommitList(branch, base string) ([]string, error) {
	cmd := exec.Command("git", "log", "--oneline", "--reverse", fmt.Sprintf("%s..%s", base, branch))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var commits []string
	for _, line := range lines {
		if line != "" {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) >= 1 {
				commits = append(commits, parts[0])
			}
		}
	}

	return commits, nil
}

func selectSplitPoint(commits []string) (string, error) {
	if len(commits) == 0 {
		return "", fmt.Errorf("no commits to select from")
	}

	// Get full commit messages for display
	var displayCommits []string
	for i, hash := range commits {
		cmd := exec.Command("git", "log", "-1", "--oneline", hash)
		output, err := cmd.Output()
		if err != nil {
			displayCommits = append(displayCommits, fmt.Sprintf("%d. %s", i+1, hash))
		} else {
			msg := strings.TrimSpace(string(output))
			displayCommits = append(displayCommits, fmt.Sprintf("%d. %s", i+1, msg))
		}
	}

	prompt := promptui.Select{
		Label: "Select split point (commits after this will be in new branch)",
		Items: displayCommits,
		Size:  10,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("commit selection cancelled")
	}

	return commits[idx], nil
}
