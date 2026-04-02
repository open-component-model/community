# Contributing

> For general OCM contribution guidelines (commit format, DCO, code style, PR workflow),
> see the [OCM Contributing Guide](https://github.com/open-component-model/.github/blob/main/CONTRIBUTING.md).
> This document covers community-repo-specific requirements only.

## Proposing a New Project

### 1. Create your project directory

```
projects/<your-project-name>/
├── README.md      # Required: describe what the project does and why
├── OWNERS.md      # Required: list at least one maintainer (GitHub handle)
└── ...
```

### 2. Open a Pull Request

- Title format: `feat(projects): add <project-name>` ([Conventional Commits](https://www.conventionalcommits.org/))
- The workflow will automatically apply a review checklist comment tagging @open-component-model/sig-runtime

### 3. SIG Runtime Review

A SIG Runtime voting member reviews the submission against the checklist:

- Project scope is compatible with OCM
- `OWNERS.md` lists at least one maintainer willing to maintain the project
- No security or licensing concerns
- Tests pass in CI

Once approved, the reviewer may merge the project at which point it counts as accepted.

### 4. Post-Merge

SIG Runtime will add a CODEOWNERS entry for your project, granting you and any listed
co-maintainers review authority over `projects/<your-project-name>/`.

## Contributing to an Existing Project

Standard PR process — the SIG Runtime approval gate only activates when a new
`projects/<name>/` directory is introduced.

## Naming Conventions

See [NAMING.md](./NAMING.md) for the `ext.ocm.software` namespace
specification, Go module path conventions, and directory naming rules.

## Code of Conduct

All contributors must follow the
[NeoNephos Code of Conduct](https://github.com/neonephos/.github/blob/main/CODE_OF_CONDUCT.md).
