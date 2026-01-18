package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var (
	syncRecursive   bool
	syncCurrentOnly bool
	syncContinue    bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync stack with remote",
	Long: `Sync the current branch and its children with remote changes.
Rebases the current branch onto its parent and recursively syncs all child branches.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSync(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	syncCmd.Flags().BoolVarP(&syncRecursive, "recursive", "r", true, "Sync child branches recursively")
	syncCmd.Flags().BoolVar(&syncCurrentOnly, "current-only", false, "Only sync current branch, skip children")
	syncCmd.Flags().BoolVar(&syncContinue, "continue", false, "Continue sync after resolving conflicts")
	rootCmd.AddCommand(syncCmd)
}

func runSync() error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Handle --continue flag
	if syncContinue {
		return continueSyncAfterConflict()
	}

	// Check if there's already a rebase in progress
	inProgress, err := git.IsRebaseInProgress()
	if err != nil {
		return fmt.Errorf("failed to check rebase status: %w", err)
	}
	if inProgress {
		return fmt.Errorf("rebase already in progress. Resolve conflicts and run: stak sync --continue")
	}

	// Get current branch to return to it later
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Fetch from remote
	ui.Info("Fetching from remote")
	if err := git.Fetch(); err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	// Find the base branch (the root of the stack - usually main)
	baseBranch, err := findBaseBranch(currentBranch)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not find base branch: %v", err))
	} else if baseBranch != "" {
		// Update base branch (main) from remote first
		ui.Info(fmt.Sprintf("Updating base branch %s from remote", baseBranch))
		if err := updateLocalBranchFromRemote(baseBranch); err != nil {
			ui.Warning(fmt.Sprintf("Could not update %s from remote: %v", baseBranch, err))
		}
	}

	// First, check and clean up all merged branches in the stack
	// This ensures we don't try to rebase onto a merged branch
	if err := cleanupMergedBranchesInStack(currentBranch); err != nil {
		return err
	// Get ALL branches with stack metadata
	allStackBranches, err := stack.GetAllStackBranches()
	if err != nil {
		return fmt.Errorf("failed to get stack branches: %w", err)
	}

	// Check if current branch was deleted during cleanup
	exists, err := git.BranchExists(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if !exists {
		// Current branch was merged and deleted
		ui.Success("Sync completed successfully")

	if len(allStackBranches) == 0 {
		ui.Warning("No stack branches found")
		return nil
	}

	ui.Info(fmt.Sprintf("Syncing %d stack branch(es)", len(allStackBranches)))

	// Find all unique base branches and update them first
	baseBranches := make(map[string]bool)
	for _, branch := range allStackBranches {
		parent, err := stack.GetParent(branch)
		if err != nil || parent == "" {
			continue
		}
		// Check if parent is also in stack
		parentInStack := false
		for _, b := range allStackBranches {
			if b == parent {
				parentInStack = true
				break
			}
		}
		// If parent is not in stack, it's a base branch (like main)
		if !parentInStack {
			baseBranches[parent] = true
		}
	}

	// Update all base branches (main, etc.) from remote
	for baseBranch := range baseBranches {
		ui.Info(fmt.Sprintf("Updating base branch %s from remote", baseBranch))
		if err := updateLocalBranchFromRemote(baseBranch); err != nil {
			ui.Warning(fmt.Sprintf("Could not update %s from remote: %v", baseBranch, err))
		}
	}

	// Clean up all merged branches first
	ui.Info("Checking for merged branches")
	for _, branch := range allStackBranches {
		exists, err := git.BranchExists(branch)
		if err != nil || !exists {
			continue
		}
		checkAndCleanupMergedBranch(branch)
	}

	// Get updated list after cleanup
	allStackBranches, err = stack.GetAllStackBranches()
	if err != nil {
		return fmt.Errorf("failed to get stack branches: %w", err)
	}

	// Sync branches in dependency order (parents before children)
	syncedBranches := make(map[string]bool)
	maxIterations := len(allStackBranches) + 1
	iteration := 0

	for len(syncedBranches) < len(allStackBranches) && iteration < maxIterations {
		iteration++
		progressMade := false

		for _, branch := range allStackBranches {
			if syncedBranches[branch] {
				continue
			}

			// Check if branch still exists
			exists, err := git.BranchExists(branch)
			if err != nil || !exists {
				syncedBranches[branch] = true
				continue
			}

			// Get parent
			parent, err := stack.GetParent(branch)
			if err != nil {
				ui.Warning(fmt.Sprintf("Could not get parent for %s: %v", branch, err))
				syncedBranches[branch] = true
				continue
			}

			// Check if parent is in stack
			parentInStack := false
			for _, b := range allStackBranches {
				if b == parent {
					parentInStack = true
					break
				}
			}

			// Can sync if: no parent, parent not in stack, or parent already synced
			if parent == "" || !parentInStack || syncedBranches[parent] {
				if err := syncBranch(branch); err != nil {
					ui.Warning(fmt.Sprintf("Failed to sync %s: %v", branch, err))
				}
				syncedBranches[branch] = true
				progressMade = true
			}
		}

		if !progressMade {
			break
		}
	}

	// Return to original branch
	if err := git.CheckoutBranch(currentBranch); err != nil {
		ui.Warning(fmt.Sprintf("Could not return to %s: %v", currentBranch, err))
	}

	ui.Success("Sync completed successfully")

	// Update stack visualization on all PRs
	ui.Info("Updating stack comments on GitHub")
	if err := updateStackComments(currentBranch); err != nil {
		ui.Warning(fmt.Sprintf("Failed to update stack comments: %v", err))
		// Don't fail the whole operation if comments fail
	}

	return nil
}

func syncBranch(branch string) error {
	ui.Info(fmt.Sprintf("Syncing branch %s", branch))

	// Get parent
	parent, err := stack.GetParent(branch)
	if err != nil {
		return fmt.Errorf("failed to get parent for branch %s: %w", branch, err)
	}

	if parent == "" {
		ui.Info(fmt.Sprintf("Branch %s has no parent, skipping rebase", branch))
		return nil
	}

	// Update local parent branch to match remote (if it exists locally and remotely)
	if err := updateLocalBranchFromRemote(parent); err != nil {
		// Don't fail if we can't update parent, just warn
		ui.Warning(fmt.Sprintf("Could not update local %s from remote: %v", parent, err))
	}

	// Checkout the branch
	if err := git.CheckoutBranch(branch); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
	}

	// Rebase onto parent
	ui.Info(fmt.Sprintf("Rebasing %s onto origin/%s", branch, parent))
	onto := fmt.Sprintf("origin/%s", parent)
	if err := git.RebaseOnto(onto); err != nil {
		if conflictErr, ok := err.(*git.RebaseConflictError); ok {
			return handleRebaseConflict(branch, conflictErr)
		}
		return fmt.Errorf("failed to rebase: %w", err)
	}

	// Push with force-with-lease
	ui.Info(fmt.Sprintf("Force pushing %s", branch))
	if err := git.Push(branch, false, true); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	ui.Success(fmt.Sprintf("Synced %s", branch))
	return nil
}

func syncBranchRecursive(branch string) error {
	// Check if branch still exists (might have been cleaned up)
	exists, err := git.BranchExists(branch)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if !exists {
		// Branch was merged and deleted, skip it
		return nil
	}

	// Sync this branch
	if err := syncBranch(branch); err != nil {
		return err
	}

	// Get children and sync them
	children, err := stack.GetChildren(branch)
	if err != nil {
		return fmt.Errorf("failed to get children of %s: %w", branch, err)
	}

	for _, child := range children {
		if err := syncBranchRecursive(child); err != nil {
			return err
		}
	}

	return nil
}

func handleRebaseConflict(branch string, conflictErr *git.RebaseConflictError) error {
	files, err := git.GetConflictedFiles()
	if err != nil {
		ui.Warning("Could not get conflicted files")
		files = []string{}
	}

	fmt.Println()
	ui.Error("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	ui.Error(fmt.Sprintf("  🔀 Rebase conflict on branch: %s", branch))
	ui.Error("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	if len(files) > 0 {
		fmt.Println("📝 Conflicted files:")
		for _, file := range files {
			fmt.Printf("   • %s\n", file)
		}
		fmt.Println()
	}

	fmt.Println("🔧 How to resolve conflicts:")
	fmt.Println()
	fmt.Println("   1️⃣  Open the conflicted files in your editor")
	fmt.Println("      Look for conflict markers:")
	fmt.Println("      <<<<<<< HEAD")
	fmt.Println("      your changes")
	fmt.Println("      =======")
	fmt.Println("      incoming changes")
	fmt.Println("      >>>>>>> parent branch")
	fmt.Println()
	fmt.Println("   2️⃣  Edit the files to keep the code you want")
	fmt.Println("      Remove the conflict markers (<<<<<<<, =======, >>>>>>>)")
	fmt.Println()
	fmt.Println("   3️⃣  Stage the resolved files:")
	if len(files) > 0 {
		for _, file := range files {
			fmt.Printf("      git add %s\n", file)
		}
	} else {
		fmt.Println("      git add <resolved-file>")
	}
	fmt.Println()
	fmt.Println("   4️⃣  Continue the sync:")
	fmt.Println("      stak sync --continue")
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("⚠️  To abort and undo the rebase:")
	fmt.Println("   git rebase --abort")
	fmt.Println()

	return fmt.Errorf("rebase conflict detected")
}

func continueSyncAfterConflict() error {
	// Check if rebase is in progress
	inProgress, err := git.IsRebaseInProgress()
	if err != nil {
		return fmt.Errorf("failed to check rebase status: %w", err)
	}
	if !inProgress {
		ui.Warning("No rebase in progress")
		fmt.Println("\nTip: Run 'stak sync' to start syncing your branches")
		return fmt.Errorf("no rebase in progress")
	}

	// Check if there are still conflicts
	hasConflicts, err := git.HasMergeConflicts()
	if err != nil {
		return fmt.Errorf("failed to check for conflicts: %w", err)
	}
	if hasConflicts {
		files, _ := git.GetConflictedFiles()

		fmt.Println()
		ui.Error("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		ui.Error("  ⚠️  Conflicts still unresolved")
		ui.Error("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		fmt.Println("📝 Files still have conflicts:")
		for _, file := range files {
			fmt.Printf("   • %s\n", file)
		}
		fmt.Println()
		fmt.Println("🔧 You need to:")
		fmt.Println("   1. Open and edit these files to resolve conflicts")
		fmt.Println("   2. Remove conflict markers (<<<<<<<, =======, >>>>>>>)")
		fmt.Println("   3. Stage the resolved files:")
		for _, file := range files {
			fmt.Printf("      git add %s\n", file)
		}
		fmt.Println("   4. Run: stak sync --continue")
		fmt.Println()

		return fmt.Errorf("resolve all conflicts before continuing")
	}

	// All conflicts resolved, continue rebase
	fmt.Println()
	ui.Info("✅ All conflicts resolved! Continuing rebase...")
	if err := git.ContinueRebase(); err != nil {
		return fmt.Errorf("failed to continue rebase: %w", err)
	}

	// Get current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Push
	ui.Info(fmt.Sprintf("Force pushing %s", currentBranch))
	if err := git.Push(currentBranch, false, true); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	fmt.Println()
	ui.Success("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	ui.Success("  🎉 Sync completed successfully!")
	ui.Success("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	return nil
}

// findBaseBranch finds the root branch of the stack (the one with no parent or non-stack parent)
func findBaseBranch(branch string) (string, error) {
	// Walk up the stack to find the base
	current := branch
	for {
		// Check if this branch has stack metadata
		hasMetadata, err := stack.HasStackMetadata(current)
		if err != nil {
			return "", err
		}

		if !hasMetadata {
			// This branch is not in the stack, so the previous one was the base
			// But we want to return this branch as it's likely main
			return current, nil
		}

		// Get parent
		parent, err := stack.GetParent(current)
		if err != nil {
			return "", err
		}

		if parent == "" {
			// No parent, this is the base
			return current, nil
		}

		// Check if parent has stack metadata
		parentHasMetadata, err := stack.HasStackMetadata(parent)
		if err != nil {
			return "", err
		}

		if !parentHasMetadata {
			// Parent is not in stack (likely main), return it as base
			return parent, nil
		}

		// Move up to parent
		current = parent
	}
}

// updateLocalBranchFromRemote updates a local branch to match its remote counterpart
func updateLocalBranchFromRemote(branch string) error {
	// Check if branch exists locally
	localExists, err := git.BranchExists(branch)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if !localExists {
		// Branch doesn't exist locally, nothing to update
		return nil
	}

	// Check if remote branch exists
	remoteExists, err := git.RemoteBranchExists(branch)
	if err != nil {
		return fmt.Errorf("failed to check if remote branch exists: %w", err)
	}
	if !remoteExists {
		// No remote branch, nothing to update
		return nil
	}

	// Save current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Checkout the branch to update
	if err := git.CheckoutBranch(branch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", branch, err)
	}

	// Reset to match remote
	ui.Info(fmt.Sprintf("Updating local %s to match origin/%s", branch, branch))
	if err := git.ResetToRemote(branch); err != nil {
		// Try to go back to original branch
		git.CheckoutBranch(currentBranch)
		return fmt.Errorf("failed to reset %s to origin/%s: %w", branch, branch, err)
	}

	// Return to original branch
	if err := git.CheckoutBranch(currentBranch); err != nil {
		return fmt.Errorf("failed to return to %s: %w", currentBranch, err)
	}

	return nil
}

// cleanupMergedBranchesInStack checks all branches in the stack and cleans up any that are merged
func cleanupMergedBranchesInStack(currentBranch string) error {
	// Get all ancestors
	ancestors, err := stack.GetAncestors(currentBranch)
	if err != nil {
		// If we can't get ancestors, just continue - don't fail
		ui.Warning(fmt.Sprintf("Could not get ancestors: %v", err))
		ancestors = []string{}
	}

	// Get all descendants
	descendants, err := stack.GetDescendants(currentBranch)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not get descendants: %v", err))
		descendants = []string{}
	}

	// Build full list: ancestors (bottom to top) + current + descendants
	allBranches := append(ancestors, currentBranch)
	allBranches = append(allBranches, descendants...)

	// Check each branch for merged PR and clean up
	for _, branch := range allBranches {
		// Check if branch still exists locally
		exists, err := git.BranchExists(branch)
		if err != nil || !exists {
			continue
		}

		// Check and clean up if merged
		_, err = checkAndCleanupMergedBranch(branch)
		if err != nil {
			// Don't fail the whole operation, just warn
			ui.Warning(fmt.Sprintf("Error checking branch %s: %v", branch, err))
		}
	}

	return nil
}

// updateLocalBranchFromRemote updates a local branch to match its remote counterpart
func updateLocalBranchFromRemote(branch string) error {
	// Check if branch exists locally
	localExists, err := git.BranchExists(branch)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if !localExists {
		return nil
	}

	// Check if remote branch exists
	remoteExists, err := git.RemoteBranchExists(branch)
	if err != nil {
		return fmt.Errorf("failed to check if remote branch exists: %w", err)
	}
	if !remoteExists {
		return nil
	}

	// Save current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Checkout the branch to update
	if err := git.CheckoutBranch(branch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", branch, err)
	}

	// Reset to match remote
	if err := git.ResetToRemote(branch); err != nil {
		git.CheckoutBranch(currentBranch)
		return fmt.Errorf("failed to reset %s to origin/%s: %w", branch, branch, err)
	}

	// Return to original branch
	if err := git.CheckoutBranch(currentBranch); err != nil {
		return fmt.Errorf("failed to return to %s: %w", currentBranch, err)
	}

	return nil
}

// checkAndCleanupMergedBranch checks if a branch's PR is merged on GitHub
// and cleans up the local branch and metadata if so
func checkAndCleanupMergedBranch(branch string) (bool, error) {
	// Get branch metadata
	metadata, err := stack.ReadBranchMetadata(branch)
	if err != nil {
		return false, fmt.Errorf("failed to read metadata for %s: %w", branch, err)
	}

	// If no PR exists, nothing to check
	if metadata.PRNumber == 0 {
		return false, nil
	}

	// Check PR status on GitHub
	status, err := github.GetPRStatus(metadata.PRNumber)
	if err != nil {
		// If we can't get PR status, don't fail - just skip cleanup
		ui.Warning(fmt.Sprintf("Could not check PR status for %s: %v", branch, err))
		return false, nil
	}

	// If PR is not merged, nothing to clean up
	if !status.IsMerged() {
		return false, nil
	}

	// PR is merged, clean up the branch
	ui.Info(fmt.Sprintf("PR #%d for branch %s is merged, cleaning up", metadata.PRNumber, branch))

	// Get parent before deleting metadata
	parentBranch := metadata.Parent

	// Get children to update their parent
	children, err := stack.GetChildren(branch)
	if err != nil {
		return false, fmt.Errorf("failed to get children of %s: %w", branch, err)
	}

	// Update each child's parent to point to this branch's parent
	for _, child := range children {
		childMetadata, err := stack.ReadBranchMetadata(child)
		if err != nil {
			ui.Warning(fmt.Sprintf("Could not read metadata for child %s: %v", child, err))
			continue
		}

		ui.Info(fmt.Sprintf("Updating %s parent: %s → %s", child, branch, parentBranch))
		if err := stack.WriteBranchMetadata(child, parentBranch, childMetadata.PRNumber); err != nil {
			ui.Warning(fmt.Sprintf("Could not update metadata for %s: %v", child, err))
		}

		// Update PR base on GitHub if PR exists
		if childMetadata.PRNumber > 0 {
			if err := github.UpdatePRBase(childMetadata.PRNumber, parentBranch); err != nil {
				ui.Warning(fmt.Sprintf("Could not update PR #%d base: %v", childMetadata.PRNumber, err))
			} else {
				ui.Info(fmt.Sprintf("Updated PR #%d base to %s", childMetadata.PRNumber, parentBranch))
			}
		}
	}

	// Get current branch so we can switch away if needed
	currentBranch, _ := git.GetCurrentBranch()
	if currentBranch == branch {
		// Switch to parent branch first
		if parentBranch != "" {
			ui.Info(fmt.Sprintf("Switching to %s", parentBranch))
			if err := git.CheckoutBranch(parentBranch); err != nil {
				return false, fmt.Errorf("failed to checkout %s: %w", parentBranch, err)
			}
		}
	}

	// Delete local branch
	ui.Info(fmt.Sprintf("Deleting local branch %s", branch))
	if err := git.DeleteBranch(branch, false); err != nil {
		ui.Warning(fmt.Sprintf("Could not delete branch %s: %v", branch, err))
	} else {
		ui.Success(fmt.Sprintf("Deleted branch %s", branch))
	}

	// Delete metadata
	if err := stack.DeleteBranchMetadata(branch); err != nil {
		ui.Warning(fmt.Sprintf("Could not delete metadata for %s: %v", branch, err))
	}

	return true, nil
}
