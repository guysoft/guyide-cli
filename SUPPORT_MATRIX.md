# GuyIDE Support Matrix

GuyIDE is built around three pluggable slots: **editor**, **multiplexer**, and **agent**. Only the combinations listed here are validated by CI and supported by `guyide doctor` without `--allow-unsupported`.

The authoritative machine-readable copy lives at `embed/support_matrix.yaml` and is baked into the `guyide` binary. This document is the human view; if the two disagree, the YAML wins.

## Slots and drivers

| Slot | Driver | Status | Notes |
|---|---|---|---|
| editor | `nvim` | supported | Requires NvGuy ≥ v0.1.0 and vscodium.nvim ≥ v0.1.0 |
| multiplexer | `tmux` | supported | tmux ≥ 3.0; GuyIDE owns `~/.tmux.conf` |
| agent | `opencode` | supported | Default agent for v0.2 |
| agent | `claude-code` | stub | Recognised but `ErrNotImplemented`; full driver planned for v0.3 |

## Validated triples

| Editor | Multiplexer | Agent | Status | Since |
|---|---|---|---|---|
| nvim | tmux | opencode | supported | v0.2.0 |
| nvim | tmux | claude-code | stub | planned v0.3.0 |

Anything not listed is unsupported. `guyide doctor` will refuse to mark such installs as ready unless invoked with `--allow-unsupported`, in which case it downgrades the failure to a warning.

## Adding a driver

1. Add a `DriverSpec` row under the appropriate slot in `embed/support_matrix.yaml`.
2. Implement the `Component` interface under `internal/components/<slot>/<driver>.go`.
3. Add at least one validated triple to the `triples:` list.
4. Update this document.
5. Wire CI coverage for the new triple before flipping its status from `stub` to `supported`.
