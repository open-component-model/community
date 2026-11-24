// Package ocm bootstraps the OCM runtime the same way the ocm CLI does.
// View-specific operations live in the view packages.
package ocm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"ocm.software/open-component-model/bindings/go/cli/cmd/configuration"
	checksumhttpv1alpha1 "ocm.software/open-component-model/bindings/go/configuration/checksum/http/v1alpha1/spec"
	filesystemv1alpha1 "ocm.software/open-component-model/bindings/go/configuration/filesystem/v1alpha1/spec"
	genericv1 "ocm.software/open-component-model/bindings/go/configuration/generic/v1/spec"
	"ocm.software/open-component-model/bindings/go/credentials"
	credentialsruntime "ocm.software/open-component-model/bindings/go/credentials/spec/config/runtime"
	httpv1alpha1 "ocm.software/open-component-model/bindings/go/http/spec/config/v1alpha1"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/repository"
	"ocm.software/open-component-model/bindings/go/runtime"
)

// Runtime holds the plugin manager, credential graph and HTTP settings shared by all views.
type Runtime struct {
	Plugins     *manager.PluginManager
	Credentials credentials.Resolver
	HTTP        *httpv1alpha1.Config
}

// Bootstrap loads the OCM configuration, registers builtin and external plugins
// and builds the credential graph.
func Bootstrap(ctx context.Context) (*Runtime, error) {
	cfg, err := configuration.GetOCMConfig(configuration.OCMConfigOptions{
		Stat:        os.Stat,
		Getenv:      os.Getenv,
		UserHomeDir: os.UserHomeDir,
		Getwd:       os.Getwd,
		Executable:  os.Executable,
	})
	if err != nil {
		slog.DebugContext(ctx, "no OCM config loaded, using defaults", slog.String("error", err.Error()))
		cfg = &genericv1.Config{}
	}
	fsCfg, err := filesystemv1alpha1.LookupConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("reading filesystem config: %w", err)
	}
	httpCfg, err := httpv1alpha1.ResolveHTTPConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("reading http config: %w", err)
	}
	checksumCfg, err := checksumhttpv1alpha1.LookupConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("reading checksum-http config: %w", err)
	}

	pm := manager.NewPluginManager(ctx)
	if home, err := os.UserHomeDir(); err == nil {
		err := pm.RegisterPlugins(ctx, filepath.Join(home, ".ocm", "plugins"), manager.WithIdleTimeout(time.Hour))
		if err != nil && !errors.Is(err, manager.ErrNoPluginsFound) && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("registering plugins: %w", err)
		}
	}
	if err := registerBuiltins(pm, fsCfg, httpCfg, checksumCfg); err != nil {
		return nil, fmt.Errorf("registering builtin plugins: %w", err)
	}

	credCfg, err := credentialsruntime.LookupCredentialConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("reading credential config: %w", err)
	}
	if credCfg == nil {
		credCfg = &credentialsruntime.Config{}
	}
	graph, err := credentials.ToGraph(ctx, credCfg, credentials.Options{
		RepositoryPluginProvider:       pm.CredentialRepositoryRegistry,
		CredentialPluginProvider:       pm.CredentialPluginRegistry,
		CredentialRepositoryTypeScheme: pm.CredentialRepositoryRegistry.RepositoryScheme(),
		CredentialTypeSchemeProvider:   pm.CredentialTypeRegistry,
		ConsumerIdentityTypeScheme:     pm.CredentialTypeRegistry.GetConsumerIdentityTypeScheme(),
	})
	if err != nil {
		return nil, fmt.Errorf("building credential graph: %w", err)
	}

	return &Runtime{Plugins: pm, Credentials: graph, HTTP: httpCfg}, nil
}

// Shutdown stops all external plugins.
func (r *Runtime) Shutdown(ctx context.Context) error {
	return r.Plugins.Shutdown(ctx)
}

// Resolve returns the credentials configured for identity, or nil when there are none.
func (r *Runtime) Resolve(ctx context.Context, identity runtime.Identity) (runtime.Typed, error) {
	creds, err := r.Credentials.Resolve(ctx, identity)
	if errors.Is(err, credentials.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolving credentials: %w", err)
	}
	return creds, nil
}

// Repository connects to the component version repository of spec with its credentials.
func (r *Runtime) Repository(ctx context.Context, spec runtime.Typed) (repository.ComponentVersionRepository, error) {
	if spec == nil {
		return nil, errors.New("reference has no repository")
	}
	registry := r.Plugins.ComponentVersionRepositoryRegistry
	var creds runtime.Typed
	if identity, err := registry.GetComponentVersionRepositoryCredentialConsumerIdentity(ctx, spec); err == nil {
		if creds, err = r.Resolve(ctx, identity); err != nil {
			return nil, err
		}
	}
	repo, err := registry.GetComponentVersionRepository(ctx, spec, creds)
	if err != nil {
		return nil, fmt.Errorf("connecting to repository: %w", err)
	}
	return repo, nil
}
