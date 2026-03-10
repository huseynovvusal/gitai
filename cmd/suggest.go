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
		Long: `Suggest commit messages for changed files using AI.
You can optionally provide a hint as a positional argument if you've already selected files via flags or if you want to use it for all changed files.
Example: gitai suggest "fix: resolve auth issue"`,
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

			// Positional hint logic: if the first argument is not a file that exists, treat it as a hint.
			var files []string
			hint := cfg.Suggest.Hint

			for _, arg := range args {
				if _, err := os.Stat(arg); err == nil {
					files = append(files, arg)
				} else {
					if hint == "" {
						hint = arg
					} else {
						// If hint is already set, maybe append? For now just treat as file (which will be filtered out if invalid)
						files = append(files, arg)
					}
				}
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
				NoSession:   cfg.AI.NoSession,
			})
			if err != nil {
				if errors.Is(err, provider.ErrAPIKeyNotSet) {
					cmd.PrintErrln("Error: API key not set for provider", providerEnum)
					cmd.PrintErrln("Please set it in your config file (~/.config/gitai/gitai.yaml) or via environment variable (e.g. OPENAI_API_KEY)")
				} else {
					cmd.PrintErrln("Error creating AI provider:", err)
				}
				return
			}

			service := ai.NewService(aiProvider, cfg.Suggest.BulletPoint, cfg.AI.DebugFile)
			gitService := git.NewService()

			flowConfig := suggest.FlowConfig{
				EditorMode:       cfg.Suggest.Editor,
				SecurityKeywords: cfg.Security.Keywords,
				Amend:            cfg.Suggest.Amend,
				ForcePush:        cfg.Suggest.ForcePush,
				Atomic:           cfg.Suggest.Atomic,
				Verbose:          cfg.Suggest.Verbose,
			}

			flow := suggest.NewFlow(service, gitService, flowConfig, suggest.JiraHintProcessor, suggest.GitHubHintProcessor).
				WithHint(hint).
				WithSkipHint(cfg.Suggest.NoHint)
			flow.Run(rootCtx, files)
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
