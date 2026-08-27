// Package ocm bootstraps the OCM runtime for the standalone TUI binary.
// It uses only the public bindings API, not CLI internals.
package ocm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	genericv1 "ocm.software/open-component-model/bindings/go/configuration/generic/v1/spec"
	"ocm.software/open-component-model/bindings/go/credentials"
	credentialsRuntime "ocm.software/open-component-model/bindings/go/credentials/spec/config/runtime"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
)

// Runtime holds the bootstrapped OCM runtime components.
type Runtime struct {
	Config          *genericv1.Config
	PluginManager   *manager.PluginManager
	CredentialGraph credentials.Resolver
}

// Bootstrap initializes the OCM runtime.
func Bootstrap(ctx context.Context) (*Runtime, error) {
	// 1. Load OCM config.
	cfg, err := loadOCMConfig()
	if err != nil {
		slog.Debug("could not load OCM config, using defaults", slog.String("error", err.Error()))
		cfg = &genericv1.Config{}
	}

	// 2. Create plugin manager and discover plugins.
	pm := manager.NewPluginManager(ctx)

	pluginDir := defaultPluginDir()
	if pluginDir != "" {
		if err := pm.RegisterPlugins(ctx, pluginDir,
			manager.WithIdleTimeout(time.Hour),
		); err != nil && !errors.Is(err, manager.ErrNoPluginsFound) {
			return nil, fmt.Errorf("registering plugins from %s: %w", pluginDir, err)
		}
	}

	// Register builtin OCI support.
	if err := registerBuiltins(pm); err != nil {
		return nil, fmt.Errorf("registering builtins: %w", err)
	}

	// 3. Build credential graph.
	graph, err := buildCredentialGraph(ctx, cfg, pm)
	if err != nil {
		return nil, fmt.Errorf("building credential graph: %w", err)
	}

	return &Runtime{
		Config:          cfg,
		PluginManager:   pm,
		CredentialGraph: graph,
	}, nil
}

// Shutdown cleanly stops the plugin manager.
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r.PluginManager != nil {
		return r.PluginManager.Shutdown(ctx)
	}
	return nil
}

func buildCredentialGraph(ctx context.Context, cfg *genericv1.Config, pm *manager.PluginManager) (credentials.Resolver, error) {
	opts := credentials.Options{
		RepositoryPluginProvider: pm.CredentialRepositoryRegistry,
		CredentialPluginProvider: credentials.GetCredentialPluginFn(
			func(_ context.Context, typed runtime.Typed) (credentials.CredentialPlugin, error) {
				return nil, fmt.Errorf("no credential plugin for type %s", typed)
			},
		),
		CredentialRepositoryTypeScheme: pm.CredentialRepositoryRegistry.RepositoryScheme(),
	}

	credCfg, err := credentialsRuntime.LookupCredentialConfig(cfg)
	if err != nil || credCfg == nil {
		credCfg = &credentialsRuntime.Config{}
	}

	return credentials.ToGraph(ctx, credCfg, opts)
}

func defaultPluginDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".ocm", "plugins")
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	return ""
}

// --- Config loading ---

func loadOCMConfig() (*genericv1.Config, error) {
	paths := configPaths()
	if len(paths) == 0 {
		return nil, fmt.Errorf("no OCM config found")
	}
	var cfgs []*genericv1.Config
	for _, path := range paths {
		cfg, err := loadConfigFile(path)
		if err != nil {
			continue
		}
		slog.Debug("loaded OCM config", slog.String("path", path))
		cfgs = append(cfgs, cfg)
	}
	return genericv1.FlatMap(cfgs...), nil
}

func loadConfigFile(path string) (*genericv1.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg genericv1.Config
	if err := genericv1.Scheme.Decode(f, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func configPaths() []string {
	var paths []string
	if env := os.Getenv("OCM_CONFIG"); env != "" {
		if _, err := os.Stat(env); err == nil {
			paths = append(paths, env)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, name := range []string{".ocm/config", ".ocmconfig"} {
			for _, base := range []string{
				filepath.Join(home, ".config"),
				home,
			} {
				c := filepath.Join(base, name)
				if _, err := os.Stat(c); err == nil {
					paths = append(paths, c)
				}
			}
		}
	}
	return paths
}
