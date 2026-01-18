package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var (
	createTitle string
	createBody  string
	createDraft bool
)

var createCmd = &cobra.Command{
	Use:   "create [branch-name]",
	Short: "Create a new stacked PR",
	Long: `Create a new branch stacked on top of the current branch and create a pull request.
The new PR will target the current branch as its base.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var branchName string
		if len(args) > 0 {
			branchName = args[0]
		}

		if err := runCreate(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	createCmd.Flags().StringVarP(&createTitle, "title", "t", "", "PR title")
	createCmd.Flags().StringVarP(&createBody, "body", "b", "", "PR body/description")
	createCmd.Flags().BoolVar(&createDraft, "draft", false, "Create as draft PR")
	rootCmd.AddCommand(createCmd)
}

func runCreate(branchName string) error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Check if gh CLI is authenticated
	if !github.IsGHAuthenticated() {
		return fmt.Errorf("gh CLI not authenticated. Run: gh auth login")
	}

	// Get current branch (will be the parent)
	parentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Prompt for branch name if not provided
	if branchName == "" {
		fmt.Print("Enter new branch name: ")
		reader := bufio.NewReader(os.Stdin)
		branchName, err = reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read branch name: %w", err)
		}
		branchName = strings.TrimSpace(branchName)
	}

	if branchName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Check if branch already exists
	exists, err := git.BranchExists(branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if exists {
		return fmt.Errorf("branch %s already exists", branchName)
	}

	// Create and checkout new branch
	ui.Info(fmt.Sprintf("Creating branch %s from %s", branchName, parentBranch))
	if err := git.CreateBranch(branchName); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// Store metadata with parent branch (PR number will be set when submitted)
	if err := stack.WriteBranchMetadata(branchName, parentBranch, 0); err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	ui.Success(fmt.Sprintf("Created and checked out branch %s", branchName))
	ui.Info("Now make your changes and commit them.")
	ui.Info(fmt.Sprintf("When ready, run: stak submit"))
	ui.Info("The PR description will be auto-generated from your commits.")

	return nil
}
