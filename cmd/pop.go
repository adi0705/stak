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
	popKeep  bool
	popForce bool
)

var popCmd = &cobra.Command{
	Use:     "pop [branch]",
	Aliases: []string{"pp"},
	Short:   "Remove branch from stack, keeping changes",
	Long:    `Pop a branch from the stack, preserving its changes locally. The changes are stashed and can be applied to the parent branch or discarded.`,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := ""
		if len(args) > 0 {
			branchName = args[0]
		}

		if err := runPop(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	popCmd.Flags().BoolVar(&popKeep, "keep", false, "Keep the branch (don't delete it)")
	popCmd.Flags().BoolVarP(&popForce, "force", "f", false, "Skip confirmation prompts")
	rootCmd.AddCommand(popCmd)
}

func runPop(branchName string) error {
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
		parent = "main" // fallback
	}

	// Get children
	children, err := stack.GetChildren(branchName)
	if err != nil {
		return fmt.Errorf("failed to get children: %w", err)
	}

	// Show confirmation
	if !popForce {
		ui.Info("This will:")
		ui.Info(fmt.Sprintf("  - Stash changes from %s", branchName))
		ui.Info(fmt.Sprintf("  - Switch to %s", parent))
		if len(children) > 0 {
			ui.Info(fmt.Sprintf("  - Update %d child branch(es) to point to %s", len(children), parent))
		}
		if !popKeep {
			ui.Info(fmt.Sprintf("  - Delete local branch %s", branchName))
		}
		if metadata.PRNumber > 0 {
			ui.Info(fmt.Sprintf("  - Close PR #%d", metadata.PRNumber))
		}
		ui.Info("  - Remove stack metadata")

		prompt := promptui.Select{
			Label: "Proceed with pop?",
			Items: []string{"Yes", "No"},
		}

		_, result, err := prompt.Run()
		if err != nil || result == "No" {
			ui.Info("Pop cancelled")
			return nil
		}
	}

	// Checkout the branch
	currentBranch, _ := git.GetCurrentBranch()
	if currentBranch != branchName {
		ui.Info(fmt.Sprintf("Checking out %s", branchName))
		if err := git.CheckoutBranch(branchName); err != nil {
			return fmt.Errorf("failed to checkout branch: %w", err)
		}
	}

	// Check for uncommitted changes
	hasChanges, err := git.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}

	stashCreated := false
	if hasChanges {
		// Stash changes
		ui.Info("Stashing changes")
		cmd := exec.Command("git", "stash", "push", "-m", fmt.Sprintf("stak-pop-%s", branchName))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to stash changes: %w", err)
		}
		stashCreated = true
		ui.Success("Changes stashed")
	}

	// Switch to parent
	ui.Info(fmt.Sprintf("Switching to %s", parent))
	if err := git.CheckoutBranch(parent); err != nil {
		return fmt.Errorf("failed to checkout parent: %w", err)
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
	}

	// Close PR if exists
	if metadata.PRNumber > 0 {
		ui.Info(fmt.Sprintf("Closing PR #%d", metadata.PRNumber))
		if err := github.ClosePR(metadata.PRNumber); err != nil {
			ui.Warning(fmt.Sprintf("Could not close PR #%d: %v", metadata.PRNumber, err))
		} else {
			ui.Success(fmt.Sprintf("Closed PR #%d", metadata.PRNumber))
		}
	}

	// Delete branch if not keeping
	if !popKeep {
		ui.Info(fmt.Sprintf("Deleting local branch %s", branchName))
		if err := git.DeleteBranch(branchName, true); err != nil {
			ui.Warning(fmt.Sprintf("Could not delete branch %s: %v", branchName, err))
		} else {
			ui.Success(fmt.Sprintf("Deleted branch %s", branchName))
		}
	}

	// Delete metadata
	if err := stack.DeleteBranchMetadata(branchName); err != nil {
		ui.Warning(fmt.Sprintf("Could not delete metadata: %v", err))
	}

	// Inform about stashed changes
	if stashCreated {
		ui.Info("")
		ui.Info("Your changes have been stashed.")
		ui.Info("To apply them to the current branch:")
		ui.Info("  git stash pop")
		ui.Info("")
		ui.Info("To discard them:")
		ui.Info("  git stash drop")
	}

	ui.Success(fmt.Sprintf("Popped %s from stack", branchName))
	return nil
}
