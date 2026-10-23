package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/open-component-model/community/odgcli/internal/config"
	"github.com/open-component-model/community/odgcli/internal/tui"
	"github.com/open-component-model/community/odgcli/pkg/github"
	"github.com/open-component-model/community/odgcli/pkg/odg"
)

// cfg holds shared configuration populated in PersistentPreRunE and
// available to all subcommands.
var cfg *config.Config

// configPath allows overriding the config file location via --config flag.
var configPath string

var rootCmd = &cobra.Command{
	Use:   "odgcli",
	Short: "CVE compliance tooling for Open Delivery Gear",
	Long:  "odgcli provides a TUI for browsing CVE compliance data from Open Delivery Gear.",
	// When invoked without a subcommand, start the interactive TUI.
	RunE: func(cmd *cobra.Command, args []string) error {
		ghClient := github.NewClient(cfg.GithubURL, cfg.AccessToken)
		odgClient, err := odg.NewClient(context.Background(), cfg.BaseURL, ghClient)
		if err != nil {
			return fmt.Errorf("failed to create ODG client: %w", err)
		}
		return tui.Run(odgClient, ghClient, cfg.RootComponent)
	},
	// Silence cobra's default usage/error printing so the TUI stays clean.
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	// Define flags.
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path (default: ~/.config/odgcli/config.yaml)")
	rootCmd.PersistentFlags().String("root", "", "Root component name (overrides config file and ODG_ROOT env var)")

	// Bind flags and env vars to viper keys.
	config.BindFlags(rootCmd)

	// Load config before each command that needs it.
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		config.SetConfigFile(configPath)

		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}
		return nil
	}
}

// Execute is the main entry point called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
