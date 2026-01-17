package stack

import (
	"stacking/pkg/models"
)

// TraversePreOrder performs a pre-order traversal of the branch tree
// Calls the visitor function for each branch, passing the branch and its depth
func TraversePreOrder(branch *models.Branch, depth int, visitor func(*models.Branch, int)) {
	visitor(branch, depth)
	for _, child := range branch.Children {
		TraversePreOrder(child, depth+1, visitor)
	}
}

// TraversePostOrder performs a post-order traversal of the branch tree
// Calls the visitor function for each branch after visiting its children
func TraversePostOrder(branch *models.Branch, depth int, visitor func(*models.Branch, int)) {
	for _, child := range branch.Children {
		TraversePostOrder(child, depth+1, visitor)
	}
	visitor(branch, depth)
}

// GetBranchPath returns the full path from a root to the given branch
func GetBranchPath(stack *models.Stack, branchName string) []*models.Branch {
	branch := stack.GetBranch(branchName)
	if branch == nil {
		return nil
	}

	path := []*models.Branch{branch}
	current := branch

	// Walk up the parent chain
	for current.Parent != "" {
		parent := stack.GetBranch(current.Parent)
		if parent == nil {
			break
		}
		path = append([]*models.Branch{parent}, path...)
		current = parent
	}

	return path
}

// FindRoot finds the root branch for a given branch
func FindRoot(stack *models.Stack, branchName string) *models.Branch {
	path := GetBranchPath(stack, branchName)
	if len(path) == 0 {
		return nil
	}
	return path[0]
}

// GetAllBranchesInOrder returns all branches in the stack in pre-order traversal
func GetAllBranchesInOrder(stack *models.Stack) []*models.Branch {
	branches := []*models.Branch{}

	for _, root := range stack.Roots {
		TraversePreOrder(root, 0, func(b *models.Branch, depth int) {
			branches = append(branches, b)
		})
	}

	return branches
}

// GetDepth returns the depth of a branch in the tree (distance from root)
func GetDepth(stack *models.Stack, branchName string) int {
	path := GetBranchPath(stack, branchName)
	return len(path) - 1
}
