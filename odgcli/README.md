# odgcli

CVE compliance tooling for Open Delivery Gear.

## Installation

```bash
go install github.com/open-component-model/community/odgcli@latest
```

## Configuration

odgcli uses a YAML config file at `~/.config/odgcli/config.yaml`. Values can be overridden by environment variables or CLI flags.

```yaml
base_url: "https://delivery-service.demo.ci.gardener.cloud"
github_url: "https://api.github.com"
root_component: ocm.software/ocmcli
```

| Key              | Env Variable   | Flag     | Description                  |
|------------------|----------------|----------|------------------------------|
| `base_url`       | `BASE_URL`     | —        | Delivery Service base URL    |
| `github_url`     | `GITHUB_URL`   | —        | GitHub API base URL          |
| `access_token`   | `ACCESS_TOKEN` | —        | GitHub personal access token |
| `root_component` | `ODG_ROOT`     | `--root` | Root component to browse     |

## Authentication

The access token can be provided via environment variable, config file, or the system keychain. The recommended approach is the system keychain:

```bash
# Store token in system keychain (interactive prompt)
odgcli auth login
```

## Usage

Start the interactive TUI (default when no subcommand is given):

```bash
odgcli

# Override the root component
odgcli --root ocm.software/ocmcli
```

## Development

This project uses [Task](https://taskfile.dev) as a task runner.

```bash
task          # Build the binary
task test     # Run tests
task lint     # Run golangci-lint
task check    # Run all checks (fmt, vet, lint, test)
task clean    # Remove build artifacts
```
