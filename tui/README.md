# OCM TUI

An interactive terminal UI for the [Open Component Model](https://ocm.software).

## Features

- **Explore**: browse component versions, resources, sources, references, signatures and labels in a tree with a detail pane. References expand in place. Download resources to the current directory.
- **Transfer**: a wizard that transfers a component version to another repository (OCI registry or CTF), with option selection and a review of the transformation graph.
- **Command palette**: press `:` anywhere to fuzzy-find a view, return to the menu or quit.

The colours follow the [ocm.software](https://ocm.software) brand (brand blue, cyan accent, cyan → blue gradient for progress) and adapt to light and dark terminals. Use a truecolor terminal (`COLORTERM=truecolor`) for exact brand colours.

## Getting Started

```bash
go build -o ocm-tui ./cmd
./ocm-tui                                   # menu
./ocm-tui explore ghcr.io/org/repo//comp:1.0.0   # open a view directly
./ocm-tui transfer ghcr.io/org/repo//comp:1.0.0
```

It needs an interactive terminal. Logs go to `ocm-tui/debug.log` in the user cache directory (`~/Library/Caches` on macOS, `~/.cache` on Linux).

The TUI loads OCM configuration from the same locations as the `ocm` CLI (`$OCM_CONFIG`, `$XDG_CONFIG_HOME/ocm/config`, `~/.ocmconfig`, `./.ocmconfig`, ...), including credentials, HTTP, filesystem and checksum settings. Plugins are loaded from `~/.ocm/plugins`.

It registers the same builtin access and signing types as the CLI: OCI/CTF, Helm, wget, GitHub, S3, Git, RSA, Sigstore and GPG.

## Key Bindings

| Key | Action |
|-----|--------|
| `j` / `k` / arrows | Navigate |
| `enter` | Select / expand |
| `esc` | Back (collapse node, previous wizard step, or leave view) |
| `:` | Command palette |
| `ctrl+c` | Quit |

### Explorer

| Key | Action |
|-----|--------|
| `h` / `left` | Collapse |
| `tab` | Switch focus between tree and detail pane |
| `d` | Download selected resource |
| `t` | Transfer the selected component version (opens the wizard with the source filled in) |

### Transfer

| Key | Action |
|-----|--------|
| `space` / `enter` | Toggle option / proceed |
| `esc` | Previous step |

## Architecture

The TUI follows [The Elm Architecture](https://guide.elm-lang.org/architecture/), which Bubble Tea implements:

- Every component and page is a value `Model` with `Update(msg) (Model, tea.Cmd)` and `View() string`. `Update` returns a new model and never changes the old one.
- Parents own their children and forward messages to them. Children report events back as messages (`prompt.SubmitMsg`, `list.ChosenMsg`, `tree.ExpandMsg`, `ui.ExitMsg`, `ui.OpenMsg`) and never call their parent.
- Side effects (repository calls, downloads, transfers) run only in `tea.Cmd`s, and their results come back as messages keyed by node ID.

Each top-level command is one package under `internal/view/` that holds its page and its backend (`backend.go`, the OCM calls that view needs). `cmd/main.go` registers the views by name; views open each other with `ui.OpenMsg`.

```text
cmd/                    entry point: bootstraps OCM, registers the views, parses "<view> [arg]"
internal/app/           root model: menu, command palette, active page
internal/view/explore/  explore command: reference prompt -> tree + detail pane, download
internal/view/transfer/ transfer command: source -> target -> options -> review -> run
internal/component/     reusable components: list (bubbles/list), prompt, tree
internal/ui/            brand theme, help footer (bubbles/help), frame layout, page messages
internal/ocm/           shared OCM runtime: config, builtin plugins, credentials, repositories
```

## Testing

```bash
go test ./...                              # everything; ./integration needs Docker
go test ./internal/...                     # unit and screen tests only
go test ./internal/app/ ./internal/view/... -update   # refresh testdata/*.golden after an intended UI change
```

- `integration/` runs each command end to end against an OCI registry in a container, like `bindings/go/cli/integration` in the OCM repository: explore (connect with credentials, browse, download) and transfer (registry to CTF).
- Components and views are tested by calling `Update` directly. Screen tests compare the rendered view with `testdata/*.golden`, so layout changes show up in review.

## License

Apache-2.0
