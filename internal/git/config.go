package git

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GetConfig retrieves a git config value
func GetConfig(key string) (string, error) {
	cmd := exec.Command("git", "config", "--get", key)
	output, err := cmd.Output()
	if err != nil {
		// Exit code 1 means key doesn't exist
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("failed to get git config %s: %w", key, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// SetConfig sets a git config value
func SetConfig(key, value string) error {
	cmd := exec.Command("git", "config", key, value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git config %s=%s: %w", key, value, err)
	}
	return nil
}

// UnsetConfig removes a git config value
func UnsetConfig(key string) error {
	cmd := exec.Command("git", "config", "--unset", key)
	if err := cmd.Run(); err != nil {
		// Ignore error if key doesn't exist
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 5 {
			return nil
		}
		return fmt.Errorf("failed to unset git config %s: %w", key, err)
	}
	return nil
}

// GetConfigRegexp retrieves all git config entries matching a regexp
func GetConfigRegexp(pattern string) (map[string]string, error) {
	cmd := exec.Command("git", "config", "--get-regexp", pattern)
	output, err := cmd.Output()
	if err != nil {
		// Exit code 1 means no matches
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("failed to get git config regexp %s: %w", pattern, err)
	}

	result := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result, nil
}

// GetBranchParent retrieves the parent branch for a given branch
func GetBranchParent(branch string) (string, error) {
	key := fmt.Sprintf("stack.branch.%s.parent", branch)
	return GetConfig(key)
}

// SetBranchParent sets the parent branch for a given branch
func SetBranchParent(branch, parent string) error {
	key := fmt.Sprintf("stack.branch.%s.parent", branch)
	return SetConfig(key, parent)
}

// GetBranchPRNumber retrieves the PR number for a given branch
func GetBranchPRNumber(branch string) (int, error) {
	key := fmt.Sprintf("stack.branch.%s.pr-number", branch)
	value, err := GetConfig(key)
	if err != nil {
		return 0, err
	}
	if value == "" {
		return 0, nil
	}
	prNum, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid PR number for branch %s: %s", branch, value)
	}
	return prNum, nil
}

// SetBranchPRNumber sets the PR number for a given branch
func SetBranchPRNumber(branch string, prNumber int) error {
	key := fmt.Sprintf("stack.branch.%s.pr-number", branch)
	return SetConfig(key, strconv.Itoa(prNumber))
}

// GetAllStackBranches retrieves all branches that have stack metadata
func GetAllStackBranches() ([]string, error) {
	configs, err := GetConfigRegexp("^stack\\.branch\\.")
	if err != nil {
		return nil, err
	}

	branchSet := make(map[string]bool)
	for key := range configs {
		// Extract branch name from key like "stack.branch.feature-a.parent"
		parts := strings.Split(key, ".")
		if len(parts) >= 3 {
			branchName := parts[2]
			branchSet[branchName] = true
		}
	}

	branches := make([]string, 0, len(branchSet))
	for branch := range branchSet {
		branches = append(branches, branch)
	}
	return branches, nil
}

// UnsetBranchMetadata removes all stack metadata for a given branch
func UnsetBranchMetadata(branch string) error {
	parentKey := fmt.Sprintf("stack.branch.%s.parent", branch)
	prKey := fmt.Sprintf("stack.branch.%s.pr-number", branch)
	frozenKey := fmt.Sprintf("stack.branch.%s.frozen", branch)

	if err := UnsetConfig(parentKey); err != nil {
		return err
	}
	if err := UnsetConfig(prKey); err != nil {
		return err
	}
	if err := UnsetConfig(frozenKey); err != nil {
		return err
	}
	return nil
}

// GetBranchFrozen retrieves the frozen status for a given branch
func GetBranchFrozen(branch string) (string, error) {
	key := fmt.Sprintf("stack.branch.%s.frozen", branch)
	return GetConfig(key)
}

// SetBranchFrozen sets the frozen status for a given branch
func SetBranchFrozen(branch, frozen string) error {
	key := fmt.Sprintf("stack.branch.%s.frozen", branch)
	if frozen == "false" || frozen == "" {
		// Unset the key if unfreezing
		return UnsetConfig(key)
	}
	return SetConfig(key, frozen)
}
