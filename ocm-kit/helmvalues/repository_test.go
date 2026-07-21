package helmvalues

import (
	"bytes"
	"context"
	"os"
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

// TestOCIRepository_ResourceBytes proves ResourceBytes works end to end against
// a real *oci.Repository backed by a CTF archive. The resource version here
// differs from the component version to confirm the identity-map lookup used by
// ResourceBytes ({"name": resourceName}) does not depend on the version.
func TestOCIRepository_ResourceBytes(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	fs, err := filesystem.NewFS(t.TempDir(), os.O_RDWR)
	r.NoError(err)
	store := ocictf.NewFromCTF(ctf.NewFileSystemCTF(fs))
	inner, err := oci.NewRepository(ocictf.WithCTF(store), oci.WithTempDir(t.TempDir()))
	r.NoError(err)

	content := []byte("hello")
	res := &descriptor.Resource{
		Relation:    descriptor.LocalRelation,
		ElementMeta: descriptor.ElementMeta{ObjectMeta: descriptor.ObjectMeta{Name: "greeting", Version: "9.9.9"}},
		Type:        "plainText",
		Access:      &v2.LocalBlob{LocalReference: digest.FromBytes(content).String(), MediaType: "text/plain"},
	}
	desc := &descriptor.Descriptor{
		Meta: descriptor.Meta{Version: "v2"},
		Component: descriptor.Component{
			Provider:      descriptor.Provider{Name: "acme.org"},
			ComponentMeta: descriptor.ComponentMeta{ObjectMeta: descriptor.ObjectMeta{Name: "acme.org/app", Version: "1.0.0"}},
			Resources:     []descriptor.Resource{*res},
		},
	}
	newRes, err := inner.AddLocalResource(ctx, "acme.org/app", "1.0.0", res, inmemory.New(bytes.NewReader(content)))
	r.NoError(err)
	desc.Component.Resources[0] = *newRes
	r.NoError(inner.AddComponentVersion(ctx, desc))

	repo := &OCIRepository{repo: inner}

	got, err := repo.GetComponentVersion(ctx, "acme.org/app", "1.0.0")
	r.NoError(err)
	r.Equal("acme.org/app", got.Component.Name)

	data, err := repo.ResourceBytes(ctx, "acme.org/app", "1.0.0", "greeting")
	r.NoError(err)
	r.Equal(content, data)
}
