package helmvalues

import (
	"testing"

	"github.com/stretchr/testify/require"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	ociaccess "ocm.software/open-component-model/bindings/go/oci/spec/access"
	ociaccessv1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
	"ocm.software/open-component-model/bindings/go/runtime"
)

func TestParseOCIRef(t *testing.T) {
	ref, err := ParseOCIRef("ghcr.io/acme/app:v1.2.3")
	require.NoError(t, err)
	require.Equal(t, "ghcr.io", ref.Host)
	require.Equal(t, "acme/app", ref.Repository)
	require.Equal(t, "v1.2.3", ref.Tag)
	require.Empty(t, ref.Digest)

	require.Equal(t, "ghcr.io/acme/app:v1.2.3", ref.String())
}

func TestParseOCIRef_Digest(t *testing.T) {
	const dig = "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"
	d, err := ParseOCIRef("ghcr.io/acme/app@" + dig)
	require.NoError(t, err)
	require.Equal(t, "ghcr.io", d.Host)
	require.Equal(t, "acme/app", d.Repository)
	require.Equal(t, dig, d.Digest)
}

func TestResourceOCIReference_OCIImage(t *testing.T) {
	res := descriptor.Resource{
		Relation: descriptor.ExternalRelation,
		Access: &ociaccessv1.OCIImage{
			Type:           runtime.Type{Name: ociaccessv1.OCIImageType, Version: "v1"},
			ImageReference: "ghcr.io/acme/app:v1",
		},
	}
	ref, ok, err := ResourceOCIReference(res)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/app:v1", ref)

	// Non-OCI access returns ok=false, no error.
	_, ok, err = ResourceOCIReference(descriptor.Resource{})
	require.NoError(t, err)
	require.False(t, ok)
}

// TestResourceOCIReference_Raw exercises the read-back form. Empirically, when a
// descriptor is read back via repo.GetComponentVersion the resource's Access
// arrives as an un-decoded *runtime.Raw (verified: %T == *runtime.Raw, Type ==
// "OCIImage/v1", Data == the OCIImage JSON envelope). This test builds that Raw
// exactly the way the SDK does — by converting a concrete *OCIImage through the
// oci access Scheme into a *runtime.Raw — so it mirrors real read-back rather
// than a hand-built value.
func TestResourceOCIReference_Raw(t *testing.T) {
	raw := &runtime.Raw{}
	err := ociaccess.Scheme.Convert(&ociaccessv1.OCIImage{
		Type:           runtime.Type{Name: ociaccessv1.OCIImageType, Version: "v1"},
		ImageReference: "ghcr.io/acme/app:v1",
	}, raw)
	require.NoError(t, err)
	require.Equal(t, "OCIImage/v1", raw.Type.String())

	ref, ok, err := ResourceOCIReference(descriptor.Resource{Access: raw})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/app:v1", ref)
}

// TestResourceOCIReference_RawLegacyType covers a Raw carrying a legacy OCI
// access type name (e.g. ociArtifact), which real components may still use.
func TestResourceOCIReference_RawLegacyType(t *testing.T) {
	raw := &runtime.Raw{}
	err := ociaccess.Scheme.Convert(&ociaccessv1.OCIImage{
		Type:           runtime.Type{Name: ociaccessv1.LegacyType, Version: "v1"},
		ImageReference: "ghcr.io/acme/legacy:v2",
	}, raw)
	require.NoError(t, err)
	require.Equal(t, "ociArtifact/v1", raw.Type.String())

	ref, ok, err := ResourceOCIReference(descriptor.Resource{Access: raw})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/legacy:v2", ref)
}

// TestResourceOCIReference_RawNonOCI ensures a Raw of an unrelated access type
// returns ok=false with no error.
func TestResourceOCIReference_RawNonOCI(t *testing.T) {
	raw := &runtime.Raw{
		Type: runtime.Type{Name: "localBlob", Version: "v1"},
		Data: []byte(`{"type":"localBlob/v1"}`),
	}
	_, ok, err := ResourceOCIReference(descriptor.Resource{Access: raw})
	require.NoError(t, err)
	require.False(t, ok)
}
