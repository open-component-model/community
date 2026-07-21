# ocm-kit Community Contribution + OCM v2 Migration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate ocm-kit from the OCM v1 library to the OCM v2 SDK and make it a governance-compliant community-repo contribution, on a single feature branch.

**Architecture:** Redesign ocm-kit's public library API directly on v2 semantics (breaking changes allowed, no isolation shim). Pure logic (label matching, OCI-ref parsing, template rendering) is unit-tested with fakes; the registry-touching surface is a small public `Repository` interface implemented over the v2 `oci` bindings. Contribution-readiness changes (module rename, CODEOWNERS, CI, README) land last as mechanical commits.

**Tech Stack:** Go 1.26, `ocm.software/open-component-model/bindings/go/*` (v2 SDK, alpha/`v0.0.x`), Cobra, sprig, `oras.land/oras-go/v2`, Make + Nix, zot + Docker for e2e.

**Reference material:** The v2 SDK is cloned at `/tmp/ocm-v2-research` (multi-module workspace). Verbatim working examples live in `bindings/go/examples/{04_repository_test.go,06_oci_test.go,02_descriptor_test.go}`. Verified types: `descriptor/runtime/descriptor.go` (`Resource`, `Label`, `Component`, `LocalRelation`/`ExternalRelation`), `oci/spec/access/v1` (`OCIImage{Type, ImageReference}`, consts `OCIImageType="OCIImage"`, `LegacyType="ociArtifact"`), `oci/looseref/parse.go` (`ParseReference`, `LooseReference{Scheme, oras.Reference (Registry, Repository, Reference), Tag}`), `descriptor/runtime/local_access.go` (`LocalBlob.GlobalAccess`).

---

## File Structure

Current tree (`ocm-kit/`): `cmd/ocm-kit/main.go`, `helmvalues/{helmvalues.go,pullsecrets_file.go,pullsecrets-schema.json,*_test.go}`, `compver/{ref.go,ref_test.go}`, `go.mod`, `Makefile`, `test/e2e.sh`, `.github` lives at the **community-repo root** (one level above `ocm-kit/`).

Files created/modified by this plan:

- `helmvalues/repository.go` — **new.** Public `Repository` interface + the v2-backed implementation (`OCIRepository`), plus `OpenRepository`. Isolates all v2 `oci`/`resolver`/`blob` calls.
- `helmvalues/repository_fake.go` — **new.** `FakeRepository` for unit tests (exported so `cmd` tests could reuse it).
- `helmvalues/labels.go` — **new.** Label-key constants (new + legacy) and pure label-matching helpers over `descriptor.Resource`.
- `helmvalues/ociref.go` — **new.** `ImageReference` struct + `ParseOCIRef` built on `looseref`.
- `helmvalues/helmvalues.go` — **rewrite.** Template find/fetch/render + rendering-input extraction, now over `*descriptor.Descriptor` + `Repository`.
- `helmvalues/pullsecrets_file.go` — **unchanged** (no OCM dependency).
- `compver/ref.go` — **unchanged.**
- `cmd/ocm-kit/main.go` — **rewrite.** v2 wiring + `context.Context` + signal cancellation.
- `helmvalues/credentials.go` — **new.** Credential bootstrap (docker config + anonymous; optional OCM config file).
- `go.mod` / `go.sum` — v2 deps; module path rename (last).
- `.github/CODEOWNERS` (repo root) — add ocm-kit entry.
- `.github/workflows/ocm-kit.yaml` (repo root) — **new.** Path-filtered Go CI.
- `.pre-commit-config.yaml` (repo root) — fix broken hooks.
- `ocm-kit/README.md`, `ocm-kit/test/e2e.sh` — update for v2.

---

## Phase 0 — Baseline

### Task 0: Capture the "before" baseline

**Files:** none (measurement only).

- [ ] **Step 1: Confirm branch and record baseline metrics**

Run:
```bash
cd ocm-kit
git branch --show-current        # expect: feat/ocm-kit-community-contribution
go build ./... && go test -short ./... 2>&1 | tail -20
go mod graph | wc -l             # record: dependency-edge count BEFORE migration
```
Expected: build+tests pass; note the `go mod graph` count in the commit message of Task 17 for the shrink comparison.

- [ ] **Step 2: No commit** (baseline only).

---

## Phase 1 — Walking skeleton (lock down real v2 signatures)

### Task 1: Add v2 dependencies and prove the repository round-trip compiles

Goal: get the real v2 API compiling+running in-tree against a CTF backend before touching production code. This validates every signature the later tasks depend on.

**Files:**
- Test: `helmvalues/skeleton_test.go` (temporary; deleted in Task 6 Step 5)

- [ ] **Step 1: Write the failing skeleton test** (copied from the verified example `04_repository_test.go`)

```go
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
```

- [ ] **Step 2: Add the v2 modules and resolve versions**

Run (from `ocm-kit/`):
```bash
go get ocm.software/open-component-model/bindings/go/oci@latest
go get ocm.software/open-component-model/bindings/go/repository@latest
go get ocm.software/open-component-model/bindings/go/descriptor/runtime@latest
go get ocm.software/open-component-model/bindings/go/descriptor/v2@latest
go get ocm.software/open-component-model/bindings/go/blob@latest
go get ocm.software/open-component-model/bindings/go/ctf@latest
go get ocm.software/open-component-model/bindings/go/credentials@latest
go get ocm.software/open-component-model/bindings/go/runtime@latest
go get github.com/opencontainers/go-digest github.com/stretchr/testify oras.land/oras-go/v2
go mod tidy
```
Then **pin** the resolved versions (replace `@latest` intent) — do not leave floating; record exact versions in `go.mod`.

- [ ] **Step 3: Run the test to verify it passes**

Run: `go test ./helmvalues/ -run TestSkeleton_CTFRoundTrip -v`
Expected: PASS. If any symbol/signature differs from the snippet, correct it here — this is the point of the skeleton. Every later task uses only symbols proven in this test.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum helmvalues/skeleton_test.go
git commit -m "chore(ocm-kit): add OCM v2 SDK deps and verify repository round-trip"
```

---

## Phase 2 — Redesign the helmvalues library on v2

### Task 2: Public `Repository` interface + v2 implementation

**Files:**
- Create: `helmvalues/repository.go`
- Create: `helmvalues/repository_fake.go`
- Test: `helmvalues/repository_test.go`

- [ ] **Step 1: Write the failing test** (fake satisfies the interface; CTF-backed impl round-trips)

```go
package helmvalues

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./helmvalues/ -run TestFakeRepository_Implements -v`
Expected: FAIL — `Repository`, `FakeRepository` undefined.

- [ ] **Step 3: Implement `repository.go`**

```go
package helmvalues

import (
	"bytes"
	"context"
	"fmt"

	"github.com/opencontainers/go-digest"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	"ocm.software/open-component-model/bindings/go/blob"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ocm.software/open-component-model/bindings/go/oci"
	urlresolver "ocm.software/open-component-model/bindings/go/oci/resolver/url"
)

// Repository is the narrow, v2-backed surface helmvalues needs from an OCM
// repository. It is public so consumers and tests can supply their own.
type Repository interface {
	// GetComponentVersion resolves a component descriptor.
	GetComponentVersion(ctx context.Context, name, version string) (*descriptor.Descriptor, error)
	// ResourceBytes downloads the local blob of the named resource.
	ResourceBytes(ctx context.Context, name, version, resourceName string) ([]byte, error)
}

// OCIRepository implements Repository over the v2 oci bindings.
type OCIRepository struct{ repo *oci.Repository }

// OpenRepository opens an OCI-registry-backed OCM repository at baseURL
// (host + optional path, e.g. "ghcr.io/acme"). plainHTTP selects http vs https.
func OpenRepository(baseURL, tempDir string, plainHTTP bool, client *auth.Client) (*OCIRepository, error) {
	if client == nil {
		client = &auth.Client{Client: retry.DefaultClient, Cache: auth.NewCache()}
	}
	resolver, err := urlresolver.New(
		urlresolver.WithBaseURL(baseURL),
		urlresolver.WithPlainHTTP(plainHTTP),
		urlresolver.WithBaseClient(client),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resolver: %w", err)
	}
	repo, err := oci.NewRepository(oci.WithResolver(resolver), oci.WithTempDir(tempDir))
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}
	return &OCIRepository{repo: repo}, nil
}

func (o *OCIRepository) GetComponentVersion(ctx context.Context, name, version string) (*descriptor.Descriptor, error) {
	return o.repo.GetComponentVersion(ctx, name, version)
}

func (o *OCIRepository) ResourceBytes(ctx context.Context, name, version, resourceName string) ([]byte, error) {
	readBlob, _, err := o.repo.GetLocalResource(ctx, name, version, map[string]string{"name": resourceName})
	if err != nil {
		return nil, fmt.Errorf("failed to get local resource %q: %w", resourceName, err)
	}
	var buf bytes.Buffer
	if err := blob.Copy(&buf, readBlob); err != nil {
		return nil, fmt.Errorf("failed to read resource %q: %w", resourceName, err)
	}
	return buf.Bytes(), nil
}

var _ = digest.Canonical // keep go-digest referenced where needed by callers
```

Note: if `GetLocalResource` requires the `version` identity key too (verify against the skeleton test — the example passed both `name` and `version`), pass `{"name": resourceName, "version": <resource version>}`. Resolve the resource version from the descriptor before calling, or add a `resourceVersion` parameter. Confirm the exact identity keys against Task 1's passing test before finalizing.

- [ ] **Step 4: Implement `repository_fake.go`**

```go
package helmvalues

import (
	"context"
	"fmt"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
)

// FakeRepository is an in-memory Repository for tests.
type FakeRepository struct {
	Descriptor *descriptor.Descriptor
	Blobs      map[string][]byte // resource name -> content
}

func (f *FakeRepository) GetComponentVersion(_ context.Context, _, _ string) (*descriptor.Descriptor, error) {
	if f.Descriptor == nil {
		return nil, fmt.Errorf("no descriptor configured")
	}
	return f.Descriptor, nil
}

func (f *FakeRepository) ResourceBytes(_ context.Context, _, _, resourceName string) ([]byte, error) {
	b, ok := f.Blobs[resourceName]
	if !ok {
		return nil, fmt.Errorf("resource %q: %w", resourceName, ErrNotFound)
	}
	return b, nil
}
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./helmvalues/ -run TestFakeRepository_Implements -v`
Expected: PASS (`ErrNotFound` is defined in `helmvalues.go`; if not yet present, add `var ErrNotFound = errors.New("not found")` to `labels.go` in Task 3 and rerun).

- [ ] **Step 6: Commit**

```bash
git add helmvalues/repository.go helmvalues/repository_fake.go helmvalues/repository_test.go
git commit -m "feat(ocm-kit): add v2-backed Repository interface and fake"
```

### Task 3: Label matching over descriptor resources (both namespaces)

**Files:**
- Create: `helmvalues/labels.go`
- Test: `helmvalues/labels_test.go`

- [ ] **Step 1: Write the failing test**

```go
package helmvalues

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
)

func resWithLabel(name string, labels ...descriptor.Label) descriptor.Resource {
	return descriptor.Resource{
		ElementMeta: descriptor.ElementMeta{
			ObjectMeta: descriptor.ObjectMeta{Name: name, Labels: labels},
		},
	}
}

func TestMatchesHelmValuesLabel_NewAndLegacy(t *testing.T) {
	newLbl := descriptor.Label{Name: LabelHelmValuesFor, Value: json.RawMessage(`"mychart"`)}
	legacyLbl := descriptor.Label{Name: LegacyLabelHelmValuesFor, Value: json.RawMessage(`"mychart"`)}

	require.True(t, matchesHelmValuesLabel(resWithLabel("a", newLbl), "mychart"))
	require.True(t, matchesHelmValuesLabel(resWithLabel("b", legacyLbl), "mychart"))
	require.False(t, matchesHelmValuesLabel(resWithLabel("c", newLbl), "other"))
	require.False(t, matchesHelmValuesLabel(resWithLabel("d"), "mychart"))
	// hasAnyHelmValuesLabel ignores the value
	require.True(t, hasAnyHelmValuesLabel(resWithLabel("e", newLbl)))
	require.True(t, hasAnyHelmValuesLabel(resWithLabel("f", legacyLbl)))
	require.False(t, hasAnyHelmValuesLabel(resWithLabel("g")))
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./helmvalues/ -run TestMatchesHelmValuesLabel_NewAndLegacy -v`
Expected: FAIL — undefined constants/functions.

- [ ] **Step 3: Implement `labels.go`**

```go
package helmvalues

import (
	"encoding/json"
	"errors"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
)

const (
	// LabelHelmValuesFor is the compliant community label key (NAMING.md:
	// ext.ocm.software namespace). Preferred.
	LabelHelmValuesFor = "ext.ocm.software/helm.values-for"
	// LegacyLabelHelmValuesFor is the pre-contribution key, still read for
	// backward compatibility with already-published components.
	LegacyLabelHelmValuesFor = "opendefense.cloud/helm/values-for"
)

// ErrNotFound is returned when a requested Helm values template is not found.
var ErrNotFound = errors.New("not found")

func isHelmValuesLabelKey(name string) bool {
	return name == LabelHelmValuesFor || name == LegacyLabelHelmValuesFor
}

// matchesHelmValuesLabel reports whether res carries a helm-values label
// (new or legacy key) whose value equals chart.
func matchesHelmValuesLabel(res descriptor.Resource, chart string) bool {
	for _, l := range res.Labels {
		if isHelmValuesLabelKey(l.Name) && labelValueEquals(l, chart) {
			return true
		}
	}
	return false
}

// hasAnyHelmValuesLabel reports whether res carries any helm-values label,
// regardless of value.
func hasAnyHelmValuesLabel(res descriptor.Resource) bool {
	for _, l := range res.Labels {
		if isHelmValuesLabelKey(l.Name) {
			return true
		}
	}
	return false
}

func labelValueEquals(l descriptor.Label, target string) bool {
	// v2 label values are always json.RawMessage; a string value is JSON-quoted.
	var s string
	if err := json.Unmarshal(l.Value, &s); err == nil {
		return s == target
	}
	return string(l.Value) == target
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./helmvalues/ -run TestMatchesHelmValuesLabel_NewAndLegacy -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add helmvalues/labels.go helmvalues/labels_test.go
git commit -m "feat(ocm-kit): match helm-values label on ext.ocm.software and legacy keys"
```

### Task 4: OCI reference parsing via looseref

**Files:**
- Create: `helmvalues/ociref.go`
- Test: `helmvalues/ociref_test.go`

- [ ] **Step 1: Write the failing test**

```go
package helmvalues

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOCIRef(t *testing.T) {
	ref, err := ParseOCIRef("ghcr.io/acme/app:v1.2.3")
	require.NoError(t, err)
	require.Equal(t, "ghcr.io", ref.Host)
	require.Equal(t, "acme/app", ref.Repository)
	require.Equal(t, "v1.2.3", ref.Tag)
	require.Empty(t, ref.Digest)

	require.Equal(t, "ghcr.io/acme/app:v1.2.3", ref.String())

	d, err := ParseOCIRef("ghcr.io/acme/app@sha256:" + "abc123")
	require.NoError(t, err)
	require.Equal(t, "ghcr.io", d.Host)
	require.Equal(t, "acme/app", d.Repository)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./helmvalues/ -run TestParseOCIRef -v`
Expected: FAIL — undefined `ParseOCIRef`/`ImageReference`.

- [ ] **Step 3: Implement `ociref.go`** (maps `looseref.LooseReference` → the stable public `ImageReference`)

```go
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
	return ImageReference{
		Host:       lr.Registry,
		Repository: lr.Repository,
		Tag:        lr.Tag,
		Digest:     lr.Reference.Reference, // digest string, empty if tag-only
	}, nil
}
```

Note: verify `lr.Reference.Reference` holds the digest (the `oras.Reference` embed exposes `.Reference`; `LooseReference.Tag` holds the tag). If the digest sits elsewhere, adjust using Task 1's proven types. Get `oci/looseref` via `go get ocm.software/open-component-model/bindings/go/oci/looseref@<pinned>` if it is a separate module (it is under the `oci` module tree — `go mod tidy` will resolve it).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./helmvalues/ -run TestParseOCIRef -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add helmvalues/ociref.go helmvalues/ociref_test.go
git commit -m "feat(ocm-kit): parse OCI refs via v2 looseref parser"
```

### Task 5: Resolve a resource's absolute OCI reference from its access spec

**Files:**
- Modify: `helmvalues/ociref.go`
- Test: `helmvalues/ociref_test.go`

- [ ] **Step 1: Write the failing test** (absolute `OCIImage` access → image reference)

```go
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
```

Add imports to the test file:
```go
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	ociaccessv1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
	"ocm.software/open-component-model/bindings/go/runtime"
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./helmvalues/ -run TestResourceOCIReference_OCIImage -v`
Expected: FAIL — undefined `ResourceOCIReference`.

- [ ] **Step 3: Implement `ResourceOCIReference` in `ociref.go`**

```go
// ResourceOCIReference returns the absolute OCI image reference for a resource
// whose access is an OCIImage (or legacy ociArtifact). ok is false when the
// resource is not OCI-image backed. Relative-reference resolution is handled
// separately (see resolveRelativeReference).
func ResourceOCIReference(res descriptor.Resource) (ref string, ok bool, err error) {
	switch a := res.Access.(type) {
	case *ociaccessv1.OCIImage:
		return a.ImageReference, true, nil
	default:
		return "", false, nil
	}
}
```
Add imports:
```go
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	ociaccessv1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
```

Note: when a descriptor is read back via `GetComponentVersion`, `res.Access` may arrive as an un-decoded `*runtime.Raw` rather than a `*ociaccessv1.OCIImage`. If Task 1's descriptor round-trip shows `Access` as `*runtime.Raw`, extend this function to also handle `*runtime.Raw`: inspect its `Type` for `OCIImage`/`ociArtifact`/`ociRegistry`/`ociImage` and `json.Unmarshal(raw.Data, &OCIImage{})` to read `ImageReference`. Add a test case covering the `*runtime.Raw` form using a value produced by an actual round-trip. **This is verified, not assumed — do it against real round-tripped data.**

- [ ] **Step 4: Run to verify pass**

Run: `go test ./helmvalues/ -run TestResourceOCIReference_OCIImage -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add helmvalues/ociref.go helmvalues/ociref_test.go
git commit -m "feat(ocm-kit): resolve absolute OCI reference from resource access"
```

### Task 6: Rewrite `helmvalues.go` core (find/fetch template, rendering input, render)

**Files:**
- Rewrite: `helmvalues/helmvalues.go`
- Test: `helmvalues/helmvalues_test.go` (rework existing)
- Delete: `helmvalues/skeleton_test.go`

- [ ] **Step 1: Rework the tests** to drive the new API with `FakeRepository`. Keep the existing render/pull-secret test cases; change the setup to build a `*descriptor.Descriptor` + `FakeRepository` instead of v1 handles. Representative test:

```go
func TestGetFirstHelmValuesTemplate_FromFake(t *testing.T) {
	desc := &descriptor.Descriptor{Component: descriptor.Component{
		ComponentMeta: descriptor.ComponentMeta{ObjectMeta: descriptor.ObjectMeta{Name: "c", Version: "1.0.0"}},
		Resources: []descriptor.Resource{
			resWithLabel("tmpl", descriptor.Label{Name: LabelHelmValuesFor, Value: json.RawMessage(`"mychart"`)}),
		},
	}}
	desc.Component.Resources[0].Version = "1.0.0"
	repo := &FakeRepository{Descriptor: desc, Blobs: map[string][]byte{"tmpl": []byte("key: value")}}

	tmpl, err := GetFirstHelmValuesTemplate(context.Background(), repo, desc)
	require.NoError(t, err)
	require.Equal(t, "tmpl", tmpl.ResourceName)
	require.Equal(t, "key: value", tmpl.TemplateContent)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./helmvalues/ -run TestGetFirstHelmValuesTemplate_FromFake -v`
Expected: FAIL — new signatures undefined.

- [ ] **Step 3: Rewrite `helmvalues.go`** with these signatures (drop all `ocm.software/ocm` imports; keep `Render`, `RenderingInput`, `getFuncMap`, `ImageReference` usage unchanged in behavior):

```go
// Find/fetch (descriptor + repository based):
func FindHelmValuesTemplate(desc *descriptor.Descriptor, chart string) (*descriptor.Resource, error)      // uses matchesHelmValuesLabel
func FindFirstHelmValuesTemplate(desc *descriptor.Descriptor) (*descriptor.Resource, error)                // uses hasAnyHelmValuesLabel
func FetchHelmValuesTemplate(ctx context.Context, repo Repository, comp, ver string, res *descriptor.Resource) (*HelmValuesTemplate, error) // repo.ResourceBytes
func GetHelmValuesTemplate(ctx context.Context, repo Repository, desc *descriptor.Descriptor, chart string) (*HelmValuesTemplate, error)
func GetFirstHelmValuesTemplate(ctx context.Context, repo Repository, desc *descriptor.Descriptor) (*HelmValuesTemplate, error)

// Rendering input (iterate desc.Component.Resources, resolve OCI refs):
func GetRenderingInput(desc *descriptor.Descriptor) (*RenderingInput, error)
```

`RenderingInput.Component` changes type from `*compdesc.ComponentSpec` to `*descriptor.Component`. `GetRenderingInput` iterates `desc.Component.Resources`, calls `ResourceOCIReference(res)`; when `ok`, `ParseOCIRef` the result and store in `OCIResources[res.Name]`; skip when not ok (relative refs are filled in by Task 7). `Render`, `RenderOption`, `WithYAMLValidation`, `getFuncMap`, pull-secret funcs, `matchLabelValue` deletion (superseded by `labelValueEquals`) — carry over unchanged except for type of `Component`.

Full implementation mirrors the current `helmvalues.go` logic; only the OCM-access points change to the functions built in Tasks 3–5. Preserve `HelmValuesTemplate{ResourceName, ResourceVersion, TemplateContent}` and populate `ResourceVersion` from `res.Version`.

- [ ] **Step 4: Delete the skeleton test and run the full package**

Run:
```bash
rm helmvalues/skeleton_test.go
go test ./helmvalues/ -v
```
Expected: PASS (all reworked tests green).

- [ ] **Step 5: Commit**

```bash
git add helmvalues/helmvalues.go helmvalues/helmvalues_test.go
git rm helmvalues/skeleton_test.go
git commit -m "feat(ocm-kit)!: rewrite helmvalues library on OCM v2 descriptor API"
```

---

## Phase 3 — Relative OCI reference resolution (spike + implement)

### Task 7: Spike then implement relative-reference resolution

v2 has no drop-in for v1's `relativeociref` + `GetOCIReference(compVer)`. This task first investigates, then implements against the confirmed pattern.

**Files:**
- Modify: `helmvalues/ociref.go`, `helmvalues/helmvalues.go`
- Test: `helmvalues/ociref_test.go`

- [ ] **Step 1: SPIKE — determine the v2 relative-reference model**

Investigate in `/tmp/ocm-v2-research` (concrete acceptance criterion: produce a runnable example, added as a temporary `spike_test.go`, that reads back a component whose resource is stored local-to-the-repo and yields its resolvable absolute OCI reference):
- Read `bindings/go/descriptor/runtime/local_access.go` (`LocalBlob.GlobalAccess runtime.Typed`) and `descriptor/v2/local_access.go`.
- Grep the CLI + transfer bindings for how a `LocalBlob`'s `GlobalAccess` / `ReferenceName` is turned into an absolute reference: `grep -rn "GlobalAccess\|ReferenceName\|LocalBlob" /tmp/ocm-v2-research/bindings/go/oci /tmp/ocm-v2-research/cli`.
- Determine: for a resource whose `Access` is `LocalBlob`, does reading it back from an OCI repo populate `GlobalAccess` with an `OCIImage` (absolute)? Or must we compose `<repo base>/<component>/<resource>` ourselves?

Write findings as a comment block at the top of the implementation in Step 3. If no in-SDK resolution exists, the fallback is to compose the reference from the repository base URL + component + resource identity (document this explicitly — do not leave silent).

- [ ] **Step 2: Write the failing test** based on the confirmed model. Example (adjust to spike outcome — this assumes `GlobalAccess` carries an `OCIImage`):

```go
func TestResourceOCIReference_RelativeViaGlobalAccess(t *testing.T) {
	res := descriptor.Resource{
		Access: &v2.LocalBlob{
			LocalReference: "sha256:deadbeef",
			GlobalAccess: &ociaccessv1.OCIImage{
				Type:           runtime.Type{Name: ociaccessv1.OCIImageType, Version: "v1"},
				ImageReference: "ghcr.io/acme/app@sha256:deadbeef",
			},
		},
	}
	ref, ok, err := ResourceOCIReference(res)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/app@sha256:deadbeef", ref)
}
```

- [ ] **Step 3: Extend `ResourceOCIReference`** to handle `*v2.LocalBlob` (and `*runtime.Raw` of the same) by reading `GlobalAccess` and recursing/extracting the `OCIImage.ImageReference`. Implement per spike findings; include the fallback composition path if the spike proved `GlobalAccess` is not populated on read-back.

- [ ] **Step 4: Run the package tests**

Run: `go test ./helmvalues/ -v`
Expected: PASS. Delete any temporary `spike_test.go`.

- [ ] **Step 5: Commit**

```bash
git add helmvalues/ociref.go helmvalues/ociref_test.go helmvalues/helmvalues.go
git commit -m "feat(ocm-kit): resolve relative OCI references via v2 LocalBlob GlobalAccess"
```

---

## Phase 4 — CLI wiring + credentials

### Task 8: Credential bootstrap helper

**Files:**
- Create: `helmvalues/credentials.go`
- Test: `helmvalues/credentials_test.go`

- [ ] **Step 1: Write the failing test** (anonymous default returns a usable auth client; docker-config path is read when present)

```go
func TestDefaultAuthClient_Anonymous(t *testing.T) {
	c, err := DefaultAuthClient("")
	require.NoError(t, err)
	require.NotNil(t, c)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./helmvalues/ -run TestDefaultAuthClient_Anonymous -v`
Expected: FAIL — undefined `DefaultAuthClient`.

- [ ] **Step 3: Implement `credentials.go`** — build an `*auth.Client` (`oras.land/oras-go/v2/registry/remote/auth`) using `retry.DefaultClient` + `auth.NewCache()`. When `dockerConfigPath != ""` (or the standard `~/.docker/config.json` exists), wire a `Credential` func backed by the docker config via `bindings/go/oci/credentials.ReadDockerConfig` (confirm the exact function name/signature against `/tmp/ocm-v2-research/bindings/go/oci/credentials`). Anonymous otherwise. Keep OCM-config-file (`~/.ocmconfig`) support as an optional, additive branch behind a parameter; if the `configuration` wiring proves heavy, ship docker+anonymous and note ocmconfig as a follow-up (log it, do not pretend it works).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./helmvalues/ -run TestDefaultAuthClient_Anonymous -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add helmvalues/credentials.go helmvalues/credentials_test.go
git commit -m "feat(ocm-kit): add docker-config/anonymous credential bootstrap"
```

### Task 9: Rewrite `cmd/ocm-kit/main.go` on v2

**Files:**
- Rewrite: `cmd/ocm-kit/main.go`

- [ ] **Step 1: Rewrite `main.go`** — inside `RunE`:
  - `ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM); defer stop()`
  - `cvr, err := compver.SplitRef(ref)` (unchanged).
  - `client, err := helmvalues.DefaultAuthClient("")`.
  - `repo, err := helmvalues.OpenRepository(cvr.BaseURL host+namespace, os.TempDir()/tmpdir, plainHTTP=false, client)` — derive base URL (host + namespace) and plainHTTP from `cvr.Protocol`.
  - `desc, err := repo.GetComponentVersion(ctx, cvr.ComponentName, cvr.Version)`.
  - Template selection: local file → `HelmValuesTemplate{...}`; `--chart-resource` → `helmvalues.GetHelmValuesTemplate(ctx, repo, desc, chartResName)`; else `helmvalues.GetFirstHelmValuesTemplate(ctx, repo, desc)`.
  - `input, err := helmvalues.GetRenderingInput(desc)`; pull-secrets branch unchanged.
  - `output, err := helmvalues.Render(template, input)`; `fmt.Println(output)`.
  - Drop all `repo.Close()`/`compVer.Close()` (no such methods in v2).

Keep the three existing flags (`-r`, `-f`, `-p`) and `Use`/`Short`/`Long`/`Args` text.

- [ ] **Step 2: Build and smoke-run**

Run:
```bash
go build ./... && ./ocm-kit --help
```
Expected: builds; help shows the three flags.

- [ ] **Step 3: Commit**

```bash
git add cmd/ocm-kit/main.go
git commit -m "feat(ocm-kit)!: wire CLI to OCM v2 repository and context lifecycle"
```

---

## Phase 5 — Contribution readiness

### Task 10: Rename the Go module

**Files:**
- Modify: `go.mod`, every `.go` file importing `go.opendefense.cloud/ocm-kit/...`, `README.md`.

- [ ] **Step 1: Rename**

Run (from `ocm-kit/`):
```bash
OLD=go.opendefense.cloud/ocm-kit
NEW=github.com/open-component-model/community/ocm-kit
go mod edit -module "$NEW"
grep -rl "$OLD" --include='*.go' . | xargs sed -i "s#$OLD#$NEW#g"
go mod tidy
go build ./... && go test -short ./...
```
Expected: builds and tests pass under the new path.

- [ ] **Step 2: Commit**

```bash
git add -A
git commit -m "chore(ocm-kit)!: rename module to github.com/open-component-model/community/ocm-kit"
```

### Task 11: Add CODEOWNERS entry

**Files:**
- Modify: `.github/CODEOWNERS` (community-repo root, one level above `ocm-kit/`)

- [ ] **Step 1: Append the entry** (after the example comment block):

```
/ocm-kit/                         @trevex
```

- [ ] **Step 2: Commit**

```bash
cd .. && git add .github/CODEOWNERS
git commit -m "chore: add ocm-kit CODEOWNERS entry"
cd ocm-kit
```

### Task 12: Add project-scoped CI workflow

**Files:**
- Create: `.github/workflows/ocm-kit.yaml` (community-repo root)

- [ ] **Step 1: Create the workflow** (path-filtered Go build/test/lint; e2e as a separate job gated to run on ocm-kit changes)

```yaml
name: ocm-kit
on:
  pull_request:
    paths: [ 'ocm-kit/**', '.github/workflows/ocm-kit.yaml' ]
  push:
    branches: [ main ]
    paths: [ 'ocm-kit/**' ]
permissions:
  contents: read
jobs:
  build-test:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: ocm-kit
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: ocm-kit/go.mod
          cache-dependency-path: ocm-kit/go.sum
      - run: go build ./...
      - run: go test -short ./...
      - uses: golangci/golangci-lint-action@v6
        with:
          working-directory: ocm-kit
```

Pin the action SHAs to match the repo's existing convention (the other workflow pins by SHA) before finalizing.

- [ ] **Step 2: Commit**

```bash
cd .. && git add .github/workflows/ocm-kit.yaml
git commit -m "ci: add path-filtered build/test/lint workflow for ocm-kit"
cd ocm-kit
```

### Task 13: Fix the broken repo-root pre-commit config

**Files:**
- Replace: `.pre-commit-config.yaml` (currently a dangling symlink into `/nix/store`)

- [ ] **Step 1: Replace the symlink** with a real config whose hooks are valid at repo root (do not call non-existent root `make` targets). Minimal:

```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
      - id: end-of-file-fixer
      - id: trailing-whitespace
      - id: check-yaml
```

Run:
```bash
cd .. && rm -f .pre-commit-config.yaml
# write the file above
git add .pre-commit-config.yaml
git commit -m "chore: replace broken pre-commit symlink with valid root config"
cd ocm-kit
```

Note: confirm with the maintainer whether Go fmt/lint should run scoped to `ocm-kit/` here (pre-commit `entry` with `working-directory`), or stay in CI only. Default: CI only (this config is generic hygiene).

### Task 14: Rewrite the README for community context + v2

**Files:**
- Rewrite: `ocm-kit/README.md`

- [ ] **Step 1: Update** install/import path to `github.com/open-component-model/community/ocm-kit`, describe v2 usage, document **both** label keys (preferred `ext.ocm.software/helm.values-for`, legacy still read), and the `ext.ocm.software` naming compliance. Keep usage examples for the three flags.

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs(ocm-kit): update README for community contribution and v2"
```

---

## Phase 6 — e2e against the v2 CLI + verification

### Task 15: Migrate e2e to the OCM v2 CLI

**Files:**
- Modify: `ocm-kit/test/e2e.sh`, `ocm-kit/test/fixtures/**`, `Makefile` (`OCM` binary source)

- [ ] **Step 1: Point the e2e at the v2 CLI.** The current script uses `$(OCM)` (a v1 `ocm` binary in `bin/`) + zot to build CTF fixtures. Replace with the v2 CLI (`ocm.software/open-component-model/cli`): update `Makefile`'s `$(OCM)` acquisition (or add a `bin/ocm` install target that fetches the v2 CLI release), and update `test/e2e.sh` commands to v2 CLI syntax. Regenerate the CTF fixtures under `test/fixtures/arc/*` with the v2 CLI (the checked-in `component-constructor.yaml` should still describe the component; regenerate the `ctf`/`ctf-rel` blob trees).

- [ ] **Step 2: Run e2e locally**

Run:
```bash
make e2e
```
Expected: PASS end-to-end against zot using the v2 CLI-built fixtures. Fix ref/label formats surfaced (labels must use the new `ext.ocm.software/helm.values-for` key in regenerated fixtures; optionally keep one legacy-key fixture to prove backward compat).

- [ ] **Step 3: Commit**

```bash
git add test/ Makefile
git commit -m "test(ocm-kit): run e2e against OCM v2 CLI and regenerate fixtures"
```

### Task 16: Final verification and dependency-shrink check

**Files:** none (verification).

- [ ] **Step 1: Full green + measure**

Run:
```bash
go build ./... && go test ./... && make lint
go mod graph | wc -l    # compare to the Task 0 baseline
```
Expected: all green; edge count materially lower than baseline (v1 AWS/Azure/Aliyun tree gone). Record before/after in the commit message.

- [ ] **Step 2: Commit (if go.mod/go.sum tidied)**

```bash
git add go.mod go.sum
git commit -m "chore(ocm-kit): tidy dependencies after v2 migration" || echo "nothing to tidy"
```

- [ ] **Step 3: Review the diff against the spec** — confirm every locked decision (module path, both label keys, Make+Nix kept, relative refs preserved, `@trevex` owner, v2-CLI e2e) is reflected. Open the PR when ready.

---

## Self-Review Notes

- **Spec coverage:** sequencing (single branch, ordered commits — all phases), module rename (Task 10), label namespace both-keys (Tasks 3, 14, 15), Make+Nix kept (untouched; CI added Task 12), relative-ref spike+preserve (Task 7), clean v2-native public API no `internal/` (Tasks 2–6), credential bootstrap (Task 8), CODEOWNERS `@trevex` (Task 11), CI workflow (Task 12), pre-commit fix (Task 13), README (Task 14), v2-CLI e2e (Task 15), dep-shrink signal (Tasks 0, 16). All covered.
- **Alpha-SDK honesty:** Tasks 5, 7, 8, 15 include explicit "verify against real round-tripped data / confirm signature" steps rather than assuming — because the v2 bindings are alpha and were read, not compiled. The Task 1 walking skeleton locks the core signatures first so later tasks build on proven types.
