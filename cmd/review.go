package cmd

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/ai/provider"
	"huseynovvusal/gitai/internal/config"
	"huseynovvusal/gitai/internal/git"
	"huseynovvusal/gitai/internal/tui/review"
)

func NewReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review [files...]",
		Short: "Review changed files for potential issues using AI",
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

			reviewer := ai.NewReviewService(aiProvider)
			gitService := git.NewService()

			format, _ := cmd.Flags().GetString("format")

			flowConfig := review.FlowConfig{
				Hint:   cfg.Review.Hint,
				NoHint: cfg.Review.NoHint,
				Format: format,
			}

			flow := review.NewFlow(reviewer, gitService, flowConfig)
			flow.Run(rootCtx, args)
		},
	}

	config.RegisterReviewFlags(cmd)

	return cmd
}
