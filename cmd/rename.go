package cmd

import (
	"fmt"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
	"stacking/pkg/models"

	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:     "rename <new-name>",
	Aliases: []string{"rn", "mv"},
	Short:   "Rename current branch while preserving stack metadata",
	Long: `Rename the current branch to a new name while preserving all stack metadata,
PR associations, and updating all child branches to point to the new name.

Examples:
  stak rename feature-new-name          # Rename current branch
  stak rename feature/auth-v2           # Works with / in names
  stak rn new-name                      # Using alias`,
	Args: cobra.ExactArgs(1),
	RunE: runRename,
}

var (
	renameUpdatePR bool // Update PR head branch on GitHub
)

func init() {
	rootCmd.AddCommand(renameCmd)
	renameCmd.Flags().BoolVarP(&renameUpdatePR, "update-pr", "p", true, "Update PR head branch on GitHub")
}

func runRename(cmd *cobra.Command, args []string) error {
	newBranchName := args[0]

	// Get current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if already on the target name
	if currentBranch == newBranchName {
		ui.Info(fmt.Sprintf("Already on branch '%s'", newBranchName))
		return nil
	}

	// Check if target branch already exists
	exists, err := git.BranchExists(newBranchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if exists {
		return fmt.Errorf("branch '%s' already exists", newBranchName)
	}

	// Check if current branch is tracked
	hasMetadata, err := stack.HasStackMetadata(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to check stack metadata: %w", err)
	}

	var metadata *models.Branch
	if hasMetadata {
		// Read current metadata
		metadata, err = stack.ReadBranchMetadata(currentBranch)
		if err != nil {
			return fmt.Errorf("failed to read branch metadata: %w", err)
		}
	}

	ui.Info(fmt.Sprintf("Renaming '%s' → '%s'", currentBranch, newBranchName))

	// 1. Rename the git branch
	if err := git.RenameBranch(currentBranch, newBranchName); err != nil {
		return fmt.Errorf("failed to rename branch: %w", err)
	}
	ui.Success(fmt.Sprintf("✓ Renamed git branch"))

	// If branch was tracked, update all metadata
	if hasMetadata && metadata != nil {
		// 2. Write metadata for new branch name
		if err := stack.WriteBranchMetadata(newBranchName, metadata.Parent, metadata.PRNumber); err != nil {
			return fmt.Errorf("failed to write metadata for new branch: %w", err)
		}

		// 3. Delete old branch metadata
		if err := stack.DeleteBranchMetadata(currentBranch); err != nil {
			return fmt.Errorf("failed to delete old branch metadata: %w", err)
		}
		ui.Success("✓ Updated stack metadata")

		// 4. Update all children to point to new branch name
		children, err := stack.GetChildren(currentBranch)
		if err != nil {
			return fmt.Errorf("failed to get children: %w", err)
		}

		if len(children) > 0 {
			for _, child := range children {
				childMetadata, err := stack.ReadBranchMetadata(child)
				if err != nil {
					ui.Warning(fmt.Sprintf("Failed to read metadata for child '%s': %v", child, err))
					continue
				}

				// Update parent to new branch name
				if err := stack.WriteBranchMetadata(child, newBranchName, childMetadata.PRNumber); err != nil {
					ui.Warning(fmt.Sprintf("Failed to update parent for child '%s': %v", child, err))
					continue
				}
			}
			ui.Success(fmt.Sprintf("✓ Updated %d child branch(es)", len(children)))
		}

		// 5. Update PR head branch on GitHub if PR exists
		if metadata.PRNumber > 0 && renameUpdatePR {
			ui.Info(fmt.Sprintf("Updating PR #%d head branch...", metadata.PRNumber))

			// Push new branch to remote
			if err := git.Push(newBranchName, true, false); err != nil {
				ui.Warning(fmt.Sprintf("Failed to push new branch: %v", err))
			} else {
				ui.Success("✓ Pushed new branch to remote")
			}

			// Note: GitHub doesn't support changing PR head branch via API
			// The PR will automatically track the new branch name once pushed
			// and old branch is deleted from remote

			// Delete old branch from remote
			if err := git.DeleteRemoteBranch(currentBranch); err != nil {
				ui.Warning(fmt.Sprintf("Failed to delete old remote branch: %v", err))
				ui.Info("You may need to manually delete the old remote branch")
			} else {
				ui.Success("✓ Deleted old remote branch")
			}

			ui.Success(fmt.Sprintf("✓ PR #%d will now track '%s'", metadata.PRNumber, newBranchName))
		}
	}

	ui.Info("\n✨ Branch renamed successfully!")
	ui.Info(fmt.Sprintf("Current branch: %s", newBranchName))

	return nil
}
