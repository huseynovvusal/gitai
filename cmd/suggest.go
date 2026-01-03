package cmd

import (
	"context"
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/ai/provider"
	"huseynovvusal/gitai/internal/git"
	"huseynovvusal/gitai/internal/tui/suggest"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var suggestCmd = &cobra.Command{
	Use:   "suggest [files...]",
	Short: "Suggest commit messages for changed files using AI",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		files, err := git.GetChangedFiles()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		repoRoot, err := git.GetGitRoot()
		if err != nil {
			return files, cobra.ShellCompDirectiveNoFileComp
		}

		cwd, err := os.Getwd()
		if err != nil {
			return files, cobra.ShellCompDirectiveNoFileComp
		}

		return getFilteredSuggestions(toComplete, args, files, repoRoot, cwd), cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		rootCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		provStr := viper.GetString("ai.provider")
		providerEnum, err := provider.ParseProvider(provStr)
		if err != nil {
			cmd.PrintErrln("Invalid provider:", err)
			return
		}

		aiProvider, err := provider.NewAIProvider(providerEnum)
		if err != nil {
			cmd.PrintErrln("Error creating AI provider:", err)
			return
		}

		service := ai.NewService(aiProvider)

		editorMode := viper.GetString("suggest.editor")

		flow := suggest.NewFlow(rootCtx, service, editorMode, suggest.JiraHintProcessor, suggest.GitHubHintProcessor)
		flow.Run(args)
	},
}

func init() {
	suggestCmd.Flags().StringP("provider", "p", "", "AI provider to use (gpt|gemini|ollama|geminicli). If empty, uses env or config or default")
	suggestCmd.Flags().StringP("api_key", "k", "", "Optional API key to provide to AI provider")
	suggestCmd.Flags().StringP("editor", "e", "system", "Editor to use for commit messages (builtin, system, or command)")
	suggestCmd.Flags().Float64P("temperature", "t", 0.7, "Temperature for AI generation")
	suggestCmd.Flags().Int64("max_tokens", 256, "Maximum tokens for AI generation")
	_ = viper.BindPFlag("ai.provider", suggestCmd.Flags().Lookup("provider"))
	_ = viper.BindPFlag("ai.api_key", suggestCmd.Flags().Lookup("api_key"))
	_ = viper.BindPFlag("suggest.editor", suggestCmd.Flags().Lookup("editor"))
	_ = viper.BindPFlag("ai.temperature", suggestCmd.Flags().Lookup("temperature"))
	_ = viper.BindPFlag("ai.max_tokens", suggestCmd.Flags().Lookup("max_tokens"))
	rootCmd.AddCommand(suggestCmd)
}

func getFilteredSuggestions(toComplete string, selectedArgs []string, changedFiles []string, repoRoot, cwd string) []string {
	selectedSet := make(map[string]bool)
	for _, arg := range selectedArgs {
		if resolved, err := git.ResolvePath(arg); err == nil {
			for _, r := range resolved {
				selectedSet[r] = true
			}
		}
	}

	var suggestions []string
	for _, f := range changedFiles {
		if selectedSet[f] {
			continue
		}

		relPath, err := filepath.Rel(cwd, filepath.Join(repoRoot, f))
		if err != nil {
			relPath = f
		}

		if strings.HasPrefix(toComplete, "./") && !strings.HasPrefix(relPath, "./") && !strings.HasPrefix(relPath, "..") {
			relPath = "./" + relPath
		}
		suggestions = append(suggestions, relPath)
	}
	return suggestions
}
