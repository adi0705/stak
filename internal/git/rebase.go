package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// RebaseOnto rebases the current branch onto another branch
func RebaseOnto(onto string) error {
	cmd := exec.Command("git", "rebase", onto)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's a rebase conflict
		if strings.Contains(string(output), "CONFLICT") || strings.Contains(string(output), "could not apply") {
			return &RebaseConflictError{
				Onto:   onto,
				Output: string(output),
			}
		}
		return fmt.Errorf("rebase failed: %s", string(output))
	}
	return nil
}

// RebaseConflictError represents a rebase conflict
type RebaseConflictError struct {
	Onto   string
	Output string
}

func (e *RebaseConflictError) Error() string {
	return fmt.Sprintf("rebase conflict while rebasing onto %s", e.Onto)
}

// IsRebaseInProgress checks if a rebase is currently in progress
func IsRebaseInProgress() (bool, error) {
	// Check if .git/rebase-merge or .git/rebase-apply exists
	cmd2 := exec.Command("git", "rev-parse", "--git-path", "rebase-merge")
	gitPath, err := cmd2.Output()
	if err == nil {
		// Check if directory exists
		checkCmd := exec.Command("test", "-d", strings.TrimSpace(string(gitPath)))
		if checkCmd.Run() == nil {
			return true, nil
		}
	}

	cmd3 := exec.Command("git", "rev-parse", "--git-path", "rebase-apply")
	gitPath, err = cmd3.Output()
	if err == nil {
		// Check if directory exists
		checkCmd := exec.Command("test", "-d", strings.TrimSpace(string(gitPath)))
		if checkCmd.Run() == nil {
			return true, nil
		}
	}

	return false, nil
}

// ContinueRebase continues a rebase after resolving conflicts
func ContinueRebase() error {
	cmd := exec.Command("git", "rebase", "--continue")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to continue rebase: %s", string(output))
	}
	return nil
}

// AbortRebase aborts an in-progress rebase
func AbortRebase() error {
	cmd := exec.Command("git", "rebase", "--abort")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to abort rebase: %s", string(output))
	}
	return nil
}

// GetConflictedFiles returns a list of files with conflicts
func GetConflictedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get conflicted files: %w", err)
	}

	files := []string{}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// HasMergeConflicts checks if there are merge conflicts
func HasMergeConflicts() (bool, error) {
	files, err := GetConflictedFiles()
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}
