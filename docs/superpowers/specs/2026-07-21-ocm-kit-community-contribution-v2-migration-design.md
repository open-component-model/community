# ocm-kit — Community Contribution + OCM v2 Migration

**Date:** 2026-07-21
**Status:** Approved design, ready for implementation planning

## 1. Goal & Strategy

Contribute [`ocm-kit`](https://github.com/opendefensecloud/ocm-kit) into the OCM
community repository **and** migrate it from OCM v1 (`ocm.software/ocm`) to the
OCM v2 SDK (`ocm.software/open-component-model/bindings/go/*`) in a **single
feature branch** that becomes one governance-compliant pull request.

Work is done as **ordered commits** on that branch so the review remains legible
despite being one PR. This is done on a fork (`opendefensecloud/ocm-community`);
the real upstream PR is prepared later.

ocm-kit is a small Go CLI that renders Helm values templates embedded in OCM
components. The OCM-specific surface is concentrated in two files
(`cmd/ocm-kit/main.go`, `helmvalues/helmvalues.go`); `compver/ref.go` is pure
string parsing with no OCM dependency and is unaffected.

### Locked decisions

| Decision | Choice |
|---|---|
| Sequencing | Contribute + migrate together, single feature branch |
| Module path | Rename to `github.com/open-component-model/community/ocm-kit` |
| Label namespace | Support both; prefer `ext.ocm.software/helm.values-for`, fall back to legacy `opendefense.cloud/helm/values-for` |
| Build tooling | Keep Make + Nix flake (Task is optional per repo README) |
| Relative OCI refs | Spike to find the v2 pattern, then preserve support |
| Library API | Redesign cleanly on v2; **breaking changes allowed**; no `internal/` isolation shim |
| Maintainer | `@trevex` (CODEOWNERS) |
| e2e | Exercise the OCM **v2 CLI** (`ocm.software/open-component-model/cli`) |

## 2. Architecture — clean v2-native library API

ocm-kit only needs to support v2, so we **redesign the library's public API
directly on v2 semantics** rather than building a defensive isolation layer. The
exported surface changes freely; breaking changes are expected and welcome in
service of a clear API.

Today `helmvalues` is coupled to v1 handle types (`ocm.ComponentVersionAccess`,
`ocm.ResourceAccess`) that have no v2 equivalent. In v2 you instead hold a
repository object plus a `*descriptor.Descriptor`. The redesign:

- **Rendering logic stays SDK-light and testable.** Functions that find
  templates, build rendering input, and render operate on the v2 descriptor
  types (`descriptor.Component`, `descriptor.Resource`) plus a small **public**
  repository/client abstraction for the operations that actually touch a
  registry (open repo, download resource bytes, resolve a resource's OCI
  reference). Exposing this as a public interface (not `internal/`) keeps the
  template-rendering core unit-testable with fakes and gives library consumers a
  seam.
- **`compver` is untouched** (no OCM dependency).
- **Public `ImageReference` struct stays stable** in shape (Host, Repository,
  Tag, Digest) even though its parser changes; it is the template-facing
  contract.

Feature parity is preserved: find-template-by-chart, first-template, rendering
input extraction, pull-secret resolution, and **both** absolute and relative OCI
reference resolution.

## 3. OCM v1 → v2 API Mapping (research-grounded)

The v2 SDK is a **multi-module workspace**; each `bindings/go/<x>` is an
independently versioned module (several still `v0.0.x` / alpha). There is **no
global `Context`** and repositories have **no `Close()`** — lifecycle is via
`context.Context`.

| Concern | v1 | v2 |
|---|---|---|
| Context/config | `ocmutils.Configure(ocm.DefaultContext(), "")` | Explicit resolvers; `context.Context`; no global context |
| Open OCI repo | `RepositoryForSpec(ocireg.NewRepositorySpec(base))` | `urlresolver.New(WithBaseURL, WithPlainHTTP, WithBaseClient)` + `oci.NewRepository(WithResolver, WithTempDir)` |
| CV lookup | `repo.LookupComponentVersion(name, ver)` | `repo.GetComponentVersion(ctx, name, ver)` → `*descriptor.Descriptor` |
| Descriptor / spec | `GetDescriptor().ComponentSpec` | `desc.Component` (`descriptor/runtime`) |
| Resources | `compVer.GetResources()` | `desc.Component.Resources []Resource` |
| Resource name/ver | `res.Meta().Name/.Version` | `res.ElementMeta.ObjectMeta.Name/.Version` (embedded) |
| Labels | `res.Meta().GetLabels()` → `v1.Label` | `res.Labels []Label`; `Value` always `json.RawMessage`; `Label.GetValue(dest)` |
| Blob download | `res.BlobAccess().Reader()` | `repo.GetLocalResource(ctx,name,ver,identity)` (local) or `repo.DownloadResource(ctx,res)` (external) → `blob.ReadOnlyBlob.ReadCloser()` |
| OCI access spec | assert `*ociartifact.AccessSpec`, read `.ImageReference` | `res.Access` is `runtime.Typed`; `Scheme.Convert(res.Access, &v1.OCIImage)`, read `.ImageReference` (type `OCIImage`, legacy `ociArtifact`) |
| Relative OCI ref | `*relativeociref.AccessSpec` + `GetOCIReference(compVer)` | **No drop-in**; replaced by `LocalBlob` + `GlobalAccess` push-time materialization — see spike |
| OCI ref parse | `oci.ParseRef` → `RefSpec{Host,Repository,Tag,Digest}` | `oci/looseref.ParseReference` → `{Registry,Repository,Tag,Reference(digest)}` |
| Credentials | implicit via `Configure` (`~/.ocmconfig` + docker) | explicit: `credentials` resolvers, `oci/credentials.ReadDockerConfig`, `configuration` scheme |

Relevant v2 modules to depend on (pin exact versions):
`bindings/go/descriptor/runtime`, `descriptor/v2`, `oci`,
`oci/spec/access/v1`, `oci/looseref`, `oci/resolver/url`, `oci/credentials`,
`repository`, `blob`, `credentials`, `configuration`, `runtime`.

## 4. Migration Work (Workstream B)

- **Dependencies:** remove `ocm.software/ocm` and `ocm.software/ocm/api/oci`
  (sheds the large AWS/Azure/Aliyun transitive tree); add the pinned v2 binding
  modules above. Renovate tracks them.
- **`cmd/ocm-kit/main.go`:** replace context/repo/lookup with
  `urlresolver` + `oci.NewRepository` + `GetComponentVersion`; thread a
  `context.Context` with signal-based cancellation; drop `Close()` calls.
- **`helmvalues` (redesigned):**
  - Operate on `*descriptor.Descriptor` + the public repository abstraction
    instead of v1 handles.
  - Label matching iterates `desc.Component.Resources[].Labels`, matching **both**
    `ext.ocm.software/helm.values-for` (preferred) and legacy
    `opendefense.cloud/helm/values-for`; read values via `Label.GetValue`.
  - Access → OCI reference: convert `res.Access` via the OCI access `Scheme` to
    `OCIImage` and read `.ImageReference`.
  - OCI parsing: replace `oci.ParseRef` with `oci/looseref.ParseReference`,
    mapping `Registry/Repository/Tag/Reference` into the stable public
    `ImageReference` struct.
- **Credential bootstrap:** small helper reproducing "docker-config +
  anonymous by default, optional OCM config file" (no v1 one-liner exists).
- **Relative-reference spike:** an explicit, up-front plan task to determine the
  v2 resolution pattern (`LocalBlob`/`GlobalAccess` vs repository resolution) and
  **preserve** relative-ref support. If genuinely infeasible on current v2,
  document the gap and surface it as a follow-up — do not silently drop it.

## 5. Contribution Readiness (Workstream A)

- **Module rename** → `github.com/open-component-model/community/ocm-kit`:
  update `go.mod`, all internal imports, and README examples.
- **CODEOWNERS:** append `/ocm-kit/   @trevex` to `.github/CODEOWNERS`
  (TSC-reviewed as part of the submission PR).
- **Label namespace compliance:** `ext.ocm.software/helm.values-for` becomes the
  primary key (satisfies `NAMING.md`); legacy key still read for backward compat
  (ties into §4).
- **CI:** add a new **project-scoped** workflow `.github/workflows/ocm-kit.yaml`,
  path-filtered to `ocm-kit/**`, running Go build + `go test` + golangci-lint +
  the e2e gate. The community repo's existing `pull-request.yaml` only does PR
  labeling — there is **no code CI** today — so ocm-kit must bring its own. This
  is a new file (not an edit to a repo-wide workflow), reviewed by the TSC in the
  same PR, consistent with governance.
- **Licensing:** the root `REUSE.toml` already blankets `**` as Apache-2.0
  (`precedence = "aggregate"`), so no per-file SPDX-header churn is required.
  Keep `ocm-kit/LICENSE`; verify it is Apache-2.0.
- **Build tooling:** keep Make + Nix flake. Note: `common.mk` is fetched from
  `opendefensecloud/dev-kit` at build time — a minor external build dependency
  for community CI; acceptable, flagged.
- **README:** rewrite for the community context and v2 usage.

## 6. Testing

- **Unit:** rework `helmvalues` tests against the redesigned public API using
  fakes for the repository abstraction (no live registry). `compver` tests
  unchanged.
- **e2e (`test/e2e.sh`):** currently uses zot + docker + the OCM CLI to build CTF
  fixtures. Migrate it to use the **OCM v2 CLI**
  (`ocm.software/open-component-model/cli`): regenerate the CTF fixtures with v2
  and drive the e2e flow through the v2 CLI. Treat as its own task.
- **Dependency-shrink signal:** capture `go mod graph` size before/after to
  confirm the transitive-tree reduction.

## 7. Risks & Open Questions

- **v2 bindings are alpha / `v0.0.x`** → pin exact versions, rely on Renovate,
  and keep the registry-touching surface small so churn is contained.
- **Relative-reference resolution** is the primary technical unknown → spike
  first (§4).
- **`descriptor/v2`** spec is alpha.
- **e2e fixtures** likely need regeneration with the v2 CLI.

## 8. Phasing (single branch, ordered commits)

1. Redesign `helmvalues` public API + tests with fakes (still compiling against
   current deps where possible) — establishes the target surface.
2. Swap dependencies to v2; implement the repository abstraction and rewire
   `cmd` + `helmvalues`; adopt `looseref` parsing.
3. Relative-reference spike + implementation.
4. Contribution readiness: module rename, CODEOWNERS, label key, CI workflow,
   README.
5. Green e2e against the v2 CLI (regenerate fixtures) + dependency-shrink check.
