# ocm-kit

[![Go Report Card](https://goreportcard.com/badge/github.com/open-component-model/community/ocm-kit)](https://goreportcard.com/report/github.com/open-component-model/community/ocm-kit)
[![Go Reference](https://pkg.go.dev/badge/github.com/open-component-model/community/ocm-kit.svg)](https://pkg.go.dev/github.com/open-component-model/community/ocm-kit)

A Go library and CLI tool for working with Open Component Model (OCM) Helm values templates.

Built on the [OCM v2 SDK](https://github.com/open-component-model/open-component-model)
(`ocm.software/open-component-model/bindings/go/*`). This project is a community
contribution living in the OCM community repository; see its
[NAMING conventions](../NAMING.md) for the `ext.ocm.software` label namespace.

## Problem Statement

When an OCM ComponentVersion is transferred from one OCI Registry to another, the default values of Helm Charts will contain images of the old OCI Registry. This library provides functionality to manage and render Helm values templates that are embedded as resources in OCM components, ensuring that image references are correctly resolved regardless of the registry they're accessed from.

## Solution Overview

The library provides mechanisms to:

1. **Find Helm Values Templates**: Locate Helm values templates in OCM components using a label-based approach (`ext.ocm.software/helm.values-for`, with the legacy `opendefense.cloud/helm/values-for` key still recognized for backward compatibility)
2. **Render Templates**: Process the templates using Go's text/template with sprig functions for flexible value substitution
3. **Extract Component Data**: Automatically extract resource information from OCM components and prepare it for templating

For a smoother development experience, a small `ocm-kit` CLI is also provided, which allows local testing of rendering helm values templates or verifying outputs.

## Installation & Building

### Prerequisites
- Go 1.26 or later
- Docker (for running e2e tests)
- OCM v2 CLI (for e2e tests)

### Build
```bash
go build ./...
```

### Run Go tests

```bash
# Run all tests
make test
# Run particular tests
go test ./helmvalues
```

### Run e2e Tests
```bash
# Run e2e tests with default version (timestamp-based)
make e2e

# Run e2e tests with a stable component version
VERSION=0.1.0 make e2e

# Run e2e tests but keep zot registry running
make e2e-keep-zot

# Stop and remove zot registry
make e2e-stop-zot
```

## Releases

Tagging `ocm-kit/vX.Y.Z` publishes a GitHub Release with cross-compiled binaries
(linux/darwin/windows, amd64/arm64) and a `SHA256SUMS` file, and pushes a
multi-arch container image to `ghcr.io/open-component-model/community/ocm-kit`
(tagged with the version — no `latest` tag is published):

```bash
docker run --rm ghcr.io/open-component-model/community/ocm-kit:vX.Y.Z --version
```

### Supply-chain security

The container image is keyless-signed with [cosign](https://github.com/sigstore/cosign)
(Sigstore) and carries attestations: SLSA build provenance, an SPDX SBOM, and a
Trivy vulnerability (CVE) report. Verify the signature and attestations against
the release workflow's identity:

```bash
IMAGE=ghcr.io/open-component-model/community/ocm-kit:vX.Y.Z
IDENTITY='^https://github.com/open-component-model/community/.github/workflows/ocm-kit-release.yaml@.*'
ISSUER=https://token.actions.githubusercontent.com

# Signature
cosign verify --certificate-identity-regexp "$IDENTITY" --certificate-oidc-issuer "$ISSUER" "$IMAGE"

# Vulnerability (CVE) attestation
cosign verify-attestation --type vuln \
  --certificate-identity-regexp "$IDENTITY" --certificate-oidc-issuer "$ISSUER" "$IMAGE"

# SBOM and provenance attestations attached by BuildKit
docker buildx imagetools inspect "$IMAGE" --format '{{ json .SBOM }}'
docker buildx imagetools inspect "$IMAGE" --format '{{ json .Provenance }}'
```

## CLI Usage

The `ocm-kit` command-line tool renders Helm values templates from OCM components.

### Basic Usage
```bash
ocm-kit <component-version-ref> [flags]
```

Where `<component-version-ref>` is in the format: `protocol://host/namespace//component:version`

Examples:
```bash
# Render first values template from component in local OCI registry
ocm-kit "http://localhost:5000/my-components//opendefense.cloud/arc:0.1.0"

# Render specific values template from remote registry by providing which helm resource it is for
ocm-kit "https://registry.example.com/stable//example.com/myapp:1.2.3" -r my-chart

# Use a local template file instead of component template
ocm-kit "http://localhost:5000/my-components//opendefense.cloud/arc:0.1.0" \
  --local-helm-values-template ./values.yaml.tpl
```

### Command Flags

- `-r, --chart-resource string` - Name of the Helm chart resource in the component (default: "")
- `-f, --local-helm-values-template string` - Path to a local Helm values template file (overrides component template)
- `-p, --pull-secrets-file string` - Path to a pull secrets JSON file mapping registries to Kubernetes secret names
- `-h, --help` - Display help message

### Registry Credentials

`ocm-kit` resolves registry credentials through the OCM v2 SDK's **credential
graph**, which unifies two credential sources with an anonymous fallback:

1. **OCM config** (`~/.ocmconfig`, or the file named by `$OCM_CONFIG`). Both of
   its credential mechanisms are honored:
   - `Credentials/v1` **consumers** matched by `OCIRegistry` consumer identity
     (hostname, and optionally path/port), and
   - `DockerConfig/v1` **repositories** it declares.
2. The implicit **Docker configuration** (`$DOCKER_CONFIG/config.json`,
   otherwise `~/.docker/config.json`) — including any configured Docker
   credential helpers. This is always consulted, even without an `~/.ocmconfig`,
   so if you can `docker pull` an image, `ocm-kit` can read it.

If no source matches a given registry, `ocm-kit` falls back to **anonymous**
access — which is all that public registries need. A registry with no
credentials never fails the request.

To authenticate with the Docker config, log in with Docker:

```bash
docker login ghcr.io
# or, for a local registry:
docker login localhost:5000
```

Alternatively, declare credentials in `~/.ocmconfig` (or point `$OCM_CONFIG` at a
file). A minimal example wiring credentials for a single registry:

```yaml
type: generic.config.ocm.software/v1
configurations:
  - type: credentials.config.ocm.software
    consumers:
      - identities:
          - type: OCIRegistry
            hostname: ghcr.io       # matches all paths on this host
        credentials:
          - type: OCICredentials/v1
            username: my-user
            password: ghp_your_token_here
```

There are no CLI flags for credentials; they are resolved automatically from the
sources above.

### Pull Secrets

When rendering the Helm values template, you may need to attach
`imagePullSecrets` to deployments. The `--pull-secrets-file` flag and the
`pullSecretFor` template function work together to map OCI registries to
Kubernetes Secret names.

This approach allows the template author to specify where to add
`imagePullSecrets` in a `values.yaml` while the deployer has control over
deployment specific data like the concrete secret names.

The pull secrets file uses the following format:

```json
{
  "$schema": "https://raw.githubusercontent.com/open-component-model/community/refs/heads/main/ocm-kit/helmvalues/pullsecrets-schema.json",
  "pullSecrets": [
    {
      "registry": "docker.io",
      "secretName": "docker-hub-cred"
    },
    {
      "registry": "ghcr.io/my-org",
      "secretName": "ghcr-org-cred"
    },
    {
      "registry": "localhost:5000",
      "secretName": "regcred"
    }
  ]
}
```

The `pullSecretFor` function in templates resolves an OCI reference (hostname,
hostname/repo, or full image ref) to the matching secret name. Resolution walks
from most-specific to least-specific path segments:

- `pullSecretFor "ghcr.io/my-org/my-repo:latest"` checks `ghcr.io/my-org/my-repo` -> `ghcr.io/my-org` -> `ghcr.io`
- `pullSecretFor "docker.io"` checks the registry host directly

If no match is found `pullSecretFor` returns the empty string.

It is advised that templates may always be written in a way that gracefully
handle `pullSecretFor` returning an empty string value. Like in the example
below, template authors can make use of go-template's `with` expression.

Example template usage:

```yaml
{{- $image := index .OCIResources "my-image" }}
{{- with pullSecretFor $image.Host }}
imagePullSecrets:
  - name: {{ . }}
{{- end }}
```

CLI invocation:

```bash
ocm-kit "https://example.com/my-components//example.com/my-component:0.1.0" \
  --pull-secrets-file ./pull-secrets.json
```

### Example

#### Values Template

```yaml
apiserver:
  image:
    {{- $apiserver := index .OCIResources "arc-apiserver-image" }}
    repository: {{ $apiserver.Host }}/{{ $apiserver.Repository }}
    tag: {{ $apiserver.Tag }}

controller:
  image:
    {{- $controller := index .OCIResources "arc-controller-manager-image" }}
    repository: {{ $controller.Host }}/{{ $controller.Repository }}
    tag: {{ $controller.Tag }}

etcd:
  image:
    {{- $etcdImage := index .OCIResources "etcd-image" }}
    repository: {{ $etcdImage.Host }}/{{ $etcdImage.Repository }}
    tag: {{ $etcdImage.Tag }}
```

#### Render Output

```yaml
apiserver:
  image:
    repository: localhost:5000/my-components/opendefensecloud/arc-apiserver
    tag: v0.2.0

controller:
  image:
    repository: localhost:5000/my-components/opendefensecloud/arc-controller-manager
    tag: v0.2.0
```

## Library Usage Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/open-component-model/community/ocm-kit/compver"
    "github.com/open-component-model/community/ocm-kit/helmvalues"
)

func main() {
    ctx := context.Background()

    // Parse a component version reference of the form
    // protocol://host/namespace//component:version.
    cvr, err := compver.SplitRef("http://localhost:5000/my-components//opendefense.cloud/arc:0.1.0")
    if err != nil {
        log.Fatal(err)
    }

    // Build a credential-aware client. Credentials resolve through the OCM
    // credential graph: ~/.ocmconfig (consumers + DockerConfig repositories) and
    // the implicit ~/.docker/config.json, with anonymous fallback. Pass "" to
    // auto-discover the OCM config ($OCM_CONFIG, else ~/.ocmconfig).
    client, err := helmvalues.NewAuthClient(ctx, "")
    if err != nil {
        log.Fatal(err)
    }

    // Open the OCI-registry-backed OCM repository. The URL carries the scheme
    // (http selects plain HTTP; https/oci select TLS). Close removes the
    // repository's blob-staging directory.
    baseURL := cvr.Host + "/" + cvr.Namespace
    repo, err := helmvalues.OpenRepository(cvr.Protocol+"://"+baseURL, helmvalues.WithAuthClient(client))
    if err != nil {
        log.Fatal(err)
    }
    defer repo.Close()

    // Resolve the component descriptor.
    desc, err := repo.GetComponentVersion(ctx, cvr.ComponentName, cvr.Version)
    if err != nil {
        log.Fatal(err)
    }

    // Find the helm values template for a specific chart and download it.
    tmpl, err := helmvalues.GetHelmValuesTemplate(ctx, repo, desc, "helm-chart")
    if err != nil {
        log.Fatal(err)
    }

    // Build the rendering input (OCI resources, component metadata) and render.
    // baseURL lets relative resource references be resolved to full image refs.
    input, err := helmvalues.GetRenderingInput(desc, baseURL)
    if err != nil {
        log.Fatal(err)
    }

    renderedValues, err := helmvalues.Render(tmpl, input)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(renderedValues)
}
```

## Template Variables

When rendering templates, the following data is available via the context:

### `.OCIResources`
A map of all oci resources in the component by resource name. Only resources with an OCI-based access method are listed:

Each resource is automatically parsed into an object with:
- `.Host` - The registry host
- `.Repository` - The repository path
- `.Tag` - The image tag
- `.Digest` - The image digest

Resources without an OCI-based access method are not included in this map.

Access example (field and map names are case-sensitive, and rendering uses
`missingkey=error`, so the casing below must be matched exactly):
```yaml
{{- $image := index .OCIResources "my-image" }}
repository: {{ $image.Host }}/{{ $image.Repository }}
tag: {{ $image.Tag }}
```

### `.PullSecrets`
A map of registries to secret-names. Use the `pullSecretFor` template function
to refer to possible `imagePullSecrets`. It implements a hierarchical
path-resolution logic (See [Pull Secrets](#pull-secrets)).

### `.Component`
Component metadata available as a v2
`ocm.software/open-component-model/bindings/go/descriptor/runtime.Component`,
providing access to:
- Component name and version
- Provider information
- Resources list
- Sources
- References
- Repository contexts

## Resource Labeling

Helm values templates should be labeled in the OCM component descriptor with the
`ext.ocm.software` community namespace key:

```yaml
labels:
  - name: ext.ocm.software/helm.values-for
    value: <helm-chart-resource-name>
```

This label indicates which Helm chart resource this template is for.

The legacy key `opendefense.cloud/helm/values-for` is still recognized for
backward compatibility with already-published components, but new components
should use `ext.ocm.software/helm.values-for`.

## Dependencies

- `ocm.software/open-component-model/bindings/go/*` - OCM v2 SDK (descriptor, oci, repository, blob, ...)
- `github.com/Masterminds/sprig/v3` - Template functions
- `github.com/spf13/cobra` - CLI framework
- `oras.land/oras-go/v2` - OCI registry client and Docker credential resolution

## License

Apache-2.0. See the [LICENSE](./LICENSE) file.
