package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "stack",
	Short: "A tool for managing stacked pull requests",
	Long: `stack is a CLI tool that enables stacked PR workflows.
It helps you create, sync, and manage dependent branches and their pull requests.`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Add subcommands here as they are implemented
}
