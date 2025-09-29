package cmd

import (
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/tui/suggest"

	"github.com/spf13/cobra"
)

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Suggest commit messages for changed files using AI",
	Run: func(cmd *cobra.Command, args []string) {
		providerType, _ := cmd.Flags().GetString("provider")

		provider, err := ai.ParseProvider(providerType)
		if err != nil {
			cmd.PrintErrln("Error parsing provider:", err)
			return
		}

		suggest.RunSuggestFlow(provider)
	},
}

func init() {
	suggestCmd.Flags().StringP("provider", "p", "", "AI provider to use (gpt|gemini|ollama). If empty, uses env or default")
	rootCmd.AddCommand(suggestCmd)
}
