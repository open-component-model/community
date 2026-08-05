package helmvalues

import (
	"encoding/json"
	"fmt"
	"strings"

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

// ParseOCIRef parses an OCI image reference into its parts using the OCM SDK's
// loose reference parser.
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

// relativeOCIReferenceType is the access type name for an artifact stored in the
// same OCI registry as the component version referencing it.
const relativeOCIReferenceType = "relativeOciReference"

// isRelativeOCIReferenceTypeName reports whether a runtime type name identifies
// the registry-relative OCI access.
func isRelativeOCIReferenceTypeName(name string) bool {
	return name == relativeOCIReferenceType
}

// relativeOCIReferenceAccess mirrors the relativeOciReference access spec
// payload. Reference is an OCI repository name plus version, and carries no
// registry host.
type relativeOCIReferenceAccess struct {
	Reference string `json:"reference"`
}

// ResourceOCIReference returns the absolute OCI image reference for a resource
// backed by OCI content. ok is false when the resource has no resolvable OCI
// reference: a non-OCI access, or a component-local blob that exists only by
// digest with no repository path to build from.
//
// A resource's access is one of three forms:
//
//   - OCIImage, which carries an absolute image reference directly.
//   - LocalBlob, for component-local content. It resolves via its optional
//     GlobalAccess (an absolute OCIImage or OCIImageLayer reference) or, failing
//     that, its ReferenceName — a path relative to the repository base URL.
//   - relativeOciReference, for an artifact in the same registry as the
//     component version. Its Reference already includes the namespace, so it is
//     relative to the registry HOST rather than the repository base URL. This is
//     the form `ocm transfer` produces, so it is what mirrored components carry.
//
// The access may be a concrete typed value or, when read back from a repository,
// an un-decoded *runtime.Raw; both forms are handled.
//
// repoBaseURL is the "<host>/<namespace>" the repository was opened with (e.g.
// "127.0.0.1:5000/my-components"), and supplies whichever part of the prefix the
// access form needs: the whole base URL for a LocalBlob ReferenceName, only the
// registry host for a relativeOciReference.
func ResourceOCIReference(res descriptor.Resource, repoBaseURL string) (ref string, ok bool, err error) {
	switch a := res.Access.(type) {
	case *ociaccessv1.OCIImage:
		return a.ImageReference, true, nil
	case *v2.LocalBlob:
		return localBlobReference(a, repoBaseURL)
	case *descriptor.LocalBlob:
		// GlobalAccess is a runtime.Typed here; normalize it into the raw
		// envelope form so a single LocalBlob code path handles both.
		return localBlobReference(&v2.LocalBlob{
			Type:           a.Type,
			LocalReference: a.LocalReference,
			MediaType:      a.MediaType,
			ReferenceName:  a.ReferenceName,
			GlobalAccess:   typedToRaw(a.GlobalAccess),
		}, repoBaseURL)
	case *runtime.Raw:
		if a == nil {
			return "", false, nil
		}
		switch {
		case isOCIImageTypeName(a.Name):
			var img ociaccessv1.OCIImage
			if err := json.Unmarshal(a.Data, &img); err != nil {
				return "", false, fmt.Errorf("failed to decode OCIImage access: %w", err)
			}
			return img.ImageReference, true, nil
		case isLocalBlobTypeName(a.Name):
			var lb v2.LocalBlob
			if err := json.Unmarshal(a.Data, &lb); err != nil {
				return "", false, fmt.Errorf("failed to decode LocalBlob access: %w", err)
			}
			return localBlobReference(&lb, repoBaseURL)
		case isRelativeOCIReferenceTypeName(a.Name):
			var rel relativeOCIReferenceAccess
			if err := json.Unmarshal(a.Data, &rel); err != nil {
				return "", false, fmt.Errorf("failed to decode relativeOciReference access: %w", err)
			}
			return registryRelativeReference(rel.Reference, repoBaseURL)
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
// a static ReferenceName is used. A ReferenceName is a repository-relative path
// with no registry host, so it is reconstructed into a full absolute reference
// by prefixing repoBaseURL (the repository the CLI opened). A LocalBlob with
// neither GlobalAccess nor ReferenceName is local-only and has no absolute
// reference, so ok=false, err=nil is returned.
func localBlobReference(lb *v2.LocalBlob, repoBaseURL string) (ref string, ok bool, err error) {
	if lb == nil {
		return "", false, nil
	}
	if lb.GlobalAccess != nil {
		switch {
		case isOCIImageTypeName(lb.GlobalAccess.Name):
			var img ociaccessv1.OCIImage
			if err := json.Unmarshal(lb.GlobalAccess.Data, &img); err != nil {
				return "", false, fmt.Errorf("failed to decode LocalBlob global OCIImage access: %w", err)
			}
			if img.ImageReference != "" {
				// GlobalAccess already carries an absolute (host-qualified)
				// reference; use it as-is (absoluteReference is a no-op here).
				return absoluteReference(img.ImageReference, repoBaseURL), true, nil
			}
		case isOCIImageLayerTypeName(lb.GlobalAccess.Name):
			var layer ociaccessv1.OCIImageLayer
			if err := json.Unmarshal(lb.GlobalAccess.Data, &layer); err != nil {
				return "", false, fmt.Errorf("failed to decode LocalBlob global OCIImageLayer access: %w", err)
			}
			if layer.Reference != "" {
				return absoluteReference(layer.Reference, repoBaseURL), true, nil
			}
		}
	}
	// ReferenceName is a static, repository-relative OCI name (optionally :tag)
	// the blob is exposed under. It has no registry host, so reconstruct the full
	// reference from the repository base URL the CLI already holds.
	if lb.ReferenceName != "" {
		return absoluteReference(lb.ReferenceName, repoBaseURL), true, nil
	}
	// Local-only blob: no absolute reference exists in the descriptor.
	return "", false, nil
}

// absoluteReference reconstructs a FULL, host-qualified OCI reference from a
// candidate reference and the repository base URL. If candidate already carries
// a registry host it is returned unchanged (avoiding double-prefixing);
// otherwise it is a host-less repository path and repoBaseURL is prefixed:
//
//	absolute = <repoBaseURL> + "/" + <host-less candidate>
//
// Host detection uses the SDK's looseref parser (matching the rest of the code)
// combined with the standard OCI/Docker domain heuristic. The looseref parser
// splits on the first "/" and always calls the leading segment the "Registry",
// even for a plain namespace path like "opendefensecloud/arc-apiserver" — so an
// empty parsed Registry is not sufficient. A leading segment is only a real
// registry host when it looks like a domain: it contains a "." or a ":" (port),
// or equals "localhost". If repoBaseURL is empty or the candidate cannot be
// parsed, candidate is returned unchanged.
func absoluteReference(candidate, repoBaseURL string) string {
	if repoBaseURL == "" {
		return candidate
	}
	lr, err := looseref.ParseReference(candidate)
	if err != nil {
		return candidate
	}
	if isRegistryHost(lr.Registry) {
		// Already host-qualified: leave as-is.
		return candidate
	}
	return repoBaseURL + "/" + candidate
}

// registryRelativeReference reconstructs a FULL, host-qualified OCI reference
// from a relativeOciReference.
//
// Unlike absoluteReference this applies no isRegistryHost heuristic. The access
// type is host-less by definition and the heuristic would misfire on the common case of
// a namespace whose first segment contains a dot
func registryRelativeReference(reference, repoBaseURL string) (ref string, ok bool, err error) {
	if reference == "" {
		return "", false, nil
	}
	host := registryHostOf(repoBaseURL)
	if host == "" {
		return reference, true, nil
	}

	return host + "/" + reference, true, nil
}

// registryHostOf extracts the registry host from a repository base URL, which
// may carry a scheme and a namespace path (e.g. "https://127.0.0.1:5000/ns/sub"
// yields "127.0.0.1:5000"). It returns "" for an empty input.
func registryHostOf(repoBaseURL string) string {
	rest := repoBaseURL
	if _, after, found := strings.Cut(rest, "://"); found {
		rest = after
	}
	host, _, _ := strings.Cut(rest, "/")

	return host
}

// isRegistryHost reports whether a leading reference segment is a registry host
// rather than a repository namespace segment, using the standard OCI/Docker
// heuristic: a host contains a "." or a ":" (port), or is exactly "localhost".
func isRegistryHost(segment string) bool {
	if segment == "" {
		return false
	}
	if segment == "localhost" {
		return true
	}
	return strings.ContainsAny(segment, ".:")
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
// form) into the *runtime.Raw envelope. It returns nil for a
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
