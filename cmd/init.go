package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/ui"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize repository for stack",
	Long: `Initialize the current repository for using stack.
Verifies git repository and GitHub CLI authentication.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runInit(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit() error {
	ui.Info("Initializing repository for stack")

	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository. Run: git init")
	}
	ui.Success("Git repository detected")

	// Check for remote
	remoteURL, err := git.GetRemoteURL()
	if err != nil {
		ui.Warning("No remote repository configured")
		ui.Info("You can add a remote with: git remote add origin <url>")
	} else {
		ui.Success(fmt.Sprintf("Remote repository: %s", remoteURL))
	}

	// Check if gh CLI is installed
	ui.Info("Checking GitHub CLI (gh)")
	if !github.IsGHAuthenticated() {
		ui.Warning("GitHub CLI not authenticated")
		ui.Info("Authenticate with: gh auth login")
	} else {
		ui.Success("GitHub CLI authenticated")
	}

	// Get current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		ui.Warning("Could not determine current branch")
	} else {
		ui.Info(fmt.Sprintf("Current branch: %s", currentBranch))
	}

	ui.Success("Repository initialized for stack")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Create a new branch from your base branch")
	fmt.Println("  2. Make commits and run: stack create --title \"Your PR title\"")
	fmt.Println("  3. Use 'stack list' to visualize your stack")
	fmt.Println("  4. Use 'stack sync' to keep branches in sync")
	fmt.Println("  5. Use 'stack submit --all' to merge your stack")

	return nil
}
