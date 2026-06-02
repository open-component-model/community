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
└── ...
```

### 2. Add a CODEOWNERS entry

Your submission PR **must** include a CODEOWNERS change that assigns at least one
maintainer to your project directory:

```
# In .github/CODEOWNERS — add at the end:
/<your-project-name>/   @your-github-handle @optional-co-maintainer
```

### 3. Open a Pull Request

- Title format: `feat: add <project-name>` ([Conventional Commits](https://www.conventionalcommits.org/))
- The workflow will automatically post a review checklist comment tagging `@open-component-model/tsc`

### 4. TSC Review

A TSC member reviews the submission against the checklist:

- Project scope is compatible with OCM
- CODEOWNERS entry lists at least one maintainer willing to maintain the project
- No security or licensing concerns

TSC approval is enforced natively via CODEOWNERS — at least one TSC member must approve the PR before it can merge.

## Contributing to an Existing Project

Standard PR process — the TSC approval gate only activates when a new
top-level project directory is introduced.

## Naming Conventions

See [NAMING.md](./NAMING.md) for the `ext.ocm.software` namespace
specification and directory naming rules.

## Code of Conduct

All contributors must follow the
[Linux Foundation EU Code of Conduct](https://linuxfoundation.eu/policies/code-of-conduct).