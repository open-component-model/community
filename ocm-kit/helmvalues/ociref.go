package helmvalues

import (
	"encoding/json"
	"fmt"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
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

// ResourceOCIReference returns the absolute OCI image reference for a resource
// whose access is an OCIImage (or a legacy OCI access alias). ok is false when
// the resource is not OCI-image backed. Relative-reference resolution is added
// in a later task.
//
// The access may arrive in two forms. When a descriptor is built in-memory the
// access is a concrete *ociaccessv1.OCIImage. When a descriptor is read back
// from a repository via GetComponentVersion the access arrives as an un-decoded
// *runtime.Raw (a typed JSON envelope), so we decode it the way the SDK's own
// access Scheme does: json.Unmarshal(raw.Data, &OCIImage{}).
func ResourceOCIReference(res descriptor.Resource) (ref string, ok bool, err error) {
	switch a := res.Access.(type) {
	case *ociaccessv1.OCIImage:
		// A concrete OCIImage is an OCI image regardless of its type-name string.
		return a.ImageReference, true, nil
	case *runtime.Raw:
		if a == nil || !isOCIImageTypeName(a.Type.Name) {
			return "", false, nil
		}
		var img ociaccessv1.OCIImage
		if err := json.Unmarshal(a.Data, &img); err != nil {
			return "", false, fmt.Errorf("failed to decode OCIImage access: %w", err)
		}
		return img.ImageReference, true, nil
	default:
		return "", false, nil
	}
}
