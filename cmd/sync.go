package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"stacking/internal/git"
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
		return fmt.Errorf("branch %s is not part of a stack. Use 'stak create' to create a stacked PR", currentBranch)
	}

	// Fetch from remote
	ui.Info("Fetching from remote")
	if err := git.Fetch(); err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	// Sync current branch
	if err := syncBranch(currentBranch); err != nil {
		return err
	}

	// Sync children if recursive and not current-only
	if !syncCurrentOnly && syncRecursive {
		children, err := stack.GetChildren(currentBranch)
		if err != nil {
			return fmt.Errorf("failed to get children: %w", err)
		}

		if len(children) > 0 {
			ui.Info(fmt.Sprintf("Syncing %d child branch(es)", len(children)))
			for _, child := range children {
				if err := syncBranchRecursive(child); err != nil {
					return err
				}
			}
		}
	}

	ui.Success("Sync completed successfully")
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

	ui.Error(fmt.Sprintf("Rebase conflict on branch %s", branch))
	if len(files) > 0 {
		fmt.Println("\nConflicted files:")
		for _, file := range files {
			fmt.Printf("  - %s\n", file)
		}
	}

	fmt.Println("\nTo resolve:")
	fmt.Println("  1. Fix conflicts in the files above")
	fmt.Println("  2. Stage resolved files: git add <file>")
	fmt.Println("  3. Continue sync: stak sync --continue")
	fmt.Println("\nOr abort: git rebase --abort")

	return fmt.Errorf("rebase conflict - resolve and continue")
}

func continueSyncAfterConflict() error {
	// Check if rebase is in progress
	inProgress, err := git.IsRebaseInProgress()
	if err != nil {
		return fmt.Errorf("failed to check rebase status: %w", err)
	}
	if !inProgress {
		return fmt.Errorf("no rebase in progress")
	}

	// Check if there are still conflicts
	hasConflicts, err := git.HasMergeConflicts()
	if err != nil {
		return fmt.Errorf("failed to check for conflicts: %w", err)
	}
	if hasConflicts {
		files, _ := git.GetConflictedFiles()
		fmt.Println("Still have conflicts in:")
		for _, file := range files {
			fmt.Printf("  - %s\n", file)
		}
		return fmt.Errorf("resolve all conflicts before continuing")
	}

	// Continue rebase
	ui.Info("Continuing rebase")
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

	ui.Success("Sync completed successfully")
	return nil
}
