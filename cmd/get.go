package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"stacking/internal/git"
	"stacking/internal/github"
	"stacking/internal/stack"
	"stacking/internal/ui"
)

var getUser string

var getCmd = &cobra.Command{
	Use:     "get <branch>",
	Aliases: []string{"gt"},
	Short:   "Download and track a colleague's stack",
	Long:    `Fetch a remote branch and automatically detect and track its entire stack structure from PR relationships.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := args[0]
		if err := runGet(branchName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	getCmd.Flags().StringVar(&getUser, "user", "", "Specify the GitHub user/org (default: auto-detect from remote)")
	rootCmd.AddCommand(getCmd)
}

func runGet(branchName string) error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	// Fetch from remote
	ui.Info("Fetching from remote")
	cmd := exec.Command("git", "fetch", "origin")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch from remote: %w", err)
	}

	// Check if remote branch exists
	remoteBranch := "origin/" + branchName
	cmd = exec.Command("git", "rev-parse", "--verify", remoteBranch)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remote branch %s does not exist", branchName)
	}

	// Check if local branch already exists
	localExists, err := git.BranchExists(branchName)
	if err != nil {
		return fmt.Errorf("failed to check if local branch exists: %w", err)
	}

	if localExists {
		ui.Warning(fmt.Sprintf("Local branch %s already exists", branchName))
		ui.Info("Checking out existing branch")
		if err := git.CheckoutBranch(branchName); err != nil {
			return fmt.Errorf("failed to checkout branch: %w", err)
		}
	} else {
		// Create local branch tracking remote
		ui.Info(fmt.Sprintf("Creating local branch %s from %s", branchName, remoteBranch))
		cmd = exec.Command("git", "checkout", "-b", branchName, "--track", remoteBranch)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create local branch: %w", err)
		}
	}

	// Try to detect PR for this branch
	ui.Info("Detecting PR and stack structure")
	prNumber, err := github.GetPRNumberForBranch(branchName)
	if err != nil {
		ui.Warning("Could not find PR for branch - will only track the single branch")
		ui.Info("Branch checked out successfully")
		return nil
	}

	ui.Info(fmt.Sprintf("Found PR #%d for %s", prNumber, branchName))

	// Get PR details to find base branch
	prDetails, err := github.GetPRDetails(prNumber)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not get PR details: %v", err))
		ui.Info("Branch checked out successfully")
		return nil
	}

	baseBranch := prDetails.BaseRefName
	ui.Info(fmt.Sprintf("PR base: %s", baseBranch))

	// Build stack structure by following PR bases
	stackBranches := []string{branchName}
	currentBase := baseBranch

	// Walk up the stack (find ancestors)
	for {
		// Check if base branch has a PR
		remoteBranchExists := false
		cmd = exec.Command("git", "rev-parse", "--verify", "origin/"+currentBase)
		if cmd.Run() == nil {
			remoteBranchExists = true
		}

		if !remoteBranchExists {
			// Base is probably main/master, stop here
			break
		}

		basePRNumber, err := github.GetPRNumberForBranch(currentBase)
		if err != nil {
			// Base doesn't have a PR, stop here
			break
		}

		basePRDetails, err := github.GetPRDetails(basePRNumber)
		if err != nil {
			break
		}

		// Fetch and track this base branch too
		localExists, _ := git.BranchExists(currentBase)
		if !localExists {
			ui.Info(fmt.Sprintf("Fetching ancestor branch %s (PR #%d)", currentBase, basePRNumber))
			cmd = exec.Command("git", "checkout", "-b", currentBase, "--track", "origin/"+currentBase)
			cmd.Run() // Ignore errors
		}

		stackBranches = append([]string{currentBase}, stackBranches...) // Prepend
		currentBase = basePRDetails.BaseRefName
	}

	// Also check for descendant branches (children)
	children, err := findChildBranches(branchName)
	if err == nil && len(children) > 0 {
		for _, child := range children {
			localExists, _ := git.BranchExists(child)
			if !localExists {
				ui.Info(fmt.Sprintf("Fetching descendant branch %s", child))
				cmd = exec.Command("git", "checkout", "-b", child, "--track", "origin/"+child)
				cmd.Run() // Ignore errors
			}
			stackBranches = append(stackBranches, child)
		}
	}

	// Track all branches in the stack
	ui.Info(fmt.Sprintf("\nTracking %d branch(es) in stack:", len(stackBranches)))
	for i, branch := range stackBranches {
		var parent string
		if i == 0 {
			// First branch - parent is the final base (main, etc.)
			parent = currentBase
		} else {
			// Subsequent branches - parent is previous in list
			parent = stackBranches[i-1]
		}

		// Check if already tracked
		hasMetadata, _ := stack.HasStackMetadata(branch)
		if hasMetadata {
			ui.Info(fmt.Sprintf("  %s → already tracked", branch))
			continue
		}

		// Track the branch
		branchPR, _ := github.GetPRNumberForBranch(branch)
		if err := stack.WriteBranchMetadata(branch, parent, branchPR); err != nil {
			ui.Warning(fmt.Sprintf("  %s → failed to track: %v", branch, err))
		} else {
			ui.Success(fmt.Sprintf("  %s → %s", branch, parent))
		}
	}

	// Checkout the requested branch
	if err := git.CheckoutBranch(branchName); err != nil {
		ui.Warning(fmt.Sprintf("Could not checkout %s", branchName))
	}

	ui.Success(fmt.Sprintf("\nStack downloaded and tracked successfully"))
	ui.Info("Use 'stak list' to view the stack structure")

	return nil
}

// findChildBranches finds branches whose PRs target the given branch
func findChildBranches(parentBranch string) ([]string, error) {
	// List all remote branches
	cmd := exec.Command("git", "branch", "-r")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var children []string
	branches := strings.Split(string(output), "\n")

	for _, line := range branches {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "->") {
			continue
		}

		// Extract branch name (remove "origin/" prefix)
		remoteBranch := strings.TrimPrefix(line, "origin/")
		if remoteBranch == parentBranch {
			continue
		}

		// Check if this branch has a PR targeting parent
		prNumber, err := github.GetPRNumberForBranch(remoteBranch)
		if err != nil {
			continue
		}

		prDetails, err := github.GetPRDetails(prNumber)
		if err != nil {
			continue
		}

		if prDetails.BaseRefName == parentBranch {
			children = append(children, remoteBranch)
		}
	}

	return children, nil
}
