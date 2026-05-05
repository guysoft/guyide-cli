# Branch coordination notes

This branch (`feature/cli-skill-bridge`) coordinates work across multiple
GuyIDE repositories. Do **not** merge any of these branches into `main` until
the entire vertical slice is verified end-to-end and `guyide-cli` v0.1.0 is
tagged.

## Sibling branches

| Repo | Branch | Purpose |
|------|--------|---------|
| `guysoft/guyide-cli` (this) | `feature/cli-skill-bridge` | The CLI itself |
| `guysoft/vscodium.nvim` | `feature/cli-skill-bridge` | Skill v2 source (`.opencode/skills/debug-reach/`) |
| `guysoft/NvGuy` | `feature/cli-skill-bridge` | Cross-repo notes only in Phase 1; lazy hook lands in Phase 2 |
| `guysoft/tmux-ide` | `feature/cli-skill-bridge` | Phase 2 only (exports `GUYIDE_PANE_ROLE`); untouched in Phase 1 |
| `guysoft/GuyIDE` | `feature/cli-skill-bridge` | README "Install guyide" sub-step + Components row update |

## Verification checklist (run before merging)

- [ ] `go test ./... -coverprofile coverage.out` ≥ 75% on `internal/...`
- [ ] `go test -tags e2e ./...` passes against a real nvim + tmux + debugpy
- [ ] `guyide doctor` reports Ready in a fresh tmux-ide pane
- [ ] Skill v2 (`debug-reach`) drives a sample debug session start → break → state → step → continue → stop using only `guyide` calls
- [ ] CI green on linux/macos/windows × go 1.23
- [ ] goreleaser dry-run produces working tarballs for all 5 targets
- [ ] Tag `v0.1.0` on `guyide-cli` only after all of the above

## Phase 2 follow-up issues to file

- NvGuy `lua/plugins/init.lua:217` hardcoded macOS path (`/Users/guyshe/...`)
- Auto-install hook for `debug-reach` skill from NvGuy lazy build
- tmux-ide pane-role env injection
