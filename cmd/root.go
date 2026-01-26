package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	versionFlag bool
	appVersion  = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "stak",
	Short: "A tool for managing stacked pull requests",
	Long: `stak is a CLI tool that enables stacked PR workflows.
It helps you create, sync, and manage dependent branches and their pull requests.`,
	Run: func(cmd *cobra.Command, args []string) {
		if versionFlag {
			fmt.Printf("stak version %s\n", appVersion)
			os.Exit(0)
		}
		cmd.Help()
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// SetVersion sets the application version
func SetVersion(version string) {
	appVersion = version
}

func init() {
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print version information")
}
