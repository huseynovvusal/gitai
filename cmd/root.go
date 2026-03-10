package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func Execute(version string) {
	rootCmd := &cobra.Command{
		Use:     "gitai",
		Version: version,
		Short:   "Gitai is a CLI tool to interact with Git repositories using AI",
		Long:    `Gitai allows you to perform various Git operations with the help of AI, making version control easier and more intuitive.`,
	}
	rootCmd.AddCommand(NewSuggestCmd())
	rootCmd.AddCommand(NewReviewCmd())
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
