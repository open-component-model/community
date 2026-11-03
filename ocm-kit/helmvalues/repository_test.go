package helmvalues

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"

	"ocm.software/open-component-model/bindings/go/blob/filesystem"
	"ocm.software/open-component-model/bindings/go/blob/inmemory"
	"ocm.software/open-component-model/bindings/go/ctf"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/oci"
	ocictf "ocm.software/open-component-model/bindings/go/oci/ctf"
)

func TestFakeRepository_Implements(t *testing.T) {
	var _ Repository = (*FakeRepository)(nil)

	f := &FakeRepository{
		Descriptor: &descriptor.Descriptor{
			Component: descriptor.Component{
				ComponentMeta: descriptor.ComponentMeta{ObjectMeta: descriptor.ObjectMeta{Name: "c", Version: "1.0.0"}},
			},
		},
		Blobs: map[string][]byte{"greeting": []byte("hi")},
	}
	desc, err := f.GetComponentVersion(context.Background(), "c", "1.0.0")
	require.NoError(t, err)
	require.Equal(t, "c", desc.Component.Name)

	data, err := f.ResourceBytes(context.Background(), "c", "1.0.0", "greeting")
	require.NoError(t, err)
	require.Equal(t, []byte("hi"), data)
}

// testResourceName names the template resource writeTestCTF adds to every
// component version.
const testResourceName = "values"

// writeTestCTF writes a CTF directory holding the given component versions,
// each with one yaml resource testResourceName whose content is the template
// bytes, and returns the directory. The resource version (9.9.9) differs from
// the component versions on purpose.
func writeTestCTF(t *testing.T, templates map[ComponentVersion][]byte) string {
	t.Helper()
	r := require.New(t)
	ctx := t.Context()

	dir := t.TempDir()
	fs, err := filesystem.NewFS(dir, os.O_RDWR)
	r.NoError(err)
	store := ocictf.NewFromCTF(ctf.NewFileSystemCTF(fs))
	repo, err := oci.NewRepository(ocictf.WithCTF(store), oci.WithTempDir(t.TempDir()))
	r.NoError(err)

	for cv, content := range templates {
		res := &descriptor.Resource{
			Relation:    descriptor.LocalRelation,
			ElementMeta: descriptor.ElementMeta{ObjectMeta: descriptor.ObjectMeta{Name: testResourceName, Version: "9.9.9"}},
			Type:        "yaml",
			Access:      &v2.LocalBlob{LocalReference: digest.FromBytes(content).String(), MediaType: "application/x-yaml"},
		}
		newRes, err := repo.AddLocalResource(ctx, cv.Name, cv.Version, res, inmemory.New(bytes.NewReader(content)))
		r.NoError(err)
		r.NoError(repo.AddComponentVersion(ctx, &descriptor.Descriptor{
			Meta: descriptor.Meta{Version: "v2"},
			Component: descriptor.Component{
				Provider:      descriptor.Provider{Name: "acme.org"},
				ComponentMeta: descriptor.ComponentMeta{ObjectMeta: descriptor.ObjectMeta{Name: cv.Name, Version: cv.Version}},
				Resources:     []descriptor.Resource{*newRes},
			},
		}))
	}
	return dir
}

// tarGzDir writes dir as a .tgz archive and returns its path. AddFS writes
// directory entries such as blobs/ before their files, as archives from the
// OCM v1 CLI do.
func tarGzDir(t *testing.T, dir string) string {
	t.Helper()
	r := require.New(t)

	path := filepath.Join(t.TempDir(), "ctf.tgz")
	f, err := os.Create(path)
	r.NoError(err)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	r.NoError(tw.AddFS(os.DirFS(dir)))
	r.NoError(tw.Close())
	r.NoError(gz.Close())
	r.NoError(f.Close())
	return path
}

// TestOCIRepository_ResourceBytes proves ResourceBytes works end to end against
// a real *oci.Repository backed by a CTF archive. The resource version differs
// from the component version to confirm the identity-map lookup used by
// ResourceBytes ({"name": resourceName}) does not depend on the version.
func TestOCIRepository_ResourceBytes(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	content := []byte("hello")
	dir := writeTestCTF(t, map[ComponentVersion][]byte{{Name: "acme.org/app", Version: "1.0.0"}: content})
	archive, err := ctf.OpenCTFFromOSPath(dir, ctf.O_RDONLY)
	r.NoError(err)
	inner, err := oci.NewRepository(ocictf.WithCTF(ocictf.NewFromCTF(archive)), oci.WithTempDir(t.TempDir()))
	r.NoError(err)

	repo := &OCIRepository{repo: inner}

	got, err := repo.GetComponentVersion(ctx, "acme.org/app", "1.0.0")
	r.NoError(err)
	r.Equal("acme.org/app", got.Component.Name)

	data, err := repo.ResourceBytes(ctx, "acme.org/app", "1.0.0", testResourceName)
	r.NoError(err)
	r.Equal(content, data)
}

// TestOpenCTF opens a directory CTF and the same CTF as .tgz, lists its
// component versions, reads one template and checks that Close removes the
// temp dir and that extraction never touches the working directory.
func TestOpenCTF(t *testing.T) {
	var _ Repository = (*CTFRepository)(nil)

	app100 := ComponentVersion{Name: "acme.org/app", Version: "1.0.0"}
	app110 := ComponentVersion{Name: "acme.org/app", Version: "1.1.0"}
	other := ComponentVersion{Name: "acme.org/other", Version: "2.0.0"}
	dir := writeTestCTF(t, map[ComponentVersion][]byte{
		app100: []byte("replicas: 1"),
		app110: []byte("replicas: 2"),
		other:  []byte("replicas: 3"),
	})

	cases := []struct {
		name string
		path string
	}{
		{"directory", dir},
		{"tgz", tarGzDir(t, dir)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			ctx := t.Context()

			repo, err := OpenCTF(ctx, tc.path)
			r.NoError(err)
			r.NoDirExists("blobs")

			versions, err := repo.ComponentVersions(ctx)
			r.NoError(err)
			r.Equal([]ComponentVersion{app110, app100, other}, versions)

			desc, err := repo.GetComponentVersion(ctx, app110.Name, app110.Version)
			r.NoError(err)
			data, err := repo.ResourceBytes(ctx, desc.Component.Name, desc.Component.Version, testResourceName)
			r.NoError(err)
			r.Equal([]byte("replicas: 2"), data)

			r.NoError(repo.Close())
			r.NoDirExists(repo.tempDir)
		})
	}
}

func TestSplitScheme(t *testing.T) {
	cases := []struct {
		url       string
		wantBase  string
		wantPlain bool
	}{
		{"http://localhost:5000/my-components", "localhost:5000/my-components", true},
		{"https://ghcr.io/acme", "ghcr.io/acme", false},
		{"oci://ghcr.io/acme", "ghcr.io/acme", false},
		{"ghcr.io/acme", "ghcr.io/acme", false},
	}
	for _, tc := range cases {
		base, plain := splitScheme(tc.url)
		require.Equalf(t, tc.wantBase, base, "base for %q", tc.url)
		require.Equalf(t, tc.wantPlain, plain, "plainHTTP for %q", tc.url)
	}
}

func TestOCIRepository_CloseWithoutTempDir(t *testing.T) {
	// A repository with no owned temp dir (e.g. constructed directly) Closes cleanly.
	require.NoError(t, (&OCIRepository{}).Close())
}

func TestOpenRepository_CreatesAndRemovesTempDir(t *testing.T) {
	repo, err := OpenRepository("https://ghcr.io/acme")
	require.NoError(t, err)
	require.NotEmpty(t, repo.tempDir)
	require.DirExists(t, repo.tempDir)

	require.NoError(t, repo.Close())
	require.NoDirExists(t, repo.tempDir)
}
