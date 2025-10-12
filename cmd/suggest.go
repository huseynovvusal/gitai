package cmd

import (
	"context"
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/tui/suggest"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Suggest commit messages for changed files using AI",
	Run: func(cmd *cobra.Command, args []string) {
		rootCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		provStr := viper.GetString("ai.provider")
		provider, err := ai.ParseProvider(provStr)
		if err != nil {
			cmd.PrintErrln("Invalid provider:", err)
			return
		}

		suggest.RunSuggestFlow(rootCtx, provider)
	},
}

func init() {
	suggestCmd.Flags().StringP("provider", "p", "", "AI provider to use (gpt|gemini|ollama|geminicli). If empty, uses env or config or default")
	suggestCmd.Flags().StringP("api_key", "k", "", "Optional API key to provide to AI provider")
	_ = viper.BindPFlag("ai.provider", suggestCmd.Flags().Lookup("provider"))
	_ = viper.BindPFlag("ai.api_key", suggestCmd.Flags().Lookup("provider"))
	rootCmd.AddCommand(suggestCmd)
}
