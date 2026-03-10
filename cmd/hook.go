package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"huseynovvusal/gitai/internal/hook"
)

func NewHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage gitai git hooks",
		Long:  `Install or uninstall a prepare-commit-msg git hook that automatically generates commit messages using gitai.`,
	}

	cmd.AddCommand(newHookInstallCmd())
	cmd.AddCommand(newHookUninstallCmd())
	cmd.AddCommand(newHookStatusCmd())

	return cmd
}

func newHookInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the prepare-commit-msg hook",
		Long: `Install a prepare-commit-msg git hook that runs gitai automatically
when you use "git commit" without the -m flag.

The hook respects existing prepare-commit-msg hooks by appending
rather than overwriting. Set GITAI_SKIP_HOOK=1 to bypass the hook.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := hook.Install(); err != nil {
				return fmt.Errorf("install hook: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "gitai hook installed successfully.")
			fmt.Fprintln(cmd.OutOrStdout(), "Commit messages will be generated automatically when you run 'git commit'.")
			fmt.Fprintln(cmd.OutOrStdout(), "Set GITAI_SKIP_HOOK=1 to bypass the hook.")
			return nil
		},
	}
}

func newHookUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the prepare-commit-msg hook",
		Long:  `Remove the gitai snippet from the prepare-commit-msg hook. Other hook content is preserved.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := hook.Uninstall(); err != nil {
				return fmt.Errorf("uninstall hook: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "gitai hook uninstalled successfully.")
			return nil
		},
	}
}

func newHookStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check if the gitai hook is installed",
		RunE: func(cmd *cobra.Command, args []string) error {
			installed, err := hook.IsInstalled()
			if err != nil {
				return fmt.Errorf("check hook status: %w", err)
			}
			if installed {
				fmt.Fprintln(cmd.OutOrStdout(), "gitai hook is installed.")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "gitai hook is not installed.")
			}
			return nil
		},
	}
}
