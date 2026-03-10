package config

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RegisterSuggestFlags registers the flags for the suggest command and binds them to viper.
func RegisterSuggestFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("provider", "p", "", "AI provider to use (gpt|gemini|ollama|geminicli). If empty, uses env or config or default")
	cmd.Flags().StringP("api_key", "k", "", "Optional API key to provide to AI provider")
	cmd.Flags().StringP("editor", "e", "system", "Editor to use for commit messages (builtin, system, or command)")
	cmd.Flags().Float64P("temperature", "t", 0.7, "Temperature for AI generation")
	cmd.Flags().Int64("max_tokens", 256, "Maximum tokens for AI generation")
	cmd.Flags().StringP("hint", "H", "", "Provide a hint for the commit message directly")
	cmd.Flags().Bool("no-hint", false, "Skip the hint input prompt")
	cmd.Flags().BoolP("amend", "a", false, "Amend the previous commit with the selected files and regenerated message")
	cmd.Flags().BoolP("force", "f", false, "Force push changes (only valid with --amend)")

	cmd.Flags().Bool("atomic", false, "Suggest atomic splits of changes into multiple commits")
	cmd.Flags().BoolP("verbose", "v", false, "Show verbose output (e.g., token usage)")
	cmd.Flags().String("debug-file", "", "Path to a file where the AI prompt will be logged for debugging")
	cmd.Flags().Bool("no-session", false, "Do not attempt to resume the latest Gemini session (may be faster)")

	_ = viper.BindPFlag("ai.provider", cmd.Flags().Lookup("provider"))
	_ = viper.BindPFlag("ai.api_key", cmd.Flags().Lookup("api_key"))
	_ = viper.BindPFlag("suggest.editor", cmd.Flags().Lookup("editor"))
	_ = viper.BindPFlag("ai.temperature", cmd.Flags().Lookup("temperature"))
	_ = viper.BindPFlag("ai.max_tokens", cmd.Flags().Lookup("max_tokens"))
	_ = viper.BindPFlag("suggest.hint", cmd.Flags().Lookup("hint"))
	_ = viper.BindPFlag("suggest.no-hint", cmd.Flags().Lookup("no-hint"))
	_ = viper.BindPFlag("suggest.amend", cmd.Flags().Lookup("amend"))
	_ = viper.BindPFlag("suggest.force_push", cmd.Flags().Lookup("force"))
	_ = viper.BindPFlag("suggest.atomic", cmd.Flags().Lookup("atomic"))
	_ = viper.BindPFlag("suggest.verbose", cmd.Flags().Lookup("verbose"))
	_ = viper.BindPFlag("ai.debug_file", cmd.Flags().Lookup("debug-file"))
	_ = viper.BindPFlag("ai.no_session", cmd.Flags().Lookup("no-session"))
}

// RegisterReviewFlags registers the flags for the review command and binds them to viper.
func RegisterReviewFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("provider", "p", "", "AI provider to use (gpt|gemini|ollama|geminicli). If empty, uses env or config or default")
	cmd.Flags().StringP("api_key", "k", "", "Optional API key to provide to AI provider")
	cmd.Flags().Float64P("temperature", "t", 0.7, "Temperature for AI generation")
	cmd.Flags().Int64("max_tokens", 1024, "Maximum tokens for AI generation")
	cmd.Flags().StringP("hint", "H", "", "Provide a review focus hint (e.g. 'check for SQL injection')")
	cmd.Flags().Bool("no-hint", false, "Skip the hint input prompt")
	cmd.Flags().String("format", "text", "Output format: text or json")

	_ = viper.BindPFlag("ai.provider", cmd.Flags().Lookup("provider"))
	_ = viper.BindPFlag("ai.api_key", cmd.Flags().Lookup("api_key"))
	_ = viper.BindPFlag("ai.temperature", cmd.Flags().Lookup("temperature"))
	_ = viper.BindPFlag("ai.max_tokens", cmd.Flags().Lookup("max_tokens"))
	_ = viper.BindPFlag("review.hint", cmd.Flags().Lookup("hint"))
	_ = viper.BindPFlag("review.no-hint", cmd.Flags().Lookup("no-hint"))
}
