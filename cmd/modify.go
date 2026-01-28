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
	modifyAmend      bool
	modifyRebaseNum  int
	modifyEditPR     bool
	modifyTitle      string
	modifyBody       string
	modifyPush       bool
	modifyCommit     bool
	modifyInto       string
)

var modifyCmd = &cobra.Command{
	Use:     "modify",
	Aliases: []string{"m"},
	Short:   "Modify current branch (commits only, no push)",
	Long: `Modify the current branch by creating or amending commits locally.
By default, this command does NOT push changes - it only creates commits.
Use --push flag if you want to push and sync children after committing.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runModify(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	modifyCmd.Flags().BoolVar(&modifyAmend, "amend", false, "Amend the last commit")
	modifyCmd.Flags().IntVar(&modifyRebaseNum, "rebase", 0, "Interactive rebase last N commits")
	modifyCmd.Flags().BoolVar(&modifyEditPR, "edit", false, "Edit PR title/body")
	modifyCmd.Flags().StringVar(&modifyTitle, "title", "", "New PR title")
	modifyCmd.Flags().StringVar(&modifyBody, "body", "", "New PR body")
	modifyCmd.Flags().BoolVarP(&modifyPush, "push", "p", false, "Push changes after committing")
	modifyCmd.Flags().BoolVarP(&modifyCommit, "commit", "c", false, "Create a fresh commit instead of amending")
	modifyCmd.Flags().StringVar(&modifyInto, "into", "", "Apply changes to downstack branch")
	rootCmd.AddCommand(modifyCmd)
}

func runModify() error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
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
	if !hasMetadata {
		return fmt.Errorf("branch %s is not part of a stack", currentBranch)
	}

	// Handle --into flag (apply changes to downstack branch)
	if modifyInto != "" {
		return applyToDownstack(currentBranch, modifyInto)
	}

	// If no flags provided, show interactive menu when there are no staged changes
	if !modifyAmend && modifyRebaseNum == 0 && !modifyEditPR && modifyTitle == "" && modifyBody == "" && !modifyCommit {
		// Check if there are any staged changes specifically
		hasStagedChanges, err := git.HasStagedChanges()
		if err != nil {
			return fmt.Errorf("failed to check for staged changes: %w", err)
		}

		if !hasStagedChanges {
			// No staged changes - show interactive menu
			choice, err := showModifyMenu()
			if err != nil {
				return err
			}

			switch choice {
			case "Commit all file changes (--all)":
				if err := commitAllChanges(); err != nil {
					return err
				}
			case "Select changes to commit (--patch)":
				if err := commitPatchChanges(); err != nil {
					return err
				}
			case "Just edit the commit message":
				modifyAmend = true
			case "Abort this operation":
				ui.Info("Operation aborted")
				return nil
			}
		} else {
			// Has staged changes but no explicit flags
			// Check if there are commits on this branch - if yes, amend by default
			hasCommits, err := branchHasCommits(currentBranch)
			if err != nil {
				return fmt.Errorf("failed to check for commits: %w", err)
			}

			if hasCommits {
				// Auto-amend to existing commit
				ui.Info("Amending last commit (use -c to create new commit instead)")
				modifyAmend = true
			} else {
				// No commits yet, create first commit
				ui.Info("Creating first commit on branch")
				modifyCommit = true
			}
		}
	}

	// Handle commit (fresh commit)
	if modifyCommit {
		ui.Info("Creating new commit")
		cmd := exec.Command("git", "commit")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}
	}

	// Handle amend
	if modifyAmend {
		ui.Info("Amending last commit")
		cmd := exec.Command("git", "commit", "--amend", "--no-edit")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to amend commit: %w", err)
		}
	}

	// Handle interactive rebase
	if modifyRebaseNum > 0 {
		ui.Info(fmt.Sprintf("Starting interactive rebase for last %d commits", modifyRebaseNum))
		cmd := exec.Command("git", "rebase", "-i", fmt.Sprintf("HEAD~%d", modifyRebaseNum))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to rebase: %w", err)
		}
	}

	// Only push if --push flag is provided
	if modifyPush {
		// Determine if force push is needed
		// Force push only if we amended or rebased (which rewrites history)
		// Fresh commits with -c don't need force push
		needsForcePush := modifyAmend || modifyRebaseNum > 0

		if needsForcePush {
			ui.Info(fmt.Sprintf("Force pushing %s", currentBranch))
		} else {
			ui.Info(fmt.Sprintf("Pushing %s", currentBranch))
		}

		if err := git.Push(currentBranch, false, needsForcePush); err != nil {
			return fmt.Errorf("failed to push: %w", err)
		}

		ui.Success(fmt.Sprintf("Pushed %s", currentBranch))

		// Edit PR if requested
		if modifyEditPR || modifyTitle != "" || modifyBody != "" {
			metadata, err := stack.ReadBranchMetadata(currentBranch)
			if err != nil {
				return fmt.Errorf("failed to read branch metadata: %w", err)
			}

			if metadata.PRNumber == 0 {
				ui.Warning("No PR associated with this branch")
			} else {
				ui.Info(fmt.Sprintf("Updating PR #%d", metadata.PRNumber))
				if err := github.EditPR(metadata.PRNumber, modifyTitle, modifyBody); err != nil {
					return fmt.Errorf("failed to edit PR: %w", err)
				}
				ui.Success(fmt.Sprintf("Updated PR #%d", metadata.PRNumber))
			}
		}

		// Sync children after pushing
		children, err := stack.GetChildren(currentBranch)
		if err != nil {
			return fmt.Errorf("failed to get children: %w", err)
		}

		if len(children) > 0 {
			ui.Info(fmt.Sprintf("Syncing %d child branch(es)", len(children)))

			// Fetch first
			if err := git.Fetch(); err != nil {
				return fmt.Errorf("failed to fetch: %w", err)
			}

			// Sync each child recursively
			for _, child := range children {
				if err := syncBranchRecursive(child); err != nil {
					return err
				}
			}

			// Return to original branch
			if err := git.CheckoutBranch(currentBranch); err != nil {
				return fmt.Errorf("failed to return to branch %s: %w", currentBranch, err)
			}
		}
	} else {
		// Not pushing - just inform the user
		ui.Success("Commits created locally")
		ui.Info("Use 'stak modify --push' or 'git push' to push changes")
	}

	ui.Success("Modify completed successfully")
	return nil
}

// showModifyMenu displays an interactive menu for modify options
func showModifyMenu() (string, error) {
	prompt := promptui.Select{
		Label: "You have no staged changes. What would you like to do?",
		Items: []string{
			"Commit all file changes (--all)",
			"Select changes to commit (--patch)",
			"Just edit the commit message",
			"Abort this operation",
		},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("prompt failed: %w", err)
	}

	return result, nil
}

// commitAllChanges commits all changes with git commit --all
func commitAllChanges() error {
	ui.Info("Committing all changes")
	cmd := exec.Command("git", "commit", "--all")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to commit all changes: %w", err)
	}
	return nil
}

// commitPatchChanges commits changes interactively with git add --patch
func commitPatchChanges() error {
	ui.Info("Starting interactive patch selection")

	// First, run git add --patch
	addCmd := exec.Command("git", "add", "--patch")
	addCmd.Stdin = os.Stdin
	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("failed to select patches: %w", err)
	}

	// Then commit the staged changes
	commitCmd := exec.Command("git", "commit")
	commitCmd.Stdin = os.Stdin
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

// applyToDownstack applies current changes to a downstack (ancestor) branch
func applyToDownstack(currentBranch, targetBranch string) error {
	// Validate target branch exists
	exists, err := git.BranchExists(targetBranch)
	if err != nil {
		return fmt.Errorf("failed to check if target branch exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("target branch %s does not exist", targetBranch)
	}

	// Check if target is an ancestor of current
	isAncestor, err := isAncestorBranch(targetBranch, currentBranch)
	if err != nil {
		return fmt.Errorf("failed to check branch relationship: %w", err)
	}
	if !isAncestor {
		return fmt.Errorf("target branch %s is not an ancestor of %s", targetBranch, currentBranch)
	}

	// Check for uncommitted changes
	hasChanges, err := git.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}
	if !hasChanges {
		return fmt.Errorf("no changes to apply")
	}

	ui.Info(fmt.Sprintf("Applying changes from %s to %s", currentBranch, targetBranch))

	// Stash current changes
	ui.Info("Stashing changes")
	stashCmd := exec.Command("git", "stash", "push", "-m", fmt.Sprintf("stak-modify-into-%s", targetBranch))
	if err := stashCmd.Run(); err != nil {
		return fmt.Errorf("failed to stash changes: %w", err)
	}

	// Checkout target branch
	ui.Info(fmt.Sprintf("Switching to %s", targetBranch))
	if err := git.CheckoutBranch(targetBranch); err != nil {
		return fmt.Errorf("failed to checkout target branch: %w", err)
	}

	// Apply stash
	ui.Info("Applying changes")
	popCmd := exec.Command("git", "stash", "pop")
	popCmd.Stdout = os.Stdout
	popCmd.Stderr = os.Stderr
	if err := popCmd.Run(); err != nil {
		ui.Warning("Failed to apply stash cleanly. You may need to resolve conflicts.")
		return fmt.Errorf("stash apply failed: %w", err)
	}

	// Prompt for commit
	ui.Info("Changes applied. Creating commit...")
	commitCmd := exec.Command("git", "commit")
	commitCmd.Stdin = os.Stdin
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		ui.Warning("Commit cancelled or failed. Changes are still staged.")
		return fmt.Errorf("failed to commit: %w", err)
	}

	ui.Success(fmt.Sprintf("Changes committed to %s", targetBranch))

	// Push target branch
	ui.Info(fmt.Sprintf("Pushing %s", targetBranch))
	if err := git.Push(targetBranch, false, false); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// Rebase all descendants of target (including original current branch)
	ui.Info("Syncing descendant branches")
	children, err := stack.GetChildren(targetBranch)
	if err != nil {
		return fmt.Errorf("failed to get children: %w", err)
	}

	for _, child := range children {
		if err := syncBranchRecursive(child); err != nil {
			return err
		}
	}

	// Return to original branch
	ui.Info(fmt.Sprintf("Returning to %s", currentBranch))
	if err := git.CheckoutBranch(currentBranch); err != nil {
		return fmt.Errorf("failed to return to original branch: %w", err)
	}

	ui.Success("Successfully applied changes to downstack branch")
	return nil
}

// isAncestorBranch checks if ancestor is an ancestor of descendant
func isAncestorBranch(ancestor, descendant string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, descendant)
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil // Not an ancestor
		}
		return false, fmt.Errorf("failed to check ancestry: %w", err)
	}
	return true, nil
}

// branchHasCommits checks if the current branch has any commits beyond its parent
func branchHasCommits(branch string) (bool, error) {
	// Get parent branch
	metadata, err := stack.ReadBranchMetadata(branch)
	if err != nil {
		return false, err
	}

	if metadata.Parent == "" {
		// No parent, check if branch has any commits at all
		cmd := exec.Command("git", "rev-list", "--count", "HEAD")
		output, err := cmd.Output()
		if err != nil {
			return false, fmt.Errorf("failed to count commits: %w", err)
		}
		count := strings.TrimSpace(string(output))
		return count != "0", nil
	}

	// Check if there are commits between parent and current branch
	cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", metadata.Parent, branch))
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to count commits: %w", err)
	}

	count := strings.TrimSpace(string(output))
	return count != "0", nil
}
