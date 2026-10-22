package helmvalues

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	"ocm.software/open-component-model/bindings/go/blob"
	"ocm.software/open-component-model/bindings/go/ctf"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ocm.software/open-component-model/bindings/go/oci"
	ocictf "ocm.software/open-component-model/bindings/go/oci/ctf"
	urlresolver "ocm.software/open-component-model/bindings/go/oci/resolver/url"
)

// CTFPlaceholderHost is the base URL callers pass to GetRenderingInput for a
// component version read from a CTF. A CTF has no registry host, so
// component-local images render with this placeholder, which is deliberately
// not a valid registry host.
const CTFPlaceholderHost = "REGISTRY"

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

// CTFRepository is an OCIRepository over a CTF on the local filesystem, opened
// read-only. It also lists the component versions the CTF holds.
type CTFRepository struct {
	*OCIRepository
	archive ctf.CTF
}

// ComponentVersion identifies a component version in a CTF.
type ComponentVersion struct {
	Name    string
	Version string
}

// OpenCTF opens the CTF at path read-only. A path ending in .tar, .tgz or
// .tar.gz is an archive and is extracted into a temporary directory; any other
// path is a CTF directory. Call Close when done to remove the temporary
// directory.
func OpenCTF(ctx context.Context, path string) (_ *CTFRepository, err error) {
	tempDir, err := os.MkdirTemp("", "ocm-kit-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(tempDir)
		}
	}()

	ctfDir := path
	if format := ctf.DiscoverCTFFormatFromPath(path); format != ctf.FormatDirectory {
		ctfDir = filepath.Join(tempDir, "ctf")
		if err := extractCTF(ctx, path, format, ctfDir); err != nil {
			return nil, err
		}
	}
	archive, err := ctf.OpenCTFFromOSPath(ctfDir, ctf.O_RDONLY)
	if err != nil {
		return nil, fmt.Errorf("opening CTF %q: %w", path, err)
	}
	repo, err := oci.NewRepository(ocictf.WithCTF(ocictf.NewFromCTF(archive)), oci.WithTempDir(tempDir))
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}
	return &CTFRepository{OCIRepository: &OCIRepository{repo: repo, tempDir: tempDir}, archive: archive}, nil
}

// extractCTF extracts the CTF archive at path into dir. Every entry is anchored
// in dir through os.Root. Only regular files are written; their parent
// directories are created per file, so directory entries carry no information.
//
// Reason: ctf.ExtractTAR creates directory entries relative to the working
// directory, and ctf.OpenCTF drops its error for archives, so the SDK's own
// extraction cannot be used here.
func extractCTF(ctx context.Context, path string, format ctf.FileFormat, dir string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var r io.Reader = f
	if format == ctf.FormatTGZ {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("reading CTF archive %q: %w", path, err)
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}

	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	tr := tar.NewReader(r)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			// Reason: the tar reader stops at the end-of-archive marker, so the
			// gzip trailer with its checksum is only verified by reading on to EOF.
			if _, err := io.Copy(io.Discard, r); err != nil {
				return fmt.Errorf("reading CTF archive %q: %w", path, err)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading CTF archive %q: %w", path, err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if err := root.MkdirAll(filepath.Dir(header.Name), 0o755); err != nil {
			return fmt.Errorf("extracting CTF archive %q: %w", path, err)
		}
		out, err := root.Create(header.Name)
		if err != nil {
			return fmt.Errorf("extracting CTF archive %q: %w", path, err)
		}
		_, copyErr := io.Copy(out, tr)
		if err := errors.Join(copyErr, out.Close()); err != nil {
			return fmt.Errorf("extracting CTF archive %q: %w", path, err)
		}
	}
}

// ComponentVersions lists every component version in the CTF, ordered by
// component name and, per component, by version descending.
func (c *CTFRepository) ComponentVersions(ctx context.Context) ([]ComponentVersion, error) {
	var names []string
	err := ocictf.NewComponentLister(c.archive).ListComponents(ctx, "", func(n []string) error {
		names = n
		return nil
	})
	if err != nil {
		return nil, err
	}

	var cvs []ComponentVersion
	for _, name := range names {
		versions, err := c.repo.ListComponentVersions(ctx, name)
		if err != nil {
			return nil, err
		}
		for _, v := range versions {
			cvs = append(cvs, ComponentVersion{Name: name, Version: v})
		}
	}
	return cvs, nil
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
