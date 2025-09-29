package cmd

import (
	"huseynovvusal/gitai/internal/tui/suggest"

	"github.com/spf13/cobra"
)

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Suggest commit messages for changed files using AI",
	Run: func(cmd *cobra.Command, args []string) {
		suggest.RunSuggestFlow()
	},
}

func init() {
	rootCmd.AddCommand(suggestCmd)
}
