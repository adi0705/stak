package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PRStatus represents the status of a pull request
type PRStatus struct {
	State          string `json:"state"`
	ReviewDecision string `json:"reviewDecision"`
	StatusCheckRollup []struct {
		State string `json:"state"`
	} `json:"statusCheckRollup"`
}

// CreatePR creates a pull request and returns the PR number
func CreatePR(base, head, title, body string, draft bool) (int, error) {
	args := []string{"pr", "create", "--base", base, "--head", head}

	// Handle title and body:
	// - If both empty: use --fill-first to auto-generate both from first commit
	// - If title provided: use it with --title
	// - If body provided: use it with --body
	if title == "" && body == "" {
		// Auto-generate both title and body from first commit
		args = append(args, "--fill-first")
	} else {
		if title != "" {
			args = append(args, "--title", title)
		}
		if body != "" {
			args = append(args, "--body", body)
		}
		// If only title provided, still need body for non-interactive mode
		if title != "" && body == "" {
			args = append(args, "--body", "")
		}
	}

	if draft {
		args = append(args, "--draft")
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to create PR: %s", string(output))
	}

	// Parse PR URL from output to extract PR number
	outputStr := string(output)
	prNumber, err := extractPRNumber(outputStr)
	if err != nil {
		return 0, fmt.Errorf("failed to extract PR number from output: %w", err)
	}

	return prNumber, nil
}

// GetPRStatus retrieves the status of a pull request
func GetPRStatus(prNumber int) (*PRStatus, error) {
	cmd := exec.Command("gh", "pr", "view", strconv.Itoa(prNumber), "--json", "state,reviewDecision,statusCheckRollup")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get PR status for #%d: %w", prNumber, err)
	}

	var status PRStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return nil, fmt.Errorf("failed to parse PR status: %w", err)
	}

	return &status, nil
}

// MergePR merges a pull request
func MergePR(prNumber int, method string) error {
	args := []string{"pr", "merge", strconv.Itoa(prNumber)}

	switch method {
	case "squash":
		args = append(args, "--squash")
	case "merge":
		args = append(args, "--merge")
	case "rebase":
		args = append(args, "--rebase")
	default:
		args = append(args, "--squash") // default to squash
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to merge PR #%d: %s", prNumber, string(output))
	}

	return nil
}

// UpdatePRBase changes the base branch of a pull request
func UpdatePRBase(prNumber int, newBase string) error {
	cmd := exec.Command("gh", "pr", "edit", strconv.Itoa(prNumber), "--base", newBase)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update PR #%d base to %s: %s", prNumber, newBase, string(output))
	}

	return nil
}

// EditPR updates the title and/or body of a pull request
func EditPR(prNumber int, title, body string) error {
	args := []string{"pr", "edit", strconv.Itoa(prNumber)}

	if title != "" {
		args = append(args, "--title", title)
	}

	if body != "" {
		args = append(args, "--body", body)
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to edit PR #%d: %s", prNumber, string(output))
	}

	return nil
}

// IsGHAuthenticated checks if the gh CLI is authenticated
func IsGHAuthenticated() bool {
	cmd := exec.Command("gh", "auth", "status")
	err := cmd.Run()
	return err == nil
}

// GetPRURL gets the URL for a pull request
func GetPRURL(prNumber int) (string, error) {
	cmd := exec.Command("gh", "pr", "view", strconv.Itoa(prNumber), "--json", "url", "-q", ".url")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get PR URL for #%d: %w", prNumber, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// extractPRNumber extracts the PR number from gh pr create output
// Example output: "https://github.com/owner/repo/pull/123"
func extractPRNumber(output string) (int, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "/pull/") {
			parts := strings.Split(line, "/pull/")
			if len(parts) >= 2 {
				numStr := strings.TrimSpace(parts[1])
				num, err := strconv.Atoi(numStr)
				if err == nil {
					return num, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("could not find PR number in output: %s", output)
}

// IsApproved checks if a PR is approved
func (s *PRStatus) IsApproved() bool {
	return s.ReviewDecision == "APPROVED"
}

// IsCIPassing checks if CI checks are passing
func (s *PRStatus) IsCIPassing() bool {
	if len(s.StatusCheckRollup) == 0 {
		// No checks, consider as passing
		return true
	}

	for _, check := range s.StatusCheckRollup {
		if check.State != "SUCCESS" {
			return false
		}
	}
	return true
}

// IsOpen checks if a PR is open
func (s *PRStatus) IsOpen() bool {
	return s.State == "OPEN"
}

// IsMerged checks if a PR is merged
func (s *PRStatus) IsMerged() bool {
	return s.State == "MERGED"
}

// CommentOnPR adds or updates a comment on a pull request
// If commentID is provided, it updates that comment; otherwise creates a new one
func CommentOnPR(prNumber int, body string) error {
	args := []string{"pr", "comment", strconv.Itoa(prNumber), "--body", body}

	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to comment on PR #%d: %s", prNumber, string(output))
	}

	return nil
}

// GetPRComments retrieves all comments from a pull request
func GetPRComments(prNumber int) ([]string, error) {
	cmd := exec.Command("gh", "pr", "view", strconv.Itoa(prNumber), "--json", "comments", "-q", ".comments[].body")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for PR #%d: %w", prNumber, err)
	}

	comments := strings.Split(strings.TrimSpace(string(output)), "\n")
	return comments, nil
}
