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
	// --- Config File Definition ---
	viper.SetConfigName("gitai") // Searches for a file named 'gitai'

	// 1. System-wide configuration (highest precedence for fixed environment setup)
	viper.AddConfigPath("/etc/gitai/")

	// 2. User Home Directory paths (for user-specific settings)
	if home, err := os.UserHomeDir(); err == nil {
		// XDG Base Directory Specification (recommended user config path on modern Linux/Mac)
		// e.g., /home/user/.config/gitai/
		viper.AddConfigPath(filepath.Join(home, ".config", "gitai"))

		// Traditional dot-directory in home (common fallback)
		// e.g., /home/user/.gitai/
		viper.AddConfigPath(filepath.Join(home, ".gitai"))
	}
	// 3. Current Git repository root directory
	if gitRoot, err := git.GetGitRoot(); err == nil {
		viper.AddConfigPath(gitRoot)
	}

	// 4. Current Working Directory (for local development/overrides)
	viper.AddConfigPath(".")

	// --- Environment Variable Setup (High Precedence) ---

	// Sets the prefix for environment variables, e.g., GITAI_API_KEY
	viper.SetEnvPrefix("gitai")

	// Replaces dots in config keys with underscores for ENV var mapping
	// e.g., config key "ai.api_key" maps to ENV var GITAI_AI_API_KEY
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Enable automatic reading of environment variables
	viper.AutomaticEnv()
	_ = viper.BindEnv("ollama.path", "OLLAMA_API_PATH")
	_ = viper.BindEnv("ai.api_key", "OPENAI_API_KEY")
	_ = viper.BindEnv("ai.api_key", "GEMINI_API_KEY")
	_ = viper.BindEnv("ai.api_key", "GITAI_API_KEY")

	// --- Read Configuration ---

	// Read the config file if present;
	// Viper loads configuration from all found paths, merging them.
	_ = viper.ReadInConfig()
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
