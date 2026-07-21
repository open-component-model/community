package helmvalues

import (
	"fmt"

	"ocm.software/open-component-model/bindings/go/oci/looseref"
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
