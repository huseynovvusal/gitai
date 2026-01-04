package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is injected at build time
	Version = "0.2.0"
)

var rootCmd = &cobra.Command{
	Use:     "gitai",
	Version: Version,
	Short:   "Gitai is a CLI tool to interact with Git repositories using AI",
	Long:    `Gitai allows you to perform various Git operations with the help of AI, making version control easier and more intuitive.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}