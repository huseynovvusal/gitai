package cmd

import (
	"context"
	"errors"
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
				var invalidError *provider.InvalidProviderError
				if errors.As(err, &invalidError) {
					cmd.PrintErrln(err)
				} else {
					cmd.PrintErrln("Error parsing provider:", err)
				}

				return
			}

			aiProvider, err := provider.NewAIProvider(providerEnum, provider.Config{
				APIKey:      cfg.AI.APIKey,
				MaxTokens:   cfg.AI.MaxTokens,
				Temperature: cfg.AI.Temperature,
				Model:       cfg.AI.Model,
				OllamaPath:  cfg.Ollama.Path,
			})
			if err != nil {
				cmd.PrintErrln("Error creating AI provider:", err)

				return
			}

			service := ai.NewService(aiProvider, cfg.Suggest.BulletPoint)

			gitService := git.NewService()

			amend := cfg.Suggest.Amend
			force := cfg.Suggest.ForcePush

			if force && !amend {
				cmd.PrintErrln("Error: --force can only be used with --amend")
				return
			}

			flowConfig := suggest.FlowConfig{
				EditorMode:       cfg.Suggest.Editor,
				SecurityKeywords: cfg.Security.Keywords,
				Amend:            amend,
				ForcePush:        force,
				Atomic:           cfg.Suggest.Atomic,
				Verbose:          cfg.Suggest.Verbose,
			}

			flow := suggest.NewFlow(service, gitService, flowConfig, suggest.JiraHintProcessor, suggest.GitHubHintProcessor).
				WithHint(cfg.Suggest.Hint).
				WithSkipHint(cfg.Suggest.NoHint)
			flow.Run(rootCtx, args)
		},
	}

	config.RegisterSuggestFlags(cmd)

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
