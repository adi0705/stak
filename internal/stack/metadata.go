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

// HasStackMetadata checks if a branch has stack metadata
func HasStackMetadata(branch string) (bool, error) {
	parent, err := git.GetBranchParent(branch)
	if err != nil {
		return false, err
	}
	return parent != "", nil
}
