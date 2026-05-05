---
name: guyide
description: Drive the guyide CLI — an AI-friendly bridge to nvim, tmux, and the Debug Adapter Protocol. Use when the user wants to inspect, edit, navigate, run, or debug code via the guyide environment, or asks about guyide commands (install, doctor, nvim, tmux, debug, layout, list-backups). Trigger keywords guyide, NvGuy, tmux-ide, opencode-sessions, dap, debug adapter, lazy.sync, ~/.guyide.
---

# guyide CLI skill

`guyide` is a single binary that lets you (the agent) drive nvim, tmux, and DAP from the command line. It is installed at `~/.local/bin/guyide` (or wherever the user puts it) and configured at `~/.guyide/config.yaml`.

## When to use this skill

- The user mentions "guyide", asks to install/update/uninstall the IDE bridge, or wants to know what `~/.guyide` contains.
- You need to inspect or drive a running nvim instance over msgpack-RPC without screen-scraping.
- You need to attach to or modify a tmux session managed by guyide.
- You need to run a debugger (debugpy, delve, etc.) through DAP.

## Cheat sheet

```bash
guyide doctor                  # health check: tmux session, nvim socket, RPC, opencode PATH
guyide install [--dry-run]     # install/update editor (NvGuy) + multiplexer (tmux) + agent (opencode)
guyide uninstall               # remove guyide-managed files only; user files are never touched
guyide list-backups            # show ~/.guyide/backups/*.tar.gz with reason and component
guyide env                     # show resolved socket, session, and channel
guyide nvim ...                # drive a running nvim (open file, run command, eval lua)
guyide tmux ...                # query/modify tmux session
guyide layout ...              # apply a named pane layout
guyide debug ...               # DAP commands
guyide --json <cmd>            # machine-readable output
```

## Filesystem footprint guyide owns

- `~/.guyide/` — root. Holds `config.yaml`, `components/`, `backups/`, `manifest.json`, `installed.json`.
- `~/.config/nvim` — symlink to `~/.guyide/components/nvim` (the cloned NvGuy distro).
- `~/.tmux.conf` — replaced with the embedded `guyide.conf` only when `tmux.own_conf=true` (default). The first line is `# guyide:managed v1` so guyide can detect drift.
- `~/.config/opencode/skills/guyide/` — this skill. Bears a `.guyide-managed` marker so guyide knows it owns it. Other skills and your `AGENTS.md` are never touched.

## What guyide will NEVER do

- Modify or delete the user's hand-written `AGENTS.md` or `CLAUDE.md`.
- Overwrite skills the user wrote themselves (skills without the `.guyide-managed` marker are skipped or backed up before any change).
- Touch files outside the paths above.

## Backups

Every replacement is backed up first to `~/.guyide/backups/<RFC3339>.tar.gz`. Run `guyide list-backups` to see them. To restore: extract the tarball and the paths inside (under `home/...`) map back to `$HOME/...`.

## Config (`~/.guyide/config.yaml`)

```yaml
schema: guyide/config/v1
channel: stable           # or "dev"
components:
  editor:      { driver: nvim }
  multiplexer: { driver: tmux }
  agent:       { driver: opencode }
tmux:
  own_conf: true          # set false to leave ~/.tmux.conf alone
  reload_on_install: true
nvim:
  headless_sync: true
```

Each driver block is only validated when its slot selects it; you can keep settings for inactive drivers.

## Failure modes you may hit

- **opencode not on PATH** — `guyide install` errors with the install URL. Install opencode separately, then re-run.
- **tmux drift detected** — `~/.tmux.conf` exists but lacks the marker. guyide backs it up before replacing. The backup is in `~/.guyide/backups/`.
- **NvGuy clone fails** — usually a transient git error; re-run `guyide install`. The clone is `git clone --depth 1 --branch <ref>` from `https://github.com/guysoft/NvGuy.git`.

## Source

`github.com/guysoft/guyide-cli` — open issues there.
