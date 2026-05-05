# Contributing

> For general OCM contribution guidelines (commit format, DCO, code style, PR workflow),
> see the [OCM Contributing Guide](https://ocm.software/community/contributing/).
> This document covers community-repo-specific requirements only.

## Proposing a New Project

### 1. Create your project directory

Projects live directly at the root of this repository:

```
<your-project-name>/
├── README.md      # Required: describe what the project does and why
├── OWNERS.md      # Required: list at least one maintainer (GitHub handle)
└── ...
```

### 2. Open a Pull Request

- Title format: `feat: add <project-name>` ([Conventional Commits](https://www.conventionalcommits.org/))
- The workflow will automatically post a review checklist comment tagging `@open-component-model/tsc`

### 3. TSC Review

A TSC member reviews the submission against the checklist:

- Project scope is compatible with OCM
- `OWNERS.md` lists at least one maintainer willing to maintain the project
- No security or licensing concerns

TSC approval is enforced natively via CODEOWNERS — at least one TSC member must approve the PR before it can merge.

### 4. Post-Merge

The TSC will add a CODEOWNERS entry for your project, granting you and any listed
co-maintainers review authority over `/<your-project-name>/`.

## Contributing to an Existing Project

Standard PR process — the TSC approval gate only activates when a new
top-level project directory is introduced.

## Naming Conventions

See [NAMING.md](./NAMING.md) for the `ext.ocm.software` namespace
specification and directory naming rules.

## Code of Conduct

All contributors must follow the
[NeoNephos Code of Conduct](https://github.com/neonephos/.github/blob/main/CODE_OF_CONDUCT.md).