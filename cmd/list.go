package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all stacked branches",
	Long:  `Display a tree visualization of all stacked branches and their relationships.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runList(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList() error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Get current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Build the stack
	s, err := stack.BuildStack()
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	// Display the stack
	ui.DisplayStack(s, currentBranch)

	return nil
}
