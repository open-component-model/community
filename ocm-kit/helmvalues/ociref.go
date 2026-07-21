package helmvalues

import (
	"encoding/json"
	"fmt"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/oci/looseref"
	ociaccessv1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
	"ocm.software/open-component-model/bindings/go/runtime"
)

// ImageReference is the template-facing representation of an OCI image
// reference. Field shape is a stable public contract.
type ImageReference struct {
	Host       string
	Repository string
	Tag        string
	Digest     string
}

func (r ImageReference) String() string {
	s := ""
	if r.Host != "" {
		s += r.Host + "/"
	}
	s += r.Repository
	if r.Tag != "" {
		s += ":" + r.Tag
	}
	if r.Digest != "" {
		s += "@" + r.Digest
	}
	return s
}

// ParseOCIRef parses an OCI image reference into its parts using the v2 SDK's
// loose (OCM-extended) parser.
func ParseOCIRef(imageRef string) (ImageReference, error) {
	lr, err := looseref.ParseReference(imageRef)
	if err != nil {
		return ImageReference{}, fmt.Errorf("invalid OCI reference %q: %w", imageRef, err)
	}

	// The embedded oras.Reference.Reference field holds the digest when the ref
	// is digest-bearing, but it also mirrors the tag for tag-only references.
	// Only surface it as a digest when it validates as one.
	digest := ""
	if lr.ValidateReferenceAsDigest() == nil {
		digest = lr.Reference.Reference
	}

	return ImageReference{
		Host:       lr.Registry,
		Repository: lr.Repository,
		Tag:        lr.Tag,
		Digest:     digest,
	}, nil
}

// isOCIImageTypeName reports whether a runtime type name identifies the OCI
// image access type, including its legacy aliases. Version is not considered:
// only the primary and legacy access type names matter here.
func isOCIImageTypeName(name string) bool {
	switch name {
	case ociaccessv1.OCIImageType,
		ociaccessv1.LegacyType,  // ociArtifact
		ociaccessv1.LegacyType2, // ociRegistry
		ociaccessv1.LegacyType3: // ociImage
		return true
	default:
		return false
	}
}

// isLocalBlobTypeName reports whether a runtime type name identifies the
// component-local LocalBlob access, including its legacy alias.
func isLocalBlobTypeName(name string) bool {
	switch name {
	case v2.LocalBlobAccessType, // LocalBlob
		v2.LegacyLocalBlobAccessType: // localBlob
		return true
	default:
		return false
	}
}

// SPIKE FINDINGS (Task 7 — empirically confirmed against a real *oci.Repository
// backed by a CTF archive, mirroring helmvalues/repository_test.go):
//
//   - A component-local resource's access is stored as a v2.LocalBlob. On
//     read-back via GetComponentVersion the access arrives — exactly like the
//     OCIImage case in Task 5 — as an un-decoded *runtime.Raw with
//     Type == "LocalBlob/v1" and Data == the LocalBlob JSON envelope. It is
//     never a concrete *v2.LocalBlob / *descriptor.LocalBlob on read-back.
//
//   - v1 had a `relativeOciReference` access resolved against the component's
//     own repo via GetOCIReference(compVer). v2 has NO such type. The v2 model
//     is: a LocalBlob MAY carry an optional GlobalAccess (an absolute access)
//     and/or a ReferenceName. When present, GlobalAccess is itself a nested
//     typed envelope: on the v2 wire type it is a *runtime.Raw with
//     Type "OCIImage/v1" ({"imageReference": "<base>@<digest>"}) or
//     "OCIImageLayer/v1" ({"ref": "<base>@<digest>"}). The SDK populates it in
//     oci/internal/pack.setGlobalAccess only when the backing store is globally
//     reachable (a remote registry) under GlobalAccessPolicyAuto. Its own
//     resolution idiom (oci.Repository.downloadStream / ProcessResourceDigest)
//     is: read LocalBlob.GlobalAccess, materialize it via the access Scheme,
//     and if it is an OCIImage use .ImageReference.
//
//   - CRITICAL GAP: for a purely local store (CTF/OCI layout) GlobalAccess and
//     ReferenceName are NOT populated on read-back — there simply is no absolute
//     OCI reference for the blob, and the descriptor alone cannot yield one
//     (resolving would need repository/base-URL context not present in this
//     function's signature). This is a legitimate state, not an error: such a
//     resource is local-only. We therefore return ok=false, err=nil for it, so
//     callers can enumerate resources without a spurious failure. We resolve a
//     LocalBlob to an absolute reference ONLY when its GlobalAccess (or, as a
//     documented static hint, ReferenceName) provides one.
//
// ResourceOCIReference returns the absolute OCI image reference for a resource
// backed by OCI content. ok is false when the resource has no resolvable
// absolute OCI reference (non-OCI access, or a local-only LocalBlob without a
// global access). The access may arrive as a concrete typed value (in-memory
// descriptors) or as an un-decoded *runtime.Raw (repository read-back); both
// forms are handled the way the SDK's own access Scheme decodes them.
func ResourceOCIReference(res descriptor.Resource) (ref string, ok bool, err error) {
	switch a := res.Access.(type) {
	case *ociaccessv1.OCIImage:
		// A concrete OCIImage is an OCI image regardless of its type-name string.
		return a.ImageReference, true, nil
	case *v2.LocalBlob:
		return localBlobReference(a)
	case *descriptor.LocalBlob:
		// Runtime form: GlobalAccess is a runtime.Typed. Normalize into the v2
		// wire form (GlobalAccess as *runtime.Raw) and reuse one code path.
		return localBlobReference(&v2.LocalBlob{
			Type:           a.Type,
			LocalReference: a.LocalReference,
			MediaType:      a.MediaType,
			ReferenceName:  a.ReferenceName,
			GlobalAccess:   typedToRaw(a.GlobalAccess),
		})
	case *runtime.Raw:
		if a == nil {
			return "", false, nil
		}
		switch {
		case isOCIImageTypeName(a.Type.Name):
			var img ociaccessv1.OCIImage
			if err := json.Unmarshal(a.Data, &img); err != nil {
				return "", false, fmt.Errorf("failed to decode OCIImage access: %w", err)
			}
			return img.ImageReference, true, nil
		case isLocalBlobTypeName(a.Type.Name):
			var lb v2.LocalBlob
			if err := json.Unmarshal(a.Data, &lb); err != nil {
				return "", false, fmt.Errorf("failed to decode LocalBlob access: %w", err)
			}
			return localBlobReference(&lb)
		default:
			return "", false, nil
		}
	default:
		return "", false, nil
	}
}

// localBlobReference resolves a component-local LocalBlob to an absolute OCI
// reference. Resolution comes from the blob's optional GlobalAccess (an OCIImage
// or OCIImageLayer carrying an absolute, digest-pinned reference); failing that,
// a static ReferenceName is used as a last resort. A LocalBlob with neither is
// local-only and has no absolute reference, so ok=false, err=nil is returned.
func localBlobReference(lb *v2.LocalBlob) (ref string, ok bool, err error) {
	if lb == nil {
		return "", false, nil
	}
	if lb.GlobalAccess != nil {
		switch {
		case isOCIImageTypeName(lb.GlobalAccess.Type.Name):
			var img ociaccessv1.OCIImage
			if err := json.Unmarshal(lb.GlobalAccess.Data, &img); err != nil {
				return "", false, fmt.Errorf("failed to decode LocalBlob global OCIImage access: %w", err)
			}
			if img.ImageReference != "" {
				return img.ImageReference, true, nil
			}
		case isOCIImageLayerTypeName(lb.GlobalAccess.Type.Name):
			var layer ociaccessv1.OCIImageLayer
			if err := json.Unmarshal(lb.GlobalAccess.Data, &layer); err != nil {
				return "", false, fmt.Errorf("failed to decode LocalBlob global OCIImageLayer access: %w", err)
			}
			if layer.Reference != "" {
				return layer.Reference, true, nil
			}
		}
	}
	// ReferenceName is a static OCI repository name (optionally :tag) the blob is
	// meant to be exposed under. It is only a hint (no host), so surface it only
	// when there is no better global access.
	if lb.ReferenceName != "" {
		return lb.ReferenceName, true, nil
	}
	// Local-only blob: no absolute reference exists in the descriptor.
	return "", false, nil
}

// isOCIImageLayerTypeName reports whether a runtime type name identifies the
// OCIImageLayer access type, including its legacy alias.
func isOCIImageLayerTypeName(name string) bool {
	switch name {
	case ociaccessv1.OCIImageLayerType, // OCIImageLayer
		ociaccessv1.LegacyOCIBlobAccessType: // ociBlob
		return true
	default:
		return false
	}
}

// typedToRaw normalizes a runtime.Typed global access (the runtime.LocalBlob
// form) into the *runtime.Raw envelope used on the v2 wire. It returns nil for a
// nil input or on marshal failure (treated as "no resolvable global access").
func typedToRaw(t runtime.Typed) *runtime.Raw {
	if t == nil {
		return nil
	}
	if raw, ok := t.(*runtime.Raw); ok {
		return raw
	}
	data, err := json.Marshal(t)
	if err != nil {
		return nil
	}
	return &runtime.Raw{Type: t.GetType(), Data: data}
}
