package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"

	"github.com/open-component-model/community/odgcli/internal/config"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  "Store, remove, or check the status of your access token in the system keychain.",
	// Override root's PersistentPreRunE so auth commands work without a valid config.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Store an access token in the system keychain",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print("Paste your access token: ")
		tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // newline after hidden input
		if err != nil {
			return fmt.Errorf("failed to read token: %w", err)
		}
		token := strings.TrimSpace(string(tokenBytes))

		if token == "" {
			return fmt.Errorf("token cannot be empty")
		}

		if err := config.StoreAccessToken(token); err != nil {
			return fmt.Errorf("failed to store token in keychain: %w", err)
		}

		fmt.Println("Token stored in system keychain.")
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the access token from the system keychain",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.DeleteAccessToken(); err != nil {
			return fmt.Errorf("failed to remove token from keychain: %w", err)
		}

		fmt.Println("Token removed from system keychain.")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show where the access token is sourced from",
	Run: func(cmd *cobra.Command, args []string) {
		// Check env var.
		if os.Getenv("ACCESS_TOKEN") != "" {
			fmt.Println("Token source: ACCESS_TOKEN environment variable")
			return
		}

		// Check keychain.
		if token, err := keyring.Get(config.KeyringService, config.KeyringAccessTokenKey); err == nil && token != "" {
			fmt.Println("Token source: system keychain")
			return
		}

		// Check config file.
		config.SetConfigFile(configPath)
		loadedCfg, err := config.Load()
		if err == nil && loadedCfg.AccessToken != "" {
			fmt.Println("Token source: config file (" + config.DefaultConfigPath() + ")")
			return
		}

		fmt.Println("No access token found.")
		fmt.Println("Run 'odgcli auth login' to store a token in the system keychain,")
		fmt.Println("or set the ACCESS_TOKEN environment variable.")
	},
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
	rootCmd.AddCommand(authCmd)
}
