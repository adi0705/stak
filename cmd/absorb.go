package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var absorbCmd = &cobra.Command{
	Use:     "absorb",
	Aliases: []string{"ab"},
	Short:   "Distribute staged changes to appropriate commits",
	Long:    `Automatically determine which commits staged changes belong to and amend them appropriately. Requires git-absorb to be installed.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runAbsorb(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(absorbCmd)
}

func runAbsorb() error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Get current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if branch is tracked
	hasMetadata, err := stack.HasStackMetadata(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to check stack metadata: %w", err)
	}
	if !hasMetadata {
		return fmt.Errorf("branch %s is not tracked", currentBranch)
	}

	// Check if there are staged changes
	hasStagedChanges, err := git.HasStagedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for staged changes: %w", err)
	}
	if !hasStagedChanges {
		return fmt.Errorf("no staged changes to absorb")
	}

	// Check if git-absorb is installed
	checkCmd := exec.Command("git", "absorb", "--version")
	if err := checkCmd.Run(); err != nil {
		// git-absorb not found, provide installation instructions
		ui.Error("git-absorb is not installed")
		ui.Info("")
		ui.Info("Install git-absorb:")
		ui.Info("  macOS:   brew install git-absorb")
		ui.Info("  Linux:   cargo install git-absorb")
		ui.Info("  Windows: cargo install git-absorb")
		ui.Info("")
		ui.Info("See: https://github.com/tummychow/git-absorb")
		return fmt.Errorf("git-absorb is required for this command")
	}

	ui.Info("Running git absorb to distribute staged changes")

	// Run git absorb
	absorbCmd := exec.Command("git", "absorb")
	absorbCmd.Stdout = os.Stdout
	absorbCmd.Stderr = os.Stderr
	if err := absorbCmd.Run(); err != nil {
		return fmt.Errorf("git absorb failed: %w", err)
	}

	ui.Success("Changes absorbed into commits")

	// Force push the branch
	ui.Info(fmt.Sprintf("Force pushing %s", currentBranch))
	if err := git.Push(currentBranch, false, true); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// Get children
	children, err := stack.GetChildren(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to get children: %w", err)
	}

	// Sync children
	if len(children) > 0 {
		ui.Info(fmt.Sprintf("Syncing %d child branch(es)", len(children)))
		for _, child := range children {
			if err := syncBranchRecursive(child); err != nil {
				return fmt.Errorf("failed to sync child %s: %w", child, err)
			}
		}

		// Return to original branch
		if err := git.CheckoutBranch(currentBranch); err != nil {
			return fmt.Errorf("failed to return to branch: %w", err)
		}
	}

	ui.Success("Absorb completed successfully")
	return nil
}
