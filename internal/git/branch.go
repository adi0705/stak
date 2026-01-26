package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetCurrentBranch returns the name of the current branch
func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// BranchExists checks if a branch exists locally
func BranchExists(branch string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if branch exists: %w", err)
	}
	return true, nil
}

// CreateBranch creates a new branch from the current HEAD
func CreateBranch(name string) error {
	cmd := exec.Command("git", "checkout", "-b", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create branch %s: %w", name, err)
	}
	return nil
}

// CheckoutBranch checks out an existing branch
func CheckoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to checkout branch %s: %s", name, string(output))
	}
	return nil
}

// DeleteBranch deletes a local branch
func DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	cmd := exec.Command("git", "branch", flag, name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %s", name, string(output))
	}
	return nil
}

// Push pushes the current branch to remote
func Push(branch string, setUpstream bool, force bool) error {
	args := []string{"push"}
	if force {
		args = append(args, "--force-with-lease")
	}
	if setUpstream {
		args = append(args, "-u", "origin", branch)
	} else {
		args = append(args, "origin", branch)
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push branch %s: %s", branch, string(output))
	}
	return nil
}

// Fetch fetches from remote
func Fetch() error {
	cmd := exec.Command("git", "fetch", "origin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to fetch: %s", string(output))
	}
	return nil
}

// HasUncommittedChanges checks if there are uncommitted changes
func HasUncommittedChanges() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// HasCommits checks if the current branch has any commits
func HasCommits() (bool, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return false, nil
		}
		return false, fmt.Errorf("failed to check for commits: %w", err)
	}
	return true, nil
}

// IsGitRepository checks if the current directory is a git repository
func IsGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

// GetRemoteURL gets the remote URL for origin
func GetRemoteURL() (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// RemoteBranchExists checks if a branch exists on remote
func RemoteBranchExists(branch string) (bool, error) {
	cmd := exec.Command("git", "ls-remote", "--heads", "origin", branch)
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check remote branch: %w", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// ResetToRemote resets the current branch to match its remote counterpart
func ResetToRemote(branch string) error {
	remoteBranch := fmt.Sprintf("origin/%s", branch)
	cmd := exec.Command("git", "reset", "--hard", remoteBranch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to reset to %s: %s", remoteBranch, string(output))
	}
	return nil
}

// GetAllLocalBranches returns a list of all local branch names
func GetAllLocalBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	branchesStr := strings.TrimSpace(string(output))
	if branchesStr == "" {
		return []string{}, nil
	}

	branches := strings.Split(branchesStr, "\n")
	return branches, nil
}

// GetCommitAncestors returns a list of commit hashes in ancestry order
func GetCommitAncestors(branch string) ([]string, error) {
	cmd := exec.Command("git", "rev-list", "--first-parent", branch)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit ancestors: %w", err)
	}

	commitsStr := strings.TrimSpace(string(output))
	if commitsStr == "" {
		return []string{}, nil
	}

	commits := strings.Split(commitsStr, "\n")
	return commits, nil
}

// BranchContainsCommit checks if a branch contains a specific commit
func BranchContainsCommit(branch, commit string) bool {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", commit, branch)
	return cmd.Run() == nil
}

// HasUnstagedChanges checks if there are unstaged changes in the working directory
func HasUnstagedChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--quiet")
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return true, nil // Exit code 1 means there are changes
		}
		return false, fmt.Errorf("failed to check for unstaged changes: %w", err)
	}
	return false, nil
}

// HasStagedChanges checks if there are staged changes in the index
func HasStagedChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return true, nil // Exit code 1 means there are staged changes
		}
		return false, fmt.Errorf("failed to check for staged changes: %w", err)
	}
	return false, nil
}

// StageAll stages all changes (tracked and untracked files)
func StageAll() error {
	cmd := exec.Command("git", "add", "-A")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stage all changes: %s", string(output))
	}
	return nil
}

// Commit creates a new commit with the given message
func Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to commit: %s", string(output))
	}
	return nil
}
