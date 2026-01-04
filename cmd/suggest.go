package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/ai/provider"
	"huseynovvusal/gitai/internal/config"
	"huseynovvusal/gitai/internal/git"
	"huseynovvusal/gitai/internal/tui/suggest"
)

func MustGetBool(flags *pflag.FlagSet, name string) bool {
	val, err := flags.GetBool(name)
	if err != nil {
		panic(err)
	}
	return val
}

func NewSuggestCmd() *cobra.Command {
	cmd := &cobra.Command{
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

			amend := MustGetBool(cmd.Flags(), "amend")
			force := MustGetBool(cmd.Flags(), "force")

			if force && !amend {
				cmd.PrintErrln("Error: --force can only be used with --amend")
				return
			}

			flowConfig := suggest.FlowConfig{
				EditorMode:       cfg.Suggest.Editor,
				SecurityKeywords: cfg.Security.Keywords,
				Amend:            amend,
				ForcePush:        force,
			}

			flow := suggest.NewFlow(service, gitService, flowConfig, suggest.JiraHintProcessor, suggest.GitHubHintProcessor).
				WithHint(cfg.Suggest.Hint).
				WithSkipHint(cfg.Suggest.NoHint)
			flow.Run(rootCtx, args)
		},
	}

	cmd.Flags().StringP("provider", "p", "", "AI provider to use (gpt|gemini|ollama|geminicli). If empty, uses env or config or default")
	cmd.Flags().StringP("api_key", "k", "", "Optional API key to provide to AI provider")
	cmd.Flags().StringP("editor", "e", "system", "Editor to use for commit messages (builtin, system, or command)")
	cmd.Flags().Float64P("temperature", "t", 0.7, "Temperature for AI generation")
	cmd.Flags().Int64("max_tokens", 256, "Maximum tokens for AI generation")
	cmd.Flags().StringP("hint", "H", "", "Provide a hint for the commit message directly")
	cmd.Flags().Bool("no-hint", false, "Skip the hint input prompt")
	cmd.Flags().BoolP("amend", "a", false, "Amend the previous commit with the selected files and regenerated message")
	cmd.Flags().BoolP("force", "f", false, "Force push changes (only valid with --amend)")

	_ = viper.BindPFlag("ai.provider", cmd.Flags().Lookup("provider"))
	_ = viper.BindPFlag("ai.api_key", cmd.Flags().Lookup("api_key"))
	_ = viper.BindPFlag("suggest.editor", cmd.Flags().Lookup("editor"))
	_ = viper.BindPFlag("ai.temperature", cmd.Flags().Lookup("temperature"))
	_ = viper.BindPFlag("ai.max_tokens", cmd.Flags().Lookup("max_tokens"))
	_ = viper.BindPFlag("suggest.hint", cmd.Flags().Lookup("hint"))
	_ = viper.BindPFlag("suggest.no-hint", cmd.Flags().Lookup("no-hint"))

	return cmd
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