package stack

import (
	"fmt"
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

	// Generate markdown
	var result string
	result += "## 📚 Stack\n\n"

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

		parent := metadata.Parent
		if parent == "" {
			parent = "base"
		}

		if branch == currentBranch {
			result += fmt.Sprintf("%s%s%s** ← 👈 _current_\n", prefix, branch, prInfo)
		} else {
			result += fmt.Sprintf("%s%s%s\n", prefix, branch, prInfo)
		}
	}

	result += "\n---\n_This stack is managed by [stak](https://github.com/yourusername/stacking)_"

	return result, nil
}

// WouldCreateCycle checks if setting branch -> parent would create a cycle
func WouldCreateCycle(branch, proposedParent string) (bool, error) {
	if proposedParent == "" {
		return false, nil // Root branches can't create cycles
	}

	if branch == proposedParent {
		return true, nil // Direct self-reference
	}

	current := proposedParent
	visited := make(map[string]bool)

	for current != "" {
		if current == branch {
			return true, nil // Cycle detected
		}

		if visited[current] {
			return false, fmt.Errorf("existing cycle detected in stack")
		}
		visited[current] = true

		parent, err := GetParent(current)
		if err != nil {
			return false, err
		}
		current = parent
	}

	return false, nil
}

// IsBaseBranch checks if a branch is a common base branch
func IsBaseBranch(branch string) bool {
	baseBranches := []string{"main", "master", "develop", "development"}
	for _, base := range baseBranches {
		if branch == base {
			return true
		}
	}
	return false
}

// IsBranchFrozen checks if a branch is marked as frozen
func IsBranchFrozen(branch string) (bool, error) {
	frozen, err := git.GetBranchFrozen(branch)
	if err != nil {
		return false, err
	}
	return frozen == "true", nil
}

// FreezeBranch marks a branch as frozen to protect it from modifications
func FreezeBranch(branch string) error {
	if err := git.SetBranchFrozen(branch, "true"); err != nil {
		return fmt.Errorf("failed to freeze branch %s: %w", branch, err)
	}
	return nil
}

// UnfreezeBranch removes the frozen marker from a branch
func UnfreezeBranch(branch string) error {
	if err := git.SetBranchFrozen(branch, "false"); err != nil {
		return fmt.Errorf("failed to unfreeze branch %s: %w", branch, err)
	}
	return nil
}
