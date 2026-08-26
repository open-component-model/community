package helmvalues

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
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
	ref, ok, err := ResourceOCIReference(res, "127.0.0.1:5000/my-components")
	require.NoError(t, err)
	require.True(t, ok)
	// Absolute OCIImage: base URL is ignored, ref returned unchanged.
	require.Equal(t, "ghcr.io/acme/app:v1", ref)

	// Non-OCI access returns ok=false, no error.
	_, ok, err = ResourceOCIReference(descriptor.Resource{}, "127.0.0.1:5000/my-components")
	require.NoError(t, err)
	require.False(t, ok)
}

// TestResourceOCIReference_Raw exercises the read-back form: when a descriptor
// is read back via repo.GetComponentVersion, a resource's Access arrives as an
// un-decoded *runtime.Raw (Type "OCIImage/v1", Data the OCIImage JSON envelope).
// The Raw here is built the way the SDK does — converting a concrete *OCIImage
// through the oci access Scheme — so it mirrors real read-back.
func TestResourceOCIReference_Raw(t *testing.T) {
	raw := &runtime.Raw{}
	err := ociaccess.Scheme.Convert(&ociaccessv1.OCIImage{
		Type:           runtime.Type{Name: ociaccessv1.OCIImageType, Version: "v1"},
		ImageReference: "ghcr.io/acme/app:v1",
	}, raw)
	require.NoError(t, err)
	require.Equal(t, "OCIImage/v1", raw.Type.String())

	ref, ok, err := ResourceOCIReference(descriptor.Resource{Access: raw}, "127.0.0.1:5000/my-components")
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

	ref, ok, err := ResourceOCIReference(descriptor.Resource{Access: raw}, "127.0.0.1:5000/my-components")
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
	_, ok, err := ResourceOCIReference(descriptor.Resource{Access: raw}, "127.0.0.1:5000/my-components")
	require.NoError(t, err)
	require.False(t, ok)
}

// ociImageRaw builds the nested *runtime.Raw that a LocalBlob.GlobalAccess
// carries for an OCIImage global access (Type "OCIImage/v1", {"imageReference": ...}).
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
// whose optional GlobalAccess carries an absolute OCIImage reference. When the
// backing store is globally reachable the SDK writes the absolute, digest-pinned
// reference into GlobalAccess, and it is returned as-is.
func TestResourceOCIReference_LocalBlobGlobalAccess(t *testing.T) {
	res := descriptor.Resource{
		Access: &v2.LocalBlob{
			Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
			LocalReference: "sha256:deadbeef",
			MediaType:      "application/octet-stream",
			GlobalAccess:   ociImageRaw(t, "ghcr.io/acme/app@sha256:deadbeef"),
		},
	}
	// GlobalAccess already yields an absolute reference (has a host): the base
	// URL must NOT be prefixed.
	ref, ok, err := ResourceOCIReference(res, "127.0.0.1:5000/my-components")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/app@sha256:deadbeef", ref)
}

// TestResourceOCIReference_LocalBlobRaw covers the read-back form: the access
// arrives as *runtime.Raw with Type "LocalBlob/v1" carrying a nested GlobalAccess
// OCIImage envelope.
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

	ref, ok, err := ResourceOCIReference(descriptor.Resource{Access: raw}, "127.0.0.1:5000/my-components")
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
	ref, ok, err := ResourceOCIReference(res, "127.0.0.1:5000/my-components")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/app@sha256:cafebabe", ref)
}

// TestResourceOCIReference_LocalBlobRuntimeForm covers the runtime.LocalBlob
// (descriptor.LocalBlob) form, whose GlobalAccess is a runtime.Typed rather than
// a *runtime.Raw. It must resolve identically to the raw form.
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
	ref, ok, err := ResourceOCIReference(res, "127.0.0.1:5000/my-components")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/app:v9", ref)
}

// TestResourceOCIReference_LocalBlobLocalOnly covers a component-local blob with
// neither GlobalAccess nor ReferenceName. It has no absolute OCI reference, so
// ResourceOCIReference returns ok=false, err=nil — letting callers enumerate
// resources without a spurious error.
func TestResourceOCIReference_LocalBlobLocalOnly(t *testing.T) {
	// Concrete form. A non-empty base URL must not conjure a reference: with
	// neither GlobalAccess nor ReferenceName there is no repository path to build.
	_, ok, err := ResourceOCIReference(descriptor.Resource{
		Access: &v2.LocalBlob{
			Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
			LocalReference: "sha256:deadbeef",
			MediaType:      "application/octet-stream",
		},
	}, "127.0.0.1:5000/my-components")
	require.NoError(t, err)
	require.False(t, ok)

	// Read-back Raw form.
	raw := &runtime.Raw{
		Type: runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
		Data: []byte(`{"localReference":"sha256:deadbeef","mediaType":"application/octet-stream","type":"LocalBlob/v1"}`),
	}
	_, ok, err = ResourceOCIReference(descriptor.Resource{Access: raw}, "127.0.0.1:5000/my-components")
	require.NoError(t, err)
	require.False(t, ok)
}

// TestResourceOCIReference_LocalBlobReferenceName covers the relative-access
// case: a LocalBlob with no GlobalAccess but a host-less ReferenceName. The
// ReferenceName is a bare repository path (no registry host), so it is
// reconstructed into a FULL absolute reference by prefixing the repository base
// URL derived from the component-version reference.
func TestResourceOCIReference_LocalBlobReferenceName(t *testing.T) {
	res := descriptor.Resource{
		Access: &v2.LocalBlob{
			Type:          runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
			ReferenceName: "opendefensecloud/arc-apiserver:v0.2.0",
		},
	}
	ref, ok, err := ResourceOCIReference(res, "127.0.0.1:5000/my-components")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "127.0.0.1:5000/my-components/opendefensecloud/arc-apiserver:v0.2.0", ref)
}

// TestResourceOCIReference_LocalBlobReferenceNameAlreadyAbsolute ensures a
// ReferenceName that already carries a registry host is NOT double-prefixed.
func TestResourceOCIReference_LocalBlobReferenceNameAlreadyAbsolute(t *testing.T) {
	res := descriptor.Resource{
		Access: &v2.LocalBlob{
			Type:          runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
			ReferenceName: "ghcr.io/acme/app:v1",
		},
	}
	ref, ok, err := ResourceOCIReference(res, "127.0.0.1:5000/my-components")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/app:v1", ref)
}

// TestLocalBlobv2OCIReference ensures that LocalBlobv2OCIReference returns
// references like "oci://<repoBaseUrl>/component-descriptors/<componentName>:<resourceVersion>@<localReference>"
// only for LocalBlobs, which have the mediaType application/vnd.oci.image.manifest.v1+json
// and resource type "ociArtifact" or "ociImage". (See https://ocm.software/docs/tutorials/working-with-oci/)
func TestLocalBlobv2OCIReference(t *testing.T) {
	data, err := json.Marshal(v2.LocalBlob{
		Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
		LocalReference: "sha256:83f4282d88d92266a8517330477ac6cc8e1cf7724d569087bab5367df3b083d6",
		MediaType:      OCIImageMediaType,
	})
	require.NoError(t, err)

	successfulCall := func(t assert.TestingT, err error, ok bool, reference string) {
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "oci://localhost:1234/components/component-descriptors/example.org/component:7@sha256:83f4282d88d92266a8517330477ac6cc8e1cf7724d569087bab5367df3b083d6", reference)
	}

	noErrorInvalidType := func(t assert.TestingT, err error, ok bool, reference string) {
		assert.NoError(t, err)
		assert.False(t, ok)
	}

	tests := []struct {
		Name         string
		Resource     descriptor.Resource
		CheckResults func(t assert.TestingT, err error, ok bool, reference string) // required
	}{
		{
			Name: "v2.LocalBlob access with supported mediaType and type",
			Resource: descriptor.Resource{
				Access: &v2.LocalBlob{
					Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
					LocalReference: "sha256:83f4282d88d92266a8517330477ac6cc8e1cf7724d569087bab5367df3b083d6",
					MediaType:      OCIImageMediaType,
				},
				Type: "ociImage",
				ElementMeta: descriptor.ElementMeta{
					ObjectMeta: descriptor.ObjectMeta{
						Version: "7",
					},
				},
			},
			CheckResults: successfulCall,
		},
		{
			Name: "descriptor.LocalBlob access with supported mediaType and type",
			Resource: descriptor.Resource{
				Access: &descriptor.LocalBlob{
					Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
					LocalReference: "sha256:83f4282d88d92266a8517330477ac6cc8e1cf7724d569087bab5367df3b083d6",
					MediaType:      OCIImageMediaType,
				},
				Type: "ociImage",
				ElementMeta: descriptor.ElementMeta{
					ObjectMeta: descriptor.ObjectMeta{
						Version: "7",
					},
				},
			},
			CheckResults: successfulCall,
		},
		{
			Name: "runtime.Raw access with supported mediaType and type",
			Resource: descriptor.Resource{
				Access: &runtime.Raw{
					Data: data,
					Type: runtime.Type{
						Name:    v2.LocalBlobAccessType,
						Version: v2.LocalBlobAccessTypeVersion,
					},
				},
				Type: "ociArtifact",
				ElementMeta: descriptor.ElementMeta{
					ObjectMeta: descriptor.ObjectMeta{
						Version: "7",
					},
				},
			},
			CheckResults: successfulCall,
		},
		{
			Name: "v2.LocalBlob access with unsupported mediaType and supported type",
			Resource: descriptor.Resource{
				Access: &v2.LocalBlob{
					Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
					LocalReference: "sha256:83f4282d88d92266a8517330477ac6cc8e1cf7724d569087bab5367df3b083d6",
					MediaType:      "application/text",
				},
				Type: "ociImage",
				ElementMeta: descriptor.ElementMeta{
					ObjectMeta: descriptor.ObjectMeta{
						Version: "7",
					},
				},
			},
			CheckResults: noErrorInvalidType,
		},
		{
			Name: "v2.LocalBlob access with supported mediaType and unsupported type",
			Resource: descriptor.Resource{
				Access: &v2.LocalBlob{
					Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
					LocalReference: "sha256:83f4282d88d92266a8517330477ac6cc8e1cf7724d569087bab5367df3b083d6",
					MediaType:      OCIImageMediaType,
				},
				Type: "picture",
				ElementMeta: descriptor.ElementMeta{
					ObjectMeta: descriptor.ObjectMeta{
						Version: "7",
					},
				},
			},
			CheckResults: noErrorInvalidType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			ref, ok, err := LocalBlobv2OCIReference(tt.Resource, "oci://localhost:1234/components", "example.org/component")
			tt.CheckResults(t, err, ok, ref)
		})
	}
}
