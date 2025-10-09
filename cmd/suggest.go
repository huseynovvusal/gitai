package cmd

import (
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/config"
	"huseynovvusal/gitai/internal/tui/suggest"

	"github.com/spf13/cobra"
)

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Suggest commit messages for changed files using AI",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := config.LoadConfig("gitai.yaml")
		if err != nil {
			cmd.PrintErrln("Error loading config:", err)
			return
		}

		suggest.RunSuggestFlow(ai.Provider(config.AI.Provider))
	},
}

func init() {
	suggestCmd.Flags().StringP("provider", "p", "", "AI provider to use (gpt|gemini|ollama). If empty, uses env or default")
	rootCmd.AddCommand(suggestCmd)
}
