package helmvalues

import (
	"bytes"
	"context"
	"fmt"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	"ocm.software/open-component-model/bindings/go/blob"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ocm.software/open-component-model/bindings/go/oci"
	urlresolver "ocm.software/open-component-model/bindings/go/oci/resolver/url"
)

// Repository is the narrow, v2-backed surface helmvalues needs from an OCM
// repository. It is public so consumers and tests can supply their own.
type Repository interface {
	// GetComponentVersion resolves a component descriptor.
	GetComponentVersion(ctx context.Context, name, version string) (*descriptor.Descriptor, error)
	// ResourceBytes downloads the local blob of the named resource.
	ResourceBytes(ctx context.Context, name, version, resourceName string) ([]byte, error)
}

// OCIRepository implements Repository over the v2 oci bindings.
type OCIRepository struct{ repo *oci.Repository }

// OpenRepository opens an OCI-registry-backed OCM repository at baseURL
// (host + optional path, e.g. "ghcr.io/acme"). plainHTTP selects http vs https.
func OpenRepository(baseURL, tempDir string, plainHTTP bool, client *auth.Client) (*OCIRepository, error) {
	if client == nil {
		client = &auth.Client{Client: retry.DefaultClient, Cache: auth.NewCache()}
	}
	resolver, err := urlresolver.New(
		urlresolver.WithBaseURL(baseURL),
		urlresolver.WithPlainHTTP(plainHTTP),
		urlresolver.WithBaseClient(client),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resolver: %w", err)
	}
	repo, err := oci.NewRepository(oci.WithResolver(resolver), oci.WithTempDir(tempDir))
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}
	return &OCIRepository{repo: repo}, nil
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
