package helmvalues

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/oci/looseref"
	ociaccessv1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
	"ocm.software/open-component-model/bindings/go/runtime"
)

const OCIImageMediaType = "application/vnd.oci.image.manifest.v1+json"

var schemePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)

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

// LocalBlobv2OCIReference implements the special case:
// - Resource of type "ociArtifact" or "ociImage"
// - Access of type "LocalBlob/v1":
//   - with mediaType "application/vnd.oci.image.manifest.v1+json"
//
// Such a resource is created, when a local OCI Image is referenced as an input
// using OCMv2. OCMv2 still allows to access such resources within OCI registries
// over the reference component-descriptors/<componentName>@<localReference)
// as described here https://ocm.software/docs/tutorials/working-with-oci/. If possible
// this function generates such an OCI reference.
//
// Note: This access method works even if the image resource have a Reference (the
// case if it originated from an external OCI registry in the first place) or if
// GlobalAccess is configured. Thus LocalBlobv2OCIReference will return the reference
// as described above in such cases. To avoid this try first to dereference using
// ResourceOCIReference before calling this function.
//
// Because it does not have an explicit
// host configured, repoBaseURL is the "<host>/<namespace>" the repository was opened with (e.g.
// "127.0.0.1:5000/my-components").
//
// The returned ref(erence) is only valid, if ok is true and no error occurred.
// Ok is false, if all operations work as expected, but the given (res)ource does not
// fullfill above requirements.
func LocalBlobv2OCIReference(res descriptor.Resource, repoBaseURL string, componentName string) (ref string, ok bool, err error) {
	var lb *v2.LocalBlob
	switch a := res.Access.(type) {
	case *v2.LocalBlob:
		lb = a
	case *descriptor.LocalBlob:
		lb = &v2.LocalBlob{
			Type:           a.Type,
			LocalReference: a.LocalReference,
			MediaType:      a.MediaType,
			ReferenceName:  a.ReferenceName,
			GlobalAccess:   typedToRaw(a.GlobalAccess),
		}
	case *runtime.Raw:
		if a == nil {
			return "", false, nil
		}
		switch {
		case isLocalBlobTypeName(a.Name):
			if err := json.Unmarshal(a.Data, &lb); err != nil {
				return "", false, fmt.Errorf("failed to decode LocalBlob access: %w", err)
			}
		}
	}

	if (lb != nil) && (res.Type == "ociArtifact" || res.Type == "ociImage") && (lb.MediaType == OCIImageMediaType) {
		ref, err := joinRef(repoBaseURL, "component-descriptors", componentName)
		if err != nil {
			return "", false, fmt.Errorf("join reference: %w", err)
		}

		return fmt.Sprintf("%s:%s@%s", ref, res.Version, lb.LocalReference), true, nil
	}

	return "", false, nil
}

// ResourceOCIReference returns the absolute OCI image reference for a resource
// backed by OCI content. ok is false when the resource has no resolvable OCI
// reference: a non-OCI access, or a component-local blob that exists only by
// digest with no repository path to build from.
//
// A resource's access is either an OCIImage, which carries an absolute image
// reference directly, or a LocalBlob for component-local content. A LocalBlob
// resolves via its optional GlobalAccess (an absolute OCIImage or OCIImageLayer
// reference) or, failing that, its ReferenceName — a repository-relative path
// carrying no registry host.
//
// The access may be a concrete typed value or, when read back from a repository,
// an un-decoded *runtime.Raw; both forms are handled.
//
// repoBaseURL is the "<host>/<namespace>" the repository was opened with (e.g.
// "127.0.0.1:5000/my-components"). A host-less ReferenceName is prefixed with it
// to form a full absolute reference; references that already carry a registry
// host are returned unchanged.
func ResourceOCIReference(res descriptor.Resource, repoBaseURL string) (ref string, ok bool, err error) {
	switch a := res.Access.(type) {
	case *ociaccessv1.OCIImage:
		ref, ok = a.ImageReference, true
	case *v2.LocalBlob:
		ref, ok, err = localBlobReference(a, repoBaseURL)
		if err != nil {
			return "", false, fmt.Errorf("determine local blob address for resource %s: %w", res.Name, err)
		}
	case *descriptor.LocalBlob:
		// GlobalAccess is a runtime.Typed here; normalize it into the raw
		// envelope form so a single LocalBlob code path handles both.
		ref, ok, err = localBlobReference(&v2.LocalBlob{
			Type:           a.Type,
			LocalReference: a.LocalReference,
			MediaType:      a.MediaType,
			ReferenceName:  a.ReferenceName,
			GlobalAccess:   typedToRaw(a.GlobalAccess),
		}, repoBaseURL)
		if err != nil {
			return "", false, fmt.Errorf("determine local blob address for resource %s: %w", res.Name, err)
		}
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
			ref, ok = img.ImageReference, true
		case isLocalBlobTypeName(a.Name):
			var lb v2.LocalBlob
			if err := json.Unmarshal(a.Data, &lb); err != nil {
				return "", false, fmt.Errorf("failed to decode LocalBlob access: %w", err)
			}
			ref, ok, err = localBlobReference(&lb, repoBaseURL)
			if err != nil {
				return "", false, fmt.Errorf("determine local blob address for resource %s: %w", res.Name, err)
			}
		default:
			return "", false, nil
		}
	default:
		return "", false, nil
	}

	imageReference, err := ParseOCIRef(ref)
	if err != nil {
		return "", false, fmt.Errorf("parsing %s: %w", ref, err)
	}
	if imageReference.Digest == "" {
		imageReference.Digest = getDigest(res)
	}
	return imageReference.String(), ok, err
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

// If the resource has a suitable digest attached this function returns it a OCI digest format.
// Otherwise an empty string is returned.
func getDigest(res descriptor.Resource) string {
	if res.Digest != nil {
		var hashPrefix string
		switch res.Digest.HashAlgorithm {
		case "SHA-256":
			hashPrefix = "sha256"
		case "SHA-512":
			hashPrefix = "sha512"
		default:
			hashPrefix = ""
			fmt.Println("Unsupported hash algorithm " + res.Digest.HashAlgorithm)
		}
		if hashPrefix != "" {
			return fmt.Sprintf("%s:%s", hashPrefix, res.Digest.Value)
		}
	}

	return ""
}

// Join URLs with support of hosts without scheme (e.g. localhost:1234/foo/bar)
func joinRef(base string, part ...string) (string, error) {
	if schemePattern.MatchString(base) {
		return url.JoinPath(base, part...)
	}
	return path.Join(append([]string{base}, part...)...), nil
}
