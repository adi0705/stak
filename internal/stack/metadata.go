package stack

import (
	"fmt"
	"strconv"
	"strings"

	"stacking/internal/git"
	"stacking/pkg/models"
)

// ReadBranchMetadata reads metadata for a single branch
func ReadBranchMetadata(branch string) (*models.Branch, error) {
	parent, err := git.GetBranchParent(branch)
	if err != nil {
		return nil, fmt.Errorf("failed to read parent for branch %s: %w", branch, err)
	}

	prNumber, err := git.GetBranchPRNumber(branch)
	if err != nil {
		return nil, fmt.Errorf("failed to read PR number for branch %s: %w", branch, err)
	}

	return models.NewBranch(branch, parent, prNumber), nil
}

// WriteBranchMetadata writes metadata for a single branch
func WriteBranchMetadata(branch, parent string, prNumber int) error {
	if parent != "" {
		if err := git.SetBranchParent(branch, parent); err != nil {
			return fmt.Errorf("failed to set parent for branch %s: %w", branch, err)
		}
	}

	if prNumber > 0 {
		if err := git.SetBranchPRNumber(branch, prNumber); err != nil {
			return fmt.Errorf("failed to set PR number for branch %s: %w", branch, err)
		}
	}

	return nil
}

// DeleteBranchMetadata removes all metadata for a branch
func DeleteBranchMetadata(branch string) error {
	if err := git.UnsetBranchMetadata(branch); err != nil {
		return fmt.Errorf("failed to delete metadata for branch %s: %w", branch, err)
	}
	return nil
}

// BuildStack builds the entire stack tree from git config
func BuildStack() (*models.Stack, error) {
	stack := models.NewStack()

	// Get all branches with stack metadata
	branches, err := git.GetAllStackBranches()
	if err != nil {
		return nil, fmt.Errorf("failed to get stack branches: %w", err)
	}

	// Read metadata for each branch
	for _, branchName := range branches {
		branch, err := ReadBranchMetadata(branchName)
		if err != nil {
			return nil, err
		}
		stack.AddBranch(branch)
	}

	// Build parent-child relationships
	stack.BuildRelationships()

	return stack, nil
}

// GetParent returns the parent branch name
func GetParent(branch string) (string, error) {
	return git.GetBranchParent(branch)
}

// GetChildren returns all direct children of a branch
func GetChildren(branch string) ([]string, error) {
	stack, err := BuildStack()
	if err != nil {
		return nil, err
	}

	b := stack.GetBranch(branch)
	if b == nil {
		return []string{}, nil
	}

	children := make([]string, 0, len(b.Children))
	for _, child := range b.Children {
		children = append(children, child.Name)
	}
	return children, nil
}

// GetAncestors returns all ancestor branches from the given branch to the base
func GetAncestors(branch string) ([]string, error) {
	ancestors := []string{}
	current := branch

	// Walk up the parent chain
	for {
		parent, err := GetParent(current)
		if err != nil {
			return nil, err
		}
		if parent == "" {
			break
		}
		ancestors = append([]string{parent}, ancestors...) // Prepend to maintain order
		current = parent
	}

	return ancestors, nil
}

// GetDescendants returns all descendant branches using BFS
func GetDescendants(branch string) ([]string, error) {
	descendants := []string{}
	queue := []string{branch}
	visited := make(map[string]bool)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		children, err := GetChildren(current)
		if err != nil {
			return nil, err
		}

		for _, child := range children {
			descendants = append(descendants, child)
			queue = append(queue, child)
		}
	}

	return descendants, nil
}

// GetAllStackBranches returns all branches that have stack metadata
func GetAllStackBranches() ([]string, error) {
	return git.GetAllStackBranches()
}

// HasStackMetadata checks if a branch has stack metadata
func HasStackMetadata(branch string) (bool, error) {
	parent, err := git.GetBranchParent(branch)
	if err != nil {
		return false, err
	}
	return parent != "", nil
}

// GenerateStackVisualization creates a markdown visualization of the stack
func GenerateStackVisualization(currentBranch string) (string, error) {
	// Get all ancestors
	ancestors, err := GetAncestors(currentBranch)
	if err != nil {
		return "", fmt.Errorf("failed to get ancestors: %w", err)
	}

	// Get all descendants
	descendants, err := GetDescendants(currentBranch)
	if err != nil {
		return "", fmt.Errorf("failed to get descendants: %w", err)
	}

	// Build the full stack: ancestors + current + descendants
	fullStack := append(ancestors, currentBranch)
	fullStack = append(fullStack, descendants...)

	// Generate markdown with machine-readable metadata
	var result string
	result += "## 📚 Stack\n\n"

	// Store metadata for hidden section
	var metadataLines []string

	for _, branch := range fullStack {
		metadata, err := ReadBranchMetadata(branch)
		if err != nil {
			continue
		}

		prefix := "- "
		if branch == currentBranch {
			prefix = "- **"
		}

		prInfo := ""
		if metadata.PRNumber > 0 {
			prInfo = fmt.Sprintf(" → PR #%d", metadata.PRNumber)
		}

		if branch == currentBranch {
			result += fmt.Sprintf("%s%s%s** ← 👈 _current_\n", prefix, branch, prInfo)
		} else {
			result += fmt.Sprintf("%s%s%s\n", prefix, branch, prInfo)
		}

		// Add to machine-readable metadata
		metadataLines = append(metadataLines, fmt.Sprintf("%s:%s:%d", branch, metadata.Parent, metadata.PRNumber))
	}

	result += "\n---\n_This stack is managed by [stak](https://github.com/yourusername/stacking)_\n\n"

	// Add hidden machine-readable metadata
	result += "<!-- stak-metadata\n"
	for _, line := range metadataLines {
		result += line + "\n"
	}
	result += "-->"

	return result, nil
}

// ParseStackMetadata parses machine-readable metadata from a PR comment
func ParseStackMetadata(comment string) (map[string]*models.Branch, error) {
	// Find the metadata section
	startMarker := "<!-- stak-metadata"
	endMarker := "-->"

	startIdx := strings.Index(comment, startMarker)
	if startIdx == -1 {
		return nil, fmt.Errorf("no stack metadata found in comment")
	}

	startIdx += len(startMarker)
	endIdx := strings.Index(comment[startIdx:], endMarker)
	if endIdx == -1 {
		return nil, fmt.Errorf("malformed stack metadata in comment")
	}

	metadataSection := comment[startIdx : startIdx+endIdx]
	lines := strings.Split(strings.TrimSpace(metadataSection), "\n")

	branches := make(map[string]*models.Branch)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse format: branch:parent:prNumber
		parts := strings.Split(line, ":")
		if len(parts) != 3 {
			continue
		}

		branchName := parts[0]
		parent := parts[1]
		prNumber, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}

		branches[branchName] = models.NewBranch(branchName, parent, prNumber)
	}

	return branches, nil
}
