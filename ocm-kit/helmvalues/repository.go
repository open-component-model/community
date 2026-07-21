package helmvalues

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	"ocm.software/open-component-model/bindings/go/blob"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ocm.software/open-component-model/bindings/go/oci"
	urlresolver "ocm.software/open-component-model/bindings/go/oci/resolver/url"
)

// Repository is the narrow surface helmvalues needs from an OCM repository.
// It is public so consumers and tests can supply their own.
type Repository interface {
	// GetComponentVersion resolves a component descriptor.
	GetComponentVersion(ctx context.Context, name, version string) (*descriptor.Descriptor, error)
	// ResourceBytes downloads the local blob of the named resource.
	ResourceBytes(ctx context.Context, name, version, resourceName string) ([]byte, error)
}

// OCIRepository implements Repository over the OCM oci bindings.
type OCIRepository struct {
	repo *oci.Repository
	// tempDir is the blob-staging directory created by OpenRepository and
	// removed by Close.
	tempDir string
}

// RepositoryOption configures OpenRepository.
type RepositoryOption func(*repositoryConfig)

type repositoryConfig struct {
	authClient *auth.Client
}

// WithAuthClient sets the OCI auth client used for registry requests. Without
// it, requests are anonymous.
func WithAuthClient(client *auth.Client) RepositoryOption {
	return func(c *repositoryConfig) { c.authClient = client }
}

// OpenRepository opens an OCI-registry-backed OCM repository at url. The url
// carries the scheme: "http://..." selects plain HTTP, while "https://...",
// "oci://..." or a bare "host/path" select TLS. Call Close when done to remove
// the repository's blob-staging directory.
func OpenRepository(url string, opts ...RepositoryOption) (*OCIRepository, error) {
	cfg := &repositoryConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	client := cfg.authClient
	if client == nil {
		client = &auth.Client{Client: retry.DefaultClient, Cache: auth.NewCache()}
	}

	tempDir, err := os.MkdirTemp("", "ocm-kit-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	baseURL, plainHTTP := splitScheme(url)
	resolver, err := urlresolver.New(
		urlresolver.WithBaseURL(baseURL),
		urlresolver.WithPlainHTTP(plainHTTP),
		urlresolver.WithBaseClient(client),
	)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to create resolver: %w", err)
	}
	repo, err := oci.NewRepository(oci.WithResolver(resolver), oci.WithTempDir(tempDir))
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}
	return &OCIRepository{repo: repo, tempDir: tempDir}, nil
}

// Close removes the repository's blob-staging directory.
func (o *OCIRepository) Close() error {
	if o.tempDir == "" {
		return nil
	}
	return os.RemoveAll(o.tempDir)
}

// splitScheme separates an optional "scheme://" prefix from a registry URL,
// returning the host+path the resolver expects and whether plain HTTP applies.
func splitScheme(url string) (baseURL string, plainHTTP bool) {
	scheme, rest, found := strings.Cut(url, "://")
	if !found {
		return url, false
	}
	return rest, scheme == "http"
}

func (o *OCIRepository) GetComponentVersion(ctx context.Context, name, version string) (*descriptor.Descriptor, error) {
	return o.repo.GetComponentVersion(ctx, name, version)
}

func (o *OCIRepository) ResourceBytes(ctx context.Context, name, version, resourceName string) ([]byte, error) {
	readBlob, _, err := o.repo.GetLocalResource(ctx, name, version, map[string]string{"name": resourceName})
	if err != nil {
		return nil, fmt.Errorf("failed to get local resource %q: %w", resourceName, err)
	}
	var buf bytes.Buffer
	if err := blob.Copy(&buf, readBlob); err != nil {
		return nil, fmt.Errorf("failed to read resource %q: %w", resourceName, err)
	}
	return buf.Bytes(), nil
}
