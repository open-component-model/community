# OCM Community

Community-contributed extensions, integrations, and tools for the
[Open Component Model](https://ocm.software).

## What is this?

This repository hosts community projects that extend OCM but live outside the core monorepo.
All projects here have been reviewed and approved by the **TSC** before merge.

## Projects

Community projects live directly at the root of this repository. Each project is an
independent directory with its own `README.md` and `OWNERS.md`.

## Contributing a New Project

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full submission process. In short:

1. Create `<your-project-name>/` at the root with the required files
2. Open a PR — the workflow will notify the TSC for review
3. After TSC approval (via CODEOWNERS review) the PR can merge

## Governance

This repository is governed by [GOVERNANCE.md](./GOVERNANCE.md) under the oversight of
the **TSC**. The namespace for all community extensions is defined in
[NAMING.md](./NAMING.md).

## Build Tool

The OCM core monorepo uses [Task](https://taskfile.dev/) as its build runner.
Community projects are encouraged (but not required) to do the same.

## Licensing

All projects in the community repository must be licensed under Apache-2.0, consistent with the
[OCM Technical Charter](https://github.com/open-component-model/open-component-model/blob/main/docs/steering/CHARTER.md).

OCM follows the [NeoNephos Code of Conduct](https://github.com/neonephos/.github/blob/main/CODE_OF_CONDUCT.md).

---

<p align="center"><img alt="Bundesministerium für Wirtschaft und Energie (BMWE)-EU funding logo" src="https://apeirora.eu/assets/img/BMWK-EU.png" width="400"/></p>