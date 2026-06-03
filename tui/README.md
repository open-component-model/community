# OCM TUI

An interactive terminal UI for the [Open Component Model](https://ocm.software).

## Features

- **Explore** -- Browse component versions, resources, sources, references, and signatures in an interactive tree view with a detail pane. Download resources directly to disk.
- **Transfer** -- Step-by-step wizard to transfer component versions between repositories with option selection and transformation graph review.
- **Command Palette** -- Press `:` from anywhere to quickly switch between views, return to the menu, or quit.

## Getting Started

```bash
go build -o ocm-tui ./cmd
./ocm-tui
```

Requires an interactive terminal. The TUI loads OCM configuration and credentials from the same locations as the `ocm` CLI (`~/.ocm/config`, `~/.ocmconfig`, `$OCM_CONFIG`, Docker config).

## Key Bindings

| Key | Action |
|-----|--------|
| `j` / `k` / arrows | Navigate |
| `enter` | Select / expand |
| `esc` | Back (collapse node, previous wizard step, or exit view) |
| `tab` | Switch pane (tree / detail) |
| `:` | Open command palette |
| `ctrl+c` | Quit |

### Explorer

| Key | Action |
|-----|--------|
| `d` | Download selected resource (modal with progress) |
| `t` | Transfer selected component version (opens transfer wizard with source pre-filled) |

### Transfer

| Key | Action |
|-----|--------|
| `space` / `enter` | Toggle option / proceed |
| `esc` | Go back one step |

## Configuration

The TUI reads OCM configuration from these locations (in order):

1. `$OCM_CONFIG` environment variable
2. `$HOME/.config/.ocm/config` or `$HOME/.config/.ocmconfig`
3. `$HOME/.ocm/config` or `$HOME/.ocmconfig`

Plugins are loaded from `$HOME/.ocm/plugins`.

## License

Apache-2.0
