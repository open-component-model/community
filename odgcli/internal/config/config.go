// Package config handles application configuration via config file, environment
// variables, and system keychain for secrets.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zalando/go-keyring"
)

const (
	// KeyringService is the service name used for storing secrets in the system keychain.
	KeyringService = "odgcli"
	// KeyringAccessTokenKey is the keychain key under which the access token is stored.
	KeyringAccessTokenKey = "access_token"
)

// Config holds all application configuration.
type Config struct {
	BaseURL       string `mapstructure:"base_url"`
	GithubURL     string `mapstructure:"github_url"`
	AccessToken   string `mapstructure:"access_token"`
	RootComponent string `mapstructure:"root_component"`
}

// DefaultConfigDir returns the default configuration directory path.
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "odgcli")
}

// DefaultConfigPath returns the default configuration file path.
func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
}

// BindFlags binds cobra flags and environment variables to viper keys.
func BindFlags(cmd *cobra.Command) {
	viper.BindPFlag("root_component", cmd.PersistentFlags().Lookup("root")) //nolint:errcheck
	viper.BindEnv("base_url", "BASE_URL")                                   //nolint:errcheck
	viper.BindEnv("github_url", "GITHUB_URL")                               //nolint:errcheck
	viper.BindEnv("access_token", "ACCESS_TOKEN")                           //nolint:errcheck
	viper.BindEnv("root_component", "ODG_ROOT")                             //nolint:errcheck
}

// SetConfigFile configures viper to read from the given config file path.
// If configPath is empty, the default path (~/.config/odgcli/config.yaml) is used.
func SetConfigFile(configPath string) {
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(DefaultConfigDir())
	}
}

// Load reads the config file and resolves all configuration values.
// BindFlags and SetConfigFile should be called before this.
//
// Resolution order per value:
//  1. Cobra flag (--root)
//  2. Environment variable (ACCESS_TOKEN, BASE_URL, GITHUB_URL, ODG_ROOT)
//  3. System keychain (access token only)
//  4. Config file
func Load() (*Config, error) {
	// Read config file (not an error if it doesn't exist).
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Only error if the file exists but can't be read/parsed.
			if _, statErr := os.Stat(viper.ConfigFileUsed()); statErr == nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Resolve access token: flag/env > keyring > config file.
	// Flag, env, and config file are already handled by viper above.
	// Check keyring if still empty.
	if cfg.AccessToken == "" {
		if token, err := keyring.Get(KeyringService, KeyringAccessTokenKey); err == nil && token != "" {
			cfg.AccessToken = token
		}
	}

	// Validate required fields.
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("base_url must be set (via config file, or BASE_URL env var)")
	}
	if cfg.GithubURL == "" {
		return nil, fmt.Errorf("github_url must be set (via config file, or GITHUB_URL env var)")
	}
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("access_token must be set (via ACCESS_TOKEN env var, system keychain, or config file)")
	}
	if cfg.RootComponent == "" {
		return nil, fmt.Errorf("root_component must be set (via --root flag, ODG_ROOT env var, or config file)")
	}

	return &cfg, nil
}

// StoreAccessToken stores the access token in the system keychain.
func StoreAccessToken(token string) error {
	return keyring.Set(KeyringService, KeyringAccessTokenKey, token)
}

// DeleteAccessToken removes the access token from the system keychain.
func DeleteAccessToken() error {
	return keyring.Delete(KeyringService, KeyringAccessTokenKey)
}

// EnsureConfigDir creates the config directory with appropriate permissions if
// it doesn't exist.
func EnsureConfigDir() error {
	dir := DefaultConfigDir()
	return os.MkdirAll(dir, 0700)
}
