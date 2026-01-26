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
	// Note: We don't use --head flag because gh CLI automatically uses the current branch
	// The head parameter is kept for potential future use (e.g., cross-repo PRs)
	args := []string{"pr", "create", "--base", base}

	// Handle title and body:
	// - If both empty: use --fill-first to auto-generate both from first commit
	// - If title provided: use it with --title and --fill to auto-generate body
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
		// If only title provided, use --fill to auto-generate body from commits
		if title != "" && body == "" {
			args = append(args, "--fill")
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
// Looks for existing comment with stack marker and updates it, or creates new one
func CommentOnPR(prNumber int, body string) error {
	// First, try to find existing stack comment
	existingCommentID, err := findStackComment(prNumber)
	if err != nil {
		// If error finding comments, just create a new one
		return createComment(prNumber, body)
	}

	if existingCommentID != "" {
		// Update existing comment
		return updateComment(existingCommentID, body)
	}

	// No existing comment, create new one
	return createComment(prNumber, body)
}

// findStackComment finds the comment ID of an existing stack visualization comment
func findStackComment(prNumber int) (string, error) {
	cmd := exec.Command("gh", "api", fmt.Sprintf("/repos/{owner}/{repo}/issues/%d/comments", prNumber))
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}

	if err := json.Unmarshal(output, &comments); err != nil {
		return "", err
	}

	// Look for comment containing stack marker
	stackMarker := "_This stack is managed by [stak]"
	for _, comment := range comments {
		if strings.Contains(comment.Body, stackMarker) {
			return strconv.FormatInt(comment.ID, 10), nil
		}
	}

	return "", nil
}

// createComment creates a new comment on a PR
func createComment(prNumber int, body string) error {
	args := []string{"pr", "comment", strconv.Itoa(prNumber), "--body", body}

	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to comment on PR #%d: %s", prNumber, string(output))
	}

	return nil
}

// updateComment updates an existing comment
func updateComment(commentID string, body string) error {
	cmd := exec.Command("gh", "api", "-X", "PATCH",
		fmt.Sprintf("/repos/{owner}/{repo}/issues/comments/%s", commentID),
		"-f", fmt.Sprintf("body=%s", body))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update comment %s: %s", commentID, string(output))
	}

	return nil
}

// GetPRForBranch finds the PR associated with a branch
// Returns PR number, base branch name, and error
func GetPRForBranch(branch string) (int, string, error) {
	cmd := exec.Command("gh", "pr", "list",
		"--json", "number,headRefName,baseRefName",
		"--head", branch)
	output, err := cmd.Output()
	if err != nil {
		return 0, "", fmt.Errorf("failed to list PRs: %w", err)
	}

	var prs []struct {
		Number      int    `json:"number"`
		HeadRefName string `json:"headRefName"`
		BaseRefName string `json:"baseRefName"`
	}

	if err := json.Unmarshal(output, &prs); err != nil {
		return 0, "", fmt.Errorf("failed to parse PR list: %w", err)
	}

	if len(prs) == 0 {
		return 0, "", fmt.Errorf("no PR found for branch %s", branch)
	}

	// Return first PR's number and base
	return prs[0].Number, prs[0].BaseRefName, nil
}

// GetPRNumberForBranch finds the PR number for a branch
// Returns PR number and error
func GetPRNumberForBranch(branch string) (int, error) {
	prNumber, _, err := GetPRForBranch(branch)
	return prNumber, err
}

// PRDetails contains detailed information about a PR
type PRDetails struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	State          string `json:"state"`
	ReviewDecision string `json:"reviewDecision"`
	IsDraft        bool   `json:"isDraft"`
	BaseRefName    string `json:"baseRefName"`
	HeadRefName    string `json:"headRefName"`
	Commits        struct {
		TotalCount int `json:"totalCount"`
	} `json:"commits"`
	StatusCheckRollup []struct {
		State      string `json:"state"`
		Conclusion string `json:"conclusion"`
	} `json:"statusCheckRollup"`
}

// GetPRDetails retrieves detailed information about a PR
func GetPRDetails(prNumber int) (*PRDetails, error) {
	cmd := exec.Command("gh", "pr", "view", strconv.Itoa(prNumber), "--json",
		"number,title,state,reviewDecision,isDraft,baseRefName,headRefName,commits,statusCheckRollup")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get PR details for #%d: %w", prNumber, err)
	}

	var details PRDetails
	if err := json.Unmarshal(output, &details); err != nil {
		return nil, fmt.Errorf("failed to parse PR details: %w", err)
	}

	return &details, nil
}

// GetCIStatus returns a human-readable CI status
func (d *PRDetails) GetCIStatus() string {
	if len(d.StatusCheckRollup) == 0 {
		return "No checks"
	}

	allPassed := true
	anyFailed := false
	anyPending := false

	for _, check := range d.StatusCheckRollup {
		switch check.State {
		case "SUCCESS":
			continue
		case "FAILURE", "ERROR":
			anyFailed = true
			allPassed = false
		case "PENDING", "IN_PROGRESS":
			anyPending = true
			allPassed = false
		default:
			allPassed = false
		}
	}

	if anyFailed {
		return "Failing"
	}
	if anyPending {
		return "Running"
	}
	if allPassed {
		return "Passing"
	}
	return "Unknown"
}

// GetReviewStatus returns a human-readable review status
func (d *PRDetails) GetReviewStatus() string {
	if d.IsDraft {
		return "Draft"
	}
	switch d.ReviewDecision {
	case "APPROVED":
		return "Approved"
	case "CHANGES_REQUESTED":
		return "Changes requested"
	case "REVIEW_REQUIRED":
		return "Review required"
	case "":
		return "Pending review"
	default:
		return d.ReviewDecision
	}
}

// GetStateDisplay returns a human-readable state
func (d *PRDetails) GetStateDisplay() string {
	if d.State == "MERGED" {
		return "Merged"
	}
	if d.State == "CLOSED" {
		return "Closed"
	}
	if d.IsDraft {
		return "Draft"
	}
	return "Open"
}

// ClosePR closes a pull request
func ClosePR(prNumber int) error {
	cmd := exec.Command("gh", "pr", "close", strconv.Itoa(prNumber))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to close PR #%d: %s", prNumber, string(output))
	}
	return nil
}
