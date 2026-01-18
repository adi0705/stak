package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var (
	modifyAmend     bool
	modifyRebaseNum int
)

var modifyCmd = &cobra.Command{
	Use:   "modify",
	Short: "Modify commits locally",
	Long: `Modify the current branch by opening an interactive commit amend window (default),
or by rebasing. Changes are made locally only.

By default, opens the commit editor to amend the last commit. Use flags for other operations.
After making changes, run 'stak submit' to push changes and update the PR.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runModify(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	modifyCmd.Flags().BoolVar(&modifyAmend, "amend", false, "Amend the last commit (default if no flags)")
	modifyCmd.Flags().IntVar(&modifyRebaseNum, "rebase", 0, "Interactive rebase last N commits")
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

	// If no modification flags were provided, open commit amend by default
	if !modifyAmend && modifyRebaseNum == 0 {
		modifyAmend = true
	}

	// Handle amend
	if modifyAmend {
		// Get tree hash before amending (tree = all file contents)
		beforeTreeCmd := exec.Command("git", "log", "-1", "--format=%T")
		beforeTreeOutput, err := beforeTreeCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get tree hash: %w", err)
		}
		beforeTree := strings.TrimSpace(string(beforeTreeOutput))

		ui.Info("Opening editor to amend last commit")
		cmd := exec.Command("git", "commit", "--amend")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmdErr := cmd.Run()

		// Get tree hash after
		afterTreeCmd := exec.Command("git", "log", "-1", "--format=%T")
		afterTreeOutput, err := afterTreeCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get tree hash: %w", err)
		}
		afterTree := strings.TrimSpace(string(afterTreeOutput))

		// If tree unchanged, no actual file changes were made
		if beforeTree == afterTree {
			ui.Warning("No file changes made. Aborting.")
			return nil
		}

		// If there was an error, report it
		if cmdErr != nil {
			return fmt.Errorf("failed to amend commit: %w", cmdErr)
		}

		ui.Success("Commit amended successfully")
	}

	// Handle interactive rebase
	if modifyRebaseNum > 0 {
		ui.Info(fmt.Sprintf("Starting interactive rebase for last %d commits", modifyRebaseNum))
		cmd := exec.Command("git", "rebase", "-i", fmt.Sprintf("HEAD~%d", modifyRebaseNum))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			// Check if user aborted the rebase
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() == 1 {
					ui.Warning("Rebase canceled. No changes were made.")
					return nil
				}
			}
			return fmt.Errorf("failed to rebase: %w", err)
		}
		ui.Success("Rebase completed successfully")
	}

	ui.Success("Changes modified locally")
	ui.Info("Run 'stak submit' to push changes and update the PR")
	return nil
}
