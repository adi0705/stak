package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
	"stacking/pkg/models"
)

var (
	logShort bool
)

var logCmd = &cobra.Command{
	Use:     "log",
	Aliases: []string{"lg"},
	Short:   "Show detailed information about stack branches",
	Long:    `Display detailed information about all branches in the stack, including PR status, reviews, CI checks, and commit counts.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runLog(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	logCmd.Flags().BoolVarP(&logShort, "short", "s", false, "Show short format (same as list)")
	rootCmd.AddCommand(logCmd)
}

func runLog() error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// If short mode, just run list
	if logShort {
		return runList()
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

	// Display detailed stack information
	displayDetailedStack(s, currentBranch)

	return nil
}

func displayDetailedStack(s *models.Stack, currentBranch string) {
	if len(s.Roots) == 0 {
		fmt.Println("No stacked branches found.")
		return
	}

	// Display each root and its descendants
	for _, root := range s.Roots {
		displayBranchDetailed(root, "", currentBranch, true)
	}
}

func displayBranchDetailed(branch *models.Branch, prefix string, currentBranch string, isLast bool) {
	// Determine the branch indicator
	indicator := " "
	if branch.Name == currentBranch {
		indicator = "●"
	}

	// Determine the connector
	connector := "├─"
	if isLast {
		connector = "└─"
	}
	if prefix == "" {
		connector = ""
	}

	// Display branch name with indicator
	branchLine := fmt.Sprintf("%s%s %s %s", prefix, connector, indicator, branch.Name)
	if branch.Parent != "" {
		branchLine += fmt.Sprintf(" (%s)", branch.Parent)
	}
	fmt.Println(branchLine)

	// Get PR details if available
	if branch.PRNumber > 0 {
		details, err := github.GetPRDetails(branch.PRNumber)
		if err != nil {
			// If we can't get details, just show basic info
			detailPrefix := getDetailPrefix(prefix, isLast, false)
			fmt.Printf("%s  PR #%d (error fetching details)\n", detailPrefix, branch.PRNumber)
		} else {
			displayPRDetails(details, prefix, isLast)
		}
	} else {
		// No PR associated
		detailPrefix := getDetailPrefix(prefix, isLast, false)
		fmt.Printf("%s  No PR\n", detailPrefix)
	}

	// Display children recursively
	for i, child := range branch.Children {
		childIsLast := i == len(branch.Children)-1
		childPrefix := prefix
		if prefix == "" {
			childPrefix = " "
		} else if isLast {
			childPrefix = prefix + "   "
		} else {
			childPrefix = prefix + "│  "
		}
		displayBranchDetailed(child, childPrefix, currentBranch, childIsLast)
	}
}

func displayPRDetails(details *github.PRDetails, prefix string, isLast bool) {
	detailPrefix := getDetailPrefix(prefix, isLast, true)

	// PR title and number
	fmt.Printf("%s  PR #%d - %s\n", detailPrefix, details.Number, details.Title)

	// Status line: State, Review, CI
	statusLine := fmt.Sprintf("%s  ", detailPrefix)

	// State with icon
	stateDisplay := details.GetStateDisplay()
	stateIcon := getStateIcon(details.State, details.IsDraft)
	statusLine += fmt.Sprintf("%s %s", stateIcon, stateDisplay)

	// Review status with icon
	reviewStatus := details.GetReviewStatus()
	reviewIcon := getReviewIcon(details.ReviewDecision, details.IsDraft)
	statusLine += fmt.Sprintf("  %s %s", reviewIcon, reviewStatus)

	// CI status with icon
	ciStatus := details.GetCIStatus()
	ciIcon := getCIIcon(ciStatus)
	statusLine += fmt.Sprintf("  %s CI: %s", ciIcon, ciStatus)

	fmt.Println(statusLine)

	// Commit count
	fmt.Printf("%s  %d commit(s)\n", detailPrefix, details.Commits.TotalCount)
}

func getDetailPrefix(prefix string, isLast bool, hasMore bool) string {
	if prefix == "" {
		return ""
	}

	if isLast {
		if hasMore {
			return prefix + "   "
		}
		return prefix + "   "
	}
	return prefix + "│  "
}

func getStateIcon(state string, isDraft bool) string {
	if state == "MERGED" {
		return "✓"
	}
	if state == "CLOSED" {
		return "✗"
	}
	if isDraft {
		return "◐"
	}
	return "○" // Open
}

func getReviewIcon(reviewDecision string, isDraft bool) string {
	if isDraft {
		return "○"
	}
	switch reviewDecision {
	case "APPROVED":
		return "✓"
	case "CHANGES_REQUESTED":
		return "✗"
	case "REVIEW_REQUIRED", "":
		return "⚠"
	default:
		return "○"
	}
}

func getCIIcon(ciStatus string) string {
	switch ciStatus {
	case "Passing":
		return "✓"
	case "Failing":
		return "✗"
	case "Running":
		return "⏳"
	default:
		return "○"
	}
}
