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

// CountCommits counts the number of commits on current branch compared to base
func CountCommits(baseBranch string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", "HEAD", fmt.Sprintf("^%s", baseBranch))
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to count commits: %w", err)
	}

	var count int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count); err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}
	return count, nil
}

// SquashCommits squashes all commits on current branch into a single commit
func SquashCommits(baseBranch string) error {
	// Get the current commit message (we'll keep the first commit's message)
	cmd := exec.Command("git", "log", "--format=%B", "-n", "1", "HEAD")
	messageOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get commit message: %w", err)
	}
	message := strings.TrimSpace(string(messageOutput))

	// Soft reset to the base branch (keeps all changes staged)
	resetCmd := exec.Command("git", "reset", "--soft", baseBranch)
	if output, err := resetCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reset to %s: %s", baseBranch, string(output))
	}

	// Commit with the original message
	commitCmd := exec.Command("git", "commit", "-m", message)
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create squashed commit: %s", string(output))
	}

	return nil
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
