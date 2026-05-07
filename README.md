# guyide-cli

`guyide` is the AI-friendly CLI bridge for [GuyIDE](https://github.com/guysoft/GuyIDE):
a single static Go binary that lets agents and humans drive `nvim`, `tmux`, and
the Debug Adapter Protocol (DAP) through stable, scriptable commands.

> Status: **alpha (v0.0.0-dev)** on branch `feature/cli-skill-bridge`. Not yet
> released to brew or `go install`. See [Phasing](#phasing) below.

## Why

AI agents driving an IDE need a stable, narrow contract — not raw msgpack-RPC,
not shell-escaped vimscript, not screen-scraped tmux output. `guyide` turns the
GuyIDE stack into verbs:

```
guyide doctor                # is everything wired up?
guyide env --json            # what socket / session / pane am I in?
guyide debug start           # launch a dap session
guyide debug state --json    # where am I stopped, what are the locals?
guyide tmux watch --until '... build complete' --timeout 30s
```

Humans running the same commands in a tmux pane get a styled, Crush-themed
report. Agents piping into `--json` get stable ndjson tagged
`"schema": "guyide/v1"`.

## Output modes

`guyide` auto-detects whether to render for humans or machines. Precedence:

1. `--json` flag → ndjson machine output
2. `--no-color` flag → plain text, no ANSI, no emoji
3. `NO_COLOR=1` env → plain text
4. `CI=true` env → plain text
5. Stdout is not a TTY (piped / redirected) → JSON
6. Otherwise → full Crush palette (Charmtone via `lipgloss`)

This means a skill calling `guyide debug state` from a subshell *automatically*
gets parseable JSON; a human running the same thing in their tmux pane sees
panels and badges.

## Install

### With Go (recommended for developers)

If you have Go 1.23+ installed:

```sh
# Latest released tag
go install github.com/guysoft/guyide-cli/cmd/guyide@latest

# Or follow the main branch (gets unreleased features)
go install github.com/guysoft/guyide-cli/cmd/guyide@main
```

The binary lands in `$(go env GOBIN)` (or `$(go env GOPATH)/bin`). Make sure
that directory is on your `PATH`.

### From an unmerged feature branch

Branch names with slashes (e.g. `feature/installer`) are rejected by the Go
module proxy. Use the commit SHA instead:

```sh
go install github.com/guysoft/guyide-cli/cmd/guyide@<commit-sha>
```

### Coming soon

Once v0.1.0+ ships, Homebrew + tarball downloads will be available. macOS and
Windows users may need to bypass Gatekeeper / SmartScreen on first run; signed
releases are planned for Phase 2.

## Build from source

```sh
git clone -b feature/cli-skill-bridge https://github.com/guysoft/guyide-cli
cd guyide-cli
go build -o ~/.local/bin/guyide ./cmd/guyide
guyide version
```

## Commands (Phase 1 surface)

| Command | Purpose |
|---------|---------|
| `guyide version` | Print version, schema, build info |
| `guyide env` | Resolve socket / tmux session / pane role |
| `guyide doctor` | Full environment health report |
| `guyide nvim status\|exec\|eval` | Drive nvim via msgpack-RPC |
| `guyide tmux panes\|send\|watch` | Pane ops, including `watch --until <regex>` |
| `guyide debug start\|stop\|state\|step\|continue` | Drive nvim-dap |
| `guyide debug break set\|list\|clear` | Manage breakpoints |
| `guyide layout info` | Inspect the tmux-ide layout |

Each command supports `--json`, `--no-color`, `--socket`, `--session`,
`--timeout`, `-v/--verbose`.

## Phasing

**Phase 1 (this branch, target v0.1.0):**
- Repo scaffold + output layer + discovery
- All commands listed above, including `tmux watch --until`
- Skill v2 rewrite for `debug-reach` (consumes this CLI exclusively)
- CI matrix (linux, macos, windows × go 1.23) + goreleaser

**Phase 2 (target v0.2.0):**
- `GUYIDE_PANE_ROLE` injected by tmux-ide
- `guyide debug eval`, `debug list-configs`, `guyide events` ndjson stream
- cosign-signed releases, SLSA provenance
- NvGuy lazy hook to auto-install the skill
- Optional `guyide mcp` subcommand

## License

GPL-3.0 — matches the rest of the GuyIDE family.
