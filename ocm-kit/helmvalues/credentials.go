package helmvalues

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	"ocm.software/open-component-model/bindings/go/credentials"
	credcfgruntime "ocm.software/open-component-model/bindings/go/credentials/spec/config/runtime"
	genericv1 "ocm.software/open-component-model/bindings/go/configuration/generic/v1/spec"
	ocicredentials "ocm.software/open-component-model/bindings/go/oci/credentials"
	ocicredsspec "ocm.software/open-component-model/bindings/go/oci/spec/credentials"
	ocicredsv1 "ocm.software/open-component-model/bindings/go/oci/spec/credentials/v1"
	ociidentityv1 "ocm.software/open-component-model/bindings/go/oci/spec/identity/v1"
	"ocm.software/open-component-model/bindings/go/runtime"
)

// NewAuthClient builds an *auth.Client for talking to OCI registries whose
// credentials are resolved through the OCM credential graph.
//
// The graph unifies three credential sources, with anonymous fallback:
//
//   - OCM config file (~/.ocmconfig): both its Credentials/v1 consumers (keyed
//     by OCIRegistry consumer identities) and any DockerConfig/v1 repositories
//     it declares.
//   - The implicit docker config (~/.docker/config.json or $DOCKER_CONFIG): a
//     default DockerConfig/v1 repository is always injected so docker
//     credentials resolve even when no OCM config exists.
//   - Anonymous: any host without a match resolves to empty credentials rather
//     than failing the request.
//
// ocmConfigPath selects the OCM config file:
//   - "" auto-discovers: $OCM_CONFIG if set, otherwise ~/.ocmconfig. A missing
//     file is not an error — the graph is built from an empty credentials
//     config (plus the implicit docker repository).
//   - a non-empty value is used verbatim; a missing file is likewise tolerated.
//
// The client always returns a retrying client backed by an auth cache.
func NewAuthClient(ctx context.Context, ocmConfigPath string) (*auth.Client, error) {
	credConfig, err := loadCredentialConfig(ocmConfigPath)
	if err != nil {
		return nil, err
	}
	credConfig.Consumers = filterOCIRegistryConsumers(credConfig.Consumers)

	// Ensure the implicit docker config is resolvable by injecting a default
	// (empty) DockerConfig/v1 repository when none is configured. An empty
	// DockerConfig resolves against the default docker locations
	// ($DOCKER_CONFIG / ~/.docker/config.json).
	ensureDefaultDockerRepository(credConfig)

	ociRepo := &ocicredentials.OCICredentialRepository{}
	graph, err := credentials.ToGraph(ctx, credConfig, credentials.Options{
		RepositoryPluginProvider: credentials.GetRepositoryPluginFn(
			func(context.Context, runtime.Typed) (credentials.RepositoryPlugin, error) {
				return ociRepo, nil
			},
		),
		CredentialRepositoryTypeScheme: ociRepo.GetCredentialRepositoryScheme(),
		// Enables typed OCICredentials/v1 consumer credentials to be resolved as
		// *OCICredentials (instead of the DirectCredentials fallback), which is
		// what MapCredentials expects.
		CredentialTypeSchemeProvider: credentialTypeSchemeProvider{},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build credential graph: %w", err)
	}

	return &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: graphCredentialFunc(graph),
	}, nil
}

// graphCredentialFunc returns an auth.CredentialFunc that resolves per-host
// credentials via the credential graph. It never fails the request because one
// host has no credentials: not-found (and any resolution error) yields the
// anonymous credential.
func graphCredentialFunc(graph *credentials.Graph) auth.CredentialFunc {
	return func(ctx context.Context, hostport string) (auth.Credential, error) {
		identity, err := ociRegistryIdentity(hostport)
		if err != nil {
			// A malformed host cannot be authenticated; treat as anonymous.
			return auth.EmptyCredential, nil
		}

		resolved, err := graph.Resolve(ctx, identity)
		if err != nil {
			// Not-found is expected (public registries); any other resolution
			// error is also degraded to anonymous so one host cannot fail
			// another. Errors are not logged here to avoid surfacing secrets.
			return auth.EmptyCredential, nil
		}

		if creds, ok := resolved.(*ocicredsv1.OCICredentials); ok {
			return ocicredentials.MapCredentials(creds), nil
		}
		// A non-OCICredentials result (e.g. DirectCredentials) is not something
		// oras can consume for an OCI registry; treat as anonymous. With the
		// OCI credential type scheme wired in, OCI creds always resolve typed.
		return auth.EmptyCredential, nil
	}
}

// ociRegistryIdentity builds an OCIRegistry consumer identity for the given
// registry hostport (as passed by oras to a CredentialFunc, e.g. "ghcr.io" or
// "registry.example.com:5000").
func ociRegistryIdentity(hostport string) (runtime.Identity, error) {
	identity, err := runtime.ParseURLToIdentity(hostport)
	if err != nil {
		return nil, err
	}
	identity.SetType(ociidentityv1.Type)
	return identity, nil
}

// filterOCIRegistryConsumers drops non-OCIRegistry identities from each
// consumer, and the consumer entirely if none remain. OCM configs may declare
// consumers for other systems (e.g. HelmChartRepository credentials for Helm
// chart repos); this client only wires up plugin-based resolution for OCI
// credential types, so leaving such consumers in would otherwise trip
// plugin-based credential resolution against a nil CredentialPluginProvider.
func filterOCIRegistryConsumers(consumers []credcfgruntime.Consumer) []credcfgruntime.Consumer {
	filtered := make([]credcfgruntime.Consumer, 0, len(consumers))
	for _, consumer := range consumers {
		identities := make([]runtime.Identity, 0, len(consumer.Identities))
		for _, identity := range consumer.Identities {
			if typ, err := identity.ParseType(); err == nil && typ.Equal(ociidentityv1.Type) {
				identities = append(identities, identity)
			}
		}
		if len(identities) == 0 {
			continue
		}
		consumer.Identities = identities
		filtered = append(filtered, consumer)
	}
	return filtered
}

// loadCredentialConfig loads the credentials config from the resolved OCM config
// path. A missing file yields an empty (non-nil) credentials config so the graph
// can still be built with the implicit docker repository.
func loadCredentialConfig(ocmConfigPath string) (*credcfgruntime.Config, error) {
	path, err := resolveOCMConfigPath(ocmConfigPath)
	if err != nil {
		return nil, err
	}

	empty := &credcfgruntime.Config{}
	if path == "" {
		return empty, nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return empty, nil
		}
		return nil, fmt.Errorf("failed to open OCM config %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	generic := &genericv1.Config{}
	if err := genericv1.Scheme.Decode(file, generic); err != nil {
		return nil, fmt.Errorf("failed to decode OCM config %q: %w", path, err)
	}

	credConfig, err := credcfgruntime.LookupCredentialConfig(generic)
	if err != nil {
		return nil, fmt.Errorf("failed to extract credentials config from %q: %w", path, err)
	}
	if credConfig == nil {
		return empty, nil
	}
	return credConfig, nil
}

// resolveOCMConfigPath resolves the OCM config file path. An empty input honors
// $OCM_CONFIG, else ~/.ocmconfig. It returns "" (with no error) when discovery
// yields no candidate, signalling "no config file".
func resolveOCMConfigPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if p := os.Getenv("OCM_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// Home is not discoverable: proceed with no OCM config rather than fail.
		return "", nil
	}
	return filepath.Join(home, ".ocmconfig"), nil
}

// ensureDefaultDockerRepository appends a default (empty) DockerConfig/v1
// repository to the credentials config when none is already present, so the
// graph resolves the implicit docker config even without an OCM config.
func ensureDefaultDockerRepository(cfg *credcfgruntime.Config) {
	for _, entry := range cfg.Repositories {
		if entry.Repository == nil {
			continue
		}
		if entry.Repository.GetType().GetName() == ocicredsv1.DockerConfigType {
			return
		}
	}
	dockerRepo := &ocicredsv1.DockerConfig{}
	dockerRepo.SetType(ocicredsv1.DockerConfigVersionedType)
	cfg.Repositories = append(cfg.Repositories, credcfgruntime.RepositoryConfigEntry{
		Repository: dockerRepo,
	})
}

// credentialTypeSchemeProvider exposes the OCI credential payload type scheme
// (OCICredentials/v1) to the credential graph so typed consumer credentials are
// deserialized as *OCICredentials rather than the DirectCredentials fallback.
type credentialTypeSchemeProvider struct{}

func (credentialTypeSchemeProvider) GetCredentialTypeScheme() *runtime.Scheme {
	return ocicredsspec.CredentialTypeScheme
}
