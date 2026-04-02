# ocm-sbom-generator

A community plugin that generates Software Bill of Materials (SBOM) documents for OCM component versions and attaches them as resources using the `ext.ocm.software` namespace.

## What it does

`ocm-sbom-generator` introspects an OCM component version, enumerates all OCI image and binary resources, and produces a CycloneDX or SPDX SBOM document. The resulting document is:

1. Stored as a new OCM resource of type `sbom` on the component version.
2. Labeled with `ext.ocm.software/sbom-generator.format` (e.g. `cyclonedx-json`).
3. Optionally signed using the OCM signing framework.

## Why

Compliance teams increasingly require an SBOM for every software artifact. OCM already tracks resources and their provenance, making it a natural attachment point for SBOMs. This plugin closes the gap between OCM's resource model and SBOM tooling (Syft, Trivy, etc.).

## Usage

```bash
ocm-sbom-generator generate \
  --component ghcr.io/my-org/my-component:1.0.0 \
  --format cyclonedx-json \
  --sign my-signing-key
```

## Labels & Annotations

| Key | Values | Description |
|-----|--------|-------------|
| `ext.ocm.software/sbom-generator.format` | `cyclonedx-json`, `spdx-json` | SBOM format used |
| `ext.ocm.software/sbom-generator.version` | semver string | Plugin version that produced the SBOM |

## Status

Early prototype — feedback welcome. See [OWNERS.md](./OWNERS.md) for maintainers.