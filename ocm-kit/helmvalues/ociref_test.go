package helmvalues

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
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
		Type: runtime.Type{Name: "s3", Version: "v1"},
		Data: []byte(`{"type":"s3/v1"}`),
	}
	_, ok, err := ResourceOCIReference(descriptor.Resource{Access: raw})
	require.NoError(t, err)
	require.False(t, ok)
}

// ociImageRaw builds the nested *runtime.Raw that a LocalBlob.GlobalAccess
// carries on the v2 wire for an OCIImage global access (Type "OCIImage/v1",
// {"imageReference": ...}) — the exact shape confirmed by the Task 7 spike.
func ociImageRaw(t *testing.T, ref string) *runtime.Raw {
	t.Helper()
	data, err := json.Marshal(&ociaccessv1.OCIImage{
		Type:           runtime.NewVersionedType(ociaccessv1.OCIImageType, "v1"),
		ImageReference: ref,
	})
	require.NoError(t, err)
	return &runtime.Raw{Type: runtime.NewVersionedType(ociaccessv1.OCIImageType, "v1"), Data: data}
}

// TestResourceOCIReference_LocalBlobGlobalAccess covers a concrete v2.LocalBlob
// whose optional GlobalAccess carries an absolute OCIImage reference. This is
// the v2 replacement for v1's relative reference: the SDK writes the absolute,
// digest-pinned reference into GlobalAccess when the backing store is global.
func TestResourceOCIReference_LocalBlobGlobalAccess(t *testing.T) {
	res := descriptor.Resource{
		Access: &v2.LocalBlob{
			Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
			LocalReference: "sha256:deadbeef",
			MediaType:      "application/octet-stream",
			GlobalAccess:   ociImageRaw(t, "ghcr.io/acme/app@sha256:deadbeef"),
		},
	}
	ref, ok, err := ResourceOCIReference(res)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/app@sha256:deadbeef", ref)
}

// TestResourceOCIReference_LocalBlobRaw covers the real read-back form: the
// spike confirmed the access arrives as *runtime.Raw with Type "LocalBlob/v1",
// and (against a global store) a nested GlobalAccess OCIImage envelope.
func TestResourceOCIReference_LocalBlobRaw(t *testing.T) {
	lb := &v2.LocalBlob{
		Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
		LocalReference: "sha256:deadbeef",
		MediaType:      "application/octet-stream",
		GlobalAccess:   ociImageRaw(t, "ghcr.io/acme/app@sha256:deadbeef"),
	}
	data, err := json.Marshal(lb)
	require.NoError(t, err)
	raw := &runtime.Raw{Type: lb.Type, Data: data}

	ref, ok, err := ResourceOCIReference(descriptor.Resource{Access: raw})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/app@sha256:deadbeef", ref)
}

// TestResourceOCIReference_LocalBlobGlobalOCIImageLayer covers the non-manifest
// path: for a blob that is not an OCI-compliant manifest, the SDK sets
// GlobalAccess to an OCIImageLayer whose absolute reference lives in "ref".
func TestResourceOCIReference_LocalBlobGlobalOCIImageLayer(t *testing.T) {
	layerData, err := json.Marshal(&ociaccessv1.OCIImageLayer{
		Type:      runtime.NewVersionedType(ociaccessv1.OCIImageLayerType, "v1"),
		Reference: "ghcr.io/acme/app@sha256:cafebabe",
		MediaType: "application/octet-stream",
	})
	require.NoError(t, err)
	res := descriptor.Resource{
		Access: &v2.LocalBlob{
			Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
			LocalReference: "sha256:cafebabe",
			GlobalAccess:   &runtime.Raw{Type: runtime.NewVersionedType(ociaccessv1.OCIImageLayerType, "v1"), Data: layerData},
		},
	}
	ref, ok, err := ResourceOCIReference(res)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/app@sha256:cafebabe", ref)
}

// TestResourceOCIReference_LocalBlobRuntimeForm covers the runtime.LocalBlob
// (descriptor.LocalBlob) form, whose GlobalAccess is a runtime.Typed rather than
// a *runtime.Raw. It must resolve identically to the v2 wire form.
func TestResourceOCIReference_LocalBlobRuntimeForm(t *testing.T) {
	res := descriptor.Resource{
		Access: &descriptor.LocalBlob{
			Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
			LocalReference: "sha256:deadbeef",
			GlobalAccess: &ociaccessv1.OCIImage{
				Type:           runtime.NewVersionedType(ociaccessv1.OCIImageType, "v1"),
				ImageReference: "ghcr.io/acme/app:v9",
			},
		},
	}
	ref, ok, err := ResourceOCIReference(res)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/app:v9", ref)
}

// TestResourceOCIReference_LocalBlobLocalOnly is the CRITICAL GAP case the spike
// found: a component-local blob in a local (CTF) store has neither GlobalAccess
// nor ReferenceName on read-back. It has no absolute OCI reference, so we return
// ok=false, err=nil (a documented limitation, not a failure) — letting callers
// enumerate resources without a spurious error.
func TestResourceOCIReference_LocalBlobLocalOnly(t *testing.T) {
	// Concrete form.
	_, ok, err := ResourceOCIReference(descriptor.Resource{
		Access: &v2.LocalBlob{
			Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
			LocalReference: "sha256:deadbeef",
			MediaType:      "application/octet-stream",
		},
	})
	require.NoError(t, err)
	require.False(t, ok)

	// Read-back Raw form, exactly as observed in the spike.
	raw := &runtime.Raw{
		Type: runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
		Data: []byte(`{"localReference":"sha256:deadbeef","mediaType":"application/octet-stream","type":"LocalBlob/v1"}`),
	}
	_, ok, err = ResourceOCIReference(descriptor.Resource{Access: raw})
	require.NoError(t, err)
	require.False(t, ok)
}

// TestResourceOCIReference_LocalBlobReferenceName covers the fallback: a
// LocalBlob with no GlobalAccess but a static ReferenceName surfaces that name.
func TestResourceOCIReference_LocalBlobReferenceName(t *testing.T) {
	res := descriptor.Resource{
		Access: &v2.LocalBlob{
			Type:          runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
			ReferenceName: "acme/app:v1",
		},
	}
	ref, ok, err := ResourceOCIReference(res)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "acme/app:v1", ref)
}
