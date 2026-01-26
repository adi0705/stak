package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var (
	foldSquash bool
	foldForce  bool
)

var foldCmd = &cobra.Command{
	Use:     "fold [branch]",
	Aliases: []string{"fd"},
	Short:   "Merge branch into its parent",
	Long:    `Fold a branch into its parent by merging the commits. Updates children to point to the parent and closes/merges the PR.`,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := ""
		if len(args) > 0 {
			branchName = args[0]
		}

		if err := runFold(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	foldCmd.Flags().BoolVar(&foldSquash, "squash", true, "Squash commits when folding")
	foldCmd.Flags().BoolVarP(&foldForce, "force", "f", false, "Skip confirmation prompts")
	rootCmd.AddCommand(foldCmd)
}

func runFold(branchName string) error {
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

	// Validate parent exists
	parentExists, err := git.BranchExists(parent)
	if err != nil {
		return fmt.Errorf("failed to check if parent exists: %w", err)
	}
	if !parentExists {
		return fmt.Errorf("parent branch %s does not exist", parent)
	}

	// Get children
	children, err := stack.GetChildren(branchName)
	if err != nil {
		return fmt.Errorf("failed to get children: %w", err)
	}

	// Count commits to be folded
	commitCount, err := getCommitCount(branchName, parent)
	if err != nil {
		ui.Warning("Could not count commits")
		commitCount = 0
	}

	// Show confirmation
	if !foldForce {
		ui.Info(fmt.Sprintf("This will:"))
		ui.Info(fmt.Sprintf("  - Merge %d commit(s) from %s into %s", commitCount, branchName, parent))
		if len(children) > 0 {
			ui.Info(fmt.Sprintf("  - Update %d child branch(es) to point to %s", len(children), parent))
		}
		if metadata.PRNumber > 0 {
			ui.Info(fmt.Sprintf("  - Close PR #%d", metadata.PRNumber))
		}
		ui.Info(fmt.Sprintf("  - Delete local branch %s", branchName))

		prompt := promptui.Select{
			Label: "Proceed with fold?",
			Items: []string{"Yes", "No"},
		}

		_, result, err := prompt.Run()
		if err != nil || result == "No" {
			ui.Info("Fold cancelled")
			return nil
		}
	}

	// Checkout parent branch
	ui.Info(fmt.Sprintf("Checking out %s", parent))
	if err := git.CheckoutBranch(parent); err != nil {
		return fmt.Errorf("failed to checkout parent: %w", err)
	}

	// Merge branch into parent
	ui.Info(fmt.Sprintf("Merging %s into %s", branchName, parent))
	if foldSquash {
		// Squash merge
		cmd := exec.Command("git", "merge", "--squash", branchName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to squash merge: %s", string(output))
		}

		// Commit the squashed changes
		commitMsg := fmt.Sprintf("Fold %s into %s", branchName, parent)
		if err := git.Commit(commitMsg); err != nil {
			return fmt.Errorf("failed to commit squashed changes: %w", err)
		}
	} else {
		// Regular merge
		cmd := exec.Command("git", "merge", "--no-ff", branchName, "-m", fmt.Sprintf("Merge %s into %s", branchName, parent))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to merge: %s", string(output))
		}
	}

	ui.Success(fmt.Sprintf("Merged %s into %s", branchName, parent))

	// Push parent
	ui.Info(fmt.Sprintf("Pushing %s", parent))
	if err := git.Push(parent, false, false); err != nil {
		return fmt.Errorf("failed to push parent: %w", err)
	}

	// Update children to point to parent
	for _, child := range children {
		ui.Info(fmt.Sprintf("Updating %s parent: %s → %s", child, branchName, parent))

		childMetadata, err := stack.ReadBranchMetadata(child)
		if err != nil {
			ui.Warning(fmt.Sprintf("Could not read metadata for %s: %v", child, err))
			continue
		}

		// Update metadata
		if err := stack.WriteBranchMetadata(child, parent, childMetadata.PRNumber); err != nil {
			ui.Warning(fmt.Sprintf("Could not update metadata for %s: %v", child, err))
			continue
		}

		// Update PR base if PR exists
		if childMetadata.PRNumber > 0 {
			if err := github.UpdatePRBase(childMetadata.PRNumber, parent); err != nil {
				ui.Warning(fmt.Sprintf("Could not update PR #%d base: %v", childMetadata.PRNumber, err))
			} else {
				ui.Success(fmt.Sprintf("Updated PR #%d base to %s", childMetadata.PRNumber, parent))
			}
		}

		// Rebase child onto parent
		if err := git.CheckoutBranch(child); err != nil {
			ui.Warning(fmt.Sprintf("Could not checkout %s: %v", child, err))
			continue
		}

		ui.Info(fmt.Sprintf("Rebasing %s onto %s", child, parent))
		if err := git.RebaseOnto(parent); err != nil {
			ui.Warning(fmt.Sprintf("Failed to rebase %s: %v", child, err))
			ui.Info("You may need to manually rebase this branch")
			continue
		}

		if err := git.Push(child, false, true); err != nil {
			ui.Warning(fmt.Sprintf("Could not push %s: %v", child, err))
		}
	}

	// Return to parent
	if err := git.CheckoutBranch(parent); err != nil {
		ui.Warning(fmt.Sprintf("Could not return to %s", parent))
	}

	// Close PR if exists
	if metadata.PRNumber > 0 {
		ui.Info(fmt.Sprintf("Closing PR #%d", metadata.PRNumber))
		// Close PR by commenting and closing
		if err := github.ClosePR(metadata.PRNumber); err != nil {
			ui.Warning(fmt.Sprintf("Could not close PR #%d: %v", metadata.PRNumber, err))
			ui.Info("You may want to manually close the PR")
		} else {
			ui.Success(fmt.Sprintf("Closed PR #%d", metadata.PRNumber))
		}
	}

	// Delete local branch
	ui.Info(fmt.Sprintf("Deleting local branch %s", branchName))
	if err := git.DeleteBranch(branchName, true); err != nil {
		ui.Warning(fmt.Sprintf("Could not delete branch %s: %v", branchName, err))
	} else {
		ui.Success(fmt.Sprintf("Deleted branch %s", branchName))
	}

	// Delete metadata
	if err := stack.DeleteBranchMetadata(branchName); err != nil {
		ui.Warning(fmt.Sprintf("Could not delete metadata: %v", err))
	}

	ui.Success(fmt.Sprintf("Folded %s into %s", branchName, parent))
	return nil
}

func getCommitCount(branch, base string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", base, branch))
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var count int
	if _, err := fmt.Sscanf(string(output), "%d", &count); err != nil {
		return 0, err
	}

	return count, nil
}
