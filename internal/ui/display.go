package ui

import (
	"fmt"
	"strings"

	"stacking/pkg/models"
	"stacking/internal/stack"
)

// DisplayStack displays the entire stack in a tree format
func DisplayStack(s *models.Stack, currentBranch string) {
	if len(s.Roots) == 0 {
		fmt.Println("No stacked branches found.")
		return
	}

	for _, root := range s.Roots {
		displayBranch(root, "", true, currentBranch)
	}
}

// displayBranch recursively displays a branch and its children
func displayBranch(branch *models.Branch, prefix string, isLast bool, currentBranch string) {
	// Determine the tree characters
	var connector string
	if prefix == "" {
		connector = ""
	} else if isLast {
		connector = "└─ "
	} else {
		connector = "├─ "
	}

	// Build the branch display string
	branchDisplay := branch.Name
	if branch.PRNumber > 0 {
		branchDisplay += fmt.Sprintf(" (#%d)", branch.PRNumber)
	}
	if branch.Name == currentBranch {
		branchDisplay += " *"
	}

	fmt.Println(prefix + connector + branchDisplay)

	// Prepare prefix for children
	var childPrefix string
	if prefix == "" {
		childPrefix = ""
	} else if isLast {
		childPrefix = prefix + "   "
	} else {
		childPrefix = prefix + "│  "
	}

	// Display children
	for i, child := range branch.Children {
		isLastChild := i == len(branch.Children)-1
		displayBranch(child, childPrefix, isLastChild, currentBranch)
	}
}

// DisplayBranchPath displays the path from root to a branch
func DisplayBranchPath(s *models.Stack, branchName string) {
	path := stack.GetBranchPath(s, branchName)
	if len(path) == 0 {
		fmt.Printf("Branch %s not found in stack\n", branchName)
		return
	}

	parts := []string{}
	for _, b := range path {
		part := b.Name
		if b.PRNumber > 0 {
			part += fmt.Sprintf(" (#%d)", b.PRNumber)
		}
		parts = append(parts, part)
	}

	fmt.Println(strings.Join(parts, " → "))
}

// Success prints a success message
func Success(message string) {
	fmt.Printf("✓ %s\n", message)
}

// Error prints an error message
func Error(message string) {
	fmt.Printf("✗ %s\n", message)
}

// Info prints an info message
func Info(message string) {
	fmt.Printf("ℹ %s\n", message)
}

// Warning prints a warning message
func Warning(message string) {
	fmt.Printf("⚠ %s\n", message)
}
