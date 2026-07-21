package helmvalues

import (
	"bytes"
	"os"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"

	"ocm.software/open-component-model/bindings/go/blob"
	"ocm.software/open-component-model/bindings/go/blob/filesystem"
	"ocm.software/open-component-model/bindings/go/blob/inmemory"
	"ocm.software/open-component-model/bindings/go/ctf"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/oci"
	ocictf "ocm.software/open-component-model/bindings/go/oci/ctf"
)

func TestSkeleton_CTFRoundTrip(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	fs, err := filesystem.NewFS(t.TempDir(), os.O_RDWR)
	r.NoError(err)
	store := ocictf.NewFromCTF(ctf.NewFileSystemCTF(fs))
	repo, err := oci.NewRepository(ocictf.WithCTF(store), oci.WithTempDir(t.TempDir()))
	r.NoError(err)

	content := []byte("hello")
	res := &descriptor.Resource{
		Relation:    descriptor.LocalRelation,
		ElementMeta: descriptor.ElementMeta{ObjectMeta: descriptor.ObjectMeta{Name: "greeting", Version: "1.0.0"}},
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
	newRes, err := repo.AddLocalResource(ctx, "acme.org/app", "1.0.0", res, inmemory.New(bytes.NewReader(content)))
	r.NoError(err)
	desc.Component.Resources[0] = *newRes
	r.NoError(repo.AddComponentVersion(ctx, desc))

	got, err := repo.GetComponentVersion(ctx, "acme.org/app", "1.0.0")
	r.NoError(err)
	r.Equal("acme.org/app", got.Component.Name)

	readBlob, _, err := repo.GetLocalResource(ctx, "acme.org/app", "1.0.0", map[string]string{"name": "greeting", "version": "1.0.0"})
	r.NoError(err)
	var buf bytes.Buffer
	r.NoError(blob.Copy(&buf, readBlob))
	r.Equal(content, buf.Bytes())
}
