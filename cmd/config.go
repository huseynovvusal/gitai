package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"huseynovvusal/gitai/internal/config"
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage gitai configuration",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			config.LoadConfig(viper.GetViper())
		},
	}

	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigListCmd())

	return cmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			val := viper.Get(args[0])
			if val == nil {
				fmt.Printf("Key %s not found\n", args[0])
				return
			}
			fmt.Printf("%v\n", val)
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]

			viper.Set(key, value)

			home, err := os.UserHomeDir()
			if err != nil {
				fmt.Println("Error getting home directory:", err)
				return
			}

			configDir := filepath.Join(home, ".config", "gitai")
			if _, err := os.Stat(configDir); os.IsNotExist(err) {
				_ = os.MkdirAll(configDir, 0755)
			}

			configPath := filepath.Join(configDir, "gitai.yaml")

			err = viper.WriteConfigAs(configPath)
			if err != nil {
				fmt.Println("Error writing config file:", err)
				return
			}

			fmt.Printf("Set %s to %s (saved to %s)\n", key, value, configPath)
		},
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		Run: func(cmd *cobra.Command, args []string) {
			settings := viper.AllSettings()
			keys := make([]string, 0, len(settings))
			for k := range settings {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				printSettings(k, settings[k], 0)
			}
		},
	}
}

func printSettings(key string, value interface{}, indent int) {
	prefix := strings.Repeat("  ", indent)
	if m, ok := value.(map[string]interface{}); ok {
		fmt.Printf("%s%s:\n", prefix, key)
		subKeys := make([]string, 0, len(m))
		for k := range m {
			subKeys = append(subKeys, k)
		}
		sort.Strings(subKeys)
		for _, sk := range subKeys {
			printSettings(sk, m[sk], indent+1)
		}
	} else {
		fmt.Printf("%s%s: %v\n", prefix, key, value)
	}
}
