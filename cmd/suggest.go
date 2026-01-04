package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/ai/provider"
	"huseynovvusal/gitai/internal/config"
	"huseynovvusal/gitai/internal/git"
	"huseynovvusal/gitai/internal/tui/suggest"
)

var suggestCmd = &cobra.Command{
	Use:   "suggest [files...]",
	Short: "Suggest commit messages for changed files using AI",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		gitService := git.NewService()
		files, err := gitService.GetChangedFiles()
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

		return getFilteredSuggestions(toComplete, args, files, repoRoot, cwd, gitService), cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		rootCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cfg, err := config.LoadConfig(viper.GetViper())
		if err != nil {
			cmd.PrintErrln("Error loading config:", err)

			return
		}

		providerEnum, err := provider.ParseProvider(cfg.AI.Provider)
		if err != nil {
			cmd.PrintErrln("Invalid provider:", err)

			return
		}

		aiProvider, err := provider.NewAIProvider(providerEnum, provider.Config{
			APIKey:      cfg.AI.APIKey,
			MaxTokens:   cfg.AI.MaxTokens,
			Temperature: cfg.AI.Temperature,
			OllamaPath:  cfg.Ollama.Path,
		})
		if err != nil {
			cmd.PrintErrln("Error creating AI provider:", err)

			return
		}

		service := ai.NewService(aiProvider, cfg.Suggest.BulletPoint)

		gitService := git.NewService()

		flowConfig := suggest.FlowConfig{
			EditorMode:       cfg.Suggest.Editor,
			SecurityKeywords: cfg.Security.Keywords,
		}

		flow := suggest.NewFlow(service, gitService, flowConfig, suggest.JiraHintProcessor, suggest.GitHubHintProcessor).
			WithHint(cfg.Suggest.Hint).
			WithSkipHint(cfg.Suggest.NoHint)
		flow.Run(rootCtx, args)
	},
}

func init() {
	suggestCmd.Flags().StringP("provider", "p", "", "AI provider to use (gpt|gemini|ollama|geminicli). If empty, uses env or config or default")
	suggestCmd.Flags().StringP("api_key", "k", "", "Optional API key to provide to AI provider")
	suggestCmd.Flags().StringP("editor", "e", "system", "Editor to use for commit messages (builtin, system, or command)")
	suggestCmd.Flags().Float64P("temperature", "t", 0.7, "Temperature for AI generation")
	suggestCmd.Flags().Int64("max_tokens", 256, "Maximum tokens for AI generation")
	suggestCmd.Flags().StringP("hint", "H", "", "Provide a hint for the commit message directly")
	suggestCmd.Flags().Bool("no-hint", false, "Skip the hint input prompt")

	_ = viper.BindPFlag("ai.provider", suggestCmd.Flags().Lookup("provider"))
	_ = viper.BindPFlag("ai.api_key", suggestCmd.Flags().Lookup("api_key"))
	_ = viper.BindPFlag("suggest.editor", suggestCmd.Flags().Lookup("editor"))
	_ = viper.BindPFlag("ai.temperature", suggestCmd.Flags().Lookup("temperature"))
	_ = viper.BindPFlag("ai.max_tokens", suggestCmd.Flags().Lookup("max_tokens"))
	_ = viper.BindPFlag("suggest.hint", suggestCmd.Flags().Lookup("hint"))
	_ = viper.BindPFlag("suggest.no-hint", suggestCmd.Flags().Lookup("no-hint"))

}

type gitService interface {
	ResolvePath(path string) ([]string, error)
}

func getFilteredSuggestions(toComplete string, selectedArgs []string, changedFiles []string, repoRoot, cwd string, gs gitService) []string {
	selectedSet := make(map[string]bool)

	for _, arg := range selectedArgs {
		if resolved, err := gs.ResolvePath(arg); err == nil {
			for _, r := range resolved {
				selectedSet[r] = true
			}
		}
	}

	suggestions := make([]string, 0, len(changedFiles))

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
