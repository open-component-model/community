# Architecture

## Overview

The TUI is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and follows a component-based architecture. Each screen is a self-contained **View** that composes from shared **Components** and references a centralized **Theme**.

```
cmd/main.go                        Entry point: bootstraps OCM runtime, wires views

app.go                             Root Bubble Tea model: menu, view lifecycle, command palette
view.go                            View interface + MenuItem
keymap.go                          Global key bindings
fetch/                             Public interface definitions

internal/
  theme/                           Centralized styling
  components/                      Reusable UI primitives
  explorer/                        Component explorer view
  transfer/                        Transfer wizard view
  ocm/                             OCM runtime bootstrap and wiring
```

All implementation details live under `internal/`. The public surface is minimal: the root `tui` package (`App`, `Config`, `MenuItem`, `View`) and the `fetch` package (interfaces).

## View Interface

Every top-level screen implements the `View` interface defined in `view.go`:

```go
type View interface {
    Init() tea.Cmd
    Update(msg tea.Msg) tea.Cmd
    Render() string
    StatusInfo() string
    Hotkeys() []components.Hotkey
    OnBackPressed() int
    Resize(width, height int)
}
```

Views use **pointer receivers** so `Update` mutates in place. This avoids a circular dependency: view packages satisfy the interface via structural typing without importing the root `tui` package.

The root `app.go` stores the active view as a `View` interface value and delegates all messages to it. Views are registered as `MenuItem` closures -- the app never imports view packages directly.

## Navigation Model

Navigation follows a simple pattern inspired by Android's back stack:

- **`esc`** is intercepted by the app and routed to `OnBackPressed()`. Each view decides what "back" means:
  - Explorer: collapse the current tree node, or exit if nothing to collapse
  - Transfer: go back one wizard step, or exit at the first step
- **`:`** opens the command palette from anywhere, allowing the user to switch views, go to the menu, or quit
- **`ctrl+c`** always quits

Views never need to handle quit/back logic themselves beyond implementing `OnBackPressed`.

## Component Library

Reusable components live in `internal/components/`. They are stateless renderers or thin stateful wrappers that reference `theme.Current()` for all styling.

| Component | Package | Used By |
|-----------|---------|---------|
| **Prompt** | `internal/components/input` | Explorer reference input, transfer source/target |
| **Modal** | `internal/components/modal` | Download progress dialog |
| **Menu** | `internal/components/menu` | Main menu, command palette entries |
| **SplitPane** | `internal/components/splitpane` | Explorer tree+detail layout |
| **Palette** | `internal/components/palette` | Command palette overlay |
| **Spinner** | `internal/components/progress` | Download progress animation |
| **Tree** | `internal/components/tree` | Reusable tree navigation model |
| **Layout** | `internal/components/layout` | Standard frame: title bar + content + hotkey footer |
| **StatusBar** | `internal/components/statusbar` | Top bar with title, hint, and status info |

### Hotkey Footer

Views declare their context-sensitive key bindings via `Hotkeys()`. The `Layout` component renders them automatically in the footer. The `:: command` hint is always appended.

## Theme System

All colors and styles are defined in `internal/theme/default.go` as a `Theme` struct. Components call `theme.Current()` to get the active theme. No hardcoded `lipgloss.AdaptiveColor` values appear in view code.

To add a new theme, create a function returning `*Theme` and call `theme.SetTheme()` before starting the app.

## Per-View File Layout

Each view package follows this pattern:

| File | Responsibility |
|------|----------------|
| `view.go` | Model struct, constructors, View interface methods |
| `browse.go` | Main mode: update logic, rendering, key handling |
| `prompt.go` | Input screen (if the view starts with one) |
| Additional files | Per sub-mode (e.g. `download.go`, `steps.go`, `render.go`) |
| `keymap.go` | Key binding definitions |
| `tree.go` | Tree node builders (if applicable) |

No file exceeds ~400 lines.

## Fetch Interfaces

`fetch/fetch.go` defines the public interfaces that decouple the TUI from OCM internals:

| Interface | Purpose |
|-----------|---------|
| `ComponentFetcher` | List versions, get descriptors |
| `ResourceDownloader` | Download a resource to disk |
| `FetcherFactory` | Create a fetcher from a reference string |
| `TransferExecutor` | Build and execute transfer graphs |

Implementations live in `internal/ocm/wiring.go`.

## OCM Runtime Bootstrap

`internal/ocm/bootstrap.go` initializes the OCM runtime using only public bindings APIs (no CLI internal imports):

1. **Config**: Loads from `~/.ocm/config`, `~/.ocmconfig`, or `$OCM_CONFIG`
2. **Plugin Manager**: Creates manager, discovers plugins from `~/.ocm/plugins`
3. **Builtins**: Registers OCI repository, resource, digest, blob transformer, and credential plugins
4. **Credential Graph**: Builds from config using the registered credential repository plugins

`internal/ocm/wiring.go` creates the fetch interface implementations from the runtime, including a `repoResolver` that satisfies the transfer system's resolver interface.

## Adding a New View

1. Create a package under `internal/` (e.g. `internal/verify/`)
2. Define a `Config` struct for dependencies
3. Define a `Model` struct with the view state
4. Implement the `View` interface methods on `*Model`
5. Create a `NewView(cfg Config, width, height int) *Model` constructor
6. Compose UI from `internal/components/` primitives, reference `theme.Current()`
7. Register as a `MenuItem` in `cmd/main.go`

## Cross-View Communication

The only cross-view interaction is explorer-to-transfer (press `t` to transfer the selected component). This uses a `TransferRequestMsg` that the app intercepts. The app creates a new transfer view and calls `SetSource()` to pre-fill the source reference.

This is handled as a pragmatic exception rather than a general mechanism.
