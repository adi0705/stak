package history

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Operation represents a stack operation that can be undone
type Operation struct {
	Timestamp   time.Time              `json:"timestamp"`
	Command     string                 `json:"command"`
	Branch      string                 `json:"branch"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// GetLogPath returns the path to the operation log file
func GetLogPath() (string, error) {
	gitDir, err := getGitDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(gitDir, "stak.log"), nil
}

// LogOperation logs an operation to the history file
func LogOperation(command, branch, description string, metadata map[string]interface{}) error {
	logPath, err := GetLogPath()
	if err != nil {
		return err
	}

	op := Operation{
		Timestamp:   time.Now(),
		Command:     command,
		Branch:      branch,
		Description: description,
		Metadata:    metadata,
	}

	// Read existing operations
	ops, err := ReadOperations()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing operations: %w", err)
	}

	// Append new operation
	ops = append(ops, op)

	// Keep only last 50 operations
	if len(ops) > 50 {
		ops = ops[len(ops)-50:]
	}

	// Write back
	data, err := json.MarshalIndent(ops, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal operations: %w", err)
	}

	if err := os.WriteFile(logPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write log file: %w", err)
	}

	return nil
}

// ReadOperations reads all logged operations
func ReadOperations() ([]Operation, error) {
	logPath, err := GetLogPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Operation{}, nil
		}
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	var ops []Operation
	if err := json.Unmarshal(data, &ops); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operations: %w", err)
	}

	return ops, nil
}

// GetLastOperation returns the most recent operation
func GetLastOperation() (*Operation, error) {
	ops, err := ReadOperations()
	if err != nil {
		return nil, err
	}

	if len(ops) == 0 {
		return nil, fmt.Errorf("no operations in history")
	}

	return &ops[len(ops)-1], nil
}

// RemoveLastOperation removes the most recent operation from the log
func RemoveLastOperation() error {
	logPath, err := GetLogPath()
	if err != nil {
		return err
	}

	ops, err := ReadOperations()
	if err != nil {
		return err
	}

	if len(ops) == 0 {
		return fmt.Errorf("no operations to remove")
	}

	// Remove last operation
	ops = ops[:len(ops)-1]

	// Write back
	data, err := json.MarshalIndent(ops, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal operations: %w", err)
	}

	if err := os.WriteFile(logPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write log file: %w", err)
	}

	return nil
}

func getGitDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository")
	}

	return strings.TrimSpace(string(output)), nil
}
