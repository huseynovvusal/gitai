package cmd

import (
	"fmt"
	"huseynovvusal/gitai/internal/git"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "gitai",
	Short: "Gitai is a CLI tool to interact with Git repositories using AI",
	Long:  `Gitai allows you to perform various Git operations with the help of AI, making version control easier and more intuitive.`,
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.SetConfigName("gitai")

	// 1. System-wide configuration
	viper.AddConfigPath("/etc/gitai/")

	// 2. User Home Directory paths
	if home, err := os.UserHomeDir(); err == nil {
		// XDG Base Directory Specification (recommended user config path on modern Linux/Mac)
		viper.AddConfigPath(filepath.Join(home, ".config", "gitai"))

		// Traditional dot-directory in home
		viper.AddConfigPath(filepath.Join(home, ".gitai"))
	}
	// 3. Current Git repository root directory
	if gitRoot, err := git.GetGitRoot(); err == nil {
		viper.AddConfigPath(gitRoot)
	}

	// 4. Current Working Directory
	viper.AddConfigPath(".")

	// Sets the prefix for environment variables, e.g., GITAI_API_KEY
	viper.SetEnvPrefix("gitai")

	// config key "ai.api_key" maps to ENV var GITAI_AI_API_KEY
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.AutomaticEnv()
	_ = viper.BindEnv("ollama.path", "OLLAMA_API_PATH")
	_ = viper.BindEnv("ai.api_key", "OPENAI_API_KEY")
	_ = viper.BindEnv("ai.api_key", "GEMINI_API_KEY")
	_ = viper.BindEnv("ai.api_key", "GOOGLE_API_KEY")
	_ = viper.BindEnv("ai.api_key", "GITAI_API_KEY")

	_ = viper.ReadInConfig()
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
