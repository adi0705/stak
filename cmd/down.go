package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Move to a child branch in the stack",
	Long:  `Checkout a child branch (move down one level in the stack).`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDown(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown() error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Get current branch
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Get children branches
	children, err := stack.GetChildren(currentBranch)
	if err != nil {
		return fmt.Errorf("failed to get children: %w", err)
	}

	if len(children) == 0 {
		ui.Info(fmt.Sprintf("Branch %s has no child branches", currentBranch))
		return nil
	}

	var targetBranch string

	// If only one child, use it
	if len(children) == 1 {
		targetBranch = children[0]
	} else {
		// Multiple children - prompt user to select
		fmt.Println("Multiple child branches found:")
		for i, child := range children {
			metadata, err := stack.ReadBranchMetadata(child)
			prInfo := ""
			if err == nil && metadata.PRNumber > 0 {
				prInfo = fmt.Sprintf(" (PR #%d)", metadata.PRNumber)
			}
			fmt.Printf("  %d. %s%s\n", i+1, child, prInfo)
		}

		fmt.Print("\nSelect branch (1-" + fmt.Sprintf("%d", len(children)) + "): ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		selection, err := strconv.Atoi(input)
		if err != nil || selection < 1 || selection > len(children) {
			return fmt.Errorf("invalid selection")
		}

		targetBranch = children[selection-1]
	}

	// Checkout target branch
	ui.Info(fmt.Sprintf("Moving down: %s → %s", currentBranch, targetBranch))
	if err := git.CheckoutBranch(targetBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", targetBranch, err)
	}

	ui.Success(fmt.Sprintf("Now on branch %s", targetBranch))
	return nil
}
