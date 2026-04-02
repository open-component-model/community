# OCM Community

Community-contributed extensions, integrations, and tools for the
[Open Component Model](https://ocm.software).

## What is this?

This repository hosts community projects that extend OCM but live outside the core monorepo.
All projects here have been reviewed and approved by **SIG Runtime** before merge.

## Projects

Projects live under [`projects/`](./projects/). Each project is an independent Go module
with its own `README.md`, `go.mod`, and `Taskfile.yml`.

## Contributing a New Project

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full submission process. In short:

1. Create `projects/<your-project-name>/` with the required files
2. Open a PR — the workflow will notify SIG Runtime for review
3. After SIG Runtime approval the merge gate passes

## Governance

This repository is governed by [GOVERNANCE.md](./GOVERNANCE.md) under the oversight of
**SIG Runtime**. The namespace for all community extensions is defined in
[NAMING.md](./NAMING.md).

## Build Tool

This repository requires projects to use [Task](https://taskfile.dev/) as its build runner,
consistent with the OCM core monorepo.

## Licensing

All projects in the community repository must be licensed under Apache-2.0, consistent with the
[OCM Technical Charter](https://github.com/open-component-model/open-component-model/blob/main/docs/steering/CHARTER.md).

OCM follows the [NeoNephos Code of Conduct](https://github.com/neonephos/.github/blob/main/CODE_OF_CONDUCT.md).

---

<p align="center"><img alt="Bundesministerium für Wirtschaft und Energie (BMWE)-EU funding logo" src="https://apeirora.eu/assets/img/BMWK-EU.png" width="400"/></p>